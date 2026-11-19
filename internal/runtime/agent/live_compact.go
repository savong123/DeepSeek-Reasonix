package agent

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync/atomic"
	"time"

	"reasonix/internal/contract/provider"
	"reasonix/internal/ext/extension"
	"reasonix/internal/ext/extension/dispatch"
	"reasonix/internal/state/sessionstore"
)

const (
	liveMinInput      = 20000
	liveMaxInput      = 650000
	liveUrgent        = 48000
	liveMaxPasses     = 8
	liveCompactAction = "live_compact"
)

type liveGrowth struct {
	path string
	max  int
}

// LiveCompactTurn binds optional turn-end maintenance to one conversation.
type LiveCompactTurn struct {
	agent                  *Agent
	session                *sessionstore.Session
	path                   string
	projection, generation uint64
	baseTokens             int
	cloud                  bool
	answered               atomic.Bool
	dispatch               *dispatch.Dispatcher
}

type liveCompactTurnKey struct{}

func (a *Agent) BeginLiveCompact(path string, cloud bool) *LiveCompactTurn {
	if a == nil || path == "" || !a.svc.extensions.Intercepts(extension.PointCompactionLive) {
		return nil
	}
	return a.window().beginLiveCompact(a, path, cloud)
}

func (a *contextWindow) beginLiveCompact(owner *Agent, path string, cloud bool) *LiveCompactTurn {
	t := &LiveCompactTurn{agent: owner, session: a.Session(), path: path, cloud: cloud, dispatch: a.svc.extensions}
	a.sess.win.compactionMu.Lock()
	st := a.sess.win.compactionState
	bound := a.sess.path
	a.sess.win.compactionMu.Unlock()
	if bound != path || t.session == nil {
		return nil
	}
	t.projection, t.generation = st.Projection.ProjectionVersion, st.Generation
	if cloud {
		t.baseTokens = a.estimatedVisibleRequestTokens(a.modelVisibleHistory())
	}
	return t
}

func (t *LiveCompactTurn) Path() string { return t.path }

func (t *LiveCompactTurn) Context(ctx context.Context) context.Context {
	if t == nil {
		return ctx
	}
	return context.WithValue(ctx, liveCompactTurnKey{}, t)
}

func (a *Agent) noteLiveCompactAnswer(ctx context.Context, ms int64, err error) {
	t, _ := ctx.Value(liveCompactTurnKey{}).(*LiveCompactTurn)
	if t != nil && t.agent == a && err == nil && ms > 0 {
		t.answered.Store(true)
	}
}

func (t *LiveCompactTurn) Answered() bool { return t != nil && t.answered.Load() }

func (t *LiveCompactTurn) Finish(ctx context.Context, version uint64, guard func(func()), allow func() bool) {
	if !t.Answered() || ctx.Err() != nil {
		return
	}
	t.agent.window().finishLiveCompact(ctx, t, version, guard, allow)
}

type liveCompactWork struct {
	turn      *LiveCompactTurn
	canonical []provider.Message
	version   uint64
	baseline  sessionstore.PersistedState
	state     sessionstore.CompactionState
	context   dispatch.LiveCompactionContext
}

func (a *contextWindow) finishLiveCompact(ctx context.Context, t *LiveCompactTurn, version uint64, guard func(func()), allow func() bool) {
	work, ok := a.liveCompactWork(t, version)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	units := liveUnits(work.canonical, work.context.Floor)
	for pass := 0; pass < liveMaxPasses && ctx.Err() == nil; pass++ {
		if !a.liveCompactOnce(ctx, &work, units, guard, allow) || !work.context.Drain || work.context.InputTokens <= liveRail {
			return
		}
	}
}

func (a *contextWindow) liveCompactWork(t *LiveCompactTurn, version uint64) (liveCompactWork, bool) {
	w := liveCompactWork{turn: t, version: version}
	if a.Session() != t.session || t.session.TranscriptVersion() != version {
		return w, false
	}
	var ok bool
	w.baseline, ok = t.session.PersistedState(t.path)
	if !ok || !w.baseline.RevisionKnown || !w.baseline.UnchangedSincePersisted {
		return w, false
	}
	w.canonical, version = t.session.SnapshotMessagesVersion()
	if version != w.version {
		return w, false
	}
	w.state, ok = a.liveInitialState(t, w.canonical)
	if !ok {
		return w, false
	}
	floor := w.state.Projection.CoveredCount
	tokens := a.estimatedVisibleRequestTokens(w.visible())
	growth := a.liveTurnGrowth(t, tokens)
	drain := t.cloud && (tokens >= liveUrgent || (growth > 0 && tokens+growth > liveRail))
	w.context = dispatch.LiveCompactionContext{Floor: floor, LastUser: liveLastUser(w.canonical), InputTokens: tokens, Growth: growth, Cloud: t.cloud, Drain: drain}
	return w, liveInputEligible(tokens) && (!t.cloud || drain && tokens > liveRail)
}

func liveInputEligible(tokens int) bool { return tokens >= liveMinInput && tokens < liveMaxInput }

func (a *contextWindow) liveInitialState(t *LiveCompactTurn, msgs []provider.Message) (sessionstore.CompactionState, bool) {
	a.sess.win.compactionMu.Lock()
	defer a.sess.win.compactionMu.Unlock()
	st := a.sess.win.compactionState
	if a.sess.path != t.path || st.Foreign() || st.Generation != t.generation || st.Projection.ProjectionVersion != t.projection {
		return st, false
	}
	if st.Projection.CoveredCount != 0 && !projectionValid(st, msgs, a.currentPromptCacheKeyLocked(), nil) {
		return st, false
	}
	return st, true
}

func (a *contextWindow) liveTurnGrowth(t *LiveCompactTurn, tokens int) int {
	if !t.cloud {
		return 0
	}
	a.sess.win.compactionMu.Lock()
	defer a.sess.win.compactionMu.Unlock()
	if a.sess.path != t.path || a.Session() != t.session {
		return 0
	}
	g := &a.sess.win.liveGrowth
	if g.path != t.path {
		*g = liveGrowth{path: t.path}
	}
	g.max = max(g.max, tokens-t.baseTokens)
	return g.max
}

func (w *liveCompactWork) visible() []provider.Message {
	if w.state.Projection.CoveredCount == 0 {
		return w.canonical
	}
	return modelVisibleFromProjection(w.state.Projection, w.canonical)
}

func (a *contextWindow) liveCompactOnce(ctx context.Context, w *liveCompactWork, units []liveUnit, guard func(func()), allow func() bool) bool {
	for len(units) != 0 && units[0].start < w.context.Floor {
		units = units[1:]
	}
	choices := liveCandidates(units, w.context.LastUser, w.context.Cloud)
	cards := make(map[[2]int]provider.Message, len(choices))
	eligible := choices[:0]
	for _, c := range choices {
		card, ok := renderLiveCut(w.canonical, c.Start, c.End)
		if ok && liveMessageTokens(card) < c.Tokens && a.liveRolesAllowCard(w, c.Start, c.End) {
			eligible = append(eligible, c)
			cards[[2]int{c.Start, c.End}] = card
		}
	}
	if len(eligible) == 0 {
		return false
	}
	payload := dispatch.LiveCompactionPayload{Context: &w.context, Candidates: eligible}
	result, err := w.turn.dispatch.Intercept(ctx, extension.PointCompactionLive, &payload)
	if err != nil || result.Blocked || payload.Cut == nil || ctx.Err() != nil {
		return false
	}
	rangeKey := [2]int{*payload.Cut.Start, *payload.Cut.End}
	card, ok := cards[rangeKey]
	return ok && a.applyLiveCut(ctx, w, rangeKey, card, guard, allow)
}

func (a *contextWindow) liveRolesAllowCard(w *liveCompactWork, start, end int) bool {
	if !a.strictAlternatingRoles {
		return true
	}
	// A user-role card must not be coalesced into either unchanged neighbor.
	// ModelMessages removes LocalOnly records before doing role coalescing.
	for _, m := range w.canonical[end:] {
		if !m.LocalOnly {
			if m.Role == provider.RoleUser {
				return false
			}
			break
		}
	}
	for i := start - 1; i >= w.context.Floor; i-- {
		if m := w.canonical[i]; !m.LocalOnly {
			return m.Role != provider.RoleUser
		}
	}
	for _, msg := range slices.Backward(w.state.Projection.Messages) {
		if !msg.LocalOnly {
			return msg.Role != provider.RoleUser
		}
	}
	return true
}

func (a *contextWindow) applyLiveCut(ctx context.Context, w *liveCompactWork, cut [2]int, card provider.Message, guard func(func()), allow func() bool) bool {
	next, tokens := a.liveCutProjection(w, cut, card)
	if tokens >= w.context.InputTokens {
		return false
	}
	var installed bool
	guard(func() { installed = a.commitLiveProjection(ctx, w, &next, allow) })
	if installed {
		w.state = next
		w.context.Floor, w.context.InputTokens = cut[1], tokens
		a.emitContextMaintenance(next.LastReceipt)
	}
	return installed
}

func (a *contextWindow) liveCutProjection(w *liveCompactWork, cut [2]int, card provider.Message) (sessionstore.CompactionState, int) {
	st := w.state
	previous := st.Projection
	out := make([]provider.Message, 0, len(previous.Messages)+cut[0]-previous.CoveredCount+1)
	out = append(out, previous.Messages...)
	out = append(out, liveCheckpointGap(w.canonical[previous.CoveredCount:cut[0]])...)
	out = append(out, card)
	covered := cut[1]
	now := time.Now().UTC()
	st.SchemaVersion, st.TranscriptVersion = sessionstore.CompactionStateSchemaCurrent, w.version
	st.Generation++
	st.PromptCacheKey = a.currentPromptCacheKey()
	st.UpdatedAt = now
	st.Projection = sessionstore.ContextProjection{Messages: out, TranscriptVersion: w.version,
		ProjectionVersion: previous.ProjectionVersion + 1, CoveredCount: covered, CoveredPrefixHash: sessionstore.CoveredPrefixHash(w.canonical, covered),
		SummaryHash: previous.SummaryHash, SourceTokens: w.context.InputTokens, CreatedAt: now}
	visible := modelVisibleFromProjection(st.Projection, w.canonical)
	used := a.estimatedVisibleRequestTokens(visible)
	st.Projection.ProjectionTokens = used
	st.Projection.ViewInputHash = a.liveRequestHash(w.visible())
	st.Projection.ViewOutputHash = a.liveRequestHash(visible)
	st.LastReceipt = liveCutReceipt(st.Projection, w.context.InputTokens, now)
	return st, used
}

func (a *contextWindow) liveRequestHash(msgs []provider.Message) string {
	return a.contextMaintenanceInputHash(a.providerProjectionMessages(provider.ModelMessages(msgs)))
}

func liveCutReceipt(p sessionstore.ContextProjection, source int, now time.Time) *sessionstore.ContextMaintenanceReceipt {
	return &sessionstore.ContextMaintenanceReceipt{
		OperationID: fmt.Sprintf("live-%d-%s", p.ProjectionVersion, p.ViewOutputHash), Status: "applied", Action: liveCompactAction, Trigger: "turn_end",
		SourceProjection: p.ProjectionVersion - 1, ProjectionVersion: p.ProjectionVersion, CoveredCount: p.CoveredCount, CoveredPrefixHash: p.CoveredPrefixHash,
		InputHash: p.ViewInputHash, OutputHash: p.ViewOutputHash, InputTokens: source, ResultTokens: p.ProjectionTokens,
		SavedTokens: max(0, source-p.ProjectionTokens), CacheBreak: true, CreatedAt: now,
	}
}

func (a *contextWindow) commitLiveProjection(ctx context.Context, w *liveCompactWork, next *sessionstore.CompactionState, allow func() bool) bool {
	a.sess.win.compactionMu.Lock()
	defer a.sess.win.compactionMu.Unlock()
	st := a.sess.win.compactionState
	t := w.turn
	if ctx.Err() != nil || a.svc.extensions != t.dispatch || a.Session() != t.session || a.sess.path != t.path || a.currentPromptCacheKeyLocked() != next.PromptCacheKey ||
		st.Projection.ProjectionVersion != w.state.Projection.ProjectionVersion || st.Generation != w.state.Generation {
		return false
	}
	// Several small cuts must not replenish the recall budget. A native fold
	// still starts its own budget; capture spending under the same lock as recall.
	next.Recall = sessionstore.RecallLedger{Generation: next.Generation}
	if st.Recall.Generation == st.Generation {
		next.Recall.SpentTokens = st.Recall.SpentTokens
	}
	if err := t.session.SaveDurableCompactionState(ctx, t.path, *next, w.baseline, w.version, allow); err != nil {
		slog.Debug("agent: live compaction not published", "err", err)
		return false
	}
	a.sess.win.compactionState = *next
	a.sess.win.checkpointState = "applied"
	return true
}
