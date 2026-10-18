<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Work-item save atomicity

## Defect and contract

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
cases where SQLite has already rolled back; see [driver evidence](sqlcipher-commit-cleanup.md).

This preserves IDs, defaults, creation/update/closure rules, project ownership,
Wails signatures, JSON and database schemas. No migration or new dependency is
needed. It deliberately changes failed-readback behavior from a hidden committed
write to an unsuccessful operation with no saved change.

## Regression evidence

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

## Local validation and limits

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
