<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Account transaction cleanup

## Defect and correction

The account writer uses manual `BEGIN IMMEDIATE` on a pinned `sql.Conn`, not
the driver's transaction API. At `f233f54`, its deferred raw ROLLBACK ignored
failure before returning the connection to the pool. Driver Tx cleanup could
not protect that path.

The real, file-backed account regression confirmed four rejected operations
left the native transaction active and exposed `bob.disabled=1` to the next
pooled reader. Three also exposed the rejected audit row. These controlled
authorizer failures establish the mechanism, not corruption of user databases.
Successful rollback controls passed against the original implementation.

`inWriteTx` now discards through `Conn.Raw` with `driver.ErrBadConn` when raw
ROLLBACK fails, before deferred connection release. Original operation and
COMMIT errors remain discoverable through `errors.Is`/`errors.As`; contextual
cleanup errors are joined rather than hidden. Panic unwinding still attempts
cleanup and preserves the original panic. Successful rollback retains the
healthy connection. No retry, new dependency, schema, key, authorization or
public API change is involved.

Unlike the driver Tx rule, the account helper conservatively discards after
any raw rollback error, including an already-rolled-back transaction. This
avoids coupling account storage to native state inspection. The extra close
on this exceptional path is intentional, not a promise that all errors mean
the database is damaged. Raw BEGIN/COMMIT callers outside this helper are not
covered by this repair.

## Acceptance and test oracle

`internal/users/account_transaction_cleanup_test.go` exercises eight cases:
audit failure plus denied rollback, denied COMMIT with denied or successful
rollback, callback rejection, panic with denied or successful rollback,
ordinary successful rollback, and trigger-driven automatic rollback.

Each fixture uses the actual Store schema and account-status/audit writer,
one retained pool connection, and native authorizer/trigger failure injection.
The next physical connection identity and autocommit state are inspected
before querying, removing injection, or tearing down. Exact account state and
complete audit history must match the committed baseline. A later valid write
must commit exactly one event; reopening must preserve its exact history and
status. Both COMMIT and rollback error leaves are checked separately, and the
original callback sentinel and panic value are checked directly.

The original regression failed in all four active-transaction denial cases;
its automatic-rollback case also detected the intended conservative-eviction
difference. A first fixture iteration incorrectly compared empty slices with
nil history. Real disable/enable baseline events replaced that fixture before
the causal run, preserving exact assertions and adding history-preservation
coverage.

## Reproduction and validation

```sh
go test -count=1 -run '^TestAccountTransactionCleanup$' ./internal/users
go test -race -count=10 -shuffle=20261022 -run '^TestAccountTransactionCleanup$' ./internal/users
go test -count=1 -cover . ./internal/...
```

The focused existing account baseline and full ordinary account/driver
baseline passed before implementation. All eight repaired scenarios passed ten
shuffled race-enabled repetitions (seed `20261022`, 80 scenarios, zero observed
failures). The complete uncached ordinary root/internal Go suite passed;
account coverage remains 83.1%. Independent plan and completed-code reviews
approved the repair. The reviewer also passed the cleanup and existing
account-creation/access COMMIT-failure regressions three times under the race
detector (seed `20261023`). Zero observed failures are not proof of zero risk.

Final staged review strengthened the COMMIT error assertion: COMMIT and
ROLLBACK can both return ErrAuth, so the first error leaf must also retain
the COMMIT-specific operation context. The strengthened eight-case test passed
three further race-enabled repetitions (seed `20261024`), followed by repeated
Go vet and whitespace checks. Independent review approved the correction.

Go vet and lint (zero issues), the production Wails build, dependency integrity,
code-map freshness, test-file inventory, configuration and release-scope checks,
all 46 coverage-package headings (49 total headings), whitespace checks, and
REUSE (948/948 files) passed. The build retains the
existing macOS 13-object/11-link-target warning and is ad-hoc signed, not a
notarized distributable. No frontend source or binding changes were needed.

All acceptance criteria passed: rejected state/audit isolation, connection
disposal before reuse, healthy rollback reuse, original error/panic preservation,
and exact subsequent-write/reopen state. User guides, encryption ADRs and API
documentation are unchanged because their UI, keys, schemas and public contracts
are unchanged. The architecture, code map, coverage ledger and earlier cleanup
evidence are updated instead.

## Limits and lessons

This is local macOS evidence. Go race detection does not instrument native C.
There is no new native Windows/Linux, power-loss, cancellation or real I/O-error
evidence. Authorizer denial supplies a deterministic real native failure,
without sleeps, retries, or weaker cryptography. The earlier full users race
timeout remains a separate validation limitation; this repair does not claim
full release readiness.

Logical error return does not imply native rollback. Verify rejected state
before teardown, retain healthy-path controls, and keep cleanup at the layer
that actually owns the transaction. Recovery writes and reopen checks catch
both retained locks and durable accidental commits.
