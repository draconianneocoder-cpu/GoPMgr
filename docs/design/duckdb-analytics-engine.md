<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# DuckDB analytics engine

**Status:** Implemented for production and package builds; retirement is
approved through the gated [2026-10-27 directive](../review/architecture-directive-2026-10-27.md).
The replacement has not qualified for production cutover.
**Decision record:** [ADR-002](ADR-002-duckdb-vs-sqlcipher-evaluation.md)

## Boundary

DuckDB is an in-memory analytical helper, not a project database. SQLCipher
remains the persistent system of record. The application reads authorized rows
from project databases and supplies them to `internal/analytics`; DuckDB does
not access encrypted project files directly.

```text
SQLCipher project databases -> internal/analytics -> in-memory DuckDB -> result
```

## Build behavior

`//go:build duckdb` selects the DuckDB implementation. Production and package
builds enable the tag. Untagged development builds compile the unavailable stub
so the rest of the application remains buildable without the analytical engine.

## Safety rules

- Keep all project writes in the established SQLCipher-backed packages.
- Use parameterized operations for data passed into DuckDB.
- Do not enable runtime extension installation or network-backed readers.
- Accept imports only from an explicit user-selected path. This is the required
  boundary; the current implementation does not fully enforce it (see below).
- Keep kernel scheduling and earned-value calculations independent of DuckDB.
- Reject a Portfolio Analytics rollup when readable projects have different
  reporting currencies. The application has no FX model, so adding exact
  minor-unit values from different currencies would still be a false total.
  A future grouped-by-currency portfolio contract requires an explicit Wails
  and UI design; do not silently substitute one.

## Temporary filename restriction (LD-6)

At baseline `14c921d`, SQL-quoted filenames still expanded as DuckDB globs,
allowing a name such as `data*.csv` to import unselected neighboring files.
The maintainer approved temporary rejection on 2026-10-29, recorded in
[the directive's §7](../review/architecture-directive-2026-10-27.md#7-temporary-tabular-import-filename-restriction-2026-10-29).

`ImportTabular` now returns `ErrAmbiguousImportPath` and no dataset if the
supplied path, resolved target, or absolute reader path contains `*`, `?`, or `[`, including in
parent folders. Rename the file or containing folder before retrying. This
also rejects literal bracket names and symlinks whose targets contain these
characters. All six formats remain supported: CSV, TSV, TXT, Parquet, JSON
and NDJSON. An alias selects its reader by the chosen filename's extension.
Source symlinks are resolved before cleaning parent traversal so the reader
uses the selected source rather than a lexical sibling. When the absolute
selected path positively identifies that source, its pathname is retained so
DuckDB's inferred Hive partition columns remain unchanged. If the lexical path
is missing or its identity differs or cannot be established, the canonical path
takes precedence; path-derived metadata may then differ.

The code retains a
best-effort `allowed_directories` setting and ignores its result; with external
access enabled, that setting does not enforce confinement. Do not describe
the current import boundary as an enforced directory sandbox or single-file
capability.

The restriction prevents glob expansion; it is not a sandbox against another
process replacing paths concurrently. Revisit it when the replacement reader
qualifies for literal selected-file access. This correction does not upgrade
DuckDB or waive replacement qualification gates. See
[path-boundary verification](../testing/tabular-import-paths.md).

## Verification

Run the `internal/analytics` tests with the applicable build tags and verify a
production build before shipping. Dependency and packaging requirements are in
[DEPENDENCIES.md](../DEPENDENCIES.md).
