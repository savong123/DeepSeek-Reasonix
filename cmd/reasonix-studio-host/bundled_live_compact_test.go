package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"reasonix/internal/base/testenv"
	"reasonix/internal/ext/pluginpkg"
)

const testLiveCompactName = "live-compact"

func liveCompactFixtureDigest(t *testing.T) string {
	t.Helper()
	manifest, err := os.ReadFile(filepath.Join("..", "..", "sdk", "go", "examples", "livecompact", pluginpkg.NativeManifest))
	if err != nil {
		t.Fatal(err)
	}
	files := []struct{ Path, Hash string }{
		{"bin/live-compact.exe", digestFixtureBytes([]byte("packaged runtime"))},
		{pluginpkg.NativeManifest, digestFixtureBytes(manifest)},
	}
	body, err := json.Marshal(files)
	if err != nil {
		t.Fatal(err)
	}
	return "sha256:" + digestFixtureBytes(body)
}

func digestFixtureBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func provisionFixture(t *testing.T, home, hostExe string) error {
	t.Helper()
	return provisionBundledLiveCompact(home, hostExe, liveCompactFixtureDigest(t))
}

func liveCompactBundleFixture(t *testing.T) (hostExe, bundle string) {
	t.Helper()
	root := filepath.Join(testenv.TempDir(t), "resources")
	return filepath.Join(root, "bin", "reasonix-studio-host.exe"), filepath.Join(root, "bundled-plugins", testLiveCompactName)
}

func writeLiveCompactManifest(t *testing.T, bundle string) {
	t.Helper()
	manifest, err := os.ReadFile(filepath.Join("..", "..", "sdk", "go", "examples", "livecompact", pluginpkg.NativeManifest))
	if err != nil {
		t.Fatal(err)
	}
	writeBundleFixture(t, filepath.Join(bundle, pluginpkg.NativeManifest), manifest)
}

func writeBundleFixture(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBundledLiveCompactLifecycle(t *testing.T) {
	home := testenv.TempDir(t)
	hostExe, bundle := liveCompactBundleFixture(t)
	writeLiveCompactManifest(t, bundle)
	writeBundleFixture(t, filepath.Join(bundle, "bin", "live-compact.exe"), []byte("packaged runtime"))
	provision := func() {
		t.Helper()
		if err := provisionFixture(t, home, hostExe); err != nil {
			t.Fatal(err)
		}
	}
	provision()
	initial, ok, err := pluginpkg.FindInstalled(home, testLiveCompactName)
	if err != nil || !ok || initial.Enabled || filepath.IsAbs(initial.Root) {
		t.Fatalf("first launch should register a disabled managed package: %+v, found=%v, err=%v", initial, ok, err)
	}
	installedBin := filepath.Join(pluginpkg.ResolveRoot(home, initial.Root), "bin", "live-compact.exe")
	if got, err := os.ReadFile(installedBin); err != nil || string(got) != "packaged runtime" {
		t.Fatalf("installed payload: %q, %v", got, err)
	}
	for _, enabled := range []bool{false, true, false} {
		if err := pluginpkg.SetEnabled(home, testLiveCompactName, enabled); err != nil {
			t.Fatal(err)
		}
		provision()
		want := initial
		want.Enabled = enabled
		if got, found, err := pluginpkg.FindInstalled(home, testLiveCompactName); err != nil || !found || got != want {
			t.Fatalf("relaunch with enabled=%v changed the registry entry: %+v, found=%v, err=%v", enabled, got, found, err)
		}
		if active, warnings := pluginpkg.LoadInstalled(home); (len(active) == 1) != enabled || len(warnings) > 0 {
			t.Fatalf("runtime inventory with enabled=%v: %d packages, %v", enabled, len(active), warnings)
		}
	}
	if _, found, err := pluginpkg.Remove(home, testLiveCompactName); err != nil || !found {
		t.Fatalf("remove: found=%v, err=%v", found, err)
	}
	if err := os.RemoveAll(filepath.Dir(filepath.Dir(installedBin))); err != nil {
		t.Fatal(err)
	}
	provision()
	if got, found, err := pluginpkg.FindInstalled(home, testLiveCompactName); err != nil || found {
		t.Fatalf("removed package reappeared: %+v, found=%v, err=%v", got, found, err)
	}
}

func TestBundledLiveCompactPreservesManualInstall(t *testing.T) {
	home := testenv.TempDir(t)
	hostExe, bundle := liveCompactBundleFixture(t)
	writeLiveCompactManifest(t, bundle)
	writeBundleFixture(t, filepath.Join(bundle, "bin", "live-compact.exe"), []byte("packaged runtime"))
	manual := pluginpkg.InstalledPlugin{Name: testLiveCompactName, Source: "manually installed", Root: testenv.TempDir(t), Version: "99.0.0", Enabled: true}
	if err := pluginpkg.Upsert(home, manual); err != nil {
		t.Fatal(err)
	}
	manual.Generation = 1
	if err := provisionFixture(t, home, hostExe); err != nil {
		t.Fatal(err)
	}
	if got, found, err := pluginpkg.FindInstalled(home, testLiveCompactName); err != nil || !found || got != manual {
		t.Fatalf("manual install was replaced: %+v, found=%v, err=%v", got, found, err)
	}
	if _, found, err := pluginpkg.Remove(home, testLiveCompactName); err != nil || !found {
		t.Fatalf("remove: found=%v, err=%v", found, err)
	}
	if err := provisionFixture(t, home, hostExe); err != nil {
		t.Fatal(err)
	}
	if _, found, err := pluginpkg.FindInstalled(home, testLiveCompactName); err != nil || found {
		t.Fatalf("removed manual package was replaced by the bundle: found=%v, err=%v", found, err)
	}
}

func TestBundledLiveCompactRetriesAfterIncompleteBundle(t *testing.T) {
	home := testenv.TempDir(t)
	hostExe, bundle := liveCompactBundleFixture(t)
	if err := provisionFixture(t, home, hostExe); err != nil {
		t.Fatalf("unbundled host: %v", err)
	}
	writeLiveCompactManifest(t, bundle)
	if err := provisionFixture(t, home, hostExe); err == nil {
		t.Fatal("a manifest without the runtime was accepted")
	}
	if _, err := os.Stat(pluginpkg.StatePath(home)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial bundle wrote registry state: %v", err)
	}
	writeBundleFixture(t, filepath.Join(bundle, "bin", "live-compact.exe"), []byte("packaged runtime"))
	if err := provisionFixture(t, home, hostExe); err != nil {
		t.Fatal(err)
	}
	if got, found, err := pluginpkg.FindInstalled(home, testLiveCompactName); err != nil || !found || got.Enabled {
		t.Fatalf("corrected bundle not installed as disabled: %+v, found=%v, err=%v", got, found, err)
	}
}

func TestBundledLiveCompactRetriesAfterCopyFailure(t *testing.T) {
	home := testenv.TempDir(t)
	hostExe, bundle := liveCompactBundleFixture(t)
	writeLiveCompactManifest(t, bundle)
	writeBundleFixture(t, filepath.Join(bundle, "bin", "live-compact.exe"), []byte("packaged runtime"))
	blocked := pluginpkg.PluginsDir(home)
	writeBundleFixture(t, blocked, []byte("not a directory"))
	if err := provisionFixture(t, home, hostExe); err == nil {
		t.Fatal("failed staging did not report an error")
	}
	if _, err := os.Stat(pluginpkg.StatePath(home)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed staging wrote registry state: %v", err)
	}
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
	if err := provisionFixture(t, home, hostExe); err != nil {
		t.Fatal(err)
	}
	if got, found, err := pluginpkg.FindInstalled(home, testLiveCompactName); err != nil || !found || got.Enabled {
		t.Fatalf("retry did not register a disabled package: %+v, found=%v, err=%v", got, found, err)
	}
}

func TestBundledLiveCompactRejectsUnpinnedPayload(t *testing.T) {
	for _, file := range []string{"bin/live-compact.exe", pluginpkg.NativeManifest} {
		t.Run(file, func(t *testing.T) {
			home := testenv.TempDir(t)
			hostExe, bundle := liveCompactBundleFixture(t)
			writeLiveCompactManifest(t, bundle)
			writeBundleFixture(t, filepath.Join(bundle, "bin", "live-compact.exe"), []byte("packaged runtime"))
			writeBundleFixture(t, filepath.Join(bundle, filepath.FromSlash(file)), []byte("changed after build"))
			if err := provisionFixture(t, home, hostExe); err == nil {
				t.Fatal("changed payload accepted")
			}
			if _, err := os.Stat(pluginpkg.StatePath(home)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("changed payload wrote registry state: %v", err)
			}
		})
	}
}

func TestBundledLiveCompactDoesNotReadWorkspace(t *testing.T) {
	home, workspace := testenv.TempDir(t), testenv.TempDir(t)
	hostExe, _ := liveCompactBundleFixture(t)
	digest := liveCompactFixtureDigest(t)
	t.Chdir(workspace)
	bundle := filepath.Join(workspace, "bundled-plugins", testLiveCompactName)
	writeBundleFixture(t, filepath.Join(bundle, pluginpkg.NativeManifest), []byte("must not read"))
	if err := provisionBundledLiveCompact(home, hostExe, digest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(pluginpkg.StatePath(home)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("workspace bundle was used: %v", err)
	}
}
