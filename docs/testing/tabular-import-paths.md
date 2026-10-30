<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Tabular import: selected-path verification

LD-6 correction, qualified against `main` at `6844d16`. The maintainer approved the
[temporary filename restriction](../review/architecture-directive-2026-10-27.md#7-temporary-tabular-import-filename-restriction-2026-10-29)
on 2026-10-29. This is an interim correction in the existing engine, not
DuckDB replacement qualification.

## Requirements and evidence

All tests below are in `internal/analytics/duckdb_import_test.go` and use the
real pinned DuckDB engine, with synthetic local files only.

| Requirement | Test and observable result |
| --- | --- |
| Reject supplied glob-bearing names and folders across all six formats | `TestImportTabularRejectsGlobsAndRecoversAcrossFormats`: CSV, TSV, TXT, JSON, NDJSON and Parquet; `*`, `?`, `[12]` and unmatched `[` in filenames and folders return `ErrAmbiguousImportPath` and an empty dataset. A corrupt matching neighbor exposes the original glob expansion; it does not observe all possible discarded reads. A distinct valid file succeeds on the same engine after every rejection. |
| Check resolved sources too; preserve selection and format | `TestImportTabularSymlinksKeepSourceIdentity`: ordinary aliases to glob-bearing targets fail; a glob-bearing alias to a safe target fails; extensionless target aliases retain the selected CSV reader. Distinct target/decoy rows and a missing lexical sibling prove symlink/parent traversal selects the target. |
| Preserve selected-path partition inference | `TestImportTabularPreservesSelectedPathPartitions`: all six formats retain exact columns and values for aliases into and out of Hive partition folders; direct partition reads remain a positive control. |
| Preserve names outside the restricted tokens | `TestImportTabularLiteralNonGlobNames`: Unicode, spaces, apostrophes, closing brackets, braces and valid Unix backslashes retain exact expected columns and cell values. |
| Preserve recovery and preview bound | `TestImportTabularFailureRecoveryAndRowLimit`: pre-cancelled context and invalid Parquet return errors and no dataset, followed by exact successful imports on the same engine. A 10,001-row input still returns the existing 10,000-row preview. This does not resolve the separate truncation-disclosure defect. |
| Reject before source lookup or engine acquisition | `TestImportTabularRejectsGlobsBeforeLookupOrConnection`: a missing glob-bearing path on a closed engine still returns the policy error and no data. The symlink test repeats canonical-only refusals on a closed engine to pin rejection before connection acquisition. A relative alias from a bracket-bearing working directory also fails before connection acquisition. |

## Defect sensitivity and lessons

Before the guards, the new glob regressions failed with actual reads of the
unselected neighbor, or successful imports where explicit refusal was
required. SQL string quoting prevents query syntax injection, not file glob
expansion. Canonicalization must follow symlinks before cleaning `..`; otherwise
validation and the reader can consistently select the wrong file.

An exploratory exact-file allowlist with external access disabled rejected
escaped query patterns as well as unselected opens. Those pinned settings
cannot subsequently be changed on that database. This correction therefore
uses the approved rejection contract without changing the shared engine's
configuration or adding a new reader lifecycle.

In the original qualification at `15c7bac`, five controlled faults were
injected separately into a disposable copy.
The tests rejected removal of the raw-path guard, removal of the canonical
guard, cleaning `..` before resolving symlinks, choosing the reader from the
target's extension, and raising the row limit to 10,001. The traversal fault
returned the independent decoy values and failed the exact-content assertion.
All failures came from the intended test assertions, not compilation errors
or timeouts. The source checkout was not modified by fault injection.

## Execution and limits

### Original qualification (`15c7bac`, 2026-10-29)

Local environment: Go 1.27.3, darwin/arm64. Focused tagged tests passed;
the analytics package measured 84.5% statement coverage. The untagged stub
tests also passed. Coverage is diagnostic evidence, not a safety proof.

Also passed locally: `make verify`,
`go test -race -count=1 -tags duckdb ./internal/analytics`,
`go vet -tags duckdb ./internal/analytics`, and the tagged root portfolio
regressions (`TestRunPortfolioAnalytics|TestLegacyPortfolio`). These checks
cover the existing caller and rollup behavior as well as the changed import
boundary. Cancellation evidence is limited to a pre-cancelled context and
subsequent engine reuse, not interruption during an active read.

`make coverage-ledger-drift` passed all 48 package headings;
`make license-check release-scope` passed, including REUSE metadata on all
1,159 files. `make memory-scan` completed with 11 optional gosec findings on
unchanged source and no called vulnerable symbols reported by govulncheck.
The additional uncalled package/module advisories remain disclosed;
staticcheck was unavailable. No warning was suppressed or declared remediated
by this path correction.

### Current-main qualification (2026-10-30)

Requalification uses Go 1.27.3 on macOS arm64 with CGO enabled. The original
nine-file patch is retained, with closed-engine path assertions and selected-path
partition regressions added after independent review. The initial canonical-path
candidate changed Hive columns for ordinary aliases; a real-engine probe reproduced
both directions. The corrected reader retains the selected pathname only after
positive file-identity confirmation, with canonical fallback for missing or
unconfirmed lexical identities. Exact target/decoy assertions cover both an
existing lexical sibling and a missing one. The following checks passed on the current tree:

- `go test -count=1 -tags duckdb ./internal/analytics` and the tagged package
  race and vet checks;
- `go test -count=1 -tags duckdb -run 'TestRunPortfolioAnalytics|TestLegacyPortfolio' .`;
- `make verify` and `make coverage-ledger-drift` (all 49 package headings);
- `golangci-lint run --build-tags duckdb ./internal/analytics` (zero issues)
  and `govulncheck -tags duckdb ./internal/analytics` (no findings);
- `make license-check release-scope` (REUSE 3.3, all 1,418 files).

The native macOS arm64 build produced an ad-hoc signed application; strict
bundle-signature verification and the actual binary's notices checks passed.
This is local build evidence, not notarization or installed-release qualification.
The existing linker warning compares a Go object targeting macOS 13 with a
link target of 11; this result does not establish macOS 11 support.

The original importer failed the new real-engine regressions in a disposable
copy of current `main`, including an attempted read of an unselected Parquet
neighbor. Nine controlled faults were detected separately: omitted raw, canonical or
absolute reader-path guards; premature path cleaning; target-based format choice;
a raised preview bound; canonical refusal delayed until after connection
acquisition; always querying the canonical path; and omitted identity fallback.
The delayed guard failed with a closed-database error instead of the policy
error. The initial raw-guard fault survived the existing-file subset because
the canonical guard still rejected those paths; the missing-path early-refusal
test detected it. These are executed assertion failures, not compile failures.
The unmodified disposable control passed. No fault was applied to source files.

### Evidence limits

Literal `*` and `?` files cannot be created on Windows; bracket-name cases
remain active there. Symlink cases report a skip when Windows cannot create
links. These tests have not been executed on native Windows in this slice.
`os.SameFile` can return false when Windows cannot load file IDs; canonical
fallback then prioritizes the verified source, and path-derived partition
metadata may differ. Native Windows identity and symlink behavior remain
unverified. Concurrent filesystem replacement is outside the demonstrated guarantee.
No signed installed-app, release or replacement-worker qualification is
claimed. Existing numerical-fidelity and preview-completeness defects remain
separate work; no dependency version or storage format changes here.
