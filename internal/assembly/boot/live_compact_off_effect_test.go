package boot

import (
	"context"
	"strings"
	"testing"

	"reasonix/internal/contract/event"
	"reasonix/internal/contract/provider"
	"reasonix/internal/state/sessionstore"
)

// TestEffectNativeCompactionRunsWithLiveCompactOff is the switch-off boundary.
// With no plugin contributing at compaction.live, an oversized window must
// still fold through the host's own summary path, and no live-compact card or
// receipt may appear: the bundled package is an opt-in, never a replacement
// for Reasonix's own context management.
func TestEffectNativeCompactionRunsWithLiveCompactOff(t *testing.T) {
	t.Parallel()
	home := statedBootHome(t)
	dir := robustTempDir(t)
	kind := uniqueKind("live-compact-off")
	rec := &compactionEffectProvider{bulk: strings.Repeat("work output line with detail. ", 400)}
	provider.Register(kind, func(provider.Config) (provider.Provider, error) {
		return rec, nil
	})
	// The retention fixture's budget: 32000 stays under the 64K line where the
	// recent tail takes its 32K floor, and then no fold would happen at all.
	writeFile(t, dir, "reasonix.toml", `
default_model = "test-model"

[agent]
system_prompt = "OFF BASE"
compact_ratio = 0.5

[[providers]]
name = "test-model"
kind = "`+kind+`"
model = "x"
context_window = 32000
`)
	approveWorkspaceAt(t, home, dir)

	ctrl, err := Build(context.Background(), Options{Home: home, WorkspaceRoot: dir, Sink: event.Discard})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	for _, prompt := range []string{"start the task",
		"keep going", "keep going", "keep going", "keep going", "keep going"} {
		if err := ctrl.Run(context.Background(), prompt); err != nil {
			t.Fatalf("Run(%q): %v", prompt, err)
		}
	}

	reqs := rec.requests()
	folded := -1
	for i, req := range reqs {
		for _, m := range req.Messages {
			if strings.Contains(m.Content, "<compaction-summary>") {
				folded = i
			}
			if strings.Contains(m.Content, "<live-compact>") {
				t.Fatalf("request %d carries a live-compact card while the switch is off: %s",
					i, messageDigest(req.Messages))
			}
		}
	}
	if folded < 0 {
		t.Fatalf("the native summary path never folded with the switch off (%d requests): %s",
			len(reqs), messageDigest(reqs[len(reqs)-1].Messages))
	}
	st, ok, err := sessionstore.LoadCompactionState(ctrl.SessionPath())
	if err != nil {
		t.Fatalf("LoadCompactionState: %v", err)
	}
	if ok && st.LastReceipt != nil && st.LastReceipt.Action == "live_compact" {
		t.Fatalf("a live_compact receipt was recorded with the switch off: %+v", st.LastReceipt)
	}
}
