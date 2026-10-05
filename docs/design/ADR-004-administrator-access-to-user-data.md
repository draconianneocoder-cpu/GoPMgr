<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-004: Administrator access to users' data

**Status:** Accepted; phases 1 and 2 built (2026-10-02); phase 3's sign-in
notice built (2026-10-02); escrow rotation (decision 3) accepted but not
built. The owner accepted every recommendation under Owner decisions on
2026-09-25. The release guards that kept phases 1 and 2 from shipping before
the notice were removed with it. [ADR-005](ADR-005-super-administrator-and-succession.md)
(2026-10-05) narrows who holds the key: only the super administrator (and
their standby), so the promotion, enabling, and administrator-creation rows
below no longer grant.
**Decision date:** 2026-09-25 (owner requirement: 2026-09-24)

## Context

The owner decided that administrators may read other users' data from inside
GoPMgr, and that every such access must be recorded: who read which account,
when, and why.

Under [ADR-001](ADR-001-database-encryption-at-rest.md), each user's
data-encryption key (DEK) is wrapped only by that user's password and recovery
codes. Nobody else can unwrap it. The DEK opens the user's SQLCipher projects
and, through `crypto.DeriveSubkey`, their deletion log (`internal/deletionlog`)
and procurement catalog (`internal/catalog`). An administrator today can read
another account only by using its initial password or recovery codes outside
the app, and nothing records it.

`system.db` is plaintext and readable before sign-in. Everyone who shares the
operating-system account can read and write it.

## Decision

Add an administrator escrow built from the standard library's `crypto/hpke`
(RFC 9180) with `hpke.DHKEM(ecdh.X25519())`, `hpke.HKDFSHA256()`, and
`hpke.AES256GCM()`. Keys come from `KEM.GenerateKey`, are stored with
`PrivateKey.Bytes`/`KEM.NewPrivateKey`, and public keys are derived with
`PrivateKey.PublicKey`. No new dependency. The package's post-quantum hybrid
(`MLKEM768X25519`, X-Wing) is still an IETF draft; each escrow key stores its
KEM ID, so moving to it later is a rotation.

- **Escrow key pair.** One machine-wide key pair, identified by an ID. Each
  user's DEK is sealed to its public key and stored in `system.db`.
- **Personal key pairs.** Every account gets its own key pair. The private key
  is wrapped under a subkey of the account's DEK; the public key is stored in
  plaintext.
- **Grants.** The escrow private key is sealed to each administrator's personal
  public key, one grant per administrator.
- **Access.** An administrator unwraps their own personal private key, then
  the escrow private key, then the target user's DEK. The key is released only
  after the access record, including the stated reason, is committed.

```text
admin password ─► admin DEK ─► admin personal private key
                                   │ opens grant
                                   ▼
                          escrow private key
                                   │ opens sealed DEK
                                   ▼
user password / recovery code ─► user DEK ─► user's projects, deletion log, catalog
```

Every HPKE seal binds its purpose and owner in `info`, so a sealed value moved
to another row does not open:

| Sealed value | Recipient | `info` |
| --- | --- | --- |
| user DEK | escrow public key | `gopmgr/escrow/dek/v1`, escrow key ID, username |
| escrow private key (grant) | admin personal public key | `gopmgr/escrow/grant/v1`, escrow key ID, admin username |

### Trust anchors

Public keys in a plaintext file can be swapped by anyone who can write
`system.db`. Without anchors, swapping the escrow public key would make every
later sign-in seal its DEK to an attacker's key, and a forged grant would hand
an administrator an attacker's escrow key to seal new accounts to.

- **Escrow pin.** When an account first enrolls, it stores a MAC of the escrow
  key ID and public key under a subkey of its own DEK
  (`crypto.DeriveSubkey`). Every later seal at
  sign-in checks the pin first. On a mismatch it seals nothing and records an
  `escrow_key_mismatch` event.
- **Grant check.** Administrators are accounts too, so each has a pin. Opening
  a grant derives the public key from the unsealed escrow private key and
  checks it against that administrator's pin before using it. A grant that
  fails the check, does not open, or was sealed to a personal key the
  administrator no longer has is deleted and recorded, so their sign-in still
  commits and another administrator can grant them again.
- **Admin sessions never read the escrow public key from disk.** They derive it
  from the escrow private key they hold. That covers bootstrap, admin-created
  accounts, promotion, and rotation.
- **Personal public keys.** At sign-in, an account derives its public key from
  its own private key and repairs a swapped stored copy, recording the event.
  An administrator session also attests each personal public key with a MAC
  under a subkey derived from the escrow private key's bytes. At account creation this happens
  in the creating administrator's session. Promotion refuses a key that fails
  its attestation.
- **Grants only in an administrator's own action.** A grant is added only
  inside an explicit administrator action: creating the escrow key, creating
  an administrator account, promotion, or enabling a disabled administrator.
  A sign-in never grants, and an administrator session never attests a key
  for the first time; it only checks existing attestations. `is_admin` and
  `disabled` are plain columns, so a sign-in that granted to whoever they name
  would hand the escrow key to anyone who can write `system.db` (found while
  building phase 1, 2026-10-02). First-use attestation happens only at
  promotion or enabling and is recorded as `personal_key_trusted`. If that
  administrator's next sign-in then records `personal_key_repaired`, the key
  trusted at promotion was not theirs and the escrow key may have been sealed
  to an attacker: treat it as exposed until rotation. A disabled account is
  promoted without a grant and granted when enabled.
- **Escrow private key lifetime.** An administrator session unwraps the escrow
  private key for each operation and zeroes it afterwards; it is never kept in
  the session.
- **Remaining window, trust on first use.** An existing account's first
  enrollment trusts the escrow public key on disk at that moment. The first
  attestation of an existing account's personal key trusts that key. Pins and
  personal keys are rows in the same plaintext file, so deleting an account's
  pin or personal key puts that account back into trust on first use. Nothing
  in a plaintext file can prevent that, so it is detected instead:
  - enrolling an account that already had a pin or a personal key records
    `escrow_reenrolled` and shows in the Admin panel;
  - each administrator session checks that every sealed DEK opens with the
    real escrow key and flags any that do not, removing that DEK and the
    account's pin so its next sign-in pins the real key and seals again;
  - an administrator session never creates an escrow key while any escrow key
    row exists, active or retired; an administrator without a grant waits
    for another administrator to promote or enable them.

  The owner must accept this window.

### Where keys are sealed, granted, and removed

| Event | Key action |
| --- | --- |
| First administrator exists and has a session (sign-in, creation, or `BecomeAdmin`) | Create the escrow key pair, grant it to that administrator, seal their DEK. |
| Administrator creates an account | The new DEK, personal key pair, sealed DEK, escrow pin, and attestation are written in the same transaction as the account. (Until ADR-005, a new administrator's grant too.) |
| First account creation (the first administrator) | Bootstrap as above, in the same session. |
| Sign-in of an account that is not yet enrolled | Create the personal key pair if missing, check or set the escrow pin, seal the DEK. Failure does not block sign-in; it is retried at the next sign-in and shown in the Admin panel. |
| Sign-in of an administrator with a grant | Also check existing attestations and that every sealed DEK opens. Never grants. |
| `UnlockDEK` generates a DEK for an account older than ADR-001 (`dek.go:98`) | Seal it in the same transaction as its password wrap. |
| Password change or ordinary recovery reset | Nothing: the DEK does not change. |
| Promotion (`AdminSetUserRole`) | Super administrator only (ADR-005). Changes the role and grants nothing: the new administrator is a subordinate. A target with no personal key yet is refused until they sign in once (owner decision, 2026-10-02). The attestation check now runs when the key is passed (hand-over, and ADR-005's standby): a key the escrow key never attested is trusted on first use and recorded. |
| `BecomeAdmin` (no administrator can sign in) | Bootstrap as above if no escrow key has ever existed. Otherwise the new administrator has no grant, and with no other administrator to promote them, none until rotation. |
| Demotion, disable, or permanent deletion of an administrator | Delete that administrator's grant (`ON DELETE CASCADE` on the account row). This stops later use of the current `system.db`; an administrator who kept an earlier copy still holds the old grant, and only rotation keeps them out of accounts enrolled afterwards. |
| Super administrator hands over the role (ADR-005) | The key is passed to the new super administrator under the attestation check, and the former one's grant is removed. |
| Super administrator names a standby (ADR-005) | The key is granted to the standby under the attestation check. A replaced or removed standby's grant is removed, as is the standby's when they are demoted, disabled, or deleted. |
| Sign-in of the standby (ADR-005) | Their grant is opened and checked; one that does not open is removed and recorded. Any other subordinate's grant is removed. |
| Takeover (ADR-005) | The former super administrator's grant is removed. The standby keeps theirs; with no standby nobody holds the key until ADR-005 part 3's fresh key. |
| Re-enabling a disabled administrator | Super administrator only; grants nothing (ADR-005). |
| Permanent deletion of any account | The sealed DEK, personal keys, and grant go with the row. |

The `account_events` action list gains `admin_access`, `escrow_key_mismatch`,
`escrow_reenrolled`, `personal_key_repaired`, `personal_key_trusted`, and
`escrow_rotated`. Actions
are checked in Go (`accountEventActions`), not by a `CHECK` on the table
(removed 2026-09-28, before the table shipped in a release), so each needs
only a new entry there, not a migration.

### Rotation

Each administrator's pin names the escrow key ID it trusts, and opening a
grant removes it when the pin names another key. Rotation must therefore
re-pin administrators before it re-issues their grants, or each new grant is
removed at its holder's next sign-in.

If the owner chooses rotation (decision 3), one administrator session does it
in one `BEGIN IMMEDIATE` transaction: create a new escrow key pair, re-seal
every enrolled user's DEK to it, recompute each account's escrow pin (the
session holds each DEK at that moment), re-issue grants to the remaining
administrators, re-attest personal keys, retire the old escrow key, and record
`escrow_rotated`. Without re-pinning, every account would report
`escrow_key_mismatch` at its next sign-in.

### Access flow

1. The administrator chooses a user in the Admin panel and types a reason. The
   backend refuses self-access and a target that is not enrolled before
   recording anything, so the record never lists access that could not
   happen.
2. In one `BEGIN IMMEDIATE` transaction, the backend checks that the caller is
   an enabled administrator (read from `system.db`, not the session), opens
   the caller's grant and the target's sealed DEK, and records `admin_access`
   with the actor, the target, and the reason.
3. The DEK is released only after that transaction commits. If the record or
   the commit fails, no key is released. Opening the DEK inside the
   transaction, rather than after it as first written, means the record never
   lists access that could not happen: a grant or sealed DEK that does not
   open is recorded as a mismatch instead, and refused (as built, 2026-10-02).
4. Paths used during access are confined to the target's `projects/` folder
   with the same checks as `projectPathFor` (no symlinks, an existing regular
   file). Ordinary sessions do not change.
5. The key lives in a separate, read-scoped handle, never in `a.dek`. The
   administrator's own projects, catalog, deletion log, and exports keep using
   their own key. The handle is zeroed when the administrator stops viewing,
   signs out, or signs in again, and on shutdown. The caller's role is read
   from `system.db` again at each view, and a demoted caller's access stops.
7. Viewing only (owner decision, 2026-10-02): the administrator may look at
   the target's projects in the app and nothing else. A project is copied
   (with any `-wal` and `-shm` file) into a folder with an unpredictable name
   in the target's data folder, outside `projects/`, checked for integrity,
   and opened read-only (`query_only` on every connection). The copy has its
   own database handle and its own read-only methods, never `a.db`, so no
   export, report, archive, attachment, or edit method can reach it. The
   owner's file is only read. The copy is removed when viewing stops; a copy
   folder more than a day old, left by a crash, is removed when access to that
   account next opens (younger ones may be open in another GoPMgr process).
   A copy taken while its owner is writing from another GoPMgr process can be
   torn; it then fails the integrity check and the administrator is asked to
   try again.
8. The viewer shows the project's summary, its schedules (Gantt and CPM
   charts, with dates computed as the project's reports compute them), its
   cost entries and approved cost baselines, and its documents' titles,
   never their contents or cost attachments (owner decision, 2026-10-02).
   Every viewer read re-checks the caller's role. A compliance-mode project
   whose audit trail fails its check is shown with a warning, where its
   owner would be refused (owner decision, 2026-10-02).
6. The handle covers the target's projects. Whether it also covers their
   deletion log and catalog is part of the owner decision on scope below.


### Telling the user

Built 2026-10-02 (decision 2). Every `admin_access` event is shown to the
account it names on the first screen after each sign-in (Portfolio, where
every sign-in lands) until the user chooses "I've read this"; the
acknowledgement is recorded as `access_notice_read`, so the Admin panel's
history shows the user was told. `access_notices` keeps the newest access
the user acknowledged. An acknowledgement covers only accesses up to the
newest one shown, so one recorded while the notice was open stays unread;
it never moves back, and acknowledging nothing new records nothing.
`account_events` ids come from `AUTOINCREMENT`, so a later access always
counts as unread. If the notice cannot be read it says so, with a retry,
rather than showing nothing. App Settings, under Account, lists every access
to the user's data (owner decision, 2026-10-02).

Limits: the notice is only as good as the record. An administrator who can
write `system.db` can delete their `admin_access` row (the triggers stop only
the app) or move the user's acknowledgement mark, and the user is then not
told. Headless sign-in (the command line) shows no notice; the next desktop
sign-in does.

## Consequences

- **One administrator password now unlocks every enrolled user.** An offline
  Argon2id attack on `system.db`, which is readable by anyone who shares the
  operating-system account, yields every enrolled account's data, not just the
  administrator's own. This is why a sign-in lockout was declined: it would not
  stop an offline attack either.
- **The record covers access through the app only.** An administrator can
  decrypt offline with their own password and leave no record. The record is
  plaintext, and anyone who can write `system.db` can edit it; the triggers
  stop only the app.
- **Demotion cannot take back what an administrator already obtained.**
  Rotating the escrow key keeps them out of accounts enrolled afterwards.
  Taking back an already obtained DEK means giving that user a new DEK, which
  means re-encrypting their projects. That is out of scope.
- **Accounts that never sign in again after the upgrade are never enrolled**,
  so administrators cannot open them in the app.
- ADR-001's hierarchy gains a second path to every DEK. Its "only the user's
  password and codes" statement no longer holds and must be updated when this
  is built.

## Rejected alternatives

- **Administrators keep users' initial passwords or recovery codes (today).**
  Nothing is recorded, and access disappears when the user changes their
  password.
- **Wrap each DEK directly under each administrator's DEK.** Promoting an
  administrator would need the new administrator's DEK, which the promoting
  session does not have, and storage grows with users × administrators.
- **Seal each DEK to each administrator's personal key, with no shared escrow
  key.** It works, but storage grows with users × administrators, and promotion
  must re-seal every DEK. A shared escrow key needs one seal per user and one
  grant per administrator.
- **The operating-system keychain.** It is shared by everyone in the
  operating-system account, so it enforces no administrator boundary.
- **Re-encrypting projects under an administrator key.** It breaks ADR-001's
  one-DEK-per-user design and every opener that depends on it.

## Implementation phases

1. **Key material and enrollment:** personal key pairs, the escrow key pair,
   pins, attestation, sealing at every event in the table above, grants on
   promotion, and removal on demotion, disable, and permanent deletion. No
   access yet.
2. **Access:** the recorded access flow, the separate read-scoped key handle,
   and the Admin panel's "Open this user's data" with a required reason.
3. **Afterwards:** telling the user, and escrow rotation, if the owner chooses
   them.

Phases 1 and 2 ship in the same release, with no tag between them (owner
decision, 2026-09-28). Phase 1 alone would let any administrator's password
unlock every enrolled user's data outside the app with nothing recorded;
phase 2 adds the recorded access that justifies the escrow. They may be
separate pull requests. The trust anchors (pins and attestation) ship in the
same pull request as the first sealing, never after it.

## Test strategy

- A sealed DEK or grant moved to another account's row fails to open.
- Swapping the escrow public key in `system.db`: the next sign-in seals
  nothing and records `escrow_key_mismatch`.
- Deleting an account's pin: its next enrollment records `escrow_reenrolled`,
  and an administrator session flags a DEK sealed to a planted escrow key.
- A planted escrow key row: bootstrap does not replace it.
- Rotation: every enrolled account still opens, no account reports a
  mismatch at its next sign-in, and a removed administrator's old grant does
  not open accounts enrolled afterwards.
- A forged grant (an attacker's escrow private key sealed to an administrator's
  public key) fails the administrator's pin check.
- Swapping a personal public key: the owner's next sign-in repairs it, and a
  promotion before then is refused by the attestation check.
- Every DEK creation site (`dek.go:98` and account creation) leaves a sealed
  DEK that opens to the current DEK. A recovery reset never replaces the DEK
  (a legacy code is refused, 2026-09-29), so it needs no sealing step.
- Injecting a failure into the access record: no key is released.
- The access handle is zeroed on stop, sign-out, and shutdown, and never
  replaces `a.dek` (the administrator's own catalog and deletion log still open
  with their own key during access).
- Demotion, disable, and permanent deletion remove the grant; a demoted
  administrator's session cannot open access.
- An account that has not enrolled cannot be opened, and the Admin panel says
  why.
- Two GoPMgr processes enrolling and granting at the same time; fault seeds
  for every check above.

## Owner decisions

Accepted as recommended, 2026-09-25: read-only access through a temporary
copy, the user is told at their next sign-in, the escrow key is rotated when
someone stops being an administrator, access covers projects only, and the
trust-on-first-use window and wider exposure below are accepted.

1. **Read-only or read-write access?** Recommended: read-only. Cost: opening a
   project normally migrates an older schema in place, so read-only access
   either refuses a project that needs a migration or reads a temporary copy.
   The recommendation is to read a temporary copy in the target's folder,
   removed when access stops. With read-write, changes made while viewing would
   be recorded in the user's project audit trail under the administrator's
   name, and the owner must say whether the user can see them.
2. **Tell the user?** Recommended: yes. At their next sign-in the user sees who
   opened their data, when, and the stated reason, built from `account_events`.
3. **Rotate the escrow key when someone stops being an administrator?**
   Recommended: yes. It keeps them out of accounts enrolled afterwards but
   cannot take back what they already obtained.
4. **Scope of access:** projects only, or also the user's deletion log and
   procurement catalog? Recommended: projects only.
5. **Accept the trust-on-first-use window** under Trust anchors, and **the wider
   exposure** under Consequences, in particular that one administrator password
   now unlocks every enrolled account.

Decided 2026-10-02, while building phase 1: grants are added only inside an
administrator's explicit action, never at sign-in, and promoting an account
that has not signed in since enrollment began is refused until it has.

Decided 2026-10-02, before building the notice: it stays at every sign-in
until acknowledged, the acknowledgement is recorded, and App Settings lists
every access; rotation is a separate item.

Decided 2026-10-02, before building phase 2: access is view only (no export,
report, archive, print, or attachment while viewing), through a dedicated
read-only viewer rather than the ordinary project screens, showing the
summary, schedule, costs, and document titles; a broken audit trail is shown
with a warning; phase 2 ships as a backend slice and then the Admin panel
slice. The phase-1 escrow guard is replaced by one that blocks release until
users are told at sign-in (phase 3), since decision 2 is accepted but not
yet built.
