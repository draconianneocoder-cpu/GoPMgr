<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010 reference documents

This folder keeps every earlier ADR-010 plan unchanged. These files are
reference material, not the current proposal.

**The current proposal is v2.2. Read it in this order:**

1. [ADR-010](../ADR-010-plugin-architecture.md): quality invariants QI-01 to
   QI-15 and decisions D-01 to D-19.
2. [v2.2 review and traceability](../../review/ADR-010-v2.2-review-and-traceability.md):
   why v2.2 changes v2.1.
3. [Implementation plan](../adr-010-implementation-plan.md): contracts,
   phases, PR slices P00 to P18+, interface components and copy.
4. [Test strategy](../../testing/adr-010-test-strategy.md) and its
   [case crosswalk](../../testing/adr-010-case-crosswalk.csv).
5. [Agent prompt](../../agent/ADR-010-architecture-preservation-agent-prompt.md):
   rules for any coding agent working on ADR-010.
6. [Integration review](../../review/ADR-010-v2.2-integration-review.md):
   what was verified and corrected when v2.2 entered the repository, and the
   baseline conformance report.

All of them are PROPOSED until the maintainer accepts them.

## Files in this folder

| File | What it is | Status |
| --- | --- | --- |
| [`v1-original-plan.md`](v1-original-plan.md) | The first ADR-010 plan | Reference |
| [`v2.0-plan.md`](v2.0-plan.md) | v2.0: workers, MPXJ, normalized storage, scale | Reference; defines PLG and SCALE tests |
| [`v2-critical-review-addendum.md`](v2-critical-review-addendum.md) | Critical review of v2.0 | Reference; defines REVIEW tests |
| [`v2.1/`](v2.1/ADR-010-plugin-architecture.md) | The agent's v2.1 ADR, plan and test strategy (`49ac2bf`) | Reference; defines ADR10 tests. Superseded by v2.2 only if the maintainer accepts v2.2 |

**Rules for these files:**

- **Earlier IDs stay active.** The PLG, SCALE, REVIEW and ADR10 test IDs
  defined in these files remain active through the crosswalk.
- **Never edited.** Defects in these files are recorded in the newer
  documents, not fixed here.
- **Old links.** Links inside the `v2.1/` copies still point at their
  original locations.
- **Whitespace exemption.** `.gitattributes` turns off whitespace checks for
  the first three files, because they use trailing double spaces as line
  breaks.

## Provenance

**Received.** The v2.2 bundle was received on 2026-10-24, and every file
matched its `SHA256SUMS.txt`. Commit `33e6efe` adds its five documents exactly
as received, and the next commit holds every integration edit.

**SHA-256 of each file as received:**

| File | SHA-256 |
| --- | --- |
| `docs/design/ADR-010-plugin-architecture.md` | `1bb214ec6510911dc6e84cb8ebd10b5ca18de60ed9b796052fd6feb3e3963271` |
| `docs/design/adr-010-implementation-plan.md` | `32d5d8209cdc3b039ba1be753c1085affa8f5281c3d7259dc6877ce98104d223` |
| `docs/testing/adr-010-test-strategy.md` | `9e2119122759832e9d0bef391199fae3f8b301a98762e7a269f0d82d79a2df20` |
| `docs/review/ADR-010-v2.2-review-and-traceability.md` | `cbceb59ea63d5d0e08ceaa2aa1fd93e482bfb7960918744c969e69fd9b8cf138` |
| `docs/agent/ADR-010-architecture-preservation-agent-prompt.md` | `170819461696a2e99f4656ee96f14c29e750ca5e09025ed5ddd51ba451665564` |

**Reference copies.** The bundle's three reference files are byte-identical
to `v1-original-plan.md`, `v2.0-plan.md` and `v2-critical-review-addendum.md`
here.

**Not copied.** The bundle's own README was left out, because its links
assume the bundle's layout. Its reading order and integration steps are
above, and in review §8.
