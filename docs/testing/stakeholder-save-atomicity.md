<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Stakeholder save atomicity

## Defect and contract

`SaveStakeholder` previously auto-committed its upsert before independently
reading the record. Readback failure reported failure after persistence; retrying
a blank-ID creation could insert another row. Disposable plaintext and encrypted
fixtures reproduced hidden mutations and duplicate retries. No production
incident is asserted.

The approved contract puts the existing write and project-scoped readback in
one transaction. Readback failure rolls back; success follows commit. Errors
wrap the operation with `%w`, preserving the underlying cause. A same-file
foreign-project ID collision now returns `ErrNoStakeholder` without persisting
the attempted update. The App's existing ownership checks remain unchanged.

Public signatures, generated and supplied IDs, schemas, defaults and exact-money
normalization are unchanged. Creation timestamps remain database-owned and are
retained on update. There is no migration, dependency, production fault hook,
retry loop or frontend change. Reverting this repair requires no migration
but restores its ambiguous-save behavior.

## Regression evidence

`internal/db/stakeholder_atomicity_test.go` contains three tests with 30 subcases:

- Twenty cases combine plaintext/encrypted storage, creation/update and injected
  begin/write/SELECT/scan-conversion/commit failures. Authorizers deny SQL;
  disposable triggers produce a value the readback cannot decode; a commit hook
  rejects commit. Hook failures assert one hit and the typed SQLite error,
  including the extended commit-hook code. Every failure returns an empty record
  and leaves the physical connection out of a transaction before close.
- All stored columns, including both integer timestamps, are compared through
  raw SQL after closing and reopening read-only without migrations. Explicit
  retry must persist exactly one record and return its exact fields. Another
  read-only reopen checks the returned record and raw columns. Updates use fixed
  historical timestamp pairs to expose accidental creation-time rewrites.
- Two same-file project-collision cases assert the missing-scoped-record
  sentinel, empty result, transaction cleanup and unchanged raw rows after reopen.
- Eight compatibility cases retain zero/negative-availability defaults, category
  defaults, caller IDs, legacy major-unit conversion, exact minor-unit precedence
  and both signed int64 limits. Stored minor units are checked directly.

Against the original implementation, the twelve begin/SELECT/scan-conversion
cases and both project-collision cases failed. Write and commit rejection already
prevented persistence. All thirty cases pass after the repair, including three
shuffled repetitions and a focused race-enabled run. Existing App ownership,
CRUD, money and timestamp tests remain unmodified.

Two isolated Go source-overlay mutations were rejected without modifying the
working source: removing the project predicate failed both collision cases;
returning success after commit rejection failed the commit case. Independent
read-only review found no introduced defect and separately passed focused tests,
focused race checks, formatting and whitespace checks.

## Verification and limits

Local macOS arm64 verification uses Go 1.27.3, CGO enabled; `go.mod` remains
Go 1.27.2 and Wails 2.16.0. Focused and full database/driver tests, Go vet/lint,
module verification, production build, frontend check/lint, all 631 frontend
tests, runtime smoke, strict ad-hoc signature verification, package code-map and
release-scope checks passed. `make verify`, REUSE (982/982 files) and the
coverage-ledger checks passed, including all 46 measured package headings.
Database statement coverage measured 77.8%.

The initial broad race run used an unnecessarily short custom four-minute
deadline and timed out in root/account tests during runnable Argon2 fixture
setup. It reported no race but is not a passing suite. The repository-standard
`make race` then passed all 45 first-party packages with `-p=1`, `-count=1`,
`webkit2_41` and the unchanged default ten-minute per-package deadline. Root
completed in 400.727 seconds and accounts in 391.521 seconds. No test, assertion
or cryptographic setting was weakened. These completed results resolve the
validation gap, not every possible cause of the earlier timeout.

The vulnerability scan reports no affected code or imported packages. Its
module-only GO-2026-5932 warning concerns `x/crypto/openpgp`, absent from this
application's dependency path. Existing Wails `Not found: time.Time`, macOS
13-object/11-link-target and six jsdom canvas diagnostics remain; this change
does not alter those paths or claim chart-render verification.

Reproduce the focused evidence with:

```sh
go test ./internal/db -run '^TestSaveStakeholder' -count=1 -timeout=60s
go test ./internal/db -run '^TestSaveStakeholder' -count=3 -shuffle=20261020 -timeout=60s
go test -race ./internal/db -run '^TestSaveStakeholder' -count=1 -timeout=60s
go test ./internal/db ./internal/sqlitedriver -count=1 -timeout=120s
make verify race build lint-go code-map-current release-scope
make coverage-ledger-current coverage-ledger-drift
go vet . ./internal/... ./tools/...
go mod verify
govulncheck -show verbose . ./internal/...
npm --prefix frontend audit
make frontend-smoke
reuse lint
```

Failed-COMMIT cleanup depends on the existing patched driver; deferred
`sql.Tx.Rollback` cannot itself clean up a Go transaction already finished by
`Commit`. See [driver evidence](sqlcipher-commit-cleanup.md). A rejected commit
hook does not establish the result of every COMMIT/rollback I/O failure. No
crash/power-loss, interactive native GUI, other-platform or full-release
qualification is claimed. A response lost after successful commit still needs
separate recovery/idempotency design.

Lessons: test failures after writing, inspect raw state rather than only a
normalized DTO, check transaction cleanup before closing can conceal it, and
use repository-standard budgets for whole cryptographic account suites.
The separate stakeholder foreground-error forwarding defect remains deferred.
