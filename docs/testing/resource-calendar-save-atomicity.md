<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Resource-calendar save atomicity

## Defect and approved contract

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

## Regression evidence

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

## Reproduction and limits

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
`sql.Tx.Rollback`; see [driver evidence](sqlcipher-commit-cleanup.md). Commit-hook
rejection does not establish every COMMIT/rollback I/O outcome. A response lost
after successful commit still requires separate idempotency/recovery design.
No crash/power-loss, interactive native GUI, other-platform or full-release
qualification is claimed.

Lessons: guard ownership in the mutation itself, test raw persisted state as
well as decoded values, verify cleanup before close can conceal a leaked
transaction, and check that tests reject omitted guards rather than merely
accepting rollback later. Explicit transfer semantics remain out of scope.
