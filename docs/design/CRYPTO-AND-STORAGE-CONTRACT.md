<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->
# Cryptographic, storage, OS and compatibility contract (proposed)

## 1. Why FIDO authentication is insufficient by itself

GoPMgr presently needs the original per-user random 32-byte DEK to open SQLCipher databases. A WebAuthn/FIDO2 assertion supplies **proof of credential possession**, not a decryptable copy of this DEK. Any key-first architecture must prove a distinct, hardware-gated cryptographic route: qualified CTAP2 `hmac-secret`/PRF to derive a KEK, or FIDO2 plus a qualified PIV private operation, or a later separately approved equivalent. Authentication-only USB keys may supply MFA but not passwordless access to protected project data. Never cheat by caching an indefinitely valid DEK in a software-only file and labelling the sign-in hardware-enforced.

## 2. FIDO2 HMAC secret candidate (non-normative algorithm sketch)

1. Create a **device-bound** FIDO2 credential in the trusted native host with documented client/RP/verifier binding. Require user presence and user verification, no exportable/synced credentials for the physical USB profile. Record its public COSE key, full credential ID and verified attestation.
2. Request/enable the CTAP2 `hmac-secret` extension on credential creation; validate that the actual device and native OS transport expose it. WebAuthn `prf` is related but not the same API. A key without this capability can be MFA-only, not passwordless unless it proves another actual unlock profile.
3. Generate per-credential random **CTAP input salt** and independent **HKDF salt**; use `getAssertion` signed challenge (UV+UP required) to obtain the credential-specific extension output and verify assertion, correct credential ID, signature, RP binding and the extension's result length. Never accept an extension response on an unverified assertion.
4. Derive `KEK = HKDF-SHA256(hmac_output, salt=hkdf_salt, info=canonical('GoPMgr|DEK-wrap|v1', account_incarnation, enrollment_id, profile_version))`; use independent random nonce and AES-256-GCM to wrap the **same existing DEK**, with canonical AAD binding the same identity/profile/evidence and wrapping metadata.
5. Authenticate the AEAD tag on unwrap; check DEK size and an independently stored authenticated verifier/real encrypted project opening. Do not expose the DEK to frontend or connectors. Version all algorithm/encoding decisions; use constant-time comparisons for secrets where relevant.
6. Tier 2 may choose this path **only after** verifying exact CMVP-approved FIDO2 and HMAC operation for the specific module, model, firmware, approved-mode configuration and crypto path. A supported `hmac-secret` extension is **not automatically a FIPS-approved service**.

**Qualification constraint:** The YubiKey 5 FIPS 5.7.4 documentation lists approved FIDO2 profiles but does not establish, by that fact alone, that a particular application use of `hmac-secret` passes GoPMgr's Tier-2 approved-operation requirement. Vendor 5.8 `hmac-secret-mc` capabilities are not evidence for the validated 5.7.4 module. Use device testing and the full module security policy rather than conflating APIs.

## 3. FIDO2 + PIV candidate

- Verify a fresh real FIDO2 assertion with the account's credential first.
- Require a separately authorized PIV operation on the expected token to release/unwrap a per-token KEK/DEK envelope, using only hardware-backed approved algorithms and provisioning as recorded by the actual module security policy.
- Bind FIDO2 and PIV claims to the **same registered physical authenticator** using verified evidence, not a mere matching product name or USB port. Do not claim FIDO+PIV stronger if a malicious actor can present *different* devices to satisfy the two ceremonies.
- Complete nonvolatile key-provisioning/backup design, crypto-agility versioning and tested Windows/macOS/Linux drivers before shipping. A key that supports FIDO but no PIV may be Tier 1 through HMAC or only MFA; profile mismatch never falls back silently.

## 4. Suggested additive tables (illustrative; ADR/database review must supply exact SQL)

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

## 5. Concurrency, crash and version safety

- Prepare cryptographic values outside write lock, then `BEGIN IMMEDIATE` or the repository-approved equivalent. Within one commit, re-check account immutable incarnation, enabled status, role, auth generation, policy epoch, credential state and intended profile; atomically publish device entries, DEK wraps, audit and idempotency receipt.
- Roll back staged enrollment entirely on mismatch; preserve previous password/recovery/escrow and all prior active hardware credentials. Reconcile crash-resume using operation ID, never auto-activate partially tested credentials.
- Sign-in admission reads and re-checks the **current** effective minimum after hardware ceremony. Credentials removed during a pending assertion never issue a normal session. Invalidate session epoch on role/policy changes according to existing auth conventions.
- Protect every alternate entry point: GUI, Wails method, CLI, headless, database repair, account reset, recovery, restore, admin grant, background sync and legacy API handlers. A newer account-security format must not be written by an incompatible binary; old pre-gate binaries cannot be retroactively forced to understand it—document backup/restore and downgrade limitations.
- A copied older `system.db` can restore old password wraps and revoked credential metadata. Offline rollback defenses require platform-protected monotonic state or externally anchored records; absent such a mechanism, do not claim rollback-resistant credential revocation. Record recoverability limits without promising perfect tamper-proofing against an OS administrator.

## 6. OS, packaging, and privacy

- Windows: test native WebAuthn and/or CTAP transport capabilities, user verification, `hmac-secret`, driver prompts and PIV module boundaries on real supported Windows versions.
- macOS: test AuthenticationServices/FIDO2 capability exposure vs direct CTAP2 and PIV services on the signed/notarized app; do not assume Safari's WebAuthn capabilities exist in native Wails.
- Linux: test native libfido2/CTAP HID, udev permission policy and PIV PC/SC/PKCS#11 only as needed; no root-running application requirement.
- Any vetted native adapter is a **host implementation detail**, not a generic public plugin. Signed binary provenance, SBOM, dependency maintenance, verified release bundles and no permission-expanding auto-installation.
- Store minimum needed FIDO attestation details and, when possible, digests rather than identifying device serials. Enterprise attestation may expose device identifiers; disclose, obtain consent, minimize retention, and separate admin inventory visibility from normal users/plugins.

## 7. Security limits and evidence

FIPS 140-3 validates a cryptographic **module under the listed operating conditions**; it does not certify this application's `Argon2id`, `AES-256-GCM`, Go standard crypto, SQLCipher, local OS, or the entire desktop application's cryptographic processing. NIST AAL3 also has verifier, authenticated-channel, phishing resistance and validated-cryptography obligations; a locally enforced Tier 2 does not automatically meet it. Any user with control of the host process/OS may read the DEK after an authorized login. Password and recovery wraps remain alternate at-rest DEK recovery routes; both tier policies state the protection and its limits accurately.
