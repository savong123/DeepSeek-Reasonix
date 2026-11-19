package agent

import (
	"encoding/json"
	"slices"
	"strings"

	"reasonix/internal/contract/provider"
	"reasonix/internal/ext/extension/dispatch"
	"reasonix/internal/state/sessionstore"
)

const (
	liveMinSpan     = 1000
	liveRail        = 32000
	liveKeepBatches = 3
)

type liveUnit struct {
	start, end, tokens                            int
	user, safe, batch, longRead, pointer, failure bool
}

func liveCandidates(units []liveUnit, lastUser int, cloud bool) []dispatch.LiveCompactionCandidate {
	var candidates []dispatch.LiveCompactionCandidate
	reserved := 0
	var ordinary []dispatch.LiveCompactionCandidate
	for i, u := range slices.Backward(units) {
		if !cloud && u.start <= lastUser {
			break
		}
		if !u.batch {
			continue
		}
		if !u.longRead {
			reserved++
		}
		if !u.safe {
			continue
		}
		if u.longRead {
			candidates = append(candidates, u.candidate(1))
		}
		if reserved > liveKeepBatches && len(ordinary) < 8 {
			if cut, ok := liveOrdinarySpan(units, i, lastUser, cloud); ok {
				ordinary = append(ordinary, cut)
			}
		}
		if len(candidates) >= 8 {
			break
		}
	}
	return append(candidates, ordinary...)
}

func (u liveUnit) candidate(batches int) dispatch.LiveCompactionCandidate {
	return dispatch.LiveCompactionCandidate{Start: u.start, End: u.end, Tokens: u.tokens, ToolBatches: batches, LongRead: u.longRead, Pointer: u.pointer, Failure: u.failure}
}

func liveOrdinarySpan(units []liveUnit, end, lastUser int, cloud bool) (dispatch.LiveCompactionCandidate, bool) {
	u := units[end]
	if u.pointer || u.tokens > liveRail {
		return u.candidate(1), u.pointer
	}
	gain, batches := u.tokens, 1
	for _, p := range slices.Backward(units[:end]) {
		if !p.safe || p.start == lastUser || (!cloud && p.user) || p.tokens+u.tokens > liveRail || p.pointer {
			break
		}
		u.start, u.tokens = p.start, p.tokens+u.tokens
		u.failure = u.failure || p.failure
		if !p.user {
			gain += p.tokens
		}
		if p.batch {
			batches++
		}
	}
	return u.candidate(batches), gain >= liveMinSpan
}

func liveUnits(msgs []provider.Message, floor int) []liveUnit {
	units := make([]liveUnit, 0)
	for i := floor; i < len(msgs); {
		m := msgs[i]
		u := liveUnit{start: i, end: i + 1, safe: liveOrdinaryMessage(m), user: liveAuthoredUser(m), tokens: liveMessageTokens(m)}
		if len(m.ToolCalls) > 0 && m.Role == provider.RoleAssistant {
			u = liveBatch(msgs, u)
		}
		units = append(units, u)
		i = u.end
	}
	return units
}

func liveBatch(msgs []provider.Message, u liveUnit) liveUnit {
	calls := msgs[u.start].ToolCalls
	for u.end < len(msgs) && msgs[u.end].Role == provider.RoleTool {
		u.end++
	}
	results := msgs[u.start+1 : u.end]
	u.batch = len(results) == len(calls)
	if !u.batch {
		u.safe = false
		return u
	}
	seen := make(map[string]bool, len(calls))
	for i, call := range calls {
		m := results[i]
		if !livePairedCall(call, m) || seen[call.ID] {
			u.batch, u.safe = false, false
			return u
		}
		seen[call.ID] = true
		tokens := liveMessageTokens(m)
		failure := liveToolFailed(m)
		u.failure = u.failure || failure
		u.longRead = u.longRead || call.Name == "read_file" && tokens >= liveMinSpan
		u.pointer = u.pointer || !failure && tokens > liveRail
		u.safe = u.safe && !liveSpecialMessage(m) && call.ThoughtSignature == "" && !(failure && liveExternalToolBody(m))
		u.tokens += tokens
	}
	return u
}

func livePairedCall(call provider.ToolCall, result provider.Message) bool {
	return call.ID != "" && call.Name != "" && (call.Arguments == "" || json.Valid([]byte(call.Arguments))) &&
		result.ToolCallID == call.ID && result.Name == call.Name && len(result.ToolCalls) == 0
}

func liveSpecialMessage(m provider.Message) bool {
	return m.LocalOnly || m.Derived || m.HostAuthored || m.InterruptedTurn != nil || m.AgentComposition != nil || m.ProviderContent != "" ||
		len(m.Images)+len(m.UnavailableImages)+len(m.Videos)+len(m.VideoParts)+len(m.ResponsesItems) != 0 ||
		m.ReasoningSignature != "" || m.ReasoningID != "" || m.ReasoningStatus != ""
}

func liveOrdinaryMessage(m provider.Message) bool {
	return !liveSpecialMessage(m) && (liveAuthoredUser(m) || m.Role == provider.RoleAssistant && m.ToolCallID == "")
}

func liveAuthoredUser(m provider.Message) bool {
	if m.Role != provider.RoleUser || m.HostAuthored || m.LocalOnly || isCompactionSummary(m) {
		return false
	}
	_, steer := sessionstore.SteerText(m.Content)
	return !steer
}

func liveLastUser(msgs []provider.Message) int {
	for i, msg := range slices.Backward(msgs) {
		if liveAuthoredUser(msg) {
			return i
		}
	}
	return -1
}

func liveMessageTokens(m provider.Message) int {
	n := len(m.Content) + len(m.ReasoningContent)
	for _, call := range m.ToolCalls {
		n += len(call.Arguments) + len(call.Name)
	}
	return estimateTokensFromBytes(n)
}

func liveExternalToolBody(m provider.Message) bool {
	// This is the host's wire envelope, not a classification from tool prose.
	return strings.HasPrefix(m.Content, "[external content · ")
}

func liveToolFailed(m provider.Message) bool {
	return m.ToolFailure != nil || failedExecution(m.ToolExecution)
}
