package boot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"reasonix/internal/contract/config"
	"reasonix/internal/contract/event"
	"reasonix/internal/contract/provider"
	"reasonix/internal/contract/tool"
	"reasonix/internal/ext/pluginpkg"
	"reasonix/internal/state/sessionstore"
)

func stageLiveCompactSDK(t *testing.T) string {
	t.Helper()
	example, err := filepath.Abs(filepath.Join("..", "..", "..", "sdk", "go", "examples", "livecompact"))
	if err != nil {
		t.Fatal(err)
	}
	root := robustTempDir(t)
	ctx, cancel := context.WithTimeout(t.Context(), fullsidecarBuildTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", filepath.Join(root, "bin", "live-compact.exe"), ".")
	cmd.Dir, cmd.Env = example, goBuildEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build livecompact SDK: %v\n%s", err, out)
	}
	manifest, err := os.ReadFile(filepath.Join(example, pluginpkg.NativeManifest))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, pluginpkg.NativeManifest, string(manifest))
	return root
}

type liveChatWire struct {
	mu       sync.Mutex
	requests [][]map[string]any
}

func (w *liveChatWire) serve(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(dst http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []map[string]any `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode provider request: %v", err)
		}
		w.mu.Lock()
		w.requests = append(w.requests, body.Messages)
		n := len(w.requests)
		w.mu.Unlock()
		dst.Header().Set("Content-Type", "text/event-stream")
		if n <= 2 {
			id, path := "read", "local.txt"
			if n == 2 {
				id, path = "keep", "tail.txt"
			}
			args, _ := json.Marshal(fmt.Sprintf(`{"path":%q}`, path))
			_, _ = fmt.Fprintf(dst, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":%q,"type":"function","function":{"name":"read_file","arguments":%s}}]},"finish_reason":"tool_calls"}]}`+"\n\ndata: [DONE]\n\n", id, args)
			return
		}
		_, _ = fmt.Fprint(dst, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
}

func TestEffectLiveCompactSDKOnChatWire(t *testing.T) {
	root := stageLiveCompactSDK(t)
	isolateConfigHome(t)
	workspace := robustTempDir(t)
	t.Chdir(workspace)
	home := config.ReasonixHomeDir()
	if err := pluginpkg.Upsert(home, pluginpkg.InstalledPlugin{Name: "live-compact", Root: root, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	var wire liveChatWire
	srv := wire.serve(t)
	defer srv.Close()
	writeFile(t, workspace, "reasonix.toml", fmt.Sprintf(`
default_model = "test-model"
[agent]
system_prompt = "LIVE WIRE BASE"
[environment]
enabled = false
[codegraph]
enabled = false
[[providers]]
name = "test-model"
kind = "openai"
model = "x"
base_url = %q
context_window = 200000
`, srv.URL))
	writeFile(t, workspace, "local.txt", "UNIQUE-READ-HEAD\n"+strings.Repeat("file output line\n", 400))
	writeFile(t, workspace, "tail.txt", "intact short tool result")
	approveWorkspace(t, workspace)
	built, err := BuildRuntime(t.Context(), Options{Sink: event.Discard})
	if err != nil {
		t.Fatal(err)
	}
	ctrl := built.Controller
	defer ctrl.Close()
	if built.Extensions == nil || built.Extensions.Client("live-compact") == nil {
		t.Fatal("v2 sidecar did not activate")
	}
	path := filepath.Join(ctrl.SessionDir(), "live-wire.jsonl")
	s := liveChatTranscript()
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	if err := ctrl.Resume(s, path); err != nil {
		t.Fatal(err)
	}
	if err := ctrl.Run(t.Context(), "complete the existing fixture"); err != nil {
		t.Fatal(err)
	}
	if err := ctrl.RunTurn(t.Context(), "new unrelated turn"); err != nil {
		t.Fatal(err)
	}
	recalled, err := ctrl.Executor().RecallContext(t.Context(), tool.RecallRequest{Positions: []int{5}})
	if err != nil || !strings.Contains(recalled.Text, "UNIQUE-READ-HEAD") {
		t.Fatalf("card position did not retrieve the read: %v", err)
	}
	wire.mu.Lock()
	defer wire.mu.Unlock()
	checkLiveChatRequests(t, wire.requests)
}

func liveChatTranscript() *sessionstore.Session {
	msgs := []provider.Message{
		{Role: provider.RoleSystem, Content: "LIVE WIRE BASE"},
		{Role: provider.RoleUser, Content: strings.Repeat("retained prefix ", 6000)},
		{Role: provider.RoleAssistant, Content: "acknowledged"},
	}
	return &sessionstore.Session{Messages: msgs}
}

func checkLiveChatRequests(t *testing.T, reqs [][]map[string]any) {
	t.Helper()
	if len(reqs) != 4 || len(reqs[2]) < 8 || len(reqs[3]) < 8 {
		t.Fatalf("expected four provider requests and no model summary, got %d", len(reqs))
	}
	if !reflect.DeepEqual(reqs[2][:4], reqs[3][:4]) {
		t.Fatal("wire prefix changed before the cut")
	}
	next := reqs[3]
	card, _ := next[4]["content"].(string)
	if next[4]["role"] != "user" || !strings.Contains(card, "<live-compact>") || !strings.Contains(card, `"original":"#5"`) || strings.Contains(card, "UNIQUE-READ-HEAD") {
		t.Fatalf("unexpected provider card: %v", next[4])
	}
	if !reflect.DeepEqual(reqs[2][6:8], next[5:7]) || next[6]["tool_call_id"] != "keep" {
		t.Fatal("untouched tail lost its original call or result")
	}
}
