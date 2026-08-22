<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Cost Control Phase 2 lifecycle and authority contract

**Status:** Proposed. Implementation is blocked on the decisions in this
document. It does not ship a reserve drawdown, management-reserve release,
forecast, or authorization feature.

## Purpose and scope

Phase 1 establishes an exact-money, project-local Cost Control ledger with
planned, commitment, and actual states; two mutable reserve balances; and
immutable locally attributed baseline snapshots. It deliberately has no
remaining-spend, allocation, drawdown, or forecast metric.

The [project cost ledger scope](project-cost-ledger-scope.md) is authoritative
for ordinary budget, expense, input, material, and overhead tracking. Phase 2
does not broaden or replace that ledger merely because it later addresses
reserve movements, financial authority, or forecasts.

This document defines the implementation gate for the tracked Phase 2 work:

1. contingency-reserve allocation/drawdown movements linked to known
   risk/change evidence;
2. time-phased cash flow, burn, and EAC reporting; and
3. project-level financial authority, including the separate handling of a
   management-reserve release.

It is informed by the distinction between contingency for identified risks and
management reserve for unanticipated work described by [PMI](https://www.pmi.org/learning/library/2016/06/11/07/22/model-risk-contingency-reserve-9310).
It does not adopt another product's data model: [OpenProject's budget
documentation](https://www.openproject.org/docs/user-guide/budgets/) is useful
comparator evidence that planned, spent, and reportable cost records need clear
separation, not a compatibility target for GoPMgr.

## Current facts and limits

| Area | Current behavior | Consequence |
| --- | --- | --- |
| Ledger | `cost_entries` records planned, commitment, and actual amounts as exact signed integer minor units. The public Wails boundary uses decimal strings. | A future allocation must not silently become a commitment or actual cost. |
| Reserves | `cost_reserves` is one mutable upserted balance per project/kind, with a basis note. | It is not a movement history and cannot currently prove consumption, authority, or source evidence. |
| Baselines | `cost_baseline_snapshots` is immutable. Cost baseline is planned plus contingency; authorised funding adds management reserve. | A later movement cannot rewrite an approved snapshot. |
| Evidence | Risks/change evidence are nested document JSON, not stable relational entities. | A SQL foreign key to a nested risk ID would be invalid. |
| Authority | A baseline records the signed-in local account. `is_admin` is system-account administration. | Neither is project financial authorization. |
| Currency | No FX exists. Legacy JPY projects are read-only under their historical two-decimal convention. | Phase 2 cannot introduce currency conversion or mutate legacy JPY Cost Control data. |

## Decisions required before implementation

No drawdown implementation may start until each decision has an approved value
and the implementation issue references it.

### Gate-exit record

This document is not an approval while its status is **Proposed**. The gate
closes only when its status is changed to **Approved**, each decision row
records an accepted value rather than only a candidate, and the record names
the approving product authority and approval date or links to a durable
decision record containing them. The subsequent implementation plan must cite
that exact accepted document revision. A link to this proposed revision alone
does not satisfy the gate.

| Decision | Candidate direction | Why it is required |
| --- | --- | --- |
| Meaning of reserve balance | Define whether it is assessed, authorised-at-baseline, or remaining available. | A mutable balance is otherwise ambiguous after a baseline. |
| Contingency allocation equation | Candidate: add a planned entry and reduce remaining contingency by the same exact amount; baseline and authorised funding remain unchanged. | Prevents an allocation from being misreported as spend or additional funding. |
| Baseline binding | Require an approved baseline version and reject stale versions. | Gives a movement a stable financial context. |
| Evidence identity | Store document ID, document version, nested-item ID, and immutable evidence summary. | Preserves review context without pretending nested JSON is a relational entity. |
| Financial roles | Define project-scoped role assignments and authority limits. Do not reuse global `is_admin`. | Management-reserve release requires an actual authority boundary. |
| Correction policy | Immutable positive movement; one explicit reversal linked to its original movement, with a reason. | Preserves auditability without editing history. |
| Idempotency and time | Define operation idempotency key, project time-zone/date semantics, and retry behavior. | Prevents duplicate use and ambiguous reporting. |
| Rebaseline semantics | Define treatment of outstanding allocations in a later baseline. | Avoids double counting or disappearing reserve consumption. |

## Candidate lifecycle model, not a final decision

The candidate contingency allocation flow is intentionally narrow:

1. An authorized project actor selects an approved baseline version and a
   known-risk/change evidence reference.
2. The backend validates the project, currency, authority, evidence shape,
   positive amount, baseline version, and remaining contingency.
3. One transaction creates an immutable movement, creates the corresponding
   planned ledger entry, changes the defined remaining-contingency representation,
   and appends the movement audit event.
4. The cost baseline and authorised funding do not change. The allocation is
   neither commitment nor actual spend.
5. Correction is a linked reversal transaction, never update or delete.

Management-reserve release is excluded from that candidate. It remains blocked
until the financial-role decision is implemented and verified.

## Non-negotiable invariants

- Use checked `int64` minor-unit arithmetic and canonical decimal-string Wails
  transport. Never route money through JavaScript numbers or `float64`.
- Every record is project-scoped and uses the baseline's project currency; no
  FX conversion is introduced.
- Legacy JPY Cost Control stays read-only. No existing amount is rescaled.
- An allocation cannot exceed the defined remaining contingency and cannot
  make a reserve negative.
- Baseline snapshots and existing ledger rows never mutate.
- Movement, planned-entry effect, reserve effect, and audit event commit or
  roll back together.
- Failed, duplicated, cross-project, stale-baseline, management-reserve,
  zero/negative, overflow, and unauthorized requests have no persisted effect.
- Legacy Budget, schedule EVM, stakeholder rollups, and forecast calculations
  remain separate until their own explicit integration contracts exist.

## Persistence and migration gate

The implementation design must include an additive migration and both plaintext
and SQLCipher fixtures. The migration must preserve every existing
`cost_entries`, `cost_reserves`, baseline snapshot, and audit event unchanged.
It must be idempotent across repeated opens and fail without partially
publishing a new schema or movement.

If a movement table is introduced, it needs an immutable ID, project ID,
baseline snapshot/version reference, reserve kind, amount, evidence identity
and summary, actor, local date/time-zone information, idempotency key, reversal
link, timestamps, and canonical audit payload. A later schema design may add
fields only after documenting their ownership and compatibility impact.

## Risk-to-evidence matrix

| Risk | Minimum evidence before delivery |
| --- | --- |
| Overdraw or race | Conditional atomic update or equivalent transaction test with concurrent attempts proving total use never exceeds available reserve. |
| Partial write | Fault injection at ledger, reserve, movement, and audit writes; each failure leaves rows and audit count unchanged. |
| Duplicate submission | Repeated identical operation proves one movement/effect/audit result. |
| Authority bypass | Backend tests reject unauthorized and management-reserve actions regardless of frontend state. |
| Evidence drift | Tests preserve the captured evidence summary after the source document changes or is deleted. |
| Incorrect accounting | Tests prove allocation changes neither commitment nor actual, preserves baseline/funding, and reconciles current planned and remaining contingency values. |
| Correction abuse | Tests reject update/delete and a second reversal; one linked reversal restores the defined effect with a reason and audit chain. |
| Compatibility | Additive plaintext and SQLCipher migration fixtures plus legacy-JPY read-only tests. |
| User journey | Component tests for confirmation before mutation, duplicate-submit protection, error/retry, focus/keyboard behavior, and native Wails evidence on a disposable project. |

## Delivery order

1. Approve the decisions table and role model.
2. Produce a schema/API proposal with migration and rollback plan; obtain a
   separate adversarial review.
3. Deliver contingency allocation only, including its backend authority check,
   transaction, audit, tests, and UI confirmation.
4. Verify native Wails behavior using an isolated disposable project.
5. Deliver management-reserve release only after authority evidence exists.
6. Define time-phased forecast/EAC equations and data sources before adding a
   remaining-spend or burn metric.

## Explicit non-goals

This proposal does not implement FX, ISO currency-exponent support, general
ledger edit/delete, schedule-EVM integration, stakeholder/Budget integration,
electronic signatures, or a claim of standards compliance.
