<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# DuckDB retirement parity record

On 2026-11-02 a Go-native engine replaced DuckDB in `internal/analytics`
([owner decision](../review/ADR-012-DUCKDB-RETIREMENT-OWNER-DECISION.md)).
This record lists every difference between the two engines' results, with
its reason, as DF-G2 requires. The test `TestImportTabularParityWithDuckDB`
keeps it true: it fails on a difference not declared here and on a declared
difference that no longer occurs.

## Baseline

Before DuckDB was removed, the DuckDB engine (DuckDB 1.5.6 through duckdb-go
v2.10506.0) imported each fixture in `internal/analytics/testdata/import/`.
Its columns, row count, first five rows and last row are kept in
`duckdb-parity.json`. The baseline is only for comparison. Neither engine is
the correctness oracle: expected values in `TestImportTabularFixtures` and the
other tests were written by hand (DF-A11).

## Import results

20 of 32 fixtures give the same result. These 12 differ:

| Fixture | DuckDB | Go engine | Why |
|---|---|---|---|
| `bigint.csv` | Both values became one rounded float, 9223372036854776000 | Exact digits, as text | A value was silently changed |
| `numbers.json` | 9007199254740993 as a number, which JavaScript reads as …992 | Exact digits, as text | A value was silently changed in the UI |
| `empty.csv` | One empty column named `column0` | Error: the file is empty | The column did not exist |
| `ragged.csv` | Each line read as one text column named `a,b,c` | Error naming the line with the wrong field count | The file is malformed |
| `nested.json` | Lists and objects, shown in the UI as `[object Object]` | Compact JSON text | The value was unreadable |
| `types.csv` | Dates and timestamps as UTC times (`2026-01-02T00:00:00Z`) | The text as written | DuckDB invented a time zone |
| `types-*.parquet` (6 codecs) | Dates and timestamps as UTC times; decimals and nested values as objects | Plain dates; timestamps without an invented zone; exact decimal text; compact JSON; 64-bit values beyond 2^53 as text | Same reasons as above |

## Legacy defects

The DF0 evidence record lists ten defects in the DuckDB engine
([LD-1 to LD-10](adr-012-df0-evidence.md)). Each is now covered:

| Defect | Go engine | Test |
|---|---|---|
| LD-1: a wide Parquet decimal read as an unrelated float | Exact decimal text | `TestDecimal256KeepsExactDigits` |
| LD-2: one huge integer turns a column into rounded floats | Integer tokens outside JavaScript's safe range stay exact as text, including columns containing fractional values | `bigint.csv`, `numbers.json`, `TestImportDelimitedMixedNumbersKeepIntegerDigits` |
| LD-3: JSON `1e400` read as +Inf | Kept as the text `1e400` | `TestCellsAlwaysEncode` |
| LD-4: an empty CSV gives a phantom `column0` | Error | `empty.csv` |
| LD-5: malformed CSV and JSON accepted silently | Error, and no data | `TestImportTabularFailures`, `ragged.csv` |
| LD-6: the path is a glob, so unselected files are read | Only the selected file is read; `*`, `?` and `[` are ordinary characters | `TestImportTabularReadsLiteralNames` |
| LD-7: decimals and nested values shown as `[object Object]` | Text | `nested.json`, `types-*.parquet` |
| LD-8: integers beyond 2^53 rounded by JavaScript | Text outside ±(2^53−1) | `bigint.csv`, `numbers.json`, `TestImportDelimitedMixedNumbersKeepIntegerDigits` |
| LD-9: a larger file reported as 10,000 rows | `truncated` is set, and the Portfolio page says only the first 10,000 rows were read | `TestImportTabularTruncationIsExact` |
| LD-10: NaN or infinity makes the result unencodable, and Wails exits | Kept as text (`NaN`, `+Inf`, `-Inf`) | `TestCellsAlwaysEncode` |

The 2026-11-03 mixed-column regression closes a gap in the initial Go
replacement: a fractional value promoted the column to float and rounded
integer cells before their JSON return. Integer tokens outside ±(2^53−1)
now remain text even in those columns. Safe integers in a float column remain
numbers, and empty cells remain null. Decimal and exponent tokens retain their
existing float semantics; this is not an exact-decimal arithmetic guarantee.

## Other behaviour

- **Kept:** delimiter sniffing (comma, tab, semicolon, pipe), header
  detection, duplicate-name suffixes (`a`, `a_1`), leading zeros kept as text,
  a UTF-8 byte-order mark ignored, a `json` column for an array of scalars,
  Hive-style `key=value` partition columns from the selected path, symlinks
  read through to their target, and the 10,000-row cap.
- **New limits:** one CSV record or JSON row may not exceed 16 MiB
  (`ErrRecordTooLarge`), and reading a Parquet file may not need more than
  512 MiB (`ErrParquetMemory`). A FIFO, device or directory is refused without
  blocking. DuckDB had none of these limits.
- **Portfolio rollup:** the same sums and indices as DuckDB, from exact
  minor units. Like DuckDB's HUGEINT sum, the Go engine adds in 128 bits, so
  only a final total outside the 64-bit range fails; the error is now
  `ErrRollupOverflow` instead of a database scan error. Actual cost, earned
  value and planned value of a project without EVM are still left out, even
  when huge. Covered by `rollup_test.go` and
  `app_portfolio_analytics_test.go`.
