<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Sprint save atomicity

## Defect and approved contract

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
see [driver evidence](sqlcipher-commit-cleanup.md).

IDs, project ownership, default planning status, editable fields, database-owned
creation timestamps, public signatures, JSON and schemas are unchanged. No
migration, dependency, production fault hook or retry loop was added. The
intentional change is that a failed readback no longer leaves its write saved.

## Regression evidence

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

## Local validation and limits

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
