<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->
# Tiered FIDO2 / FIPS 140-3 acceptance and adversarial verification matrix

**All tests NOT RUN**. Every test below needs an owner, pinned source SHA, fixture version/hash, supported platform/OS build, device model/firmware and raw evidence artifact. Reject fake PASS, muted failures, release-scope reduction or mock-only certification. Tests are additive to the previous security-key and FIDO2 acceptance sets; they specifically guard the new **two-tier** decision.

## Baseline and integration

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

## Tier-1 universally available option

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

## FIPS-enforced Tier-2 policy

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

## Admin, policy, and recovery

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

## Platforms, performance, and security boundaries

| ID | Test / assertion |
|---|---|
| OS-01 | Windows supported package: tested native CTAP/hmac/PIV, PIN/UV, code signing, USB removal, normal/limited sessions and FIPS qualification |
| OS-02 | macOS arm64 (and x86_64 if advertised) signed/notarized build: real hardware/permissions, genuine native extension and FIPS operation |
| OS-03 | Linux supported distribution: real HID/udev, native libraries, key prompts, no root requirement and frozen dependency inventory |
| OS-04 | Capture cold/warm key sign-in p50/p95, device prompt/cancel timeouts, memory/CPU on approved minimum/recommended hardware with authentic fixtures |
| OS-05 | Inspect Wails bridge, frontend, logs, telemetry, crash reports, plugin IPC, stdout/env and bundle contents for PIN, PRF output, KEK, DEK and device-unique secrets |
| OS-06 | Offline attack/replay/rollback analysis explicitly distinguishes app-enforced policy from OS-admin compromise; no unsupported FIPS/AAL3 claims |

## Evidence and test discipline

- PASS requires direct reproducible evidence for each named platform/device profile. Linux test does not establish Windows, and YubiKey results do not qualify a Hirsch product.
- Mocked CTAP and SQL faults are complementary to actual USB hardware and real signed builds, not replacements.
- Every previously required SK/F2 acceptance test remains required where its invariant still applies; former **blanket FIPS for all Tier-1 keys** tests are superseded by the new tier-specific T1/T2 split.
- Release checks include `make verify`, `make check-release`, schema-version gate, real encrypted project reopen, preserved audit chain and independent red-team review; NOT RUN/BLOCKED is never PASS.
