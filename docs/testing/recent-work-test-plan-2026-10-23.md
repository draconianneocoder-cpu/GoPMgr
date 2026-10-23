<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Test plan for PRs #125 to #131

## Scope

This plan covers the work merged in #125 to #130 and the yen change in #131:

- PDF/A-2u output and the ToUnicode and font-name repair.
- IBM Plex in documents, exports, and the app.
- Font licence files in the release packages.
- Amounts in the project's reporting currency.
- The localization foundation.
- Yen as a reporting currency (ADR-009).

## Method

Line coverage alone does not show whether new code is tested. Each change
was measured by diff coverage: the lines a PR added that no test runs.

1. Run `go test -coverprofile` over `.` and `./internal/...`, and
   `vitest run --coverage` with the JSON reporter.
2. Intersect the covered lines with `git diff -U0 28caa1e..HEAD`.

Before this plan, the added lines had this coverage:

| Layer | Added executable lines | Covered |
| --- | --- | --- |
| Go | 737 | 93.8% |
| Frontend | 249 | 93.2% |

Every uncovered line was then read and classified. Some lines are tested
behavior: a test now covers them. Others are only reachable when a library
fails, such as a `godocx.NewDocument` error, or are static help text; these
are left untested. Covered lines were also checked for assertions, since a
line that runs is not always checked.

## Tests by area

| Area | Test | Type | PR |
| --- | --- | --- | --- |
| Yen reserves | `TestCostReservesReadInTheProjectCurrency` | integration | #131 |
| Yen financial report | `TestFinancialReportSnapshotShowsYenAsWholeNumbers` | integration | #131 |
| Yen table cells in combined reports | `TestBuildCombinedReport_TableAmountsUseTheReportCurrency` | PDF stream | #131 |
| Documents and screens agree | `TestFormatMoneyIn` | table-driven | #131 |
| System language tags | `canonicalTag` and format cases in `i18n.test.ts` | table-driven | this PR |
| Startup language | `follows the system at startup`, and `make frontend-smoke` | unit, smoke | this PR |
| Formats per locale (ADR-008 test plan) | `formats per locale` in `i18n.test.ts` | table-driven | this PR |
| Word template guards | `TestApplyDOCXFont_RejectsAnUnexpectedTemplate` | fault seeding | this PR |
| Imported font names in Word files | `TestApplyDOCXFont_EscapesNamesAndListsEachFontOnce` | unit, XML | this PR |
| Font licence check | `FuzzCheckEmbeddingAllowed` | fuzz | this PR |
| PDF font names | `FuzzTrueTypePostScriptName` | fuzz | this PR |
| PDF font repair | `FuzzRepairFPDFFonts` | fuzz | this PR |
| App Settings font picker | `default document font` in `AppSettings.test.ts` | component, accessibility | this PR |
| Project Settings reset | `shows IBM Plex Sans after Reset defaults` | component | this PR |

Each new assertion was checked against a deliberate fault. For example,
listing reserves with `Decimal()`, or printing table cells with
`formatMoney`, fails the yen tests.

## Defects found

- **System language tags broke formatting.** In automatic mode, the raw
  system tag became the format locale. `Intl` throws a `RangeError` for a
  POSIX tag such as `de_CH`, `en_US.UTF-8`, or `C`, which would break every
  date and number on a screen that uses the catalog. `canonicalTag` now turns
  such tags into BCP 47 tags, and drops `C`, `POSIX`, and malformed tags.
- **Reset left the project font picker blank.** Reset defaults set the font
  to the saved empty value, which no option matches. The picker showed no
  font and no description, and Apply font was disabled until the screen
  reloaded. An empty value now means IBM Plex Sans, as `GetDefaultFont`
  reports.

## Fuzzing

The fuzz targets run their seeds in every `go test`. To fuzz one locally:

```sh
go test -run '^$' -fuzz '^FuzzRepairFPDFFonts$' -fuzztime=2m ./internal/pdfmeta
```

Each target ran for 90 to 120 seconds without a failure.

## Not covered

- Error returns from `godocx`, `excelize`, and the zlib reader, which need a
  failing library.
- The schedule-report earned-value lines print two decimals for every
  currency, because that report's data carries no currency yet (ADR-009).
- The ADR-008 language picker, layout, CJK text, and input-method tests,
  which wait for the picker and the first translations.
- Help text, checked by `make help-guide-current` rather than by unit
  tests.
