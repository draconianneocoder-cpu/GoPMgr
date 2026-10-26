<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# TPC-H and TPC-DS exclusion: decision, evidence, and options

**Status:** maintainer decision recorded 2026-10-26; options below await a
maintainer choice.
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

## 2. Finding: the TPC code was never linked

PR #146 first said the `tpch` and `tpcds` extensions were "linked on every
platform." **That was wrong.** The evidence below shows that the official
prebuilt DuckDB libraries keep both extensions out of every binary GoPMgr
links, on every platform, in the current version and in the version
1.1.0-alpha.2 shipped with.

### 2.1 Why they are not linked

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
  archive only to resolve a symbol that is still undefined. Nothing
  references these archives, so they add no code.

### 2.2 Reachability proof, per platform

`tools/duckdb-notices -check` reads every archive on each platform module's
link line (ELF, Mach-O, and COFF object members). It confirms three things:

1. No symbol defined in the excluded archives is left undefined by any other
   linked archive, or called by the Go bindings' cgo code.
2. All eight markers appear in the excluded archives.
3. No marker appears in any other linked archive.

Because of (1), no link order can pull the excluded code in.

| Module (v0.10506.0) | Global symbols the TPC archives define | Referenced elsewhere | Markers in TPC archives | Markers elsewhere |
|---|---|---|---|---|
| darwin-amd64 | 649 | 0 | 8 of 8 | 0 |
| darwin-arm64 | 647 | 0 | 8 of 8 | 0 |
| linux-amd64 | 650 | 0 | 8 of 8 | 0 |
| linux-arm64 | 650 | 0 | 8 of 8 | 0 |
| windows-amd64 | 713 | 0 | 8 of 8 | 0 |

The same audit run against v0.10505.0 (darwin-arm64, linux-amd64,
windows-amd64), the version 1.1.0-alpha.2 shipped with, gave the same
result: 647, 650, and 713 symbols, none referenced, and no problems. The
cgo check covered 632 C names used by duckdb-go-bindings and duckdb-go/v2.

### 2.3 Markers: code, not incidental strings

`third_party/EXCLUDED-NATIVE-CODE.txt` lists eight markers. The four C++
type names and four messages below occur only in the generator code, and
they survive symbol stripping:

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
So every DuckDB binary contains the bare words `dbgen`, `dsdgen`, `tpch`,
and `tpcds`. That table is data in the core library: it names extensions
DuckDB could load, and it holds none of their code. It is deliberately not
a marker.

### 2.4 Binaries audited

The positive controls show that the method detects the code where it is
present, and finds DuckDB in each binary.

| Binary | Size (bytes) | DuckDB present (Parquet and JSON type info) | TPC markers | Notes |
|---|---|---|---|---|
| 1.1.0-alpha.2 `.deb`, `/usr/bin/gopmgr` | 93,251,424 | yes | 0 of 8 | `gopmgr-1.1.0-alpha.2-amd64.deb`, SHA-256 `fa1f290c8cd6259e91dee340ca0d7a1b2b123a457065e4fe48ab0252d466f9a3` |
| 1.1.0-alpha.2 `.rpm`, `/usr/bin/gopmgr` | 93,251,424 | yes | 0 of 8 | `gopmgr-1.1.0-alpha.2-x86_64.rpm`, SHA-256 `9c2de38867e3edf5b37f1204520e05c42531d76c4f67b0853b9edb30e13e7881`; the binary is byte-identical to the `.deb`'s |
| 1.1.0-alpha.2 `.dmg`, `GoPMgr.app/Contents/MacOS/gopmgr` | 82,778,112 | yes | 0 of 8 | `GoPMgr-1.1.0-alpha.2-arm64.dmg`, SHA-256 `c4a843bd8e72e27ec41bc20f8aa5f9d0186e957081a570ad2bc5424251902ca8` |
| 1.1.0-alpha.2 Windows installer, `gopmgr.exe` | 75,446,784 | yes | 0 of 8 | `GoPMgr-1.1.0-alpha.2-amd64-setup.exe`, SHA-256 `a4156473884af29526266b7aed98f845ef665e12b9d7418ee666a49ab439ef76` |
| Current branch, Linux amd64, symbols kept | 151,996,608 | yes (146,101 symbols) | 0 of 8 | 0 symbols in `tpch::` or `tpcds::`, and none of the extension or `dbgen`/`dsdgen` function classes |
| Positive control: `libtpch_extension.a` and `libtpcds_extension.a` | | n/a | 8 of 8, on every platform and both versions | `scripts/verify-excluded-native-code.sh` fails on them |

Each release binary does contain the incidental string `dbgen`, from the
autoload table described above.

**Not audited:**

- Current-branch macOS and Windows binaries. No native macOS or Windows
  build host was available. Those platforms are covered by the archive-level
  proof in §2.2, and from now on by the packaging gate on every build.
- CI build artifacts. They expire after seven days (`retention-days: 7` in
  `ci.yml` and `release.yml`).

## 3. Enforcement added (release-blocking)

| Gate | Where it runs | What fails it |
|---|---|---|
| `third_party/EXCLUDED-NATIVE-CODE.txt` | the single source for every check | `TestRepositoryExclusions` fails if either archive or the extension markers are dropped |
| `make duckdb-notices-check` (`tools/duckdb-notices -check`) | `make verify`, run in CI | an excluded symbol referenced by a linked archive or a cgo call; a marker missing from the excluded archives or present in another linked archive; a platform module in `go.mod` that cannot be read (every one is audited, not only the release platforms) |
| `scripts/verify-excluded-native-code.sh <binary>` | `scripts/verify-module-notices.sh`, which every packaging path runs: the tarball, `.deb` and `.rpm`, the macOS app and installer, the Windows release step, and `debian/rules` | any marker in the binary |
| `scripts/verify-excluded-native-code.sh --tree vendor` | `debian/rules`, before building | an excluded archive, or any file holding a marker, in the vendored tree |
| `make excluded-native-code-tests` | `make verify` | 25 regression cases, including the real TPC archives as positive controls |

Two consequences need the maintainer's attention.

- **The Debian source build is blocked by design.** `go mod vendor` copies
  every file of the platform modules, the TPC archives included, into
  `vendor/`. A source package built that way would distribute the TPC object
  code, so `debian/rules` now refuses it. The linker needs a file for each
  `-l` flag, so the archives cannot simply be deleted. The vendored layout
  that would work (option 1c below) needs approval first. The repository has
  no record of a published Debian or PPA source package; the maintainer
  should confirm whether one was ever uploaded.
- **Binary releases are not blocked by these facts.** The official prebuilt
  libraries already exclude the code, and the gate proves that for each
  artifact. Whether that satisfies the decision's "remove from the linked
  binaries" requirement is the maintainer's call. Section 5 sets out the
  alternatives.

`third_party/DUCKDB-NATIVE-NOTICES.txt` no longer reproduces the TPC
agreement, because GoPMgr does not ship that code. It now says the archives
are excluded and that the release checks enforce it. No analytics feature
changed: GoPMgr never called `dbgen` or `dsdgen`. The CSV, TSV, JSON,
NDJSON, and Parquet import, the portfolio rollup, and offline operation are
unchanged.

## 4. What GoPMgr needs from an analytics engine

The parity baseline comes from `internal/analytics/duckdb.go`:

- an in-memory database with no network extension install
  (`autoinstall_known_extensions=false`) and file access restricted to the
  import directory (`allowed_directories`);
- `read_csv_auto` for `.csv`, `.tsv`, and `.txt` files, `read_parquet` for
  `.parquet`, and `read_json_auto` for `.json` and `.ndjson`, with schema
  inference;
- SQL aggregation for `PortfolioRollup`, using exact-money normalization in
  Go (`normaliseProjectMetricsMoney`);
- linked extensions: core functions, Parquet, JSON, and ICU, plus
  Autocomplete, which DuckDB links by default.

Size: a stripped Linux amd64 build is 128,316,384 bytes (122.4 MiB) with
DuckDB, and 71,541,904 bytes (68.2 MiB) with the stub engine (no `duckdb`
tag). DuckDB therefore adds about 54 MiB.

## 5. Decision matrix

Options 1, 2, and 3 are the ones the maintainer asked to compare. Option 0
is the measured current state, shown for reference.

| | 0. Official prebuilt DuckDB, exclusion enforced (current) | 1. Source-built DuckDB without TPC | 2. Apache DataFusion replacement | 3. Block distributions temporarily |
|---|---|---|---|---|
| **Maintenance** | Low: a Dependabot bump plus the review the gate forces | High: a DuckDB build per release on 5 targets, tracking the bindings' build flags, a new CI pipeline | High at first: a new Rust toolchain, an FFI layer or worker process, new tests; then moderate | None while blocked |
| **Performance** | Unchanged | Same engine; equal if built with the same flags (not measured) | Not measured for GoPMgr's workloads; needs the S0/S7A benchmark | n/a |
| **Feature parity** | Full | Full, if the same five extensions are built | Partial until proven. CSV, TSV (delimiter option), Parquet, and NDJSON have direct readers. JSON array files, `read_*_auto` type inference, and SQL dialect differences need a parity test suite. | No new builds; 1.1.0-alpha.2 stays the latest |
| **Security** | Upstream builds; patches arrive with each bindings release | Own native build supply chain: hardening flags, provenance, and patch latency become GoPMgr's job | Memory-safe engine, but a large new crate tree (needs `cargo audit`/`cargo deny` and an SBOM); a new FFI or IPC boundary | No security fixes ship |
| **Licensing exposure** | Binaries: no TPC code (proven per artifact). Source: none in the repository; the vendored Debian source is blocked | Lowest: the TPC files never exist, so a vendored source package is possible too | No TPC code. Apache-2.0 engine; needs a new crate-notices pipeline. DataFusion's own repository has TPC-H benchmark code outside the library crates, so the exclusion list would need engine-specific markers. | No new exposure; does not change past artifacts |
| **Package size** | ≈54 MiB for DuckDB | About the same as option 0 | Not measured | n/a |
| **Release risk** | Low: in place now and gated | Medium–high: a new build on three operating systems, plus the fork assessment the decision requires | High in the near term: no approved design yet (see §6), and DuckDB must stay until parity | Stops all releases, fixes included |

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
  link line still resolves, no TPC code is distributed, and the
  `--tree vendor` gate passes. This changes third-party vendored content, so
  it needs explicit maintainer approval, a test that the empty archives
  link, and a gate that the replacement happened. It is **not
  implemented.**

### 5.2 Option 2, in more detail

The maintainer described the DataFusion replacement as already planned. The
repository does not record an approved DataFusion design. In the ADR-010
documents, which are not yet on `main`, the implementation plan's row P16
reads "Unapproved mandatory DuckDB/DataFusion decision". The v2.1 plan says
"No engine (DataFusion, Arrow, Iceberg) is chosen here." So accelerating
DataFusion starts with a decision, not with code:

1. Record the engine decision (P16) as an ADR, including the integration
   shape: a Rust static library through cgo, or a separate worker process
   over the ADR-010 IPC transport.
2. Build an engine-neutral parity suite from the current DuckDB outputs: the
   import readers listed in §4, schema inference, and the rollup. This helps
   whichever engine wins, and DuckDB keeps running until it passes.
3. Benchmark against the S0 baseline.
4. Add Rust build integration on all three platforms, plus crate notices and
   SBOM entries.
5. Swap engines behind `analytics.Engine` and remove DuckDB only after
   parity.

No duration estimate is given. Steps 1 and 3 decide most of the cost, and
neither has been done.

### 5.3 Recommendation, for the maintainer's decision

1. Keep option 0 for binary artifacts. The code is provably not linked, and
   every packaging path now fails if that ever changes. This keeps all
   analytics features and adds no fork.
2. Keep the Debian source build blocked until the maintainer approves 1c or
   decides not to publish a source package.
3. Start option 2 with step 1 (record the P16 engine decision) and step 2
   (the parity suite). Take up option 1a only if the maintainer decides
   that shipping the TPC archives inside the build-time module cache, never
   in an artifact, is itself unacceptable.
4. Option 3 is not needed on the evidence, because no artifact contains the
   code. It remains the maintainer's prerogative.

## 6. Remediation assessment: previously distributed versions

| Distribution | What was checked | Finding |
|---|---|---|
| GitHub release v1.1.0-alpha.2 (2026-09-08, tag commit `ad5c134a`, duckdb-go-bindings v0.10505.0): `.deb`, `.rpm`, `.dmg`, Windows installer | the binaries inside each asset (§2.4) and the v0.10505.0 archives (§2.2) | No TPC code in any binary. |
| GitHub's source archives for that tag | the tree at `ad5c134a` | No `vendor/` directory and no TPC file. No commit in the repository's history has added one. |
| Earlier releases | the repository's releases and tags | v1.1.0-alpha.2 is the only release. |
| CI build artifacts | retention settings | Kept seven days, then deleted. Built from the same prebuilt libraries, so the §2.1 mechanism applies, but no artifact was individually audited. |
| Debian or PPA source package | the repository | `debian/` exists, but nothing records an upload. **Open: the maintainer should confirm.** If one was published with a vendored tree, it would have contained the TPC archives as files. |

This assessment makes no legal finding. It reports what each artifact
contains. Whether earlier distributions need any further action is for
qualified legal review to decide.

## 7. Open items

- **SBOM (required by the decision, not in this change).** Proposed next
  slice: an SPDX 2.3 JSON document per release artifact. It would be built
  from `go version -m` for Go modules, the `tools/duckdb-notices` component
  list for the DuckDB native code, the frontend bundle's package set, and the
  font inventory. It would list the excluded archives as present at build
  time and not shipped, and it would get a CI drift gate like the notices
  files.
- Maintainer confirmation of whether a Debian or PPA source package was ever
  uploaded.
- Maintainer choice among the options in §5, and approval or rejection of
  option 1c.
- Current-build binary audits on native macOS and Windows hosts. The
  packaging gate now runs on every build there.
- Qualified legal review of the TPC licensing question, if the maintainer
  ever wants to ship the generators. Nothing here assumes that review.
