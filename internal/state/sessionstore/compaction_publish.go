package sessionstore

import (
	"context"
)

// SaveDurableCompactionState publishes a derived view only for the exact durable WAL revision.
func (s *Session) SaveDurableCompactionState(ctx context.Context, path string, st CompactionState, expected PersistedState, version uint64, allow func() bool) error {
	return s.withSessionSaveLocks(path, func() error {
		if err := verifyCompactionOnDisk(ctx, path, expected, st.Projection.ProjectionVersion-1); err != nil {
			return err
		}
		return s.publishDurableCompaction(ctx, path, st, expected, version, allow)
	})
}

func verifyCompactionOnDisk(ctx context.Context, path string, expected PersistedState, projection uint64) error {
	meta, ok, err := LoadBranchMeta(path)
	if err != nil {
		return err
	}
	if !ok || meta.InFlightTurn != nil || meta.Revision != expected.Revision || meta.ContentDigest != expected.DigestHex {
		return ErrSessionSnapshotConflict
	}
	old, ok, err := LoadCompactionState(path)
	if err != nil {
		return err
	}
	if ok && (old.Foreign() || old.Projection.ProjectionVersion != projection) || !ok && projection != 0 {
		return ErrSessionSnapshotConflict
	}
	limits := defaultSessionReplayLimits
	limits.ctx = ctx
	msgs, _, damaged, err := loadSessionMessagesWithLimits(path, limits)
	if err != nil {
		return err
	}
	digest, _, _, err := digestSizeAndPrefix(ctx, msgs, -1)
	if err != nil {
		return err
	}
	if damaged || digest != expected.Digest {
		return ErrSessionSnapshotConflict
	}
	return nil
}

func (s *Session) publishDurableCompaction(ctx context.Context, path string, st CompactionState, expected PersistedState, version uint64, allow func() bool) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.durableCompactionMatches(path, expected, version) {
		return ErrSessionSnapshotConflict
	}
	auth := s.writeAuth
	var unlock func()
	defer func() {
		if unlock != nil {
			unlock()
		}
	}()
	return saveCompactionState(path, st, 0o600, func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if allow != nil && !allow() {
			return context.Canceled
		}
		if s.authRequired && unlock == nil {
			var err error
			unlock, err = auth.lockForPublication(path)
			return err
		}
		return nil
	})
}

func (s *Session) durableCompactionMatches(path string, expected PersistedState, version uint64) bool {
	p := s.persisted
	return expected.RevisionKnown && expected.UnchangedSincePersisted &&
		p.ok && p.saveVerified && p.revisionKnown && p.path == canonicalSessionSavePath(path) && p.revision == expected.Revision &&
		p.digest == expected.Digest && p.version == version && s.version == version && !s.normalizedDirty && !s.eventLogDamaged
}
