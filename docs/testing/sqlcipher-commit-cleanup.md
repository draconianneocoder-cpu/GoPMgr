<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Failed-COMMIT cleanup evidence

## Problem and repair

The original v4.4.2 Go driver attempted ROLLBACK only for SQLITE_BUSY. Go's
`database/sql` finishes a Tx before returning a failed commit, making deferred
`Tx.Rollback` ineffective. The new real-file deferred-constraint regression
failed in both plaintext and keyed cases on the original driver: the same
connection was still in a transaction and exposed one rejected child row.
No user database corruption was observed; this was a controlled fixture.

The local Go patch attempts cleanup for every COMMIT error, preserves the exact
original error, and invalidates connections only when rollback fails with the
transaction still active. Invalid connections refuse preparation, statement
execution and existing cursor stepping and cannot return to the pool.
Successful cleanup retains the same healthy connection, including when a
commit hook has already rolled back the transaction. Manual BEGIN remains usable.

Failure injection also reproduced locked subsequent writes after connection
discard: the old statement Close refused finalization once the Go database
pointer was cleared. Driver-owned statements now finalize even after close_v2
and retain callback handles through final native rollback. This follows
[SQLite's deferred-close contract](https://www.sqlite.org/c3ref/close.html) and
[statement finalization contract](https://www.sqlite.org/c3ref/finalize.html).

## Acceptance evidence

| Invariant | Regression |
| --- | --- |
| Rejected rows cannot contaminate subsequent operations | Plaintext/keyed deferred FK failure, precise original extended error, same physical connection and autocommit, zero failed rows, next valid commit and file reopen |
| Automatic rollback does not poison a healthy connection | Commit-hook rejection, original extended code, healthy reuse and subsequent durable commit |
| Cleanup failure cannot leak through pooling or held handles | Denied ROLLBACK, separate pool-only and pinned cases, fresh/prepared operation refusal, already-open cursor refusal, different Go driver connection identity and successful new writer |
| Deferred disposal releases locks without losing callbacks | Zero/two retained statements, rollback hook exactly once after final destruction, repeated Close idempotence, no failed rows and new writer success |
| Previous behavior remains supported | Deterministic DELETE-journal reader blocks COMMIT with SQLITE_BUSY; failed row absent, next write succeeds; manual BEGIN/COMMIT persists |
| Published driver and file contracts remain unchanged | Whole pristine runtime-tree reconstruction, original Go source hash, bidirectional encrypted-file interchange against isolated unmodified v4.4.2 helper |

The tests are in `internal/sqlitedriver/commit_cleanup_test.go`; the encrypted
validation script already runs this package. No sleeps, weakened assertions,
retry loops, toolchain upgrades, schema migration, or new dependencies were added.

## Verification and limits

Local macOS verification passed the full driver package (including pristine
provenance and encrypted interchange), the complete uncached Go suite, and 20
shuffled race-enabled iterations of the five focused tests below. There were
zero failures in those 20 iterations; this is evidence, not proof of no flakiness.
The final test-only context-interface/cleanup adjustments passed another three
race-enabled repetitions with seed `20261016`. Independent review passed the
full driver package and three repetitions with seed `20261014`, then approved
the documentation corrections. The encrypted database gate, Go vet/lint,
module verification, production build, code-map and coverage-ledger checks,
configuration parsing, macOS packaging layout test, and REUSE (944/944) passed.
The build retains the existing macOS 13-object/11-link-target warning.

The broader race run with seed `20261015` passed `sqlitedriver`, `db`, `catalog`
and `deletionlog`, but the full `users` suite reached Go's default ten-minute
deadline. At timeout, `TestHandOverToAStandbyReportsTheKeyTheyHold` had been
running two seconds, with runnable Argon2 password-hashing workers in setup,
not a database-lock stack. No race was reported before timeout. This is incomplete
full-account race assurance, not a green result. The timeout and cryptographic
parameters were not increased or weakened. The exact interrupted test passed
alone under race detection in 4.235 seconds (seed `20261017`). The three account
concurrency/failed-COMMIT regressions passed in 5.100 seconds (seed `20261018`).
These focused results do not replace or claim completion of the full account
race suite; its runtime-budget investigation remains follow-up work.
An independent verification-gate review accepted this limitation as non-blocking
for the bounded repair, not for release. The stack and isolated pass support,
but do not prove, an execution-budget cause or baseline runtime equivalence.
The full release gate requires a successful broad race run and is not claimed
passed here.

```sh
go test -count=1 ./internal/sqlitedriver
go test -race -count=20 -shuffle=20261014 -run 'Test(FailedCommitCleanup|ManualTransactionRemainsUsable|BusyCommitRollsBack|DiscardPreservesRollbackHookUntilFinalStatement|FinalizeAfterConnectionCloseReleasesStatement)$' ./internal/sqlitedriver
go test -count=1 . ./internal/...
bash scripts/validate-encrypted-db.sh
go vet . ./internal/...
make lint-go build code-map-current coverage-ledger-current coverage-ledger-drift config-check release-scope
go mod verify
reuse lint
bash scripts/package-macos_test.sh
go test -race -count=1 -shuffle=20261015 ./internal/sqlitedriver ./internal/db ./internal/users ./internal/catalog ./internal/deletionlog # users timed out; other four passed
go test -race -count=1 -shuffle=20261017 -run 'TestHandOverToAStandbyReportsTheKeyTheyHold$' ./internal/users
go test -race -count=1 -shuffle=20261018 -run 'TestCreateAccountAs(ConcurrentFirstAccounts|ConcurrentCaseVariants|RemovesFoldersWhenCommitFails)$' ./internal/users
```

The baseline full suite had one sandbox-only invalid-UTF-8 folder rename failure;
the same test passed outside the sandbox before production edits.
The race detector checks Go ownership, not the C amalgamation. No native
Windows/Linux, power-loss, or filesystem I/O-failure durability claim is made.
Outstanding external Backup/BLOB/native-statement lifetimes are outside this
bounded fix and must end before connection close. The later
[explicit ROLLBACK repair](sqlcipher-rollback-cleanup.md) closes the follow-up
connection-invalidation gap; the COMMIT regressions remain unchanged.
Public cursor execution resets errors before
finalization; the non-OK native finalization fallback is source-reviewed, not
claimed dynamically exercised through that interface.

## Lessons

- A finished Go Tx does not prove the native transaction is finished.
- Pool eviction alone does not protect a retained connection or cursor.
- Native close_v2 can defer transaction rollback until the final statement;
  callback references must remain alive until then.
- Test lock release and persisted state, not just the error or pool size.
- Reconstruct and hash the whole original dependency for compatibility tests;
  reversing only the older C correction would silently test the modified driver.
