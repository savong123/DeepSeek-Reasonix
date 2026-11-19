package boot

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"reasonix/internal/contract/event"
	"reasonix/internal/contract/provider"
	"reasonix/internal/contract/tool"
	"reasonix/internal/ext/extension"
	"reasonix/internal/ext/extension/dispatch"
	"reasonix/internal/ext/extension/protocol"
	"reasonix/internal/state/sessionstore"
)

type liveCompactEffectSidecar struct {
	t *testing.T
}

func (s liveCompactEffectSidecar) Intercept(_ context.Context, point protocol.InterceptEvent, raw json.RawMessage, _ time.Duration) (protocol.InterceptResult, error) {
	s.t.Helper()
	if point != "compaction.live" || strings.Contains(string(raw), "PRIVATE_") {
		s.t.Errorf("unexpected metadata at %s: %s", point, raw)
	}
	var p struct {
		Candidates []struct {
			Start int `json:"start"`
			End   int `json:"end"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(raw, &p); err != nil || len(p.Candidates) == 0 {
		s.t.Errorf("candidates: %s (%v)", raw, err)
		return protocol.InterceptResult{Decision: protocol.DecisionContinue}, nil
	}
	r, err := json.Marshal(map[string]any{"cut": p.Candidates[0]})
	return protocol.InterceptResult{Decision: protocol.DecisionReplace, Replacement: r}, err
}

func (liveCompactEffectSidecar) TryNotifyEvent(protocol.InterceptEvent, json.RawMessage) error {
	return nil
}

func TestEffectLiveCompactPersistsNextProviderView(t *testing.T) {
	home, workspace := statedBootHome(t), robustTempDir(t)
	kind := uniqueKind("live-compact-effect")
	rec := &effectRecordingProvider{}
	provider.Register(kind, func(provider.Config) (provider.Provider, error) { return rec, nil })
	writeFile(t, workspace, "reasonix.toml", `
default_model = "test-model"
[agent]
system_prompt = "LIVE BASE"
[environment]
enabled = false
[codegraph]
enabled = false
[[providers]]
name = "test-model"
kind = "`+kind+`"
model = "x"
base_url = "https://example.com/v1"
context_window = 200000
`)
	approveWorkspaceAt(t, home, workspace)
	path := filepath.Join(workspace, ".reasonix", "sessions", "live.jsonl")
	point := extension.InterceptorPoint("compaction.live")
	d := dispatch.New(map[extension.InterceptorPoint][]extension.Contribution{
		point: {{Source: extension.ContributionSource{PluginID: "live-compact"}}},
	}, nil, func(string) dispatch.Client { return liveCompactEffectSidecar{t} }, nil, dispatch.Options{})
	result := provider.Message{Role: provider.RoleTool, Name: "read_file", ToolCallID: "read", Content: strings.Repeat("PRIVATE_OUTPUT0123456789\n", 10000)}
	s := &sessionstore.Session{Messages: []provider.Message{
		{Role: provider.RoleSystem, Content: "LIVE BASE"},
		{Role: provider.RoleUser, Content: "original user"},
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "read", Name: "read_file", Arguments: `{"path":"PRIVATE_PATH"}`}}},
		result,
		{Role: provider.RoleAssistant, Content: "untouched assistant"},
	}}
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	for i, input := range []string{"complete first turn", "after restart"} {
		ctrl, err := Build(t.Context(), Options{Home: home, WorkspaceRoot: workspace, Sink: event.Discard})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(ctrl.Close)
		loaded, err := sessionstore.LoadSession(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := ctrl.Resume(loaded, path); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			ctrl.SetExtensions(d)
		} else {
			got, err := ctrl.Executor().RecallContext(t.Context(), tool.RecallRequest{Positions: []int{1}})
			if err != nil || !strings.Contains(got.Text, "original user") {
				t.Fatalf("restart did not restore canonical recall: %+v, %v", got, err)
			}
		}
		run := ctrl.RunTurn
		if i == 1 {
			run = ctrl.Run
		}
		if err := run(t.Context(), input); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			st, ok, err := sessionstore.LoadCompactionState(path)
			if err != nil || !ok || st.Projection.CoveredCount != 4 || st.LastReceipt == nil || st.LastReceipt.Action != "live_compact" {
				t.Fatalf("checkpoint not persisted at the selected range: found=%t, receipt=%+v, covered=%d, err=%v", ok, st.LastReceipt, st.Projection.CoveredCount, err)
			}
		}
		ctrl.Close()
	}
	reqs := rec.requests()
	if len(reqs) != 2 {
		t.Fatalf("small cut must not call a summarizer: provider got %d requests", len(reqs))
	}
	if !reflect.DeepEqual(reqs[0].Messages[:2], reqs[1].Messages[:2]) {
		t.Fatal("provider prefix before the selected batch changed")
	}
	found := map[string]bool{}
	for _, msg := range reqs[1].Messages {
		for _, want := range []string{"<live-compact>", "#3", "untouched assistant", "after restart"} {
			found[want] = found[want] || strings.Contains(msg.Content, want)
		}
		if strings.Contains(msg.Content, "PRIVATE_") || len(msg.ToolCalls) != 0 || msg.Role == provider.RoleTool {
			t.Fatal("original or unmatched batch reached the next provider request")
		}
	}
	for want, ok := range found {
		if !ok {
			t.Errorf("next provider request is missing %q", want)
		}
	}
	stored, err := sessionstore.LoadSession(path)
	if err != nil || !reflect.DeepEqual(stored.Snapshot()[3], result) {
		t.Fatalf("canonical #3 must still contain the original result: %v", err)
	}
}
