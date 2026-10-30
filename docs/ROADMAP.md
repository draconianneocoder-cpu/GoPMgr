<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# GoPMgr Roadmap

This roadmap describes intended product direction, not a release promise.
Current release-readiness work is tracked in
[beta-release-backlog.md](beta-release-backlog.md). The next limited alpha is
planned in [next-alpha-release-plan.md](next-alpha-release-plan.md). Decisions
that change architecture belong in [design/](design/).

## Current focus

GoPMgr already provides local project storage, scheduling, earned-value
analysis, charts, documents, export, SQLCipher support, and PDF signing. The
near-term priority is making those workflows dependable for wider testing.

1. Prevent data loss in stateful editors and verify save, discard, and cancel
   behavior in native application workflows.
2. Complete real-target installation and upgrade evidence for every advertised
   package format.
3. Establish trustworthy distribution: code signing, notarization where
   applicable, and release-certificate PAdES evidence. The Ed25519 signed
   update-manifest mechanism itself (`internal/update`, `tools/update-manifest`)
   is already built and tested end-to-end; what remains is standing up a live
   channel — generating and provisioning the signing keypair as CI secrets so
   `ManifestURL`/`UpdateChannelPublicKey` are actually populated in release
   builds, which today ship with the update channel disabled by design.
4. Improve keyboard and assistive-technology coverage before broad release.

## Strategic direction: professional scheduling

On 2026-10-26 the maintainer approved a strategic direction: GoPMgr is to
exceed ProjectLibre Desktop in scheduling correctness, project controls,
interoperability, usability, security, documentation and analysis
([decision record](review/PS-professional-scheduling-ACCEPTED-OWNER-DECISION.md)).
The [modernization plan](design/professional-scheduling-modernization-plan.md)
is the canonical plan. It keeps the accepted architecture (ADR-010, ADR-011's
reserved decisions, ADR-012's gates) and changes none of it.

Approved priority order, with today's status from the
[capability matrix](review/PS-capability-matrix.md):

| Work package | Priority | Status |
| --- | --- | --- |
| PS-00 Verified baseline | Immediate | Partial: budgets (OWN-10) open |
| PS-01 Normalized connected model (ADR-011, ADR-010 S1–S4) | Critical, foundational | Proposed |
| PS-02 Schedule-quality audit | High, early parallel | Proposed: [specification](testing/PS-02-schedule-quality-audit-spec.md) |
| PS-03 File inspection and interoperability (ADR-010 G1–G4) | Critical | Partial |
| PS-04 Large-schedule storage, audit and Gantt | Critical | Blocked by architecture (S0 F1, F2); F3 fixed by RES-A-1 |
| PS-05 Connected scheduling workspace | High | Proposed |
| PS-06 Task and Resource Usage | High | Proposed |
| PS-07 Resource and financial planning | High, after PS-01 and PS-06 | Partial |
| PS-08 Advanced scheduling | Medium | Partial |
| PS-09 Analysis, comparison and reporting (ADR-012) | High, parallel | Partial |
| PS-10 Advanced local-first capabilities | Later, separate approval | Proposed |

Nothing in this table is a release promise. Schema activation, migrations,
financial authority, mandatory dependencies, scheduling-semantic changes and
release or signing policy each need separate approval. The proposed next
implementation slice is PS-02 slice 1, a read-only audit, which also awaits
authorization.

## Planned product work

### Authentication and account security — tiered hardware security keys

- **Tier 1: user-optional FIDO2 USB security keys.** Make an optional, offline-capable, device-bound FIDO2/passkey security-key sign-in capability available to all local users. Enabling key-first passwordless access requires a primary and a separately enrolled backup physical key, independent hardware-mediated unwrap of the user's existing SQLCipher DEK, verified user presence and verification, and preservation of password, recovery codes and administrator escrow. FIDO2 authentication-only devices may support a separately labelled password-plus-key MFA mode, not passwordless project unlock.
- **Tier 2: administrator-enforced FIPS 140-3.** An authorized administrator may turn on a higher security-key requirement. Installation-wide policy is owned by the existing super administrator (ADR-005); subordinate administrators may only apply an already approved policy to authorized standard accounts under explicit delegation. Two independently verified qualifying keys are provisioned before enforcement. Each token must match an **issued NIST CMVP FIPS 140-3** module certificate, hardware/firmware variant, initialization/approved mode, cryptographic service and actual FIDO2+DEK-unlock profile. A password remains available for approved restricted recovery, **not to bypass enforced routine login**.
- **Reliability and recoverability.** Keep the original per-user DEK, encrypted project files, password-derived wrap, recovery codes, authenticated account incarnation, audit and administrator escrow/succession. Require transactional enrollment, no-bypass policy evaluation for GUI/CLI/restore, crash-safe key rotation, independently tested backup key and accessible offline recovery/reenrollment. Disallow synced software passkeys in the initial physical-key profiles. No FIPS/AAL3 full-product claim from a hardware certificate.
- **Roadmap placement.** Define and review architecture, cryptographic profiles, policy scopes, real device/OS proof and risk assessment first (A-00–A-03). Follow with account version gates, Tier-1 enrollment/login (A-04–A-06), then mandatory FIPS policy, privileged recovery and production certification/packaging gates (A-07–A-09). Design/POCs may proceed without disrupting current beta P0 work. Security-key authentication is a trusted host capability, not an ADR-010 plugin, and does not block Jira, MPXJ, scheduling or analytics.

**Companion decisions:** proposed `ADR-TBD-TIERED-SECURITY-KEY-POLICY-v4.md` and the A-00–A-09 roadmap; accepted ADR-001, ADR-004 and ADR-005 retain their current authority. Design-in-principle approval is **not** source code implementation approval.

Approved in principle on 2026-10-28. Tests have not been run. Recorded proposal:

- [ADR-TBD-TIERED-SECURITY-KEY-POLICY-v4.md](design/ADR-TBD-TIERED-SECURITY-KEY-POLICY-v4.md)
- [POLICY-AND-ENROLLMENT-STATE-MACHINE.md](design/POLICY-AND-ENROLLMENT-STATE-MACHINE.md)
- [CRYPTO-AND-STORAGE-CONTRACT.md](design/CRYPTO-AND-STORAGE-CONTRACT.md)
- [ROADMAP-TIERED-SECURITY-KEYS.md](design/ROADMAP-TIERED-SECURITY-KEYS.md)
- [TEST-TIERED-SECURITY-KEYS.md](design/TEST-TIERED-SECURITY-KEYS.md)
- [CRITICAL-REVIEW-AND-DECISION-DELTA.md](review/CRITICAL-REVIEW-AND-DECISION-DELTA.md)

### Provider-neutral integrations

The provider-neutral integration framework was approved in principle on
2026-10-28. The supplemental ADR, contract, staged roadmap, and test plan
are proposed. No implementation work is authorized. The tests have not been
run. C-00 stays design review. C-01 and later packages in the linked roadmap
stay unauthorized.

Current focus and beta P0 release work stay on their current list, ahead of
any Jira connector. A larger-schedule or cloud change to the product vision
needs its own maintainer decision.

SQLCipher remains the authoritative project store. The Go calculation kernel
remains the calculation authority. ADR-011 gates remain in force. DuckDB
stays until Slice J, and only after DF0. Accepted ADR-001, ADR-004, and
ADR-005 remain the authority for current storage, escrow, and succession.

Recorded proposal:

- [ADR-TBD-provider-neutral-integration-framework.md](design/ADR-TBD-provider-neutral-integration-framework.md)
- [CONTRACT-AND-RECOVERY-REFERENCE.md](design/CONTRACT-AND-RECOVERY-REFERENCE.md)
- [ROADMAP-provider-integrations.md](design/ROADMAP-provider-integrations.md)
- [TEST-provider-integrations.md](design/TEST-provider-integrations.md)
- [CRITICAL-REVIEW-and-DECISIONS.md](review/CRITICAL-REVIEW-and-DECISIONS.md)

### Scheduling and controls

- Deeper cost forecasting and formal rebaselining workflows.
- Additional contract, procurement, and stakeholder controls.
- Continued validation of resource leveling and schedule-risk behavior against
  real project data.

### Usability and scale

- Accessible, keyboard-complete workflows.
- Large-project Gantt performance and rendering improvements.
- Internationalization after core interaction patterns are stable.

### Interoperability

- Broader project-data import and export compatibility.
- Optional local collaboration only when file-locking and conflict behavior are
  explicit and testable.
- Community extensions only after a narrow, permissioned application boundary
  has proven safe in production use.

## Guardrails

- Local use must not require a cloud account or mandatory network connection.
- New dependencies need a documented maintenance, license, security, and
  packaging review.
- Project data and export compatibility take precedence over feature breadth.
- Release claims require current, reproducible evidence.

## Implemented architecture decisions

| ADR | Decision | Status |
| --- | --- | --- |
| [ADR-001](design/ADR-001-database-encryption-at-rest.md) | Per-user database encryption at rest | Implemented |
| [ADR-002](design/ADR-002-duckdb-vs-sqlcipher-evaluation.md) | DuckDB analytics and SQLCipher project storage | Implemented |
| [ADR-003](design/ADR-003-gofpdf-to-go-pdf-fpdf-migration.md) | PDF library migration | Implemented |

## Not planned

GoPMgr will not require hosted accounts, mandatory telemetry, or online access
for core project management and export workflows. Real-time collaborative
editing remains out of scope until the project adopts a conflict model that
protects local project data.
