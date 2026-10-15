<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Sign-in bound to one account identity: credential slice

## Scope

This implements the credential part of slice 2 of the
[identity contract](../design/wbs-authenticated-account-identity.md),
"Credential coherence and WBS grants", steps 1 to 3. Before it, `App.Login`
called `Authenticate`, `UnlockDEK` and escrow enrollment separately, each by
username, with slow password work between them. If the account was purged
and created again under the same name in between, the old attempt could:

- stamp or rehash the new account;
- plant its prepared key wrap on the new account;
- seal escrow rows for the new account with the old key.

Headless maintenance sign-in composed the same calls.
`App.AdminIssueRecoveryCodes` unlocked a key and then stored codes by
username. Across a replacement, it could store codes wrapping the old key for
the new account, and a reset with one of those codes would then cut the new
account off from its encrypted projects.

`users.Store.SignIn` (desktop), `SignInHeadless` (maintenance) and
`IssueRecoveryCodesWithPassword` now work in three steps:

1. Read one snapshot: identity, hash and key wrap.
2. Check the password and unlock or create the key outside any lock.
3. In one write transaction, require the same identity, hash and wrap, recheck
   disabled and roles, then make every write.

A different identity is refused like an unknown user. A changed hash or wrap
of the same identity restarts the attempt, at most three times
(`ErrSignInContended`). The private `users.Principal` carries the identity and
the generation after the sign-in's own writes. It is not part of the public
`Account`. The unbound `Authenticate`, `UnlockDEK` and `createLegacyDEK`
were removed.

Behavior changes:

- **Escrow enrollment** now runs in a savepoint of the sign-in transaction.
  An enrollment failure is still only logged, but a commit or busy failure
  of that transaction now fails the sign-in.
- **Invalid usernames** get the same refusal as unknown ones.
- **An empty password** is a plain mismatch.
- **Admin-issued codes** need the password to match the stored hash as well
  as unwrap the key. The issuing administrator's role, and the super
  administrator rule for an administrator's codes, are checked where the codes
  are stored.
- **Headless sign-in** reports any failure as "authenticate headless user".

Not in this slice:

- App admission epochs, so a late `Login` cannot publish over a `Logout` or
  another sign-in;
- keeping the `Principal` in the App session;
- a principal for the first account's automatic sign-in;
- WBS operation grants.

Signed-in operations keep the session policy in `docs/SECURITY.md`.

## Acceptance evidence

Each race below is staged with a second `Store` on the same data root, standing
in for another process, changing the account at the
`beforeCredentialCommit` hook. That hook runs on every attempt, after the
password work and before the write transaction.

| Criterion | Regression in `internal/users/credentials_test.go` |
| --- | --- |
| A replacement with the same password, with or without a key, is refused, writes nothing to the new account, and returns no key | `TestSignInIsRefusedWhenTheAccountIsReplaced` (snapshot taken right after the replacement: last sign-in, hash, wrap, identity, generation, pin, sealed key and code counts, history row count; the new account's sealed key still opens to its own key) |
| A password changed meanwhile is re-checked | `TestSignInWithAPasswordChangedMeanwhileIsRefused` |
| A wrap changed meanwhile returns the stored wrap's key | `TestSignInReturnsTheKeyOfTheCurrentWrap` |
| Two first sign-ins share one key | `TestConcurrentFirstSignInsShareOneKey` |
| Disabled meanwhile is refused; a promotion meanwhile is not a credential change | `TestSignInRechecksTheAccountAtCommit` |
| The principal matches the registry after the sign-in's writes | `TestSignInPrincipalMatchesTheRegistry` |
| Retries are bounded and write nothing | `TestSignInGivesUpWhenTheAccountKeepsChanging` |
| Only the desktop sign-in runs the escrow session; headless still enrolls a key it makes | `TestOnlyTheDesktopSignInRunsTheEscrowSession` |
| An escrow failure keeps the sign-in and the key | `TestSignInKeepsGoingWhenEscrowFails` |
| Recovery codes restore the right key, are refused across a replacement, and recheck both roles in the storing transaction | `TestIssueRecoveryCodesWithPassword` |

Existing behavior tests now exercise the new path through two helpers in
`credentials_helpers_test.go`. That covers lazy key creation and stability,
rehash, disabled after password, recovery-reset invariants, and the entropy
and blocked-write injections. The root-package tests use `SignInHeadless`.
`dek_test.go` replaces the unreachable empty-password wrap test with
`TestSignIn_EmptyPasswordIsAMismatch` and `TestSignIn_WrapKeyFailureStoresNothing`.

## Fault seeding

Sixteen seeded faults were run against the focused tests:

- **Caught:**
  - dropping the identity, hash or wrap comparison;
  - dropping the disabled re-check;
  - treating a stale snapshot as success;
  - running the escrow session in headless mode;
  - failing the sign-in on an escrow error;
  - reading the generation before the writes;
  - skipping the recovery-code identity check;
  - taking the administrator rule from the snapshot;
  - skipping the actor re-check (which first survived; the disabled-actor case
    was added);
  - mapping the App's unknown account raw;
  - removing both the wrap comparison and the `wrapped_dek_pw = ''` predicate
    together;
  - skipping headless enrollment of a key it made (its assertion was added
    at the final review, before this seed ran).
- **Survived, as designed:** dropping only the conditional rehash predicate,
  or only the wrap predicate. Inside the serialized `BEGIN IMMEDIATE`
  transaction, the comparisons already guarantee what those predicates
  check; they are kept as a second layer.

## Validation and limits

Local macOS arm64, Go 1.27.2, CGO, 2026-10-14. All passed:

```sh
go vet . ./internal/...
GOOS=windows go vet .
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...   # 0 issues
go test -count=1 . ./internal/...
go test -race -count=3 -shuffle=on -run 'SignIn|ConcurrentFirstSignIns|OnlyTheDesktop|IssueRecoveryCodesWithPassword|TestUnlockDEK|TestRecoveryReset|Disabled|AccountIdentity' ./internal/users
make coverage-ledger-drift coverage-ledger-current
GOPMGR_SKIP_NSIS_COMPILE=1 make verify
make license-check
git diff --check
```

The focused race run covered the new tests and the identity, disabled and
recovery tests three times in shuffled order. After the final review's test
additions, `go test -race -count=1 -shuffle=on . ./internal/users` also passed
(root 388s, users 517s). Other packages were not race-instrumented. No real
account database is used. Cross-process races are
modeled with two connections to one file, not separate executables. Native
platforms other than macOS are not exercised. Keys are cleared on the paths
that return errors, but clearing is not observable from tests.
