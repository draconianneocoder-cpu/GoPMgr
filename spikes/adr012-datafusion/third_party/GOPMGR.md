<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Temporary DF0 Zstd source overrides

The maintainer approved these isolated spike overrides on 2026-10-31 after
the [candidate assessment](../../../docs/testing/adr-012-df0-zstd-assessment.md).
The GoPMgr maintainers own their review, updates and removal. They are not
approved for the production application or its packages.

## Origin and change

These are complete published crate archives, verified against their SHA-256
checksums in the workspace lock at baseline
`d880056262fc3234a4d137fc575fe29b1be14317`:

| Crate | Archive SHA-256 |
|---|---|
| `arrow-ipc` 59.3.2 | `f5dd1afadda2eb821aaf851bb4c0adef60c30cc0f1a29bd02fb99dd07fb9f58d` |
| `parquet` 59.3.2 | `8eb260d566f2fbdabe356d26aaa5d13403aac29b5b4e44463b8f9bd7b2b12680` |
| `datafusion` 55.2.0 | `4cd5317db77ba6bca25c761a8091b9d294a9f4188e4f9dcd0243e6a57de862ac` |
| `datafusion-datasource` 55.2.0 | `210aa1e9525e9454f2399dbb8b0b95510145bd8c1fdc35de74e9ccbf81bcb699` |

Registry origins: [Arrow IPC](https://crates.io/crates/arrow-ipc/59.3.2),
[Parquet](https://crates.io/crates/parquet/59.3.2),
[DataFusion](https://crates.io/crates/datafusion/55.2.0),
[DataFusion datasource](https://crates.io/crates/datafusion-datasource/55.2.0).

Only normalized `Cargo.toml` files are modified. Their normal Zstd dependency
requirements, and Parquet's dev requirement, are pinned to `=0.14.3` instead
of `0.13` / `0.13.0`. Each changed file carries a modification notice. All
Rust code and feature declarations remain upstream bytes. Crate versions
are retained; the workspace explicitly selects local sources rather than
presenting them as new published releases.

`Cargo.toml.orig` and each upstream `Cargo.lock` remain unchanged provenance
artifacts. Cargo resolves the spike using `rust/Cargo.toml` and
`rust/Cargo.lock`, not those nested locks. No cache marker, build tree or
credential is included. The directory is named `third_party` so Go does not
mistake it for its automatic vendor mode.

## Licensing and integrity

All four crates declare Apache-2.0. Their `LICENSE.txt` and `NOTICE.txt` files
are preserved verbatim, including upstream third-party attributions. The Arrow
license's MIT reference names `arrow-array/src/delta.rs`, outside these copies;
its reference and notice are retained. Repository REUSE annotations describe
these source copies, not a production distribution clearance or full DF0 SBOM.

`go test -count=1 ./scripts -run 'TestDF0Source'` verifies the four selected
paths and complete source-tree pins. For each crate, the digest is SHA-256
over sorted slash-relative filenames. Each record contains an eight-byte
big-endian UTF-8 filename length, its bytes and the raw 32-byte SHA-256 of
the file. Symlinks and nonregular files fail. Added, missing, renamed or changed
files change the digest. `.gitattributes` disables text conversion only for
these four trees, preserving bytes on Windows too. This protects a reviewed
snapshot; it is not a signature or an independent audit of upstream code.

To reconstruct or review a change, obtain each exact published archive,
verify its checksum above before unpacking, apply only the five constraints
and modification notices, then compare every file with the checked-in tree.
Do not change a pin merely to pass a check. A new source, feature or semantic
change needs dependency review and fresh audit, tests and corpus evidence.

## Removal condition

Replace these copies with compatible published versions as soon as they:

1. Remove the vulnerable Zstd pair from the resolved graph without dropping
   a supported codec or changing the accepted engine/API baseline silently.
2. Pass the unfiltered active-workspace audit, source/feature review,
   interrupted-context regressions and applicable native build checks.
3. Preserve portfolio, tabular and filesystem results, retaining honest
   dispositions for existing DF0 failures.

Then remove the four workspace patches, source trees, their REUSE and Git
attributes, config inventory entries and `TestDF0Source*` guards. Update the
lock and this record. Keep the behavioral context regressions, update
`TestDF0ZstdLockPins` for the reviewed replacement versions, and update the
evidence links. Remove Apache-2.0's root license text only if no remaining
repository file needs it. No production cutover follows automatically.
