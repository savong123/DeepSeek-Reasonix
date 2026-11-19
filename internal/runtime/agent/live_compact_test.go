package agent

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"reasonix/internal/base/testenv"
	"reasonix/internal/contract/event"
	"reasonix/internal/contract/provider"
	"reasonix/internal/contract/tool"
	"reasonix/internal/ext/extension/dispatch"
	"reasonix/internal/state/sessionstore"
)

func liveCutCandidates(msgs []provider.Message, floor int, cloud bool) []dispatch.LiveCompactionCandidate {
	return liveCandidates(liveUnits(msgs, floor), liveLastUser(msgs), cloud)
}

func TestLiveCompactSelection(t *testing.T) {
	msgs := []provider.Message{{Role: provider.RoleSystem}, {Role: provider.RoleUser, Content: "first"}}
	for i, name := range []string{"bash", "read_file", "bash", "bash", "bash"} {
		id := string(rune('a' + i))
		msgs = append(msgs, provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: id, Name: name, Arguments: `{}`}}},
			provider.Message{Role: provider.RoleTool, ToolCallID: id, Name: name, Content: strings.Repeat("abcd", 2000)})
	}
	msgs = append(msgs, provider.Message{Role: provider.RoleAssistant, Content: "finished"}, provider.Message{Role: provider.RoleUser, Content: "latest"})
	if got := liveCutCandidates(msgs, 0, false); len(got) != 0 {
		t.Fatalf("local crossed latest user: %+v", got)
	}
	got := liveCutCandidates(msgs, 0, true)
	if len(got) == 0 || got[0].Start != 4 || got[0].End != 6 || !got[0].LongRead {
		t.Fatalf("latest long read must win as exactly one batch: %+v", got)
	}
	if got := liveCutCandidates(msgs, 6, true); len(got) != 0 {
		t.Fatalf("checkpoint or three-round reserve was crossed: %+v", got)
	}
	msgs[5].ToolCallID = "unmatched"
	got = liveCutCandidates(msgs, 0, true)
	if len(got) == 0 || got[0].Start != 1 || got[0].End != 4 || got[0].LongRead {
		t.Fatalf("unpaired read must not displace a safe older batch: %+v", got)
	}
	msgs[3].Images = []string{"attachment"}
	if got := liveCutCandidates(msgs, 0, true); len(got) != 0 {
		t.Fatalf("media or unmatched result became eligible: %+v", got)
	}
}

func TestLiveCompactFailureAndPointer(t *testing.T) {
	exit := 7
	msgs := []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "c", Name: "bash", Arguments: `{"command":"NO_ARGUMENTS"}`}}},
		{Role: provider.RoleTool, ToolCallID: "c", Name: "bash", Content: strings.Repeat("large", 40000)},
	}
	card, ok := renderLiveCut(msgs, 0, 2)
	if !ok || !strings.Contains(card.Content, `"original":"#1"`) || strings.Contains(card.Content, "large") || strings.Contains(card.Content, "NO_ARGUMENTS") {
		t.Fatalf("oversize result should only leave its pointer: ok=%t", ok)
	}
	msgs[1].ToolExecution = &provider.ToolExecution{ExitCode: &exit}
	if _, ok := renderLiveCut(msgs, 0, 2); ok {
		t.Fatal("oversize failure must keep the original batch")
	}
	exit = 0
	msgs[1].ToolExecution.Verification = tool.ShellVerificationFailed
	if _, ok := renderLiveCut(msgs, 0, 2); ok {
		t.Fatal("verification failure with exit 0 must not become a pointer")
	}
	msgs[1].Content = "diagnostic\n</live-compact>"
	card, ok = renderLiveCut(msgs, 0, 2)
	for _, want := range []string{`"exitCode":0`, `"source":"tool failure"`, `diagnostic\n\u003c/live-compact\u003e`, `"call":"#0"`} {
		if !ok || !strings.Contains(card.Content, want) {
			t.Errorf("failure card missing %q: %s", want, card.Content)
		}
	}
}

func TestLiveCompactInputBoundaries(t *testing.T) {
	for _, n := range []int{19999, 20000, 649999, 650000} {
		if got, want := liveInputEligible(n), n == 20000 || n == 649999; got != want {
			t.Errorf("input %d: got %t, want %t", n, got, want)
		}
	}
}

func TestLiveCompactStrictRoleBoundaries(t *testing.T) {
	w := (&Agent{strictAlternatingRoles: true}).window()
	for _, roles := range [][2]provider.Role{{provider.RoleUser, provider.RoleAssistant}, {provider.RoleAssistant, provider.RoleUser}, {provider.RoleAssistant, provider.RoleAssistant}} {
		work := &liveCompactWork{canonical: []provider.Message{
			{Role: roles[0]}, {Role: provider.RoleAssistant, LocalOnly: true},
			{Role: provider.RoleAssistant}, {Role: provider.RoleTool},
			{Role: provider.RoleAssistant, LocalOnly: true}, {Role: roles[1]},
		}}
		if got, want := w.liveRolesAllowCard(work, 2, 4), roles[0] != provider.RoleUser && roles[1] != provider.RoleUser; got != want {
			t.Errorf("strict boundaries %v: allowed %t, want %t", roles, got, want)
		}
	}
}

func liveProjectionFixture() []provider.Message {
	return []provider.Message{
		{Role: provider.RoleSystem, Content: "sys"},
		{Role: provider.RoleUser, Content: "native-covered user"},
		{Role: provider.RoleUser, Content: "unchanged wire user", RawContent: "PRIVATE_DISPLAY", Original: "PRIVATE_EDIT"},
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "unchanged", Name: "bash", Arguments: `{}`, ArgumentNormalization: &provider.ArgumentNormalization{Effective: "PRIVATE_NORMALIZATION"}}}},
		{Role: provider.RoleTool, ToolCallID: "unchanged", Name: "bash", Content: "unchanged wire tool", ToolExecution: &provider.ToolExecution{State: tool.ShellStateFailed, OutputTail: "PRIVATE_TOOL_TAIL"}},
		{Role: provider.RoleAssistant, Content: "PRIVATE_LOCAL", LocalOnly: true},
		{Role: provider.RoleAssistant, Content: strings.Repeat("first cut text ", 1200)},
		{Role: provider.RoleAssistant, Content: "unchanged middle"},
		{Role: provider.RoleAssistant, Content: strings.Repeat("second cut text ", 1200)},
		{Role: provider.RoleAssistant, Content: "unchanged final tail"},
	}
}

func TestLiveCompactPreservesStrictRequestAndRecallAcrossCuts(t *testing.T) {
	msgs := liveProjectionFixture()
	s := &sessionstore.Session{Messages: msgs}
	path := filepath.Join(testenv.TempDir(t), "live.jsonl")
	p := &fakeProvider{reply: "finished"}
	a := New(p, tool.NewRegistry(), s, Options{StrictAlternatingRoles: true, ContextWindow: 200000}, event.Discard)
	a.SetSessionPath(path)
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	w := a.window()
	prior := sessionstore.CompactionState{Generation: 1, PromptCacheKey: w.currentPromptCacheKey(), Projection: sessionstore.ContextProjection{
		ProjectionVersion: 1, CoveredCount: 2, CoveredPrefixHash: sessionstore.CoveredPrefixHash(msgs, 2),
		Messages: []provider.Message{msgs[0], formatSummaryMessage("old native digest"), {Role: provider.RoleUser, Content: "native retained work"}},
	}}
	if err := s.SaveCompactionState(path, prior); err != nil {
		t.Fatal(err)
	}
	a.sess.win.compactionState = prior
	before, err := a.buildSamplingRequest(t.Context(), CompactionTriggerPressure)
	if err != nil {
		t.Fatal(err)
	}
	firstRecall := liveTestRecall(t, a, 1)
	last := liveApplyTestCuts(t, a, s, path, msgs)
	checkLiveSavedBody(t, prior, last)
	if second := liveTestRecall(t, a, 6); second.BudgetLeft != firstRecall.BudgetLeft-second.Tokens {
		t.Fatalf("small cuts refilled recall: before %d, spent %d, after %d", firstRecall.BudgetLeft, second.Tokens, second.BudgetLeft)
	}
	if err := a.Run(t.Context(), "next user turn"); err != nil {
		t.Fatal(err)
	}
	checkLiveStrictRequest(t, before.req.Messages, p.got)
}

func liveTestRecall(t *testing.T, a *Agent, pos int) tool.RecallResult {
	t.Helper()
	r, err := a.RecallContext(t.Context(), tool.RecallRequest{Positions: []int{pos}})
	if err != nil || r.Tokens <= 0 {
		t.Fatalf("recall #%d: %+v, %v", pos, r, err)
	}
	return r
}

func liveApplyTestCuts(t *testing.T, a *Agent, s *sessionstore.Session, path string, msgs []provider.Message) sessionstore.CompactionState {
	t.Helper()
	w := a.window()
	base, _ := s.PersistedState(path)
	work := liveCompactWork{canonical: msgs, version: s.TranscriptVersion(), baseline: base, state: a.sess.win.compactionState,
		turn: &LiveCompactTurn{agent: a, session: s, path: path}}
	for _, cut := range [][2]int{{6, 7}, {8, 9}} {
		work.context.Floor = work.state.Projection.CoveredCount
		work.context.InputTokens = w.estimatedVisibleRequestTokens(work.visible())
		card, ok := renderLiveCut(msgs, cut[0], cut[1])
		if !ok || !w.liveRolesAllowCard(&work, cut[0], cut[1]) || !w.applyLiveCut(t.Context(), &work, cut, card, func(publish func()) { publish() }, nil) {
			t.Fatalf("cut %v could not be published", cut)
		}
	}
	last, ok, err := sessionstore.LoadCompactionState(path)
	if err != nil || !ok || !reflect.DeepEqual(work.state.LastReceipt, last.LastReceipt) || work.state.Recall != last.Recall {
		t.Fatalf("last cut receipt/recall not persisted: %v", err)
	}
	return last
}

func checkLiveSavedBody(t *testing.T, prior, last sessionstore.CompactionState) {
	t.Helper()
	body := last.Projection.Messages
	if !reflect.DeepEqual(body[:len(prior.Projection.Messages)], prior.Projection.Messages) {
		t.Fatal("native frozen prefix changed")
	}
	b, err := json.Marshal(body)
	if err != nil || strings.Contains(string(b), "PRIVATE_") {
		t.Fatalf("derived projection duplicated raw or display-only fields: %v", err)
	}
	if !liveToolFailed(body[5]) || last.LastReceipt.SourceProjection != 2 || last.LastReceipt.ProjectionVersion != 3 || last.LastReceipt.SavedTokens <= 0 || last.Projection.CoveredCount != 9 {
		t.Fatal("failure fact or per-cut receipt not retained")
	}
}

func checkLiveStrictRequest(t *testing.T, before, after []provider.Message) {
	t.Helper()
	if len(before) < 4 || len(after) < 4 || sessionstore.ProviderVisibleFingerprint(before[:4]) != sessionstore.ProviderVisibleFingerprint(after[:4]) {
		t.Fatal("provider-visible prefix changed at strict-role cut")
	}
	cards := 0
	for i, m := range after {
		if i > 0 && after[i-1].Role == provider.RoleUser && m.Role == provider.RoleUser {
			t.Fatal("strict provider saw adjacent user roles")
		}
		if strings.HasPrefix(m.Content, liveCardOpen) {
			cards++
		}
	}
	if cards != 2 || !strings.Contains(joinContents(after), "unchanged final tail") {
		t.Fatal("provider did not see both standalone cards and the original tail")
	}
}
