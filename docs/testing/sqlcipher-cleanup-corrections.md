<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# SQLCipher cleanup corrections

Three corrections to SQLCipher connection cleanup after failed transactions.

Each section below is a record kept as written, with its headings one
level lower and its links updated; nothing else changed.

## What this file holds

- [Failed-COMMIT cleanup evidence](#failed-commit-cleanup-evidence): formerly `docs/testing/sqlcipher-commit-cleanup.md`, undated, from commit `9ab5d91`.
- [Explicit ROLLBACK cleanup evidence](#explicit-rollback-cleanup-evidence): formerly `docs/testing/sqlcipher-rollback-cleanup.md`, undated, from commit `9ab5d91`.
- [SQLCipher WAL-reset correction evidence](#sqlcipher-wal-reset-correction-evidence): formerly `docs/testing/sqlcipher-wal-reset.md`, 2026-11-10, from commit `9ab5d91`.

## Failed-COMMIT cleanup evidence

### Problem and repair

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

### Acceptance evidence

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

### Verification and limits

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
Those focused results did not establish full-account race completion at that
time. Subsequent [full account race verification](account-race-assurance.md)
at source revision `3f26b76` passed both the complete account suite and the
44-package release race scope under unchanged default deadlines. The historical
timeout's cause remains unproven, and the later report records its local
environment and one pre-existing APFS exclusion.
An independent verification-gate review accepted this limitation as non-blocking
for the bounded repair, not for release. The stack and isolated pass support,
but do not prove, an execution-budget cause or baseline runtime equivalence.
The complete release gate is not claimed passed by either bounded report.

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
[explicit ROLLBACK repair](#explicit-rollback-cleanup-evidence) closes the follow-up
connection-invalidation gap; the COMMIT regressions remain unchanged.
Public cursor execution resets errors before
finalization; the non-OK native finalization fallback is source-reviewed, not
claimed dynamically exercised through that interface.

### Lessons

- A finished Go Tx does not prove the native transaction is finished.
- Pool eviction alone does not protect a retained connection or cursor.
- Native close_v2 can defer transaction rollback until the final statement;
  callback references must remain alive until then.
- Test lock release and persisted state, not just the error or pool size.
- Reconstruct and hash the whole original dependency for compatibility tests;
  reversing only the older C correction would silently test the modified driver.

## Explicit ROLLBACK cleanup evidence

### Confirmed defect and correction

At `85baae3`, SQLCipher's Go driver returned an explicit ROLLBACK error without
invalidating the connection. `database/sql` marks the logical Tx finished
before returning that error, so subsequent Rollback calls cannot repair it.

The new real-file regression failed in all four denial cases: plaintext/keyed,
pooled/pinned. The native transaction stayed active, the pooled connection was
reused, and reads returned two rows instead of the one committed row. Retained
statement execution and cursor stepping also remained possible. These are
controlled fixtures, not evidence that a user's database was corrupted.

The driver's `SQLiteTx.Rollback` now owns the existing invalidation rule: only an error with a live
handle and an active native transaction sets the sticky unsafe flag. Failed
Commit delegates cleanup to Rollback but still returns its original COMMIT
error. Rollback retains its own original error. There is no retry, schema/key
change, new public interface, new goroutine, or dependency upgrade.

### Acceptance criteria

| Criterion | Evidence required |
| --- | --- |
| Denial cannot leak through pooling | Exact ErrAuth; next driver connection identity differs before any query or forced disposal; native autocommit restored and failed row absent |
| Held handles cannot execute after denial | Invalidity checked while pinned; retained statement Exec/Query and cursor Next return ErrBadConn; public connection read refused |
| Healthy connections remain reusable | Successful rollback and trigger-driven automatic rollback keep the original valid, autocommit connection; automatic rollback's original ErrError remains visible |
| Persisted contents remain exact | Later valid transaction commits; reopening returns only IDs `[1, 3]`, never rejected ID 2 |
| Existing contracts remain intact | Existing COMMIT/manual-BEGIN/callback/BUSY regressions plus unchanged pristine source hashes and encrypted-file interchange |

`internal/sqlitedriver/rollback_cleanup_test.go` contains eight scenarios.
The encrypted database gate already runs the driver package. Tests inspect
the next connection before querying, do not set expiry or cancel it, and never
force ErrBadConn from their identity inspection. Closing a fixture cannot hide
the original rejected-row visibility. Positive cases prevent over-invalidation.

### Verification

All five acceptance criteria passed on local macOS. The full driver suite
passed, including unchanged pristine Go/runtime hashes and encrypted interchange.
The combined cleanup regressions passed 30 shuffled race-enabled iterations
(seed `20261019`), with zero observed failures. The complete uncached normal Go
suite and broader `db`/`catalog`/`deletionlog` race tests passed. Three affected
account concurrency/failed-COMMIT race cases passed with seed `20261021`.

Independent adversarial review approved the repair and independently passed
focused regressions/provenance, three shuffled race iterations (seed `20261021`),
and unmodified-driver encrypted interchange. Its pool/pinned test-oracle
requirements were incorporated before implementation.

The encrypted database gate, Go vet/lint (zero issues), production Wails build,
dependency verification, code-map, test-file inventory, seven configuration
checks, release-scope check, and REUSE (946/946) passed. The production build
retains the existing macOS 13-object/11-link-target warning. Coverage-ledger
verification matched all 46 package entries and accounted for all 49 headings.

```sh
go test -count=1 ./internal/sqlitedriver
go test -race -count=30 -shuffle=20261019 -run '^Test(RollbackCleanup|FailedCommitCleanup|DiscardPreservesRollbackHookUntilFinalStatement|BusyCommitRollsBack|ManualTransactionRemainsUsable|FinalizeAfterConnectionCloseReleasesStatement)' ./internal/sqlitedriver
go test -count=1 . ./internal/...
go test -race -count=1 -shuffle=20261020 ./internal/db ./internal/catalog ./internal/deletionlog
go test -race -count=1 -shuffle=20261021 -run '^TestCreateAccountAs(ConcurrentFirstAccounts|ConcurrentCaseVariants|RemovesFoldersWhenCommitFails)$' ./internal/users
bash scripts/validate-encrypted-db.sh
go vet . ./internal/...
make lint-go build code-map-current coverage-ledger-current config-check release-scope
make coverage-ledger-drift
go mod verify
reuse lint
```

Native C is not instrumented by Go's race detector. No native Windows/Linux,
power-loss or native I/O-error fault injection is claimed. Authorizer denial
exposes the supported rollback-error mechanism deterministically. Existing
external Backup/BLOB/native-statement lifetime limits remain unchanged.
Cancellation-specific behavior is not newly changed: the installed Go Tx
implementation already forces disposal on cancellation because this driver
has no SessionResetter. No cancellation assurance beyond that source review
is claimed here.

The prior full account-store race timeout is preserved in
[COMMIT cleanup evidence](#failed-commit-cleanup-evidence). Subsequent
[full account verification](account-race-assurance.md) at source revision
`3f26b76` closes the current local race-evidence gap without changing timeouts,
cryptography or tests; the historical cause remains unproven. Neither bounded
repair establishes that the complete release gate passes.

### Separate manual-SQL boundary

This correction applies to `sql.Tx.Rollback` through the driver transaction,
not arbitrary SQL sent through `Conn.Exec`. Final caller inspection at `f233f54`
found that `internal/users/account_status.go:inWriteTx` owned a manual BEGIN
IMMEDIATE, ignored its raw ROLLBACK error, then returned the connection to the
pool. This was an analogous gap, not reproduced or repaired by the driver work.
The subsequent [account transaction repair](account-transaction-cleanup.md)
confirms and fixes that helper with account-state and audit regressions. Do not
infer that arbitrary manual transactions are safe from the driver tests.

### Lessons

- Error return and logical Tx completion do not prove native rollback.
- Connection identity and rejected-row visibility must be checked before
  teardown or another operation can accidentally make a negative test pass.
- One shared rollback rule avoids divergence between explicit and failed-COMMIT
  cleanup while preserving error precedence.
- An error after automatic rollback must not discard a healthy connection.

## SQLCipher WAL-reset correction evidence

> **Superseded on 2026-11-10.** The engine was upgraded to SQLCipher 4.20.0
> (SQLite 3.54.1), which contains this correction upstream, so GoPMgr no
> longer carries the backport. The test described below now runs against
> the vendored engine, with a control that removes SQLite's own check. See
> [the upgrade evidence](sqlcipher-4.20-upgrade.md). The rest of this page
> is the record of the backport as it was.

### Risk and correction

The embedded SQLite 3.33.0 core predates SQLite's WAL-reset correction. During
a PASSIVE checkpoint, another connection can reset and reuse the WAL after the
checkpoint caches its header but before it takes read-lock slot zero. The old
checkpoint can then backfill frames or advance bookkeeping using a stale WAL
generation.

GoPMgr backports SQLite commit
`fe57e14b49f9189b56da8233ab3415ce5ff6b1ff`. Once the checkpoint owns read-lock
slot zero, it compares the live WAL salt with the cached header. A mismatch
skips all backfill I/O and bookkeeping for that stale generation while leaving
lock and iterator cleanup unchanged. This is a confirmed affected-source
condition, not evidence that a GoPMgr user database was corrupted.

### Test design

`TestWALResetCheckpointPreservesCommittedData` reconstructs the exact published
v4.4.2 amalgamation and refuses to run unless its SHA-256 matches the recorded
source. A temporary-only fault hook pauses the checkpoint at the upstream
interleaving point. A second real connection commits the write that resets the
WAL. The pristine plaintext and keyed controls must report stale-generation
backfill with their dedicated exit code; compile failures, crashes, and timeouts
do not count as defect detection. The corrected cases must preserve exact row
contents through later WAL growth, a fresh checkpoint, integrity checks, close,
and reopen.

`TestEncryptedFilesInteroperateWithUnmodifiedDriver` builds a separate helper
module with `GOWORK=off`, verifies the unmodified v4.4.2 amalgamation hash, and
tests encrypted files in both creation directions. Both drivers must read and
append the exact rows, reject a wrong key, pass integrity checks, and retain an
encrypted header. `TestVendoredSQLCipherProvenance` separately pins the runtime
inventory, tree hashes, and complete binary-notice artifact.

Run the ordinary gate with:

```sh
go test -count=1 ./internal/sqlitedriver
make check-encrypted-db
```

AddressSanitizer is an optional local diagnostic for the native harness on a
compiler that supports it:

```sh
GOPMGR_SQLCIPHER_ASAN=1 go test -count=1 ./internal/sqlitedriver \
  -run '^TestWALResetCheckpointPreservesCommittedData$'
```

### Evidence boundary

The deterministic harness exercises a real two-connection reset schedule on
the current native host. It does not prove absence of every SQLite defect,
power-loss durability, or Windows locking behavior. The Go race detector does
not instrument the C amalgamation, so the native causal control is the relevant
proof for this interleaving. Windows native execution and platform power-loss
testing remain separate release evidence.
