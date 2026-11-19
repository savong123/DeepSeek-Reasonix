package boot

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"reasonix/internal/contract/event"
	"reasonix/internal/contract/provider"
	"reasonix/internal/ext/extension"
	"reasonix/internal/ext/extension/dispatch"
	"reasonix/internal/state/sessionstore"
)

func TestEffectLiveCompactReclassifiesOnModelSwitch(t *testing.T) {
	home, workspace := statedBootHome(t), robustTempDir(t)
	kind := uniqueKind("live-compact-switch")
	localWire, cloudWire := &effectRecordingProvider{}, &effectRecordingProvider{}
	provider.Register(kind, func(cfg provider.Config) (provider.Provider, error) {
		if cfg.Model == "y" {
			return cloudWire, nil
		}
		return localWire, nil
	})
	writeFile(t, workspace, "reasonix.toml", liveSwitchConfig(kind))
	approveWorkspaceAt(t, home, workspace)
	path := filepath.Join(workspace, ".reasonix", "sessions", "switch.jsonl")
	s := &sessionstore.Session{Messages: []provider.Message{
		{Role: provider.RoleSystem, Content: "SWITCH BASE"},
		{Role: provider.RoleUser, Content: "earlier user"},
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "read", Name: "read_file", Arguments: `{}`}}},
		{Role: provider.RoleTool, Name: "read_file", ToolCallID: "read", Content: strings.Repeat("historic read result\n", 10000)},
		{Role: provider.RoleAssistant, Content: "done"},
	}}
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	opts := Options{Home: home, WorkspaceRoot: workspace, Model: "local", Sink: event.Discard}
	local, err := Build(t.Context(), opts)
	if err != nil {
		t.Fatal(err)
	}
	defer local.Close()
	point := extension.PointCompactionLive
	d := dispatch.New(map[extension.InterceptorPoint][]extension.Contribution{
		point: {{Source: extension.ContributionSource{PluginID: "live-compact"}}},
	}, nil, func(string) dispatch.Client { return liveCompactEffectSidecar{t} }, nil, dispatch.Options{})
	local.SetExtensions(d)
	if err := local.Resume(s, path); err != nil {
		t.Fatal(err)
	}
	if err := local.RunTurn(t.Context(), "complete on loopback"); err != nil {
		t.Fatal(err)
	}
	if st, ok, err := sessionstore.LoadCompactionState(path); err != nil || ok && st.Projection.CoveredCount != 0 {
		t.Fatalf("loopback compacted behind the latest user: %v", err)
	}
	opts.Model = "cloud"
	cloud, err := Build(t.Context(), opts)
	if err != nil {
		t.Fatal(err)
	}
	defer cloud.Close()
	if err := ApplyRuntimeMigration(cloud, local, CaptureRuntimeMigration(local)); err != nil {
		t.Fatal(err)
	}
	cloud.SetExtensions(d)
	if err := cloud.RunTurn(t.Context(), "complete on the new provider"); err != nil {
		t.Fatal(err)
	}
	if len(localWire.requests()) != 1 || len(cloudWire.requests()) != 1 {
		t.Fatal("each turn must reach its own provider")
	}
	st, ok, err := sessionstore.LoadCompactionState(path)
	if err != nil || !ok || st.Projection.CoveredCount != 4 || st.LastReceipt == nil || st.LastReceipt.Action != "live_compact" {
		b, _ := json.Marshal(st.LastReceipt)
		t.Fatalf("switched provider did not allow cloud drain: covered=%d receipt=%s err=%v", st.Projection.CoveredCount, b, err)
	}
}

func liveSwitchConfig(kind string) string {
	return fmt.Sprintf(`
default_model = "local"
[agent]
system_prompt = "SWITCH BASE"
[environment]
enabled = false
[codegraph]
enabled = false
[[providers]]
name = "local"
kind = %q
model = "x"
base_url = "http://127.0.0.1:5000/v1"
context_window = 200000
[[providers]]
name = "cloud"
kind = %q
model = "y"
base_url = "https://cloud.example/v1"
context_window = 200000
`, kind, kind)
}
