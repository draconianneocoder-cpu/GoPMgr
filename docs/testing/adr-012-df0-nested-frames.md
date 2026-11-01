<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# DF0: nested view frame preparation

This isolated worker correction preserves Arrow schema, selected values, nulls
and ordering while preparing bounded result frames. Production APIs, protocol
versions, dependency pins, SQLCipher and release packaging are unchanged.

## Defects and correction

The previous serializer compacted only top-level Utf8View/BinaryView columns.
Its payload preflight also ignored nested leaves. A literal
Struct → List → Utf8View regression selected one small row between large
unselected rows, but produced a 6,817,160-byte frame and failed the 64 KiB cap.
The same test now passes without changing its selected value or schema.

The correction shares one traversal between payload preflight and compaction
for Struct, List, LargeList, FixedSizeList and Map. Selected child ranges and
rebased offsets pass through checked Arrow constructors. Fixed-size lists and
empty structs retain their explicit row
counts. Struct/fixed-list parent nulls propagate to their children; null
list/map ranges are excluded before concatenating the selected children.
Fields, map ordering, metadata and null masks are preserved. Union, list-view,
dictionary and run-encoded handling remain unchanged and unqualified here.

Adversarial review exposed a second mismatch: the pinned Arrow `gc` copies
populated view descriptors behind null bits, although the worker's preflight
counted only non-null values. A valid all-null fixture with populated backing
views failed with a 164,680-byte single-row frame. The worker now rebuilds view
leaves through Arrow's safe null-aware iterators, copying logical values and
omitting hidden-null payloads from result frames. The unchanged regression fits one frame,
preserves all 128 null rows and excludes its hidden sentinel. No dependency
patch or new unsafe operation was introduced. Final adversarial review added
a parent-null regression: a logically null container retained a 2,883,584-byte
child payload and failed before correction. Mixed valid/null rows now round-trip
in one frame for all five targeted containers, excluding that hidden sentinel.
Logical validity must be applied at every targeted container level, not only
at the leaf.

For these targeted view paths, payload bytes across a proposed frame are
counted before compaction.
Shared-view expansion triggers row splitting first; an oversized single row
returns an error without copying its payload. Frames are still encoded lazily
and checked against the cap including protocol headroom. Offset metadata and
non-view serialization are not a general memory bound; this does not close
RSS, CPU, sandbox, parser-hardening or full confidentiality requirements.

## Evidence and proof limits

The [environment and hashes](../../spikes/adr012-datafusion/evidence/df0-nested-frames-macos/environment.json)
bind source, tools, the newly rebuilt macOS worker and validation results.
Six new Rust tests cover the reproduced nested defect, all five container
classes with string/binary views, nonzero slices, Unicode/binary data,
nulls/empty collections, metadata, zero-width lists/empty structs, ordered
incremental splitting, oversized-row refusal, populated null views and hidden
children behind null containers. These are direct worker serializer tests using
real Arrow IPC writing/reading.

The [native job record](../../spikes/adr012-datafusion/evidence/df0-nested-frames-macos/native.json)
contains five authenticated Go↔Rust jobs: range and stream reads of one selected
nested value, oversized-row refusal for each source mode, and recovery.
Success requires source/completion digests, exact ID/value, normal exit and
endpoint removal; refusal requires `RESOURCE_LIMIT`, zero rows and no result
frames. Sentinels are rejected in result frames and diagnostics.

That Parquet fixture produces **ordinary Utf8**, not Utf8View. Its initial run
also passed the old worker. The test explicitly records/checks that type and
serves as native compatibility/refusal/recovery evidence, not proof that the
new nested-view path executed through DataFusion. The Rust regressions provide
the actual view-defect evidence. Full nested-schema and production Wails/UI
qualification remain open.

Local workspace tests, strict all-target Clippy, formatting, all 14 worker
release tests, the rebuilt release worker, the full native spike Go race suite
and vet passed. The development engine test binary emitted the previously
recorded large `__eh_frame` linker warning about exception-handling performance;
it was not suppressed. Worker release tests/build passed. Existing
source/configuration guards and default/DuckDB analytics regressions passed.
The version-scoped Cargo advisory scan reported zero vulnerabilities and zero
warnings; its database revision is recorded separately from code review.
Linux Go host cross-compilation passed; it is not native Linux execution.
REUSE, test-ledger, release-scope, source/evidence
hashes, documentation links and whitespace checks passed separately.
Controlled faults and their specific failing assertions are recorded in the
environment file; clean/restored controls pass and disposable copies are removed.

Dependencies were cached, not cleanly rebuilt as a complete release. Historical
SBOM/native inventory records remain bound to their earlier worker hash; the
final shipping artifact will need fresh binding and release qualification.
Independent review capacity was unavailable; structured plan/mid/final review
is the disclosed fallback. Native Linux/Windows, minimum-macOS, signed/installed
app, PDF and production UI gates were not run for this isolated correction.
No hosted CI or intermediate PR was started.

## Reproduce locally

From `spikes/adr012-datafusion/rust` with the pinned toolchain already installed:

```sh
cargo test --locked --offline -p df0-worker frames_
cargo test --locked --offline --release -p df0-worker
cargo build --locked --offline --release -p df0-worker
```

From `spikes/adr012-datafusion`, using that rebuilt worker:

```sh
DF0_WORKER=/absolute/path/to/rebuilt/df0-worker \
DF0_NESTED_FRAME_EVIDENCE=/absolute/path/to/native.json \
  go test -race -count=1 -v ./internal/ipchost \
  -run '^TestNativeNestedFrameRecovery$'
```

Without `DF0_WORKER` the native test explicitly skips. The optional evidence
file is written only after all five jobs pass. Rollback is a code revert;
there is no schema migration or production deployment in this slice.
