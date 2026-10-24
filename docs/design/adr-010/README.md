<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010 source documents

This folder keeps the three planning documents that ADR-010 was built from,
copied here unchanged on 2026-10-24. They are reference material, not the
current decision.

**To find out what GoPMgr plans to do, read these instead:**

1. [ADR-010](../ADR-010-plugin-architecture.md): the decisions (v2.1).
2. [Implementation plan](../adr-010-implementation-plan.md): phases,
   contracts, and interface design.
3. [Test strategy](../../testing/adr-010-test-strategy.md): what each phase
   must prove.

## Files

| File | What it is | How v2.1 uses it |
| --- | --- | --- |
| [`v2.0-plan.md`](v2.0-plan.md) | ADR-010 v2.0: workers, MPXJ, normalized storage, scale, and interoperability | Primary source |
| [`v2-critical-review-addendum.md`](v2-critical-review-addendum.md) | Critical review of v2.0: 8 P0 findings, P1 performance items, interface items, amendments A0-A9, tests REVIEW-001 to REVIEW-020 | Applied in full; ADR-010 lists where each item went |
| [`v1-original-plan.md`](v1-original-plan.md) | The first ADR-010 plan | Kept for reference only |

**Reading order for the sources:** v2.0, then the addendum, then the original.

## Status

- **Not current.** These files describe plans as they stood before the
  review. Where they disagree with ADR-010 v2.1, v2.1 wins. ADR-010 lists
  each correction under "Changes from v2.0".
- **Not implemented.** No source code, schema, or migration follows from them.
- **Not edited.** Defects in them, such as duplicated numbering, are recorded
  in ADR-010 and fixed only in the new documents.
- **Exact copies.** `.gitattributes` turns off whitespace checks for these
  three files, because they use trailing double spaces as Markdown line
  breaks.

## References

Section references such as [v2 §9.2] or [P0-02] in the plan and the test
strategy point into these files.
