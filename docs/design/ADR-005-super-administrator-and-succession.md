<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-005: Super administrator and succession

**Status:** Accepted (owner decisions, 2026-10-05). Part 1 (the role,
subordinate limits, hand-over) built 2026-10-05; part 2 (standby and
takeover) and part 3 (escrow rotation) next. All three ship before the next
release tag.
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
  standby can take over (default 30 days, minimum 7). The standby holds the
  administrator key so the role is usable after a takeover, but acts as a
  subordinate until then. Once the period has passed, the Admin panel offers
  the standby "Become super administrator" at their next sign-in; nothing
  changes until they choose it, and it is recorded. A former super
  administrator who returns is a subordinate; the current super
  administrator can hand the role back.
- **Takeover with no standby (part 2).** When the super administrator has
  named no standby, however they got the role (first account, upgrade, or
  hand-over), any enabled administrator who has signed in can take over
  once the period has passed (30 days unless the super administrator set
  another). The first to confirm gets the role, and it is recorded. Nobody
  else holds the administrator key, so the new super administrator has the
  role without it until part 3's fresh key (owner decisions, 2026-10-05).
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

The `super_admin` row is plaintext in `system.db`, like `is_admin`. It
decides what the app offers, never what the key allows: every key operation
(opening data, passing the key at hand-over, granting the standby, rotating)
also needs the caller's own usable grant, opened with their own DEK and
checked against their pin. A row forged to name a subordinate gives them no
data access and no key to pass. Phase 1's lesson applies: granting must never
follow a plaintext column.

## Consequences

- Promotion, enabling an administrator, and creating an administrator no
  longer grant the key (they did under ADR-004 phase 1); a grant left on a
  subordinate from before is removed at their next sign-in or when the super
  administrator demotes or disables them, and recorded.
- The last-administrator guard can no longer be reached through the store:
  the super administrator is always an enabled administrator and cannot be
  removed. It stays as defense in depth.
- Until part 2 ships, if the super administrator stops signing in, nobody
  can manage administrators. Part 2's takeover is the recovery path, with or
  without a standby; without one, users' data can be opened again only
  after part 3's fresh key and each user's next sign-in.

## Implementation parts

1. The role, subordinate limits, migration, hand-over (2026-10-05).
2. Standby, inactivity period, takeover (with or without a standby),
   hand-back.
3. Escrow rotation, automatic and manual, and a fresh key when nobody holds
   the old one.
