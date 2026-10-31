<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-012 Rust dependency and native-worker review input

2026-10-31: partial DF0-F/F2 evidence for the isolated macOS arm64 spike.
This fills the inventory document named by the
[DF0 specification](../testing/adr-012-datafusion-feasibility-spike.md).
It is **not a complete SBOM, license audit, DF-049 PASS or LC-GATE clearance**.
Production still uses DuckDB; no release package or dependency pin changed.

## Dependency scope and provenance

The [dependency manifest](../../spikes/adr012-datafusion/evidence/df0-f-macos/dependencies.json)
contains **268 packages and 1,051 relationships** reachable from `df0-worker`
without development edges, using locked, offline Cargo metadata filtered for
`aarch64-apple-darwin`. The workspace graph has 269 packages; the separate CLI
is outside this closure. The unfiltered lockfile advisory scan counts 295
dependencies, including other resolved platforms/scopes. These are different
inventories, not interchangeable package counts.

The manifest preserves source identity, declared license expressions, enabled
workspace-resolved features, declared target kinds, manifest hashes, and hashes
of available root `LICENSE*`, `LICENCE*`, `COPYING*`, `NOTICE*` files/directories
and explicit Cargo `license_file` entries. There are 1,039 normal and 12 build
relationships. Normal dependencies can include procedural macros; this is not
a runtime-versus-build artifact classification. Thirty-five packages declare
custom-build targets; declared targets are not proof they executed in this
binary build. Workspace feature unification is not a worker-only feature proof.

There are 262 registry packages and six repository-local packages: the worker,
engine and four [approved temporary source overrides](../../spikes/adr012-datafusion/third_party/GOPMGR.md).
The overrides remain spike-only, with their existing provenance, source-tree
integrity tests and removal conditions. No source was relabeled as an upstream
release. Registry and local sources remain distinguishable after private paths
are replaced with repository-relative identities. Unexpected registry/git
sources and unexplained local paths fail rather than silently losing provenance.

All 268 packages declare a license expression, spanning 24 distinct strings.
Historical Cargo spellings such as `MIT/Apache-2.0` are preserved exactly, not
silently normalized into an SPDX conclusion. No available root notice file was
found by this bounded scan for `alloc-stdlib`, `flatbuffers`, `tonic-prost`,
`df0-engine` or `df0-worker`. This does not prove missing rights or that no
license text exists elsewhere. First-party licensing also uses the repository's
`LICENSES/GPL-3.0-or-later.txt` and source headers. Per-crate obligations, license
choices, notices outside the scan, nested native source and distribution
requirements remain review work.

The JSON is deliberately a custom review manifest. It does not claim SPDX
3.0.1 conformance, exact linked composition or complete runtime/build/dev SBOM
relationships. A complete artifact-specific SBOM must also account for actual
build inputs, bundled native implementations, the compiler standard library,
platform libraries, source overrides and the installed package.

## Advisory scan

The [unfiltered Cargo audit](../../spikes/adr012-datafusion/evidence/df0-f-macos/cargo-audit.json)
completed with **zero vulnerabilities and zero informational warnings** using
`cargo-audit 0.22.2`, no ignored advisory IDs and database commit
`2295466c4e9df26647fd67d8fa1c705ac2fc7f81` (last updated 2026-10-30).
This is version/database-scoped evidence, not a source-security audit. It does
not clear Go/frontend dependency alerts or qualify bundled native code.

## Existing native macOS worker

[Binary observations](../../spikes/adr012-datafusion/evidence/df0-f-macos/native-worker.json)
are tied to the unchanged reviewed worker SHA-256
`337b43a96713b9111266dfd624d8a7c715e9445ce9e64c45cc8b46b2f5a10204`.
It is 120,839,344 bytes (about 115.2 MiB), thin arm64, with three observed
system dynamic-library references: CoreFoundation, libiconv and libSystem.
No `LC_RPATH` was observed. The Mach-O load command declares macOS 11.0 and SDK
27.2; only the current macOS 27.2 host was exercised. These declarations do not
establish minimum-OS support or the final application's footprint.

The existing linker ad-hoc signature passes strict integrity verification.
It has no Developer ID/team identity, notarization or installed-app evidence.
The installed pinned compiler probe reports Rust 1.99.1; the existing worker
was reused, not rebuilt here. This slice cannot establish fresh-build
reproducibility or upgrade/rollback behavior.

`nm` returned 221,495 symbol lines, including 9,848 containing `df0_worker`
(a populated-output control), zero literal `tpch`/`tpcds` name matches, and
133 `duckdb` name matches. **All 133 also contain `sqlparser`**; the resolved
sqlparser 0.62.0 source defines DuckDB grammar/AST names such as
`DuckDbDialect` and `AttachDuckDBDatabaseOption`. The dependency graph contains
no DuckDB crate. This explains those name matches without treating a name scan
as engine-code detection. No complete static archive/source audit or proof of
absence of embedded implementations follows. Full F2 and LC-GATE remain open,
including the shipped Go/DuckDB application path.

## Reproduction and verification

From the repository root, with existing pinned tools/cache:

```sh
bash spikes/adr012-datafusion/scripts/with-rust-toolchain.sh cargo metadata \
  --manifest-path spikes/adr012-datafusion/rust/Cargo.toml --locked --offline \
  --filter-platform aarch64-apple-darwin --format-version 1 > /absolute/path/to/metadata.json
```

From `spikes/adr012-datafusion`:

```sh
go run ./cmd/df0-inventory -repo-root ../.. -target aarch64-apple-darwin \
  < /absolute/path/to/metadata.json > /absolute/path/to/dependencies.json
go test -race -count=1 ./cmd/df0-inventory
```

The target label records the operator's metadata command; it is not inferred
from Cargo JSON. Input is trusted Cargo metadata and stable local source/cache,
not a hardened reader for concurrently hostile filesystem mutation. Notice
symlinks/escapes, missing files, missing graph nodes, duplicate identities and
ambiguous workers fail. Output is sorted, and unknown declarations stay unknown.

Also run locally: `cargo tree -e features` with the pinned launcher, lock,
offline mode, worker package and target; `cargo-audit audit --file
spikes/adr012-datafusion/rust/Cargo.lock --format json`; `otool -L`, `otool -l`,
`nm`, and `codesign` display/strict verification on the reviewed worker.
[Environment and hashes](../../spikes/adr012-datafusion/evidence/df0-f-macos/environment.json)
identify source, artifacts, tools, feature-tree fingerprint and proof limits.

Focused tests, the full native spike race suite, Go vet, source-override
integrity guards and an independent Python graph/hash comparison passed. The
new command also cross-compiles for Linux amd64; native Linux worker/package
qualification was not performed. Six compiling faults were detected at their
intended assertions: dev inclusion, invented license, skipped notice boundary,
ignored file content, ignored missing node and skipped local-source boundary.
The last fault initially survived because its test used a missing outside
manifest. The test now uses a valid file and requires the specific boundary
error; the same fault fails. A clean control passed. A new
parent-symlink regression failed before the fix and passed afterward.
Independent reviewer quota was unavailable; plan, mid-work and final reviews
use the disclosed structured self-review fallback.

## Remaining retirement gates

- Complete per-crate/native-source licensing dispositions and an actual
  artifact-specific SBOM; qualify F2/LC-GATE separately from engine cutover.
- Qualify installed/signed macOS packaging, missing-worker recovery,
  update/rollback and minimum-OS behavior without changing project formats.
- Finish containment/security and controlled performance/full wire evidence.
- Obtain the separate production integration/cutover decisions, then remove
  DuckDB code, native bindings, tags, packaging and obsolete docs coherently.

Linux follows macOS. Windows execution is deferred by owner priority; its
accepted platform gates remain unqualified rather than silently passing.
