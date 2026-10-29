<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->
# Policy, authority, state machine, and recovery reference (v4)

**Status:** Proposed implementation contract. The maintainer's two-tier direction is approved in principle; none of the described code is implemented.

## 1. Domain rules

- Local authentication policy belongs to the **trusted GoPMgr host**; external workers/plugins cannot authenticate a human or open the account DEK.
- The **account's immutable incarnation ID**, not username or folder name, scopes every security-key credential and operation. A deleted/recreated username must never inherit an old key.
- The **super administrator** owns installation-wide security policy. Delegated subordinate administrators may enforce the existing policy on standard users, never lower it, widen scope or manage protected administrators. This is compatible with ADR-005.
- Policy preference is separate from the **effective minimum**. A user's `FIDO2_OPTIONAL` preference cannot override a super-administrator `FIPS1403_REQUIRED` minimum.
- There are **two physical keys**, but one-key sign-in. The second is for recovery, not multi-key concurrency. Enrollments require evidence of distinctness; no claim that two credential IDs imply two devices.
- Users cannot obtain an ordinary session by invoking recovery, password reset, old CLI sign-in, legacy API routes, or a copied stale local session.
- Policy administration requires fresh reauthentication/step-up, not merely possession of an authenticated frontend view.

## 2. Durable policy evaluation

```text
Request(account_incarnation, operation, presented_credential)
  -> Read current global policy + local account preference and revision
  -> Determine effective minimum and credential capabilities
  -> Evaluate account enabled, role, recovery state, auth generation
  -> Perform hardware/secret verification OUTSIDE database write lock
  -> Re-read account + global policy epoch + credential revision in write transaction
  -> Verify still authorized, append audit and short idempotency receipt
  -> Admit NORMAL or RECOVERY_RESTRICTED session with explicit capability set
  -> Never grant elevated session from UI-only status
```

Policy epoch advances for every global change. Account auth generation advances for key adds/removals, password resets, recovery restriction and FIPS profile changes, and invalidates previously authenticated device operations. A bounded retry is allowed on legitimate concurrent change; repeated change leads to a user-visible contention response.

## 3. Roles and permissions

| Operation | Account owner | Subordinate admin | Super administrator |
|---|---|---|---|
| Opt into Tier 1 | Yes, with step-up | Cannot impersonate owner | Cannot impersonate owner |
| Prepare own primary/backup | Yes | Cannot silently register a user's device | Cannot silently register a user's device |
| Require FIPS for all managed accounts | No | No | Yes |
| Apply existing FIPS policy to delegated standard account | No | Only explicitly delegated | Yes |
| Relax global FIPS policy | No | No | Yes, audited and separately step-up-approved |
| Change own Tier-2 routine login to password | No | No | No; must follow separate policy-change and recovery process |
| Issue restricted recovery authorization | Request | Only where delegated and two-party policy allows | Yes, consistent with ADR-005 |
| Admin escrow and succession | Existing ADR only | Existing ADR only | Existing ADR only |

**FIPS admin enablement is not a per-login option:** any administrative action to enforce it on an account is a versioned security-policy mutation. Releasing it is also a privileged policy change with a stated reason and migration outcome, not a side effect of a failed login.

## 4. Enrollment state transitions

```text
PASSWORD_COMPAT
   └─ voluntary enrollment -> STAGING_PRIMARY
       └─ first independently proven wrap -> STAGING_BACKUP
           └─ second separately proven wrap -> VALIDATING_BOTH
               ├─ any error/cancel -> PASSWORD_COMPAT + cleanup of only staged data
               └─ both full fresh-session reopen tests -> FIDO2_OPTIONAL

FIDO2_OPTIONAL
   └─ admin marks enforcement target -> ENFORCEMENT_PENDING
       ├─ missing certification/profile/recovery -> PENDING_REMEDIATION
       └─ all prerequisites confirmed + privileged commit -> FIPS1403_REQUIRED

FIPS1403_REQUIRED
   ├─ valid qualifying key -> NORMAL_SESSION
   ├─ no key -> RECOVERY_RESTRICTED (with independent recovery authorization)
   ├─ surviving key, backup lost -> REPLACEMENT_REQUIRED (still FIPS key for normal login)
   └─ verified policy release -> explicitly approved lower profile

RECOVERY_RESTRICTED
   ├─ abort, expiry, failure -> no normal access
   └─ re-enroll/verify two qualifying devices -> return to effective tier
```

**No unsafe enrollment crash window:** Stage public material and encrypted wraps under a distinct operation ID. Test both device wraps against the exact existing DEK, and activate both + policy preference in one atomic `BEGIN IMMEDIATE`/validated transaction (subject to repository's real writer conventions). Do not begin an expensive device ceremony while holding the DB write lock. Audit success only after commit. Keep an idempotency receipt so a dropped response doesn't create a third credential or claim a failed action.

## 5. Deployment modes

**Personal/unmanaged installation.** Tier 1 displayed by default for each account; existing passwords work unchanged. FIPS mode is not automatically imposed because hardware is FIPS-ready. Super administrator may choose to manage the installation, but must explicitly acknowledge its security/recovery implications.

**Locally managed installation.** Super administrator chooses a coverage policy for standard users, administrators and new accounts. A rollout preview shows `ready`, `pending`, `unsupported-platform`, `recovery-unprepared`, and `qualifying-devices-unverified` counts. Existing unready accounts remain `PENDING_REMEDIATION`; the UI must not claim 100% enforcement. New users provision from a restricted session where mandatory, and gain normal project access only after two qualified keys are verified. Enforce with host authorization checks, not CSS/UI controls.

**Device or OS unsupported.** Do not enable strict enforcement for the unsupported combination. A deployment rollout is blocked for an in-scope unsupported device/platform; no local workaround silently converts a FIPS floor into password-only access. Existing password-only install remains functional where not under an enforced policy.

## 6. Recovery permission boundaries

- `NORMAL_SESSION`: existing allowed project operations subject to roles.
- `RECOVERY_RESTRICTED_SESSION`: may inspect security-key enrollment status, verify recovery materials, perform freshly approved key re-enrollment, and sign out. May temporarily hold DEK inside trusted auth service to wrap it for replacement authenticators. Cannot open, export, modify or synchronize project files, use plugin services, administer other users, or issue OAuth/network tokens.
- For Tier 2, enter recovery with **password plus independent recovery code** OR **password plus an independently authorized admin-supervised recovery approval** subject to current ADR-005 control. Never from password alone. The admin's authorization does not grant direct access to the user's projects.
- If all keys and backup credentials are lost, recovery may be impossible without existing ADR-004 escrow paths. Escrow is not a substitute for claiming the user has successfully authenticated.
- Role takeover and re-issue of administrator escrow keys are separate operations; security-key enrollment cannot magically repair or overwrite escrow grants.
- Expiration and cancellation invalidate recovery authorizations; audit the event ID, approver, method category and outcome, but no secrets or sensitive PIN details.

## 7. Distinct-device and privacy boundary

Ordinary FIDO2 credentials authenticate a **credential**; default attestation often identifies a *model* via AAGUID. Tier 1 uses a practical staged remove/reinsert ceremony and independent credential/unlock tests, plus model/serial indicators where consented and available. Tier 2 records what uniqueness evidence was actually seen: enterprise attestation or authenticated device-unique identity, or different attested model identities, or merely supervised physical custody. The last case **does not claim cryptographic uniqueness**; whether it is acceptable in an administered high-tier deployment must be an explicit policy/approval gate. Do not persist identifying serials when a keyed digest or per-install salted mapping is sufficient. Do not expose such metadata through general plugin APIs.

## 8. Administrative user experience

- Settings -> Security -> Security keys: status, key label, provider/model, enrolled date, proven DEK-unlock capability, firmware/evidence class, primary/backup, last verified use, and recovery readiness (no raw key secrets).
- Tier 1 enrollment wizard: prepare -> enroll primary -> remove -> enroll backup -> independently test each -> confirm; error handling always returns to safe password-only or prior valid state.
- Super Admin -> Security policy: target population, FIPS requirement toggle, certificate/approved-mode evidence, rollout preview, blocked accounts, administrative recovery owner, signed evidence catalog revision, activation receipt, and meaningful downgrade warnings.
- Sign-in: relevant device prompt; in `FIPS1403_REQUIRED`, selecting password routes to **recover account**, never a normal session. Avoid misleading phrase "passwordless without a PIN." Clear accessible non-color status for pending/ineligible/replacement-required.
