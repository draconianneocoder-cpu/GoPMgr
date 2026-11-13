<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Reporting and extension assurance strategy

**Status:** Proposed verification contracts, integrated 2026-11-13. No report,
extension runtime, containment mechanism or installed release is qualified here.
Use the [execution plan](../design/reporting-extension/implementation-plan.md) and
[requirement register](../design/reporting-extension/requirements.json).

## Qualification and evidence

RILD Q1 domain/provenance; Q2 math/semantics; Q3 snapshot/history; Q4 presentation
and format parity; Q5 security/recovery; Q6 usability/accessibility; Q7 scale and
platforms; Q8 independent sign-off all require an explicit applicability decision.
Record NOT RUN, PASS, FAIL or BLOCKED separately. N/A needs a reason and reviewer.
A list of test IDs is a specification, not proof of executed tests.

The complete source proof contracts remain mandatory target requirements:

| Gate | Proof required to mark qualified |
| --- | --- |
| Q1 Domain / provenance | Schema and relationships, authoritative owner, coverage reconciliation and source lineage; unsupported records explicit. |
| Q2 Math / semantics | Independent fixtures, boundary and property tests, reference calculations, calendar/constraint semantics and metric definitions. |
| Q3 Snapshot / history | Concurrency, revision capture, restatement, exact retrieval, audit/approval integrity. |
| Q4 Presentation / parity | Extract values from actual exports; visual page inspection, bookmarks/links, accessible reading order and data table alternatives. |
| Q5 Security / recovery | Authorization, hostile inputs, export injection, partial write/cancel/crash recovery, backup/restore. |
| Q6 Usability / accessibility | Representative user tasks on installed apps; keyboard, screen-reader, zoom and error recovery. Record limitations. |
| Q7 Scale / platforms | Approved hardware, representative 1k/10k/50k activities, 100k stress; no loss/truncation; native macOS 26 and Ubuntu 26.04; Windows only after approved qualification. |
| Q8 Independent sign-off | Reviewer not author of implementation/expected oracle; exact commit, build, dataset/hash, command, environment, observed results and residual findings. |


RILD evidence E0 gap, E1 documented design, E2 reviewed source, E3 retained executed
independent test, E4 applicable accepted end-to-end/platform/usability evidence.
These levels are unrelated to EXT execution profiles E0–E3. Record exact commit,
build/tool identities, inputs/hashes, environment, commands, observed result and
scope of every claim. Independent sign-off reviewers must be authors of neither the implementation nor
the expected oracle. Oracles must not merely reuse implementation-derived values. Do not average or count evidence states as an assurance score.

## Metric contract and report truth

Before implementation, each metric definition names stable ID/version, decision,
source owner and schema, grain and inclusion/eligibility rules, effective cutoff
and recorded knowledge time, baseline, units/currency, formula, numerator and
denominator membership, corrections, precision/rounding, applicability, coverage,
provenance and output-specific limitations. No metric calculation belongs in a
renderer. Calculated, imported, management-assessed and forecast values remain
separately attributable. Filters must not infer unauthorized data.

Use the canonical value states **Known**, **Unknown**, **NotApplicable**,
**Unsupported**, **Invalid** and **Suppressed** (ADR-014 ARCH14). Suppression records
permission/policy withholding without disclosing protected values. Freshness and
coverage are separate dimensions: stale/partial do not replace a value state.
Zero is a known value, never a missing-data substitute.
A required unavailable metric blocks a complete/approved issue under its policy;
an explicitly partial draft shows coverage and exclusions. Empty denominators
produce unavailable ratios, not 0 or 1. Never invent historic observations, rates,
financial recognition rules, RAG thresholds or certainty about source facts.

| Metric family | Controlling definition and pitfalls | Independent oracle |
| --- | --- | --- |
| Schedule/progress | Working-calendar finish/slip and explicit baseline; progress basis physical, duration or effort is named. KRN-01/KRN-02 limit affected claims. Do not double-count summary activities. | T-S01–04; independently enumerate working dates, constraints, lag, holidays, timezone/DST and remaining work. |
| EVM | SV=EV−PV, CV=EV−AC; SPI=EV/PV, CPI=EV/AC; EAC method/version is named, not always BAC/CPI. Financial basis and ledger reconciliation are explicit. | T-F01; boundary zero/negative/correction cases; portfolio ratios use sums of applicable numerators and denominators. |
| Investment/commercial | Cash-flow timing, discount/recognition policy, capital/operating basis, FX/date and rates are versioned; profit/revenue/invoice/cash are distinct. | T-F02–04; exact rational/decimal calculation and approved presentation rounding. Fractional JPY remains compatible under ADR-009. |
| Resource/time | Sum assignment demand over shared identity once; calendar/leave and approved/pending/corrected time have distinct states. | T-R01–02; zero capacity with positive demand is overload, not a valid utilization ratio. |
| Agile/quality | Versioned state transitions, scope and acceptance events; sprint commitment versus current scope; no fabricated pre-event history or individual productivity ranking. | T-A01–02; replay with scope changes/reopening; sample/censoring and method recorded for flow, DORA and quality statistics. |
| Portfolio/program | Scope/denominator coverage, independent project revision vector and authorization. Project schedule owners retain write authority. | T-P01 and three-project interface fixture; no mean of ratios, mixed-currency total or fictitious simultaneous global snapshot. |
| Monitoring/control | Metric/threshold version and observation → action/change → implementation → effectiveness evidence; distinguish potential risk and realized issue. | MC-01–14 and original row oracles. Closing an action does not prove effectiveness; no automated baseline mutation. |

Recommended reporting quality KPIs for qualification are (1) semantic agreement with
independent oracles for the claimed outputs, (2) complete source/coverage lineage
for issued sections, and (3) successful exact issued-artifact retrieval after
recovery. Drivers include invalid/missing inputs by reason and time/resource use
by approved workload profile. Guardrails are zero unauthorized disclosure or
mutation, and zero silently omitted required records. These are future engineering
acceptance measures, not observed product statistics. Defined fixtures and policy
must set their populations and pass criteria. Latency targets and operational
alert rates remain unset until measured baselines and DEC-HARDWARE approval; do
not copy provisional EXT budgets into product SLOs.

## Independent fixture catalog

The following source expectations are retained as specifications. Independently
review inputs, expected values and applicability before implementing or executing.

| Fixture and inputs | Expected result and adversarial cases |
| --- | --- |
| T-S01 Network: A=3d; B=2d FS A; C=4d FS A; D=1d FS B,C; no calendar exceptions. | Finish offset 8 working days; critical A-C-D; B float 2d. Cycle D→A rejected; unsupported mandatory constraint blocked. Verify actual dates on a Monday–Friday calendar separately. |
| T-S02 Baseline B finish day 8; current finish day 10. | Finish slip +2 working days; original baseline unchanged; report explicitly names baseline. Rebaseline creates new identity. |
| T-S03 Calendar: 8h/day; Monday holiday; 16h starts Tuesday. | Work ends Wednesday at end of second workday. Test resource overrides, negative lag, DST and actual/remaining rules with independently enumerated timeline. |
| T-S04 Import 10,001 rows; one unknown constraint. | Counts reconcile; no silently dropped record; unsupported semantics flagged; complete schedule claim blocked. 50k/100k qualification separately. |
| T-F01 BAC 1m; PV 500k; EV 400k; AC 450k. | SV −100k; CV −50k; SPI .8; CPI 8/9; EAC 1.125m under BAC/CPI; VAC −125k. Zero AC/PV yields unavailable, not 0/1. |
| T-F02 Cash flows −1000 now, +600 at year 1 and 2; r=10%. | NPV = 600/1.1 + 600/1.21 −1000 ≈41.32; BCR ≈1.0413; undiscounted payback 1.667 years only if linear within year; display assumption. |
| T-F03 Base 100, contingency 10, management 5, commitment 40, actual 20. | Baseline 110; authorized funding 115 under stated policy; no summing commitment+actual without overlap rule. Exact minor units; reserve change revision. |
| T-F04 Revenue 100, cost 60, invoiced 80, collected 50; all same currency. | Profit 40, margin 40%; receivable 30 (no credits); unbilled eligibility calculated from approved billable records, not revenue minus cash. |
| T-R01 One person capacity 40h; A demand 28h; B demand 20h; leave already included in capacity. | Total 48h; 120% utilization; overload 8h. Do not count 40h capacity twice. Zero capacity with demand is explicit overload, no ratio. |
| T-R02 Approved 6h + pending 2h; approved correction −1h; plan 8h. | Approved 5h, pending 2h, planned–approved variance 3h; no silent pending-to-zero conversion. History retained. |
| T-A01 Sprint starts with 10 points; add 3, complete 4, reopen 1. | Remaining 10, 13, 9, 10 at successive cutoffs; accepted completion 3 at close if reopened 1 of completed 4. Original commitment remains 10. |
| T-A02 5 items: day 1 states 3 todo, 2 doing; day 2 one todo→doing, one doing→done. | Day 2 counts 2 todo, 2 doing, 1 done; total 5. Reconcile scope additions/removals and reopening. |
| T-P01 P1 EV 80 PV 100 AC 100; P2 EV 20 PV 10 AC 10; P3 non-EVM. | Portfolio SPI=CPI=100/110≈.9091; exclude P3 from EVM and show coverage. Mixed currency no consolidated total without authorized policy. |
| T-D01 Approved revision A, edit draft B, approve B, archive A. | A and B retained; A hash/approval unchanged; archive actor/reason required; report can retrieve either. Reject unauthorized approval and deletion. |
| T-X01 Concurrent edit between preflight and compilation; crash after PDF staged. | Compile one frozen revision only; no mismatched findings. No issued state until complete package verified; recovery finds incomplete staging. |
| T-X02 Export restricted data; malicious CSV/HTML value; long Unicode table. | Unauthorized fields absent in all outputs/manifest; formula/HTML escaped; all rows preserved, pages readable and linked. |

## Required negative and recovery evidence

- Seed calculation-sign, row-omission, denominator, revision and authorization
  faults in disposable fixtures; each intended oracle must fail for its named
  reason. Compilation failures or unrelated errors do not detect the fault.
- Test concurrent edits between preflight and compilation, inaccessible projects,
  late corrections, missing linked records, canceled generation, disk exhaustion
  and crashes before/after every durable stage. An incomplete PDF/manifest pair
  must never be labelled an issued package. Preserve existing export compatibility
  through an explicitly approved change rather than silently deleting old output.
- Validate all actual formats against one IR: ordered IDs/counts, metric values,
  units, coverage, findings and approved narrative. Inspect rendered pages, links,
  fonts, reading order and accessible tables; a PDF build is not visual assurance.
- Exercise HTML/script and CSV/XLSX formula injection, XML/ZIP and compile bombs,
  traversal/symlink/TOCTOU races, malicious assets and audience redaction in outputs
  and manifests. No executable templates, macros or external assets by default.
- EXT execution additionally requires the original adversarial corpus in ADR-015:
  allocation-before-expansion, callback deadlock, slow consumer, resource race,
  stale/revoked handles, cross-project access, hostile package, process-tree
  cleanup and installed OS confinement. A guest memory cap is not a whole-process
  or aggregate memory guarantee. Failed cleanup retains unhealthy accounting.
- Preserve audit hashes/old event verification, approved baselines and original
  imported representations. New issuance/command receipt schemas and encryption
  key lifecycle choices require accepted format/audit/security contracts.

## Source and matrix preservation checks

Verify archived SHA-256 values from the integration review. Re-extract the original
DOCX tables and compare the **set** of explicit IDs and the four original row cells
with the register; combined source rows contribute each ID separately. Expected
counts are 120 explicit IDs, eight catalog-family IDs and 17 architecture IDs.
Verify EXT-Q01–10, EXT-D01–14 and EX0–6 in both the canonical proposal and crosswalk.
Validate local links and all referenced paths against the recorded source baseline.
These checks establish documentary completeness, not runtime qualification.

Qualification is indexed by requirement, domain contract, format, platform,
profile, workload and build. A report family or extension profile may not be
reported complete while any mandatory criterion is unrun or failed.
