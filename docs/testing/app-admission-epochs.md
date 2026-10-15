<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# App admission epochs and the session-held identity

## Scope

This implements step 4 of "Credential coherence" in the
[identity contract](../design/wbs-authenticated-account-identity.md), and the
first account's principal. The [credential slice](credential-coherence.md)
made each sign-in coherent on its own. Two App-level gaps remained.

- **Late publication.** `Login`, the first account's automatic sign-in, and
  `AdminOpenUserData` published their result whenever they finished. A
  sign-out or another sign-in that finished meanwhile was overtaken, and
  administrator access that finished after sign-out kept the target's key
  past the session.
- **Unpinned pairing.** About twenty methods read the user (or a confined
  path) and later read the key separately. A session switch in between paired
  one account's name, folder, or path with another account's key:
  - **Escrow operations** opened the administrator's grant with the wrong key.
    The store then deleted the grant as unusable and recorded a mismatch.
  - **`IssueRecoveryCodes`** stored codes for one account wrapping another's
    key.
  - **`EncryptProjectAtRest`** encrypted one account's project under another's
    key.
  - **`DeleteProject`** recorded one account's deletion in another's log.
  - **`SecureArchive`** archived whichever project was open into the folder
    of the path it was given, so one account's project could be written into
    another's folder. The final review found this one.

Now:

- **Two transitions.** `session_state.go` changes the session only through
  `setSessionLocked` and `clearSessionLocked`. Both advance `App.epoch`, and
  between them they clear:
  - the project, its services, and its open-check;
  - administrator access;
  - the key;
  - prepared recovery codes and the no-codes choice.
- **Captured session.** Operations capture a `sessionRef` (user, principal,
  epoch) once and take the key only through `sessionDEK(s)`. The old
  `withSessionDEK` without a session is gone; `projectPathFor` returns the
  session it confined the path for.
- **Guarded publication.** Sign-in, first-account sign-in, project opening,
  administrator access, a viewed project copy, `BecomeAdmin`,
  `rememberExportDirectory`, and `PrepareRecoveryCodes` publish only if the
  epoch is unchanged, and `SecureArchive` archives the open project only for
  the session whose folder the path is in.
- **Owned access.** Administrator access records its session, and
  `openAccess` and the project view check it.
- **Held identity.** The session holds the `users.Principal` from `SignIn`, or
  from the creation transaction for the first account
  (`Store.CreateAccountWithPrincipal`).

Remaining direct key reads, each with the user read under the same lock:

- `sessionDEKLocked`;
- `openCatalog`, which reads `a.user` and the key in one locked section;
- `RepairAndSwap`, which holds the write lock throughout.

Behavior changes:

- **Refused sign-in.** A refused sign-in has still recorded the sign-in in
  `system.db`. Its message: "signing in was interrupted because you signed out
  or another account signed in; try again".
- **Refused administrator access.** It stays recorded, so the user is told of
  it at sign-in. Its message says so.
- **First account.** If the session changed while it was created, the account
  still exists and the message asks the user to sign in.
- **Sign-out clears more.** Sign-out now also clears the project services and
  the open-check, and sign-in clears the previous session's prepared codes.
- **Other refusals.** Operations refused after a switch return "you signed out
  or switched accounts while this was in progress, so it was stopped; try
  again".

Not in this slice:

- **Draining at sign-out.** An operation that already holds the key may finish
  after sign-out, for the same account. Draining it is slice 3's operation
  pins.
- **Project ordering.** Ordering between two project opens in the same session.
- **First account's escrow.** Its escrow bootstrap still runs by username.
- **Session policy.** Signed-in sessions acting on a replaced account remain
  under the existing policy.

## Acceptance evidence

Each switch happens at the `sessionGap` test hook, which runs at the point
where an operation has captured the session and is about to use or publish
it. The tests are in `session_state_test.go`.

| Criterion | Test |
| --- | --- |
| A sign-out or another sign-in that finished first wins; the session holds the winner's account, identity and key | `TestALateSignInDoesNotOvertakeASignOut`, `TestALateSignInDoesNotOvertakeAnotherSignIn` |
| The session principal matches the registry (generation 2 for a new first account) and is cleared at sign-out | `TestTheSessionHoldsTheSignedInIdentity` |
| The first account is created but not signed in after a session change | `TestTheFirstAccountIsNotSignedInAfterTheSessionChanged` |
| Administrator access is not kept after sign-out, stays recorded, and is unavailable next session; access and a viewed copy belong to their session | `TestAdministratorAccessIsNotKeptAfterSignOut`, `TestAdministratorAccessBelongsToItsSession`, `TestAViewedProjectBelongsToItsSession` |
| An escrow operation across a switch leaves the grant and records no mismatch | `TestAnEscrowOperationAcrossASessionSwitchLeavesTheGrant` |
| Recovery codes are not issued across a switch | `TestRecoveryCodesAcrossASessionSwitchAreNotIssued` |
| A project is not encrypted or opened across a switch; sign-out leaves no project state | `TestEncryptingAcrossASessionSwitchLeavesTheProjectAlone`, `TestOpeningAcrossASessionSwitchOpensNothing` |
| Sign-in drops the previous session's prepared codes | `TestSigningInDropsThePreviousSessionsPreparedCodes` |
| `rememberExportDirectory` and `BecomeAdmin` update only their own session | `TestSessionUpdatesStayInTheirSession` |
| A security backup, a deletion, and prepared codes across a switch write nothing; a viewed copy from an earlier session is not attached to a newer access | `TestASecurityBackupAcrossASessionSwitchWritesNothing`, `TestADeletionAcrossASessionSwitchDeletesNothing`, `TestPreparedCodesAcrossASessionChangeAreNotKept`, `TestAViewFromAnEarlierSessionIsNotAttached` |

All existing root and users tests pass unchanged.

## Fault seeding

Twenty seeds, each caught by the tests above:

- an unpinned key read;
- a sign-out that doesn't advance the epoch;
- a sign-in that doesn't clear the previous session;
- no sign-in guard (reworked once after it broke the build);
- no first-account guard;
- no access publication guard;
- project opening with the current key (reworked once after it broke the
  build);
- encryption with the current key;
- no `BecomeAdmin` guard;
- no export-directory guard;
- no access owner check;
- no view owner check;
- prepared codes surviving sign-in;
- the principal not stored;
- project state left at sign-out;
- the creation principal's generation taken before the writes;
- no security-backup guard;
- the deletion request taking the current key;
- no prepared-codes guard;
- a viewed copy published without its starting session.

## Validation and limits

Local macOS arm64, Go 1.27.2, CGO, 2026-10-15. All passed:

```sh
go vet . ./internal/...
GOOS=windows go vet .
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...   # 0 issues
go test -count=1 . ./internal/...
go test -race -count=3 -shuffle=on -run 'LateSignIn|SessionHolds|FirstAccountIsNotSignedIn|AdministratorAccess|ViewedProject|EscrowOperationAcross|RecoveryCodesAcross|EncryptingAcross|OpeningAcross|SigningInDrops|SessionUpdatesStay|Login|Logout|AdminOpen' .
go test -race -count=1 -shuffle=on .
make coverage-ledger-drift coverage-ledger-current code-map-current
GOPMGR_SKIP_NSIS_COMPILE=1 make verify
make frontend-smoke license-check
git diff --check
```

Session switches are staged within one process at a hook, not by concurrent
user interaction. Frontend behavior is
unchanged, and the new messages arrive through the existing error display.
No GUI walkthrough was run.
