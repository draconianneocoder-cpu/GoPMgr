<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-012 / DF0: DataFusion-for-DuckDB replacement feasibility spike — v0.2

**Status:** ACCEPTED by the maintainer on 2026-10-26 ([acceptance record](../review/ADR-012-v0.2-ACCEPTED-OWNER-DECISIONS.md)); DF0 (DF-OWN-02) is AUTHORIZED, evidence-only. Results: [DF0 evidence](adr-012-df0-evidence.md).  
**Date:** 2026-10-26 · **Revision:** 0.2; original DF-001–DF-060 retained; additional DF-061–DF-065  
**Parent:** [ADR-012](../design/ADR-012-datafusion-analytics-replacement.md)  
**Source baseline:** GoPMgr `main` `28a758d`; ADR-010 accepted target on its P00 branch and G0 transport evidence in PR #141.  
**Scope:** Independent Rust modules/fixtures/benchmark harness and reports. **No production GoPMgr code, `go.mod`, SQLCipher files, release flags, published data, or feature promises.**

## 0. Feasibility question

**Independent release-compliance warning:** PR #146 reports bundled DuckDB TPC-H/TPC-DS code with unresolved distribution licensing. The main production DuckDB build must not be newly distributed under the current maintainer restriction until `LC-GATE` passes, even if it remains a useful controlled baseline. DF0 must not use official TPC benchmark fixtures without separately reviewed rights. See [ADR DF-A01](../design/ADR-012-datafusion-analytics-replacement.md#df-a01--separate-duckdb-release-compliance-from-datafusion-cutover).

Can a bundled Rust Apache DataFusion + Arrow worker, using **real native gRPC over the already-approved local IPC**, replace both currently shipped DuckDB functions with equivalent behavior, bounded security/privacy, reliable lifecycle and acceptable **end-to-end** latency/memory/package cost on every advertised platform?

A result may be **PASS**, **FAIL**, **BLOCKED**, or **NOT-RUN**. An inability to run a check is never PASS. Failure is useful evidence; do not simplify requirements to manufacture a pass.

The spike is not a DataFusion advocacy benchmark. A result showing no advantage—or that a small Go rollup is better—is acceptable and must be reported. The owner decides whether architectural fit outweighs measured regressions.

## 1. Concrete deliverables

1. `docs/testing/adr-012-df0-evidence.md`: commit hashes, measured results, raw PASS/FAIL/BLOCKED and caveats.
2. `docs/interoperability/adr-012-format-parity-matrix.md`: current DuckDB vs candidate semantics, fixtures, disposition of each difference, exact provenance.
3. `docs/interoperability/adr-012-rust-dependency-security-sbom.md`: Cargo metadata, full transitive dependency licensing, security advisories, platform compatibility and pinned versions.
4. `spikes/adr012-datafusion/`: **separate** Rust workspace and minimal Go harness, pinned toolchains and committed `Cargo.lock`/Go module hashes, test fixture generator, replay scripts.
5. `spikes/adr012-datafusion/fixtures/manifest.json`: file SHA-256, source generator command, schema/expected counts, sensitivity label, golden results.
6. `spikes/adr012-datafusion/evidence/`: machine-readable JSONL/CSV test and benchmark records with toolchain/OS/hardware settings; avoid raw credentials, user data or absolute private paths.
7. `docs/review/adr-012-df0-decision-report.md`: independent comparison, alternatives, quantified risks, recommended next milestone and exact unresolved owner decisions.
8. Proof that the previously implemented DuckDB production engine **remains untouched**.

All scratch files created by the spike must be outside the GoPMgr project file directories and be deleted after run. Do not commit real customer/MS Project proprietary schedules. Fixtures must be fabricated, documented, and privacy safe.

## 1A. Non-negotiable three-way evidence and provenance

DF0 measures: **(A) current DuckDB** production-style invocation (test-only reference; release licensing independently blocked when unresolved), **(B) Rust DataFusion handling both current methods**, and **(C) deterministic native Go `PortfolioRollup` plus Rust DataFusion tabular operations**. Compare all three under equivalent user-visible policy, equal inputs and a reference oracle. This is essential even if the agent expects DataFusion to win.

Every datum in the benchmark ledger is labeled `legacy_parity`, `plausible_production`, `future_s7a` or `adversarial_stress`. A 50,000-task schedule is not a 50,000-project portfolio; do not conflate them. Each input receives a generated-on-repository fixture hash, schema/type oracle, licensing source and privacy classification. **Do not use TPC-H/TPC-DS generators, query copies or datasets** as default evidence.

**Red/yellow/green semantics:** FAIL remains FAIL even if a developer knows why; BLOCKED is unexecuted or lacking independent enforcement proof; NOT-RUN is a recorded omission; a subset PASS is never generalized as G0 complete. The maintainer—not the spike—chooses the eventual method-to-provider allocation.

## 2. Minimum controlled environment

- Target candidate: latest **verified stable** DataFusion release at the time of spike; as of 2026-10-26 **DataFusion 55.2.0** is a verified stable patch release (2026-10-07); do not infer a future version without a new registry/release check. Pin the exact `Cargo.lock` commit/semantic version, Arrow, parquet, tonic, tokio, prost and any Windows adapters; check MSRV/target toolchains.
- Baseline: GoPMgr's **existing pinned** `duckdb-go/v2` dependency; use the actual `internal/analytics` implementation. Do not compare DataFusion with a synthetic hand-optimized DuckDB query that ignores current behavior, and do not compare DuckDB via app with DataFusion inside its process without stating the distinction.
- Host: Go executable performing the exact `analytics.Engine` operation over a fresh, authenticated temporary local endpoint. Reuse/prototype ADR-010's gRPC schema and auth/peer rules; do not bootstrap a TCP listener.
- Worker: Rust DataFusion process. It may serve as gRPC client connecting to the Go host, mirroring the accepted host-server/worker-client design. If tonic/Hyper uses a custom connector, document every platform-specific byte-stream adapter. A new ad hoc `localhost:port` bridge is disallowed.
- Targets: GoPMgr-advertised Linux x64, macOS arm64, Windows x64 and any other currently declared platform; extra arm64 evidence may be gathered. Pin actual minimum versions from `docs/INSTALL.md` (do not invent support policies).
- Machines: one owner-approved reference desktop per target + controlled CI. Report CPUs (model/core count), memory, storage, OS version, power mode, cache state and temperature/throttling policy. Missing profiles remain BLOCKED.
- Run each comparator using the same data and parameters and either the same core/thread limit or explicitly documented resource differences.

### 2.1 Proposed spike source layout

```text
spikes/adr012-datafusion/
  README.md
  versions.lock.md
  rust-toolchain.toml
  Cargo.toml
  Cargo.lock
  proto/analytics_spike_v1.proto
  worker/src/main.rs
  worker/src/engine.rs
  worker/src/transport/{unix,windows}.rs
  worker/src/security.rs
  host/go.mod
  host/main.go
  host/duckdb_baseline.go       # outside production package; baseline build tag
  host/transport_{unix,windows}.go
  fixtures/generate.py
  fixtures/manifest.json
  tests/contract/
  tests/security/
  tests/performance/
  scripts/run-all.sh
  scripts/check-results.py
  evidence/                      # synthetic only
```

The layout is illustrative, not authority to copy ADR-010 G0 code into production. Prefer importing stable generated schema from G1 once it has been accepted, rather than creating a second plugin framework.

## 3. Required snapshot of shipped behavior

### 3.1 API/contract census

Capture all `analytics.Engine` calls, Wails bindings, UI responses, error mapping and `duckdb` build-tag consumers on current HEAD. Capture the actual `PortfolioRollup` and `ImportTabular` outputs (including individual cell types, nulls, order, and errors) from the shipped engine on the same fixture corpus.

Required protocol-side data representations:

| Concept | Wire/Arrow representation | Invariant |
|---|---|---|
| Project ID / name | UTF-8 string with size limit | No truncation or normalization without a documented policy |
| Budget/committed/actual/EV/PV | signed 64-bit minor units or checked wider decimal | Exact aggregate; reject overflow |
| Reporting currency | ISO code supplied/verified by host | Never sum incompatible currencies |
| Status/as-of date | ISO date with explicit calendar semantics | Preserve host date checks |
| EVM availability | bool independent of amount | Missing != zero |
| SPI/CPI | host-approved ratio with undefined flag/convention | Sum first, divide once |
| Column metadata | stable typed descriptors, nullability | Do not infer incompatible schema silently |
| Dataset batches | Arrow RecordBatch encoded, chunked, checksum-bound | Bounded frames, total limits, ordered batch accounting |
| Error | typed enum/code, sanitized metadata | No opaque exception text to UI |

The present `Dataset` API is `Columns []string` and `Rows [][]any`; validate types and ordering as observed, not assumed. The currently fixed 10,000-row materialization limit is a **display cap**, not a complete-import claim.

### 3.2 Golden dataset corpus

Generate deterministic, seed-pinned synthetic cases. Each fixture records expected rows, columns, inferred types where stable, SHA-256, and current DuckDB output. If DuckDB behavior differs between OSes, record both and identify why; do not pick whichever result is favorable.

| Fixture family | Cases |
|---|---|
| Portfolio | zero projects; 1, 10, 100, 1k, 10k projects; EVM absent/zero/nonzero; CPI/SPI undefined; conflicting float/minor-unit inputs; negative/very large exact values; overflow; mixed currency; incompatible as-of dates |
| CSV/TSV/TXT | header/no header; quotes; embedded newline/comma/tab; BOM; UTF-8 and non-ASCII; CRLF; duplicate names; empty; single wide row; huge cells; scientific notation; decimal and boolean; nulls; malformed quotes |
| Parquet | Int64/Decimal128/Decimal256, timestamps/timezones, row groups, dictionary encoding, nulls, nested arrays/maps, compression, corrupt footer, unusual metadata |
| JSON/NDJSON | NDJSON lines, single object, JSON array, nested arrays/objects, booleans/nulls, duplicate keys, escaped Unicode, large integers >2^53, invalid syntax, mixed type per column |
| Size/stress | 0, 1, 100, 10k, 10,001, 50k, 100k, 1m rows; very wide 100–1,000 columns; high skew/cardinality; sparse data; repeated reads |
| Filesystem | symlink, reparse point, path traversal, overlong path, unreadable file, mutation during read, corrupt gzip/snappy, denied scratch allocation |
| Security | hidden network connection attempt, SQL with DDL/DML, attempts to use local `file:` URL/table, spill quota bypass, cross-session handles, forged schema and length |

For JSON arrays and other input forms not natively equivalent, document an explicit safe parser/preprocessing adapter; avoid relying on DataFusion `register_json` semantics to magically match DuckDB `read_json_auto`.

## 4. Execution phases and evidence gates

### DF0-A — Baseline freeze (no DataFusion required)

1. Pin current GoPMgr HEAD, `go.mod`, frontend lock, DuckDB build tag and shipped release paths.
2. Enumerate actual analytics callers, feature use, currency/EVM preprocessing and error surfaces.
3. Run existing `go test -tags duckdb ./internal/analytics/...` and default stub tests. Capture test logs, CPU, versions and build failures separately.
4. Generate full golden corpus; run and hash DuckDB expected output. Cross-check numerical results using an **independent Go big-int/int128 oracle** (as appropriate), not just DuckDB versus DataFusion agreeing with each other.
5. Record benchmark functions from code: creation, `PortfolioRollup`, CSV and Parquet reads; include SQL/binding overhead.
6. Block DF0-A if baseline can't be reproduced; do not blame candidate engine for a broken baseline.

### DF0-B — Standalone Rust DataFusion and Go reference correctness

1. Pin latest verified stable DataFusion + Arrow dependency set. Confirm `cargo build --locked --offline` when caches are populated, reproducible release builds and license/security inventory.
2. Implement portfolio aggregation from typed Arrow batches. Use checked signed minor units; verify independent oracle. Do not convert amounts to float before aggregation.
3. Implement CSV/TSV/TXT/Parquet/JSON/NDJSON format readers with explicit options and schema inference limits. Use temporary synthetic files only.
4. Return a versioned typed result, truncation and complete-read statuses, meaningful typed failures.
5. Demonstrate query memory cap, parser chunking and bounded spill policy. Log engine metric counts but not user data.
6. Collect micro-level execution timings, memory and flamegraphs to identify regressions; these are **not** end-to-end victory claims.

### DF0-C — Real Go↔Rust gRPC local-IPC feasibility

1. Build Go host gRPC server with one ephemeral private endpoint per operation and Rust tonic worker as client, adhering to ADR-010 G1 auth/peer checks.
2. Unix: use UDS with private 0700 directory and verified PID. Windows: attempt approved **single overlapped named pipe** first, verify Rust transport `AsyncRead + AsyncWrite` compatibility, HTTP/2 backpressure, shutdown and same-user anti-squat controls. If infeasible, produce a failure report and alternatives; **do not** automatically switch to AF_UNIX/TCP.
3. One-time bootstrap secret, host and worker mutual authentication, single-use proof, per-session expiry and process handle verification. Bootstrap is not business transport.
4. Exercise 1/64/1024+ Arrow batches; per-frame cap ≤4 MiB or approved G1 value; stream sizes up to ≥100,000 records and ≥1 GiB logical total without whole-project memory materialization.
5. Repeat full-duplex import/progress/cancel, slow consumer, blocked output, exit during batch, sequence gap/duplicate, malformed schema and hash mismatch. 50 fresh processes/endpoint cycles, no fd/handle/thread leak.
6. Verify no TCP listeners/sockets on every pass, not simply that HTTP endpoints weren't used in tests. All unavailable OS-level checks are BLOCKED, not PASS.
7. Signed/packaged app and GoPMgr's future G1 production supervisor testing remains a later gate; the spike cannot claim G1 passed.

### DF0-D — Security and privacy

1. Attempt cross-account/project/copy access using forged operation ID, expired nonce, reused credential, remote named pipe, unauthorized second instance and wrong peer PID.
2. Validate host data scope; Rust sees only supplied typed records/bytes, never SQLCipher keys or plaintext database path.
3. Prove malformed Arrow IPC schemas, huge field sizes, compression bombs, dangerous JSON nesting, invalid UTF-8 and big-endian/canonicalization ambiguities are rejected safely.
4. Enforce DataFusion `SQLOptions` to disallow DDL/DML and unneeded statements; keep `enable_url_table` disabled; disable remote object stores and arbitrary local file paths. Rely on OS file restrictions as additional boundary, not solely query SQL options.
5. Measure DataFusion memory-pool allocation and worker RSS independently; enforce scratch size and cleanup after crash. For decrypted project data, no persistent plaintext spill. If safe temporary spill cannot be demonstrated, return `RESOURCE_LIMIT` or equivalent.
6. Exercise networking-disabled offline mode; no runtime plugin/model/extension download. CI must actively measure attempts (do not equate no visible TCP with a system-enforced network sandbox).
7. Use fuzzer and redaction sentinel corpus to prove no sensitive content in command line, environment, diagnostics, errors or normal logs. Verify detector self-test before trusting leak-negative reports.

### DF0-E — Performance and scale

Run both engines using **identical fixtures and verified equivalent semantics**, warm and cold. For a cold Rust worker, include OS launch and credential setup; for warm work, keep the session bounded, documented, and compatible with G1. For DuckDB, record actual `New()`, connection hardening, table create, row bindings, query, scan and result serialization.

Required axes:

| Workload | Input scales | Metrics |
|---|---|---|
| PortfolioRollup | 0/1/10/100/1k/10k/50k projects | cold end-to-end p50/p95/p99, warm end-to-end, CPU, RSS, allocated bytes, transfer bytes |
| File import preview | 10/10k/10,001/50k/100k/1m rows, 10–1,000 cols | open+scan+paged transfer time, peak RSS, truncation/complete labels, bytes read, release memory |
| Wide Parquet/joins | 100k/1m/10m rows where fixtures fit | SQL planner/executor time, predicate/column pruning, spill, throughput, p95 end-to-end |
| Repeat query/snapshot | 10/100 repeated calls | cached metadata, reload overhead, latency variance, stale revision rejection |
| IPC only | 1–1,000 Arrow batches, 64KiB–4MiB chunks | per-batch serialization/deserialization, throughput, backpressure stalls, CPU and allocations |
| Lifecycle | 30–100 cold starts and cancels | startup p50/p95/p99, cancel→exit p50/p95/p99, kill→reap, leaked processes/handles |
| Parallel users | 2/4/8 clients with isolated sessions | fairness, memory budget, crash blast radius, no cross-tenant records |
| Packaging | every release target and test installer | signed binary/JAR size, install footprint, startup, offline viability, virus scanning outcome |

**Statistical policy:** At least 30 timed samples for short latency measurements after warm-up; record every sample, p50/p95/p99 and variance. For expensive multi-minute stress workloads, use fewer fully specified repetitions, raw values and intervals; never derive credible p99 from five samples. Use deterministic fixture order, randomized scenario order between rounds, clean and warm filesystem caches separately, and thermal/power controls. Exclude build time from query latency but report release binary build reproducibility separately. No source code improvements may be counted as query-engine improvements unless applied symmetrically to both comparators.

### DF0-F — Package/release and dependency economics

1. Build Rust targets for Linux x64, macOS arm64 and Windows x64; extend to currently advertised minimum OS versions. Record `cargo tree -e features`, static vs dynamic linkage, toolchain provenance, binary hardening, SBOM and CVE/OSV advisories.
2. Validate the selected Windows named-pipe Rust transport from a test **signed/installed app** when resources exist; leave BLOCKED until actually run. macOS ad-hoc signing is not Developer ID/notarization evidence.
3. Compare bundle size (existing DuckDB native modules versus Rust worker), startup CPU/RSS, time-to-first-portfolio, installer portability and offline update/repair.
4. Confirm no third-party dependency writes outside approved scratch and no runtime network requirement.
5. Prepare an explicit staged rollout + rollback protocol. Do not remove DuckDB in DF0.

### DF0-G — Final evidence review, not automatic approval

Create one comparison report containing scope, pinned dependencies, fixture provenance, test results, confidence levels, exact numerical variances, latency distributions, resource/security failures, supported platform matrix, residual risks, and whether the Rust IPC implementation is suitable for **G1-compatible integration**. Label each item PASS/FAIL/BLOCKED/NOT-RUN. Submit an Architecture Change Request if parity, platform, security, or latency trade-offs conflict with accepted ADR-010 goals. The owner separately decides DF-P03 production integration and later cutover.

## 4A. Additional execution phases required by the v0.2 review

**DF0-A2 — Independently verify legacy amounts and financial truth.** Build a checked big-integer/`i128` reference summation with currency/scale/date rules. Exercise legacy `normaliseProjectMetricsMoney` explicitly: a provided zero minor-unit value and nonzero major-unit float may be ambiguous; record an independent, correct expected amount and a separate legacy DuckDB result. Do not “fix” the old result inside parity fixtures without declaring the discrepancy. Distinguish missing EVM from actual zero and avoid averaging per-project ratios.

**DF0-B2 — Pure-Go reference candidate.** Implement Go aggregation **inside the isolated spike module** without changing `internal/analytics`; run identical current portfolio fixtures, normal/warm/cold cases, overflow and comparison cases. This candidate must be evaluated side-by-side with DuckDB and DataFusion. Report Go implementation complexity and maintenance burden, not just latency.

**DF0-C2 — Host-mediated Parquet seeking.** Implement or design enough of a safe `ReadAt(source_id,offset,length)` transport to measure Parquet footer reads and row-group pruning. Open source once in host, bind to immutable job identity, enforce byte bounds, in-flight request limits, stable source size/version/digest, and expiry. Record `requests`, `requested_bytes`, `actual_bytes`, `physical_bytes`, `seek/read amplification`, and effects on plan/execution. Compare sequential stream against ReadAt and show whether a proposed DataFusion/Parquet `ObjectStore` adapter is technically viable. No global file providers, arbitrary paths or remote URL stores.

**DF0-D2 — No-spill + aggregate host admission.** Use a known sensitive sentinel in input, force memory pressure in spill-capable plans, detect all newly created files at host and OS levels, and prove fail-closed behavior **without plaintext spill**. Inspect post-crash and post-kill scratch. Then run 2, 4 and 8 workers under one host-wide RSS/CPU/job quota; reject work before oversubscription rather than relying solely on query-pool caps. An unavailable no-spill enforcement mechanism marks DF-062 BLOCKED/FAIL—not PASS.

**DF0-E2 — Full pipeline numeric fidelity.** For `int64` greater than 2^53, negatives, Decimal128/256 scale, timestamps, nulls and nested columns, verify expected values after Rust → Arrow → protobuf → Go → Wails-compatible encoding → JavaScript-decoding logic. The spike may test a faithful JS/Wails test harness without changing production UI, but must not claim actual UI integration PASS without exercising the app.

**DF0-F2 — Independent license and release-branch matrix.** Demonstrate no DuckDB `tpch`/`tpcds` executable code in any **new distributed artifact**, or keep its release path BLOCKED pending qualified legal disposition. A synthetic controlled DuckDB reference benchmark cannot satisfy release compliance. Record native symbols/archive analysis separately for Linux, macOS and Windows; reporting strings alone is not adequate proof. Review every Rust crate and benchmark fixture license independently. Licensing gate is `LC-GATE`, not DF-G6.

## 5. Acceptance tests (unique DF-XXX IDs)

### 5.1 Functional and numerical equivalence

| ID | Required observable behavior | Gate |
|---|---|---|
| DF-001 | Existing shipped DuckDB tests reproduced with pinned baseline | DF0-A |
| DF-002 | Empty portfolio yields zero projects and defined neutral outputs | DF0-B |
| DF-003 | EVM available/unavailable counts and filtered totals exact | DF0-B |
| DF-004 | Integer minor-unit sums equal independent reference for normal, negative and large values | DF0-B |
| DF-005 | Sum overflow is typed error, never wraps silently | DF0-B |
| DF-006 | SPI/CPI aggregate ratios and undefined denominator semantics exact | DF0-B |
| DF-007 | Mixed currency rejected by authoritative host before aggregation | DF0-C |
| DF-008 | Float legacy fallback and explicit minor-unit preference documented and reproduced | DF0-B |
| DF-009 | 1st and 2nd portfolio calls do not accumulate old rows | DF0-B |
| DF-010 | Inconsistent reporting date prohibited or surfaced as current host policy | DF0-B |
| DF-011 | CSV/TSV/TXT dialect, UTF-8 and null semantics fixture parity | DF0-B |
| DF-012 | Parquet common scalar, decimal, nested and compression semantics verified | DF0-B |
| DF-013 | JSON/NDJSON object, array and nested/mixed-type cases explicitly accounted for | DF0-B |
| DF-014 | All former accepted filename extensions retained or owner-approved change recorded | DF0-B |
| DF-015 | 10,000-row legacy output bounded and includes accurate `truncated`/`fully_ingested` in new API | DF0-B |
| DF-016 | 10,001+ rows are not falsely described as fully imported | DF0-B |
| DF-017 | Malformed, missing, directory and unauthorized input produce typed errors | DF0-B |
| DF-018 | Stable file/column/row ordering where API promises order; else documented | DF0-B |
| DF-019 | Concurrent calculations are isolated and deterministic under same input | DF0-C |
| DF-020 | No mutation to original input file or SQLCipher project file | DF0-C |

### 5.2 Transport, security and lifecycle

| ID | Required observable behavior | Gate |
|---|---|---|
| DF-021 | Actual Go host ↔ Rust client gRPC over Linux UDS, without TCP | DF0-C |
| DF-022 | Actual Go host ↔ Rust client gRPC over macOS UDS, without TCP | DF0-C |
| DF-023 | Windows named-pipe overlapped gRPC/HTTP2 duplex; no stealth TCP/stdio business protocol | DF0-C |
| DF-024 | Host and worker mutually authenticate per-launch credentials; replay rejected | DF0-C |
| DF-025 | Windows pipe instance ownership, client PID, DACL and anti-squat/impersonation | DF0-C |
| DF-026 | Unix endpoint private mode, peer PID and stale-socket refusal | DF0-C |
| DF-027 | Bounded gRPC message/chunk frames, corrupted sequence and schema rejected | DF0-C |
| DF-028 | Slow-consumer backpressure has bounded allocation and eventual cancellation | DF0-C |
| DF-029 | Crash/hang/kill/OOM/process-tree tests reap without orphan/leaked endpoints | DF0-C |
| DF-030 | Session, principal and project-copy isolation; no cross-session records | DF0-D |
| DF-031 | Worker never receives project DEK/SQLCipher path/privileged callback | DF0-D |
| DF-032 | DDL/DML, URL table and remote object-store access blocked with evidence | DF0-D |
| DF-033 | DataFusion pool, RSS and spill scratch bounded separately | DF0-D |
| DF-034 | Decrypted project data does not persist in unapproved plaintext scratch | DF0-D |
| DF-035 | Explicit network-offline test and detector self-check pass or BLOCKED | DF0-D |
| DF-036 | Sensitive log/argv/env leak detector and redaction sentinel pass | DF0-D |
| DF-037 | No successful connection accepted before peer identity/expected child is known | DF0-D |
| DF-038 | Malicious file and Arrow schema fuzz corpus causes no unsafe panic/resource escape | DF0-D |
| DF-039 | Cancel/timeout produces deterministic status and bounded process termination | DF0-C |
| DF-040 | Protobuf/Arrow schema version incompatibility refused, not silently decoded | DF0-C |

### 5.3 Performance, packaging and governance

| ID | Required observable behavior | Gate |
|---|---|---|
| DF-041 | DuckDB + DataFusion both execute equivalent warm-query suite; raw samples archived | DF0-E |
| DF-042 | Cold end-to-end latency includes worker spawn and bootstrap; no hidden startup omission | DF0-E |
| DF-043 | IPC Arrow serialization/deserialization overhead measured separately | DF0-E |
| DF-044 | 50k and 100k+ representative scans bounded by configured memory policy | DF0-E |
| DF-045 | p50/p95/p99 sampled correctly, hardware/caches/versions stated | DF0-E |
| DF-046 | Linux release candidate builds and works fully offline | DF0-F |
| DF-047 | macOS arm64 release candidate runs from signed and installed app (Dev ID if release claim) | DF0-F |
| DF-048 | Windows x64 release candidate runs with named pipes from installed/signed app | DF0-F |
| DF-049 | Complete Cargo license/SBOM/advisory inventory, no unresolved high-risk runtime dependency | DF0-F |
| DF-050 | Owner-readable CPU/RSS/installer-footprint comparison covers both engines | DF0-E |
| DF-051 | G0 Rust spike never modifies GoPMgr production Go modules, SQLCipher or release paths | DF0-G |
| DF-052 | Any semantic mismatch is preserved in report, with owner decision; no deleted test | DF0-G |
| DF-053 | All supported OS/arch and minimum OS claims have PASS or explicitly BLOCKED labels | DF0-G |
| DF-054 | DataFusion and DuckDB comparison accounts for exact rounding and undefined EVM | DF0-E |
| DF-055 | S0 chart/Gantt/audit measurements are not attributed to DataFusion | DF0-G |
| DF-056 | Worker deployment/update/rollback contract doesn't change project format | DF0-F |
| DF-057 | Production system remains on DuckDB until separate DF-P06/DF-P07 cutover authorization | DF0-G |
| DF-058 | No Iceberg/Ballista/Valkey mandatory dependency introduced | DF0-G |
| DF-059 | No downgrade of ADR-010 transport, plugin capability registry or security invariants | DF0-G |
| DF-060 | GoPMgr main production tests remain unchanged; spike tests isolated and reproducible | DF0-G |

Every test requires: test ID; exact commit; OS/arch; rustc/go/compiler versions; source/fixture hashes; test command; expected result; observed result; PASS/FAIL/BLOCKED/NOT-RUN; recorded artifact path; issue/ACR reference when applicable. For a negative test, document the positive control that proves the detector works.

### 5.4 Additional v0.2 mandatory tests (DF-061–DF-065)

| ID | Requirement | Phase | Independent pass criterion |
|---|---|---|---|
| DF-061 | Legitimate zero minor units vs absent field, negative/overflow, legacy float fallback and new typed-presence semantics | DF0-A2/B2 | Per-case separate legacy result and checked financial oracle; no hidden change to money |
| DF-062 | No plaintext spill with sensitive records under forced memory pressure, cancel, crash, OOM and restart | DF0-D2 | No unapproved temp files or durable plaintext; typed failure; independent OS/file observation |
| DF-063 | Host-mediated `ReadAt` Parquet footer/row-group pruning and stable immutable source identity | DF0-C2 | Range protocol, authorization/expiry, correct results, comparable IO/amplification and no worker path grant |
| DF-064 | Aggregate admission at 2, 4, 8 concurrent Rust workers, including crash/over-quota | DF0-D2 | Host-wide peak RSS/CPU/jobs in configured limits or early typed rejection; clean teardown |
| DF-065 | Exact big integer, Decimal128/256, timestamps, nulls and nested values across Rust/Arrow/protobuf/Go/Wails-JS encoding | DF0-E2 | Bit-/string-exact golden values incl. >2^53; no unapproved JS `number` coercion |

### 5.5 Critical test execution contract and evidence rubric

Every DF-001–DF-065 test produces one machine-readable record per platform/configuration with `test_id`, `commit`, `tool_versions`, `fixture_sha256`, `fixture_class`, `privacy_profile`, `method/provider`, `setup`, `action`, `expected_oracle`, `actual_digest`, `pass_fail_blocked_not_run`, `reason`, `cpu_ms`, `peak_worker_rss`, `peak_host_rss`, `host_aggregate_rss`, `scratch_bytes_created`, `gRPC_bytes`, `worker_spawn_ms`, `wall_ms`, `source_read_bytes`, `signed_package` and `evidence_url_or_path` (redacted). False or missing fields are explicit `null`, never invented zeros. For deterministic P0 tests, compare exact values, not tolerances; floats for display ratios use documented tolerances only.

| ID range | Fixture / injection | Independent oracle | Pass condition |
|---|---|---|---|
| DF-001–DF-010, DF-054, DF-061 | Zero/absent, multi-currency, negative, huge integer, EVM unavailable, duplicate/repeat | Checked Go integer or independently reviewed decimal reference | Every documented legacy-vs-correctness difference classified; exact totals and counts |
| DF-011–DF-018, DF-063 | Generated CSV/TSV/JSON array/NDJSON, nested Parquet/row groups, mutated file, malicious footers | Expected schema, rows and SHA from independent generator and bounded reference decoder | No unreviewed supported-format loss; source identity stable and read amplification recorded |
| DF-021–DF-029, DF-039–DF-040 | Protocol replay, wrong peer, hostile competing pipe, cancel + slow client, malformed batch | Host authorization policy and transport frame specification | No unauthorized row delivered, no TCP fallback, bounded cleanup; all platform rows explicit |
| DF-030–DF-038, DF-062, DF-064 | Sensitive sentinel; forced spill; OOM; subprocess escape; 2/4/8 workers | OS process, file and socket monitors plus host quotas | No plaintext disk or unauthorized network; no over-budget work accepted |
| DF-041–DF-050, DF-065 | Identical 3-provider fixtures and JS large numeric round trips | Controlled cold/warm traces; independent numeric oracle | Both relative and absolute performance gates evaluated; no precision loss |
| DF-046–DF-053, DF-056–DF-060 | Signed packages, rollback, runtime missing, forbidden deps | Installed artifact inventory, go.mod and Cargo.lock comparison | No prohibited shipped binary, no silently lost feature; BLOCKED rows prevent release |

**Sampling:** For inexpensive RPC/rollup operations, run ≥1,000 timed samples across several independent processes and cold/warm phases, with bootstrapped intervals where useful. For costly scans, pre-register feasible repetition counts, publish raw samples/range/max and mark p99 provisional or **not estimable** when n is too small. Randomize alternating candidate order, use fixed machine power/thermal settings, record OS cache state and memory budget. Never publish an unwarranted p99 from 30 observations.

## 6. Proposed performance acceptance policy (OWNER DECISION REQUIRED)

**No pre-spike numerical speedup promise.** As engineering targets to be adjusted *only with owner approval after baseline evidence*:

- Semantic parity is 100% for authorized, currently supported workflows; critical money, EVM, confidentiality and cross-account errors allow **zero** unexplained mismatch.
- **Small warm portfolio rollups:** target no worse than 1.5× DuckDB's p95 at equivalent resource caps, measured end-to-end from API call to result. A slower result is recorded and requires owner approval; it does not automatically disqualify a modular architecture if UX remains acceptable.
- **Cold single-use rollups:** report absolute p50/p95, proportion attributable to process spawn and schema negotiation, and UX implications; no false claim that DataFusion is faster based on excluding startup.
- **Existing bounded tabular preview:** target p95 no worse than 1.25× baseline for the same format/rows and exact semantics, with zero OOM/unsafe spill. This is a *provisional* target, not an accepted gate until endorsed by owner.
- **New analytical workloads:** test absolute latency and memory against proposed S7A metrics, not just ratios to DuckDB that never implemented those capabilities.
- **Memory/installer size:** document changed RSS, cold start, binary/uncompressed installer and disk footprint; limits come from owner hardware profiles (OWN-10).
- **Reliability:** no successful sensitive data leak, unauthorized operation, silent truncation, dropped frame, dangling worker, or unacknowledged corrupted batch in claimed PASS runs.

If DataFusion loses a small-query latency test but wins on packaging and future capability, report the exact loss. Include a scoped, deterministic Go aggregation candidate in the mandatory three-way comparison. Choose its production ownership only through a separate owner decision; never weaken tests or inflate success metrics.

## 7. Measurement schema (required machine-readable JSONL)

One JSON object per run with at least these fields (illustrative, not valid JSON here to avoid generating false results):

`scenario_id`, `test_id`, `commit`, `engine`, `engine_version`, `go_version`, `rust_version`, `os`, `arch`, `cpu_model`, `logical_cores`, `ram_bytes`, `filesystem`, `power_state`, `fixture_id`, `fixture_sha256`, `currency`, `rows`, `columns`, `warm`, `run_index`, `success`, `status`, `elapsed_ns_total`, `elapsed_ns_spawn`, `elapsed_ns_handshake`, `elapsed_ns_prepare`, `elapsed_ns_serialize`, `elapsed_ns_ipc`, `elapsed_ns_engine`, `elapsed_ns_decode`, `rss_peak_bytes`, `mem_pool_peak_bytes`, `spill_peak_bytes`, `bytes_transferred`, `error_code`, `network_check`, `notes`.

Store sample metadata separately from sensitive data; no original project names, user paths, credentials or project content in evidence. Derive percentile summaries reproducibly from raw samples. Include independent oracle hashes and evidence of test detector self-checks.

## 7A. Admission and migration decision matrix

| Decision gate | Question | Must be documented before approval |
|---|---|---|
| `DF0` | Is the Rust worker correct and locally connectable? | All OS-specific PASS/FAIL/BLOCKED; no projected numbers masquerading as results |
| `METHOD` | Who should execute portfolio rollups? | A/B/C report with independent oracles; owner chooses Go-native or DataFusion; no automatic default |
| `SECURITY` | Can DataFusion process sensitive projections without spill/exfiltration? | OS-enforced vs application-enforced guarantees; DF-062/064 PASS on release targets |
| `API` | Does the typed v2 API preserve exact values to UI? | Contracts and DF-065 PASS; legacy Dataset remains bounded |
| `LC-GATE` | May this binary be distributed? | Full license audit; no unresolved TPC-H/TPC-DS embedded implementation without approved resolution |
| `CUTOVER` | Is replacement better and feature-complete? | All existing formats and math, measured UX, signed offline bundles, owner approval |

No source-compatibility refactoring or version bump can make an existing PASS test go away; a new contract requires test migration **in addition** to retained historical parity fixtures.

## 8. Security and rollback exit criteria

**Fail closed:** if Rust IPC/native packaging cannot meet ADR-010 on a required platform, record BLOCKED/FAIL and retain DuckDB. If input format equivalence fails, do not present an inaccurate imported dataset or misstate full ingestion. If decrypted data may be spilled to insecure storage, refuse that job rather than change project-file encryption assumptions. If a worker crashes, do not reuse partial results without explicit input identity/complete-batch hash verification.

The eventual production adapter can use a separately selected backend for test/release experimentation, never as an unnoticed automatic fallback. DuckDB removal needs one further owner authorization after signed release, full tests, rollback rehearsals, and build/link dependency audit. The feasibility spike alone does **not** authorize a cutover or any default engine change.

## 8A. Release compliance is not a rollback exception

Historical DuckDB executables can be used as **controlled reference evidence** only under applicable license/policy constraints; a previously signed artifact is not authorization to redistribute one that contains unresolved TPC generators. If DataFusion fails, either qualify a TPC-free DuckDB build for release, obtain separately approved legal resolution, or hold the affected release. Never ship an analytics-disabled stub as an unapproved workaround. Preserve notice and SBOM enhancements from PR #146 for all remaining dependencies.

## 9. Open questions for owner after DF0

1. What performance penalties in *cold* and *warm* analytics are acceptable given improved modularity and platform isolation?
2. Which minimum/recommended hardware profiles will be supported, and which physical/VM hosts will produce acceptance measurements?
3. Must the Rust worker support Windows ARM64/Linux ARM64 beyond the currently advertised release matrix?
4. What are the permissible scratch/spill media, encryption and cleanup guarantees for decrypted analytical projections?
5. Should tiny portfolio rollups eventually use a deterministic Go fast path if the Rust worker's cold-start latency is unacceptably high?
6. Should schema type-inference differences be reproduced exactly or normalized to a new explicitly documented versioned import API? No silent breaking change.
7. When are full signed Windows installer and macOS notarization test identities available? Missing resources stay BLOCKED.
8. What measured size/security cost justifies packaging a second Rust executable, and should the worker be per-job or short session-scoped?

## 10. Final required report format

**Executive decision:** GO / CONDITIONAL GO / NO-GO / BLOCKED, with reasons and owner decisions outstanding.  
**Evidence:** frozen baseline, fixture corpus/hash, release targets and real IPC architecture.  
**Parities:** numerical/format/security/operational PASS/FAIL/BLOCKED counts, every mismatch.  
**Performance:** side-by-side cold/warm p50/p95/p99, end-to-end vs query-only, CPU/RSS, spill and footprint.  
**Security:** threat model, tests, residual risks, signing and local-IPC claims.  
**Next work:** numbered small PRs with test IDs, rollbacks, permissions and release impact.  
**Change control:** accepted ACRs only; explicitly note none where none exist.

## 11. Version pins and traceability (v0.2)

- Candidate DataFusion: `55.2.0` as known 2026-10-26; Arrow Rust crate versions follow **actual** locked dependency resolution, not hand-guessed exact pairs. Recheck latest security patch and MSRV at execution; pin `Cargo.lock` and document all changes.
- Use ADR-010's Go-owned gRPC endpoint: Linux/macOS UDS; Windows conditionally one overlapped named pipe with one-instance ownership and mutual authentication. Rust `tonic`/Tokio must independently validate real full-duplex HTTP/2 with backpressure and shutdown. No Go–Java pass may substitute.
- Release-compliance check from PR #146 is separate from DataFusion semantic parity and benchmark results. The accepted architecture demands **both**.
- The specific design amendments are recorded in [the v0.2 change audit](../review/adr-012-v0.2-improvements-and-traceability.md). No DF0 results have been measured merely by authoring this document.
