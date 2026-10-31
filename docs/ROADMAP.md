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

1. Expedite the qualified DuckDB replacement on macOS first, Linux second,
   through the isolated DF0 work below. Production cutover remains a separate
   owner decision.
2. Prevent data loss in stateful editors and verify save, discard, and cancel
   behavior in native application workflows.
3. Complete real-target installation and upgrade evidence for every advertised
   package format.
4. Establish trustworthy distribution: code signing, notarization where
   applicable, and release-certificate PAdES evidence. The Ed25519 signed
   update-manifest mechanism itself (`internal/update`, `tools/update-manifest`)
   is already built and tested end-to-end; what remains is standing up a live
   channel — generating and provisioning the signing keypair as CI secrets so
   `ManifestURL`/`UpdateChannelPublicKey` are actually populated in release
   builds, which today ship with the update channel disabled by design.
5. Improve keyboard and assistive-technology coverage before broad release.

## DuckDB replacement: current delivery sequence

Snapshot baseline: main `6102a28` on 2026-10-31. Linked DF0 records also cover
isolated branch work; that does not mean production integration. DuckDB remains
the shipped analytics engine. The accepted [ADR-012 authority](review/ADR-012-v0.2-ACCEPTED-OWNER-DECISIONS.md)
authorizes the isolated feasibility spike; it does not authorize production
integration, a release-package change or removal of the legacy engine. Do not
upgrade DuckDB as part of its retirement.

1. **Complete macOS containment qualification.** Host-mediated Parquet reads,
   forced no-spill controls, per-child kernel resource accounting and pre-launch
   reservation checks are implemented in the spike (PRs #173–#175). Reservations
   are not actual memory or CPU caps. The [native limit probes](testing/adr-012-df0-macos-limits.md)
   demonstrate why address-space limits, per-process CPU time and process groups
   cannot be substituted for combined RSS/CPU/process-tree enforcement. DF-033
   and full DF-064 remain unqualified; full DF-062 is BLOCKED on independent
   OS-wide filesystem observation and still lacks other required controls. Resolve
   the [containment decision request](review/ADR-012-macos-containment-ACR.md)
   without weakening accepted guarantees by implication.
2. **Finish the remaining DF0 evidence.** Close the security matrix; compare
   DuckDB, DataFusion and Go exact-rollup plus DataFusion fairly (DF0-E), verify
   complete numeric/wire fidelity (E2), and qualify packaging, dependencies and
   licensing separately (F/F2 and LC-GATE). The first
   [macOS E2 typed-cell harness](testing/adr-012-df0-numeric-fidelity.md) now
   checks 10 jobs/60 rows, including JavaScript decoding and decimal-scale
   preservation. Production DTO/renderer use and semantic decoding of nested
   numbers remain unqualified; this is partial E2 evidence. A first
   [macOS portfolio backend characterization](testing/adr-012-df0-portfolio-performance.md)
   checks identical 0/100/1,000-project fixtures across all three candidates,
   including fresh-worker startup and cleanup. All 306 calls match the oracle;
   30 measured calls per provider/size are descriptive evidence, not the required
   controlled cold/warm or ≥1,000-sample/multi-process performance qualification.
   The Go rollup observations justify further qualification, not cutover. The
   [evidence record](testing/adr-012-df0-evidence.md) distinguishes completed
   experiments from NOT-RUN and BLOCKED gates. macOS is the execution priority,
   Linux follows; Windows execution is deferred, not silently removed from the
   accepted platform requirements. Minimum-version and signed-app evidence stay
   separate from development-machine tests.
3. **Obtain the production decision.** Submit DF0-G's go/no-go report and the
   required owner decisions, including budgets and production G1-compatible
   integration. Then implement and qualify Slice J in reviewable stages, with
   rollback and exact behavior/format compatibility. Only after those gates and
   an explicit cutover decision remove DuckDB code, bindings, build tags,
   release paths and obsolete documentation together. Preserve SQLCipher storage
   authority and the Go calculation kernel.

At the GitHub snapshot above there were no open PRs. The qualification branch
`new-merge/df0-macos-limit-qualification` and its successor
`new-merge/df0-numeric-fidelity`, followed by
`new-merge/df0-portfolio-characterization`, hold the current additive evidence
slices. Review/integrate that work and remove the three refs only after a
verified merge. The first two published commits await their scoped PR/hosted-CI
approval; branch publication alone does not qualify them for merge.
The retained legacy branch `claude/slice-b-readonly-access` belongs to closed,
unmerged PR #163.
Review and selectively port useful missing read-only regression fixtures before
retiring it; do not merge its rejected strict-schema/refusal interface wholesale.
Local backup refs and active worktrees need a separate content/ownership check
before deletion. A closed PR or a different test name alone does not establish
that all useful work has been superseded.

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
release or signing policy each need separate approval. Within the scheduling
stream, the proposed next implementation slice is
PS-02 slice 1, a read-only audit, which also awaits authorization. The authorized
DF0 priority above does not grant that scheduling implementation authority.

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
