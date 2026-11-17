<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Documentation inventory, 2026-11-17

**Snapshot of:** `main` at `9ab5d91`. The tables below cover all 279 tracked
files under `docs/`, one row per file. Frozen snapshot directories and DF0
data directories get one row each, with a file count.
**Status:** a point-in-time snapshot for the
[reorganization plan](docs-reorganization-plan.md). It is not maintained.
Phase P1 replaces it with `docs/inventory.json`, which a check keeps current.
The appendix lists, for every proposed merge and rename, each other file that
would need updating.

## How this was produced

A one-off scan of `git ls-files` at the snapshot commit produced this
inventory, not a committed tool. The same counts come out only on the same
commit.

| Column | Meaning |
| --- | --- |
| Kind | The plan's kind vocabulary (section 6.1). For frozen snapshots, the kind of the original |
| Status | The plan's status vocabulary. For ADRs, designs, plans and test plans, derived from the file's own status line; "approved in principle" counts as `proposed`. Frozen and governed records follow section 3 of the plan. Set by hand, each matching its own file's text: ADR-002 `accepted` (implemented; only its DuckDB part was superseded); ADR-012 `superseded` ("superseded for the current analytics allocation"); `CRYPTO-AND-STORAGE-CONTRACT.md` `proposed` (its title); `ROADMAP-TIERED-SECURITY-KEYS.md` `proposed` ("approved … in principle"; execution not approved) and `TEST-TIERED-SECURITY-KEYS.md` `proposed` (no status label; "All tests NOT RUN", part of a proposal its ADR calls "PROPOSED, NOT IMPLEMENTED"); `resource-resilience.md` `accepted` ("Required … obligations"); `gosec-adjudication.md` `plan`, `accepted` (owner-approved dispositions) |
| Stated status | For ADRs, designs, plans and engineering records: the file's own status text, quoted and shortened to 60 characters. "none stated" means the file states no status. Read the file before acting on a quote |
| Linked from | How many other tracked documents name the file, by full path or, when its name is unique in the repository, by name. Documents under `website/`, `frontend/` and the code areas are not counted |
| Wired by | *code N*: N files under `scripts/`, `tools/`, `internal/`, `Makefile`, `.github/`, `code-map/`, `REUSE.toml` or `.git*` name it, comments included. *website pin*: `website/quality/source-dependencies.json` pins it by hash. *frontend N*: N frontend files name it |
| Action | What the plan proposes. "Keep" with a reason means the path must not change |

Limits:

- These are counts of mentions, not of reads. Section 2.2 of the plan lists
  the consumers that actually read, check, show or pin files.
- Files with common names, such as `environment.json`, are counted only when
  cited by full path. The DF0 data is cited by directory.
- "Cited by frozen or governed records" is computed: one of the records in
  the plan's section 3 names the file, by path, by unique name or by
  `blob/main` URL.

## Files

### `docs/` (20 files)

| File | Kind | Status | Stated status | Linked from | Wired by | Action |
| --- | --- | --- | --- | ---: | --- | --- |
| `ARCHITECTURE.md` | reference | current |  | 8 | — | Keep (cited by frozen or governed records) |
| `DEPENDENCIES.md` | reference | current |  | 34 | code 3 | Keep (cited by AGENTS.md) |
| `DEVELOPER_HANDBOOK.md` | reference | current |  | 5 | code 20, frontend 8 | Keep (cited by AGENTS.md) |
| `ERROR_HANDLING.md` | reference | current |  | 2 | code 2 | Keep (entry-point reference) |
| `INSTALL.md` | guide | current |  | 12 | code 5, website pin, frontend 1 | Keep (cited by frozen or governed records) |
| `ROADMAP.md` | roadmap | current |  | 20 | code 3, website pin | Keep (cited by frozen or governed records) |
| `SBOM.md` | reference | current |  | 5 | code 4 | Keep (entry-point reference) |
| `SECURITY.md` | reference | current |  | 10 | code 1, website pin | Keep (entry-point reference) |
| `STATUS.md` | status | current |  | 17 | — | Keep (cited by AGENTS.md) |
| `STYLE.md` | reference | current |  | 2 | — | Keep (entry-point reference) |
| `TESTING.md` | reference | current |  | 11 | — | Keep (cited by frozen or governed records) |
| `TEST_COVERAGE_LEDGER.md` | reference | current |  | 9 | code 10, frontend 7 | Keep (cited by AGENTS.md) |
| `VISION.md` | reference | current |  | 9 | — | Keep (cited by frozen or governed records) |
| `beta-release-backlog.md` | plan | current | none stated | 12 | code 4, frontend 3 | Keep (entry-point reference) |
| `branding.md` | reference | current |  | 2 | code 3, website pin | Keep (entry-point reference) |
| `cornucopia-elicitation-2026-09-01.md` | review | historical |  | 2 | — | Rename/move to review/cornucopia-elicitation-2026-09-01.md |
| `next-alpha-release-plan.md` | plan | current | none stated | 6 | — | Keep (entry-point reference) |
| `published-release-tags.txt` | release | current |  | 4 | code 2, website pin | Keep (wired) |
| `release-preflight.md` | guide | current |  | 4 | code 3 | Keep (cited by frozen or governed records) |
| `user-guide.md` | guide | current |  | 6 | code 1, website pin | Keep (cited by frozen or governed records) |

### `docs/agent/` (4 files)

| File | Kind | Status | Stated status | Linked from | Wired by | Action |
| --- | --- | --- | --- | ---: | --- | --- |
| `ADR-010-architecture-preservation-agent-prompt.md` | agent | current |  | 6 | — | Keep (ADR-010/012 governed) |
| `ADR-010-v2.2-AGENT-HANDOFF.md` | agent | current |  | 1 | — | Keep (ADR-010/012 governed) |
| `adr-012-coding-agent-instructions.md` | agent | accepted |  | 5 | code 1 | Keep (ADR-012 governed; body as received, status line maintained) |
| `adr-012-owner-prompt-to-paste.md` | agent | accepted |  | 1 | code 1 | Keep (ADR-012 governed; body as received, status line maintained) |

### `docs/design/` (45 files)

| File | Kind | Status | Stated status | Linked from | Wired by | Action |
| --- | --- | --- | --- | ---: | --- | --- |
| `ADR-001-database-encryption-at-rest.md` | adr | accepted | “Implemented” | 9 | code 1 | Keep (cited by frozen or governed records) |
| `ADR-002-duckdb-vs-sqlcipher-evaluation.md` | adr | accepted | “Implemented; the DuckDB analytics part was superseded on” | 6 | code 1 | Keep (cited by frozen or governed records) |
| `ADR-003-gofpdf-to-go-pdf-fpdf-migration.md` | adr | accepted | “Implemented” | 2 | code 1 | Keep |
| `ADR-004-administrator-access-to-user-data.md` | adr | accepted | “Accepted; phases 1 and 2 built (2026-10-02); phase 3's sign-…” | 9 | — | Keep (cited by frozen or governed records) |
| `ADR-005-super-administrator-and-succession.md` | adr | accepted | “Accepted (owner decisions, 2026-10-05 and 2026-10-07). Part…” | 7 | — | Keep (cited by frozen or governed records) |
| `ADR-006-pdfa-2u-archival-format.md` | adr | accepted | “Implemented” | 2 | code 1 | Keep |
| `ADR-007-ibm-plex-script-coverage.md` | adr | accepted | “Implemented; amended by ADR-008,” | 1 | code 3, frontend 1 | Keep |
| `ADR-008-localization.md` | adr | accepted | “Accepted” | 2 | frontend 3 | Keep |
| `ADR-009-yen-reporting-currency.md` | adr | accepted | “Implemented (merged in #131, 2026-10-23)” | 5 | code 2 | Keep (cited by frozen or governed records) |
| `ADR-010-plugin-architecture.md` | adr | accepted | “ACCEPTED — architecture only, v2.2. The maintainer accepted…” | 9 | — | Keep (ADR-010/011 governed) |
| `ADR-011-normalized-schedule-store.md` | adr | proposed | “Proposed. Draft 1, 2026-10-27.” | 3 | code 1 | Keep (ADR-010/011 governed) |
| `ADR-012-datafusion-analytics-replacement.md` | adr | superseded | “Historical architecture accepted on 2026-10-26 (acceptance r…” | 4 | code 1 | Keep (ADR-012 governed; body as received, status line maintained) |
| `ADR-013-engineering-build-assurance.md` | adr | accepted | “accepted maintainer direction; implementation staged.” | 4 | — | Keep (cited by AGENTS.md) |
| `ADR-014-reporting-integrity-and-lifecycle.md` | adr | proposed | “Proposed detailed architecture; documentation integrated 202…” | 7 | — | Keep |
| `ADR-015-secure-extension-platform.md` | adr | proposed | “PROPOSED integrated design; no new implementation authorizat…” | 7 | — | Keep |
| `ADR-TBD-TIERED-SECURITY-KEY-POLICY-v4.md` | adr | proposed | “POLICY DIRECTION APPROVED BY MAINTAINER IN PRINCIPLE, 2026-1…” | 2 | code 1 | Keep |
| `ADR-TBD-provider-neutral-integration-framework.md` | adr | proposed | “APPROVED IN PRINCIPLE by maintainer on 2026-10-28; normative…” | 5 | code 1 | Keep |
| `CONTRACT-AND-RECOVERY-REFERENCE.md` | design | current | “Design reference. Examples and layouts below are not migrati…” | 4 | code 1 | Keep (a preserved source cites it by URL) |
| `CRYPTO-AND-STORAGE-CONTRACT.md` | design | proposed | none stated; title “Cryptographic, storage, OS and compatibility contract (proposed)” | 2 | code 1 | Merge into design/security-keys-v4-design.md |
| `POLICY-AND-ENROLLMENT-STATE-MACHINE.md` | design | proposed | “Proposed implementation contract. The maintainer's two-tier…” | 2 | code 1 | Merge into design/security-keys-v4-design.md |
| `ROADMAP-TIERED-SECURITY-KEYS.md` | plan | proposed | “Decision direction: Maintainer approved the two-tier feature policy in principle…” | 3 | code 1 | Merge into design/security-keys-v4-design.md |
| `ROADMAP-provider-integrations.md` | plan | proposed | “framework approved in principle on 2026-10-28. A short summa…” | 2 | code 1 | Merge into design/provider-integrations-v2-plan.md |
| `TEST-TIERED-SECURITY-KEYS.md` | strategy | proposed | no status label; opens “All tests NOT RUN” | 2 | code 1 | Merge into design/security-keys-v4-design.md |
| `TEST-provider-integrations.md` | strategy | proposed | “framework approved in principle; this is a proposed test pla…” | 2 | code 1 | Merge into design/provider-integrations-v2-plan.md |
| `accessibility-remaining-work-plan.md` | plan | proposed | “Planning document, 2026-08-24; updated 2026-09-04 — M1 (nati…” | 3 | — | Keep (cited by frozen or governed records) |
| `adr-010-g1-implementation-plan.md` | plan | accepted | “APPROVED by the maintainer on 2026-10-26, under the” | 4 | — | Keep (ADR-010/011 governed) |
| `adr-010-g1-transport-design.md` | design | current | “the basis for G1 implementation. Dated 2026-10-25 and” | 11 | — | Keep (ADR-010/011 governed) |
| `adr-010-implementation-plan.md` | plan | accepted | “ACCEPTED target architecture (maintainer, 2026-10-24; decisi…” | 3 | — | Keep (ADR-010/011 governed) |
| `cost-control-phase-2-lifecycle.md` | design | proposed | “Proposed. Implementation is blocked on the decisions in this” | 3 | code 1 | Keep |
| `dashboard-ia-restructuring-proposal.md` | design | accepted | “R1–R4 implemented 2026-08-17/18 (see §7 for the implementati…” | 4 | frontend 2 | Keep |
| `duckdb-analytics-engine.md` | design | superseded | “Retired 2026-11-02. A Go-native engine replaced DuckDB; see…” | 1 | — | Keep |
| `governed-presentation-only-fx.md` | design | proposed | “Proposed for product and architecture review.” | 4 | — | Keep; receives the superseded options note |
| `portfolio-fx-governance-options.md` | design | superseded | “Superseded.” | 1 | — | Merge into design/governed-presentation-only-fx.md (note) |
| `professional-scheduling-domain-model.md` | design | proposed | “a design proposal for the” | 1 | — | Keep |
| `professional-scheduling-modernization-plan.md` | plan | accepted | “APPROVED strategic direction and priority order” | 7 | — | Keep |
| `project-cost-ledger-scope.md` | design | accepted | “Current Phase 1 product boundary.” | 5 | code 6, frontend 2 | Keep |
| `project-settings-tab-restructuring.md` | design | accepted | “implemented 2026-08-18.” | 2 | frontend 1 | Keep |
| `resource-resilience.md` | design | accepted | “Required planning and acceptance obligations for affected de…” | 15 | — | Keep (cited by AGENTS.md) |
| `reusable-procurement-catalog.md` | design | current | “Catalog foundation, project-ledger procurement detail, and” | 1 | code 1 | Keep |
| `typography-ibm-plex.md` | design | accepted | “Implemented.” | 0 | code 7, frontend 3 | Keep |
| `wails-v3-migration-design.md` | design | accepted | “the migration is merged into main (merge” | 6 | — | Keep |
| `wbs-authenticated-account-identity.md` | design | accepted | “D1–D3 policies approved by the owner on 2026-10-14; implemen…” | 5 | code 3 | Keep |
| `wbs-capability-parity-plan.md` | plan | current | “Product direction and first pure model/validation slice appr…” | 0 | — | Keep |
| `wbs-storage-boundary.md` | design | current | none stated | 4 | code 1 | Keep (cited by frozen or governed records) |
| `wbs-unified-model-contract.md` | design | current | “Pure model and the bounded internal transactional storage fo…” | 5 | code 1 | Keep (cited by frozen or governed records) |

### `docs/design/adr-010/` (7 files)

| File | Kind | Status | Stated status | Linked from | Wired by | Action |
| --- | --- | --- | --- | ---: | --- | --- |
| `README.md` | reference | current |  | 1 | — | Keep |
| `v1-original-plan.md` | design | frozen |  | 1 | code 1 | Keep (frozen; never edit) |
| `v2-critical-review-addendum.md` | design | frozen |  | 2 | code 1 | Keep (frozen; never edit) |
| `v2.0-plan.md` | design | frozen |  | 2 | code 1 | Keep (frozen; never edit) |
| `v2.1/**` | design | frozen |  | 1 | — | Keep (frozen; never edit) (3 files) |

### `docs/design/adr-012/` (11 files)

| File | Kind | Status | Stated status | Linked from | Wired by | Action |
| --- | --- | --- | --- | ---: | --- | --- |
| `README.md` | reference | current |  | 0 | — | Keep |
| `v0.1/**` | design | frozen |  | 0 | — | Keep (frozen; never edit) (4 files) |
| `v0.2-as-submitted/**` | design | frozen |  | 0 | — | Keep (frozen; never edit) (6 files) |

### `docs/design/adr-013/` (1 file)

| File | Kind | Status | Stated status | Linked from | Wired by | Action |
| --- | --- | --- | --- | ---: | --- | --- |
| `v0.3-as-submitted/**` | design | frozen |  | 3 | — | Keep (frozen; never edit) (1 file) |

### `docs/design/reporting-extension/` (6 files)

| File | Kind | Status | Stated status | Linked from | Wired by | Action |
| --- | --- | --- | --- | ---: | --- | --- |
| `GoPMgr_Reporting_Extension_System_Design.docx` | design | current |  | 0 | — | Keep |
| `extension-requirements.json` | design | current |  | 4 | — | Keep |
| `implementation-plan.md` | plan | proposed | “Integrated planning baseline, 2026-11-13, source ac20cbe711d…” | 11 | — | Keep (cited by AGENTS.md) |
| `requirements.json` | design | current |  | 5 | — | Keep |
| `sources/ADR-EXT-001-GoPMgr-Secure-Extension-Platform.md` | design | frozen |  | 4 | — | Keep (frozen; never edit) |
| `sources/GoPMgr_Reporting_Integrity_Architecture_Plan.docx` | evidence-data | frozen |  | 3 | — | Keep (frozen; never edit) |

### `docs/engineering/` (27 files)

| File | Kind | Status | Stated status | Linked from | Wired by | Action |
| --- | --- | --- | --- | ---: | --- | --- |
| `adr-013-actionlint.md` | reference | current | none stated | 8 | — | Keep (cited by AGENTS.md) |
| `adr-013-baseline.md` | reference | current | “P0 complete for the source and available hosts” | 4 | — | Keep (cited by AGENTS.md) |
| `adr-013-checkout-guard-plan.md` | plan | accepted | “owner-approved on 2026-11-05 and implemented in the common G…” | 4 | — | Keep (cited by AGENTS.md) |
| `adr-013-compiler-policy.md` | reference | current | none stated | 4 | — | Keep (ADR-013 record) |
| `adr-013-govulncheck.md` | reference | current | none stated | 4 | — | Keep (cited by AGENTS.md) |
| `adr-013-license-check.md` | reference | current | “implemented under the approved P6-B contract.” | 4 | — | Keep (cited by AGENTS.md) |
| `adr-013-native-make-environment.md` | reference | current | none stated | 5 | — | Keep (cited by AGENTS.md) |
| `adr-013-provisioning.md` | reference | current | none stated | 9 | — | Keep (cited by AGENTS.md) |
| `adr-013-release-license-plan.md` | plan | accepted | “owner-approved on 2026-11-05; implemented and locally qualif…” | 5 | — | Keep (cited by AGENTS.md) |
| `adr-013-release-native-environment.md` | reference | current | none stated | 3 | — | Keep (cited by AGENTS.md) |
| `adr-013-shellcheck-findings.json` | evidence-data | current |  | 1 | code 1 | Keep (ADR-013 record) |
| `adr-013-shellcheck.md` | reference | current | none stated | 7 | — | Keep (cited by AGENTS.md) |
| `adr-013-shfmt-plan.md` | plan | accepted | “maintainer approved on 2026-11-05. Implementation and normal…” | 1 | — | Keep (approved contract its result record cites) |
| `adr-013-shfmt.md` | reference | current | none stated | 7 | — | Keep (cited by AGENTS.md) |
| `adr-013-zizmor-plan.md` | plan | accepted | “maintainer approved on 2026-11-05 after independent plan rev…” | 1 | — | Keep (approved contract its result record cites) |
| `adr-013-zizmor.md` | reference | current | none stated | 8 | — | Keep (cited by AGENTS.md) |
| `code-health-audit-2026-11-09.md` | review | historical | none stated | 3 | — | Keep |
| `decompression-advisory-repair.md` | reference | current | none stated | 2 | — | Keep |
| `engine-advisories.md` | reference | current | none stated | 6 | code 4 | Keep |
| `escrow-field-length.md` | reference | current | none stated | 2 | — | Keep |
| `gnupg-private-staging-plan.md` | plan | accepted | “owner approved Option 1; implemented and locally qualified.…” | 2 | — | Keep |
| `gosec-adjudication.md` | plan | accepted | “owner approved native no-replace publication; implementation…” | 3 | — | Keep |
| `macos26-support-policy.md` | reference | current | none stated | 8 | — | Keep |
| `memory-scan-integrity-plan.md` | plan | accepted | “Option 1 accepted by owner; implemented and locally qualifie…” | 4 | — | Keep (cited by AGENTS.md) |
| `verapdf-provenance.md` | reference | current | none stated | 3 | — | Keep |
| `website-production-ci.md` | reference | current | none stated | 0 | — | Keep |
| `xcrypto-advisory-plan.md` | plan | accepted | “exact upstream pin approved on 2026-11-05; implementation lo…” | 3 | — | Keep |

### `docs/interoperability/` (4 files)

| File | Kind | Status | Stated status | Linked from | Wired by | Action |
| --- | --- | --- | --- | ---: | --- | --- |
| `adr-010-g0-dependency-report.md` | review | historical |  | 2 | — | Keep |
| `adr-010-g1-dependency-review.md` | review | current |  | 4 | — | Keep |
| `adr-012-rust-dependency-security-sbom.md` | review | historical |  | 4 | — | Keep |
| `transport-matrix.md` | review | current |  | 4 | — | Keep |

### `docs/release-notes/` (1 file)

| File | Kind | Status | Stated status | Linked from | Wired by | Action |
| --- | --- | --- | --- | ---: | --- | --- |
| `platform-notice.md` | release | current |  | 2 | code 3 | Keep (wired) |

### `docs/releases/` (1 file)

| File | Kind | Status | Stated status | Linked from | Wired by | Action |
| --- | --- | --- | --- | ---: | --- | --- |
| `v1.1.0-alpha.2.md` | release | historical |  | 1 | website pin | Keep (wired) |

### `docs/review/` (25 files)

| File | Kind | Status | Stated status | Linked from | Wired by | Action |
| --- | --- | --- | --- | ---: | --- | --- |
| `ADR-010-G0-windows-transport-decision.md` | decision | historical |  | 7 | — | Keep |
| `ADR-010-G1-P04-authorization-decision.md` | decision | historical |  | 9 | — | Keep |
| `ADR-010-G1-authorization-clarification-2026-10-28.md` | review | historical |  | 9 | — | Keep (cited by frozen or governed records) |
| `ADR-010-G1-open-questions-decision.md` | decision | historical |  | 7 | — | Keep |
| `ADR-010-v2.2-ACCEPTED-OWNER-DECISIONS.md` | decision | frozen |  | 14 | code 1 | Keep (frozen; never edit) |
| `ADR-010-v2.2-integration-review.md` | review | historical |  | 3 | — | Keep (cited by frozen or governed records) |
| `ADR-010-v2.2-review-and-traceability.md` | review | historical |  | 4 | — | Keep |
| `ADR-012-DUCKDB-RETIREMENT-OWNER-DECISION.md` | decision | historical |  | 17 | code 3, website pin | Keep (cited by AGENTS.md) |
| `ADR-012-macos-containment-ACR.md` | review | historical |  | 5 | — | Keep |
| `ADR-012-v0.2-ACCEPTED-OWNER-DECISIONS.md` | decision | historical |  | 9 | — | Keep (cited by frozen or governed records) |
| `CRITICAL-REVIEW-AND-DECISION-DELTA.md` | review | historical |  | 1 | code 1 | Rename/move to review/security-keys-v4-critical-review.md |
| `CRITICAL-REVIEW-and-DECISIONS.md` | review | historical |  | 3 | code 1 | Rename/move to review/provider-integrations-v2-critical-review.md |
| `LINUX-MINIMUM-UBUNTU-26.04-OWNER-DECISION.md` | decision | historical |  | 5 | — | Keep |
| `PS-04-owner-decisions-2026-10-27.md` | decision | historical |  | 4 | code 1 | Keep |
| `PS-capability-matrix.md` | review | historical |  | 4 | — | Keep |
| `PS-professional-scheduling-ACCEPTED-OWNER-DECISION.md` | decision | historical |  | 2 | — | Keep |
| `TPC-benchmark-exclusion-decision.md` | decision | historical |  | 3 | — | Keep |
| `adr-012-repository-verification.md` | review | historical |  | 3 | — | Keep (cited by frozen or governed records) |
| `adr-012-v0.2-improvements-and-traceability.md` | review | accepted |  | 3 | code 1 | Keep (ADR-012 governed; body as received, status line maintained) |
| `architecture-directive-2026-10-27.md` | decision | historical |  | 7 | code 1 | Keep |
| `evaluation-2026-11-10-response-plan.md` | review | historical |  | 5 | — | Keep |
| `independent-evaluation-2026-11-10.md` | review | historical |  | 2 | — | Keep |
| `owner-decisions-2026-11-14.md` | decision | historical |  | 2 | — | Keep |
| `reporting-extension-integration.md` | review | historical |  | 1 | — | Keep |
| `sec-04-triage.md` | review | historical |  | 2 | code 2, frontend 1 | Keep |

### `docs/testing/` (67 files)

| File | Kind | Status | Stated status | Linked from | Wired by | Action |
| --- | --- | --- | --- | ---: | --- | --- |
| `PS-02-schedule-quality-audit-spec.md` | strategy | current | “Slice 1 was AUTHORIZED for implementation on 2026-11-11” | 5 | code 1 | Keep |
| `account-identity-registry.md` | evidence | historical |  | 3 | — | Keep |
| `account-race-assurance.md` | strategy | current | none stated | 4 | — | Keep |
| `account-race-oracle.md` | evidence | historical |  | 1 | — | Keep |
| `account-transaction-cleanup.md` | evidence | historical |  | 4 | code 1 | Keep |
| `adr-010-case-crosswalk.csv` | evidence-data | current |  | 3 | code 1 | Keep |
| `adr-010-g0-evidence.md` | evidence | historical |  | 5 | — | Keep |
| `adr-010-g0-windows-transport-report.md` | evidence | historical |  | 6 | — | Keep |
| `adr-010-g1-blocked.md` | evidence | historical |  | 5 | — | Keep |
| `adr-010-s0-baseline.md` | evidence | historical |  | 3 | — | Keep |
| `adr-010-test-strategy.md` | strategy | current | “ACCEPTED test strategy (maintainer, 2026-10-24; decision rec…” | 4 | — | Keep (cited by frozen or governed records) |
| `adr-012-datafusion-feasibility-spike.md` | plan | accepted | “ACCEPTED by the maintainer on 2026-10-26 (acceptance record)…” | 4 | code 1 | Keep (ADR-012 governed; body as received, status line maintained) |
| `adr-012-df0-admission.md` | evidence | historical |  | 2 | — | Keep (archived spike evidence) |
| `adr-012-df0-c2.md` | evidence | historical |  | 2 | — | Keep (archived spike evidence) |
| `adr-012-df0-evidence.md` | evidence | historical |  | 11 | — | Keep (cited by frozen or governed records) |
| `adr-012-df0-g-report.md` | evidence | historical |  | 32 | code 1 | Keep (cited by AGENTS.md) |
| `adr-012-df0-macos-limits.md` | evidence | historical |  | 2 | — | Keep (archived spike evidence) |
| `adr-012-df0-memory-only.md` | evidence | historical |  | 3 | — | Keep (archived spike evidence) |
| `adr-012-df0-nested-frames.md` | evidence | historical |  | 2 | — | Keep (archived spike evidence) |
| `adr-012-df0-no-spill.md` | evidence | historical |  | 3 | — | Keep (archived spike evidence) |
| `adr-012-df0-numeric-fidelity.md` | evidence | historical |  | 1 | — | Keep (archived spike evidence) |
| `adr-012-df0-observer-sandbox.md` | evidence | historical |  | 3 | — | Keep (archived spike evidence) |
| `adr-012-df0-portfolio-performance.md` | evidence | historical |  | 1 | — | Keep (archived spike evidence) |
| `adr-012-df0-resources.md` | evidence | historical |  | 3 | — | Keep (archived spike evidence) |
| `adr-012-df0-zstd-assessment.md` | evidence | historical |  | 4 | — | Keep (archived spike evidence) |
| `adr-012-rust-1.99.1.md` | evidence | historical |  | 1 | — | Keep (archived spike evidence) |
| `app-admission-epochs.md` | evidence | historical |  | 1 | code 1 | Keep |
| `credential-coherence.md` | evidence | historical |  | 2 | code 1 | Keep |
| `duckdb-notices-qualification.md` | evidence | historical |  | 1 | — | Keep |
| `duckdb-retirement-parity.md` | evidence | historical |  | 6 | code 4 | Keep |
| `go-1.27.3-security.md` | evidence | historical |  | 1 | — | Keep |
| `id-generation-128-bit.md` | evidence | historical |  | 2 | — | Keep |
| `macos-installer-policy.md` | strategy | current | none stated | 2 | code 1 | Keep |
| `macos-package-lifecycle-2026-10-17.md` | evidence | historical |  | 2 | — | Keep |
| `native-close-macos-2026-10-12.md` | evidence | historical |  | 3 | — | Keep |
| `native-editor-discard-failure-2026-10-16.md` | evidence | historical |  | 2 | — | Keep |
| `native-new-record-quit-2026-10-18.md` | evidence | historical |  | 1 | — | Keep |
| `native-new-record-retry-2026-10-18.md` | evidence | historical |  | 2 | — | Keep |
| `native-sprint-recovery-2026-10-18.md` | evidence | historical |  | 3 | — | Keep |
| `native-sprint-update-2026-10-19.md` | evidence | historical |  | 3 | — | Keep |
| `ps04-own10-qualification-plan.md` | strategy | current | “plan. The maintainer approved preparing it on 2026-10-27 (ow…” | 2 | — | Keep |
| `read-only-project-access.md` | evidence | historical |  | 3 | code 1 | Keep |
| `recent-work-test-plan-2026-10-23.md` | strategy | historical | none stated | 1 | — | Keep (cited by frozen or governed records) |
| `release-race-package-policy.md` | strategy | current | none stated | 0 | — | Keep |
| `reliability-2026-10-12.md` | evidence | historical |  | 0 | — | Keep |
| `reporting-extension-assurance.md` | strategy | current | “Proposed verification contracts, integrated 2026-11-13. No r…” | 6 | — | Keep (cited by AGENTS.md) |
| `resource-calendar-save-atomicity.md` | evidence | historical |  | 2 | — | Merge into testing/editor-save-atomicity.md |
| `resource-overallocation-contract.md` | strategy | current | none stated | 7 | code 4 | Keep |
| `resource-resilience-assurance.md` | strategy | current | “Test specifications, 2026-11-14. Every case below is NOT RUN…” | 14 | — | Keep (cited by AGENTS.md) |
| `sprint-guard-errors.md` | evidence | historical |  | 3 | — | Merge into testing/editor-guard-errors.md |
| `sprint-save-atomicity.md` | evidence | historical |  | 2 | code 1 | Merge into testing/editor-save-atomicity.md |
| `sprint-save-feedback.md` | evidence | historical |  | 2 | — | Keep |
| `sprint-timestamp-boundary.md` | evidence | historical |  | 2 | code 1 | Merge into testing/editor-timestamp-boundary.md |
| `sqlcipher-4.20-upgrade.md` | evidence | historical |  | 8 | code 2 | Keep |
| `sqlcipher-commit-cleanup.md` | evidence | historical |  | 8 | code 1 | Merge into testing/sqlcipher-cleanup-corrections.md |
| `sqlcipher-rollback-cleanup.md` | evidence | historical |  | 3 | code 1 | Merge into testing/sqlcipher-cleanup-corrections.md |
| `sqlcipher-wal-reset.md` | evidence | historical |  | 0 | — | Merge into testing/sqlcipher-cleanup-corrections.md |
| `stakeholder-guard-errors.md` | evidence | historical |  | 2 | — | Merge into testing/editor-guard-errors.md |
| `stakeholder-save-atomicity.md` | evidence | historical |  | 2 | — | Merge into testing/editor-save-atomicity.md |
| `tabular-import-paths.md` | evidence | historical |  | 2 | — | Keep |
| `wails-v3-migration-evidence.md` | evidence | historical |  | 8 | — | Keep |
| `wbs-model-validation-2026-10-13.md` | evidence | historical |  | 2 | — | Merge into testing/wbs-foundation-evidence-2026-10.md |
| `wbs-ordering-2026-10-13.md` | evidence | historical |  | 1 | code 1 | Merge into testing/wbs-foundation-evidence-2026-10.md |
| `wbs-storage-boundary-2026-10-14.md` | evidence | historical |  | 1 | — | Merge into testing/wbs-foundation-evidence-2026-10.md |
| `work-item-guard-errors.md` | evidence | historical |  | 2 | — | Merge into testing/editor-guard-errors.md |
| `work-item-save-atomicity.md` | evidence | historical |  | 2 | code 1 | Merge into testing/editor-save-atomicity.md |
| `work-item-timestamp-boundary.md` | evidence | historical |  | 3 | code 1 | Merge into testing/editor-timestamp-boundary.md |

### `docs/testing/adr-012-df0-records/` (60 files)

| File | Kind | Status | Stated status | Linked from | Wired by | Action |
| --- | --- | --- | --- | ---: | --- | --- |
| `df0-a/**` | evidence-data | historical |  | 4 | — | Keep; cited by directory (5 files) |
| `df0-admission-macos/**` | evidence-data | historical |  | 0 | — | Keep; cited by directory (3 files) |
| `df0-b/**` | evidence-data | historical |  | 2 | — | Keep; cited by directory (4 files) |
| `df0-c/**` | evidence-data | historical |  | 1 | — | Keep; cited by directory (2 files) |
| `df0-c2-macos/**` | evidence-data | historical |  | 1 | — | Keep; cited by directory (2 files) |
| `df0-d2-macos/**` | evidence-data | historical |  | 1 | — | Keep; cited by directory (2 files) |
| `df0-exec-limits-macos/**` | evidence-data | historical |  | 4 | — | Keep; cited by directory (4 files) |
| `df0-f-macos/**` | evidence-data | historical |  | 2 | — | Keep; cited by directory (4 files) |
| `df0-fidelity-macos/**` | evidence-data | historical |  | 0 | — | Keep; cited by directory (2 files) |
| `df0-limits-macos/**` | evidence-data | historical |  | 6 | — | Keep; cited by directory (4 files) |
| `df0-memory-only-macos/**` | evidence-data | historical |  | 8 | — | Keep; cited by directory (6 files) |
| `df0-native-source-macos/**` | evidence-data | historical |  | 3 | — | Keep; cited by directory (2 files) |
| `df0-nested-frames-macos/**` | evidence-data | historical |  | 0 | — | Keep; cited by directory (2 files) |
| `df0-observer-sandbox-macos/**` | evidence-data | historical |  | 8 | — | Keep; cited by directory (6 files) |
| `df0-performance-macos/**` | evidence-data | historical |  | 0 | — | Keep; cited by directory (2 files) |
| `df0-resources-macos/**` | evidence-data | historical |  | 2 | — | Keep; cited by directory (3 files) |
| `df0-sbom-macos/**` | evidence-data | historical |  | 14 | — | Keep; cited by directory (7 files) |

## Appendix: files to update for each merge and rename

Each other file that names a source, found by `git grep` for the source's name at the snapshot commit. Mentions inside the sources and receiving files are rewritten as part of the merge, so they are not listed. **(pinned)** marks a website-pinned file; updating it means re-pinning and re-reviewing its pages (plan D7, phase P2b).

| Destination | Sources | Other files that name a source |
| --- | --- | --- |
| `docs/design/governed-presentation-only-fx.md` | `design/portfolio-fx-governance-options.md` | — |
| `docs/design/provider-integrations-v2-plan.md` | `design/ROADMAP-provider-integrations.md`<br>`design/TEST-provider-integrations.md` | `code-map/recent-decisions.md`, `docs/ROADMAP.md` **(pinned)**, `docs/design/ADR-TBD-provider-neutral-integration-framework.md` |
| `docs/design/security-keys-v4-design.md` | `design/CRYPTO-AND-STORAGE-CONTRACT.md`<br>`design/POLICY-AND-ENROLLMENT-STATE-MACHINE.md`<br>`design/ROADMAP-TIERED-SECURITY-KEYS.md`<br>`design/TEST-TIERED-SECURITY-KEYS.md` | `code-map/recent-decisions.md`, `docs/ROADMAP.md` **(pinned)**, `docs/design/ADR-TBD-TIERED-SECURITY-KEY-POLICY-v4.md`, `docs/review/independent-evaluation-2026-11-10.md` |
| `docs/testing/editor-guard-errors.md` | `testing/sprint-guard-errors.md`<br>`testing/stakeholder-guard-errors.md`<br>`testing/work-item-guard-errors.md` | `docs/ERROR_HANDLING.md`, `docs/TEST_COVERAGE_LEDGER.md`, `docs/beta-release-backlog.md` |
| `docs/testing/editor-save-atomicity.md` | `testing/resource-calendar-save-atomicity.md`<br>`testing/sprint-save-atomicity.md`<br>`testing/stakeholder-save-atomicity.md`<br>`testing/work-item-save-atomicity.md` | `code-map/recent-decisions.md`, `docs/ERROR_HANDLING.md`, `docs/TEST_COVERAGE_LEDGER.md`, `docs/beta-release-backlog.md` |
| `docs/testing/editor-timestamp-boundary.md` | `testing/sprint-timestamp-boundary.md`<br>`testing/work-item-timestamp-boundary.md` | `code-map/recent-decisions.md`, `docs/TEST_COVERAGE_LEDGER.md`, `docs/beta-release-backlog.md`, `docs/testing/native-editor-discard-failure-2026-10-16.md` |
| `docs/testing/sqlcipher-cleanup-corrections.md` | `testing/sqlcipher-commit-cleanup.md`<br>`testing/sqlcipher-rollback-cleanup.md`<br>`testing/sqlcipher-wal-reset.md` | `code-map/recent-decisions.md`, `docs/ARCHITECTURE.md`, `docs/TEST_COVERAGE_LEDGER.md` |
| `docs/testing/wbs-foundation-evidence-2026-10.md` | `testing/wbs-model-validation-2026-10-13.md`<br>`testing/wbs-ordering-2026-10-13.md`<br>`testing/wbs-storage-boundary-2026-10-14.md` | `code-map/recent-decisions.md`, `docs/design/wbs-unified-model-contract.md`, `docs/engineering/code-health-audit-2026-11-09.md`, `docs/testing/reliability-2026-10-12.md` |
| `docs/review/cornucopia-elicitation-2026-09-01.md` | `cornucopia-elicitation-2026-09-01.md` | `.claude/skills/cornucopia-risk-cards/SKILL.md`, `docs/beta-release-backlog.md` |
| `docs/review/provider-integrations-v2-critical-review.md` | `review/CRITICAL-REVIEW-and-DECISIONS.md` | `code-map/recent-decisions.md`, `docs/ROADMAP.md` **(pinned)**, `docs/design/ADR-TBD-provider-neutral-integration-framework.md` |
| `docs/review/security-keys-v4-critical-review.md` | `review/CRITICAL-REVIEW-AND-DECISION-DELTA.md` | `code-map/recent-decisions.md`, `docs/ROADMAP.md` **(pinned)** |
