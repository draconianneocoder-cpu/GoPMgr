<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Account creation write-lock oracle

## Requirement and repair

Account creation checks and insertion must share the immediate write transaction
that protects the first-administrator rule and case-insensitive uniqueness.
The old test helper paused at `afterAccountRuleCheck` for 300ms. Password hashing
and key wrapping happen before the transaction, so that pause did not guarantee
contender overlap. This is a strongly evidenced test-sensitivity gap, not an
observed production authorization failure.

The replacement opens and warms an independent connection to the temporary
store's actual `system.db`. At the existing checkpoint it attempts an immediate
transaction with zero busy timeout. Only a typed SQLite busy error is accepted;
unexpected success is reported and its transaction is rolled back. A transaction
lease prevents probe callbacks from interleaving BEGIN and ROLLBACK on one pooled
connection. Hook invocation counts prevent a removed checkpoint from passing.
Cleanup restores the global hook and closes the probe after workers join.

Two isolated cases, first-account and administrator-created standard account,
exclude another writer as the explanation for contention. Existing concurrent
tests retain their start barrier, real password hashing, winner/error checks,
stored-role checks, rejected-folder checks and first-account creation-history
assertion. They do not guarantee simultaneous contender execution.

## Acceptance and causal evidence

| Requirement | Evidence |
| --- | --- |
| Lock held at rule checkpoint | Independent write acquisition returns SQLite busy; exact invocation counts in isolated and concurrent cases |
| Real account behavior preserved | Existing concurrent outcomes plus isolated persisted roles and creation-event counts/actions |
| Missing lock detected | Temporary deferred `BEGIN` made both isolated cases fail with `account rule check does not hold write lock`, exit 1; production transaction restored before further checks |
| Cleanup | Successful negative-control transactions explicitly roll back; independent pool closes; hooks restore after joined workers |

Local baseline: clean `fb281d3`, Go 1.27.2, macOS arm64, CGO. The prior two local
commits were preserved. Focused pre-change tests passed in 1.950s; focused
post-change tests passed in 0.709s. Deferred-BEGIN negative control failed as
intended in 0.383s. This establishes the isolated oracle's causal sensitivity;
it does not establish how often the old fixture would miss that mutation.

## Verification commands

```sh
go test -count=1 -run '^TestCreateAccountAs(ConcurrentFirstAccounts|ConcurrentCaseVariants|HoldsWriteLockAcrossRuleCheck)$' ./internal/users
go test -race -count=20 -shuffle=20261026 -run '^TestCreateAccountAs(ConcurrentFirstAccounts|ConcurrentCaseVariants|HoldsWriteLockAcrossRuleCheck)$' ./internal/users
go test -count=1 -cover . ./internal/...
go test -race -count=1 -shuffle=20261027 -json ./internal/users
go vet . ./internal/...
go mod verify
make lint-go code-map-current coverage-ledger-current coverage-ledger-drift license-check release-scope
```

Twenty repetitions of all three top-level tests (60 top-level executions,
including 40 isolated subtest executions) passed under the race detector with
seed `20261026` in 101.057s, with zero observed failures. Removing the checkpoint
call made both isolated cases fail on the exact zero-invocation assertion
(exit 1, 0.316s); the call was restored before subsequent checks.

Fresh `go test -count=1 -cover . ./internal/...` passed all 43 packages;
root completed in 81.149s and accounts in 80.173s, with unchanged account
statement coverage of 83.1%. `go vet . ./internal/...` and `make lint-go` passed
(zero lint issues). Code-map freshness, coverage test-file inventory, REUSE
(950/950 files) and release-scope checks passed. Dependency verification passed.
Coverage-ledger drift passed all 46 package headings, including DuckDB analytics.
The complete account suite passed with race detection and shuffled seed
`20261027` in 398.630s, under the unchanged default ten-minute deadline.
All 257 inventoried top-level names matched terminal passes, with no missing
or extra names; all 101 observed subtests passed. There were zero failures,
zero skips, no cached results and process exit zero. The subtest count describes
this execution, not every possible platform-dependent subtest.

Independent adversarial plan and implementation reviews approved the bounded
repair. They challenged wrong errors/files, absent callbacks, incidental writer
contention, cleanup, deadlock and concurrent reporting. No material findings
remained. The reviewer inspected code independently; execution results were
provided by the implementing agent, not independently rerun by the reviewer.

All acceptance criteria above passed. The final scope contains the test repair,
the hook-comment correction, the coverage-ledger update and this evidence report.
Architecture/API/user documentation is unchanged because no runtime contract
changed. The investigation deliberately proves checkpoint exclusion, not every
possible schedule or crash boundary. No unrelated cleanup was justified.

## Limits and prevention

No runtime logic, public API, data format, cryptographic parameters, schema,
dependency, timeout or production synchronization changed. The existing hook
remains test-only and tests replacing it must not run in parallel. Probe pool
serialization protects only the diagnostic connection, not production callers.
No native Windows/Linux, power-loss, full release or UI evidence is claimed.
The Go race detector does not instrument the SQLCipher C core.

Prefer an observable forbidden operation at the critical checkpoint over an
elapsed-time assumption. Check isolated ownership as well as concurrent outcomes,
and require causal controls to fail for the intended reason rather than any error.
