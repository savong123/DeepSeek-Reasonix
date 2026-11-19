---
owner: @SivanCola
backup: @esengine
status: active
reviewed: 2026-11-18
---

# Live Compact in Studio

Studio ports the turn-end small cuts of [DSH Live Compact 1.1.12][upstream]. It
keeps native large compaction, canonical transcript storage, and request hooks
unchanged. No `compaction.live` subscriber means no small cuts. Disabling a
subscriber does not undo an installed projection.

[upstream]: https://github.com/Tranquillitatis-Lab/dsh-live-compact/tree/337b21619c65945f828bd1433345566ddf4e4e7e

## Enable or disable

Starting with Studio v2.63.1, the Windows installer and portable zip include
`live-compact` and register it with the switch off on first launch.

- Enable it in **Settings → Extensions → Plugin packages** to use it on eligible turns.
- Turn it off in the same place to stop new cuts. Both choices survive restart.
- Studio preserves a manually installed copy and does not reinstall a removed copy.

Other builds do not bundle it. See the [Go SDK sidecar](../sdk/go/examples/livecompact)
for manual installation or rollback. Turning off or removing the package does
not undo existing cuts; the original transcript remains intact.

## Turn and storage boundaries

1. A foreground executor completes a reply. Failed, refused, unexecuted,
   cancelled, unbound and retargeted turns do not compact.
2. Display and edited records settle. The controller saves canonical history and
   clears the in-flight marker before asking for a proposal. A native checkpoint
   committed during this turn takes precedence; small-cut work skips this turn.
3. The window binds the proposal to the session path, conversation, canonical
   revision and projection generation. It validates the WAL, write authority
   and cancellation state before publishing. Failure changes neither the
   completed reply nor the existing projection.
4. The published body is `prior projection + unchanged gap + card`. Coverage
   ends at the cut, not the transcript end. The normal context window appends
   the untouched canonical tail before sending and after restart.

## Selection

| Rule | Default |
| --- | --- |
| Eligible estimated visible input | At least 20,000 and strictly below 650,000 tokens |
| Local | Strictly after the latest real user message |
| Cloud | Drain at 48,000 or on observed growth predicting over 32,000; stop at 32,000 or no legal forward range |
| Ordinary reserve | Three newest completed short tool batches |
| Preferred batch | Newest eligible complete batch with a `read_file` result of at least 1,000 tokens |
| Ordinary range | At least 1,000 non-user tokens; at most 32,000 total tokens |
| Oversized result | Strictly over 32,000 tokens; success or unknown state only; include the entire call batch |

Cloud cuts may include older user messages, never the latest one. The rebuilt
controller uses the selected model's endpoint; private, loopback and unknown
endpoints (including broker and extension catalogs without URLs) stay local.
Classification does not resolve hostnames or follow redirects.

The canonical covered count is the floor, not the number of stored messages.
A native body may contain a stable head, digest, retained work, and an older
body's remainder. Live Compact leaves all of it unchanged and only considers
later canonical messages. The recent native tail remains eligible once safe.

- Cuts cannot span media, unpaired calls, host-only records or replay items.
- In strict role mode, reject a card that would merge with an unchanged user message.
- Newly frozen gaps drop display-only data.

## Wire and cards

`compaction.live` is an optional v2 intercept, not a replacement slot. It carries
only canonical indices, floor, last user, token estimates, local/cloud flags and
bounded candidate metadata. It sends no transcript text, tool arguments, call
IDs, attachment references or file paths.

The sidecar may `continue` or `replace` with `{ "cut": { "start": N, "end": M } }`
using zero-based, end-exclusive bounds. The host only accepts a range it offered.
Errors, blocks, timeouts and malformed proposals do not fail the turn. A cloud
drain can request a fresh single-range proposal after an accepted cut.

The host builds a `<live-compact>` card with role-labelled JSON excerpts and
zero-based canonical `#n` recall positions. These are historical data, not new
instructions or authorization. Display-only raw content never becomes an
excerpt. The card does not extract tool arguments, paths or media refs.

- Short tool outputs (up to 12 lines and 512 characters) keep their visible
  text. Other eligible outputs keep a bounded tail or a pointer. Existing
  external-content envelopes are pointer-only; external failures stay verbatim.
- Failures are recognized through host metadata. If a complete diagnostic and
  call identity do not fit, or completeness cannot be established, the original
  batch stays. Tool calls and results are never split.
- Canonical history is the backup. Small cuts carry spent `recall` budget
  forward; native folds start a new budget as before. A single oversized
  message may not fit, so a pointer is not a promise of unlimited replay.

## Scope and evidence

There is no DSH shadow, capture worker, Cordis setup, native retuning or
Thunder call.

Each cut emits a receipt; the projection sidecar keeps only the last one.
Its token estimate is not a billing, cache-hit or DSH rewrite count.
