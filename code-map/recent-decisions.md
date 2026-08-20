<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Recent decisions

This index points to current, durable decisions. It is not a session log;
historical rationale and superseded investigations remain in Git history.

## Architecture decisions

| Decision | Source of truth |
| --- | --- |
| SQLCipher project-database encryption and recovery-key hierarchy | [ADR-001](../docs/design/ADR-001-database-encryption-at-rest.md), `internal/users`, `internal/crypto`, `internal/db` |
| Cost Control Phase 1 uses project-local exact-money ledger rows and separate reserves; it does not import the legacy stakeholder/agile rollup or schedule EVM | `app_cost_control.go`, `internal/db/cost_control.go`, `internal/money`, `docs/user-guide.md`, `docs/beta-release-backlog.md` |
| Cost Control reporting is one supported currency per project (USD default); a project may select USD, EUR, GBP, CAD, AUD, JPY, or CHF only while it has neither a budget nor ledger entry, with no FX conversion | `internal/db/project.go`, `internal/db/cost_control.go`, `frontend/src/lib/components/project/ProjectSettings.svelte`, `internal/db/cost_control_test.go` |
| DuckDB for in-memory analytics and SQLCipher for project persistence | [ADR-002](../docs/design/ADR-002-duckdb-vs-sqlcipher-evaluation.md), `internal/analytics` |
| Maintained Go PDF renderer | [ADR-003](../docs/design/ADR-003-gofpdf-to-go-pdf-fpdf-migration.md), `internal/documents`, `internal/pdfmeta` |
| Combined-report and signed-export orchestration | `internal/reporting`, `app_documents.go` |

## Product and safety boundaries

- GoPMgr is local-first. Core project, scheduling, and export workflows do not
  require a hosted service.
- Frontend-supplied project paths are confined to the signed-in user's project
  directory before filesystem access.
- New projects use `.gopmgr`; compatible legacy project formats remain readable
  through the migration and project-path code.
- PAdES is the final PDF mutation. PDF/A changes require a new signature.
- CSV and TSV exports neutralize user-controlled formula prefixes through
  `internal/exportsafe`.

## How to update this map

Add an entry only when a decision changes a durable boundary or gives future
contributors a faster route to the source of truth. Link to code, tests, or an
ADR. Do not add hardware observations, personal paths, credentials, or a
chronological work diary.
