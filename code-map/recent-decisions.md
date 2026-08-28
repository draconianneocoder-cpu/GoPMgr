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
| Reusable supplier and item records are a signed-in-user convenience catalog, encrypted separately from both pre-login `system.db` and project files. Records are versioned and archived. Project-ledger rows already retain independent display snapshots of item, SKU, supplier name, invoice reference, quantity, and unit, never supplier contact PII; there is no live cross-database catalog reference or current catalog-assisted ledger selection. Attachment ZIP export binds each manifest archive name to its output entry and rechecks stored byte count/SHA-256 before writing that entry. | [Reusable procurement catalog](../docs/design/reusable-procurement-catalog.md), `internal/catalog`, `app_catalog.go`, `app_cost_control.go`, `app_cost_control_attachments.go`, `internal/db/cost_control.go`, `internal/db/cost_control_attachments.go`, `internal/export/attachments_zip.go`, `frontend/src/lib/components/project/ProcurementCatalog.svelte`, `frontend/src/lib/components/project/CostControlPanel.svelte`, `internal/db/cost_control_procurement_test.go`, `internal/db/cost_control_attachments_test.go`, `app_cost_control_attachments_test.go`, `internal/export/attachments_zip_test.go` |
| Cost Control Phase 1 uses project-local exact-money ledger rows and separate reserves; persistence lists only owning-project rows, and a cost type from another project is rejected before it can create a ledger or audit record. It does not import the legacy stakeholder/agile rollup or schedule EVM | `app_cost_control.go`, `internal/db/cost_control.go`, `internal/db/cost_control_test.go`, `internal/money`, `docs/user-guide.md`, `docs/beta-release-backlog.md` |
| Legacy Dashboard Budget values use canonical decimal strings across every Wails-facing project-metadata and summary route. The internal database/rollup models retain their compatibility floats and exact minor units; neither form is sent to JavaScript for this feature. | `app_project_wire.go`, `app_budget_wire.go`, `internal/money/money.go`, `app_budget_wire_test.go`, `frontend/src/lib/components/project/BudgetPanel.svelte`, `frontend/src/lib/components/project/ProjectSettings.svelte` |
| Portfolio Analytics keeps DuckDB's internal aggregate model private and exposes canonical decimal strings for every monetary total. Remaining is checked and computed in Go, while the renderer only formats the returned string and uses its sign for styling. It refuses to aggregate readable projects with different reporting currencies because no FX model exists; a successful wire states its one ISO code. | `app_portfolio_wire.go`, `app_foundation.go`, `internal/db/cost_control.go`, `internal/analytics`, `app_portfolio_analytics_test.go`, `app_portfolio_wire_test.go`, `frontend/src/lib/components/project/Portfolio.svelte` |
| Launchpad enables the Software-Dev Pack only after all requested starter artifacts succeed. A partial seed error retains its receipts and project, but leaves the pack disabled so the dashboard cannot imply a complete agile setup | `app_foundation.go`, `encryption_project_test.go`, `internal/templates/seeds.go`, `docs/beta-release-backlog.md` |
| Shared confirmation prompts are keyboard-contained: they focus Cancel on open, keep Tab within the prompt, and restore a connected trigger on close. Other dialogs make no such shared guarantee until individually verified. | `frontend/src/lib/components/ConfirmDialog.svelte`, `frontend/src/lib/components/ConfirmDialog.test.ts`, `frontend/src/lib/components/help/HelpReference.svelte` |
| Cost Control exposes the legacy Budget rollup only as clearly separate context. It never labels it Funding, and it does not present a remaining/unallocated metric until allocation, drawdown, and forecast semantics exist | `app_cost_control.go`, `frontend/src/lib/components/project/CostControlPanel.svelte`, `docs/user-guide.md`, `app_cost_control_test.go` |
| Cost Control reporting is one project currency (USD default). New projects and eligible empty-project changes may select USD, EUR, GBP, CAD, AUD, or CHF, with no FX conversion. Existing JPY projects preserve their original fixed-two-decimal values for read-only inspection; JPY cannot be newly selected or changed to, and the app does not claim exponent-aware JPY support | `internal/db/project.go`, `app_cost_control.go`, `frontend/src/lib/components/project/ProjectSettings.svelte`, `frontend/src/lib/components/project/CostControlPanel.svelte`, `app_cost_control_test.go`, `internal/db/cost_control_test.go`, `docs/user-guide.md`, `docs/beta-release-backlog.md` |
| Cost Control baselines are immutable backend-derived snapshots approved by the signed-in local account; they exclude the legacy Budget rollup and are not RBAC or electronic signatures | `internal/db/cost_control.go`, `app_cost_control.go`, `frontend/src/lib/components/project/CostControlPanel.svelte` |
| Cost Control reserve upserts resolve the persisted `(project_id, kind)` identity before writing the audit event, so a conflicting caller-supplied ID cannot create an audit reference to a nonexistent reserve; a failed audit append rolls the reserve write back | `internal/db/cost_control.go`, `internal/db/cost_control_test.go` |
| Cost Control Phase 2 implementation is gated by a proposed lifecycle and authority contract. Phase 1 reserves remain mutable assessed balances, not drawdown or authorization history; management-reserve release remains blocked pending project financial roles | [Cost Control Phase 2 lifecycle and authority contract](../docs/design/cost-control-phase-2-lifecycle.md), `internal/db/cost_control.go`, `docs/beta-release-backlog.md` |
| Cost Control classifies each ledger entry by direct/indirect attribution, fixed/variable behavior, and CapEx/OpEx/not-applicable treatment. These are independent reconciliation lenses rather than additive cost categories; inactive types remain visible on historical rows but cannot receive new entries | `internal/db/cost_control.go`, `app_cost_control.go`, `frontend/src/lib/components/project/CostControlPanel.svelte`, `app_cost_control_test.go` |
| Project financial exports are built from one read-transaction snapshot and keep Legacy Budget context visibly separate from Cost Control. They carry canonical decimal strings only, include ledger/reserve/baseline facts, and intentionally omit Phase 2 allocation, drawdown, forecast, EAC, and authority semantics. The landscape tables use width-aware clipping for descriptive fields, while their amount column is sized and tested for the full canonical signed-`int64` decimal range so exact money is never elided. | `app_financial_report.go`, `internal/documents/financial_report.go`, `internal/documents/financial_report_test.go`, `app_financial_report_test.go`, `frontend/src/lib/components/project/CostControlPanel.svelte` |
| Document, combined-report, signed, GnuPG, and schedule exports use a user-selected new destination in desktop mode. Primary and generated sidecars are no-replacement artifacts; exported path metadata stored in audit records is a basename rather than a private absolute directory. | `app_export_destination.go`, `internal/exportfs`, `app_documents.go`, `internal/reporting/reporting.go`, `app_pades_export_test.go` |
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
