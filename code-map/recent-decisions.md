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
| Cost Control exposes the legacy Budget rollup only as clearly separate context. It never labels it Funding, and it does not present a remaining/unallocated metric until allocation, drawdown, and forecast semantics exist | `app_cost_control.go`, `frontend/src/lib/components/project/CostControlPanel.svelte`, `docs/user-guide.md`, `app_cost_control_test.go` |
| Cost Control reporting is one project currency (USD default); a project may select USD, EUR, GBP, CAD, AUD, JPY, or CHF only while it has no Budget value, ledger entry, non-zero reserve balance, or approved Cost Control baseline, with no FX conversion. The current Cost Control wire accepts every selectable currency using two decimal places; JPY is therefore accepted under that fixed convention but is not yet exponent-aware or standards/interoperability-complete | `internal/db/project.go`, `internal/db/cost_control.go`, `app_cost_control.go`, `frontend/src/lib/components/project/ProjectSettings.svelte`, `internal/db/cost_control_test.go`, `docs/beta-release-backlog.md` |
| Cost Control baselines are immutable backend-derived snapshots approved by the signed-in local account; they exclude the legacy Budget rollup and are not RBAC or electronic signatures | `internal/db/cost_control.go`, `app_cost_control.go`, `frontend/src/lib/components/project/CostControlPanel.svelte` |
| Cost Control classifies each ledger entry by direct/indirect attribution, fixed/variable behavior, and CapEx/OpEx/not-applicable treatment. These are independent reconciliation lenses rather than additive cost categories; inactive types remain visible on historical rows but cannot receive new entries | `internal/db/cost_control.go`, `app_cost_control.go`, `frontend/src/lib/components/project/CostControlPanel.svelte`, `app_cost_control_test.go` |
| Opening a pre-Cost-Control project adds its exact-money/currency and Cost Control schema additively; legacy project currency defaults to USD without conversion. Plaintext-to-encrypted migration retains an independent protected plaintext backup, completes all fallible preparation before one replace-existing publish, and preserves the canonical plaintext path on synchronous failure | `internal/db/sqlite.go`, `internal/db/encryption.go`, `internal/db/replace_file_*.go`, `internal/db/encryption_test.go`, `internal/db/cost_control_migration_test.go` |
| Login submission is locally idempotent while authentication is in progress; the UI fails open to account creation only when the administrator check resolves false or fails | `frontend/src/lib/components/auth/Login.svelte`, `frontend/src/lib/components/auth/Login.test.ts`, `app_session.go` |
| Saved GoPMgr Penpot evidence boards use the separately configured self-hosted bridge. Penpot AI Kit behavior was installed with `--mode none`, so that installation did not replace the separately managed bridge configuration | `docs/beta-release-backlog.md`, saved Penpot board `Cost Control — Penpot Recovery Evidence (2026-08-21)` |
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
