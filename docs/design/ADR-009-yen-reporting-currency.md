<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-009: Yen as a reporting currency

**Status:** Implemented (merged in #131, 2026-10-23)
**Date:** 2026-10-23
**Deciders:** GoPMgr maintainer

## Context

GoPMgr stores every amount as a signed int64 count of hundredths of the
project's reporting currency, and it parses and formats amounts with two
decimals. Yen has no minor unit in use (its ISO 4217 exponent is 0), so a
yen amount read "1500.00".

Earlier, JPY was contained:

- Existing JPY projects stayed readable, with Cost Control read-only.
- New projects could not choose JPY.

The containment guarded one risk: no stored value may silently take on a new
meaning. `docs/beta-release-backlog.md` listed what exponent-aware JPY would
need first: per-project scale metadata, scale capture in baselines, a
migration, and agreement with Budget, earned value, rollups, analytics,
reporting, and FX.

The maintainer wants Japanese teams served (ADR-008 adds Japanese to wave 1).
The `feature/jpy-currency-support` branch tried storing yen in whole units.
It parsed and formatted Cost Control at exponent 0 and lifted the
containment, with no migration. That reads an existing value stored as
12,345 hundredths (¥123.45) as ¥12,345, a hundredfold change. Budget,
earned value, portfolio totals, and the financial report still used
hundredths. The branch also did not compile.

## Decision

1. **Storage is unchanged.** Every currency, JPY included, keeps amounts in
   hundredths of its major unit. No value is migrated or rescaled.
2. **Yen reads as whole numbers.** `money.Amount.DecimalFor` drops ".00" from
   a whole yen amount (`1500`). An amount with a fraction, which only an
   older JPY project can hold, keeps both digits (`123.45`), so it reads
   exactly as stored. Other currencies keep two decimals. Every screen and
   document that shows a project's amounts uses it:
   - Cost Control.
   - The project budget and the budget rollup.
   - Portfolio totals.
   - The financial report.
   - PDF, DOCX, and ODT documents, and combined reports.
   - The Six Sigma report's solution costs (since 2026-10-24).
3. **Input is unchanged.** Amounts parse as before, a whole number or up to
   two decimals, so whole yen parses exactly, and an older entry with a
   fraction can still be edited and saved.
4. **JPY is an ordinary reporting currency.** New projects may choose it.
   The usual lock applies: the currency changes only while the project has
   no Budget value, Cost Control entry, reserve balance, or baseline. Older
   JPY projects become editable, since their values keep their meaning.

## Options considered

| Option | Assessment |
| --- | --- |
| A. Store yen in whole units (the branch) | Matches ISO minor units, but existing values change meaning a hundredfold without a migration. Every money consumer would need the exponent. |
| B. Store a per-project exponent with a migration | Correct, but needs the schema, migration, baseline, and analytics work the backlog lists, for no visible gain over C. |
| **C. Keep hundredths; whole yen at the display boundary (chosen)** | No migration, no new metadata, one meaning for every stored value. Totals, earned value, and analytics need no change. The stored unit for yen is a hundredth of a yen, finer than needed, and the range is still about 9.2 × 10¹⁶ yen. |

## Consequences

- Japanese teams can report in yen, on screen and in documents.
- Older JPY projects keep their exact values and are no longer read-only.
- A table can mix `150000` with a legacy `123.45`; each reads exactly as
  stored.
- Input still accepts two decimals for yen, so a user may enter sen. That
  is harmless, since it is exact, and it keeps older entries editable.
- Korean won (exponent 0) could follow the same pattern. It is not offered
  until the maintainer decides to add it.
- The FX design (`governed-presentation-only-fx.md`) still excludes JPY.
  Converting to or from yen is FX work, not part of this decision.
- Schedule reports (`internal/export`) print earned value with two decimals;
  their payload carries no currency yet.

## Verification

- `internal/money`: `DecimalFor` cases (whole, fraction, negative, minimum
  int64, other currencies), and a round trip through `ParseDecimal`.
- `TestLegacyJPYCostControlKeepsValuesAndAcceptsEdits`: an older entry
  stored as 12,345 still reads `123.45`, a ¥150,000 entry saves and reads
  `150000`, and the baseline keeps `128.45`. Reading stored values as whole
  yen would fail it.
- `TestProjectReportingCurrencyAcceptsJPY`: a new JPY project, empty-project
  switches both ways, and the lock once ledger data exists.
- `TestWiresShowYenAsWholeNumbers`: project budget, budget rollup, and
  portfolio wires.
- `TestCostReservesReadInTheProjectCurrency`: a saved and a listed reserve
  read `2000000` for yen and `2000000.00` for dollars; an older fraction
  reads `123.45`.
- `TestFinancialReportSnapshotShowsYenAsWholeNumbers`: totals, ledger rows,
  and reserves in the financial report.
- Documents and DOCX/ODT exports print `1,284,500` for yen and never
  `1,284,500.00`, including table cells in combined reports.
  `TestFormatMoneyIn` checks that documents print the same digits as the
  screens.
- Project Settings offers Japanese yen, saves it, and restores it after a
  rejected change.
