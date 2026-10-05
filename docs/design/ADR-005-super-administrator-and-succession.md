<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-005: Super administrator and succession

**Status:** Accepted (owner decisions, 2026-10-05). Part 1 (the role,
subordinate limits, hand-over) and part 2 (standby and takeover) built
2026-10-05; part 3 (escrow rotation and a fresh key) next. All three ship
before the next release tag.
**Decision date:** 2026-10-05

## Context

[ADR-004](ADR-004-administrator-access-to-user-data.md) gave every
administrator the administrator (escrow) key, which opens every enrolled
user's data. Its decision 3, rotating that key when someone stops being an
administrator, raised a gap: a rotation needs an administrator who holds the
key, and any administrator could remove any other. The owner chose a single
super administrator instead.

## Decision

- **One super administrator.** Every other administrator is a subordinate.
- **Subordinates manage standard accounts only:** create, disable, enable,
  and delete them, and issue their recovery codes, and read the account
  history.
- **Only the super administrator** promotes, demotes, disables, or deletes
  administrators, creates administrator accounts, issues an administrator's
  recovery codes, holds the administrator key, opens users' data, and
  rotates the key. Nobody can demote, disable, or delete the super
  administrator; they hand the role over first.
- **Who is super administrator:** the first account on a new install. On an
  install that already has several administrators, the enabled administrator
  created first (compared as times; `created_at` is RFC 3339 text that does
  not sort as a string). The role is reassigned only when it names nobody
  who is an enabled administrator (for example after an edit to
  `system.db`), by the same rule; every assignment is recorded as
  `super_admin_assigned`. When no administrator can sign in, `BecomeAdmin`'s
  claimant becomes super administrator.
- **Hand-over now.** The super administrator can make an enabled
  administrator who has signed in since enrollment began the super
  administrator at once. The administrator key goes with the role, under the
  same attestation check as any grant (a target whose personal key fails it
  is refused), and the former super administrator's grant is removed. If the
  key cannot be opened the role still moves, the user is told, and the
  history says the key could not be passed. Recorded as
  `super_admin_handed_over`. Any data the former super administrator had
  open is closed.
- **Standby successor (part 2).** The super administrator names a standby
  in advance and sets how long they may go without signing in before the
  standby can take over (default 30 days, minimum 7; the maximum of 365 is a
  bound chosen while building, not an owner decision). The period counts
  from the later of the super administrator's last sign-in and the time
  they got the role, so an upgrade or a hand-over starts it again. The
  standby holds the administrator key so the role is usable after a
  takeover, but acts as a subordinate until then. Once the period has
  passed, the Admin panel offers the standby "Become super administrator";
  nothing changes until they confirm it, and it is recorded. Racing
  claimants are serialized by `system.db`'s write lock, and the loser finds
  the role newly given and the takeover no longer due. A former super
  administrator who returns is a subordinate; the current super
  administrator can hand the role back.
- **Takeover with no standby (part 2).** When the super administrator has
  named no standby, however they got the role (first account, upgrade, or
  hand-over), any enabled administrator who has signed in can take over
  once the period has passed (30 days unless the super administrator set
  another). The first to confirm gets the role, and it is recorded. Nobody
  else holds the administrator key, so the new super administrator has the
  role without it until part 3's fresh key (owner decisions, 2026-10-05).
- **Takeover after the standby is also inactive (part 2b).** When the
  standby has also gone the takeover period without signing in, any enabled
  administrator can take over, without the key, as when no standby is
  named (owner decision, 2026-10-05).
- **Takeover reason and protection (part 2b).** The administrator taking
  over must choose a reason before confirming; it is recorded with the
  takeover and sets how long the former super administrator is protected
  from being disabled, deleted, or demoted (owner decisions, 2026-10-05).
  Handing the role back stays allowed.

  | Reason | Protected for |
  | --- | --- |
  | Vacation | 30 days |
  | Parental leave (maternity or paternity) | 180 days |
  | Medical or convalescence leave | 90 days |
  | No longer an employee | 7 days |
  | Other (a note is required) | 30 days |

- **Rotation (part 3).** The escrow key is rotated automatically whenever
  someone stops holding it (a hand-over, a takeover, or a standby being
  replaced or removed), and on demand with "Replace administrator key".
- **Fresh key when nobody holds it (part 3).** Rotation re-seals with the
  old private key, which is gone after a takeover with no standby or a
  hand-over that could not pass the key. The super administrator then
  creates a new key; each user's data is re-sealed to it at their next
  sign-in, which is recorded in their history. Until a user signs in again,
  nobody can open their data.

## Security rule

The `super_admin` row and the standby in `super_succession` are plaintext
in `system.db`, like `is_admin`. They decide what the app offers, never what
the key allows: every key operation (opening data, passing the key at
hand-over, granting the standby, rotating) also needs the caller's own
usable grant, opened with their own DEK and checked against their pin. A
row forged to name a subordinate gives them no data access and no key to
pass. Phase 1's lesson applies: granting must never follow a plaintext
column.

One exception follows from the standby holding the key: a row forged to
name the standby does let them open data, as the super administrator could.
The opening is still recorded and the user still told
(`TestForgedSuperRowNamingTheStandbyOpensDataAndIsRecorded`). A standby
column naming the super administrator or someone who is not an enabled
administrator is cleared when read.

## Consequences

- Promotion, enabling an administrator, and creating an administrator no
  longer grant the key (they did under ADR-004 phase 1); a grant left on a
  subordinate from before is removed at their next sign-in or when the super
  administrator demotes or disables them, and recorded.
- The last-administrator guard can no longer be reached through the store:
  the super administrator is always an enabled administrator and cannot be
  removed. It stays as defense in depth.
- A super administrator who stops signing in can be replaced: by the
  standby, or, with no standby, by any administrator, who then has the role
  without the key until part 3's fresh key and each user's next sign-in.
- Accepted risks (part 2):
  - The period is measured with this computer's clock. An administrator who
    sets the clock forward can take over early. With no standby that gives
    them the role without the key, but the role alone lets them disable or
    permanently delete the former super administrator and every other
    administrator, and deleting an account deletes its folder. The takeover
    is recorded, and the former super administrator sees it when they
    return.
  - The period counts sign-ins, not use: a super administrator who stays
    signed in for longer than the period without signing in again can be
    taken over.
  - If both the super administrator and the standby stop signing in, nobody
    can take over until part 2b, which lets any administrator take over
    once the standby has also been inactive for the period.
  - Moving the clock forward can also run out a protection window early;
    part 2b's protection limits the damage a takeover can do, but uses the
    same clock.

## Implementation parts

1. The role, subordinate limits, migration, hand-over (2026-10-05).
2. Standby, inactivity period, takeover (with or without a standby),
   hand-back (2026-10-05).
   2b. Takeover once the standby is also inactive; takeover reason and the
   former super administrator's protection.
3. Escrow rotation, automatic and manual, and a fresh key when nobody holds
   the old one.
