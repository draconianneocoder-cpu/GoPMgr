<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# PMForge Roadmap

This document is the public-facing roadmap summary. The full strategic plan,
RICE scores, dependency evaluations, and implementation detail are in
`PMForge-Strategic-Roadmap-2026.docx` at the repository root. Architecture
decisions are in `docs/design/ADR-*.md`.

## Current State (v1.1.0-rc.1)

PMForge ships with a kernel that performs CPM scheduling, Earned Value
Management (BAC/PV/EV/AC/SV/CV/SPI/CPI/EAC/ETC/VAC), baselines, anchor
scheduling, basic resource levelling, and typed dependency links. The
frontend covers 21 chart types, 25 document kinds, Agile and Six Sigma
methodology packs, MSPDI import/export, PDF/A-3, PAdES digital signing,
SQLCipher encryption at rest, and Argon2id authentication.

The PDF rendering layer was migrated (post-rc.1) from the archived
`jung-kurt/gofpdf` to the maintained `go-pdf/fpdf` community fork
(ADR-003). All downstream PDF generation is now on the maintained path.

## Phase 1 — Kernel Depth (Q3–Q4 2026)

Priority work that deepens the scheduling kernel and delivers the
highest-value analytical capabilities. No new external dependencies are
required for any Phase 1 item.

**Monte Carlo Schedule Simulation** (RICE 180)
Duration uncertainty per task using Triangular (optimistic / most likely /
pessimistic) inputs via `gonum/stat/distuv` (already in `go.mod`). Outputs:
P10/P50/P80/P90 finish dates, cost-at-completion distribution, critical-path
sensitivity index. Implementation in `internal/kernel/montecarlo.go`.

Acceptance criterion: a test using Triangular(1,4,9) with N=5000 iterations
must yield (a) a simulated mean within ±2% of the analytical mean (4.667 days)
and (b) a simulated median (P50) within ±2% of the analytical median (≈4.51
days). Both gates must pass. This is the mandatory convergence gate before the
feature ships.

**Advanced Resource Levelling** (RICE 144)
Enhance `internal/kernel/resources.go` with priority-override for critical
tasks and partial-assignment splitting (split a task across days when
resource demand exceeds supply).

_Done:_ the `levelingHorizon = 10000` constant is now the exported
`DefaultLevelingHorizon`, overridable per schedule via
`LevelResourcesWithOptions(tasks, plan, LevelingOptions{Horizon})`, which
returns `ErrLevelingHorizonExceeded` (with the unplaceable task IDs in
`LevelingResult`) instead of silently capping. The pre-existing
`LevelResources`/`LevelResourcesWithPlan` wrappers keep their original
`bool` signature for backward compatibility. The production path is wired
end-to-end: `App.LevelChartResources` now returns a `LevelResult` (pinned
count plus unplaceable task IDs/labels) and the CPM editor shows a
dismissible “N task(s) still overallocated” warning instead of silently
capping. The Earliest-Deadline (EDF) and Least-Total-Float (LTF) heuristics
are implemented as a `LevelingOptions.Strategy` selector, wired through
`App.LevelChartResources(chartID, strategy)` and exposed as a heuristic
dropdown in the CPM editor (LTF is the default and preserves prior
behaviour). _Remaining:_ priority-override for critical tasks and
partial-assignment splitting.

**What-If / Scenario Analysis** (RICE 144)
Fork a named scenario from the current plan, apply changes, and compute the
resulting CPM/EVM deltas without modifying the live project. Scenarios live
in the `.pmforge` SQLite file as first-class rows alongside baselines.

**Richer Audit Trail + PAdES Timestamping** (RICE 120)
Append-only event log in `internal/audit` with structured fields (actor,
timestamp, entity type/ID, before/after JSON diff). RFC 3161 timestamping
for PAdES signatures. PDF/A-3 embedded audit attachment support.

**Risk/Issue/Opportunity Workflow + Risk Matrix Chart** (RICE 120)
Structured risk register (ID, title, probability, impact, score, mitigation,
owner, status, linked task). A 22nd chart type: 5×5 probability/impact heat
map rendered via the existing pdfrender pipeline.

**Portfolio Roll-ups** (RICE 90)
Aggregate EV/AC/PV/SPI/CPI across the projects the user has open or has
recently opened. Roll-up dashboard in `frontend/src/lib/components/project/Portfolio.svelte`.

## Phase 2 — Professional Controls (2027)

Contract and Procurement module, Advanced Cost Forecasting (ETC/EAC
variants: BAC/CPI, BAC/SPI, independent estimate, mixed), Formal Rebaselining
workflow, EVM+Agile Hybrid (sprint velocity overlaid on EV curves), and
Dynamic RACI integration (stakeholder records drive RACI automatically).

## Phase 3 — Scale and Reach (2027)

Virtualized Gantt renderer (Svelte 5 + HTML Canvas, no new Go dependency,
target: 10 000+ tasks at 60 fps), Local AI via `github.com/ollama/ollama/api`
(Go SDK, MIT, requires user-installed Ollama — never bundled), Accessibility
(WCAG 2.1 AA audit pass), i18n (`go-i18n/v2` + `@inlang/paraglide-js`),
Local Collaboration (read/write to a Syncthing-managed shared folder — see
note below), and a Mobile Companion web view.

**Local Collaboration note:** PMForge must acquire an exclusive SQLite lock
before opening a project and must release it on close. The sync folder must
not be transferred by Syncthing while PMForge holds the lock. The
recommended user workflow is: close the project in PMForge, allow Syncthing
to sync, reopen on the other machine. PMForge will detect and warn on
concurrent-open conflicts detected via the WAL/SHM presence heuristic.
Merge conflict resolution of concurrent edits is out of scope; the model is
"one writer at a time."

## Phase 4 — Ecosystem (2028+)

Self-hosted local sync server option (for teams that want shared access
without Syncthing), Plugin architecture via `github.com/tetratelabs/wazero`
(pure Go, zero-CGo WASM sandbox, Apache-2.0), Primavera XER/PMXML import,
Certification packs (ISO 21502, PMBOK 7), and open data standard exports.

## Dependency Policy

New dependencies require a documented evaluation (ADR or inline rationale in
the implementing PR) covering: maintenance status, CGO requirement, licence
compatibility (GPL-3.0-or-later or compatible), and whether an existing
in-repo package already covers the need. Preference: Go or Rust; no CGO
without justification; no archived libraries.

## Architecture Decision Records

| ADR | Title | Status |
|-----|-------|--------|
| [ADR-001](docs/design/ADR-001-database-encryption-at-rest.md) | Per-user database encryption at rest | Implemented |
| [ADR-002](docs/design/ADR-002-duckdb-vs-sqlcipher-evaluation.md) | DuckDB vs SQLCipher evaluation | Implemented |
| [ADR-003](docs/design/ADR-003-gofpdf-to-go-pdf-fpdf-migration.md) | PDF library migration: gofpdf → go-pdf/fpdf | Implemented |

## Manual Action Required

**GitHub Security Advisories** — enable via repo Settings → Security →
Advisories. This cannot be automated. Without it, Dependabot and the
GitHub security advisory feed will not surface CVEs against PMForge's
dependency tree in the repository UI.

## What Is Not on the Roadmap

Anything that violates the principles in `VISION.md`. Specifically: mandatory
cloud sync, server-side AI processing, proprietary export formats, and
real-time collaborative editing over a network are not planned.
