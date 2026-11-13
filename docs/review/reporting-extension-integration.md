<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Reporting and extension plan integration review

**Date:** 2026-11-13. **Code baseline:** `ac20cbe711d9c03479cf937d82c5b7da991454d3`.
Scope: reconcile supplied plans with actual source, accepted decisions and the
roadmap; preserve full intended capabilities, independent assurance and agent
execution order. This records documentation integration, not production adoption.

## Inputs and authority

| Archived original | SHA-256 |
| --- | --- |
| [Reporting plan DOCX](../design/reporting-extension/sources/GoPMgr_Reporting_Integrity_Architecture_Plan.docx) | `bfbbef3b98dc13588812383379122df3b480fa2a99184b8ea9d86e7160ec39af` |
| [Extension proposal](../design/reporting-extension/sources/ADR-EXT-001-GoPMgr-Secure-Extension-Platform.md) | `6d23b028443bf77934abdb0880e2037280a4a5a5c1bd0c6a3a4e767f2c966925` |

The originals remain byte-identical. Their embedded execution instructions are
source material, not additional owner authority. The current request authorizes
this integration. The reporting source separately records approval of workstream
establishment; proposed detailed designs, reserved decisions, implementation
slices and qualification must not be conflated. Canonical identifiers ADR-014
and ADR-015 were allocated after checking the numbered design inventory, which
ended at ADR-013; source aliases RILD-01 and ADR-EXT-001 remain searchable.

## Corrections and preservation

| Source issue | Reconciled treatment and evidence |
| --- | --- |
| Older `09afe43e`/`5bd3ebe` baselines | Preserve source claims in archive/register; review current `ac20cbe`. PS-02 rules and App method exist; findings UI remains open. `docs/STATUS.md` tracks KRN-01 and KRN-02. |
| Reporting read consistency overgeneralized | Financial reporting already uses a read transaction (`internal/desktop/app_financial_report.go`); combined report preflight/resolution use separate reads and retain PDF on manifest failure (`internal/reporting/reporting.go`). Extend the good pattern; design issue atomicity separately. |
| Existing document history could be mistaken for immutable revisions | `internal/db/documents.go` updates current rows/version and records audit/approval operations; deletion removes a document. Preserve this foundation and historic audit verification; immutable approvals/issued bytes remain target work. |
| Templates and manually supplied chart values mistaken for metric qualification | Existing status fields and burndown arrays are useful authored inputs. Source-derived RAG/events require contracts and evidence, not relabelling current templates as complete reports. |
| EXT-Q09 says no registry | Clarify no **external** registry dependency. Keep the mandatory generalized **internal** capability registry from ADR-010 D-02/QI-10. |
| EXT-D11 recalculation could overwrite imported facts | Preserve observations/representations, derive host results separately and record disagreement. Future write commands/receipts/recovery are not implemented merely because App methods exist. |
| One semantic contract could become a universal API | Share semantic rules and infrastructure across separately versioned import, reporting/export, analytics and provider contracts; retain each domain state machine. |
| Reporting blocked behind executable extensions | Host-native reporting can qualify without Wasm, MPXJ, public packages or network brokers. Missing authoritative domains still gate dependent claims. Export remains a scoped disclosure/filesystem effect. |
| Ambiguous E0/E2 labels | Use **RILD evidence E0–E4** versus **EXT profile E0–E3** consistently. |
| Source resource numbers treated as requirements already approved | Keep candidate values provisional under DEC-HARDWARE/OWN-10 and applicable G1 conditions. No weaker limits, fallback, platform claim or reduction of 50k/100k objectives. |
| Apparent source baseline equals runtime platform | `internal/interop/deps_pin.go` is dependency retention; G0 transport spikes are feasibility. No production extension or confinement qualification follows. Go-native analytics and DataFusion non-adoption remain unchanged. |

## Completeness and future evidence

The versioned [reporting register](../design/reporting-extension/requirements.json)
retains 120 explicit IDs: 69 benchmark rows, 17 cross-cutting identifiers, 14 MC
cases and 20 PPM cases. The combined B1-X01/B6-X03 source row contributes both
IDs. Eight expanded families and 17 architecture contracts receive new stable
IDs without dropping their full source content. The [EXT register](../design/reporting-extension/extension-requirements.json)
retains all ten Q invariants, fourteen D decisions and seven EX gates.

Each row carries original requirement/source, authority/schema or domain links,
source-path evidence limits, target, acceptance IDs/oracles, owner, evidence,
disposition and next stage. Original P/L/G path claims are historical, not live
feature acceptance. All future qualification remains NOT RUN; applicable gates
need explicit reviewer decisions, including a reason for N/A. Expanded-family
completion requires every distinct listed capability, not one representative.

Sixteen T fixtures and eight Q gates are preserved in the assurance strategy.
Metric definitions state source grain, effective/knowledge time, coverage,
precision, denominators, unknown/unsupported states and authorization. Operational
latency/resource targets require measured evidence and owner acceptance. No KPI
percentage, benchmark result or installed support claim is invented.

## Review and verification record

Independent source reviews covered the reporting and extension plans. A separate
adversarial correction-plan review required scoped E labels, full machine-readable
row contracts and distinct direction/detail/implementation authority; the revised
plan was approved before canonical documentation edits. These are source/design
reviews, not independent runtime test executions.

Executed documentary checks on 2026-11-13:

- SHA-256 and byte equality against both supplied originals: passed.
- Re-extracted original DOCX row-cell and source-position comparison: 120 explicit
  requirements, eight expanded families and 17 linked architecture sections retained.
- Canonical/original EXT text comparison: 31 identifiers retained; reconciled text
  matches the crosswalk. All 16 fixtures and all eight full Q proof contracts retained.
- Register structure, authorization flags, unrun gate states, acceptance references,
  split snapshot/issue phases, source-path existence and 39 new local links: passed.
- `make license-check`: REUSE 3.3 compliant, 1,731/1,731 files with licensing and
  copyright information; no missing licenses or read errors.
- `git diff --check`: passed before staging; repeat on the staged documentation.
- Companion DOCX rendered to seven pages with the bundled document renderer.
  Every page was visually inspected; after the final metric-state wording change,
  page four was inspected again and the other six page images were pixel-identical.
  Template styles, relationships, numbering and all preserve-only ZIP parts remain
  byte-identical; section geometry is unchanged. Word field refresh was not run;
  update-on-open remains enabled.

Final source reviewers `/root/website_plan_review`, `/root/plan_review` and
`/root/ci_guard_advisor` approved documentary integration. Review corrections
completed full acceptance references, canonical value states including Suppressed,
Q7 workload sizes, Q8 independence from both implementation and oracle authors,
and repeated pre-disclosure/execution/result/commit qualification. EX4 applies
semantic checks to read-only results; only its mutation obligations require write
contracts. Reviewers did not independently rerun the checks or inspect DOCX layout.

Companion DOCX SHA-256:
`928efad5b0c398404cbef4605ca854e1e5b2b56532a128374b1b1db02a162ce7`.

No application build, runtime, platform, benchmark or new feature qualification was
run or claimed for this documentation-only change. No push, hosted CI, publication
or application modification is included. The first future reporting slice is
RILD-0: assign owners, approve bounded metric/acceptance contracts and resolve the
specific decisions before dependent implementation.
