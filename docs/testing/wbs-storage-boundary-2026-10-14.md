<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# WBS storage boundary evidence

Scope: explicit internal storage initialization and revision-checked writes;
no application/session/renderer activation, upgrade or source association.
Baseline main `70a3e51` was clean with no staged or unrelated public work.

## Acceptance and counterexamples

| Requirement | Test evidence |
| --- | --- |
| One winner for stale concurrent saves | Independent database handles start at revision 1; exactly one commits and one conflicts; valid audit chain has only one additional event |
| Retried writes do not commit twice | Replay after a newer save and SQLCipher/plaintext reopen returns original revision/audit ID; current content remains newer |
| Identity reuse is refused | Same request ID with changed actor or title fails; copy/lifetime mismatch fails; closed store cannot save |
| Atomic content/revision/receipt/audit | SQLite triggers fail audit/receipt insertion and pruning at save257; no advanced chart or partial audit survives |
| Bounded receipts | Latest256 retained; replay does not refresh receipt1; expired create request conflicts rather than recreating |
| Fail-closed initialization | Two initializers preserve one copy ID; missing singleton, inconsistent revisions and partial schema fail |
| Semantic request identity | Empty dictionary/arrays and omitted equivalents replay; exact title/Unicode content persists |
| Protected generic paths | Submitted v2 kind/envelope and stored-record membership block generic update/retype/delete |
| Boundary safety | Cancelled create writes no chart; canonical max uint64 overflow refuses; lifetime Close leaves shared pool usable |
| Failed commit cleanup | Deferred foreign-key constraint forces COMMIT failure; discarded connection leaves no chart, receipt or audit mutation |

The commit-cleanup test initially failed three race-enabled runs: after the
reported failure, the pool still observed an uncommitted chart. Source inspection
confirmed `SQLiteTx.Commit` in the pinned driver rolls back only `SQLITE_BUSY`.
Go already marks `sql.Tx` finished, so deferred `Rollback` cannot rescue another
failure. The WBS writer now retains a dedicated `sql.Conn` through commit and
discards it via the documented `Raw`/`driver.ErrBadConn` mechanism on failure.
The unchanged regression passes after correction. General driver cleanup for
other database writers is a directly discovered follow-up, not a claimed fix.

## Recorded local verification

- Baseline `go test -count=1 ./internal/wbs ./internal/db` and the application
  WBS commit/layout round-trip test passed.
- Initial focused WBS storage tests passed; ten shuffled race-enabled runs
  with seed20261014 passed before the deferred-constraint counterexample.
- Expanded post-correction race suite: seed20261015, three runs, passed.
- Final nine-test storage suite: seed20261017, three race-enabled runs, passed;
  final database regression suite passed at 77.6% statement coverage after
  dead-assignment cleanup. `make lint-go` passed with zero issues.
- Final `go vet . ./internal/...`, map and test-inventory checks passed.
- Production `make build` passed, retaining the existing macOS13-object versus
  macOS11-link-target warning.
- Broad Go regression suite passed before the dedicated-connection correction;
  final coverage-ledger drift reran the complete default Go coverage suites and
  DuckDB analytics successfully, with all 46 package headings matching.
- Static analysis, code-map generation/check, test inventory, configuration,
  release-scope and REUSE passed during implementation.

## Evidence limits and lessons

Internal caller-provided actor/lifetime IDs are a trusted boundary, not
authentication. Account incarnation, source incarnation, restored-copy reset,
old-application protection, runtime conflict UX and whole-file mutation paths
must precede production activation. Source associations deliberately fail closed.
Current native evidence is local macOS arm64; Windows/Linux runtime, bridge
transport, power-loss durability and full release gates were not exercised.

Keep replay lookup ahead of today's revision check; never label current content
as the original receipt's snapshot. Failed COMMIT is a resource-lifetime case,
not just an error return. Test it with a real deferred constraint rather than a
mock, and verify subsequent pooled operations as well as returned errors.
