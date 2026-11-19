package sessionstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"reasonix/internal/base/fileutil"
	"reasonix/internal/base/testenv"
	"reasonix/internal/contract/provider"
)

func TestLiveCompactPublicationKeepsNativeFileMode(t *testing.T) {
	path := filepath.Join(testenv.TempDir(t), "session.jsonl")
	s := NewSession("sys")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "task"})
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	base, ok := s.PersistedState(path)
	if !ok {
		t.Fatal("no durable baseline")
	}
	state := CompactionState{Projection: ContextProjection{ProjectionVersion: 1}}
	if err := s.SaveCompactionState(path, state); err != nil {
		t.Fatal(err)
	}
	checkCompactionMode(t, path, 0o644)
	state.Projection.ProjectionVersion++
	if err := s.SaveDurableCompactionState(t.Context(), path, state, base, s.TranscriptVersion(), nil); err != nil {
		t.Fatal(err)
	}
	checkCompactionMode(t, path, 0o600)
}

func checkCompactionMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(ContextStatePath(path))
	if err != nil {
		t.Fatal(err)
	}
	// Windows maps both modes to a writable file, so only assert exact mode elsewhere.
	if got := info.Mode().Perm(); runtime.GOOS != "windows" && got != want {
		t.Fatalf("compaction sidecar mode = %#o, want %#o", got, want)
	}
}

func TestLiveCompactPublicationRefusesStaleDiskAndCancellation(t *testing.T) {
	for _, arm := range []string{"cancel-before-rename", "new-wal-old-ledger", "unsupported-projection", "revoked-generation"} {
		t.Run(arm, func(t *testing.T) {
			path := filepath.Join(testenv.TempDir(t), "session.jsonl")
			s := NewSession("sys")
			s.Add(provider.Message{Role: provider.RoleUser, Content: "task"})
			lease, err := TryAcquireSessionLease(path)
			if err != nil {
				t.Fatal(err)
			}
			defer lease.Release()
			auth, err := lease.IssueWriteAuthority(NextSessionWriteGeneration())
			if err != nil {
				t.Fatal(err)
			}
			s.BindWriteAuthority(auth)
			if err := s.Save(path); err != nil {
				t.Fatal(err)
			}
			baseline, ok := s.PersistedState(path)
			if !ok {
				t.Fatal("no baseline")
			}
			old := CompactionState{Projection: ContextProjection{ProjectionVersion: 1}}
			if err := s.SaveCompactionState(path, old); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var expected error
			switch arm {
			case "cancel-before-rename":
				expected = context.Canceled
				fileutil.CrashPoint = func(op, target string) {
					if op == "atomic-write" && target == ContextStatePath(path) {
						cancel()
					}
				}
				defer func() { fileutil.CrashPoint = nil }()
			case "new-wal-old-ledger":
				expected = ErrSessionSnapshotConflict
				added := []provider.Message{{Role: provider.RoleAssistant, Content: "a later answer"}}
				digest, err := DigestSessionMessages(append(s.Snapshot(), added...))
				if err != nil {
					t.Fatal(err)
				}
				if err := appendSessionAppendEvent(path, 2, added, digest, baseline.Revision); err != nil {
					t.Fatal(err)
				}
			case "unsupported-projection":
				expected = ErrContextSchemaUnsupported
				if err := os.WriteFile(ContextStatePath(path), []byte(`{"schema_version":999}`), 0o600); err != nil {
					t.Fatal(err)
				}
			case "revoked-generation":
				expected = ErrSessionWriteAuthorityStale
				if _, err := lease.IssueWriteAuthority(NextSessionWriteGeneration()); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(ContextStatePath(path))
			if err != nil {
				t.Fatal(err)
			}
			next := CompactionState{Projection: ContextProjection{ProjectionVersion: 2}}
			err = s.SaveDurableCompactionState(ctx, path, next, baseline, s.TranscriptVersion(), nil)
			if !errors.Is(err, expected) {
				t.Fatalf("publish: %v, want %v", err, expected)
			}
			after, err := os.ReadFile(ContextStatePath(path))
			if err != nil || string(before) != string(after) {
				t.Fatalf("old projection not preserved: %v", err)
			}
		})
	}
}
