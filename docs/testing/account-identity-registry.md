<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Account identity registry: first implementation slice

## Scope and acceptance

This implements only the registry, legacy identity initialization and both
account creators from the [approved identity contract](../design/wbs-authenticated-account-identity.md).
Username reuse previously could not distinguish account incarnations. The new
opaque identity and protected-state generation are prerequisites, not an
authentication fix: login coherence, private principals, WBS grants and operation
leases remain unimplemented. No WBS bridge or UI is activated.

The additive registry preserves the users primary key, exact historical names,
folders, credentials, DEKs, escrow/recovery bindings and public Account DTO.
Retired rows retain only opaque tokens with NULL bindings and generations.
This prevents reuse within the supported database lineage, not after rollback
or replacement of system.db, and is not erasure of historical database pages.

| Acceptance criterion | Regression evidence in `internal/users/account_identity_test.go` |
| --- | --- |
| Atomic identity schema, legacy backfill and readiness; preserve existing rows | Legacy snapshots, second-allocation entropy/collision failures, independent concurrent initializers and full concurrent `Open` on a complete preceding schema |
| Stable identities; refuse partial, unknown or damaged state without repair | Exact schema/marker/coverage checks, missing or changed guards, malformed persisted tokens and reopen snapshots |
| Both creators allocate in their existing account transaction | Helper/production role and audit contracts, entropy/collision/INSERT/audit/COMMIT failures, no returned account/DEK or newly owned folder on failure |
| Retirement is permanent, including FK-disabled deletion | Purge/recreate, detached tombstones, raw deletion with foreign keys on/off, forbidden replacement/deletion/rename writes and full snapshots |
| Live creators require readiness | Remove marker after opening, then refuse both creators without persistent effects |
| Generation changes only for actual protected changes | Cosmetic/no-op updates, password/hash/wrap/role/disable and ABA transitions; one increment for multi-field changes |
| Exhaustion aborts the whole statement | Single/multi-row user and registry updates under ordinary, IGNORE, FAIL and REPLACE conflict policies, including earlier-row audit effects |

The existing incomplete-purge regression also verifies that failed filesystem
cleanup leaves identity retired and that leftover-folder refusal does not create
another identity or audit event.

## Review findings and prevention

Independent plan review rejected relying on a CHECK constraint for overflow:
outer SQLite IGNORE/FAIL conflict policies can suppress failure or retain earlier
row effects. Explicit `RAISE(ABORT)` guards enforce whole-statement refusal.
Independent implementation review found that SQLite text length/pattern checks
stop at NUL; token CHECK, insert guard and reopen validation now explicitly reject
embedded NUL bytes. Regression tests use the project's SQLCipher-capable driver,
not a different SQLite implementation or a mock.

Final review added FK-off replacement probes so constraints cannot hide a broken
guard, live-handle missing-readiness checks, and concurrent full startup checks.
Fault fixtures remove and restore individual guards only to seed otherwise
unreachable boundary states; production has no bypass hook.

## Evidence and limitations

Local environment: Go 1.27.2, CGO, macOS arm64, 2026-10-14. Initial focused
account baselines passed in 1.363s and the unmodified users package in 49.968s.
Nineteen new top-level identity tests cover the matrix above.

Completed checks:

```sh
go test -count=1 -cover . ./internal/...
go test -race -count=5 -shuffle=20261028 -run '^TestAccountIdentity|^TestPurgeAccountReportsFolderItCouldNotRemove$' ./internal/users
go test -race -count=1 -shuffle=20261029 -json ./internal/users
go vet . ./internal/...
make lint-go
go mod verify
make build
codesign --verify --deep --strict build/bin/gopmgr.app
make code-map-current coverage-ledger-current
make coverage-ledger-drift
make config-check release-scope
reuse lint
gitleaks git --pre-commit --staged --redact --no-banner .
git diff --check
```

All passed: root tests 88.200s, users 91.494s with 83.2% statement coverage;
five targeted race repetitions 96.788s with zero observed failures. Lint reported
zero issues and module integrity verified. The native build compiled/packaged
and ad-hoc signed the app; strict signature verification passed separately.
Build output included Wails `Not found: time.Time` binding diagnostics and an
object built for macOS 13.0 linked against 11.0 warning. Bindings and public DTOs
are unchanged; older-OS runtime compatibility is not established by this build.
The build is not a GUI startup, notarization or distribution-signing check.
Coverage drift checked all 46 package headings, including the DuckDB analytics
pass; test inventory, seven tracked configuration files, release scope and
REUSE compliance also passed.
The complete affected-package race run passed in 457.787s: 276 top-level tests
and 190 nested cases passed, with zero failures or skips; the JSON output
confirmed shuffle seed 20261029. All in-scope acceptance criteria above passed.
The broader root/internal suite was not race-instrumented; the full users
package and repeated focused regressions were.
The complete staged patch was reviewed and its secret scan reported no leaks.
No unrelated, generated, dependency, private handoff or temporary-control files
are included.

Independent final review approved the slice after five focused regressions
passed in 1.366s. Two temporary causal controls also failed for the intended
effects, then were restored before the passing race/build checks:

```sh
# Remove all three explicit NUL checks, then run:
go test -count=1 -run '^TestAccountIdentityMalformedTokenIsRefused$|^TestAccountIdentityDamagedStateIsNotHealed$/embedded_NUL_identity$' ./internal/users
# Replace the registry UPDATE guard's RAISE(ABORT) with SELECT 1, then run:
go test -count=1 -run '^TestAccountIdentityGenerationExhaustionAbortsWholeStatement$/OR_(IGNORE|FAIL)/all_users$' ./internal/users
```

The first control accepted malformed insertion and damaged migration state.
The second failed to abort under IGNORE and retained changed users, identity
generation and audit rows under FAIL. These are regression-oracle evidence,
not claims of a production exploit. There is no temporary bypass in the final
implementation. Zero observed race failures does not prove zero failure risk.

This is local macOS evidence. No real account database is upgraded by these
tests. Whole historical startup migration atomicity, crash/power-loss durability,
actual older executable compatibility, physical database replacement/restore,
other operating systems and authenticated WBS behavior are not established.
Older SQL creation/replacement patterns are refused, but this is not a claim
that every old binary's migration/import path is safe. Direct local schema
tampering remains outside the existing security boundary. Registry growth and
opaque-token correlation are accepted policy costs.

No frontend, crypto algorithm, dependency or project-file format changes are
needed. UI audits are inapplicable to this backend-only slice. Native packaging
and hosted CI evidence, if absent, must not be inferred from Go test results.
