<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->
# Critical review and formal decision delta — tiered hardware authentication v4

**Review date:** 2026-10-28. **Policy input:** the maintainer now selects optional FIDO2/passkey USB security keys for all users, plus an administrator-enabled FIPS 140-3-required tier. The earlier blanket FIPS-for-all-user security key constraint is explicitly **superseded**, not silently carried forward. Implementation decisions are provisional and require an approved ADR and real technical proof.

## Changes from v3 / FIDO2 addendum

| Decision ID | Former position | New position | Reason / impact |
|---|---|---|---|
| TP-01 | Every security-key user had to own FIPS-validated hardware | Tier 1 permits compatible **non-FIPS** device-bound FIDO2 USB hardware for all users | Makes useful key authentication broadly accessible without undermining strict environments |
| TP-02 | One key-preferred profile with backup password | Two distinct policy profiles: self-service optional and admin-enforced FIPS | Makes policy authorization, UI, and security claims explicit |
| TP-03 | Password backup was a fully functional alternative for key-preferred sign-in | Tier 1: ordinary alternative; Tier 2: **restricted recovery only** | Prevents mandatory FIPS mode being bypassed by password |
| TP-04 | FIDO2 extension was a possible future addition to PIV | Evaluate FIDO2 CTAP/PRF unlock as common Tier-1 profile, with Tier-2 approved-service qualification; FIDO2+PIV second option | Prevents design lock-in and rework across tiers |
| TP-05 | Admin control of strict mode not fully specified | Existing **super admin** owns installation-wide floor; subordinate only explicit delegation | Preserves ADR-005 and prevents lower-level admins changing their own baseline |
| TP-06 | Two keys required for every key-preferred mode | Keep two-key activation for both tiers; track honest hardware-uniqueness proof | Preserves prior recovery intent but no false claim from two credential IDs |
| TP-07 | Passkeys loosely equated with physical keys | Restrict v1 to device-bound **USB** keys; synced/platform credentials are future profiles | Tightens offline custody and credible FIPS classification |
| TP-08 | FIPS verified by module information in device enrollment | CMVP certificate **plus** exact firmware, module, approved mode and actual selected service | Prevents false product and algorithm-level certification claims |
| TP-09 | High-tier rollout potentially instantaneous | Staged pending/readiness/coverage preview and durable enforcement commit | Avoids accidental mass lockout and false 100%-protected claim |
| TP-10 | Recovery step-up not differentiated from normal login | Explicit `RECOVERY_RESTRICTED` session with no project or plugin access | Allows password backup without violating mandatory key policy |
| TP-11 | Local policy assumed to resist all tampering | Threat model bounds offline enforcement against OS-admin and plaintext `system.db` rollback | Security statement remains honest and testable |
| TP-12 | Hardware test set centered on FIPS-only vendors | Broader Tier-1 compatibility conformance and separate Tier-2 allowlist qualification | Controls future maintenance burden and vendor portability |

## Key adversarial critique

1. **Can an attacker use the existing password instead of a FIPS key?** For Tier 2, an ordinary session is refused; only tightly scoped recovery is possible after independent authorization. This is policy enforcement, not proof that the existing password wrap cannot be decrypted by a person who has the password and copied database metadata.
2. **Can any passkey USB token unlock projects?** No. FIDO signature-only keys have no general way to produce the DEK wrap KEK. Key-first login requires an actual CTAP PRF/HMAC or qualified PIV hardware operation. MFA-only must be labelled accurately.
3. **Are two credentials the same as two physical devices?** No. Device attestation often describes a model, not a unique serial. Tier 2 needs independently reviewed proof/inventory controls and disclosure of unprovable uniqueness. The safety of backup custody is an operational concern.
4. **Can ordinary admins require FIPS without touching the super administrator?** Only explicit delegation and within current ADR-005 powers. The super administrator alone owns installation-wide controls and protected-admin changes.
5. **Can a global policy be flipped on when a user has no compatible key?** No. That account remains pending/blocked, with a provisioning and recovery path. The interface must never claim full enforcement.
6. **Does FIPS 140-3 on a security key validate GoPMgr?** No; it validates a specified cryptographic module when operated as described in its CMVP security policy. The host SQLCipher/Go crypto stack needs independent assurance for stronger system claims.
7. **Can this be enforced offline if someone rewrites the account database?** Only within supported local software/OS trust boundaries. A hostile OS-admin or a pre-format-gate older binary may undermine offline policy; do not fabricate anti-rollback or immutable physical enforcement.
8. **Does retaining password recovery reduce Tier-2 assurance?** Yes, particularly for copied encrypted wraps plus stolen recovery secrets. The benefit is strong ordinary-login enforcement and auditable recovery; do not claim passwords are cryptographically unusable at rest.
9. **Can hardware failure strand an administrator and all accounts?** Keep two-key activation, independent user recovery, account-disable and administrator-escrow controls; rehearse super-admin succession and special key-loss cases *before* enforcement.
10. **Can lower-trust connectors trigger re-enrollment?** No. Keys, PINs, and auth-service privilege remain host-private; no plugin IPC, generic webhooks or AI writes can obtain credential-management capabilities.

## Unresolved release decisions (BLOCKED until proof)

- Exact CTAP2 `hmac-secret` approved-service status and API exposure in each proposed FIPS device/firmware/OS; the alternative FIDO2+PIV binding, if required.
- How high-tier installations independently substantiate two distinct physical devices while respecting attestation/privacy; what to do when only model-level proof is available.
- Exact protected population (all accounts, delegated standard accounts, administrators, future new users) and managed-policy staging semantics.
- Recovery authorization when the super administrator loses both keys or when other admins are unavailable; do not add an undocumented master override.
- Hardware-attested anti-rollback, if desired, versus the supported local same-OS-user threat model; target OS native APIs and signing/packaging.
- Precise database migrations and account-format writer gates across every headless/legacy entrypoint, with ADR-004/005 regression and backup restore.

## Reviewed primary anchors

- NIST CMVP certificate and validation scope: https://csrc.nist.gov/projects/cryptographic-module-validation-program/certificate/5291 and https://csrc.nist.gov/projects/cryptographic-module-validation-program/validated-modules
- NIST 800-63B-4 authenticated hardware and synced-passkey restrictions: https://pages.nist.gov/800-63-4/sp800-63b/aal/
- FIDO CTAP2 `hmac-secret`: https://fidoalliance.org/specs/fido-v2.2-ps-20250714/fido-client-to-authenticator-protocol-v2.2-ps-20250714.html
- FIDO attestation privacy/identity: https://fidoalliance.org/fido-attestation-enhancing-trust-privacy-and-interoperability-in-passwordless-authentication/
- Hirsch uTrust user guide supplied by maintainer: product capabilities only, no authoritative module certificate or service qualification in supplied four pages.
