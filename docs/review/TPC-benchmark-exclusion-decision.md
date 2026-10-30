<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# TPC-H and TPC-DS exclusion: decision, evidence, and options

**Status:** exclusion decision recorded 2026-10-26; qualification corrections
recorded 2026-10-29. DuckDB retirement remains subject to the accepted ADR-012
feasibility and owner gates. This record does not authorize distribution.
**Scope:** every GoPMgr production binary, installer, and source package, on
all platforms.
**Not legal advice.** This record states technical facts. It does not say
whether any past distribution infringed a license, and it does not say that
any licensing question is resolved. Those calls need qualified legal review.

## 1. Maintainer decision (2026-10-26, on PR #146)

Quoted:

> Decision: REMOVE FROM DISTRIBUTED BINARIES. I do not approve bundling
> TPC-H or TPC-DS benchmark implementations with GoPMgr's production
> executables or installers under the current unresolved licensing
> circumstances. This decision applies to all supported platforms and
> distribution formats.

> No new production artifact containing the unresolved TPC implementations
> is authorized for distribution unless a qualified legal review establishes
> an acceptable basis and I separately approve that exception.

> No architectural downgrades, weakened tests, or silent feature removals
> are authorized.

Summarized, the decision also requires the following.

- Keep PR #146's notice improvements and correct its licensing conclusions.
- Remove the code from the linked binaries; disabling, hiding, or deleting
  notices is not enough.
- Investigate a clean DuckDB build without either extension that keeps CSV,
  TSV, JSON, NDJSON, Parquet, portfolio analytics, and offline use.
- No unreviewed custom DuckDB fork. A source-built binding needs a
  maintenance, security, licensing, reproducibility, and packaging
  assessment.
- Prioritize the Apache DataFusion replacement and estimate the cost of
  accelerating it. Do not remove DuckDB before the replacement reaches
  parity.
- License compliance is release-blocking. No new production artifact
  containing the unresolved TPC code may be distributed unless a qualified
  legal review finds an acceptable basis and the maintainer separately
  approves.
- Prove removal from release binaries, not only from source configuration:
  check symbols, linked objects, archive contents, and representative
  binaries for each platform. Show that the `tpch`, `tpcds`, `dbgen`, and
  `dsdgen` implementations are absent, and tell incidental strings apart
  from executable code.
- Keep the full licensing inventory, generate an SBOM, and keep the notice
  and version-change gates.
- Do not silently remove analytics features. A limited build needs explicit
  labeling and separate approval.
- Review previously distributed versions and write a factual remediation
  assessment.

## 2. Finding: separate archives with no detected references in reviewed inputs

PR #146 initially described the TPC extensions as linked on every platform.
Their presence on the link line does not establish that their object members
are linked. The reviewed module inventories, symbol audit and marker scans
provide the bounded evidence below. They do not prove arbitrary code absence,
cover changed inputs, or settle the licensing question.

### 2.1 Link mechanism under review

- duckdb-go-bindings v0.10506.0 is built from DuckDB v1.5.6, commit
  `069cc9f9b5be802405797faecc284961b07c70ef`. That commit's
  `.github/config/bundled_extensions.cmake` declares
  `duckdb_extension_load(tpcds DONT_LINK)` and
  `duckdb_extension_load(tpch DONT_LINK)`.
- The generated extension loader in the prebuilt libraries references only
  the Autocomplete, CoreFunctions, ICU, JSON, and Parquet extensions. This
  was checked on all three release platforms, in v0.10505.0 and v0.10506.0.
- Each platform module ships the two extensions as separate archives,
  `libtpch_extension.a` (1.1–1.2 MB) and `libtpcds_extension.a` (2.5–2.7 MB),
  and names them on its cgo link line. A static linker takes a member from an
  archive only to resolve a symbol that is still undefined. The audit found no
  references to their externally defined symbols in the reviewed inputs.
  Forced-loading flags would invalidate this reasoning; the gate rejects
  unsupported flags and changed module inventories.

### 2.2 Archive audit, per platform

`tools/duckdb-notices -check` reads every archive on each platform module's
link line (ELF, Mach-O, and COFF object members). It first verifies the complete
file inventories of both cgo modules and all five platform modules against
reviewed SHA-256 pins, then requires readable objects and symbol tables.
For these fixed inputs it checks three things:

1. No symbol defined in the excluded archives is left undefined by any other
   linked archive, or called by the Go bindings' cgo code.
2. All eight markers appear in the excluded archives.
3. No marker appears in any other linked archive.

These checks support the normal static-link mechanism described above. They
are not a general linker proof or a substitute for checking each built artifact.
The 2026-10-29 local rerun reproduced these five rows:

| Module (v0.10506.0) | Global symbols the TPC archives define | Referenced elsewhere | Markers in TPC archives | Markers elsewhere |
|---|---|---|---|---|
| darwin-amd64 | 649 | 0 | 8 of 8 | 0 |
| darwin-arm64 | 647 | 0 | 8 of 8 | 0 |
| linux-amd64 | 650 | 0 | 8 of 8 | 0 |
| linux-arm64 | 650 | 0 | 8 of 8 | 0 |
| windows-amd64 | 713 | 0 | 8 of 8 | 0 |

The original PR #146 review reported an audit against v0.10505.0 (darwin-arm64, linux-amd64,
windows-amd64), the version 1.1.0-alpha.2 shipped with, gave the same
result: 647, 650, and 713 symbols, none referenced, and no problems. The
cgo check covered 632 C names used by duckdb-go-bindings and duckdb-go/v2.

### 2.3 Markers and incidental strings

`third_party/EXCLUDED-NATIVE-CODE.txt` lists eight reviewed markers. The
original review reported all eight in the excluded archives used as positive
controls and zero listed-marker matches in the release binaries shown in §2.4.
Those historical binary measurements were not independently repeated during
this qualification. The markers are:

- `N6duckdb13TpchExtensionE`
- `N6duckdb14TpcdsExtensionE`
- `N6duckdb17DBGenFunctionDataE`
- `N6duckdb18DSDGenFunctionDataE`
- "dbgen is only supported for DuckDB database files"
- "dsdgen is only supported for DuckDB database files"
- "Transaction Processing Performance Council (TPC)"
- `s_catalog_order_lineitem`

The type names are run-time type information for the extension classes and
the `dbgen`/`dsdgen` function data. The messages are what those functions
raise.

Some strings are **incidental**. DuckDB core keeps an extension-autoload name
table (`src/include/duckdb/main/extension_entries.hpp`), with entries such as
`{"dbgen", "tpch", ...}`, `{"dsdgen", "tpcds", ...}`, and `"tpch_queries"`.
The reviewed binaries contain bare names such as `dbgen` and `dsdgen`. That table is data in the core library: it names extensions
DuckDB could load, and it holds none of their code. It is deliberately not
a marker.

### 2.4 Binaries audited

The following table preserves the original PR #146 reviewer's reported
measurements. These historical artifacts were not downloaded and independently
re-audited during the 2026-10-29 qualification. Their marker results are not a
legal conclusion or universal absence proof.

| Binary | Size (bytes) | DuckDB present (Parquet and JSON type info) | TPC markers | Notes |
|---|---|---|---|---|
| 1.1.0-alpha.2 `.deb`, `/usr/bin/gopmgr` | 93,251,424 | yes | 0 of 8 | `gopmgr-1.1.0-alpha.2-amd64.deb`, SHA-256 `fa1f290c8cd6259e91dee340ca0d7a1b2b123a457065e4fe48ab0252d466f9a3` |
| 1.1.0-alpha.2 `.rpm`, `/usr/bin/gopmgr` | 93,251,424 | yes | 0 of 8 | `gopmgr-1.1.0-alpha.2-x86_64.rpm`, SHA-256 `9c2de38867e3edf5b37f1204520e05c42531d76c4f67b0853b9edb30e13e7881`; the binary is byte-identical to the `.deb`'s |
| 1.1.0-alpha.2 `.dmg`, `GoPMgr.app/Contents/MacOS/gopmgr` | 82,778,112 | yes | 0 of 8 | `GoPMgr-1.1.0-alpha.2-arm64.dmg`, SHA-256 `c4a843bd8e72e27ec41bc20f8aa5f9d0186e957081a570ad2bc5424251902ca8` |
| 1.1.0-alpha.2 Windows installer, `gopmgr.exe` | 75,446,784 | yes | 0 of 8 | `GoPMgr-1.1.0-alpha.2-amd64-setup.exe`, SHA-256 `a4156473884af29526266b7aed98f845ef665e12b9d7418ee666a49ab439ef76` |
| Original PR #146 branch, Linux amd64, symbols kept | 151,996,608 | yes (146,101 symbols) | 0 of 8 | 0 symbols in `tpch::` or `tpcds::`, and none of the extension or `dbgen`/`dsdgen` function classes |
| Positive control: `libtpch_extension.a` and `libtpcds_extension.a` | | n/a | 8 of 8, on every platform and both versions | `scripts/verify-excluded-native-code.sh` fails on them |

The original reviewer reported the incidental string `dbgen` in these release
binaries, from the autoload table described above.

**Not audited:**

- Original-review macOS and Windows builds. The original reviewer reported
  no native build hosts available. The archive-level
  evidence in §2.2 is separate from native artifact evidence. Packaging checks
  must pass on the actual candidate; they do not establish past execution.
- CI build artifacts. They expire after seven days (`retention-days: 7` in
  `ci.yml` and `release.yml`).

## 3. Enforcement added (release-blocking)

| Gate | Where it runs | What fails it |
|---|---|---|
| `third_party/EXCLUDED-NATIVE-CODE.txt` | the single source for every check | `TestRepositoryExclusions` fails if either archive or the extension markers are dropped |
| `make duckdb-notices-check` (`tools/duckdb-notices -check`) | `make verify`, run in CI | changed module inventories, unsupported link flags, missing archives or symbol evidence, excluded-symbol references, or marker mismatches; a notice file whose full digest differs from the reviewed artifact |
| `scripts/verify-excluded-native-code.sh <binary>` | `scripts/verify-module-notices.sh`, which every packaging path runs: the tarball, `.deb` and `.rpm`, the macOS app and installer, the Windows release step, and `debian/rules` | a marker match or a scan/traversal/tool error |
| `scripts/verify-excluded-native-code.sh --tree vendor` | `debian/rules`, before building | an excluded archive, or any file holding a marker, in the vendored tree |
| `make excluded-native-code-tests` | `make verify` | synthetic failures and malformed inputs, including the real TPC archives as positive controls |

Two consequences need the maintainer's attention.

- **The Debian source build is blocked by design.** `go mod vendor` copies
  every file of the platform modules, the TPC archives included, into
  `vendor/`. A source package built that way would distribute the TPC object
  code. `debian/rules` refuses that vendored tree at `dh_auto_build`; it
  does not intercept `dpkg-source -b` or source-only archive creation. Those
  source archives must not be published under the exclusion policy. No
  source-publication workflow is implemented here. The linker needs a file for each
  `-l` flag, so the archives cannot simply be deleted. The vendored layout
  that would work (option 1c below) needs approval first. The repository has
  no record of a published Debian or PPA source package; the maintainer
  should confirm whether one was ever uploaded.
- **A passing marker gate is not distribution approval.** Native artifact,
  license, SBOM and other release requirements remain independent. Keep a
  failed or unavailable gate explicit rather than treating notice delivery
  as clearance.

The notice file omits the TPC agreement under the recorded exclusion policy.
Its generated preamble now states the limits of the technical checks. The
license bodies for the other components remain unchanged. No analytics
features are removed by this notices change.

## 4. What GoPMgr needs from an analytics engine

The parity baseline comes from `internal/analytics/duckdb.go`:

- an in-memory database with no network extension install
  (`autoinstall_known_extensions=false`). It sets `allowed_directories`,
  but with external access enabled that is not an enforced file sandbox;
  reader-boundary qualification is separate work;
- `read_csv_auto` for `.csv`, `.tsv`, and `.txt` files, `read_parquet` for
  `.parquet`, and `read_json_auto` for `.json` and `.ndjson`, with schema
  inference;
- SQL aggregation for `PortfolioRollup`, using exact-money normalization in
  Go (`normaliseProjectMetricsMoney`);
- linked extensions: core functions, Parquet, JSON, and ICU, plus
  Autocomplete, which DuckDB links by default.

Historical measurements from the original PR #146 review: a stripped Linux amd64 build is 128,316,384 bytes (122.4 MiB) with
DuckDB, and 71,541,904 bytes (68.2 MiB) with the stub engine (no `duckdb`
tag). DuckDB therefore adds about 54 MiB.

## 5. Decision matrix

Options 1, 2, and 3 are the ones the maintainer asked to compare. Option 0
is the measured current state, shown for reference.

| | 0. Official prebuilt DuckDB, exclusion enforced (current) | 1. Source-built DuckDB without TPC | 2. Apache DataFusion replacement | 3. Block distributions temporarily |
|---|---|---|---|---|
| **Maintenance** | Low: a Dependabot bump plus the review the gate forces | High: a DuckDB build per release on 5 targets, tracking the bindings' build flags, a new CI pipeline | High at first: a new Rust toolchain, the accepted native gRPC worker boundary, new tests; then moderate | None while blocked |
| **Performance** | Unchanged | Same engine; equal if built with the same flags (not measured) | Not measured for GoPMgr's workloads; needs the S0/S7A benchmark | n/a |
| **Feature parity** | Full | Full, if the same five extensions are built | Partial until proven. CSV, TSV (delimiter option), Parquet, and NDJSON have direct readers. JSON array files, `read_*_auto` type inference, and SQL dialect differences need a parity test suite. | No new builds; existing releases remain unchanged |
| **Security** | Upstream builds; patches arrive with each bindings release | Own native build supply chain: hardening flags, provenance, and patch latency become GoPMgr's job | Memory-safe engine, but a large new crate tree (needs `cargo audit`/`cargo deny` and an SBOM); the authenticated native IPC boundary | No security fixes ship |
| **Licensing exposure** | Fixed-input symbol and marker evidence only; each artifact and licensing requirement remains a separate gate. The vendored Debian build is blocked; source-only creation is not intercepted | Lowest: the TPC files never exist, so a vendored source package is possible too | No TPC code. Apache-2.0 engine; needs a new crate-notices pipeline. DataFusion's own repository has TPC-H benchmark code outside the library crates, so the exclusion list would need engine-specific markers. | No new exposure; does not change past artifacts |
| **Package size** | ≈54 MiB for DuckDB | About the same as option 0 | Not measured | n/a |
| **Release risk** | Low: in place now and gated | Medium–high: a new build on three operating systems, plus the fork assessment the decision requires | High in the near term: accepted target architecture, feasibility not yet established (§5.2); DuckDB must stay until parity | Stops all releases, fixes included |

### 5.1 Option 1, in more detail

- **1a. Build DuckDB from source with `BUILD_EXTENSIONS` limited to the five
  linked extensions, and link it through the bindings'
  `duckdb_use_static_lib` tag.** DuckDB's source is unchanged; only the
  build configuration differs. It still needs the assessment the decision
  requires: who rebuilds on each DuckDB release, reproducible build flags,
  build provenance, security-patch latency, and packaging for each format.
  Build times were not measured here.
- **1b. Link a system `libduckdb` through `duckdb_use_lib` (dynamic).** This
  moves the problem to whoever packages that library. Whether
  Debian or Ubuntu packages `libduckdb` was not verified.
- **1c. Debian source package only: vendor the modules, then replace the two
  excluded archives with empty archives (`!<arch>` and no members).** The
  proposed link layout would need verification before any distribution;
  `--tree vendor` alone would not qualify it. This changes third-party vendored content, so
  it needs explicit maintainer approval, a test that the empty archives
  link, and a gate that the replacement happened. It is **not
  implemented.**

### 5.2 Option 2, in more detail

[ADR-012 v0.2 was accepted on 2026-10-26](ADR-012-v0.2-ACCEPTED-OWNER-DECISIONS.md).
It targets a Go exact-rollup candidate and a Rust DataFusion/Arrow worker
behind authenticated native gRPC and approved local IPC. It does not authorize
FFI integration or production cutover. Only the isolated DF0 feasibility spike
is authorized. SQLCipher remains authoritative for project persistence.

The [DF0 evidence record](../testing/adr-012-df0-evidence.md) now reports
Linux results for A/B/C. Host-mediated reads (C2), security/no-spill (D),
comparative performance and numeric fidelity (E), packaging/licensing (F),
the report (G), and native-platform gaps remain. These precede the reserved
owner decisions. Preserve import formats, exact-money behavior and offline
operation. Do not upgrade DuckDB as a prerequisite for retirement, start Slice J,
or claim replacement complete before its qualification gates pass.

### 5.3 Current disposition

1. Keep the existing engine while repairing its release safeguards. This is
   interim maintenance, not a new engine-selection decision.
2. Keep the Debian build block and the prohibition on publishing source
   archives carrying the excluded files. Empty archive replacement (1c) is
   an unapproved proposal, not an implemented workaround.
3. Continue the accepted DF0 evidence path before proposing production
   retirement. A notices or marker PASS cannot replace LC-GATE, an SBOM,
   native candidate checks or owner authorization.

## 6. Remediation assessment: previously distributed versions

| Distribution | What was checked | Finding |
|---|---|---|
| GitHub release v1.1.0-alpha.2 (2026-09-08, tag commit `ad5c134a`, duckdb-go-bindings v0.10505.0): `.deb`, `.rpm`, `.dmg`, Windows installer | the binaries inside each asset (§2.4) and the v0.10505.0 archives (§2.2) | Original review reported no listed TPC markers in these binaries. |
| GitHub's source archives for that tag | the tree at `ad5c134a` | No `vendor/` directory and no TPC file. No commit in the repository's history has added one. |
| Earlier releases | original reviewer's release/tag inventory | At that review, v1.1.0-alpha.2 was reported as the only release. |
| CI build artifacts | retention settings | Kept seven days, then deleted. Built from the same prebuilt libraries, so the §2.1 mechanism applies, but no artifact was individually audited. |
| Debian or PPA source package | the repository | `debian/` exists, but nothing records an upload. **Open: the maintainer should confirm.** If one was published with a vendored tree, it would have contained the TPC archives as files. |

This historical assessment makes no legal finding. A marker scan reports
listed bytes, not a complete inventory of every artifact's code. Whether earlier distributions need any further action is for
qualified legal review to decide.

## 7. Open items

- SBOM generation remains required and unimplemented here. Select its format
  and artifact coverage under the repository supply-chain standard; no SPDX
  2.3 implementation or approval is implied by the earlier proposal.
- Maintainer confirmation of any Debian or PPA source-package uploads remains
  unavailable. Do not infer that no upload occurred from repository silence.
- Native checks must name the exact candidate and distinguish local macOS,
  Linux, Windows, packaged artifacts and hosted CI. Missing evidence remains
  unavailable, not PASS.
- Qualified legal review and separate maintainer approval are required before
  any exception allowing distribution of the TPC implementations.

See [the qualification procedure](../testing/duckdb-notices-qualification.md)
for current checks, failure cases, reproducibility and proof limits.
