<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Full account race verification

## Scope

Source revision: `3f26b762c98f35a762d2cde82aa6cd64118e14f0`, local macOS
arm64, Go 1.27.2, CGO enabled, ten logical CPUs and 32 GiB memory. This task
addresses incomplete full-account race evidence after the earlier broad run
with seed `20261015` reached the default ten-minute package deadline.

No code, tests, cryptographic parameters, timeouts, concurrency limits, build
tags, dependencies or assertions are changed. The checks use complete package
test binaries, not filtered test batches, short mode or retries. Source and
test behavior are identical to the stated revision.

## Investigation and acceptance

The old timeout stack showed runnable Argon2 workers in account fixture setup,
and the interrupted test passed alone. That supports an execution-cost
hypothesis, not proof of the historical cause. Inspection found that password
hashing uses 64 MiB, three passes and four lanes; key wrapping uses 64 MiB, one
pass and four lanes. Recovery issuance hashes eight codes and additionally
wraps eight keys when keyed. Standby fixtures create four real accounts.
These operations must remain real; no cheaper cryptographic substitute is
justified by a slow race run.

Acceptance requires exact reconciliation of all expected account test names,
terminal package success and process exit zero, no account skips or cached
results, and completion under unchanged default deadlines. A subsequent run
must include the exact package/tag scope of the release race gate. Broader
conditional skips must be assessed explicitly rather than counted as passes.
The release race step is not the complete release gate.

## Complete account baseline

```sh
go test -list '^Test' ./internal/users
go test -race -count=1 -shuffle=20261015 -json ./internal/users
```

Passed in **389.682 seconds**: all 256 inventoried top-level names matched,
all 99 observed subtests passed, zero failures, zero skips, process exit zero.
The observed-subtest count is not an inventory of every possible conditional
subtest. The previously interrupted `TestHandOverToAStandbyReportsTheKeyTheyHold`
passed in 1.74 seconds
within the full run. This rules out a persistent failure of that test in this
execution, not every possible historical stall or race.

Longest top-level tests in this run:

| Test | Seconds |
| --- | ---: |
| `TestConfirmRecoveryCodesRefusesAfterTheCodesChanged` | 24.61 |
| `TestCreateAccountRefusesWindowsDeviceNames` | 12.33 |
| `TestConfirmedRecoveryCodesReplaceTheOldOnes` | 11.14 |
| `TestEachReasonProtectsForItsDays` | 9.61 |
| `TestPreparedRecoveryCodesLeaveTheOldCodesWorking` | 9.19 |

Nested subtest times are not added to their parent durations. These are
observed timings, not portable performance thresholds or profiles proving
where all CPU time was spent.

## Release race scope

```sh
go list -tags webkit2_41 . ./internal/... ./tools/update-manifest
go test -race -count=1 -shuffle=20261025 -tags webkit2_41 -json . ./internal/... ./tools/update-manifest
```

The package inventory contains 44 packages and matches
`scripts/check-release.sh`'s race command scope. All 44 unique expected package
names matched terminal passes, with no missing or extra packages and process
exit zero. Root completed in **414.063 seconds** and accounts in
**474.709 seconds**, within the default ten-minute deadline. The account
inventory again matched all 256 top-level tests and all 99 observed subtests,
with zero account failures/skips. Neither run used cached test results.

One existing root test was skipped:
`TestProjectWithInvalidUTF8FolderIsReachableFromWirePath`. APFS rejected the
invalid-UTF-8 fixture name with `EILSEQ`; the unchanged test skips only that
filesystem rejection and fails other errors. Its Linux-specific path assertions
were not exercised here. This assessed platform exclusion is nonblocking for
the bounded account verification and local race-step completion, not a pass
for that test or evidence of native Linux verification. No other skip or
failure occurred in this run.

## Review and outcome

Independent plan and evidence reviews checked exact name accounting, duplicate
and unfinished events, package results, cache markers and race diagnostics.
The runner recorded process exit zero; JSON independently establishes package
success, not a separate shell status. Conditional-subtest counts and the APFS
exclusion remain explicit. No meaningful test was weakened or disabled.

The current full-account verification gap is closed for this source revision
and local environment. No runtime defect was reproduced, so there is no
speculative code repair or performance claim. The earlier timeout remains a
historical failed observation; its cause is not resolved by these passing runs.
Higher contention or a different machine could still exceed the default budget.

## Limits and lessons

The historical timeout remains an observed failed run with an unproven cause.
Do not equate a timeout with deadlock, or an isolated pass with full-suite
completion. Preserve process status when piping JSON output into diagnostic
logs, reconcile names rather than totals alone, and distinguish top-level
durations from nested subtest durations.

Go's race detector does not instrument native C. No native Linux/Windows,
power-loss, trusted signing or complete release-gate assurance is supplied
by these local checks. User guides, APIs, architecture and code maps are
unchanged because this task changes verification evidence, not behavior.
