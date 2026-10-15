<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Immutable account identity for authenticated WBS operations

**Status: D1–D3 policies approved by the owner on 2026-10-14; implementation
design remains subject to per-slice review.** The first registry/migration/
creator slice is implemented; see its [bounded evidence](../testing/account-identity-registry.md).
Coherent authentication and
WBS operation grants are not implemented by this slice. The
[storage foundation](wbs-storage-boundary.md) remains dormant.

## Observed problem and scope

At source revision `d7403ca`, `users.Account` and the `users` primary key use
username. `PurgeAccount` removes that row, allowing the name to be reused.
`App.Login` calls `Authenticate`, `UnlockDEK` and escrow enrollment separately
by username before publishing the session. `Authenticate` stamps login and
may rehash by username after verification. `createLegacyDEK` reselects by name.
An immutable ID fetched only after these calls cannot establish that the
verified credentials, unlocked key and intermediate writes belonged to one
account incarnation. This is source evidence of an integration risk, not a
reproduced production exploit.

`db.WBSBinding.ActorID` is already included in the request fingerprint and
project audit. The storage layer deliberately trusts its caller; it does not
authenticate. `docs/SECURITY.md` says an already signed-in account disabled in
another process keeps working until sign-out. This proposal must not silently
change that policy or the promise that password changes keep the App session.

Preserve username primary keys, folders, public account DTOs, password/recovery
formats, DEKs and existing username-bound escrow/AAD. No account rename,
cryptographic rebinding, project-file migration or new renderer is proposed.

## Owner decisions

| Gate | Recommendation | Consequence and alternative |
| --- | --- | --- |
| D1: retired identities | Keep permanent opaque tombstones | Prevents intentional or collision-driven reuse within the supported lineage. Retired rows contain only token and retirement state, not username or credentials. Tokens still permit correlation and storage grows with account creation. Without retention, collision/non-reuse assurance is weaker and must be stated differently. |
| D2: older account writers | Require an identity-capable version for account mutations after upgrade | Older binaries may remain read-only only after actual compatibility tests. Their account-creation/replacement paths must be refused, not silently assigned identity later. Supporting older mutation needs a separately proven protocol. Approval does not itself make old binaries safe. |
| D3: WBS revocation | Reauthorize each WBS operation and retained-receipt replay under a system-database writer lease | A disable, deletion or protected-state change that commits first prevents a later WBS operation. An operation that validated while holding the lease can finish before that change commits. This narrows only the new WBS grant, not existing App login/legacy-view policy. Admission-only rechecks avoid the lease but leave a check-to-commit race. |

The owner approved all three recommendations on 2026-10-14. This records policy,
not runtime enforcement or approval of unreviewed implementation details. Each
implementation slice still requires its own plan, review and verification.
System-database restore/replacement remains a separate recovery-policy gate.

## Proposed identity and migration contract

- Allocate an opaque token `ac1:` plus 64 lowercase hexadecimal digits from
  32 bytes read with `io.ReadFull(crypto/rand.Reader, ...)`. It is an identifier,
  not a secret or an authentication credential. Entropy failure or any collision
  aborts without mutation; do not retry or derive identity from a name/key.
- Prefer an additive identity registry over changing the `users` primary key
  or rebuilding that table. Rebuilding can cascade-delete recovery/escrow rows.
  An additive column is smaller, but alone cannot remember retired IDs or
  safely distinguish an older writer's new account from damaged identity state.
- The registry has a unique immutable token, unique exact active username,
  and positive, bounded signed-64-bit authorization generation. A detached row
  has NULL username and no credential/generation data. NULL means retired only;
  it is never an allocation state and cannot be rebound. Tokens cannot be
  updated or deleted through supported writers.
- Creation preallocates a fresh registry row with non-NULL username, then
  inserts the user in the **same transaction**, using a deferred username
  foreign key. Commit validates the association. Abandoned allocation rolls
  back; it cannot leave a pending row authorizing a subsequent account insert.
  Apply this to `CreateAccountWithKey` and the direct `Store.CreateAccount`
  helper, preserving each path's existing role and audit semantics.
- When ready, an insert guard requires a fresh preallocated association and
  refuses an insert over an existing username, including `INSERT OR REPLACE`.
  A users-delete trigger detaches the registry row independently of foreign-key
  enforcement; relying only on `ON DELETE SET NULL` misses FK-disabled writers.
  The registry also uses that FK action. Required guards and their definitions
  must be validated, not merely assumed present.
- Only a wholly absent identity schema qualifies for legacy initialization.
  Reserve the writer before inspection; atomically create schema/guards,
  assign identities to every exact stored username, validate complete coverage,
  then publish versioned readiness. Preserve existing case variants. Two
  initializers must converge on the same IDs. Partial, unknown or damaged state
  fails closed, not through automatic token regeneration or backfill.
- After readiness, missing or inconsistent identity refuses WBS authorization.
  Do not silently enroll an account created by an older binary. Existing
  identities must survive reopen, password change, relocation and role changes.
  Purge retires identity in the account/event transaction; filesystem cleanup
  failure does not reactivate it or bypass the existing leftover-folder refusal.

The first slice's SQL definitions, trigger sequencing and creator changes have
independent implementation review and real project-driver regressions documented
in the evidence above. Physical replacement, table rebuild,
trigger removal and unverified older migration/import paths are not supported
writers. They must be gated before activation; direct local SQL tampering remains
outside the existing security boundary, not a problem tokens can solve.

## Credential coherence and WBS grants

A private Go principal contains verified incarnation, exact username and current
authorization generation. It is not added to the public `users.Account` DTO.
Capture an App admission epoch before slow authentication or project-path
resolution, independently of the persistent account generation.

1. Read one coherent account/identity/hash/wrap/generation snapshot. Verify
   password and unwrap its key from that snapshot, outside database writer locks.
2. Before any login stamp, rehash, legacy-key publication or escrow enrollment,
   require the same incarnation and credential snapshot under a transaction.
   Writes must use conditional predicates or incarnation-bound transaction
   helpers. Final rejection cannot undo a write already applied to a replacement
   account. Do not compose the current username-only calls into a trusted grant.
3. Transparent rehash and initial DEK creation may advance generation. Only
   that successfully validated credential operation receives the resulting
   generation. Its conditional writes and enrollment must remain coherent.
   Clear derived keys on every failure. Preserve generic credential refusals.
4. Publish a session/project binding only if the captured App epoch still
   matches; logout, another login or project replacement cannot be overtaken by
   a late completion. Project-path confinement and DEK selection must use the
   same pinned principal, not different calls to `requireUser`.

First-account auto-sign-in is another authentication entry point: its private
creation result must carry incarnation/generation with the newly created DEK
from the same transaction. Re-reading identity by username after creation is
not safe. Existing public creation signatures can remain wrappers; no identity
may be recovered from a frontend-returned account DTO as proof of authority.

Under D3, validated triggers advance generation only when `disabled`, `is_admin`,
`password_hash` or `wrapped_dek_pw` actually change. Login stamps, display names
and preferences do not advance it. Validate integer type, positive range and
overflow refusal atomically. Never reset an exhausted counter to an old value.
Even transparent rehash may invalidate another WBS grant under this conservative
rule; it does not sign that App session out.

Disable/enable ABA cannot revive a stale WBS grant. Password change/recovery
reset, promotion/demotion and other protected changes require a fresh WBS
authentication grant; existing App sessions remain as currently promised. An
old grant cannot refresh itself by reading the new generation. Any local
generation replacement must come from a newly validated authentication result.

## Operation ownership, ordering and outcomes

Admission takes `App.mu` briefly to validate the epoch, capture immutable
session/project references and acquire a counted operation pin, then releases
it. No database wait or expensive hashing occurs while that mutex is held.
The proposed acquisition order is:

`operation pin → system writer lease → WBS store read lock → project transaction`

Under the lease, recheck exact incarnation, generation, enabled state and own
project binding. Read-only administrator access is not an editing grant.
Perform all project operations, including receipt replay, before releasing
the lease. Do not call another system write-transaction helper or reacquire
`App.mu` while holding it. Scope the lease helper narrowly; no generic callback
framework or background worker is needed.

The lease orders concurrent revocation: lease acquisition plus successful
validation establishes admission; the project transaction establishes save
commit. If revocation's system transaction commits first, the operation refuses.
If the save acquired the lease first, revocation waits until the save resolves.
This is ordering, not an atomic transaction across two database files.

One caller deadline, at most ten seconds from operation admission, covers lease
acquisition and WBS SQL. Actual native cancellation behavior must be tested;
this is not a claimed hard bound on hashing, filesystem or uninterruptible I/O.
Pins release and owned key copies clear on every return/panic path.

A transition marks admission closing and advances the App epoch while holding
`App.mu`, then releases it **before** cancellation/draining. After pins finish,
close the WBS store, project handles and owned keys. Never wait for an operation
while holding a lock it needs. Drain timeout retains a non-admissible closing
owner and its resources for explicit cleanup retry; it cannot report successful
logout or free in-use handles. No detached cleanup goroutine is proposed.

A known project commit remains committed even if system-lease cleanup later
fails: preserve its receipt and report cleanup separately, invalidate the grant
and discard an uncertain system connection. Do not turn that result into a
noncommit or automatic new request. Existing indeterminate project-commit
handling still requires reconciliation.

## Bridge and replay contract

The future typed frontend request carries copy/chart/lifetime/request identity,
expected revision string, creation intent, title and immutable model bytes,
not actor identity, generation, password or DEK. The backend obtains authority
from its private grant. Client-supplied actor fields must never select it.
Keep snake_case wire names and exact revision strings; no generated bindings
or TypeScript code change in this slice.

`WBSBinding.ActorID` uses the stable token. The existing fingerprint includes
it but excludes lifetime; continue excluding authorization generation. Thus a
newly authenticated grant of the same incarnation can reconcile its original
request, while a recreated account cannot claim the prior receipt. Reauthorize
before replay; replay never authorizes itself or replaces a newer draft.
Permission/closing errors preserve the draft and distinguish a known committed
receipt from an indeterminate outcome. Detached scope sharing omits identity
metadata; full project audit history retains its existing confidentiality policy.

## Verification and rollout gates

The following is the full design's verification plan. The first slice establishes
identity migration, creator rollback, retirement, guarded old-SQL writes and
generation-transition evidence; see its separate evidence document. Credential,
App, operation-lease, real older-executable and restore checks remain **required
future evidence**. Use real file-backed stores and explicit barriers, not sleeps,
and verify exact persisted effects as well as return errors.

| Risk | Acceptance evidence |
| --- | --- |
| Identity reuse | Purge/recreate same name gets a new token; old token stays detached and cannot rebind; forced retired-token collision refuses with no user/folder/event publication |
| Migration loss | Legacy accounts, case variants, key wraps, recovery and escrow rows unchanged except new metadata; reopen/concurrent initialization retains IDs; partial schema, damaged guards, entropy/collision failures publish nothing |
| Stale credential side effects | Pause verification, purge/recreate or reset password; old attempt cannot stamp/rehash/enroll/replace the new account's key or publish its session |
| Stale App publication | Pause login/path resolution; logout, other login or project switch wins; late work cannot publish or use the wrong account's path/DEK; clear temporary keys |
| Revocation race and ABA | Independent handles/processes order saves and disable/delete/role/password changes in both directions; disable/enable never revives the old grant; cosmetic changes preserve it; generation overflow changes nothing |
| Cleanup and commit ambiguity | Cancel during lease acquisition/write/drain, inject rollback/commit cleanup failure and panic; no deadlock/leak/wrong-target SQL; a confirmed project commit retains its receipt even if lease cleanup fails |
| Authority and replay | Forged actor/generation, read-only admin access and changed copy/project/lifetime cannot authorize; same incarnation after fresh sign-in replays the retained original request without another audit; recreated account refuses |
| Old writers and restore | Run actual supported-version binaries and connection modes, including FK-off delete/replace probes; no surviving identity binding. Restored system DB can roll generations/retired IDs back: require separate recovery enforcement before activation |

Use causal mutations dropping incarnation/generation predicates, guards, epoch
checks, lease or replay authorization. Require failures for the prohibited effect,
not an unrelated setup error. Run focused, shuffled race and broader affected
tests; verify plaintext system metadata and SQLCipher project reopen/audit.

The implementation slices are: (1) additive identity migration and all creators
(implemented), (2) coherent credentials/private grants and App admission epochs,
(3) guarded WBS operations and close/replay outcomes. Each needs its own plan,
adversarial review and causal tests. None alone enables WBS production writes:
system restore/replacement, project-copy reset, old-binary protection, source
incarnation and whole-file mutation gates from the storage contract remain.

## Evidence for this design task

The following records the earlier design task, not current implementation
verification. Registry implementation evidence is recorded separately above.

Clean `main` at `d7403ca`, Go 1.27.2, local macOS arm64; prior work preserved.
Current baselines passed:

```sh
go test -count=1 -run '^TestWBSStore(ReplayAndReopen|ConcurrentWriters|Refusals|CommitFailureAndPartialSchema)$' ./internal/db
go test -count=1 -run '^TestLoginRefusesWhileProjectOpen$' .
```

Database baseline: 0.367s; session baseline: 0.535s. Neither proves any proposed
identity behavior. Independent plan review required credential-write coherence,
admission epochs, precise generations, migration validation, old-writer and
restore gates, and the deferred allocation sequence; those corrections are
incorporated here. At design time no new runtime tests, build, native bridge,
cross-platform or full-release checks were claimed, and runtime security
documentation and accepted ADRs were unchanged. Policy approval alone does not
establish enforcement; the first slice's reviewed implementation and evidence
are linked above.
