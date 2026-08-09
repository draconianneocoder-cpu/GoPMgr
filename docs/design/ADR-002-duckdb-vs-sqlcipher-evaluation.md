<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-002: DuckDB analytics and SQLCipher project storage

**Status:** Implemented
**Decision date:** 2026-06-23
**Related:** [ADR-001](ADR-001-database-encryption-at-rest.md)

## Context

GoPMgr needs cross-project analytical rollups and tabular import without
weakening its transactional project store, encryption model, repair behavior,
or backup format. DuckDB is optimized for analytical scans; SQLCipher-backed
SQLite is the established project system of record.

## Decision

Keep SQLCipher-backed SQLite as the persistent project database. Use DuckDB
only as an in-memory analytical engine behind `internal/analytics` and the
`duckdb` build tag. Shipped builds enable that tag; an untagged development
build uses the explicit unavailable stub.

GoPMgr reads already-authorized project data, passes it to DuckDB through
parameterized operations, and discards the analytical database when the
operation ends. DuckDB does not open, modify, or persist project databases.

## Consequences

- Scheduling and earned-value calculations remain in `internal/kernel`.
- DuckDB supports portfolio aggregation and approved tabular import paths; it
  does not replace project persistence.
- The CGO dependency and its packaging impact are isolated to builds that
  enable the `duckdb` tag.
- Network extension installation remains disabled. File import accepts only
  an explicit user-selected path and applies directory confinement.

## Rejected alternative

Replacing SQLCipher project storage with DuckDB would change transaction,
concurrency, repair, migration, and encryption assumptions at once. The
benefit does not justify that risk for a local project-management application.

## Verification

The implementation and tests live in `internal/analytics`; build behavior is
checked by the production build and release gates. See
[duckdb-analytics-engine.md](duckdb-analytics-engine.md) for the operational
boundary.
