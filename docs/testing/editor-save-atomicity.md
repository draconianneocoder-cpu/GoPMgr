<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Editor save atomicity

One save-atomicity contract (write and project-scoped readback in one transaction), proven for four editors.

Each section below is a record kept as written, with its headings one
level lower and its links updated; nothing else changed.

## What this file holds

- [Resource-calendar save atomicity](#resource-calendar-save-atomicity): formerly `docs/testing/resource-calendar-save-atomicity.md`, undated, from commit `9ab5d91`.
- [Sprint save atomicity](#sprint-save-atomicity): formerly `docs/testing/sprint-save-atomicity.md`, 2026-10-18, from commit `9ab5d91`.
- [Stakeholder save atomicity](#stakeholder-save-atomicity): formerly `docs/testing/stakeholder-save-atomicity.md`, undated, from commit `9ab5d91`.
- [Work-item save atomicity](#work-item-save-atomicity): formerly `docs/testing/work-item-save-atomicity.md`, undated, from commit `9ab5d91`.

## Resource-calendar save atomicity

### Defect and approved contract

`SaveResourceCalendar` previously auto-committed its upsert before independently
reading the record. A SELECT, scan or JSON decoding failure could therefore
report failure after persistence; retrying a blank-ID creation could insert
another row. Disposable file-backed tests reproduced hidden mutations and
duplicate retries. No production incident is asserted.

The write and project-scoped readback now share one transaction. Readback
failure rolls back; success follows commit. Errors wrap the operation with `%w`
and retain the underlying cause. An existing calendar's project is immutable
through this writer: a cross-project ID collision returns `ErrNoResourceCalendar`
and an empty result without updating any calendar field or timestamp.

Rejecting lower-level project reassignment is an intentional, approved behavior
restriction, matching the existing App ownership policy. The UPSERT guard rejects
the entire foreign update, not just its `project_id` assignment. A scoped read
alone would not protect ownership if the preceding write could reassign it.
An explicit transfer operation would require its own contract and review; it is
not part of an ordinary save. The App checks remain unchanged.

Public signatures, JSON tags, supplied/generated IDs, capacity/name defaults,
nil-map/slice normalization, `time.Time` fields and timestamp storage are
unchanged. Updates retain creation timestamps. Existing timestamp parsing policy
is not tightened. No schema migration, dependency, retry loop, production fault
hook or frontend change is needed. Reverting requires no migration but restores
the ambiguous saves and lower-level reassignment behavior.

### Regression evidence

`internal/db/resource_calendar_atomicity_test.go` contains four tests with
60 subcases:

- Forty-four combine plaintext/encrypted storage, creation/update and eleven
  faults: begin/write/SELECT denial, scan conversion, each of four JSON fields,
  missing row, changed project during readback, and commit rejection. Authorizers
  and commit hooks assert one hit and typed SQLite errors; JSON cases assert the
  decoding error type. Disposable triggers simulate post-write readback defects.
- Every failure returns an empty calendar and leaves the native connection out
  of a transaction before close. Raw snapshots compare every stored calendar
  column after read-only reopen without migrations. Explicit retry persists
  exactly one record with the expected fields, retained historical creation
  timestamp on updates, and matching text/integer timestamp pairs. Another
  read-only reopen checks the committed record.
- Two same-file project-collision cases assert the missing-scoped-record
  sentinel, zero calendar UPDATE callbacks, connection cleanup and unchanged
  raw stored rows. Zero callbacks distinguishes rejecting the write from
  updating and subsequently rolling back a foreign row.
- Six compatibility cases preserve caller IDs, zero/negative capacity defaults,
  positive capacity, name fallback and non-null empty JSON maps/slices, including
  their stored representation. Eight input cases preserve rejection of NaN and
  infinity in weekly/override maps without persisting a row. Generated IDs,
  nonempty maps/tags, negative offsets, Unicode notes and timestamps are checked
  by the failure/retry matrix. Existing App scope/CRUD tests remain unchanged.

Before the repair, 34 failure-matrix cases and both project-collision cases
failed. Write/commit rejection and a deleted new row already prevented durable
mutation; they are retained as negative controls. All 60 cases pass after the
repair, including three shuffled repetitions and a focused race-enabled run.

Three isolated Go source overlays were rejected without modifying working
source: removing only the UPSERT ownership guard failed the zero-update check;
removing scoped readback failed the changed-project case; ignoring commit errors
failed the commit case. These checks demonstrate sensitivity to specific wrong
implementations, not proof against all defects.

### Reproduction and limits

```sh
go test ./internal/db -run '^TestSaveResourceCalendar' -count=1 -timeout=60s
go test ./internal/db -run '^TestSaveResourceCalendar' -count=3 -shuffle=20261021 -timeout=60s
go test -race ./internal/db -run '^TestSaveResourceCalendar' -count=1 -timeout=60s
go test ./internal/db ./internal/sqlitedriver -count=1 -timeout=120s
go test . -run '^TestResourceCalendar|^TestAppMethodsRequireOpenProject$/SaveResourceCalendar' -count=1 -timeout=60s
make verify race build lint-go code-map-current release-scope
make coverage-ledger-current coverage-ledger-drift
go vet . ./internal/... ./tools/...
go mod verify
govulncheck -show verbose . ./internal/...
npm --prefix frontend run lint
npm --prefix frontend audit
make frontend-smoke
reuse lint
```

Focused database/driver, App and race checks, Go vet/lint, module verification
and the production build passed locally on macOS arm64 with Go 1.27.3 and CGO.
Database statement coverage measured 78.4%; coverage is diagnostic, not proof.
The Go vulnerability scan found no affected code or imported packages; its
module-only GO-2026-5932 advisory concerns unused `x/crypto/openpgp`.

`make verify` passed the ordinary Go suite, all 631 frontend tests, type checking,
frontend build budgets and repository contract checks. The unchanged repository
`make race` passed all 45 packages with `-p=1`, `-count=1`, `webkit2_41` and the
default ten-minute per-package deadline: root completed in 418.967 seconds and
accounts in 405.528 seconds. No assertion, timeout or cryptographic parameter
was weakened. Independent read-only review approved the change with no findings
and separately passed focused database, race, App, formatting and whitespace
checks.

Frontend lint/audit (zero npm advisories), runtime smoke, strict ad-hoc signature
verification, code-map/release-scope checks and REUSE (984/984 files) passed.
Coverage-ledger inventory and live drift checks passed all 46 measured package
headings. Existing Wails `Not found: time.Time`, macOS 13-object/11-link-target
warnings and six jsdom canvas diagnostics remain; chart rendering is not claimed
verified by those jsdom tests.

Failed-COMMIT cleanup depends on the existing patched driver, not just deferred
`sql.Tx.Rollback`; see [driver evidence](sqlcipher-cleanup-corrections.md#failed-commit-cleanup-evidence). Commit-hook
rejection does not establish every COMMIT/rollback I/O outcome. A response lost
after successful commit still requires separate idempotency/recovery design.
No crash/power-loss, interactive native GUI, other-platform or full-release
qualification is claimed.

Lessons: guard ownership in the mutation itself, test raw persisted state as
well as decoded values, verify cleanup before close can conceal a leaked
transaction, and check that tests reject omitted guards rather than merely
accepting rollback later. Explicit transfer semantics remain out of scope.

## Sprint save atomicity

### Defect and approved contract

`SaveSprint` previously auto-committed an upsert before calling `GetSprint`.
A readback failure therefore returned an empty record despite a saved change.
An explicit retry of a blank-ID draft generated another ID and inserted a
duplicate. A real-driver SELECT denial reproduced both duplicate creation and
a rejected update that durably changed the goal, capacity and completion status,
in plaintext and encrypted files. No production incident is asserted.

The approved repair puts the upsert and project-scoped readback in one
transaction. A readback failure rolls back; success follows commit. Save and
Get share the same scoped query, but Save reads through its transaction rather
than a separate pooled connection. Errors wrap the original cause with operation
context. Failed-COMMIT cleanup remains the existing driver's responsibility;
see [driver evidence](sqlcipher-cleanup-corrections.md#failed-commit-cleanup-evidence).

IDs, project ownership, default planning status, editable fields, database-owned
creation timestamps, public signatures, JSON and schemas are unchanged. No
migration, dependency, production fault hook or retry loop was added. The
intentional change is that a failed readback no longer leaves its write saved.

### Regression evidence

`internal/agile/sprint_atomicity_test.go` has 16 real-driver cases combining
creation/update, plaintext/encrypted files, and Begin/write/readback/commit
failures. A single physical pooled connection receives the authorizer or commit
hook, then is released before the Store call to avoid a pool self-deadlock.
Each fault must fire once, return the expected SQLite error and an empty record,
and leave no active transaction. Commit rejection checks the specific extended
commit-hook error code.

The writer is closed before a read-only connection checks the exact prior record
set without migrations. Explicit retry must persist exactly one returned record
with the draft's fields and Store-owned project ID. Its identity and exact fields
must survive another close/read-only reopen. Update fixtures seed a fixed
historical `created_at` so a rewrite cannot hide behind millisecond precision;
failure, retry and reopened-state assertions preserve that timestamp.

Against the old implementation, Begin/readback cases failed in both storage
modes; readback rejection left hidden mutations and duplicate new-record retries.
Write/commit rejection already prevented writes and remained passing. Existing
tests retain default-status, entropy-error and cross-project protections.

### Local validation and limits

On 2026-10-18, macOS arm64, Go 1.27.2 with CGO enabled, these checks passed:

```sh
go test ./internal/agile -count=1 -timeout=60s
go test -race ./internal/agile ./internal/sqlitedriver -count=1 -timeout=120s
go test -race . -run '^TestSprint_AppMethodsRoundTrip$' -count=1 -timeout=120s
go test . ./internal/... ./tools/... -count=1 -timeout=180s
go test -race ./internal/agile -run 'Test(SaveSprint|GetSprint|StoreMethodsLeaveAnotherProjectsRowsAlone)' -count=5 -shuffle=20261018 -timeout=60s
go vet . ./internal/... ./tools/...
make lint-go code-map-current release-scope
go mod verify
make build
codesign --verify --deep --strict build/bin/gopmgr.app
npm --prefix frontend run check
make frontend-smoke
make coverage-ledger-current coverage-ledger-drift
reuse lint
git diff --check
```

All 46 package coverage headings matched the live ledger check. REUSE passed
for 974 files, and the complete task diff passed secret scanning. The independent
read-only review reported no findings and passed focused race tests for all 16
fault cases, defaults and project isolation. The independent plan critique led to
the deterministic historical-timestamp fixture described above.

The production build retains the existing Wails `Not found: time.Time` diagnostics
and macOS 13-object/11-link-target warning. An ad-hoc signature is not distribution
signing or notarization evidence. No new native GUI, installer, other-platform,
crash/power-loss or full-release qualification is claimed.

This repair does not make multiple saves in sprint activation one transaction,
or solve a response lost after successful commit. These injected failures do not
establish the outcome of every disk, rollback or COMMIT I/O failure.

The separate new-sprint desktop boundary remains broken: `SprintList.openNew`
seeds `created_at: ''`, and its save forwards that string into Go `time.Time`.
The current JSON decoder rejects it before the App method runs; Wails uses that
decoder in `BoundMethod.ParseArgs`. The typed Go App round-trip and mocked
frontend tests do not exercise that wire boundary. This is source/decoder
evidence, not a new native GUI trial, and no frontend fix is included here.

Lessons: test after the write, not only before it; use fixed historical values
when timestamp precision could mask a rewrite; and distinguish typed App tests
from real wire decoding. A green persistence test does not prove the desktop
form can submit its payload.

## Stakeholder save atomicity

### Defect and contract

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

### Regression evidence

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

### Verification and limits

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
`Commit`. See [driver evidence](sqlcipher-cleanup-corrections.md#failed-commit-cleanup-evidence). A rejected commit
hook does not establish the result of every COMMIT/rollback I/O failure. No
crash/power-loss, interactive native GUI, other-platform or full-release
qualification is claimed. A response lost after successful commit still needs
separate recovery/idempotency design.

Lessons: test failures after writing, inspect raw state rather than only a
normalized DTO, check transaction cleanup before closing can conceal it, and
use repository-standard budgets for whole cryptographic account suites.
The separate stakeholder foreground-error forwarding defect remains deferred.

## Work-item save atomicity

### Defect and contract

`SaveWorkItem` previously auto-committed its upsert before reading the stored
record. A readback error returned an empty record despite a durable mutation.
For a new blank-ID draft, an explicit retry then generated another ID and
inserted a duplicate. A real-driver SELECT denial reproduced two durable rows
after one rejected save and its retry; no production incident is asserted.

The approved contract is one transaction for the upsert and project-scoped
readback. A readback failure rolls back; success is returned only after commit.
The readback uses the same scoped query as `GetWorkItem`, on the transaction
rather than a separate pooled connection. Errors retain their underlying type
through wrapping. The existing driver owns failed-COMMIT cleanup, including
cases where SQLite has already rolled back; see [driver evidence](sqlcipher-cleanup-corrections.md#failed-commit-cleanup-evidence).

This preserves IDs, defaults, creation/update/closure rules, project ownership,
Wails signatures, JSON and database schemas. No migration or new dependency is
needed. It deliberately changes failed-readback behavior from a hidden committed
write to an unsuccessful operation with no saved change.

### Regression evidence

`internal/agile/work_item_atomicity_test.go` uses fresh private file-backed
projects in both plaintext and SQLCipher-encrypted modes, not mocked storage.
Its 16 cases combine creation/update with failures at Begin, write, readback
and commit. A single physical pooled connection receives the actual driver
authorizer or commit hook, which is cleared before verification.

Assertions require the injected failure to fire once, an error with the expected
SQLite code, an empty returned record, and no active transaction. After closing
the writer, a read-only connection checks the entire previously committed
record set without migrations. Explicit retry must persist exactly one returned
record with all draft fields, enforced project ownership and stable creation
identity; that exact record must survive another close/read-only reopen.

Before the repair, Begin/readback cases failed in both storage modes. In
particular, readback failures retained the write and new-record retries produced
two rows. Write/commit rejection already prevented writes and remained passing.
Existing CRUD tests separately retain defaults, closure and cross-project guards.

### Local validation and limits

Verified on macOS arm64 with Go 1.27.2 and CGO enabled:

```sh
go test ./internal/agile -count=1 -timeout=60s
go test -race ./internal/agile ./internal/sqlitedriver -count=1 -timeout=120s
go test -race . -run 'Test(WorkItemTimestampJSON|ColumnAndWorkItem_AppMethodsRoundTrip)$' -count=1 -timeout=120s
go test . ./internal/... ./tools/... -count=1 -timeout=180s
go vet . ./internal/... ./tools/...
go test -race ./internal/agile -run 'Test(SaveWorkItem|GetWorkItem|StoreMethodsLeaveAnotherProjectsRowsAlone)' -count=5 -shuffle=20261018 -timeout=60s
make lint-go coverage-ledger-current coverage-ledger-drift
go mod verify
make build code-map-current license-check release-scope
npm --prefix frontend run check
make frontend-smoke
codesign --verify --deep --strict build/bin/gopmgr.app
```

The five shuffled race repetitions passed. The independent read-only review
found no introduced defects and independently passed focused atomicity,
defaults, closure and project-isolation tests. All 46 package coverage headings
matched the live ledger check. The complete task diff passed secret scanning.

The existing Wails `Not found: time.Time` diagnostics and macOS 13-object/11-link-
target warning remain in build output; the selected App timestamp/round-trip
tests and production build passed. An ad-hoc signature is not distribution
signing or notarization evidence.

No new native GUI, installer, Linux/Windows, crash/power-loss or full-release
qualification is claimed. In particular, losing a response **after a successful
commit** can still leave a blank-ID caller uncertain and make a retry duplicate
the creation. That is a separate acknowledgement/idempotency design problem,
not solved by moving readback into the transaction. These injected failures do
not establish the outcome of every disk, rollback or COMMIT I/O failure.

The useful prevention lesson is to test the boundary *after* a successful write:
closed-database and pre-write contention tests cannot detect a readback that
reports failure after an auto-committed mutation.
