<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: CC0-1.0
-->

# Recent decisions

A short, chronological index of PMForge's significant technical decisions and
their current status — the "why" behind the code, not the "what" (the code
is the source of truth for that). Full detail lives in the linked ADRs,
review docs, and `session-notes.md`. This file is generated/maintained
alongside the rest of `code-map/`; update it when a decision of similar
weight lands.

## Architecture Decision Records

| Date | ADR | Decision | Status |
|---|---|---|---|
| 2026-06-13 | [ADR-001](../docs/design/ADR-001-database-encryption-at-rest.md) | Per-user database encryption at rest via `mutecomm/go-sqlcipher/v4`, DEK wrapped by password + each recovery code | Implemented |
| 2026-06-23 | [ADR-002](../docs/design/ADR-002-duckdb-vs-sqlcipher-evaluation.md) | Evaluated replacing SQLCipher with DuckDB; **rejected** — DuckDB's encryption is too new (non-NIST, RNG CVE, `httpfs` auto-install). Adopted as a **complementary in-memory analytics engine** instead (Option B) | Implemented |
| 2026-06-25 | [ADR-003](../docs/design/ADR-003-gofpdf-to-go-pdf-fpdf-migration.md) | Migrated PDF library from archived `jung-kurt/gofpdf` to maintained `go-pdf/fpdf` (mechanical import-path swap, 38 files) | Implemented |

Design doc (not a formal ADR but decision-bearing): [duckdb-analytics-engine.md](../docs/design/duckdb-analytics-engine.md) — in-memory-only DuckDB engine behind the `duckdb` build tag, hardened against network/extension auto-install, feeding `internal/analytics`.

## Security review resolutions

| Date | Finding | Decision | Status |
|---|---|---|---|
| 2026-06-23 | F1 — unpinned AppImage build tools | Removed AppImage delivery entirely rather than pin+verify | Resolved |
| 2026-06-23 | F2 — security scanners configured but never run in CI | `govulncheck` made a blocking CI gate (default + `duckdb`-tagged build) | Resolved |
| 2026-06-23 | F3 — `errcheck`/`staticcheck`/`unused` disabled in `.golangci.yml` | Re-enabled all three; ~43-issue backlog cleared in code, not suppressed | Resolved |
| 2026-06-23 | F4 — DEK held as immutable hex string (can't be zeroed) | Accepted — SQLCipher's `PRAGMA key` requires a string literal; no `[]byte` path exists. Narrowed the one site holding it longer than needed | Accepted risk |
| 2026-06-29 | F-1 — `EncryptProjectAtRest`/`SecureArchive`/`OpenProject`/`IsProjectEncrypted` skipped the `projectPathFor` confinement check `DeleteProject`/`CloneProject` already used | All four routed through `projectPathFor`; regression test added | Resolved |
| 2026-06-29 | F-2 — encrypted DSN built by string concat; `?` in a path could inject `_pragma_*` options | `encryptedDSN` now rejects paths containing `?`/`#` | Resolved |
| 2026-06-29 | F-3 — CSV/XLSX export had no spreadsheet formula-injection neutralization (CWE-1236) | Added `internal/exportsafe`; applied to CSV sinks. XLSX left alone — verified empirically that excelize never emits formula cells for plain strings | Resolved |
| 2026-06-29 | F-4 — no login throttling/lockout | Accepted — Argon2id cost (64 MiB, t=3) makes brute force impractical for a local-first app with no live users yet | Accepted, no action |
| 2026-06-29 | F-5 — 5 Dependabot alerts, all in frontend build/dev tooling | Bumped `vite` 5→8, `@sveltejs/vite-plugin-svelte` 4→7; 0 vulnerabilities after | Resolved |

## Frontend test infrastructure (2026-07-04)

- Added **Vitest + `@testing-library/svelte` + jsdom** for behaviour-level frontend tests (the app previously had only `svelte-check` + node grep-scripts). Toolchain stayed on Vite 8 / Svelte 5, 0 npm vulnerabilities.
- Pattern: pure rendering geometry lives in sibling `*_geometry.ts` modules (fast unit tests); presentational components render from props with no Wails bridge so they mount directly. The Gantt bar canvas was extracted from `GanttEditor.svelte` into a testable `GanttBars.svelte` + `gantt_geometry.ts` — the extraction makes the tests cover the production render path, not a copy.
- `make frontend-stability` now runs `npm test` (Vitest), so it is enforced by `make verify` / `make check-release`. Test files are excluded from the app `svelte-check`.

## Other notable decisions (from `session-notes.md`)

- **2026-06-15** — Wails main package moved to the repo root (required by `wails build`); `cmd/` directory retired.
- **2026-06-15** — Per-project unique-ID subfolders adopted, with legacy flat-file layout preserved for backward compatibility (`projectPathFor` accepts both).
- **2026-06-20** — `CreateAccount` duplicate check changed from case-sensitive to `lower(username) = lower(?)` after an APFS case-insensitive collision let two accounts share one data directory.
- **2026-06-22** — Wails v2.9.2 → v2.12.0 upgrade, pulling `golang.org/x/crypto`/`x/net`/`x/sys` security-hygiene bumps via `go mod tidy`.

## Open / deferred (not yet decided or implemented)

- **Advanced Resource Levelling** (ROADMAP Phase 1) — _horizon slice done end-to-end._ The leveling horizon is the exported `DefaultLevelingHorizon`, configurable per schedule via `LevelResourcesWithOptions(..., LevelingOptions{Horizon})`, which returns the `ErrLevelingHorizonExceeded` sentinel plus `LevelingResult.UnplacedTaskIDs` instead of silently capping (`internal/kernel/resources.go`, 2026-07-02). The production path is wired: `App.LevelChartResources` returns a `LevelResult` (pinned + unplaceable IDs/labels), routing a cycle as a hard error but a horizon overflow as a non-fatal warning; the CPM editor shows a dismissible “still overallocated” warning (2026-07-03). EDF/LTF leveling heuristics added as a `LevelingOptions.Strategy` selector, wired through `App.LevelChartResources(chartID, strategy)` and a CPM-editor dropdown (LTF default preserves prior behaviour, 2026-07-03). Priority-override (`LevelingOptions.PriorityCritical`) protects the critical path, wired as `LevelChartResources(chartID, strategy, priorityCritical)` + a “Protect critical” checkbox. Partial-assignment splitting (`LevelingOptions.AllowSplitting` + `Task.WorkDays`, with `ResourceUsage`/`DetectOverallocations` made split-aware) interrupts tasks across non-contiguous days; surfaced read-only via `App.PreviewSplitLeveling` + a “Preview splitting” button (2026-07-04). Split schedules are now also **persistable and renderable**: `dag.WorkSegment` + `LayeredNode.WorkSegments` (task-relative offset runs) store a split, `App.LevelChartResources(chartID, strategy, priorityCritical, allowSplitting)` persists them, the Gantt layout emits absolute segments, and `GanttEditor` draws interrupted bars via a “Level (split)” action (2026-07-04). The PDF Gantt renderer (`pdfrender/gantt.go`) draws the same interrupted bars so exports match the screen (2026-07-04). _Remaining on the same foundation:_ EVM treats the split span as contiguous (cost/work unaffected). **RICE-144 Advanced Resource Levelling is complete.**
- **Risk/Issue/Opportunity workflow + Risk Matrix chart** (22nd chart kind) — not started.
- **RFC 3161 PAdES timestamping** — not started (the audit-trail half of this roadmap item, `audit_events`, is done).
- **Portfolio rollup SPI/CPI** — `RunPortfolioAnalytics` reports 0 ("n/a") for these by design, pending a later enhancement (see the method's doc comment in `main.go`).
- **RPM Fedora runtime** — built on Ubuntu, cross-distro behavior unverified on a real Fedora box.
- **Windows NSIS scaffold** — `build/windows/` not committed; first Windows release build will ship a default-branded installer until it's run once and the generated scaffold is committed.
- **PAdES trusted-chain / Acrobat validation** — blocked on a real trusted signing source; `make check-pades-trusted` reports "not configured" in the interim.
