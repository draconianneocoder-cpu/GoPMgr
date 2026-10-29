<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->
# GoPMgr A-series roadmap v4 — User-optional FIDO2 first; administrator-enforced FIPS 140-3 second

**Decision direction:** Maintainer approved the two-tier feature policy in principle on 2026-10-28. **Execution and release:** not approved; all implementation tests `NOT RUN`. Main GoPMgr beta P0 release blockers and ADR-001/004/005 stability work retain priority. Auth stays independent of ADR-010 provider plugin runtime, ADR-011 normalized schedule storage, Jira, MPXJ and analytics.

## Core architectural path

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

## Scheduling, coupling and risk

1. **Do now without blocking beta:** A-00 architecture approval, and bounded A-01/A-02 research. Hardware POCs must not jeopardize P0 regression work.
2. **Tier 1 first, but not insecure:** A-03 must show key-mediated DEK unlock before committing to a passwordless sign-in claim. User opt-in requires a confirmed backup and preserves legacy login.
3. **Tier 2 second, without retrofits:** Policy schema, role authority, recovery-restricted session, credential evidence and FIPS allowlist are designed in A-04 even if enforcement UI ships later. Avoid inferring that Tier 1's unrestricted password alternative is acceptable under Tier 2.
4. **No dependency on providers:** Jira/MPXJ work has separate release gates; plugin workers cannot receive keys. Provider plugin updates may use a separately approved fresh-re-auth policy, but are not prerequisites for key login.
5. **Conformance before release:** Unit/fuzz tests are necessary but never replace actual hardware tests, signed binary packaging tests, migration/escrow backup restores, and per-platform driver support. BLOCKED != PASS.

## Feature flags and migration semantics

- `auth.hardware.enrollment_experimental`: internal/false until A-04/05 gates.
- `auth.fido2_optional`: only on a per-OS/per-device profile after qualification. Legacy users default `PASSWORD_COMPAT`; opt-in is not automatic on migration.
- `auth.fips_required`: absent/off by default; enabling requires explicit privileged admin review, proof of two qualified keys for covered accounts, and a recovery operator/route. A global rollout may have `ENFORCEMENT_PENDING` accounts; no implied enforcement for them.
- Old account backups can contain earlier auth wraps/permissions. Document downgrade risk and pre-gate writer limitations; optional future hardware rollback anchoring is a distinct proof gate.
- An offline FIPS evidence snapshot is pinned/signed and periodically updated explicitly; no background Internet service is required. Certificate change policy cannot erase working DEKs and must preserve a tested repair path.

## Minimum release decision criteria

| Release milestone | Required outcome |
|---|---|
| **Tier-1 GA** | All `T1-*` and shared `SEC-*` tests pass on advertised OS/arch, password/DEK/escrow regression complete, both-key test and recovery verified; MFA-only is never marketed as passwordless |
| **Tier-2 GA** | Tier-1 gates plus `T2-*`, `POL-*`, `REC-*` and `OS-*` on real qualified devices, exact CMVP module/service/firmware proof, admin role tests, zero routine password bypass, independently verified recovery drills |
| **Future compliance claims** | Separate verified NIST/FIPS/AAL product/verifier assessment and maintenance procedure—not inferred from a key's CMVP certificate |

## Proposed exact repository insert

The proposed insertion is now in `docs/ROADMAP.md` as the **`### Authentication and account security — tiered hardware security keys`** subsection under `## Planned product work`. The prior `## Current focus` section is unchanged. The work stays proposed; tests have not been run.
