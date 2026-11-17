<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Tiered security keys (v4): design reference

The proposal's state machine, cryptographic and storage contract, roadmap and test plan; its ADR stays separate.

Each section below is a record kept as written, with its headings one
level lower and its links updated; nothing else changed.

## What this file holds

- [Policy, authority, state machine, and recovery reference (v4)](#policy-authority-state-machine-and-recovery-reference-v4): formerly `docs/design/POLICY-AND-ENROLLMENT-STATE-MACHINE.md`, undated, from commit `9ab5d91`.
- [Cryptographic, storage, OS and compatibility contract (proposed)](#cryptographic-storage-os-and-compatibility-contract-proposed): formerly `docs/design/CRYPTO-AND-STORAGE-CONTRACT.md`, undated, from commit `9ab5d91`.
- [GoPMgr A-series roadmap v4 — User-optional FIDO2 first; administrator-enforced FIPS 140-3 second](#gopmgr-a-series-roadmap-v4--user-optional-fido2-first-administrator-enforced-fips-140-3-second): formerly `docs/design/ROADMAP-TIERED-SECURITY-KEYS.md`, 2026-10-28, from commit `9ab5d91`.
- [Tiered FIDO2 / FIPS 140-3 acceptance and adversarial verification matrix](#tiered-fido2--fips-140-3-acceptance-and-adversarial-verification-matrix): formerly `docs/design/TEST-TIERED-SECURITY-KEYS.md`, undated, from commit `9ab5d91`.

## Policy, authority, state machine, and recovery reference (v4)

**Status:** Proposed implementation contract. The maintainer's two-tier direction is approved in principle; none of the described code is implemented.

### 1. Domain rules

- Local authentication policy belongs to the **trusted GoPMgr host**; external workers/plugins cannot authenticate a human or open the account DEK.
- The **account's immutable incarnation ID**, not username or folder name, scopes every security-key credential and operation. A deleted/recreated username must never inherit an old key.
- The **super administrator** owns installation-wide security policy. Delegated subordinate administrators may enforce the existing policy on standard users, never lower it, widen scope or manage protected administrators. This is compatible with ADR-005.
- Policy preference is separate from the **effective minimum**. A user's `FIDO2_OPTIONAL` preference cannot override a super-administrator `FIPS1403_REQUIRED` minimum.
- There are **two physical keys**, but one-key sign-in. The second is for recovery, not multi-key concurrency. Enrollments require evidence of distinctness; no claim that two credential IDs imply two devices.
- Users cannot obtain an ordinary session by invoking recovery, password reset, old CLI sign-in, legacy API routes, or a copied stale local session.
- Policy administration requires fresh reauthentication/step-up, not merely possession of an authenticated frontend view.

### 2. Durable policy evaluation

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

### 3. Roles and permissions

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

### 4. Enrollment state transitions

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

### 5. Deployment modes

**Personal/unmanaged installation.** Tier 1 displayed by default for each account; existing passwords work unchanged. FIPS mode is not automatically imposed because hardware is FIPS-ready. Super administrator may choose to manage the installation, but must explicitly acknowledge its security/recovery implications.

**Locally managed installation.** Super administrator chooses a coverage policy for standard users, administrators and new accounts. A rollout preview shows `ready`, `pending`, `unsupported-platform`, `recovery-unprepared`, and `qualifying-devices-unverified` counts. Existing unready accounts remain `PENDING_REMEDIATION`; the UI must not claim 100% enforcement. New users provision from a restricted session where mandatory, and gain normal project access only after two qualified keys are verified. Enforce with host authorization checks, not CSS/UI controls.

**Device or OS unsupported.** Do not enable strict enforcement for the unsupported combination. A deployment rollout is blocked for an in-scope unsupported device/platform; no local workaround silently converts a FIPS floor into password-only access. Existing password-only install remains functional where not under an enforced policy.

### 6. Recovery permission boundaries

- `NORMAL_SESSION`: existing allowed project operations subject to roles.
- `RECOVERY_RESTRICTED_SESSION`: may inspect security-key enrollment status, verify recovery materials, perform freshly approved key re-enrollment, and sign out. May temporarily hold DEK inside trusted auth service to wrap it for replacement authenticators. Cannot open, export, modify or synchronize project files, use plugin services, administer other users, or issue OAuth/network tokens.
- For Tier 2, enter recovery with **password plus independent recovery code** OR **password plus an independently authorized admin-supervised recovery approval** subject to current ADR-005 control. Never from password alone. The admin's authorization does not grant direct access to the user's projects.
- If all keys and backup credentials are lost, recovery may be impossible without existing ADR-004 escrow paths. Escrow is not a substitute for claiming the user has successfully authenticated.
- Role takeover and re-issue of administrator escrow keys are separate operations; security-key enrollment cannot magically repair or overwrite escrow grants.
- Expiration and cancellation invalidate recovery authorizations; audit the event ID, approver, method category and outcome, but no secrets or sensitive PIN details.

### 7. Distinct-device and privacy boundary

Ordinary FIDO2 credentials authenticate a **credential**; default attestation often identifies a *model* via AAGUID. Tier 1 uses a practical staged remove/reinsert ceremony and independent credential/unlock tests, plus model/serial indicators where consented and available. Tier 2 records what uniqueness evidence was actually seen: enterprise attestation or authenticated device-unique identity, or different attested model identities, or merely supervised physical custody. The last case **does not claim cryptographic uniqueness**; whether it is acceptable in an administered high-tier deployment must be an explicit policy/approval gate. Do not persist identifying serials when a keyed digest or per-install salted mapping is sufficient. Do not expose such metadata through general plugin APIs.

### 8. Administrative user experience

- Settings -> Security -> Security keys: status, key label, provider/model, enrolled date, proven DEK-unlock capability, firmware/evidence class, primary/backup, last verified use, and recovery readiness (no raw key secrets).
- Tier 1 enrollment wizard: prepare -> enroll primary -> remove -> enroll backup -> independently test each -> confirm; error handling always returns to safe password-only or prior valid state.
- Super Admin -> Security policy: target population, FIPS requirement toggle, certificate/approved-mode evidence, rollout preview, blocked accounts, administrative recovery owner, signed evidence catalog revision, activation receipt, and meaningful downgrade warnings.
- Sign-in: relevant device prompt; in `FIPS1403_REQUIRED`, selecting password routes to **recover account**, never a normal session. Avoid misleading phrase "passwordless without a PIN." Clear accessible non-color status for pending/ineligible/replacement-required.

## Cryptographic, storage, OS and compatibility contract (proposed)

### 1. Why FIDO authentication is insufficient by itself

GoPMgr presently needs the original per-user random 32-byte DEK to open SQLCipher databases. A WebAuthn/FIDO2 assertion supplies **proof of credential possession**, not a decryptable copy of this DEK. Any key-first architecture must prove a distinct, hardware-gated cryptographic route: qualified CTAP2 `hmac-secret`/PRF to derive a KEK, or FIDO2 plus a qualified PIV private operation, or a later separately approved equivalent. Authentication-only USB keys may supply MFA but not passwordless access to protected project data. Never cheat by caching an indefinitely valid DEK in a software-only file and labelling the sign-in hardware-enforced.

### 2. FIDO2 HMAC secret candidate (non-normative algorithm sketch)

1. Create a **device-bound** FIDO2 credential in the trusted native host with documented client/RP/verifier binding. Require user presence and user verification, no exportable/synced credentials for the physical USB profile. Record its public COSE key, full credential ID and verified attestation.
2. Request/enable the CTAP2 `hmac-secret` extension on credential creation; validate that the actual device and native OS transport expose it. WebAuthn `prf` is related but not the same API. A key without this capability can be MFA-only, not passwordless unless it proves another actual unlock profile.
3. Generate per-credential random **CTAP input salt** and independent **HKDF salt**; use `getAssertion` signed challenge (UV+UP required) to obtain the credential-specific extension output and verify assertion, correct credential ID, signature, RP binding and the extension's result length. Never accept an extension response on an unverified assertion.
4. Derive `KEK = HKDF-SHA256(hmac_output, salt=hkdf_salt, info=canonical('GoPMgr|DEK-wrap|v1', account_incarnation, enrollment_id, profile_version))`; use independent random nonce and AES-256-GCM to wrap the **same existing DEK**, with canonical AAD binding the same identity/profile/evidence and wrapping metadata.
5. Authenticate the AEAD tag on unwrap; check DEK size and an independently stored authenticated verifier/real encrypted project opening. Do not expose the DEK to frontend or connectors. Version all algorithm/encoding decisions; use constant-time comparisons for secrets where relevant.
6. Tier 2 may choose this path **only after** verifying exact CMVP-approved FIDO2 and HMAC operation for the specific module, model, firmware, approved-mode configuration and crypto path. A supported `hmac-secret` extension is **not automatically a FIPS-approved service**.

**Qualification constraint:** The YubiKey 5 FIPS 5.7.4 documentation lists approved FIDO2 profiles but does not establish, by that fact alone, that a particular application use of `hmac-secret` passes GoPMgr's Tier-2 approved-operation requirement. Vendor 5.8 `hmac-secret-mc` capabilities are not evidence for the validated 5.7.4 module. Use device testing and the full module security policy rather than conflating APIs.

### 3. FIDO2 + PIV candidate

- Verify a fresh real FIDO2 assertion with the account's credential first.
- Require a separately authorized PIV operation on the expected token to release/unwrap a per-token KEK/DEK envelope, using only hardware-backed approved algorithms and provisioning as recorded by the actual module security policy.
- Bind FIDO2 and PIV claims to the **same registered physical authenticator** using verified evidence, not a mere matching product name or USB port. Do not claim FIDO+PIV stronger if a malicious actor can present *different* devices to satisfy the two ceremonies.
- Complete nonvolatile key-provisioning/backup design, crypto-agility versioning and tested Windows/macOS/Linux drivers before shipping. A key that supports FIDO but no PIV may be Tier 1 through HMAC or only MFA; profile mismatch never falls back silently.

### 4. Suggested additive tables (illustrative; ADR/database review must supply exact SQL)

| Table | Core indexed key | Purpose |
|---|---|---|
| `security_policy_epochs` | `(installation_id, epoch)` | Current super-admin policy, covered account classes, minimum tier, signed evidence catalog version |
| `account_auth_preferences` | `(account_incarnation)` | User preference and effective tier cached with policy revision; cache not an authority over current policy |
| `hardware_credentials` | `(account_incarnation, credential_id_hash)` unique | COSE key, credential ID in protected form, attestation/identity class, FIPS evidence pointers, profile, state, revision |
| `hardware_credential_dek_wraps` | `(account_incarnation, enrollment_id, envelope_version)` | Separate AEAD-wrapped same DEK, CTAP/HKDF salts, AAD version, nonce, never plaintext key |
| `auth_enrollment_operations` | `(account_incarnation, operation_id)` unique | Durable stages, expected revisions, independent device tests, expiration, idempotency receipts |
| `auth_recovery_operations` | `(account_incarnation, recovery_operation_id)` unique | Scoped recovery authorization, allowed capabilities, operator approvals, expiration, outcome |
| `auth_security_events` | `(account_incarnation, monotonic_event_id)` | Appended audit bridge for enroll, enforcement, recovery, revocation and failed privileged attempts |

Existing `users.password_hash` and `users.wrapped_dek_pw` fields remain untouched by ordinary enrollment; existing `recovery_codes`, administrator `escrow_*`, account incarnation and generation remain authoritative for their current domains. No schema may overwrite an existing hash chain or invalidate recovery/escrow in a migration. `system.db` is plaintext; it contains **wrapped** secrets and public metadata only. SQLCipher encryption remains in per-project `.gopmgr` files.

### 5. Concurrency, crash and version safety

- Prepare cryptographic values outside write lock, then `BEGIN IMMEDIATE` or the repository-approved equivalent. Within one commit, re-check account immutable incarnation, enabled status, role, auth generation, policy epoch, credential state and intended profile; atomically publish device entries, DEK wraps, audit and idempotency receipt.
- Roll back staged enrollment entirely on mismatch; preserve previous password/recovery/escrow and all prior active hardware credentials. Reconcile crash-resume using operation ID, never auto-activate partially tested credentials.
- Sign-in admission reads and re-checks the **current** effective minimum after hardware ceremony. Credentials removed during a pending assertion never issue a normal session. Invalidate session epoch on role/policy changes according to existing auth conventions.
- Protect every alternate entry point: GUI, Wails method, CLI, headless, database repair, account reset, recovery, restore, admin grant, background sync and legacy API handlers. A newer account-security format must not be written by an incompatible binary; old pre-gate binaries cannot be retroactively forced to understand it—document backup/restore and downgrade limitations.
- A copied older `system.db` can restore old password wraps and revoked credential metadata. Offline rollback defenses require platform-protected monotonic state or externally anchored records; absent such a mechanism, do not claim rollback-resistant credential revocation. Record recoverability limits without promising perfect tamper-proofing against an OS administrator.

### 6. OS, packaging, and privacy

- Windows: test native WebAuthn and/or CTAP transport capabilities, user verification, `hmac-secret`, driver prompts and PIV module boundaries on real supported Windows versions.
- macOS: test AuthenticationServices/FIDO2 capability exposure vs direct CTAP2 and PIV services on the signed/notarized app; do not assume Safari's WebAuthn capabilities exist in native Wails.
- Linux: test native libfido2/CTAP HID, udev permission policy and PIV PC/SC/PKCS#11 only as needed; no root-running application requirement.
- Any vetted native adapter is a **host implementation detail**, not a generic public plugin. Signed binary provenance, SBOM, dependency maintenance, verified release bundles and no permission-expanding auto-installation.
- Store minimum needed FIDO attestation details and, when possible, digests rather than identifying device serials. Enterprise attestation may expose device identifiers; disclose, obtain consent, minimize retention, and separate admin inventory visibility from normal users/plugins.

### 7. Security limits and evidence

FIPS 140-3 validates a cryptographic **module under the listed operating conditions**; it does not certify this application's `Argon2id`, `AES-256-GCM`, Go standard crypto, SQLCipher, local OS, or the entire desktop application's cryptographic processing. NIST AAL3 also has verifier, authenticated-channel, phishing resistance and validated-cryptography obligations; a locally enforced Tier 2 does not automatically meet it. Any user with control of the host process/OS may read the DEK after an authorized login. Password and recovery wraps remain alternate at-rest DEK recovery routes; both tier policies state the protection and its limits accurately.

## GoPMgr A-series roadmap v4 — User-optional FIDO2 first; administrator-enforced FIPS 140-3 second

**Decision direction:** Maintainer approved the two-tier feature policy in principle on 2026-10-28. **Execution and release:** not approved; all implementation tests `NOT RUN`. Main GoPMgr beta P0 release blockers and ADR-001/004/005 stability work retain priority. Auth stays independent of ADR-010 provider plugin runtime, ADR-011 normalized schedule storage, Jira, MPXJ and analytics.

### Core architectural path

| Gate | Priority / predecessor | Deliverable | Exit condition |
|---|---|---|---|
| **A-00** | Now, design-only; parallel with release P0 | Approve two-tier policy, meanings of FIPS and device-bound passkey, admin/role jurisdiction, reduced-session recovery, no-bypass requirements | Owner signs ADR and scoped policy; no silently implied code authorization |
| **A-01** | Parallel bounded forensic work | Baseline ADR-001 password/DEK/recovery, ADR-004 escrow, ADR-005 succession, `credentials.go` account-incarnation transaction, account format/CLI/backup coverage | Verified behavior matrix, pristine existing project fixture and source SHA; independent regression oracle |
| **A-02** | POC after A-00; no production migrations | Two hardware classes: broad USB FIDO2 devices for Tier 1 and CMVP-qualified FIPS 140-3 devices for Tier 2; attestation, UV/UP, BE/BS, device uniqueness/privacy, candidate module policies | Model/firmware/approved-service matrix; Tier 2 has authoritative CMVP evidence; unknowns BLOCKED |
| **A-03** | A-02 + baseline security review | Offline actual device DEK-unlock proof. Evaluate CTAP2 `hmac-secret` and, if needed, FIDO2+PIV on Windows/macOS/Linux; qualify each profile/OS independently | Two different real keys each reopen same existing encrypted DEK; signature-only path conclusively rejected as key-only login |
| **A-04** | A-01/A-03, after separate owner code approval | Additive account security schema/format guard, policy precedence and authorization; separate recovery-only session; signed offline FIPS evidence catalog | Upgrade/rollback crash tests; old password/escrow/codes preserved; no cross-account collision or bypass via CLI |
| **A-05** | A-04 | Release-quality **Tier-1 user enrollment** with 2-key wizard, atomic activation, optional key-first sign-in, documented MFA-only mode for keys lacking hardware unseal | Verified credential/DEK on both keys, native PIN path, accessible UI, existing users unaffected, no mandatory FIPS |
| **A-06** | A-05 | User-optional **Tier-1 offline sign-in**, revocation, replacement and backup flows on each advertised platform | Real packaged cold sign-in, logout/DEK zero, old-project reopen, concurrency, no unsafe fallback, recovery pass |
| **A-07** | A-04 + A-02/A-03 Tier-2 proof, after Tier-1 stability | **Tier-2 super-admin policy**: target scope, delegated role bounds, preview, pending readiness, individual enforcement and no-bypass checks | No enforced claim on unready accounts; password cannot grant full routine session; FIPS evidence and 2-key readiness true |
| **A-08** | A-07 | Managed recovery, secure re-provisioning, new-account bootstrap, audit/SBOM/installer evidence, administrator escrow and succession drills | Lost-both-key recovery restricted; breakglass cannot access project; all ADR-004/005 regressions on real machines PASS |
| **A-09** | A-06 and A-08 | Release qualification/expansion to additional tested OS/vendor profiles; later optional enterprise fleet policy or standards review | Signed production packages, benchmark matrix, exact module allowlist, platform truth in release notes, no unsupported certification claim |

**Incremental PR breakdown:** PR-A0 normative approval and decision ledger; PR-A1 baseline auth tests only; PR-A2 feasibility harness/tooling (isolated); PR-A3 neutral hardware qualification adapter and test fixtures; PR-A4 versioned schema + security policy evaluator (dark); PR-A5 Tier-1 enrollment; PR-A6 Tier-1 login and password fallback; PR-A7 super-admin policy and supervised enrollment; PR-A8 recovery/migration; PR-A9 packaging, measured performance and hardening. No enormous auth+database+UI rewrite in one PR.

### Scheduling, coupling and risk

1. **Do now without blocking beta:** A-00 architecture approval, and bounded A-01/A-02 research. Hardware POCs must not jeopardize P0 regression work.
2. **Tier 1 first, but not insecure:** A-03 must show key-mediated DEK unlock before committing to a passwordless sign-in claim. User opt-in requires a confirmed backup and preserves legacy login.
3. **Tier 2 second, without retrofits:** Policy schema, role authority, recovery-restricted session, credential evidence and FIPS allowlist are designed in A-04 even if enforcement UI ships later. Avoid inferring that Tier 1's unrestricted password alternative is acceptable under Tier 2.
4. **No dependency on providers:** Jira/MPXJ work has separate release gates; plugin workers cannot receive keys. Provider plugin updates may use a separately approved fresh-re-auth policy, but are not prerequisites for key login.
5. **Conformance before release:** Unit/fuzz tests are necessary but never replace actual hardware tests, signed binary packaging tests, migration/escrow backup restores, and per-platform driver support. BLOCKED != PASS.

### Feature flags and migration semantics

- `auth.hardware.enrollment_experimental`: internal/false until A-04/05 gates.
- `auth.fido2_optional`: only on a per-OS/per-device profile after qualification. Legacy users default `PASSWORD_COMPAT`; opt-in is not automatic on migration.
- `auth.fips_required`: absent/off by default; enabling requires explicit privileged admin review, proof of two qualified keys for covered accounts, and a recovery operator/route. A global rollout may have `ENFORCEMENT_PENDING` accounts; no implied enforcement for them.
- Old account backups can contain earlier auth wraps/permissions. Document downgrade risk and pre-gate writer limitations; optional future hardware rollback anchoring is a distinct proof gate.
- An offline FIPS evidence snapshot is pinned/signed and periodically updated explicitly; no background Internet service is required. Certificate change policy cannot erase working DEKs and must preserve a tested repair path.

### Minimum release decision criteria

| Release milestone | Required outcome |
|---|---|
| **Tier-1 GA** | All `T1-*` and shared `SEC-*` tests pass on advertised OS/arch, password/DEK/escrow regression complete, both-key test and recovery verified; MFA-only is never marketed as passwordless |
| **Tier-2 GA** | Tier-1 gates plus `T2-*`, `POL-*`, `REC-*` and `OS-*` on real qualified devices, exact CMVP module/service/firmware proof, admin role tests, zero routine password bypass, independently verified recovery drills |
| **Future compliance claims** | Separate verified NIST/FIPS/AAL product/verifier assessment and maintenance procedure—not inferred from a key's CMVP certificate |

### Proposed exact repository insert

The proposed insertion is now in `docs/ROADMAP.md` as the **`### Authentication and account security — tiered hardware security keys`** subsection under `## Planned product work`. The prior `## Current focus` section is unchanged. The work stays proposed; tests have not been run.

## Tiered FIDO2 / FIPS 140-3 acceptance and adversarial verification matrix

**All tests NOT RUN**. Every test below needs an owner, pinned source SHA, fixture version/hash, supported platform/OS build, device model/firmware and raw evidence artifact. Reject fake PASS, muted failures, release-scope reduction or mock-only certification. Tests are additive to the previous security-key and FIDO2 acceptance sets; they specifically guard the new **two-tier** decision.

### Baseline and integration

| ID | Test / assertion |
|---|---|
| SEC-01 | Current password-only accounts, encrypted projects and password-change flows behave identically when hardware feature disabled |
| SEC-02 | Recovery codes, existing DEK wraps, administrator escrow key, attestation/pins, rotation and ADR-005 succession preserve old crypto authority |
| SEC-03 | Account deleted/recreated with same username cannot reuse former hardware credentials or recovery operations |
| SEC-04 | Race hardware challenge with account disable, role change, password reset, changed auth generation, policy epoch, key revocation; no stale session |
| SEC-05 | Session sign-out, process termination, account switch and in-flight cancellation clear/restrict secret references and invalidate session epoch |
| SEC-06 | GUI, Wails, CLI/headless, restore, repair, backup, background jobs, admin and legacy routes all enforce same effective tier |
| SEC-07 | Migrations/backups across last supported account format, corrupted records, stale snapshots and pre-format-gate binaries have documented safe behavior |
| SEC-08 | Verify every saved receipt/audit event and that failed writes never report successful key activation |
| SEC-09 | FIDO challenge, signature, RP hash, signed UV/UP flags, credential ID, extension, assertion type and replay negative suite |
| SEC-10 | Fuzz malformed CBOR/COSE/attestation/AAD/envelopes/lengths; no crash, key exposure or unauthorized auth |

### Tier-1 universally available option

| ID | Test / assertion |
|---|---|
| T1-01 | On an eligible installed build all account types, including standard users, may voluntarily view and opt into FIDO2 USB-key enrollment; no administrator enables ordinary Tier 1 |
| T1-02 | Two physically distinct credential-bearing USB devices independently sign and decrypt *same original* DEK; interrupted enrollment never switches modes |
| T1-03 | USB key with only FIDO2 signature and no qualified key unseal gets truthful **MFA-only** status, never passwordless/key-only status or readiness credit |
| T1-04 | Non-FIPS FIDO2 hardware accepted for Tier 1 only if cryptography/attestation/UV and DEK-unlock constraints are met; no accidental Tier-2 classification |
| T1-05 | User password remains an explicit alternate full login under Tier 1, including cold offline relaunch and no hardware |
| T1-06 | Remove first key, test backup alone; remove backup, test primary alone; reset either key and demonstrate loss/re-enrollment semantics |
| T1-07 | Key enrollment, labels, replacement, expiry/cancel and errors are accessible, keyboard-operable and do not print secrets |
| T1-08 | Symlink/USB HID/device hot swap, native PIN lockout, PC/SC denial and driver failure produce bounded failure not passwordless software fallback |

### FIPS-enforced Tier-2 policy

| ID | Test / assertion |
|---|---|
| T2-01 | CMVP active FIPS 140-3 certificate and its **actual module SKU/firmware/approved mode/approved service** verified; negative FIPS 140-2, MIP, marketing-only, mismatched firmware and out-of-mode fixtures |
| T2-02 | Verified Tier-1-only non-FIPS key cannot authenticate as routine Tier 2 even if credential was formerly enrolled |
| T2-03 | Minimum two distinct qualifying FIPS devices independently unwrap the original DEK before enforcement activation; distinguish actual physical evidence from two credential IDs |
| T2-04 | FIPS approved-service validation includes concrete FIDO2 operation and actual unlock path (`hmac-secret`/PIV); algorithm-only certificate is insufficient |
| T2-05 | Device-bound vs BE/BS flags: synced/exportable/platform passkey rejected from Tier 2 even when user interaction succeeds |
| T2-06 | Exact policy decision trace recorded, including certificate/allowlist revision, module/firmware, credential, role, account incarnation and policy epoch, without sensitive data |
| T2-07 | Unsupported OS, unqualified device, unknown attestation, stale approved-mode assurance or missing driver BLOCK enforcement/activation for the target; no silent password default |
| T2-08 | Signed catalog change, certificate expiration/status change and offline stale metadata follow documented response without deleting existing DEK or password backups |
| T2-09 | Tier-2 login requiring USB key works **entirely offline**, PIN/UV enforced with real key, and after signed packaged application restart |
| T2-10 | A missing backup causes `REPLACEMENT_REQUIRED` while an enrolled approved surviving primary can sign in only under explicitly approved grace/replacement rules |

### Admin, policy, and recovery

| ID | Test / assertion |
|---|---|
| POL-01 | Only current super administrator can enable/disable global mandatory FIPS; subordinate privileges cannot override or self-exempt |
| POL-02 | Explicitly delegated subordinate may enforce—not weaken—already approved minimum on in-scope standard accounts, not super-admin or another administrator |
| POL-03 | User can opt into Tier 1 but cannot downgrade global Tier 2; UI/API/CLI refusal consistent |
| POL-04 | Global enforcement preview counts pending/ineligible accounts; cannot declare all enforced while any in scope unresolved; new account bootstrap restricted appropriately |
| POL-05 | A concurrent role/coverage change between check and commit refuses stale policy enrollment; idempotent retry cannot duplicate/audit false success |
| POL-06 | Password-only attempt on `FIPS1403_REQUIRED` never yields NORMAL_SESSION; returns explicit recovery path/status, even via old entrypoints |
| POL-07 | Unauthorized recovery approval, forged role row, stale session/epoch, copied idempotency token or manual DB policy edit cannot grant normal session within supported threat boundaries |
| POL-08 | Admin's own FIPS enrollment and succession tests comply with ADR-005; role handover does not silently copy user keys or evade recovery obligations |
| REC-01 | Tier-1 password login from cold restart works after both keys disappear; existing recovery codes and escrow unaffected |
| REC-02 | Tier-2 password plus approved independent recovery code/admin authorization creates only scoped RECOVERY_RESTRICTED_SESSION |
| REC-03 | Restricted recovery cannot open/export/write project, run plugins, access cloud connectors, or change admin roles even if DEK in host auth memory |
| REC-04 | With no FIPS keys, recovery can enroll and test replacement primary+backup; only committed successful test returns routine Tier-2 access |
| REC-05 | Lose key during enrollment, process crash, power loss, reset, disk-full, database locked/corrupt or cancelled recovery: no partial activation or loss of password/escrow DEK |
| REC-06 | No password, no physical keys, no recovery code: program accurately describes available escrow-authorized routes and potentially unrecoverable case; no manufactured master override |

### Platforms, performance, and security boundaries

| ID | Test / assertion |
|---|---|
| OS-01 | Windows supported package: tested native CTAP/hmac/PIV, PIN/UV, code signing, USB removal, normal/limited sessions and FIPS qualification |
| OS-02 | macOS arm64 (and x86_64 if advertised) signed/notarized build: real hardware/permissions, genuine native extension and FIPS operation |
| OS-03 | Linux supported distribution: real HID/udev, native libraries, key prompts, no root requirement and frozen dependency inventory |
| OS-04 | Capture cold/warm key sign-in p50/p95, device prompt/cancel timeouts, memory/CPU on approved minimum/recommended hardware with authentic fixtures |
| OS-05 | Inspect Wails bridge, frontend, logs, telemetry, crash reports, plugin IPC, stdout/env and bundle contents for PIN, PRF output, KEK, DEK and device-unique secrets |
| OS-06 | Offline attack/replay/rollback analysis explicitly distinguishes app-enforced policy from OS-admin compromise; no unsupported FIPS/AAL3 claims |

### Evidence and test discipline

- PASS requires direct reproducible evidence for each named platform/device profile. Linux test does not establish Windows, and YubiKey results do not qualify a Hirsch product.
- Mocked CTAP and SQL faults are complementary to actual USB hardware and real signed builds, not replacements.
- Every previously required SK/F2 acceptance test remains required where its invariant still applies; former **blanket FIPS for all Tier-1 keys** tests are superseded by the new tier-specific T1/T2 split.
- Release checks include `make verify`, `make check-release`, schema-version gate, real encrypted project reopen, preserved audit chain and independent red-team review; NOT RUN/BLOCKED is never PASS.
