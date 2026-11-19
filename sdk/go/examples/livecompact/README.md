---
owner: @SivanCola
backup: @esengine
status: active
reviewed: 2026-11-18
---

# Live Compact sidecar

This optional v2 package opts an updated Studio runtime into [turn-end Live Compact](../../../../docs/LIVE_COMPACT.md). It accepts the host's highest-ranked eligible range, receives no transcript bodies, does not install a replacement slot, and requests no other capabilities.

The Studio v2.63.1 Windows installer and portable zip include this package,
with its switch off by default. Use **Settings → Extensions → Plugin packages**
to enable `live-compact` without a manual install.

For other builds, build both binaries from the repository root in Bash and
preview the package:

```bash
go build -o bin/reasonix.exe ./cmd/reasonix
(cd sdk/go && go build -o examples/livecompact/bin/live-compact.exe ./examples/livecompact)
./bin/reasonix.exe plugin install ./sdk/go/examples/livecompact --dry-run
```

The `.exe` suffix is literal; Go still builds for the current OS. The preview
flags this as a full-trust native process; only install binaries you trust.
After reviewing the preview, opt in:

```bash
./bin/reasonix.exe plugin install ./sdk/go/examples/livecompact --yes
```

Quit and reopen Studio. If the kernel predates `compaction.live`, rebuild
and launch from this checkout with `STUDIO_REBUILD=1 bash desktop/electron/run-studio.sh`.

To stop new cuts, run `./bin/reasonix.exe plugin disable live-compact` and
restart Studio. To uninstall, run `./bin/reasonix.exe plugin remove live-compact --yes`.
Both leave existing projections intact. For a full rollback of one session,
quit all processes using it and rename its derived sidecar (example):

```bash
mv -- /path/to/session.context.json /path/to/session.context.json.bak
```

This drops the whole projection, including native folds and its recall ledger.
On reopen the host uses canonical history from the `.jsonl` and WAL; do not
move those files. Keep the backup if you may need the old projection.
