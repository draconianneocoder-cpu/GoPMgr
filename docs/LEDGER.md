<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Documentation ledger

Generated from [`ledger.json`](ledger.json) by `make docs-ledger`. Do not edit
this file; edit the ledger and regenerate. The rules are in section 6 of the
[reorganization plan](design/docs-reorganization-plan.md#6-documentation-ledger).
Find a document by ID with `make docs-ledger find=DOC-0001`.

## `docs/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0001 | [`ARCHITECTURE.md`](ARCHITECTURE.md) | reference | current | Architecture: runtime shape, data layout, packages, frontend and release architecture. |
| DOC-0002 | [`DEPENDENCIES.md`](DEPENDENCIES.md) | reference | current | Dependencies: toolchain, Go and frontend dependencies, external tools and change rules. |
| DOC-0003 | [`DEVELOPER_HANDBOOK.md`](DEVELOPER_HANDBOOK.md) | reference | current | GoPMgr Developer Handbook |
| DOC-0004 | [`ERROR_HANDLING.md`](ERROR_HANDLING.md) | reference | current | Error handling: sentinel errors, wrapping, diagnostics, fail-soft and fail-hard behaviour. |
| DOC-0005 | [`INSTALL.md`](INSTALL.md) | guide | current | Installing & running GoPMgr |
| DOC-0240 | [`LEDGER.md`](LEDGER.md) | index | current | Documentation ledger, generated: every document with its ID, kind and status. |
| DOC-0239 | [`README.md`](README.md) | index | current | Documentation map: where to start, by reader and task. |
| DOC-0006 | [`ROADMAP.md`](ROADMAP.md) | roadmap | current | GoPMgr Roadmap |
| DOC-0007 | [`SBOM.md`](SBOM.md) | reference | current | Software bill of materials (SBOM) |
| DOC-0008 | [`SECURITY.md`](SECURITY.md) | reference | current | Security: local accounts, encryption, secrets, document security, audit integrity and release safety. |
| DOC-0009 | [`STATUS.md`](STATUS.md) | status | current | GoPMgr current status |
| DOC-0010 | [`STYLE.md`](STYLE.md) | reference | current | Style: repository, Go, frontend, documentation and security-sensitive coding conventions. |
| DOC-0011 | [`TESTING.md`](TESTING.md) | reference | current | Testing: local, frontend, race, PDF, PAdES and release test guidance. |
| DOC-0012 | [`TEST_COVERAGE_LEDGER.md`](TEST_COVERAGE_LEDGER.md) | reference | current | GoPMgr Test Coverage Ledger |
| DOC-0013 | [`VISION.md`](VISION.md) | reference | current | GoPMgr Vision |
| DOC-0018 | [`beta-release-backlog.md`](beta-release-backlog.md) | plan | current | Beta release backlog |
| DOC-0019 | [`branding.md`](branding.md) | reference | current | GoPMgr branding assets |
| DOC-0241 | [`ledger.json`](ledger.json) | index | current | Documentation ledger: the source of truth for every document under docs/. |
| DOC-0124 | [`next-alpha-release-plan.md`](next-alpha-release-plan.md) | plan | current | Next alpha release plan |
| DOC-0125 | [`published-release-tags.txt`](published-release-tags.txt) | release | current | Published release tags, one per line; read by the release reference-truth check. |
| DOC-0127 | [`release-preflight.md`](release-preflight.md) | guide | current | Release pre-flight checklist |
| DOC-0238 | [`user-guide.md`](user-guide.md) | guide | current | GoPMgr User Guide |

## `docs/agent/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0014 | [`ADR-010-architecture-preservation-agent-prompt.md`](agent/ADR-010-architecture-preservation-agent-prompt.md) | agent | current | Coding-agent master prompt — Execute ADR-010 without architectural degradation |
| DOC-0015 | [`ADR-010-v2.2-AGENT-HANDOFF.md`](agent/ADR-010-v2.2-AGENT-HANDOFF.md) | agent | current | Coding agent — maintainer acceptance of ADR-010 v2.2 |
| DOC-0242 | [`README.md`](agent/README.md) | index | current | Agent instructions folder: what is here and how to use it. |
| DOC-0016 | [`adr-012-coding-agent-instructions.md`](agent/adr-012-coding-agent-instructions.md) | agent | accepted | Coding-agent directive: ADR-012 v0.2 DataFusion feasibility without architectural compromise |
| DOC-0017 | [`adr-012-owner-prompt-to-paste.md`](agent/adr-012-owner-prompt-to-paste.md) | agent | accepted | GoPMgr ADR-012 v0.2 — Maintainer Prompt for a Coding Agent |

## `docs/design/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0021 | [`ADR-001-database-encryption-at-rest.md`](design/ADR-001-database-encryption-at-rest.md) | adr | accepted | ADR-001: Per-user database encryption at rest |
| DOC-0022 | [`ADR-002-duckdb-vs-sqlcipher-evaluation.md`](design/ADR-002-duckdb-vs-sqlcipher-evaluation.md) | adr | accepted | ADR-002: DuckDB analytics and SQLCipher project storage |
| DOC-0023 | [`ADR-003-gofpdf-to-go-pdf-fpdf-migration.md`](design/ADR-003-gofpdf-to-go-pdf-fpdf-migration.md) | adr | accepted | ADR-003: PDF library migration |
| DOC-0024 | [`ADR-004-administrator-access-to-user-data.md`](design/ADR-004-administrator-access-to-user-data.md) | adr | accepted | ADR-004: Administrator access to users' data |
| DOC-0025 | [`ADR-005-super-administrator-and-succession.md`](design/ADR-005-super-administrator-and-succession.md) | adr | accepted | ADR-005: Super administrator and succession |
| DOC-0026 | [`ADR-006-pdfa-2u-archival-format.md`](design/ADR-006-pdfa-2u-archival-format.md) | adr | accepted | ADR-006: PDF/A-2u as the archival PDF format |
| DOC-0027 | [`ADR-007-ibm-plex-script-coverage.md`](design/ADR-007-ibm-plex-script-coverage.md) | adr | accepted | ADR-007: Which IBM Plex packages GoPMgr bundles |
| DOC-0028 | [`ADR-008-localization.md`](design/ADR-008-localization.md) | adr | accepted | ADR-008: Interface, Help, and document languages |
| DOC-0029 | [`ADR-009-yen-reporting-currency.md`](design/ADR-009-yen-reporting-currency.md) | adr | accepted | ADR-009: Yen as a reporting currency |
| DOC-0030 | [`ADR-010-plugin-architecture.md`](design/ADR-010-plugin-architecture.md) | adr | accepted | ADR-010: Local-first interoperable worker architecture, scalable project interchange, and future extension boundaries |
| DOC-0031 | [`ADR-011-normalized-schedule-store.md`](design/ADR-011-normalized-schedule-store.md) | adr | proposed | ADR-011: Normalized schedule store, project format gate and encrypted migration |
| DOC-0032 | [`ADR-012-datafusion-analytics-replacement.md`](design/ADR-012-datafusion-analytics-replacement.md) | adr | superseded | ADR-012: Replace DuckDB analytics with an Apache DataFusion/Arrow worker |
| DOC-0033 | [`ADR-013-engineering-build-assurance.md`](design/ADR-013-engineering-build-assurance.md) | adr | accepted | ADR-013: Make-centered engineering build assurance |
| DOC-0034 | [`ADR-014-reporting-integrity-and-lifecycle.md`](design/ADR-014-reporting-integrity-and-lifecycle.md) | adr | proposed | ADR-014 Reporting integrity and lifecycle documentation |
| DOC-0035 | [`ADR-015-secure-extension-platform.md`](design/ADR-015-secure-extension-platform.md) | adr | proposed | ADR-015: Capability-Secure, Resource-Bounded, Fault-Isolated Extension Platform |
| DOC-0036 | [`ADR-TBD-TIERED-SECURITY-KEY-POLICY-v4.md`](design/ADR-TBD-TIERED-SECURITY-KEY-POLICY-v4.md) | adr | proposed | ADR-TBD — Tiered local FIDO2 hardware authentication, FIPS 140-3 administrative enforcement, and encrypted-project… |
| DOC-0037 | [`ADR-TBD-provider-neutral-integration-framework.md`](design/ADR-TBD-provider-neutral-integration-framework.md) | adr | proposed | ADR-TBD — Provider-neutral external work-management integration framework |
| DOC-0038 | [`CONTRACT-AND-RECOVERY-REFERENCE.md`](design/CONTRACT-AND-RECOVERY-REFERENCE.md) | design | current | Contract and recovery reference — provider-neutral framework v2.0 |
| DOC-0039 | [`CRYPTO-AND-STORAGE-CONTRACT.md`](design/CRYPTO-AND-STORAGE-CONTRACT.md) | design | proposed | Cryptographic, storage, OS and compatibility contract (proposed) |
| DOC-0040 | [`POLICY-AND-ENROLLMENT-STATE-MACHINE.md`](design/POLICY-AND-ENROLLMENT-STATE-MACHINE.md) | design | proposed | Policy, authority, state machine, and recovery reference (v4) |
| DOC-0243 | [`README.md`](design/README.md) | index | current | Design folder: the ADR register, then designs and plans. |
| DOC-0041 | [`ROADMAP-TIERED-SECURITY-KEYS.md`](design/ROADMAP-TIERED-SECURITY-KEYS.md) | plan | proposed | GoPMgr A-series roadmap v4 — User-optional FIDO2 first; administrator-enforced FIPS 140-3 second |
| DOC-0042 | [`ROADMAP-provider-integrations.md`](design/ROADMAP-provider-integrations.md) | plan | proposed | Proposed GoPMgr roadmap insertion — provider-neutral integrations |
| DOC-0043 | [`TEST-TIERED-SECURITY-KEYS.md`](design/TEST-TIERED-SECURITY-KEYS.md) | strategy | proposed | Tiered FIDO2 / FIPS 140-3 acceptance and adversarial verification matrix |
| DOC-0044 | [`TEST-provider-integrations.md`](design/TEST-provider-integrations.md) | strategy | proposed | Provider integration assurance, adversarial validation and acceptance |
| DOC-0045 | [`accessibility-remaining-work-plan.md`](design/accessibility-remaining-work-plan.md) | plan | proposed | Accessibility — coverage map and remaining work |
| DOC-0046 | [`adr-010-g1-implementation-plan.md`](design/adr-010-g1-implementation-plan.md) | plan | accepted | ADR-010 G1 implementation plan |
| DOC-0047 | [`adr-010-g1-transport-design.md`](design/adr-010-g1-transport-design.md) | design | current | ADR-010 G1 worker connection design (transport-neutral) |
| DOC-0048 | [`adr-010-implementation-plan.md`](design/adr-010-implementation-plan.md) | plan | accepted | ADR-010 v2.2 — Detailed engineering blueprint and phased implementation plan |
| DOC-0068 | [`cost-control-phase-2-lifecycle.md`](design/cost-control-phase-2-lifecycle.md) | design | proposed | Cost Control Phase 2 lifecycle and authority contract |
| DOC-0069 | [`dashboard-ia-restructuring-proposal.md`](design/dashboard-ia-restructuring-proposal.md) | design | accepted | Dashboard information-architecture restructuring — design proposal |
| DOC-0070 | [`docs-reorganization-inventory.md`](design/docs-reorganization-inventory.md) | plan | historical | Documentation inventory, 2026-11-17 |
| DOC-0071 | [`docs-reorganization-plan.md`](design/docs-reorganization-plan.md) | plan | accepted | Documentation structure: reorganization and consolidation plan |
| DOC-0072 | [`duckdb-analytics-engine.md`](design/duckdb-analytics-engine.md) | design | superseded | DuckDB analytics engine |
| DOC-0073 | [`governed-presentation-only-fx.md`](design/governed-presentation-only-fx.md) | design | proposed | Governed presentation-only FX for GoPMgr Portfolio reporting |
| DOC-0075 | [`professional-scheduling-domain-model.md`](design/professional-scheduling-domain-model.md) | design | proposed | Connected project model: domains, dependencies and open ADR-011 decisions |
| DOC-0076 | [`professional-scheduling-modernization-plan.md`](design/professional-scheduling-modernization-plan.md) | plan | accepted | Professional scheduling and project controls modernization plan |
| DOC-0077 | [`project-cost-ledger-scope.md`](design/project-cost-ledger-scope.md) | design | accepted | Project cost ledger scope |
| DOC-0078 | [`project-settings-tab-restructuring.md`](design/project-settings-tab-restructuring.md) | design | accepted | Project Settings tab restructuring |
| DOC-0085 | [`resource-resilience.md`](design/resource-resilience.md) | design | accepted | Resource resilience: required development contract |
| DOC-0086 | [`reusable-procurement-catalog.md`](design/reusable-procurement-catalog.md) | design | current | Reusable procurement catalog |
| DOC-0087 | [`typography-ibm-plex.md`](design/typography-ibm-plex.md) | design | accepted | Typography: the IBM Plex superfamily |
| DOC-0088 | [`wails-v3-migration-design.md`](design/wails-v3-migration-design.md) | design | accepted | Wails v3 migration design |
| DOC-0089 | [`wbs-authenticated-account-identity.md`](design/wbs-authenticated-account-identity.md) | design | accepted | Immutable account identity for authenticated WBS operations |
| DOC-0090 | [`wbs-capability-parity-plan.md`](design/wbs-capability-parity-plan.md) | plan | current | Connected WBS capability and usability plan |
| DOC-0091 | [`wbs-storage-boundary.md`](design/wbs-storage-boundary.md) | design | current | WBS transactional storage foundation |
| DOC-0092 | [`wbs-unified-model-contract.md`](design/wbs-unified-model-contract.md) | design | current | Unified local-first WBS model contract |

## `docs/design/adr-010/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0049 | [`README.md`](design/adr-010/README.md) | reference | current | ADR-010 reference documents |
| DOC-0050 | [`v1-original-plan.md`](design/adr-010/v1-original-plan.md) | design | frozen | ADR-010: Secure, Local-First Plugin Architecture for GoPMgr |
| DOC-0051 | [`v2-critical-review-addendum.md`](design/adr-010/v2-critical-review-addendum.md) | design | frozen | Critical Design Review and Improvement Addendum — GoPMgr ADR-010 v2.0 |
| DOC-0052 | [`v2.0-plan.md`](design/adr-010/v2.0-plan.md) | design | frozen | ADR-010: Secure, Local-First Plugin Architecture for GoPMgr |

## `docs/design/adr-010/v2.1/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0053 | [`ADR-010-plugin-architecture.md`](design/adr-010/v2.1/ADR-010-plugin-architecture.md) | design | frozen | ADR-010: Interchange workers and the data platform for large schedules |
| DOC-0054 | [`adr-010-implementation-plan.md`](design/adr-010/v2.1/adr-010-implementation-plan.md) | design | frozen | ADR-010 implementation plan (v2.1) |
| DOC-0055 | [`adr-010-test-strategy.md`](design/adr-010/v2.1/adr-010-test-strategy.md) | design | frozen | ADR-010 test strategy |

## `docs/design/adr-012/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0056 | [`README.md`](design/adr-012/README.md) | reference | current | ADR-012 document set |

## `docs/design/adr-012/v0.1/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0057 | [`README.md`](design/adr-012/v0.1/README.md) | design | frozen | GoPMgr — ADR-012 DataFusion replacement design bundle |

## `docs/design/adr-012/v0.1/docs/agent/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0058 | [`adr-012-coding-agent-instructions.md`](design/adr-012/v0.1/docs/agent/adr-012-coding-agent-instructions.md) | design | frozen | Coding-agent directive: ADR-012 DataFusion feasibility without architectural compromise |

## `docs/design/adr-012/v0.1/docs/design/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0059 | [`ADR-012-datafusion-analytics-replacement.md`](design/adr-012/v0.1/docs/design/ADR-012-datafusion-analytics-replacement.md) | design | frozen | ADR-012: Replace DuckDB analytics with an Apache DataFusion/Arrow worker |

## `docs/design/adr-012/v0.1/docs/testing/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0060 | [`adr-012-datafusion-feasibility-spike.md`](design/adr-012/v0.1/docs/testing/adr-012-datafusion-feasibility-spike.md) | design | frozen | ADR-012 / DF0: DataFusion-for-DuckDB replacement feasibility spike |

## `docs/design/adr-012/v0.2-as-submitted/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0061 | [`README.md`](design/adr-012/v0.2-as-submitted/README.md) | design | frozen | GoPMgr ADR-012 v0.2 — DataFusion replacement architecture and feasibility bundle |

## `docs/design/adr-012/v0.2-as-submitted/docs/agent/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0062 | [`adr-012-coding-agent-instructions.md`](design/adr-012/v0.2-as-submitted/docs/agent/adr-012-coding-agent-instructions.md) | design | frozen | Coding-agent directive: ADR-012 v0.2 DataFusion feasibility without architectural compromise |
| DOC-0063 | [`adr-012-owner-prompt-to-paste.md`](design/adr-012/v0.2-as-submitted/docs/agent/adr-012-owner-prompt-to-paste.md) | design | frozen | GoPMgr ADR-012 v0.2 — Maintainer Prompt for a Coding Agent |

## `docs/design/adr-012/v0.2-as-submitted/docs/design/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0064 | [`ADR-012-datafusion-analytics-replacement.md`](design/adr-012/v0.2-as-submitted/docs/design/ADR-012-datafusion-analytics-replacement.md) | design | frozen | ADR-012: Replace DuckDB analytics with an Apache DataFusion/Arrow worker |

## `docs/design/adr-012/v0.2-as-submitted/docs/review/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0065 | [`adr-012-v0.2-improvements-and-traceability.md`](design/adr-012/v0.2-as-submitted/docs/review/adr-012-v0.2-improvements-and-traceability.md) | design | frozen | ADR-012 v0.2 — Critical Review, Amendment Traceability and Approval Gates |

## `docs/design/adr-012/v0.2-as-submitted/docs/testing/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0066 | [`adr-012-datafusion-feasibility-spike.md`](design/adr-012/v0.2-as-submitted/docs/testing/adr-012-datafusion-feasibility-spike.md) | design | frozen | ADR-012 / DF0: DataFusion-for-DuckDB replacement feasibility spike — v0.2 |

## `docs/design/adr-013/v0.3-as-submitted/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0067 | [`approved-phased-plan.md`](design/adr-013/v0.3-as-submitted/approved-phased-plan.md) | design | frozen | GoPMgr ADR-013: Maintainer-Approved Phased Engineering Build and Assurance Implementation Plan |

## `docs/design/reporting-extension/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0079 | [`GoPMgr_Reporting_Extension_System_Design.docx`](design/reporting-extension/GoPMgr_Reporting_Extension_System_Design.docx) | design | current | Reporting and extension system design, as supplied (DOCX). |
| DOC-0080 | [`extension-requirements.json`](design/reporting-extension/extension-requirements.json) | design | current | Extension (EXT) requirement register (proposed target requirements). |
| DOC-0081 | [`implementation-plan.md`](design/reporting-extension/implementation-plan.md) | plan | proposed | Reporting and extension implementation sequence |
| DOC-0082 | [`requirements.json`](design/reporting-extension/requirements.json) | design | current | Reporting requirement register (proposed target requirements). |

## `docs/design/reporting-extension/sources/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0083 | [`ADR-EXT-001-GoPMgr-Secure-Extension-Platform.md`](design/reporting-extension/sources/ADR-EXT-001-GoPMgr-Secure-Extension-Platform.md) | design | frozen | ADR-EXT-001: Capability-Secure, Resource-Bounded, Fault-Isolated Extension Platform |
| DOC-0084 | [`GoPMgr_Reporting_Integrity_Architecture_Plan.docx`](design/reporting-extension/sources/GoPMgr_Reporting_Integrity_Architecture_Plan.docx) | evidence-data | frozen | Reporting integrity architecture plan, preserved as received (DOCX). |

## `docs/engineering/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0244 | [`README.md`](engineering/README.md) | index | current | Engineering folder: ADR-013 build-assurance records, then other engineering records. |
| DOC-0093 | [`adr-013-actionlint.md`](engineering/adr-013-actionlint.md) | reference | current | ADR-013 P4: local workflow correctness |
| DOC-0094 | [`adr-013-baseline.md`](engineering/adr-013-baseline.md) | reference | current | ADR-013 P0: baseline reconciliation |
| DOC-0095 | [`adr-013-checkout-guard-plan.md`](engineering/adr-013-checkout-guard-plan.md) | plan | accepted | ADR-013 P6: checkout credential guard |
| DOC-0096 | [`adr-013-compiler-policy.md`](engineering/adr-013-compiler-policy.md) | reference | current | ADR-013 P6-E: native compiler and deployment-target characterization |
| DOC-0097 | [`adr-013-govulncheck.md`](engineering/adr-013-govulncheck.md) | reference | current | ADR-013 P6-C: immutable govulncheck installation |
| DOC-0098 | [`adr-013-license-check.md`](engineering/adr-013-license-check.md) | reference | current | ADR-013 P6-B: separate licensing validation from cleanup |
| DOC-0099 | [`adr-013-native-make-environment.md`](engineering/adr-013-native-make-environment.md) | reference | current | P6-E: native Make test environments |
| DOC-0100 | [`adr-013-provisioning.md`](engineering/adr-013-provisioning.md) | reference | current | ADR-013 P1: auxiliary-tool provisioning |
| DOC-0101 | [`adr-013-release-license-plan.md`](engineering/adr-013-release-license-plan.md) | plan | accepted | ADR-013 P6-D: mandatory release licensing contract |
| DOC-0102 | [`adr-013-release-native-environment.md`](engineering/adr-013-release-native-environment.md) | reference | current | P6-E: standalone release and validator environments |
| DOC-0103 | [`adr-013-shellcheck-findings.json`](engineering/adr-013-shellcheck-findings.json) | evidence-data | current | ShellCheck findings inventory adjudicated for ADR-013 P2. |
| DOC-0104 | [`adr-013-shellcheck.md`](engineering/adr-013-shellcheck.md) | reference | current | ADR-013 P2: ShellCheck inventory and adjudication |
| DOC-0105 | [`adr-013-shfmt-plan.md`](engineering/adr-013-shfmt-plan.md) | plan | accepted | ADR-013 P3: approved shfmt contract |
| DOC-0106 | [`adr-013-shfmt.md`](engineering/adr-013-shfmt.md) | reference | current | ADR-013 P3: local shell formatting check |
| DOC-0107 | [`adr-013-zizmor-plan.md`](engineering/adr-013-zizmor-plan.md) | plan | accepted | ADR-013 P5: approved local security policy |
| DOC-0108 | [`adr-013-zizmor.md`](engineering/adr-013-zizmor.md) | reference | current | ADR-013 P5: local workflow security policy |
| DOC-0109 | [`code-health-audit-2026-11-09.md`](engineering/code-health-audit-2026-11-09.md) | review | historical | Dead and inefficient code audit, 2026-11-09 |
| DOC-0110 | [`decompression-advisory-repair.md`](engineering/decompression-advisory-repair.md) | reference | current | Decompression dependency repair |
| DOC-0111 | [`engine-advisories.md`](engineering/engine-advisories.md) | reference | current | Vendored engine advisories (SEC-06) |
| DOC-0112 | [`escrow-field-length.md`](engineering/escrow-field-length.md) | reference | current | Escrow context field-length repair |
| DOC-0113 | [`gnupg-private-staging-plan.md`](engineering/gnupg-private-staging-plan.md) | plan | accepted | GnuPG signature staging repair plan |
| DOC-0114 | [`gosec-adjudication.md`](engineering/gosec-adjudication.md) | plan | accepted | Gosec findings: adjudication and restore repair plan |
| DOC-0115 | [`macos26-support-policy.md`](engineering/macos26-support-policy.md) | reference | current | macOS 26 Tahoe minimum support policy |
| DOC-0116 | [`memory-scan-integrity-plan.md`](engineering/memory-scan-integrity-plan.md) | plan | accepted | Memory-safety scanner integrity repair plan |
| DOC-0117 | [`verapdf-provenance.md`](engineering/verapdf-provenance.md) | reference | current | PDF/A validator version and failure handling |
| DOC-0118 | [`website-production-ci.md`](engineering/website-production-ci.md) | reference | current | Website production verification in CI |
| DOC-0119 | [`xcrypto-advisory-plan.md`](engineering/xcrypto-advisory-plan.md) | plan | accepted | x/crypto advisory: bounded dependency repair |

## `docs/interoperability/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0245 | [`README.md`](interoperability/README.md) | index | current | Interoperability folder: dependency reviews and the transport matrix. |
| DOC-0120 | [`adr-010-g0-dependency-report.md`](interoperability/adr-010-g0-dependency-report.md) | review | historical | ADR-010 G0 dependency, native-code and license inventory |
| DOC-0121 | [`adr-010-g1-dependency-review.md`](interoperability/adr-010-g1-dependency-review.md) | review | current | ADR-010 G1 dependency review (for approval) |
| DOC-0122 | [`adr-012-rust-dependency-security-sbom.md`](interoperability/adr-012-rust-dependency-security-sbom.md) | review | historical | ADR-012 Rust dependency and native-worker review input |
| DOC-0123 | [`transport-matrix.md`](interoperability/transport-matrix.md) | review | current | ADR-010 transport matrix |

## `docs/release-notes/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0246 | [`README.md`](release-notes/README.md) | index | current | Release notes folder: text that goes into a GitHub release. |
| DOC-0126 | [`platform-notice.md`](release-notes/platform-notice.md) | release | current | Windows unverified-preview notice placed first in every GitHub release's notes. |

## `docs/releases/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0247 | [`README.md`](releases/README.md) | index | current | Releases folder: evidence for each published release. |
| DOC-0128 | [`v1.1.0-alpha.2.md`](releases/v1.1.0-alpha.2.md) | release | historical | v1.1.0-alpha.2 release evidence |

## `docs/review/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0129 | [`ADR-010-G0-windows-transport-decision.md`](review/ADR-010-G0-windows-transport-decision.md) | decision | historical | ADR-010 G0 — Maintainer decision on the Windows transport |
| DOC-0130 | [`ADR-010-G1-P04-authorization-decision.md`](review/ADR-010-G1-P04-authorization-decision.md) | decision | historical | ADR-010 G1 / P04 — Maintainer authorization |
| DOC-0131 | [`ADR-010-G1-authorization-clarification-2026-10-28.md`](review/ADR-010-G1-authorization-clarification-2026-10-28.md) | review | historical | ADR-010 G1: authorization clarification, 2026-10-28 |
| DOC-0132 | [`ADR-010-G1-open-questions-decision.md`](review/ADR-010-G1-open-questions-decision.md) | decision | historical | ADR-010 G1 — Maintainer direction on the open questions |
| DOC-0133 | [`ADR-010-v2.2-ACCEPTED-OWNER-DECISIONS.md`](review/ADR-010-v2.2-ACCEPTED-OWNER-DECISIONS.md) | decision | frozen | GoPMgr ADR-010 v2.2 — Accepted Maintainer Decision Record |
| DOC-0134 | [`ADR-010-v2.2-integration-review.md`](review/ADR-010-v2.2-integration-review.md) | review | historical | ADR-010 v2.2 integration review |
| DOC-0135 | [`ADR-010-v2.2-review-and-traceability.md`](review/ADR-010-v2.2-review-and-traceability.md) | review | historical | ADR-010 v2.2 — Critical independent review, decisions and traceability |
| DOC-0136 | [`ADR-012-DUCKDB-RETIREMENT-OWNER-DECISION.md`](review/ADR-012-DUCKDB-RETIREMENT-OWNER-DECISION.md) | decision | historical | ADR-012: DuckDB retirement and provider allocation |
| DOC-0137 | [`ADR-012-macos-containment-ACR.md`](review/ADR-012-macos-containment-ACR.md) | review | historical | ADR-012 ACR: macOS containment implementation gap |
| DOC-0138 | [`ADR-012-v0.2-ACCEPTED-OWNER-DECISIONS.md`](review/ADR-012-v0.2-ACCEPTED-OWNER-DECISIONS.md) | decision | historical | ADR-012 v0.2: maintainer acceptance and DF0 authorization |
| DOC-0139 | [`CRITICAL-REVIEW-AND-DECISION-DELTA.md`](review/CRITICAL-REVIEW-AND-DECISION-DELTA.md) | review | historical | Critical review and formal decision delta — tiered hardware authentication v4 |
| DOC-0140 | [`CRITICAL-REVIEW-and-DECISIONS.md`](review/CRITICAL-REVIEW-and-DECISIONS.md) | review | historical | Critical review and v2 decisions — 2026-10-28 |
| DOC-0141 | [`LINUX-MINIMUM-UBUNTU-26.04-OWNER-DECISION.md`](review/LINUX-MINIMUM-UBUNTU-26.04-OWNER-DECISION.md) | decision | historical | Linux minimum target: Ubuntu 26.04 |
| DOC-0142 | [`PS-04-owner-decisions-2026-10-27.md`](review/PS-04-owner-decisions-2026-10-27.md) | decision | historical | PS-04 owner decisions, 2026-10-27 |
| DOC-0143 | [`PS-capability-matrix.md`](review/PS-capability-matrix.md) | review | historical | Professional scheduling: requirements-to-capabilities matrix |
| DOC-0144 | [`PS-professional-scheduling-ACCEPTED-OWNER-DECISION.md`](review/PS-professional-scheduling-ACCEPTED-OWNER-DECISION.md) | decision | historical | Professional scheduling and project controls: maintainer decision |
| DOC-0248 | [`README.md`](review/README.md) | index | current | Review folder: owner decisions, reviews and triage, newest first. |
| DOC-0145 | [`TPC-benchmark-exclusion-decision.md`](review/TPC-benchmark-exclusion-decision.md) | decision | historical | TPC-H and TPC-DS exclusion: decision, evidence, and options |
| DOC-0146 | [`adr-012-repository-verification.md`](review/adr-012-repository-verification.md) | review | historical | ADR-012 v0.2: repository and acceptance-contract verification |
| DOC-0147 | [`adr-012-v0.2-improvements-and-traceability.md`](review/adr-012-v0.2-improvements-and-traceability.md) | review | accepted | ADR-012 v0.2 — Critical Review, Amendment Traceability and Approval Gates |
| DOC-0148 | [`architecture-directive-2026-10-27.md`](review/architecture-directive-2026-10-27.md) | decision | historical | Maintainer architecture directive, 2026-10-27: addendum and baseline |
| DOC-0020 | [`cornucopia-elicitation-2026-09-01.md`](review/cornucopia-elicitation-2026-09-01.md) | review | historical | OWASP Cornucopia security review |
| DOC-0149 | [`evaluation-2026-11-10-response-plan.md`](review/evaluation-2026-11-10-response-plan.md) | review | historical | Response plan for the 2026-11-10 independent evaluation |
| DOC-0150 | [`independent-evaluation-2026-11-10.md`](review/independent-evaluation-2026-11-10.md) | review | historical | GoPMgr: Consolidated Architecture, Capability, and Outstanding-Issues Evaluation |
| DOC-0151 | [`owner-decisions-2026-11-14.md`](review/owner-decisions-2026-11-14.md) | decision | historical | Owner decisions, 2026-11-14 |
| DOC-0152 | [`reporting-extension-integration.md`](review/reporting-extension-integration.md) | review | historical | Reporting and extension plan integration review |
| DOC-0153 | [`sec-04-triage.md`](review/sec-04-triage.md) | review | historical | SEC-04 triage: low-severity scan items |

## `docs/testing/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0154 | [`PS-02-schedule-quality-audit-spec.md`](testing/PS-02-schedule-quality-audit-spec.md) | strategy | current | PS-02 schedule-quality audit: specification and slice plan |
| DOC-0249 | [`README.md`](testing/README.md) | index | current | Testing folder: test strategies, evidence records, qualification records and the DF0 spike. |
| DOC-0155 | [`account-identity-registry.md`](testing/account-identity-registry.md) | evidence | historical | Account identity registry: first implementation slice |
| DOC-0156 | [`account-race-assurance.md`](testing/account-race-assurance.md) | strategy | current | Full account race verification |
| DOC-0157 | [`account-race-oracle.md`](testing/account-race-oracle.md) | evidence | historical | Account creation write-lock oracle |
| DOC-0158 | [`account-transaction-cleanup.md`](testing/account-transaction-cleanup.md) | evidence | historical | Account transaction cleanup |
| DOC-0159 | [`adr-010-case-crosswalk.csv`](testing/adr-010-case-crosswalk.csv) | evidence-data | current | ADR-010 test case crosswalk between plan versions. |
| DOC-0160 | [`adr-010-g0-evidence.md`](testing/adr-010-g0-evidence.md) | evidence | historical | ADR-010 P01/G0 — Go↔Java gRPC local-IPC evidence |
| DOC-0161 | [`adr-010-g0-windows-transport-report.md`](testing/adr-010-g0-windows-transport-report.md) | evidence | historical | ADR-010 G0 — Windows transport comparative report |
| DOC-0162 | [`adr-010-g1-blocked.md`](testing/adr-010-g1-blocked.md) | evidence | historical | ADR-010 G1 — BLOCKED tests and resource plan |
| DOC-0163 | [`adr-010-s0-baseline.md`](testing/adr-010-s0-baseline.md) | evidence | historical | ADR-010 P02/S0 — Source, storage and performance baseline |
| DOC-0164 | [`adr-010-test-strategy.md`](testing/adr-010-test-strategy.md) | strategy | current | ADR-010 v2.2 — Test strategy, quality gates and traceable acceptance matrix |
| DOC-0165 | [`adr-012-datafusion-feasibility-spike.md`](testing/adr-012-datafusion-feasibility-spike.md) | plan | accepted | ADR-012 / DF0: DataFusion-for-DuckDB replacement feasibility spike — v0.2 |
| DOC-0166 | [`adr-012-df0-admission.md`](testing/adr-012-df0-admission.md) | evidence | historical | DF0: pre-launch reservation admission |
| DOC-0167 | [`adr-012-df0-c2.md`](testing/adr-012-df0-c2.md) | evidence | historical | DF0-C2: host-mediated Parquet reads |
| DOC-0168 | [`adr-012-df0-evidence.md`](testing/adr-012-df0-evidence.md) | evidence | historical | ADR-012 DF0 evidence |
| DOC-0169 | [`adr-012-df0-g-report.md`](testing/adr-012-df0-g-report.md) | evidence | historical | ADR-012 DF0-G: go/no-go report |
| DOC-0170 | [`adr-012-df0-macos-limits.md`](testing/adr-012-df0-macos-limits.md) | evidence | historical | DF0: native macOS limit primitives |
| DOC-0171 | [`adr-012-df0-memory-only.md`](testing/adr-012-df0-memory-only.md) | evidence | historical | DF0: memory-only filesystem policy for the sandboxed macOS worker |
| DOC-0172 | [`adr-012-df0-nested-frames.md`](testing/adr-012-df0-nested-frames.md) | evidence | historical | DF0: nested view frame preparation |
| DOC-0173 | [`adr-012-df0-no-spill.md`](testing/adr-012-df0-no-spill.md) | evidence | historical | DF0-D2: native macOS no-spill experiment |
| DOC-0174 | [`adr-012-df0-numeric-fidelity.md`](testing/adr-012-df0-numeric-fidelity.md) | evidence | historical | DF0-E2: typed tabular numeric fidelity on macOS |
| DOC-0175 | [`adr-012-df0-observer-sandbox.md`](testing/adr-012-df0-observer-sandbox.md) | evidence | historical | DF0: native macOS observation and App Sandbox feasibility |
| DOC-0176 | [`adr-012-df0-portfolio-performance.md`](testing/adr-012-df0-portfolio-performance.md) | evidence | historical | DF0-E: partial macOS portfolio backend characterization |
| DOC-0194 | [`adr-012-df0-resources.md`](testing/adr-012-df0-resources.md) | evidence | historical | DF0: host-observed child resources on macOS |
| DOC-0195 | [`adr-012-df0-zstd-assessment.md`](testing/adr-012-df0-zstd-assessment.md) | evidence | historical | DF0 Zstd security assessment |
| DOC-0196 | [`adr-012-rust-1.99.1.md`](testing/adr-012-rust-1.99.1.md) | evidence | historical | ADR-012: Rust 1.99.1 qualification |
| DOC-0197 | [`app-admission-epochs.md`](testing/app-admission-epochs.md) | evidence | historical | App admission epochs and the session-held identity |
| DOC-0198 | [`credential-coherence.md`](testing/credential-coherence.md) | evidence | historical | Sign-in bound to one account identity: credential slice |
| DOC-0199 | [`duckdb-notices-qualification.md`](testing/duckdb-notices-qualification.md) | evidence | historical | DuckDB native notices qualification |
| DOC-0200 | [`duckdb-retirement-parity.md`](testing/duckdb-retirement-parity.md) | evidence | historical | DuckDB retirement parity record |
| DOC-0251 | [`editor-guard-errors.md`](testing/editor-guard-errors.md) | evidence | historical | Editor errors in the exit guard: One exit-guard error contract, proven for three editors. |
| DOC-0250 | [`editor-save-atomicity.md`](testing/editor-save-atomicity.md) | evidence | historical | Editor save atomicity: one write-and-readback transaction contract, proven for four editors. |
| DOC-0252 | [`editor-timestamp-boundary.md`](testing/editor-timestamp-boundary.md) | evidence | historical | Editor timestamp boundary: One timestamp-boundary contract, proven for two editors. |
| DOC-0201 | [`go-1.27.3-security.md`](testing/go-1.27.3-security.md) | evidence | historical | Go 1.27.3 security baseline |
| DOC-0202 | [`id-generation-128-bit.md`](testing/id-generation-128-bit.md) | evidence | historical | Identifier generation: bounded Slice C repair |
| DOC-0203 | [`macos-installer-policy.md`](testing/macos-installer-policy.md) | strategy | current | Local macOS PKG policy |
| DOC-0204 | [`macos-package-lifecycle-2026-10-17.md`](testing/macos-package-lifecycle-2026-10-17.md) | evidence | historical | macOS package lifecycle evidence, 2026-10-17 |
| DOC-0205 | [`native-close-macos-2026-10-12.md`](testing/native-close-macos-2026-10-12.md) | evidence | historical | Native editor Cmd+Q evidence, 2026-10-12 |
| DOC-0206 | [`native-editor-discard-failure-2026-10-16.md`](testing/native-editor-discard-failure-2026-10-16.md) | evidence | historical | Native editor Discard and failed-save recovery, 2026-10-16 |
| DOC-0207 | [`native-new-record-quit-2026-10-18.md`](testing/native-new-record-quit-2026-10-18.md) | evidence | historical | Native new-record quit evidence, 2026-10-18 |
| DOC-0208 | [`native-new-record-retry-2026-10-18.md`](testing/native-new-record-retry-2026-10-18.md) | evidence | historical | Native new-record failed-save/retry evidence, 2026-10-18 |
| DOC-0209 | [`native-sprint-recovery-2026-10-18.md`](testing/native-sprint-recovery-2026-10-18.md) | evidence | historical | Native new-sprint quit recovery, 2026-10-18 |
| DOC-0210 | [`native-sprint-update-2026-10-19.md`](testing/native-sprint-update-2026-10-19.md) | evidence | historical | Saved-sprint native update recovery, 2026-10-19 |
| DOC-0211 | [`ps04-own10-qualification-plan.md`](testing/ps04-own10-qualification-plan.md) | strategy | current | PS-04 performance qualification plan (OWN-10) |
| DOC-0212 | [`read-only-project-access.md`](testing/read-only-project-access.md) | evidence | historical | Read-only project access: implementation and evidence |
| DOC-0213 | [`recent-work-test-plan-2026-10-23.md`](testing/recent-work-test-plan-2026-10-23.md) | strategy | historical | Test plan for PRs #125 to #131 |
| DOC-0214 | [`release-race-package-policy.md`](testing/release-race-package-policy.md) | strategy | current | Explicit race-test package budget |
| DOC-0215 | [`reliability-2026-10-12.md`](testing/reliability-2026-10-12.md) | evidence | historical | Reliability repairs, 2026-10-12 |
| DOC-0216 | [`reporting-extension-assurance.md`](testing/reporting-extension-assurance.md) | strategy | current | Reporting and extension assurance strategy |
| DOC-0218 | [`resource-overallocation-contract.md`](testing/resource-overallocation-contract.md) | strategy | current | Resource overallocation: legacy contract and test oracle |
| DOC-0219 | [`resource-resilience-assurance.md`](testing/resource-resilience-assurance.md) | strategy | current | Resource resilience acceptance contract |
| DOC-0222 | [`sprint-save-feedback.md`](testing/sprint-save-feedback.md) | evidence | historical | Sprint save feedback, 2026-10-19 |
| DOC-0224 | [`sqlcipher-4.20-upgrade.md`](testing/sqlcipher-4.20-upgrade.md) | evidence | historical | SQLCipher 4.20.0 upgrade evidence (2026-11-10) |
| DOC-0253 | [`sqlcipher-cleanup-corrections.md`](testing/sqlcipher-cleanup-corrections.md) | evidence | historical | SQLCipher cleanup corrections: Three corrections to SQLCipher connection cleanup after failed transactions. |
| DOC-0230 | [`tabular-import-paths.md`](testing/tabular-import-paths.md) | evidence | historical | Tabular import: selected-path verification |
| DOC-0231 | [`wails-v3-migration-evidence.md`](testing/wails-v3-migration-evidence.md) | evidence | historical | Wails v3 / GTK 4 migration evidence |
| DOC-0254 | [`wbs-foundation-evidence-2026-10.md`](testing/wbs-foundation-evidence-2026-10.md) | evidence | historical | WBS foundation evidence, October 2026: model validation, write ordering and the storage boundary. |

## `docs/testing/adr-012-df0-records/`

| ID | File | Kind | Status | Summary |
| --- | --- | --- | --- | --- |
| DOC-0177 | [`df0-a/**`](testing/adr-012-df0-records/df0-a/) | evidence-data | historical | DF0 spike record data: df0-a (archived spike, 2026-11). |
| DOC-0178 | [`df0-admission-macos/**`](testing/adr-012-df0-records/df0-admission-macos/) | evidence-data | historical | DF0 spike record data: df0-admission-macos (archived spike, 2026-11). |
| DOC-0179 | [`df0-b/**`](testing/adr-012-df0-records/df0-b/) | evidence-data | historical | DF0 spike record data: df0-b (archived spike, 2026-11). |
| DOC-0180 | [`df0-c/**`](testing/adr-012-df0-records/df0-c/) | evidence-data | historical | DF0 spike record data: df0-c (archived spike, 2026-11). |
| DOC-0181 | [`df0-c2-macos/**`](testing/adr-012-df0-records/df0-c2-macos/) | evidence-data | historical | DF0 spike record data: df0-c2-macos (archived spike, 2026-11). |
| DOC-0182 | [`df0-d2-macos/**`](testing/adr-012-df0-records/df0-d2-macos/) | evidence-data | historical | DF0 spike record data: df0-d2-macos (archived spike, 2026-11). |
| DOC-0183 | [`df0-exec-limits-macos/**`](testing/adr-012-df0-records/df0-exec-limits-macos/) | evidence-data | historical | DF0 spike record data: df0-exec-limits-macos (archived spike, 2026-11). |
| DOC-0184 | [`df0-f-macos/**`](testing/adr-012-df0-records/df0-f-macos/) | evidence-data | historical | DF0 spike record data: df0-f-macos (archived spike, 2026-11). |
| DOC-0185 | [`df0-fidelity-macos/**`](testing/adr-012-df0-records/df0-fidelity-macos/) | evidence-data | historical | DF0 spike record data: df0-fidelity-macos (archived spike, 2026-11). |
| DOC-0186 | [`df0-limits-macos/**`](testing/adr-012-df0-records/df0-limits-macos/) | evidence-data | historical | DF0 spike record data: df0-limits-macos (archived spike, 2026-11). |
| DOC-0187 | [`df0-memory-only-macos/**`](testing/adr-012-df0-records/df0-memory-only-macos/) | evidence-data | historical | DF0 spike record data: df0-memory-only-macos (archived spike, 2026-11). |
| DOC-0188 | [`df0-native-source-macos/**`](testing/adr-012-df0-records/df0-native-source-macos/) | evidence-data | historical | DF0 spike record data: df0-native-source-macos (archived spike, 2026-11). |
| DOC-0189 | [`df0-nested-frames-macos/**`](testing/adr-012-df0-records/df0-nested-frames-macos/) | evidence-data | historical | DF0 spike record data: df0-nested-frames-macos (archived spike, 2026-11). |
| DOC-0190 | [`df0-observer-sandbox-macos/**`](testing/adr-012-df0-records/df0-observer-sandbox-macos/) | evidence-data | historical | DF0 spike record data: df0-observer-sandbox-macos (archived spike, 2026-11). |
| DOC-0191 | [`df0-performance-macos/**`](testing/adr-012-df0-records/df0-performance-macos/) | evidence-data | historical | DF0 spike record data: df0-performance-macos (archived spike, 2026-11). |
| DOC-0192 | [`df0-resources-macos/**`](testing/adr-012-df0-records/df0-resources-macos/) | evidence-data | historical | DF0 spike record data: df0-resources-macos (archived spike, 2026-11). |
| DOC-0193 | [`df0-sbom-macos/**`](testing/adr-012-df0-records/df0-sbom-macos/) | evidence-data | historical | DF0 spike record data: df0-sbom-macos (archived spike, 2026-11). |

## Removed documents

| ID | Former path | Content now in | Evidence |
| --- | --- | --- | --- |
| DOC-0074 | `docs/design/portfolio-fx-governance-options.md` | DOC-0073 | PR 229 |
| DOC-0217 | `docs/testing/resource-calendar-save-atomicity.md` | DOC-0250 | PR 229 |
| DOC-0220 | `docs/testing/sprint-guard-errors.md` | DOC-0251 | PR 229 |
| DOC-0221 | `docs/testing/sprint-save-atomicity.md` | DOC-0250 | PR 229 |
| DOC-0223 | `docs/testing/sprint-timestamp-boundary.md` | DOC-0252 | PR 229 |
| DOC-0225 | `docs/testing/sqlcipher-commit-cleanup.md` | DOC-0253 | PR 229 |
| DOC-0226 | `docs/testing/sqlcipher-rollback-cleanup.md` | DOC-0253 | PR 229 |
| DOC-0227 | `docs/testing/sqlcipher-wal-reset.md` | DOC-0253 | PR 229 |
| DOC-0228 | `docs/testing/stakeholder-guard-errors.md` | DOC-0251 | PR 229 |
| DOC-0229 | `docs/testing/stakeholder-save-atomicity.md` | DOC-0250 | PR 229 |
| DOC-0232 | `docs/testing/wbs-model-validation-2026-10-13.md` | DOC-0254 | PR 229 |
| DOC-0233 | `docs/testing/wbs-ordering-2026-10-13.md` | DOC-0254 | PR 229 |
| DOC-0234 | `docs/testing/wbs-storage-boundary-2026-10-14.md` | DOC-0254 | PR 229 |
| DOC-0235 | `docs/testing/work-item-guard-errors.md` | DOC-0251 | PR 229 |
| DOC-0236 | `docs/testing/work-item-save-atomicity.md` | DOC-0250 | PR 229 |
| DOC-0237 | `docs/testing/work-item-timestamp-boundary.md` | DOC-0252 | PR 229 |

## Planned documents

| Path | Promised by |
| --- | --- |
| `docs/interoperability/mpxj-dependency-inventory.md` | `docs/design/adr-010-implementation-plan.md` |
| `docs/interoperability/mpxj-field-mapping.md` | `docs/design/adr-010-implementation-plan.md` |
| `docs/interoperability/mpxj-format-support.md` | `docs/design/adr-010-implementation-plan.md` |
| `docs/design/provider-integrations-v2-plan.md` | `docs/design/docs-reorganization-plan.md` |
| `docs/design/security-keys-v4-design.md` | `docs/design/docs-reorganization-plan.md` |
| `docs/review/provider-integrations-v2-critical-review.md` | `docs/design/docs-reorganization-plan.md` |
| `docs/review/security-keys-v4-critical-review.md` | `docs/design/docs-reorganization-plan.md` |
