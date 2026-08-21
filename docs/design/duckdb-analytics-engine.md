<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# DuckDB analytics engine

**Status:** Implemented for production and package builds
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
- Accept imports only from an explicit user-selected path and keep directory
  confinement in the import boundary.
- Keep kernel scheduling and earned-value calculations independent of DuckDB.
- Reject a Portfolio Analytics rollup when readable projects have different
  reporting currencies. The application has no FX model, so adding exact
  minor-unit values from different currencies would still be a false total.
  A future grouped-by-currency portfolio contract requires an explicit Wails
  and UI design; do not silently substitute one.

## Verification

Run the `internal/analytics` tests with the applicable build tags and verify a
production build before shipping. Dependency and packaging requirements are in
[DEPENDENCIES.md](../../DEPENDENCIES.md).
