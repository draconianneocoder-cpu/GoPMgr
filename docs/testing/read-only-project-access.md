<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Read-only project access: candidate evidence

**Status: local validation and review passed; publication pending.**
This describes the BL-1 corrections reviewed on 2026-10-29, based on PR #166
(`3077ab2`) and `main` at `475ceba`. It is not a merged implementation or
release claim. DuckDB replacement is outside this slice.

## Requirements and verification

| Requirement | Evidence |
| --- | --- |
| Plaintext and encrypted headless reads do not migrate the project | `TestHeadlessPlaintextReadsPreserveLegacyFile`; existing encrypted headless tests. Repair and vacuum retain writable initialization. |
| Reader and export guard identify the same source project | `TestReadOnlySourceTraversalKeepsProjectAndExportIdentity`: distinct target/decoy plaintext and encrypted files, symlink/parent traversal, exact selected project identity and canonical `Database.Path`, successful ordinary exports and both exporters' refusal to overwrite the selected main/journal files. |
| Project metadata defaults only known additive fields | `TestGetProjectRejectsMissingRequiredColumns`: each original field missing with empty and populated tables returns a schema error. `TestGetProjectDefaultsOnlyMissingAdditiveColumns`: each of seven additive omissions, all seven together and stored-value controls retain independent money expectations and unchanged source bytes. |
| Preserve existing committed cost when additive columns are absent | `TestLegacyPortfolioPreservesContractAndFractionalLabour`: independent literal 100000 budget and 38518 committed minor units, fractional-hour rounding, stale compatibility floats, each missing column and the combined case, plus independently migrated-copy parity. |
| Preserve administrator access to legacy ledger values | `TestAdminLegacyCostsKeepAmountsAndDefaultOnlyMissingDetail`: six optional procurement fields independently and together, missing entry/baseline tables, and malformed present tables even when empty. Source and copied database bytes remain unchanged. |
| Legacy chart/document ordering uses instants rather than text | `TestLegacyListsOrderInstantsWithoutBackfilling`: offsets, whole seconds, fractional seconds, nanoseconds and migration's malformed-timestamp sentinel; project/kind filtering. |
| Reports retain the newest schedule and propagate read failures | `TestLegacyScheduleKeepsChronologyAndHeadlessExportData` and `TestBrokenScheduleIsNotAnEmptyExportOrUnavailableEVM`. Removing the consumer's redundant lexical sort prevents it undoing database ordering. |
| Stats tolerate additive omissions without accepting malformed tables | `TestLegacyStatsAndAuditExportDoNotRequireAdditiveColumnsOrTables` and `TestStatsRejectsMalformedPresentTablesEvenWhenEmpty`. Counts remain project-scoped. An absent legacy audit log exports the existing CSV header without creating a table. |
| Write protection comes from SQLite open mode | `TestReadOnlyModeProtectsEveryConnectionWithoutQueryOnly`: two simultaneously held connections reject writes after `query_only` is disabled; migration also fails. |
| Exports cannot overwrite project storage | `TestHeadlessExportsCannotOverwriteProjectFiles`: direct paths, symlinks, hardlinks, linked parents, dangling links, symlink/parent traversal and SQLite journal names including case variants. Ordinary outputs, valid non-project output aliases and dangling links to new reports remain usable; distinct names on case-sensitive filesystems remain valid. Both schedule and audit exporters check immediately before their file-writing call. This protects against accidental self-overwrite, not a same-user attacker concurrently replacing filesystem links. |
| Reads see committed live-WAL data and leave the writer usable | `TestReadOnlySeesLiveWALAndKeepsWriterUsable`. This is execution evidence for the local SQLCipher build, not native Windows proof. |
| SQLite coordination does not become a database write | `TestReadOnlyClosedWALAllowsCoordinationOnly`: plaintext/encrypted files at `0444` and `0600`, unchanged main bytes/permissions and WAL mode, no new WAL frames, only expected sibling names. The live-WAL test disables automatic checkpointing, proves the target commit resides in WAL and compares main/WAL bytes before opening, after reads and after reader close while the writer remains idle and open. |

Only documented additive fields receive defaults. Required columns in
present tables retain errors; existing integer money remains authoritative.
The schedule reader preserves the migration's numeric timestamp semantics
without adding or backfilling columns. Existing audit verification and CSV
formula-injection protections remain in place.

## Approved WAL coordination contract

On 2026-10-29 the maintainer approved SQLite coordination files only, recorded
in [the directive's additive §6 amendment](../review/architecture-directive-2026-10-27.md#6-read-only-sqlite-coordination-2026-10-29).
The original no-sidecar test demonstrated that the actual SQLCipher read-only
open creates WAL/SHM coordination for a closed, checkpointed WAL database.
Its replacement enforces the approved exception and retains the prohibited
effects: no main/schema changes, no frames in a new WAL, no alteration of
existing WAL data, chmod, migration, journal-mode changes or checkpoint.
Coordination files need not disappear at reader close. This is not zero
source-directory writes. Do not use `immutable=1` on a live WAL database.

## Evidence boundaries

Local environment: Go 1.27.3, darwin/arm64, Node 24.18.0. Focused legacy-data,
administrator, schedule, stats and approved-WAL tests passed. The results
below describe local source qualification, not a packaged release.
The stale-float fixture was corrected to alter only fields with nonzero
authoritative minor units; a zero integer with a nonzero compatibility float
still invokes the existing legacy fallback. Expected literal totals did not
change. Export-overwrite counterexamples failed before the guard. A first
live-SHM probe crashed the disposable test process during cleanup; the
regression fixture now uses DELETE journal mode so a broken guard fails
cleanly without truncating a mapped SHM file.

The required-project-column regression failed in all 22 cases before the
correction. Known additive omissions already passed, so the correction is
limited to rejecting malformed schemas. Post-correction focused project,
legacy ordering and live-WAL tests passed. Race checks for the corrected
export aliases and malformed stats tables passed. These focused results do
preceded the approved coordination-only contract. Four additional export cases
reproduced a path-resolution bypass before correction: cleaning `..` before
following a symlink made the guard inspect a different missing destination
from the writer. Resolution now preserves raw traversal until the parent
is resolved; both-exporter legitimate controls exercise the same traversal.
Final adversarial review also exposed source-path confusion: `filepath.Abs`
cleaned `..` before following source symlinks, selecting a decoy database.
Both plaintext and encrypted regressions reproduced that wrong selection.
The openers now resolve the source before cleaning, then share the canonical
path across the DSN, file validation and export guard.

After adopting that contract, `make verify`, `make coverage-ledger-drift`,
`make memory-scan license-check release-scope` and `make frontend-smoke`
passed. The ledger checks all 48 Go package coverage headings, including
root 72.8%, database 77.6% and notices tool 71.3%. The full
`go test -race -p=1 -count=1 . ./internal/...` run passed, as did
`go test -race ./tools/module-notices` and the DuckDB-tagged root portfolio
regressions. The complete export/stats race subset had also passed after
the traversal correction. After the final source-path correction, the
affected-package command `go test -race -p=1 -count=1 . ./internal/db` passed
again (root 352.182s, database 16.275s), along with the repeated pre-commit
and coverage-ledger gates.

Independent adversarial reviews approved the bounded project-schema,
export-path and revised WAL-test plans, their scoped corrections and the
final BL-1 source review after resolving the source-path finding.
`make verify` initially exposed
a notices-generator assumption about Go's license location under Homebrew.
The bounded per-file fallback is independently reviewed, tested against
missing/invalid inputs and passes the real notices freshness check; no
license check was bypassed and the generated notices did not change.
The optional security scan reports concatenated SQL in the two legacy
projections; source review confirms that names/defaults come only from fixed
code allowlists and values remain bound. These flags are not suppressed.
Other pre-existing optional scanner warnings remain outside this slice.
`govulncheck` reported no reachable vulnerable symbols, with additional
package/module advisories it classified as uncalled. Staticcheck was unavailable.

## PR consolidation and retained evidence

[PR #147](https://github.com/draconianneocoder-cpu/GoPMgr/pull/147) was closed
without merging on 2026-10-29: Wails 2.17.0 is already on main, and the
remaining DuckDB 2.0 upgrade is outside the qualified retirement sequence.
DuckDB remains at the existing version until the separate Slice J cutover.

[PR #163](https://github.com/draconianneocoder-cpu/GoPMgr/pull/163) remains
open until the corrected #166 candidate qualifies. Its immutable head
`4e4fda007067972d6b79e29dec7d731f6caa326d` retains additional hot-journal,
descriptor-cleanup, cancellation, damage and administrator live-WAL-copy
fixtures in `internal/db/readonly_test.go` and `project_readonly_test.go`.
Those fixtures and their original evidence remain inspectable; their entire
suite has not been re-executed against this candidate. Do not automatically
adopt #163's legacy-project refusals, symlink policy or `unreadable_reason`
interface. Preserve useful fixture coverage separately before deleting its
branch; a superseded implementation is not evidence that its unique tests
are unnecessary.

Native Windows/Linux and signed installed-app checks have not run for this
candidate. Existing npm audit output identifies the low-severity deepmerge
advisory through the Svelte plugin with no available fix; this slice does not
change dependencies or suppress that advisory.
