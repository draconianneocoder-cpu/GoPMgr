<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# DF0-C2: host-mediated Parquet reads

This is isolated feasibility evidence for DF-063. It does not authorize a
production cutover or removal of DuckDB. Native macOS arm64 evidence passes the scoped checks below. Linux C2 has not
been run in this slice; Windows remains blocked on its native transport/host.

## Contract

The host opens the selected source once and captures at most 64 MiB into an
owned immutable byte slice. It checks regular-file status, size and modification
time before/after capture. This detects ordinary concurrent modification; it
is not an atomic filesystem snapshot against a writer that conceals metadata
changes. After capture, changes, renames and deletion cannot change the job's
bytes. The SHA-256 of the captured bytes defines its version.

Each job gets a random source ID, operation ID, stable size/version/digest and
expiry. The worker receives no source filename. The host requires exact scope,
monotonic request IDs, subtraction-checked offset/length bounds and limits for
read size, admitted requests, materialized bytes and in-flight leases. A lease
lasts until the transport send returns; timeout stops and joins the sender.
Defaults are experimental: 1 MiB reads (or frame cap minus 1 KiB), 8192 requests,
1 GiB cumulative materialization and one outstanding response per job. These
are not proposed product limits or host-wide admission control.

The worker validates descriptor and each response's identity, request ID,
offset, exact length and SHA-256. It serializes range requests. The normal
query failure/cancellation path aborts and joins its task; protocol shutdown
aborts and joins the outer operation (dropping its query task set also requests
abort). The host reaps the process and removes its private endpoint. Protocol faults fail closed; a fresh job can retry.

A private DataFusion context registers only `gopmgr://job/source.parquet`,
backed by the job's reader. It removes the default `file://` store and registers
no remote provider. The store denies writes, deletes, copies, unknown paths and
stale versions. Metadata inspection does not issue source reads. Queries use
validated column projections and an optional numeric ID interval through the
DataFrame API, without accepting SQL strings. DiskManager is disabled for this
context. That setting is not a complete no-spill or OS sandbox proof: DF0-D/D2
remain separate requirements.

## Measurement and oracle

The comparison fixture has 2048 rows in eight sorted row groups of 256, exact
`id` values starting at 2^53 + 1, integer `value = index*137 - 500000`, and a
1024-byte string column. It is uncompressed and dictionary encoding is off.
Go verifies every returned value, type, projected column, duplicate and count
from the generator formula. The pruned query selects group 3 (zero-based) and
projects only `id,value`. Range-all uses the same projection without filtering;
stream-pruned runs exactly the pruned query after sequential upload.

The test selects a literal filename containing `?`, `[` and `*`, alongside an
unselected decoy. No globbing is needed at the worker boundary.

`source_io` reports admitted requests, refused requests, requested/materialized
bytes, full captured source bytes and peak leases. `actual_bytes` includes a
read materialized before cancellation even if no payload is sent. `source_reads`
records admitted range offsets/lengths. `read_amplification` is materialized
bytes / whole source size, not logical decoded bytes; `physical_amplification`
is 1 because the host captures the full source once. `physical_bytes` counts
source bytes read by the host API, not device traffic or OS page-cache misses.

`capture_ns` includes open, copy, identity checks and SHA-256. `wall_ns` includes
launch, transport, query, Arrow verification and cleanup. `end_to_end_ns` spans capture start through job cleanup, including host query
setup between those intervals. There are no cold-cache controls, repeated latency
samples or CPU measurements here. Do not treat this comparison as DF0-E or
claim disk-read savings: range pruning can reduce RPC bytes while host capture
still reads the whole file.

## Reproduction

From the repository root, with the pinned Rust 1.99.1 already installed:

```sh
DF0_READAT_EVIDENCE=/absolute/path/to/readat.json \
  bash spikes/adr012-datafusion/scripts/run-df0c2.sh
```

Generated bindings require the separately provisioned pinned tools in
`scripts/generate-df0ipc.sh`. They are committed; ordinary builds need none.
The current generation used task-only protoc 25.9, protoc-gen-go 1.36.12 and
protoc-gen-go-grpc 1.6.2; repeating generation produced identical bytes.
The protoc executable came from Maven Central's 3.25.9 macOS aarch64 artifact,
with its published SHA-1 checked over HTTPS. Its locally computed SHA-256 is
`6db6172ef2ab95972707537c9deb26b024594654232f266d0952ec8e922b02b0`.
This is provenance, not independent signature or publisher SHA-256 verification.

## Results (2026-10-31, native macOS arm64)

Raw [job records](../../spikes/adr012-datafusion/evidence/df0-c2-macos/readat.json)
and [build/source identity](../../spikes/adr012-datafusion/evidence/df0-c2-macos/environment.json)
record the tested source hashes, binaries, toolchain and measurements.

| Query | Rows | Source requests | Materialized / RPC payload bytes | Host source bytes |
|---|---:|---:|---:|---:|
| Range, both columns, all groups | 2048 | 9 | 558112 | 2174716 |
| Range, same projection, group 3 | 256 | 2 | 528516 | 2174716 |
| Sequential upload, same group-3 query | 256 | 3 | 2174716 | 2174716 |

The first range request prefetches the final 524288 bytes, including the footer
and some data. The pruned query then reads 4228 bytes from the selected group's
projected columns; the all-groups query makes eight such reads. The trace shows
row-group pruning, but footer prefetch dominates: pruning saves only 29596 bytes
relative to range-all in this fixture. The range/stream comparison demonstrates
technical viability, not a production performance decision.

Validation completed:

- `scripts/run-df0c2.sh`: **18/18 C2 scenarios PASS**, plus **3/3 existing generate,
  duplex and rollup regression scenarios PASS**, against the freshly rebuilt
  release worker. Exact values above 2^53, empty result, invalid projection,
  wrong scope/version/range, replay, quota, corrupt replies, cancellation and
  fresh-job recovery are checked. Endpoints are removed and processes reaped.
- Spike `go test -race -count=1 ./internal/... ./cmd/...` with `DF0_WORKER` set,
  and `go vet ./internal/... ./cmd/...`: **PASS**.
- Rust locked workspace tests in dev and release, fresh release build,
  final worker tests, strict workspace/all-target Clippy and formatting: **PASS**.
- Controlled fault seeds removing job identity validation and the C2 empty-result
  row-count guard each fail their focused test; restoration and affected checks
  pass. Independent response-field corruptions and provider/write refusals pass.
- All **121** previous candidate functional records match, excluding `wall_ns`.
  The 94-case fixture manifest is unchanged. Historical rejected-valid and
  silent-accept outcomes remain recorded; this does not waive parity gaps.
- Locked metadata: **no package versions or resolved features changed**. Four
  existing dependencies become direct engine edges and two direct worker edges.
  Fresh `cargo-audit` reports **zero vulnerabilities and zero warnings**.
- Root `make verify`, `make coverage-ledger-drift`, `make license-check`,
  `make release-scope`, and diff whitespace checks: **PASS**.

Adversarial review used the documented self-review fallback because independent
review capacity was unavailable. Findings fixed: send-timeout credit release,
operation cleanup on stream/protocol failure, same-length identity negatives,
and a false completed count for a zero-batch C2 result. The last fix is scoped
to source jobs; legacy rollup's zero-batch project count retains its contract.
The first native fixture attempt double-closed the Parquet writer's file;
the fixture now writes through a buffer, and all native jobs pass.

Lessons: an open file descriptor is not immutable content; a send queue is not
proof of completed transmission; an empty result still needs count validation;
and Parquet's metadata prefetch must be included when interpreting pruning.

## Remaining boundaries

Production integration, release artifact licensing, host-wide resource admission,
forced no-spill observation, hostile parser/resource tests, Wails/JavaScript
numeric fidelity, repeated comparative performance and native platform packaging
remain unqualified. Windows has no DF0 named-pipe worker implementation or host.
The Darwin `/proc` socket/secret inspector is unimplemented; successful UDS jobs
cannot establish its assertions. No accepted historical FAIL/BLOCKED/NOT-RUN
result is replaced by this narrower evidence.
