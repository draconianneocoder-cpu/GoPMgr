<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# DF0 Zstd security assessment

Assessment date: 2026-10-31. Source baseline:
`d880056262fc3234a4d137fc575fe29b1be14317`.
The maintainer approved temporary source overrides confined to the isolated
ADR-012 spike on 2026-10-31. The active workspace now selects the patched pair;
the earlier baseline findings below remain historical evidence. This approval
does not cover production integration or waive any advisory.

## Dependency conflict at baseline

The locked graph contains both Zstd 0.13.3 / zstd-safe 7.3.0 and patched Zstd
0.14.3 / zstd-safe 8.2.0. Four parents retain the old pair:

| Parent | Version | Published Zstd requirement |
|---|---|---|
| `datafusion` | 55.2.0 | `0.13` |
| `datafusion-datasource` | 55.2.0 | `0.13` |
| `arrow-ipc` | 59.3.2 | `0.13.0` |
| `parquet` | 59.3.2 | `0.13` (normal and dev dependencies) |

A direct dependency bump cannot satisfy those requirements. Registry inspection
found DataFusion 55.2.0 still on Arrow 59; Arrow IPC and Parquet 60.1.0 use Zstd
0.14, but substituting Arrow 60 is a broader compatibility change. The open
[Arrow maintenance PR #11969](https://github.com/apache/arrow-rs/pull/11969)
targets Arrow 55, not the selected Arrow 59 line. It is not a compatible
published fix for this graph.

## Baseline audit and bounded call-path assessment

`cargo-audit audit --file spikes/adr012-datafusion/rust/Cargo.lock --json`
returned exit 1 on the source baseline: three vulnerabilities and ten
unsoundness warnings. Advisory
database commit: `2295466c4e9df26647fd67d8fa1c705ac2fc7f81`.
No advisory ignores, severity threshold or target filters were used.

Locked metadata enables only `std` in the old safe wrapper and shared native
library; `experimental`, `seekable`, `zstdmt`, `legacy` and `zdict_builder` are
absent. The following is a bounded exposure assessment, not a security waiver.
The adopted graph removes the affected versions; this table explains the
bounded investigation of the earlier baseline, not a waiver for retaining it.

| RustSec ID (2026) | Relevant condition | Evidence for this locked configuration |
|---|---|---|
| 0391 | Abandoned borrowed stream context reused | Synthetic encoder reproduces plaintext carryover. Inspected runtime callers use bulk contexts or a fresh read decoder, not `with_context` |
| 0403 | Corrupt legacy frame, streaming output committed on error | Legacy disabled; inspected readers use bulk or the read decoder, which the advisory excludes |
| 0398 | Seekable decompression beyond end | Seekable disabled |
| 0395 | Stable input buffer retained after borrow | Experimental parameter disabled |
| 0402 | Cloning a multithreaded compression context | Experimental clone and multithreading disabled |
| 0406 | Changing an unfinished multithreaded context | Multithreading disabled |
| 0404 | Cloning a decompression context with owned state | Experimental clone disabled |
| 0400 | Streaming and one-shot calls mixed on one context | Arrow IPC and Parquet bulk contexts do not expose a streaming transition; no mixed-context caller found in inspected runtime paths |
| 0399 | Borrowed dictionary outlives its owner | Affected experimental initialization methods disabled |
| 0401 | Overflow in dictionary-training sample sizes | `zdict_builder` disabled |
| 0405 | Invalid input position in seekable compressor | Seekable disabled |
| 0397 | Output position beyond initialized vector length | No `around_pos` call found in the four parents or Zstd wrapper; inspected stream adapters construct output at position zero |
| 0396 | Uninitialized or invalid seek table | Seekable disabled |

Inspected sources: Arrow IPC `src/compression.rs`, Parquet
`src/compression.rs`, DataFusion datasource `src/file_compression_type.rs`,
Zstd `src/bulk/`, `src/stream/raw.rs`, `src/stream/zio/`, and affected
zstd-safe methods. New callers, feature unification or dependency changes
invalidate this bounded assessment. Version-based audit findings remain even
where the selected features exclude an affected API.

## Disposable candidate

An external temporary workspace copies the baseline Rust workspace and the
four exact registry crates above. Cached crate archives were SHA-256 checked
against the committed lockfile, and every archived source file matched the
extracted registry source before copying. Only the four normalized
`Cargo.toml` files differ from those sources: their normal Zstd constraints,
plus Parquet's dev constraint, become `=0.14.3`. No Rust implementation or
feature declaration changes. Local `[patch.crates-io]` entries select those
copies. This is a local experiment, not an upstream release or maintained fork.

Reproduce from a disposable copy, never by editing the shared Cargo cache:

1. Copy the tracked spike Rust workspace and the four locked registry crate
   sources to an external directory; retain upstream license and notice files.
2. Apply the five version-constraint edits above and four local path overrides.
3. Use the repository's `with-rust-toolchain.sh` launcher for every Cargo call.
   From the copied workspace, run `cargo update --offline -p datafusion
   -p datafusion-datasource -p arrow-ipc -p parquet`, then
   `cargo metadata --locked --offline --format-version 1`.
4. Compare package versions, sources, resolved edges and features with baseline
   metadata. Only the four source substitutions and removal of the old Zstd
   pair are permitted. Audit the resulting lockfile without filters.
5. Run workspace formatting, release-profile all-target compilation, tests,
   build and Clippy; run the synthetic corpus against the newly built CLI.
   Preserve existing parity failures and compare their classifications with
   baseline, rather than requiring every known legacy mismatch to disappear.

The candidate resolution removes only Zstd 0.13.3 and zstd-safe 7.3.0. No other
package version or resolved feature set changes. The patched pair and shared
zstd-sys 2.1.2+zstd.1.5.7 remain. Its unfiltered audit returns exit 0 with zero
vulnerabilities or informational warnings against the same advisory database.
The copied overlay is 377 files, about 8.9 MB, before build output; even
manifest-only maintenance therefore has a meaningful provenance and upkeep
cost if adopted.

The safe synthetic regression abandons an encoder after writing one payload,
then reuses its context for a different payload and asserts the decoded result
contains only the second. Zstd 0.13.3 fails that assertion by prepending the
first payload; 0.14.3 passes. A corresponding interrupted-decoder reuse test
errors on 0.13.3 and passes on 0.14.3. This demonstrates test sensitivity and
the dependency correction; it does not demonstrate application exploitability.
No memory-unsafe exploit was executed.

## Disposable candidate results and limits

- Baseline stub and DuckDB-tagged `internal/analytics` tests pass.
- Candidate `cargo fmt --all -- --check` passes.
- Candidate `cargo check --release --workspace --all-targets --locked
  --offline` passes on macOS arm64 with Rust 1.99.1.
- Candidate audit and synthetic context tests have the results above.
- Candidate `cargo test --release --workspace --locked --offline` passes:
  five engine tests and four worker tests. The CLI and doctest targets have
  no tests; they are not additional test coverage.
- Candidate `cargo build --release --workspace --locked --offline` and
  `cargo clippy --release --workspace --all-targets --locked --offline --
  -D warnings` pass.
- Spike `go test -count=1 ./internal/...` passes its tested packages;
  `df0pb` and Darwin `ipchost` have no test files. Fixture generation with
  `go run ./cmd/df0-fixtures -out CORPUS -check` matches the committed
  94-entry manifest. The optional large performance fixture is listed but
  not written by this invocation.
- Running `cmd/df0-candidates` with the new release CLI produces 48 portfolio,
  69 tabular and four filesystem records. Each equals the prior Rust 1.99.1
  native baseline after removing only `wall_ns`. Both portfolio candidates
  match all 24 legacy-oracle cases. The runner skips the large performance
  fixture; it does not report 70 tabular executions.
- Existing tabular classifications remain: 39 match, 14 match with notes,
  six degenerate, four rejected, three rejected valid inputs, one ambiguous,
  one loss and one silent acceptance. This is unchanged behavior, not complete
  import compatibility or clearance of the recorded DF0 failures.
- Documentation REUSE and release-scope checks passed. At this experimental
  stage, the repository lockfile remained byte-identical to baseline.

Record new results separately using the pinned launcher, from the original
spike directory, with external `CORPUS`, `CANDIDATE` and `RESULTS` paths:

```sh
bash scripts/with-rust-toolchain.sh go run ./cmd/df0-candidates \
  -corpus "$CORPUS" -cli "$CANDIDATE/rust/target/release/df0-cli" -out "$RESULTS"
```

Baseline lock SHA-256:
`682d7c39f6ce319e3fb6aefafa95f965ae2716d15e59c2d4511ae032b9b3d717`.
Candidate lock SHA-256:
`a2cf60dafdb139b39b3d92ce4e93bc35a3567fd80bc91ec0fba625266c938698`.
Source comparison finds only the copied workspace's manifest and lock changed;
all owned Rust code is identical to baseline. Rust 1.99.1 reports commit
`58cf8f9c35b355199727627d07ae34a4e2846d67`, LLVM 23.1.1. Compiler wrappers,
Rust flags, alternate C compiler settings and `ZSTD_SYS_USE_PKG_CONFIG` were
unset. These are local macOS arm64 results, not performance measurements.

An independent plan reviewer was unavailable. Structured self-review checked
graph drift, hidden duplicate vulnerable versions, disabled-format shortcuts,
source provenance, feature unification and the difference between an audit
result and reachable application behavior. This is not independent approval.
Windows/Linux native runs, MSRV compilation, Miri/sanitizers, production
packaging and no-spill qualification have not been performed for this candidate.
Historical DF0 evidence and its FAIL/BLOCKED/NOT-RUN rows remain unchanged.

## Approved adoption and removal

Compatible published fixes remain the durable baseline target. After reviewing
the experiment, the maintainer selected: **adopt temporary spike-only
overrides**, with pinned provenance and removal when compatible published
fixes pass the same checks. Waiting was considered but would leave DF0-C2
blocked. Broad engine upgrades or disabling supported codecs were rejected
as unnecessary compatibility changes.

The four complete published crate copies contain 373 files. Only normalized
Cargo manifests change: five Zstd constraints and modification notices.
Workspace patch entries select them explicitly. A dev-only engine dependency
on the already-selected Zstd 0.14.3 supports the two permanent regressions;
it adds no package or enabled feature. All other resolved dependency edges,
versions and features are preserved except the four redirected Zstd edges
and removal of the old pair.

[GOPMGR.md](../../spikes/adr012-datafusion/third_party/GOPMGR.md) records archive
checksums, licensing, GoPMgr maintainer ownership, byte-preserving Git attributes,
source checks and the removal procedure. Source guards run in `make config-check`
without requiring Rust on the root application's builders. They reject source
drift, missing/wrong patch paths and a reintroduced old Zstd pair. Upstream
licenses and notices remain verbatim. This source assessment does not replace
the full DF0-F SBOM or production distribution review.

## Adopted-workspace validation

Fresh repository-path builds use Rust 1.99.1 on macOS arm64 and an external
target directory. All adoption acceptance criteria pass within this scope:

| Criterion | Evidence |
|---|---|
| Remove the old pair without unrelated graph drift | Locked metadata removes only the two old packages, redirects four parent edges and adds the test-only engine edge; all remaining versions and resolved features match baseline |
| Preserve upstream implementation and provenance | All 373 copied files compared with checksum-verified archives; only the four marked Cargo manifests differ as described |
| Correct interrupted-context reuse | All 11 workspace release tests pass, including the new pair; the exact new test source fails against 0.13.3 and passes against 0.14.3 in disposable probes |
| Preserve observable corpus results | 48 portfolio, 69 tabular and four filesystem records equal the previous native baseline after removing only `wall_ns`; existing failure classifications persist |
| Audit the adopted graph without suppression | Active lock audit returns exit 0, zero vulnerabilities and zero informational warnings against database `2295466c4e9df26647fd67d8fa1c705ac2fc7f81` |
| Prevent source/selection drift | Root guards pass; disposable faults reject old/duplicate Zstd, missing zstd-safe and absent patches. Changed/added/missing/renamed files and symlinks are covered. A disposable index preserves all 373 source blobs with `core.autocrlf=true` |
| Preserve isolation and maintainability | Production Go graph, application behavior, build tags and installer logic unchanged; source ownership, licenses and removal documented; Apache-2.0 added to the repository license inventory |

Commands passed: pinned-launcher `cargo fmt --all -- --check`,
`cargo test --release --workspace --locked --offline`,
`cargo build --release --workspace --locked --offline`,
`cargo clippy --release --workspace --all-targets --locked --offline --
-D warnings`, `go test -race -count=1 ./scripts`, `go vet ./scripts`,
`make config-check`, `make verify`, `make coverage-ledger-drift`,
`make license-check` and `make release-scope`.
The initial `make verify` stopped because the new source guard lacked a
coverage-ledger entry; adding its risk/evidence row fixed the failure and
the gate passed on rerun. No test or gate was weakened.
The existing exclusion scanner also passes on the new source tree: none of
the reviewed excluded archives or eight markers occur. This bounded marker
check is not universal code-absence proof or distribution clearance.

Adopted lock SHA-256:
`4ca38687983045e12c0672568ac62a1559fa5edf806316c2640ab9966ce4dd86`.
Fresh release artifact SHA-256 values:

- CLI: `59314f4ac029663298bd692f10a8a32d1671a7dfbcdaf0b0a0b0d06156c8f87b`.
- Worker: `df8c03653a88135ccb270a439e93b58af21ef022fc2acc976fe2fa709fe0ffe2`.

Plan, mid-work and final reviews used the disclosed structured self-review
fallback; independent review remained unavailable. Findings about Go automatic
vendoring, line-ending conversion, source drift and stale documentation were
addressed. No unresolved material finding remains for this spike-only change.
The four complete upstream test suites were not run: qualification covers the
selected workspace/features and synthetic corpus. Native Linux/Windows, MSRV,
no-spill, packaging, signing and full distribution/SBOM gates remain open;
the local checks do not substitute for them. Production PDF/A/PAdES release
qualification was not repeated because production runtime code and packaging
logic did not change. The new Apache-2.0 text can accompany existing license
bundles; no Rust worker is added to a package. Historical DF0 classifications
remain intact.

Retaining DuckDB as the compatibility reference does not authorize distribution
of excluded benchmark code.

No production Go dependency, build tag, SQLCipher database, installer or DuckDB
implementation changes. Complete DF0-C2/D/E/F/G, native-platform gaps, the
independent LC-GATE and the owner-reviewed go/no-go report before requesting
production integration or DuckDB removal.
