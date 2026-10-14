<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Explicit ROLLBACK cleanup evidence

## Confirmed defect and correction

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

## Acceptance criteria

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

## Verification

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
[COMMIT cleanup evidence](sqlcipher-commit-cleanup.md). Subsequent
[full account verification](account-race-assurance.md) at source revision
`3f26b76` closes the current local race-evidence gap without changing timeouts,
cryptography or tests; the historical cause remains unproven. Neither bounded
repair establishes that the complete release gate passes.

## Separate manual-SQL boundary

This correction applies to `sql.Tx.Rollback` through the driver transaction,
not arbitrary SQL sent through `Conn.Exec`. Final caller inspection at `f233f54`
found that `internal/users/account_status.go:inWriteTx` owned a manual BEGIN
IMMEDIATE, ignored its raw ROLLBACK error, then returned the connection to the
pool. This was an analogous gap, not reproduced or repaired by the driver work.
The subsequent [account transaction repair](account-transaction-cleanup.md)
confirms and fixes that helper with account-state and audit regressions. Do not
infer that arbitrary manual transactions are safe from the driver tests.

## Lessons

- Error return and logical Tx completion do not prove native rollback.
- Connection identity and rejected-row visibility must be checked before
  teardown or another operation can accidentally make a negative test pass.
- One shared rollback rule avoids divergence between explicit and failed-COMMIT
  cleanup while preserving error precedence.
- An error after automatic rollback must not discard a healthy connection.
