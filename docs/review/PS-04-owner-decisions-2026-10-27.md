<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# PS-04 owner decisions, 2026-10-27

**Status:** decided by the maintainer, James L. Burns, on 2026-10-27.
The text was sent in a working session and is recorded verbatim below.
**Repository state when decided:** `main` at `16d5e01`.
**Context:** the PS-04 design package (documents PS04-ARCH-001 to
PS04-EXEC-006; their hashes are in
[resource-overallocation-contract.md](../testing/resource-overallocation-contract.md))
asked for these decisions. The strategic direction they refine is the
2026-10-26 professional-scheduling decision on
[PR #152](https://github.com/draconianneocoder-cpu/GoPMgr/pull/152), which is
not yet merged.

## Decision text (verbatim)

```text
GoPMgr — Owner Decision Recommendations for Implementation Planning

1. DEC-09: Prioritize RES-A-1 before PS-02 because it addresses the verified large-schedule Gantt bottleneck. Complete the isolated, compatibility-preserving optimization and its independent/differential testing before proceeding to PS-02. No unapproved RES-B redesign or scheduling-semantic changes.
2. OWN-10: Prepare qualification plans for minimum and recommended hardware profiles. Numerical performance budgets remain provisional until representative fixtures and native-platform measurements justify their approval.
3. AP-01: Use the proposed domain-ownership table from the professional scheduling domain model as the ADR-011 starting contract. Every authoritative record must have one write owner. Preserve the independence of WBS v2, financial controls, and the audit chain.
4. ADR-011: Begin drafting immediately. This authorizes architectural design and review only, not schema activation, project migration, or production database modifications.
5. PR #147: Investigate and fix the failing Verify check. Preserve existing quality gates; do not bypass, weaken, quarantine or delete tests to obtain a passing status.
6. Existing PRs: Do not merge all stacked PRs automatically. Reconcile dependencies, review each PR independently, and obtain the necessary authorization before merging.

Maintain Go scheduling authority, SQLCipher storage, legacy compatibility, deterministic verification, and the existing ADR-010/ADR-012 boundaries. Document all deviations and submit any architectural change requests before implementation.
```

## Decision ledger

IDs are the PS-04 handoff's (PS04-EXEC-006 §2). A field this text does not
settle is marked PENDING; nothing here grants more than the text says.

| ID | Decision | Conditions | Scope | Still pending |
| --- | --- | --- | --- | --- |
| DEC-09 | **APPROVED:** RES-A-1 goes before PS-02 | Finish RES-A-1 and its independent and differential testing before starting PS-02 | Ordering of the next two production slices | PS-02's own implementation approval |
| DEC-02 | **APPROVED** through DEC-09: implement RES-A-1 | Isolated and compatibility-preserving; no RES-B redesign; no scheduling-semantic change | `internal/kernel` overallocation detection only (PS04-RES-002 §4) | RES-A-2 (Gantt integration) and every RES-B decision (DEC-05) |
| DEC-06 (OWN-10) | **APPROVED:** prepare qualification plans for minimum and recommended hardware profiles | Numeric budgets stay provisional | A plan document; no budget is accepted | The budgets, and the hardware profiles themselves |
| DEC-08 (AP-01) | **APPROVED:** the proposed domain-ownership table in the professional-scheduling domain model is ADR-011's starting contract | One write owner per authoritative record; WBS v2, financial controls and the audit chain stay independent | ADR-011 drafting | Final ownership, decided through ADR-011 review |
| DEC-03 (ADR-011) | **APPROVED:** begin drafting ADR-011 | Design and review only | Architecture documents | Schema activation, project migration and any production database change |
| PR #147 | **APPROVED:** investigate and fix the failing Verify check | Do not bypass, weaken, quarantine or delete tests or gates | The Dependabot Go bump | Merging it |
| Open PRs | Do not merge the stacked PRs automatically | Reconcile dependencies and review each PR on its own | PRs #140–#146 and #149–#154 | Authorization to merge each one |

**Standing constraints restated by the maintainer:**
- Go stays the scheduling authority, and SQLCipher the storage.
- Legacy compatibility and deterministic verification are preserved.
- The ADR-010 and ADR-012 boundaries hold.
- Any deviation is documented, and any architectural change request is
  submitted before implementation.
