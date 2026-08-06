<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# GoPMgr Roadmap

This document is the public-facing roadmap summary. The full strategic plan,
RICE scores, dependency evaluations, and implementation detail are in
`GoPMgr-Strategic-Roadmap-2026.docx` at the repository root. Architecture
decisions are in `docs/design/ADR-*.md`.

## Current Development State (1.1.0 line)

GoPMgr ships with a kernel that performs CPM scheduling, Earned Value
Management (BAC/PV/EV/AC/SV/CV/SPI/CPI/EAC/ETC/VAC), baselines, anchor
scheduling, basic resource levelling, and typed dependency links. The
frontend covers 22 chart types, 25 document kinds, Agile and Six Sigma
methodology packs, MSPDI import/export, PDF/A-3, PAdES digital signing,
SQLCipher encryption at rest, and Argon2id authentication.

Since the public alpha, the PDF rendering layer was migrated from the archived
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

**Advanced Resource Levelling** (RICE 144) — **done.**
`internal/kernel/resources.go` now supports the full set: a configurable
leveling horizon, the EDF/LTF heuristics, priority-override for critical
tasks, and partial-assignment splitting.

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
behaviour). Priority-override (`LevelingOptions.PriorityCritical`) protects
the critical path from being delayed by floating tasks and is exposed as a
“Protect critical” checkbox. Partial-assignment splitting
(`LevelingOptions.AllowSplitting` + `Task.WorkDays`) interrupts a task
across non-contiguous days when contiguous placement would finish later;
splitting is surfaced read-only via `App.PreviewSplitLeveling` and a
“Preview splitting” button in the CPM editor. Split schedules can also be
**persisted and rendered**: `App.LevelChartResources(..., allowSplitting)`
stores each split task's working-day runs as node `WorkSegments`, the Gantt
layout emits them as absolute bar pieces, and the Gantt editor draws split
(interrupted) bars via a “Level (split)” action; the persisted segments are
a snapshot that the Gantt editor clears on any manual schedule edit so stale
interrupted bars can't render until the schedule is re-leveled. The PDF
Gantt renderer draws the same interrupted bars, so exports match the
on-screen view. EVM now consumes that same persisted `WorkSegments` snapshot:
planned value accrues only inside worked segments and remains flat through
idle gaps, while ordinary tasks retain linear ES-to-EF accrual.

**What-If / Scenario Analysis** (RICE 144)
Fork a named scenario from the current plan, apply changes, and compute the
resulting CPM/EVM deltas without modifying the live project. Scenarios live
in the `.gopmgr` SQLite file as first-class rows alongside baselines.

**Richer Audit Trail + PAdES Timestamping** (RICE 120)
Append-only event log in `internal/audit` with structured fields (actor,
timestamp, entity type/ID, before/after JSON diff). RFC 3161 timestamping
for PAdES signatures. PDF/A-3 embedded audit attachment support.

**Risk/Issue/Opportunity Workflow + Risk Matrix Chart** (RICE 120)
Structured risk register (ID, title, probability, impact, score, mitigation,
owner, status, linked task). A 22nd chart type: 5×5 probability/impact heat
map rendered via the existing pdfrender pipeline.

_Foundation complete:_ `risk_matrix` is registered as the fifth Matrix-engine
kind with a typed backend layout, deterministic 5×5 scoring bands, validation,
vector PDF rendering, dashboard creation, and a dedicated editor for risks,
active issues, and opportunities. Risk Register documents can optionally link
the chart for combined-report embedding. The document editor now provides an
explicit **Refresh Risk Matrix** action that validates saved register rows and
then replaces the linked chart items, preserving chart metadata and audit
history. The write is deliberately one-way and user-invoked so chart-only
changes are never overwritten without a visible action.

**Agent Plugin Interface + Built-in MCP Server** (RICE 108)
Almost all of GoPMgr's real capability lives in Go services behind the
Wails `App` struct — CPM/PDM scheduling, constraints, baselines, EVM,
resource levelling, calendars, scenarios, the Agile pack, and the
hash-chained audit trail. An AI agent that is supposed to create, update,
and manage projects needs controlled access to that kernel, not UI
scraping or a generic REST surface, so the integration point is a plugin
interface whose primary built-in consumer is a local MCP (Model Context
Protocol) server. A new `internal/plugin` (or `internal/agent`) package
defines a minimal contract (`Name`, `Init(host HostAPI) error`,
`Shutdown`); `HostAPI` is obtained only through the existing locked `App`
helpers (`requireUser`/`requireDB`, write lock for mutations) and exposes
domain operations, never raw SQL or unconstrained file access, so every
mutating call routes through the same services the UI already calls and
keeps writing to the `audit_events` hash chain.

The built-in MCP plugin runs a local, localhost-only server (stdio for
local agent harnesses; SSE/HTTP only if a harness needs it), implemented
directly against the MCP JSON-RPC 2.0 wire format with `encoding/json` and
`bufio` — no new external dependency, consistent with the no-new-deps rule
above. Tools are intent-oriented and deliberately few (project/schedule
snapshot, critical path, task update, recompute, baseline set/compare, EVM
at a status date, resource levelling, Agile board/sprint operations,
chart/document listing, scenario activate/promote, and read access to
recent audit events and the current permission scope) rather than a 1:1
mirror of every table. Tool calls are scoped by permission level
(read-only / schedule edit / structural / admin), destructive or
high-impact actions require human confirmation, an agent session is bound
to one explicitly-opened project, and per-user isolation plus private
file modes (0600/0700) are not weakened. GoPMgr's own code currently
spawns zero goroutines — the Wails runtime is the only spawner today — so
the MCP server's accept/read loop is a deliberate, explicitly isolated
exception to that invariant, not a precedent for goroutines elsewhere.
New `App` methods enable/disable the server, report connection status,
set the permission level, and copy a connection snippet for agent
harnesses (Goose, Claude, etc.); a settings panel or status indicator is
a later, secondary increment.

Acceptance criterion: an integration test drives the MCP server as a real
client would — `tools/list`, then `get_critical_path` and `update_task`
against a fixture project — and asserts the resulting `audit_events` row
is identical in shape to the same mutation performed through
`App.UpdateTask`; a second test asserts any tool call made before a
project is explicitly opened through the plugin session is rejected
before it reaches `HostAPI`. Both gates must pass before the feature
ships.

A general third-party plugin loader (a WASM sandbox via
`github.com/tetratelabs/wazero`, pure Go, zero-CGo, Apache-2.0) remains
Phase 4 — Ecosystem material for community extensions, worth taking on
only once this built-in MCP plugin has proven the `HostAPI` boundary in
practice.

**Portfolio Roll-ups** (RICE 90)
Aggregate EV/AC/PV/SPI/CPI across the projects the user has open or has
recently opened. Roll-up dashboard in `frontend/src/lib/components/project/Portfolio.svelte`.

_Complete 2026-07-25:_ `RunPortfolioAnalytics` reads each available project's
authoritative current schedule, computes EVM in the Go kernel at one UTC status
date, and sends exact minor-unit EV/PV/AC rows to the in-memory DuckDB
aggregator. Portfolio SPI and CPI are weighted ratios (`ΣEV/ΣPV` and
`ΣEV/ΣAC`), never averages of project ratios. Committed estimates remain
separate from actual cost, and the dashboard reports included/excluded project
counts so missing dates, malformed or cyclic schedules, and absent cost data
cannot silently appear as zero performance.

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
note below), Nostr Presence, Signaling & Team Messaging (see note below),
and a Mobile Companion web view.

**Local Collaboration note:** GoPMgr must acquire an exclusive SQLite lock
before opening a project and must release it on close. The sync folder must
not be transferred by Syncthing while GoPMgr holds the lock. The
recommended user workflow is: close the project in GoPMgr, allow Syncthing
to sync, reopen on the other machine. GoPMgr will detect and warn on
concurrent-open conflicts detected via the WAL/SHM presence heuristic.
Merge conflict resolution of concurrent edits is out of scope for now; the
model is "one writer at a time." CRDTs or operational transforms are a
possible later direction *if and when* GoPMgr deliberately decides to
support concurrent editing — no such decision has been made, and
one-writer-at-a-time remains the model until it is.

**Nostr Presence, Signaling & Team Messaging note:** Nostr (NIP-01) is a
complementary comms layer, not a sync mechanism — GoPMgr never transmits
project files, the SQLite database, or bulk project content over Nostr
events; the sync mechanism stays whichever of Local Collaboration (above)
or the Phase 4 self-hosted sync server (below) the user has configured.
Scope is deliberately narrow:
- **Presence/discovery** — which authorized teammates' GoPMgr instances
  are currently online.
- **Lightweight signaling** — e.g. "project X is closed, safe to pull"
  hints that reduce the WAL/SHM concurrent-open collisions the Local
  Collaboration note above already warns about.
- **Optional identity layer** — a user's `npub` is an alternative or
  complementary identifier to their local GoPMgr account, never a
  replacement for Argon2id authentication.
- **Short, in-app team messaging and public/semi-public project status
  notes** — both scoped to explicitly authorized team members only, never
  broadcast to the open Nostr network by default.

This is not real-time collaborative editing: it carries no task data, and,
per `VISION.md`'s "Out of scope (for now)" list, real-time collaborative
editing over a network stays out of scope regardless. Per the local-first
design principle, Nostr connectivity is strictly optional and
user/team-initiated, never required for scheduling, EVM, document
generation, or export. Relay configuration is user- or team-controlled — a
private/self-hosted relay is supported for teams that want to keep
presence and messages off public relays entirely, and public relays remain
an option for teams that accept that tradeoff; GoPMgr never hard-codes a
single relay dependency. Team membership and authorization (which `npub`s
may see presence, receive signals, or send messages for a given
project/team) are modeled explicitly, and relay traffic addressed to a
project from an unauthorized key must be rejected, not merely ignored
client-side. Requires a maintained Go Nostr client library (candidate:
`github.com/nbd-wtf/go-nostr`, MIT) — subject to the Dependency Policy's
evaluation (maintenance status, licence, CGO) before adoption, and its own
ADR given the new external-service and identity-key surface it
introduces. Persistent relay connections are, like the Phase 1 MCP
server's accept/read loop, a deliberate, explicitly isolated exception to
GoPMgr's current zero-goroutines-in-own-code invariant (see
`DEVELOPER_HANDBOOK.md`).

## Phase 4 — Ecosystem (2028+)

Self-hosted local sync server option (for teams that want shared access
without Syncthing — a small Go service that mediates exclusive access and
transfers project files only when no writer holds the lock, over a LAN or
a user's own machine; deliberately not built on Nostr's public-relay/
event-chunking model, though it may optionally consume the Phase 3 Nostr
signaling layer's lock-release hints as a convenience, never a
dependency), a general third-party plugin loader via
`github.com/tetratelabs/wazero` (pure Go, zero-CGo WASM sandbox,
Apache-2.0) for community extensions, Primavera XER/PMXML import,
Certification packs (ISO 21502, PMBOK 7), and open data standard exports.
The third-party loader is deliberately sequenced after Phase 1's Agent
Plugin Interface + Built-in MCP Server, which lands the `Plugin`/`HostAPI`
boundary first for a single trusted, in-process built-in plugin — the WASM
sandbox is only worth building once that boundary has been proven in
practice and untrusted third-party code needs isolating from it.

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
GitHub security advisory feed will not surface CVEs against GoPMgr's
dependency tree in the repository UI.

**Post-rename cleanup (August 2026 PMForge → GoPMgr rename)** — a few
items outside git's reach:
- Launchpad-hosted `launchpad` and `launchpad-project` remotes still
  point at the pre-rename Launchpad repos; renaming them requires
  Launchpad-side action, not just a local `git remote set-url`.
- `origin/testing` and `origin/session/docs-review-and-resource-leveling`
  still carry the `pmforge` Go module path — rebase or reapply the
  rename before merging either into `main`.
- `GOPMGR_UPDATE_PUBLIC_KEY` / `GOPMGR_UPDATE_PRIVATE_KEY` need to
  exist as real GitHub Actions secrets before release signing works;
  no `PMFORGE_*`-named secrets existed to migrate, so this is net-new
  setup, not a rename.
- Existing macOS installs will see a one-time TCC (privacy/permissions)
  re-prompt after upgrading, since `CFBundleIdentifier` changed from
  `dev.pmforge.PMForge` to `dev.gopmgr.GoPMgr`. Expected, not a bug.

## Next Recommended Audit

**Frontend↔backend path-string consistency — done (2026-08-04).**
Confirmed first that no Wails binding exposes `users.DefaultRootDir()`'s
resolved value to the frontend (`main.go`'s `App` type binds no
root/log-path getter — `Greet`, `SetUnsavedChanges`, and lifecycle
hooks only), so the "render the real value instead of a second copy"
fix wasn't available; pinned the strings instead.
`frontend/src/lib/persistence-boundary-strings.test.ts` now asserts
`HelpGuide.svelte` contains the exact macOS
(`~/Library/Application Support/PMForge/`) and Linux/Windows
(`~/Documents/PMForge/`) data-root strings, and that `HelpGuide.svelte`,
`Dashboard.svelte`, and the `ProjectLaunchpad.test.ts` fixture all
contain the `.pmforge` extension — each assertion verified to fail
under a deliberate rename of the string it pins. Also found and closed
an adjacent gap while in this code: the fourth frozen literal,
`PMForge_Archive_` (`internal/admin/workflow.go`), had no positive test
at all — the only existing reference to it,
`TestSecureArchiveRemovesArchiveWhenCreatedAuditLogFails`, globs for it
but only asserts *zero* matches on a failure path, so it would keep
passing even if the prefix were renamed. Added
`TestSecureArchiveUsesPMForgeArchivePrefix` (`internal/admin/admin_test.go`),
which exercises the success path and asserts the literal against a real
archive filename; also break-verified. All four frozen literals from
`DEVELOPER_HANDBOOK.md` §9 (first 2026-08-04 entry) were, at the time
this paragraph was written, positively pinned somewhere for the first
time. That "frozen" framing was superseded hours later the same day —
see the next entry below, which is current.

**PMForge → GoPMgr persistence-literal rename — done (2026-08-04).**
The four literals the entry above pinned as permanently frozen were
renamed after all, once a same-day follow-up conversation confirmed the
scope per literal (this paragraph replaces an earlier "declined pending
scoping" note — see `docs/security-quality-review-2026-08-04.md` for
the full decomposition and verification writeup):

- **Data-root directory** `PMForge` → `GoPMgr`
  (`internal/users/store.go`'s `DefaultRootDir`). `MigrateLegacyRoot`
  now checks up to two legacy locations in precedence order (current
  pre-rename default first, then the older pre-2026-06 Documents
  location on macOS) and copies whichever has data into the new root,
  non-destructively, exactly as it already did for the 2026-06
  relocation.
- **Project-file extension** `.pmforge` → `.gopmgr`
  (`app_projects.go`). New projects only ever get `.gopmgr`; opening,
  deleting, cloning, and listing projects (`projectPathFor`,
  `enumerateProjects`) accept **both** extensions permanently — nothing
  migrates an existing project's extension, since `.pmforge` files can
  live anywhere a user put them (Downloads, external drives, their own
  scripts), not just under the data root.
- **Backup archive entry** `project.pmforge` → `project.gopmgr`
  (`internal/db/backup.go`), keyed off a new `schema_version` 2 (was 1).
  `RestoreArchivalBundle` looks up the project entry's name by the
  archive's declared schema version rather than sniffing whichever name
  is present, so a tampered archive can't swap in the wrong entry under
  the wrong version's name. `.pmba` itself was **not** renamed — see
  the review doc for why extending the dual-read list to a fifth entry
  wasn't warranted.
- **Backup filename prefix** `PMForge_Archive_` → `GoPMgr_Archive_`
  (`internal/admin/workflow.go`). Pure rename, no compatibility branch:
  nothing reads this prefix back.

Every changed or newly-added assertion (frontend and backend) was
break-verified the same way the tests above were. Frontend copy
(`HelpGuide.svelte`) now documents the new data-root path and extension
while still naming `.pmforge` as still-openable, and tells upgrading
users their old data folder was copied forward automatically.

**Update (2026-08-04, later the same day):** an adversarial self-review
pass fixed a real bug this section's e2e suggestion would have caught —
`users.Account.DataDir` was trusted from a `data_dir` column written once
at account-creation time, so a migrated account kept pointing at the
deleted old PMForge root instead of the copied data. See the
DEVELOPER_HANDBOOK.md entry "Migrated accounts silently kept
reading/writing the deleted old root." `migration_e2e_test.go`'s
`TestPreRenameInstallIsUsableAfterMigration` now builds a real pre-rename
install through the App/users public API (not a synthetic tree-copy
fixture), migrates it, deletes the old root, and proves login + project
listing still work. It does not point at a literal pre-existing disk
install (`~/Library/Application Support/PMForge`) or restore a `.pmba`
backup produced by a pre-2026-08-04 release — those two checks are still
worth naming rather than starting now.

Frontend coverage was measured, not just counted: `vitest run --coverage
--coverage.all --coverage.include='src/**/*.{ts,svelte}'` (script:
`npm --prefix frontend run test:coverage`) puts true repo-wide statement
coverage at 14.36% (1895/13195) before this pass, not the 61% the default
(imported-files-only) report showed. Most of the gap is presentational
`.svelte` components, which need component-mount tests, not plain unit
tests — a materially different and larger effort than one tranche. Added
two previously-0% pure-logic modules (`terminology.ts`,
`charts/_flow_shapes.ts`) with break-verified tests, 61 → 81 vitest tests,
coverage to 14.69%. `make frontend-stability` (svelte-check + vitest) is
still the entire enforced frontend gate; `test:coverage` is a manual
measurement command, not wired into any gate, and no coverage threshold
is set. See the DEVELOPER_HANDBOOK.md entry "Frontend coverage: measured,
not just counted" for what's still uncovered and the priority order to
pick it back up by (regression cost, not file count).

**Persistence-boundary invariant tests — done (2026-08-04).** The
August 2026 PMForge → GoPMgr rename relied on a rule that previously
lived only in prose comments and one README-text assertion in
`scripts/release-gate-scope-check.sh`. Table-driven tests now pin all
four frozen literals (`internal/users/root_dir_test.go`,
`project_path_confinement_test.go`, `internal/db/backup_test.go`), each
verified to fail under a deliberate break of the literal it pins,
including a follow-up fix once it was noticed the first version of the
`DefaultRootDir` full-path assertion only ran on darwin — CI runs on
Linux, so that gap would have shipped green. See
`DEVELOPER_HANDBOOK.md` §9 (2026-08-04 entry) for the full rationale
and which test covers which literal.

**Update-path audit for existing installs — blocked, not yet
exercisable.** The rename changed `CFBundleIdentifier`, packaged
artifact names, and the update signing key names. This session
verified that a renamed GoPMgr binary can still open a pre-existing
`~/Library/Application Support/PMForge` install's encrypted `.pmforge`
project file untouched. Separately, `internal/update.ManifestURL` and
`UpdateChannelPublicKey` both default to empty and are only wired via
`-ldflags` in `release.yml` when `GOPMGR_UPDATE_PUBLIC_KEY` is set —
and per Manual Action Required above, that secret does not exist yet.
So no shipped GoPMgr (or PMForge) release has ever had automatic
update-checking active, and there is no "installed pre-rename build
polling a now-broken endpoint" scenario to worry about today. Once
`GOPMGR_UPDATE_PUBLIC_KEY`/`GOPMGR_UPDATE_PRIVATE_KEY` are created and
the channel is first enabled, verify end-to-end before relying on it:
confirm `internal/update.CheckLatest` can fetch and verify a real
signed manifest and that the packaged artifact names it points to
match what `release.yml` actually produces.

## In Progress: 100% Full-Repo Test Coverage

**Started 2026-08-04.** Goal, scope, exclusions, and enforcement mechanism
are fixed and documented in DEVELOPER_HANDBOOK.md's "100% coverage: scope
and exclusions" entry — read that before touching this effort, it defines
what "100%" means (both Go build configs independently, frontend with real
interaction assertions, a named exclusion set) so the target doesn't drift.

**Phase 0 (foundations) — done.** `scripts/coverage-go.sh`,
`scripts/coverage-frontend.sh`, `scripts/coverage-ratchet.sh`, and
`coverage-baseline.json` (checked in). `make coverage-ratchet` fails only
on a regression below the last recorded high-water mark, not on being
under 100% — a hard floor would block all unrelated work for this effort's
full multi-session duration. **Deliberately not wired into `make verify`
yet**: CI's `verify` job installs no DuckDB CGO toolchain, so a
duckdb-tagged coverage run would hard-fail CI today; wire it in once CI
gets that toolchain, or in Phase 7 when this becomes a hard floor.
Baseline: go_default 8525/14611 (58.35%), go_duckdb 8603/14709 (58.49%),
frontend 1939/13195 (14.69%), recorded on macOS with a platform-symmetric
exclusion list so it reproduces identically on CI's ubuntu-24.04 runner.

**Phase 1 (cheapest Go wins) — in progress.** `internal/cli`: done, 0% →
100% under both build configs. `ParseFlags()` had zero tests despite
existing `Config`/`PrintVersion` tests in the same file; added 7 tests
covering every one of its 18 flags plus positional-arg handling, using
the standard `flag.CommandLine`-reset technique (no production code
changed). "21 statements / 7 tests" is not a meaningful cost figure to
carry into the Phase 6 frontend extrapolation: `ParseFlags` is 21
statements of flag registration effectively covered by a single call, so
the ratio reflects how assertions were split across test functions, not
real per-test cost — the effort here was near-trivial (a table entry
against a pure function), which is the actual reason this was cheap.
One data point, one function; doesn't move the frontend estimate.
Break-verified two independent mutations (a flag default, the `NArg()`
positional-arg guard), both caught.

`internal/templates`: done, 20.7% → 97.3% under both build configs. Not
100% — see the DEVELOPER_HANDBOOK.md "First wall: the exclusion mechanism
is file-granular" entry for why, and why that's being left as a recorded
gap rather than forced or worked around on a sample of one. `seeds.go`
(0% → 100%): built `db.InitDB`-backed tests exercising every seed kind
(7 charts, 8 documents, kanban/backlog/sprint1, the unknown-seed no-op),
plus 4 SQLite-trigger fault-injection tests (same technique as
`internal/users`'s `TestAuthenticateReturnsLastLoginUpdateError`) pinning
each seed handler's error-propagation path, plus a direct call to the
unexported `seedDocument` helper with an invalid kind (a different
technique — no DB fault injection needed, since the kind string isn't
type-constrained to the 8 real values), plus `Apply`'s
partial-success-on-failure contract — the one genuinely non-obvious
behavior in the file, previously unpinned.
`jdm.go` (83.3%/66.7% → 83.3%/90.9%): deleted 3 branches proven dead by
type (`SeedRequest` marshal, its map unmarshal, and `json.RawMessage`
marshal all cannot fail — see handbook), and replaced the marshal/
unmarshal round-trip building the zen input map with a direct map
literal, removing the round-trip's own uncovered lines along with it.
Part of `Evaluate`'s percentage gain is from tests (the nil-`*Engine`
guard, `e.z.Evaluate`'s error path via the same error-returning-loader
fixture zen-go's own test suite uses, and a known-decision-table-row test
proving the map's `"industry"`/`"methodology"` keys are spelled right --
added after adversarial review found `TestEngineEvaluatesFallback` alone
would stay green even if both keys were misspelled, since an unknown pair
and a misspelled-key lookup hit the same fallback row) and part is simply
from a smaller denominator after deletion — both are named so a reader
doesn't credit the whole gain to new tests. Left untested and undeleted:
the loader-miss branch (defensive code for the package doc's stated
future of sibling JDM files, not reachable via any node type in zen-go's
own fixtures — searched, didn't force it) and the final `SeedResponse`
unmarshal error (reachable only via a malformed-shape JDM response,
judged not worth hand-crafting).

Also fixed during this pass: `scripts/coverage-ratchet.sh` compared
statement-coverage *percentage*, which has a false-positive trap this
pass hit directly — deleting `jdm.go`'s 3 dead-by-type branches removed
statements that were already 100% covered, which mathematically lowers
the repo-wide percentage (removing above-average-coverage code from a
below-100%-average repo always does). The ratchet now compares
*uncovered statement count* instead: deleting fully-covered code leaves
it unchanged, only adding untested code or removing tests increases it.
Break-verified with the synthetic-inflated-baseline case used for the
original ratchet, plus confirming this session's real deletion now holds
cleanly with no override needed.

Noted, not changed: `Evaluate(ctx context.Context, ...)`'s `ctx` parameter
is never read in the function body. Left as-is rather than removed —
dropping it changes an exported package signature, a separate decision
from coverage work, not something to fold into this pass.

**Phase 1 — complete (2026-08-05).** The last remaining item, refactoring
the 6 `runtime.GOOS ==` conditionals in `main.go`/`internal/users/store.go`
behind an injectable seam, is done. `internal/users/store.go`'s two
conditionals (`DefaultRootDir`, `legacyRootCandidates`) were split into
pure `defaultRootDirForGOOS(goos, home string)` /
`legacyRootCandidatesForGOOS(goos, home string)` helpers that the public,
runtime.GOOS-reading functions delegate to; tests call the pure helpers
directly with both `"darwin"` and a non-darwin value in one run.
`main.go`'s `buildAppMenu` gained a `goos string` parameter (its one real
call site in `buildAppOptions` passes `runtime.GOOS`); new tests build the
menu with `"darwin"` and with `"windows"`/`"linux"` and assert on the
returned `*menu.Menu`'s structure (role menus vs. hand-built Window
submenu, where Quit lives). `internal/users/store.go` was the file
`f1a5501`'s migration data-loss fix landed in — every existing test in
that package (including `TestPreRenameInstallIsUsableAfterMigration`)
passes unchanged, confirmed before and after.

Also closed: the `os.UserHomeDir()` error branch shared by
`DefaultRootDir`/`legacyRootCandidates` (unrelated to GOOS, but adjacent
and cheap — `t.Setenv("HOME", "")` triggers it hermetically on POSIX).

Break-verified 4 independent mutations across this change (a resolved
path on each pure helper, the darwin/non-darwin File-menu Quit
conditional) — all caught, confirmed by the existing
`runtime.GOOS`-dependent tests staying GREEN under the very mutation
the new pure-helper tests caught, demonstrating exactly why the seam
was needed: this machine is macOS, so the old tests could never have
caught a non-darwin regression.

`internal/users` coverage: `DefaultRootDir`/`legacyRootCandidates`/
`defaultRootDirForGOOS`/`legacyRootCandidatesForGOOS` all now 100%.
`main.go`'s `buildAppMenu` 57.9% → 71.1% — the remaining gap is menu
*callback bodies* (the closures that fire `wailsruntime.EventsEmit`,
`Quit`, `WindowToggleMaximise`, etc.), which need a live Wails runtime
context to exercise meaningfully and are explicitly left for Phase 4
(root/App layer), not folded into this seam-focused change.

Also fixed, two pre-existing linter hints on files this change already
touched (`main.go`'s `Bind: []any{app}` instead of `[]interface{}{app}`;
`internal/users/store.go`'s `migrateAdminColumn` now uses
`slices.Contains` instead of a manual loop) — both mechanical,
behavior-identical simplifications, confirmed via `go build`/`go vet`/the
full test suite passing unchanged before and after.

**Phase 2 (Go completion tier) — in progress.** `a0360b5` was
adversarially reviewed (via `/advisor`, recovered from a session-long
outage) before this phase began, per standing process; two non-blocking
findings from that review (a `TEST_COVERAGE_LEDGER.md` row overstating
what one test proves, and no drift enforcement on the ledger) were fixed
in this pass alongside the code — see `DEVELOPER_HANDBOOK.md`'s matching
dated entry for both.

`internal/applog`: 72.5% → 100% of `applog.go` (95.7% of the package;
`dialog_darwin.go`/`opendir_darwin.go` stay excluded as platform-narrow).
`Init`'s three "never fail outright" fallback branches (unresolvable log
dir, `MkdirAll` failure, `OpenFile` failure) and `Fatal`'s dialog/exit
sequence were the entire gap. Closed via two techniques: real-but-
controlled filesystem obstacles (a blocker file/directory placed exactly
where `Init` needs to create its own — chosen over permission bits, which
behave inconsistently when tests run as root in CI containers) for the
`MkdirAll`/`OpenFile` branches, and the same injectable-seam pattern used
for `runtime.GOOS` in Phase 1 (`userHomeDir`/`tempDir`/`showError`/
`osExit` as package-level func vars) for the fully-unresolvable-dir
branch and for `Fatal` — the latter needed it regardless of filesystem
tricks, since it otherwise pops a real native OS dialog and calls
`os.Exit`. Break-verified 5 distinct mutations (each of `Init`'s 3
fallback branches, `Fatal`'s `showError` call, `resolveLogDir`'s temp-
fallback branch) — all eventually caught, but 2 of the 5 (`Init`'s
`MkdirAll`-fails and its `logDir == ""` branches) were **not** caught on
the first attempt: the induced failure cascaded into a *different*
fallback branch (`OpenFile`, or `MkdirAll("")`) and produced the same
`logPath == ""` observable, which the test's original assertion couldn't
tell apart from the branch actually under test. Both were fixed by
asserting the specific log message each branch emits, via a real
`os.Pipe()`-based stderr capture (`Init`'s failure branches call
`log.SetOutput(os.Stderr)` themselves, which silently discards a
`log.SetOutput(&buf)` done from the test side). One test remains
disclosed as non-discriminating rather than fixed: `pruneOldLogs`'s
`os.ReadDir`-error branch's entire contract is "return early, touch
nothing," and deleting that early return still produces a clean,
silent no-op (`range` over the resulting nil slice), so no assertion
can distinguish the branch running from the branch being deleted — see
`DEVELOPER_HANDBOOK.md`'s matching entry.

Baseline updated: go_default 8612/14605 (5993 uncovered) → 8628/14605
(5977 uncovered); go_duckdb 8690/14703 (6013 uncovered) → 8706/14703
(5997 uncovered).

**`internal/crypto`: 72.9% → 85.7%. Found and fixed a real production bug while covering `LoadCertificate`, not just closed a coverage gap.**
`LoadCertificate` called `pkcs12.Decode`, which hard-requires exactly one
private-key bag and one certificate bag and errors on anything else — its
own doc comment says "if there are more use ToPEM instead." Any
commercially-issued signing certificate exported with its issuing chain
bundled in (the normal, expected shape for a real cert, not an edge
case) made `LoadCertificate` fail outright with "expected exactly two
safe bags in the PFX PDU" instead of loading and populating
`Signer.ExtraCerts`. Confirmed live and user-facing before fixing:
`LoadCertificate` is called from `internal/documents/charter.go` (document
signing) and `internal/export/pdf.go` (PDF export), wired through
`app_documents.go` — not dead code reached only by tests.

Fixed by switching to `pkcs12.ToPEM` (no 2-bag limit) and adding
`parseP12Blocks`/`splitLeafCertificate` to do the classification `Decode`
used to do internally: collect every PRIVATE KEY/CERTIFICATE block, then
identify the signer's own certificate by matching its public key against
the loaded private key (not the P12's optional `localKeyId` attribute,
which some exporters omit — a metadata-based match could silently put an
intermediate in `Cert` and the real leaf in `ExtraCerts`, producing a CMS
signature real-world validators would reject). Reproduced first as a
failing test against a hand-built 3-bag P12 fixture (`testdata/testonly-
rsa-3bag.p12`) before writing the fix, so the red test doubled as the bug
report and the fix's own break-verification — no synthetic mutation
needed, the mutation was the pre-existing code. Regression-tested against
the 2-bag shape every previously-working certificate has
(`testdata/testonly-rsa-2bag.p12`) to confirm nothing that worked before
stopped working. Test fixtures generated with `openssl -legacy`
specifically (see `testdata/README.md`): `golang.org/x/crypto/pkcs12` is
a frozen, decode-only package that can't read OpenSSL 3.x's default
AES-256-CBC/PBKDF2 P12 encryption, only the older schemes `-legacy`
selects.

Also closed: `SignPDFHash` (0% → 100%, no fixture needed — reuses the
in-memory key/cert generator already in the test file, since the
function only needs a `*rsa.PrivateKey`). `LoadCertificate` itself sits
at 92.3%, not 100%: the one remaining line is `LoadCertificate`'s
propagation of `splitLeafCertificate`'s "no certificate matches this
key" error, which real `.p12` tooling won't produce — OpenSSL refuses to
export a P12 whose bundled certificate doesn't match the given key
("No cert in -in file matches private key"), and the underlying logic is
already proven correct by a direct unit test on `splitLeafCertificate`
itself. Left undocumented-and-forced would be worse than left disclosed;
recorded here per the `internal/templates` "first wall" precedent rather
than hand-crafting a mismatched P12 to force one line.

Break-verified 3 targeted mutations against the fix's actual logic (the
leaf/extra-cert public-key match condition, the non-RSA rejection, the
multiple-private-key rejection) — all caught, on top of the red-test-
first reproduction above.

Baseline updated: go_default 8628/14605 (5977 uncovered) → 8674/14630
(5956 uncovered); go_duckdb 8706/14703 (5997 uncovered) → 8752/14728
(5976 uncovered).

**`internal/crypto` follow-up complete: 85.7% → 91.7%.** Closed the
scattered 71–92% gaps across `encrypt.go`, `keywrap.go`, `pdf_cms.go`,
and `pdf_cms_timestamp.go` deliberately held out of the `LoadCertificate`
bug-fix commit above. Reviewed GO-2026-5932
(`golang.org/x/crypto/openpgp`, unmaintained/unsafe-by-design, no fixed
version) first, per the user's mid-session flag — confirmed via
`govulncheck`'s call-graph analysis (not just a version check) that this
repository never imports `openpgp` (only `argon2` and `pkcs12`), so
`govulncheck ./...` already exits 0 and no dependency or code change was
possible or needed; see `DEVELOPER_HANDBOOK.md`'s dated entry for the
full record and the forward-looking rule if OpenPGP is ever needed here.

Two bugs surfaced and fixed in the test tooling itself, both found by
break-verifying rather than trusting a passing first draft: a fake
`rand.Reader` that delegated through the very global it had replaced
(infinite recursion, crashed a test run with a real stack overflow —
not merely a wrong assertion), and the same cascading-failure trap
`applog`'s Phase 2 entry already named, recurring in a new package
(a "fails after N successful calls, forever" reader design let a
deleted salt-error-check pass by tripping the nonce check instead).
Both fixed; see the handbook for the exact designs.

`internal/crypto` is now at 91.7%, not 100% — every remaining line is
one of: a value-relationship guard kept deliberately (not deleted, since
the "can't fail" guarantee depends on an editable constant, not a Go
type — a sharper version of the `internal/templates` dead-code-deletion
rule), a process-global FIPS-140 branch with no per-test override, or an
`asn1.Marshal`/`rsa.SignPKCS1v15` error-propagation line whose underlying
failure is proven real by a direct test on the helper function but not
reachable through this package's own well-formed production call sites.
All individually named in the handbook, none left as an unexplained
percentage. Coverage ratchet updated: go_default 8674/14630 (5956
uncovered) → 8687/14630 (5943 uncovered); go_duckdb 8752/14728 (5976
uncovered) → 8765/14728 (5963 uncovered).

**`internal/crypto` complete for Phase 2 purposes.** No package-specific
work remains queued for this package short of a hard-100% push (Phase 7).

**`internal/admin`: 76.1% → 97.7%.** Closed all four gaps identified in
`workflow.go`'s `SecureArchive`, `LogDocumentSignatureOutcome`, and
`logSignatureCheckpointWithStatus`. `SecureArchive`'s settings-load
failure was forced by closing the `*db.Database` before calling it (a
throwaway probe confirmed `Close` is idempotent and any subsequent
method returns `sql: database is closed` — a simpler alternative to
the SQLite-trigger technique for DB-unavailable branches, used
alongside it here). Its cert-bundling failure and the
`settings.CertPath != ""` branch were covered together by one test
(pointing `CertPath` at a directory, same technique as
`internal/db/backup_test.go`'s existing bundle-failure test).
`LogDocumentSignatureOutcome`'s two empty-string defaulting branches
were covered directly. `logSignatureCheckpointWithStatus`'s dead
`json.Marshal` error check was deleted, not tested — the struct being
marshaled has three plain `string` fields, and `json.Marshal` cannot
fail on a plain-string struct (invalid UTF-8 is replaced, not
rejected), the same class of guarantee as the `internal/templates`
`jdm.go` deletions but sharper: safe only as long as every field
stays a `string`, noted in the code so a future `any`/map-typed field
doesn't silently reopen a real error path.

One break-verification catch during this pass: the first draft of the
defaulting test asserted against `audit_events.signature_status`, but
`internal/db/audit.go`'s `appendAuditEventTx` already re-defaults
`SignatureStatus == ""` to `"unsigned"` downstream — so deleting
`workflow.go`'s own default didn't turn that assertion red. Fixed by
asserting against `audit_log.details` instead (`LogAction`'s
plain-text trail, written from this package's local variables with no
downstream re-defaulting) — the only place this package's own
defaulting is actually observable. See `DEVELOPER_HANDBOOK.md`'s
matching dated entry.

One line is knowingly left uncovered and disclosed rather than forced:
`SecureArchive`'s `os.Remove` double-fault branch (removing an
already-unaudited archive itself fails). Unlink permission is governed
by the parent directory, and making that directory non-writable would
also block `CreateArchivalBundle` from creating the archive in the
first place — so no single directory permission reaches this line
without also short-circuiting the test before it gets here. Documented
in-place in `workflow.go` and in the ledger.

Coverage ratchet updated: go_default 8687/14630 (5943 uncovered) →
8694/14627 (5933 uncovered); go_duckdb 8765/14728 (5963 uncovered) →
8772/14725 (5953 uncovered).

**`internal/users` (`dek.go` increment): 73.2% → 90.2% of `dek.go`
(73.5% → 75.6% of the package).** `internal/users` is the largest
completion-tier package (`dek.go` + `recovery.go` + `store.go`,
~30 total forceable branches across all three), so per the
`internal/crypto` split precedent this is deliberately the first of
several increments, scoped to `dek.go` alone —
`migrateDEKColumns`/`UnlockDEK`/`HasLegacyRecoveryCodeWraps`, the
package's security-critical DEK-unwrap logic. `recovery.go` and
`store.go` remain for follow-up increments.

Three forcing techniques, reused from `internal/admin`'s pattern
where they applied: the closed-DB technique (`migrateDEKColumns`'s
`PRAGMA table_info` query failure), a SQLite trigger blocking
`UPDATE OF wrapped_dek_pw ON users` (matching `store_test.go`'s
existing `block_last_login`/`block_password_rehash` triggers), and
one new technique validated by direct probe before use: dropping the
`users` table after `Open()` succeeds, then calling
`migrateDEKColumns()` directly — the column-presence probe correctly
reports the column absent (zero rows from a dropped table), so the
function proceeds to `ALTER TABLE users ...`, which then fails with
"no such table" — forcing the ALTER branch specifically, distinct
from the earlier PRAGMA-query-failure branch. `UnlockDEK`'s
`WrapKey` error branch was forced with an empty password rather than
a broken entropy source: `EncryptBuffer` rejects an empty password
before ever touching `rand.Reader`, so no fake reader was needed —
but this requires a *fresh* account (`wrapped_dek_pw == ""`), since
any account with a real DEK already routes to `UnwrapKey` instead,
which never calls `WrapKey`.

One masked mutation, caught by break-verifying before commit, not
after: the first draft of `TestUnlockDEK_GenerateDEKEntropyFailure`
asserted only `err != nil`. Deleting `UnlockDEK`'s
`GenerateDEK`-error check still produced a non-nil error — `WrapKey`
independently rejects the resulting nil/wrong-length key with
`ErrBadDEK` — so the test stayed green under a deleted guard. Fixed
by asserting the specific "generate DEK" error text instead; see
`DEVELOPER_HANDBOOK.md`'s matching entry. Two branches
(`migrateDEKColumns`'s `rows.Scan` and `rows.Err()` on the `PRAGMA
table_info` cursor) stay disclosed-untested: SQLite's fixed six-
column pragma output makes `Scan` unable to fail under normal
operation, and no portable way exists to inject a mid-iteration
cursor I/O error.

Also fixed in passing: `dek.go`'s pre-existing `interface{}` →
`any` linter hint in the function this change already touched
(mechanical, behavior-identical).

Coverage ratchet updated: go_default 8694/14627 (5933 uncovered) →
8702/14627 (5925 uncovered); go_duckdb 8772/14725 (5953 uncovered) →
8780/14725 (5945 uncovered).

**`internal/users` (`recovery.go` increment 1 of 2): 70.5% → 83.0% of
`recovery.go` (75.6% → 79.4% of the package).** Split along the
function boundary per `/advisor`'s plan review, one level finer than
the `dek.go`/`recovery.go`/`store.go` file-level split: this increment
covers `migrateRecoveryTable`, `RemainingRecoveryCodes`, and
`IssueRecoveryCodes` (9 forceable branches); `ResetWithRecoveryCode`'s
twelve remain for increment 2. Reason for the finer split: a
call-indexed `rand.Reader` fake gates one test in each function at a
different call index, and its correctness rests on call counts
derived by reading source — validate it on the simpler function
first, with the index confirmed by a throwaway measuring probe rather
than trusted from the read, before reusing it in the more complex one.

Every forcing technique was probed directly before the real test was
written, continuing this session's standing discipline: `s.conn.Begin()`
on a closed store returns `sql: database is closed` (confirmed before
building the `ResetWithRecoveryCode` `tx.Begin` test planned for
increment 2); `DROP TABLE recovery_codes` cleanly yields `no such
table` from a subsequent `tx.Query` (confirmed for the same
increment); and a throwaway counting `rand.Reader` wrapper measured
`IssueRecoveryCodes`' exact per-iteration call sequence
(`generateCode` call 1 → `HashPassword` call 2 → `WrapKey` calls 3–4)
before the call-indexed fake was built around it.

One SQLite behavior found the hard way, not assumed: a `BEFORE
DELETE` trigger never fires for a `DELETE` that matches zero rows, so
`TestIssueRecoveryCodes_DeleteFailsOnBlockedTrigger`'s first draft
(against a freshly-created, code-less account) silently passed through
the trigger untouched and failed for the wrong reason. Fixed by
issuing codes once before installing the trigger, so the second
`IssueRecoveryCodes` call has a real row to delete.

`RemainingRecoveryCodes`'s unreachable `sql.ErrNoRows` branch and
`generateCode`'s `len(enc) < 16` branch were documented in-place as
kept-not-deleted this pass (both dead only given an adjacent editable
expression — a bare `COUNT(*)` and the `rawCodeBytes` constant,
respectively — not dead by type), resolving the miscategorization
`/advisor` flagged during the `dek.go` increment's planning.

Two branches in `IssueRecoveryCodes` stay disclosed-untested:
`tx.Begin()` is preceded by the user-existence `QueryRow`, so a
closed-DB fault trips that earlier check first and can't isolate
`Begin()` on its own; `tx.Commit()` has no portable forcing method in
this package (no deferred-constraint or disk-level fault injection
available).

`internal/users`' full suite runtime grew from ~3.3s to ~4.5s across
the `dek.go` and this increment combined (each `IssueRecoveryCodes`
call runs 8 real Argon2id hashes at 64 MiB) — noted per `/advisor`'s
flag, not yet a problem, but worth watching as `ResetWithRecoveryCode`
and `store.go` add more.

Coverage ratchet updated: go_default 8702/14627 (5925 uncovered) →
8716/14627 (5911 uncovered); go_duckdb 8780/14725 (5945 uncovered) →
8794/14725 (5931 uncovered).

**`internal/users` (`recovery.go` increment 2 of 2): 83.0% → 92.9% of
`recovery.go` (79.4% → 82.3% of the package).** Closes
`ResetWithRecoveryCode`'s eleven forceable branches, reusing every
technique validated in increment 1: closed-DB for `tx.Begin()` (this
function's genuine first DB call, unlike `IssueRecoveryCodes`'), a
dropped `recovery_codes` table for the `SELECT` failure, the
call-indexed `rand.Reader` fake at the index the prior increment's
probe already confirmed (index 3, `HashPassword`'s salt read on the
non-legacy path), and distinct `RAISE(ABORT, ...)` messages per
trigger. New this increment: corrupting only the `wrapped_dek` column
(leaving `code_hash` untouched, so `VerifyPassword` still matches) to
force `UnwrapKey`'s "recovery wrap corrupt" branch — the most
security-critical test in this file, confirmed by direct probe before
writing it.

Two real findings from break-verification, both corrected before
commit, both the same species named in the `internal/admin` and
`dek.go` entries but with new specifics:

- `TestResetWithRecoveryCode_WrapKeyEntropyFailure`'s first draft
  asserted only `err != nil`. Deleting `WrapKey`'s guard doesn't make
  the function succeed — the very next call, `auth.HashPassword`,
  independently reads `rand.Reader` for its own salt and fails on the
  same broken reader, with a *different*, wrapped error text
  (`"auth: read salt: ..."` vs `WrapKey`'s bare propagation). Fixed by
  asserting the exact bare error string.
- `TestResetWithRecoveryCode_RejectsInvalidUsername` turned out not to
  be break-verifiable as a guard-presence test at all: deleting
  `ValidateUsername`'s check inside this function still produces the
  identical `ErrInvalidRecoveryCode`, because the malformed username
  simply matches zero rows in the following `SELECT` and falls through
  to the `matchID < 0` branch, which returns the same sentinel — by
  design, the two paths are meant to be indistinguishable
  (anti-enumeration). The test still pins a real behavioral guarantee;
  its docstring and the ledger now say so precisely instead of
  claiming it isolates the guard.

Four branches stay disclosed-untested: `ResetWithRecoveryCode`'s own
`rows.Scan`/`rows.Err()` (same PRAGMA-cursor-class reasoning as
`dek.go`'s disclosures — this table has exactly one writer in the
whole codebase) and `tx.Commit()`, alongside `IssueRecoveryCodes`'
`tx.Begin()`/`tx.Commit()` disclosed in increment 1.

`internal/users`' `dek.go` (90.2%) and `recovery.go` (92.9%) are
complete for Phase 2 purposes — every remaining uncovered line is a
named, disclosed reason, none left unexplained. `store.go` (75.5%,
untouched this pass) is the final increment for this package.

Coverage ratchet updated: go_default 8716/14627 (5911 uncovered) →
8727/14627 (5900 uncovered); go_duckdb 8794/14725 (5931 uncovered) →
8805/14725 (5920 uncovered).

**`internal/users` (`store.go` increment, sub-increment 1 of 2 —
DB-backed functions): 75.5% → 87.7% of `store.go` (82.3% → 89.5% of
the package).** `/advisor` recommended splitting `store.go` into two
sub-increments from the start rather than discovering the split
mid-implementation, since it mixes two forcing-technique families:
DB-backed functions (`Open`, `Close`, `RootDir`, `migrate`,
`migrateAdminColumn`, `SetAdmin`, `DeleteAccount`, `CreateAccount`,
`Authenticate`, `List` — all closed here) and filesystem-backed
functions (`MigrateLegacyRoot`/`migrateLegacyRoot`/`copyTree`/
`copyFile`/`ensurePrivateDir`/`ensurePrivateSQLiteFiles`, unstarted).
`CreateAccount`, `Authenticate`, and `List` reach 100%; the other seven
functions have documented, disclosed-untested remainders. Every forcing
technique was empirically probed (a throwaway probe file, deleted
after use) before being written into a real test, per this session's
established discipline — probing surfaced two techniques new to this
package: writing a non-integer string into the `is_admin` column
(`UPDATE users SET is_admin = 'not-an-int'`) exploits SQLite's type
affinity to force a `Scan` conversion failure without a closed
connection or a trigger, used for both `Authenticate`'s and `List`'s
row-scan branches; and `t.Setenv("XDG_DATA_HOME", ...)` makes
`MigrateLegacyRoot`'s otherwise-unexported `legacyRootCandidates()`
path fully test-controllable for a later sub-increment, since it reads
the env var fresh on every call rather than caching it.

Break-verification (mutate the guard, confirm the test goes red,
restore, diff against a backup to confirm a clean restore) surfaced two
real findings, both anticipated by `/advisor`'s plan review before any
code was written and confirmed empirically rather than assumed:

- `TestOpen_RootDirIsFile` (root pre-occupied by a file, forcing
  `ensurePrivateDir`'s `MkdirAll` to fail) asserts the specific
  "mkdir root" wrapper text, not a bare error check. Deleting `Open`'s
  guard doesn't make `Open` fail differently: `sql.Open` connects
  lazily and never touches the filesystem, so execution falls through
  to `s.migrate()`'s first `Exec`, which fails against the same broken
  path with an unrelated "unable to open database file" error — the
  same cascading-fallible-path shape as this session's other
  masked-mutation findings, this time caught by probing the mutation
  before writing the assertion instead of by a failed break-verify run.
- `TestAuthenticate_ValidateUsernameShortCircuitsBeforeDBAccess` is a
  second instance of `recovery.go`'s anti-enumeration-unification
  shape: on an open store, a present `ValidateUsername` guard and a
  deleted one both return the identical `ErrNoSuchUser` value (present:
  the guard itself; deleted: `sql.ErrNoRows` from the fallthrough
  SELECT, per `ErrNoSuchUser`'s own merge-in-UI doc comment), so no
  equality check on an open store can break-verify it — confirmed by
  probe before writing the test, not assumed. Unlike `recovery.go`'s
  version, this one IS break-verifiable: closing the store first makes
  the two paths diverge on *when* the DB gets touched. A present guard
  still returns `ErrNoSuchUser` without touching the closed connection;
  a deleted one reaches it and surfaces "database is closed" instead, a
  value `ErrNoSuchUser` can never equal. Confirmed empirically both
  ways: the mutation goes red under the closed-store test, and a
  separate probe confirmed an open-store equality check alone would
  have stayed green under the same mutation.

Ten branches stay disclosed-untested in `store.go`, each with an
in-code comment at the point of the check: `Open`'s own `sql.Open`
error and both migrate/private-file close-error cascades (no portable
way to fail `conn.Close()` itself after an already-failed statement);
`migrate`'s `migrateRecoveryTable`/`migrateDEKColumns` propagation
checks (no hook between them and the prior successful `Exec` on the
same live connection, and SQLite triggers don't fire on the CREATE
TABLE/ALTER TABLE DDL those functions issue); `migrateAdminColumn`'s
PRAGMA-cursor `Scan`/`rows.Err()` (same class as `dek.go`'s
`migrateDEKColumns` disclosure — the six-column shape is fixed by
SQLite); `SetAdmin`'s and `DeleteAccount`'s admin-COUNT query errors
(same live-connection, no-DML-trigger-possible reasoning as `migrate`'s
propagation checks).

Coverage ratchet updated: go_default 8727/14627 (5900 uncovered) →
8754/14627 (5873 uncovered); go_duckdb 8805/14725 (5920 uncovered) →
8832/14725 (5893 uncovered).

A same-day follow-up correction closed one more branch:
`ensurePrivateSQLiteFiles`'s own "main path missing" `os.Chmod` failure,
directly forceable by pointing it at a path that was never created —
`TestEnsurePrivateSQLiteFiles_MainPathMissing` in `store_test.go`,
break-verified the same way as every other test this increment.
`ensurePrivateDir` was already at 100% as a side effect of
`TestOpen_RootDirIsFile`'s root-as-file collision (it's the function
that test forces to fail). `store.go`: 87.7% → 88.2%; package: 89.5% →
89.8%.

**`internal/users` (`store.go` increment, sub-increment 2 of 2 —
filesystem-backed functions): 88.2% → 93.6% of `store.go` (89.8% →
93.0% of the package). This closes out `internal/users` entirely** —
every remaining uncovered line across `dek.go`, `recovery.go`, and
`store.go` traces to a named, disclosed reason. 9 new tests close
`MigrateLegacyRoot`, `migrateLegacyRoot`, `copyTree`, and `copyFile`;
`MigrateLegacyRoot`, `migrateLegacyRoot`, and `copyFile` reach 100%.
Every forcing technique was probe-confirmed before being written into
a real test, including `syscall.Mkfifo` (not just predicted from
`sun_path` reasoning — actually run and confirmed to land in
`copyTree`'s `default:` skip branch, before the earlier same-day
correction entry above finished writing that prediction down).

`/advisor`'s plan review, conducted before any code was written,
predicted four of the eleven tests' assertions would be masked as
planned and specified the fix for each — all four confirmed exactly as
predicted by direct mutation before finalizing:

- **All four `copyFile` tests are cascade-prone by construction.**
  Every guard in `copyFile` leaves a nil `*os.File` behind when
  deleted, and nil-receiver `*os.File` methods return `os.ErrInvalid`
  ("invalid argument") rather than panicking, so a downstream guard
  always produces *some* non-nil error. `TestCopyFile_MkdirAllFails`
  asserts "mkdir" (a deleted guard cascades into `os.OpenFile`'s "not a
  directory," which happens to also contain that substring — probed
  first, assertion written narrow enough to discriminate anyway).
  `TestCopyFile_SourceOpenFails` asserts "no such file or directory"
  (cascades to the disjoint "invalid argument" instead).
  `TestCopyFile_DestOpenFails` asserts "is a directory" (same disjoint
  cascade). `TestCopyFile_IoCopyFails` is deliberately a bare non-nil
  check: deleting that guard falls through to a valid, empty-file
  `out.Close()` — genuinely nil, not a cascade — and pinning
  platform-specific wording would be unreliable, since Linux's
  `io.Copy` may route a directory read through `copy_file_range`
  rather than macOS's "read ...: is a directory" text.
- **`copyTree`'s symlink and `default:` arms are behaviorally
  identical, so neither is guard-verifiable individually.** Deleting
  the symlink case's body makes a symlink fall through to
  `default:`'s identical nil return (its `IsRegular()` check is false,
  Lstat-based), and `default:` can't be deleted at all without a
  "missing return" compile error. Kept the test — "symlinks and
  irregular files are never copied" is a real contract — but disclosed
  it as pinning behavior, not verifying either arm's presence: a
  fourth distinct disclosed-limit shape this session, alongside
  `recovery.go`'s error-unification and `internal/admin`'s
  no-propagation disclosures.
- **`MigrateLegacyRoot`'s own early-return guard is masked by
  `migrateLegacyRoot`'s identical duplicate check one call deeper** —
  the `internal/admin` redundant-downstream-guard shape recurring
  again. Confirmed by direct mutation before writing the test: deleting
  the outer check produces the exact same `(false, nil)`. Kept as a
  documented pure-optimization comment rather than a guard-presence
  test; `TestMigrateLegacyRoot_NoLegacyInstallFound` (the loop-exhausted
  fallback) is coverage-only for the same underlying reason a different
  branch can't be deleted at all — no mutation exists to run.

The real break-verification test in that trio,
`TestMigrateLegacyRoot_PropagatesCopyTreeError`, forces
`migrateLegacyRoot`'s `copyTree` call to fail (a `newRoot` subdirectory
pre-occupied by a file) and confirms `MigrateLegacyRoot` propagates the
error out through its loop rather than swallowing it — genuinely
break-verified, no cascade.

Two branches stay disclosed-untested in `copyTree`: `filepath.Rel`'s
error (unforceable — `WalkDir` always builds `path` from `src` via
`Join`, so the two are always Rel-compatible) and `d.Info()`'s error (a
mid-walk TOCTOU race with no deterministic, portable trigger).

Coverage ratchet updated: go_default 8755/14627 (5872 uncovered) →
8767/14627 (5860 uncovered); go_duckdb 8833/14725 (5892 uncovered) →
8845/14725 (5880 uncovered).

**`internal/pdfmeta` (increment 1 of at least 2): 81.7% → 86.6% of the
package.** Chosen as the next completion-tier target: 27 of its
functions had some uncovered branch, the largest concentration among
the 70–99% packages this session surveyed (`fonts`/`rfc3161`/`signing`
all ~81.6%, `sigma/service` 85.0% — all smaller). Deliberately scoped
to the file's two pure, fixture-free outliers first —
`ApplyPDFAMetadata` (0% → 100%, a real production helper that sets
fpdf `/Info` dict fields, called from `internal/documents` and
`internal/export`, just never unit-tested directly) and
`insertOutputIntentsReference` (30.3% → 100%, pure byte-manipulation
with three branches and no error paths) — leaving the file's larger
cluster of byte-surgery functions (`parseTrailerSizeAndRoot`,
`InjectPAdESSignature`, etc., 70–95%, all needing malformed-PDF
fixtures) for a later increment. 12 new tests.

`/advisor`'s plan review, before any code was written, flagged one
masking risk and pre-verified the plan's biggest unknown — fpdf has no
exported getters for `/Info` dict fields, so testing
`ApplyPDFAMetadata` means inspecting actual rendered PDF bytes, and the
values are UTF-16BE-with-BOM encoded (confirmed by reading
`go-pdf/fpdf@v0.9.0`'s source before relying on it, not assumed):

- `TestApplyPDFAMetadata_DefaultsAuthorWhenEmpty`'s first draft would
  have asserted a bare `bytes.Contains(output, utf16be("GoPMgr"))`.
  Masked: `SetCreator("GoPMgr", true)` runs unconditionally a few
  lines later regardless of whether the author-default guard ran, so
  "GoPMgr" appears in the output either way. Fixed by asserting the
  key-adjacent needle `"/Author (" + utf16be("GoPMgr")` instead — the
  guard's own field, not just any occurrence of the string anywhere in
  the PDF. Confirmed red under the targeted mutation.
- `TestInsertOutputIntentsReference`'s three "value already exists"
  tests (array, bare indirect-ref, value-ending-at-`>>`) exist because
  deleting that entire branch doesn't produce an obviously-wrong
  result: execution falls through to the "insert fresh" branch and
  produces a **second** `/OutputIntents` key while the stale one
  survives — confirmed by direct mutation before finalizing the
  assertion. `bytes.Count(result, []byte("/OutputIntents")) == 1`, not
  a bare presence check, is the discriminator in all three.

A genuinely new break-verification mechanism for this package: both
`ApplyPDFAMetadata`'s `pdf == nil` guard (behind
`TestApplyPDFAMetadata_NilPDFIsNoop`) and its `len(spec.Keywords) > 0`
guard (protecting `spec.Keywords[0]`, exercised by every
`ApplyPDFAMetadata` test that passes an empty `Keywords` slice) are
covered and break-verify via a **panic** — deleting either crashes the
caller (nil-pointer method call; index-out-of-range on an empty slice)
rather than returning a differing value — confirmed by mutation, not
assumed. No prior `internal/pdfmeta` test needed this framing.
`TestInsertOutputIntentsReference_InsertsFreshEntry` is covered but not
break-verifiable: it's the function's final statement with nothing
following it, so deleting it is a "missing return" compile error, not a
behavior change — same category as `internal/users`' `MigrateLegacyRoot`
loop-exhausted fallback. The one genuine disclosed-untested branch this
increment leaves behind is `icc.go`'s `DefaultICCProfile` `len(sRGBICC)
== 0` check: `sRGBICC` is populated by a `//go:embed` directive from a
file checked into the repo, so the branch is unreachable in any build
that actually compiles.

Also surfaced: `ApplyPDFAMetadata` hardcoded `/Info` dict Creator to
`"GoPMgr"` and ignored `XMPSpec.CreatorTool` entirely, while
`BuildXMPPacket` honored `CreatorTool` for the XMP packet's
`<xmp:CreatorTool>` element — a caller setting `CreatorTool` got
inconsistent metadata between the two surfaces. Flagged as a
follow-up at the time, not silently pinned as if intentional.

Coverage ratchet updated: go_default 8767/14627 (5860 uncovered) →
8805/14627 (5822 uncovered); go_duckdb 8845/14725 (5880 uncovered) →
8883/14725 (5842 uncovered).

### 2026-08-05 — resolved the `CreatorTool` inconsistency; removed three dead re-export shims

Fixed same-day, before starting increment 2. Framing matters here:
this is a **consistency alignment**, not a bug fix — `ApplyPDFAMetadata`
now defaults `spec.CreatorTool` to `"GoPMgr"` and honors it, mirroring
`BuildXMPPacket`'s exact pattern, but the change produces **zero
observable difference for any current caller**, confirmed by grepping
every call site before making it:
- `internal/documents/fonts.go`'s only direct call already passes
  `CreatorTool: "GoPMgr"`, identical to the prior hardcoded value.
- `internal/export`'s `ApplyPDFAMetadata` wrapper used to override
  Creator with a version-suffixed string immediately after delegating
  here — but that wrapper turned out to be **dead code**: 0% coverage,
  no tests, and unreachable from anywhere in the module (grepped every
  `.ApplyPDFAMetadata(` call site; the only two are the definition
  itself and its own internal delegate call). The real export-package
  renderers (`pdf.go`, `sigma_report.go`, `montecarlo_report.go`) all
  call `pdf.SetCreator` directly and never went through this wrapper.

Removed the dead wrapper rather than leaving it to paper over. This is
what resolves the "which surface stays divergent" question cleanly:
after removal there is no live path left that sets Creator to anything
other than what `spec.CreatorTool` (or its default) specifies, so
there's nothing left to reconcile. The same grep-every-call-site check
applied to the rest of `internal/export/pdfa.go` found two more dead
shims with the identical profile (0% coverage, no tests, no unqualified
callers): `InjectPAdESSignature` (the real caller,
`internal/signing/pades.go:127`, calls `pdfmeta.InjectPAdESSignature`
directly) and `HasDefaultICC` (the real caller,
`internal/export/sigma_report.go:52`, calls `pdfmeta.HasDefaultICC()`
directly, not the shim — found on `/advisor`'s second pass after the
first draft of this entry incorrectly claimed all remaining shims were
confirmed live). All three removed. The file's other shims
(`BuildXMPPacket`, `InjectXMPStream`, `MakePDFA3`, `DefaultICCProfile`)
each have a genuine unqualified caller inside the package and stay.

Two new tests — `TestApplyPDFAMetadata_HonorsCreatorTool` and
`TestApplyPDFAMetadata_DefaultsCreatorToolWhenEmpty` — cover the
default-then-use logic and break-verify independently of each other
(reverting to the hardcoded value fails the first but not the second;
making `CreatorTool` unconditional fails the second but not the
first).

Coverage ratchet updated: go_default 8805/14627 (5822 uncovered) →
8807/14624 (5817 uncovered); go_duckdb 8883/14725 (5842 uncovered) →
8885/14722 (5837 uncovered). Decomposition: the three removed shims
totaled 5 statements (`ApplyPDFAMetadata`'s body: the delegate call,
the `if pdf != nil` guard, the `SetCreator` call — 3 statements;
`InjectPAdESSignature` and `HasDefaultICC` — 1 each), all of them
previously uncovered, so removing them alone accounts for the full −5
uncovered delta. The `ApplyPDFAMetadata` fix in `pdfmeta.go` added 2
new statements (the `if spec.CreatorTool == ""` guard and its
assignment), both immediately covered by the two new tests — they
contribute to the +2 covered delta and the denominator math (−5
removed +2 added = −3 total) but not to the uncovered delta, since a
newly-added *and* newly-covered statement was never counted as
uncovered in the first place.

### 2026-08-05 — `internal/pdfmeta` increment 2a: 86.6% → 88.0%; the leaf parsers in the `InjectXMPStream` call chain, split from 2b per `/advisor`'s plan review

`/advisor`'s plan review split the originally-scoped increment 2 (8
functions, ~20 tests, mixing pure byte-literal unit tests with
staged-fixture wrapping-chain tests) into 2a (leaf parsers, byte
literals, no staging) and 2b (the call chain itself, needs staged
fixtures and wrap-prefix discrimination) — same reasoning as the
`internal/admin` review-load lesson: two techniques in one increment
is two reviews' worth of risk in one pass. 2a covers
`findLastStartxref`, `readDictInt`, `readDictRef`, `findObjectBody`,
and `insertMetadataReference` (11 statements, all now 100%); test-only
change, `pdfmeta.go`'s production code untouched (`git diff --stat`
confirms only `pdfmeta_test.go` changed).

Three masking risks found by actually running each mutation, not
predicted — a discipline that paid off three separate times in one
increment:

1. `readDictInt`/`readDictRef`'s "key not present" guards are
   fixture-dependent cascades: deleting the guard leaves `idx == -1`,
   and the resulting index arithmetic can coincidentally land on a
   non-digit byte, tripping the *next* guard's error instead of
   returning cleanly — a bare `err == nil` check stays green under
   that mutation. Fixed by asserting the guard's own error-text
   substring.
2. `readDictRef`'s "no id digit" guard is masked **unconditionally**,
   not fixture-dependently — a new category, distinct from every
   other cascade this session (all of which were fixture- or
   context-dependent and could in principle be dodged by a different
   input). The leading whitespace-skip before the id-digit scan has
   already consumed every space/tab/newline/CR, so whatever non-digit
   byte blocks the id-digit scan is guaranteed non-whitespace — which
   means the second (space/tab-only) skip before the gen-digit scan
   can't advance past it either, so the gen-digit scan fails at the
   *identical* byte position for every possible input. No fixture
   exists that avoids this; error-text assertion is the only
   discriminator.
3. `insertMetadataReference`'s leading-whitespace-trim test is masked
   by its own next guard: an untrimmed body starts with a whitespace
   byte instead of `<<`, so deleting the trim loop trips the
   "malformed input" guard and falls back to wrapping the whole
   (still-untrimmed) body in a fresh `<<...>>` shell, which still
   contains both expected substrings — a bare `Contains` check stays
   green. Discriminated via `bytes.Count(got, []byte("<<")) == 1`, the
   same duplicate-key-counting technique as increment 1's
   `insertOutputIntentsReference` fix.

Two of the new tests' discriminating claims were independently
re-verified via targeted half-mutations after `/advisor`'s adversarial
pass flagged that the first draft asserted rather than demonstrated
them: `TestFindLastStartxref_OffsetOutOfRange`'s "both operands
independently" claim (confirmed: each half of the `||` condition,
mutated alone, fails only its own subcase) and
`TestInsertMetadataReference_WrapsNonDictBody`'s exact mutation
output (a stale `42`-vs-`99` docstring literal was also caught and
fixed in the same pass).

Coverage ratchet updated: go_default 8807/14624 (5817 uncovered) →
8818/14624 (5806 uncovered); go_duckdb 8885/14722 (5837 uncovered) →
8896/14722 (5826 uncovered) — 11 statements newly covered, denominator
flat on both tags (test-only change, matches the prediction exactly).
The ratchet script's known exclude-filter flake (see
`DEVELOPER_HANDBOOK.md`'s "coverage ratchet exclude-filter flake" entry; a background
task is tracking the fix) reproduced once during this increment's
`--update` run, reporting a spurious ~3× inflated REGRESSED before a
clean re-run gave the correct numbers above — not accepted, re-run
instead, per the established handling for that flake.

### 2026-08-06 — `internal/pdfmeta` increment 2b: 88.0% → 89.9%; the `InjectXMPStream` call chain itself (`parseTrailerSizeAndRoot`, `InjectXMPStream`, `BuildXMPPacket`)

Closes the three functions 2a deliberately deferred (see 2a's entry
above): `parseTrailerSizeAndRoot`, `InjectXMPStream`, `BuildXMPPacket`
— all now 100%; test-only change, `pdfmeta.go`'s production code
untouched (`git diff --stat` confirms only `pdfmeta_test.go` changed).
13 new tests, package total 53 → 66.

+15 covered statements (`BuildXMPPacket` +3, `InjectXMPStream` +7,
`parseTrailerSizeAndRoot` +5) — not the "12 statements" this section
previously estimated. The gap is a scoping lesson, not an error in the
work itself: the original estimate eyeballed one statement per
`@@UNCOV@@` HTML span, which undercounts any span containing multiple
assignments — `InjectXMPStream`'s xref-entry-swap block
(`first, second = second, first; firstOff, secondOff = ...; firstGen,
secondGen = ...`) is one `if` but three statements, accounting for the
extra 2. A fourth branch, `BuildXMPPacket`'s `spec.Description != ""`
guard, wasn't in the original plan at all — it surfaced only after a
coverage check following the Title/Author tests showed 97.0%, not the
assumed 100%. Lesson for future increment scoping: verify statement
counts from `go tool cover -func`'s numbers, not from counting
`@@UNCOV@@` spans by eye. The +15 matches the coverage ratchet's
observed uncovered-count improvement exactly on both build tags
(5806 → 5791 on go_default, 5826 → 5811 on go_duckdb), confirming the
denominator held flat as expected for a test-only change.

`parseTrailerSizeAndRoot`'s five guards are structurally identical to
2a's `readDictInt`/`readDictRef` cascade shape, so every test asserts
the guard's own error text rather than a bare `err != nil` check —
reusing rather than rediscovering that lesson:

1. `TestParseTrailerSizeAndRoot_XrefOffsetOutOfRange` (3 subcases:
   negative, exactly `len(b)`, past `len(b)`) is a slice of structs,
   not a map — map iteration order is randomized, and the "negative"
   subcase's mutated-guard behavior differs qualitatively from "past
   len(b)"'s (see next point), so randomized order would make which
   subcases actually execute before a panic non-deterministic across
   runs. Confirmed by mutation (stripping only the guard's `>= len(b)`
   half) that "exactly len(b)" cascades cleanly into the next guard's
   "trailer keyword not found" text (`b[len(b):]` is a legal empty Go
   slice — no panic, just a different wrong error), while "past
   len(b)" panics on a genuine slice-bounds violation. Both observed
   directly.
2. `TestParseTrailerSizeAndRoot_SizeReadError`/`_RootReadError` assert
   the `"/Size: %w"`/`"/Root: %w"` wrap prefixes. First-draft
   break-verification mutation was invalid — it let a later assignment
   silently overwrite `err`, so the test went red for the wrong reason
   (`/advisor` caught this on review). Corrected mutation strips only
   the wrap (`return 0, 0, 0, err` in place of
   `return 0, 0, 0, fmt.Errorf("/Size: %w", err)`) while preserving
   guard control flow; re-run confirmed red with the guard's own bare
   `"not present"` text, no `/Size:`/`/Root:` prefix.

`InjectXMPStream`'s three error-propagation tests each assert their
own `"pdfmeta: <stage>: "` wrap prefix (locate startxref / parse
trailer / locate Catalog object) rather than a bare non-nil check —
the function wraps three sequential stage calls, and a fixture aimed
at breaking stage N can accidentally also break stage N-1 while still
returning non-nil, which wrap-prefix discrimination catches.
`TestInjectXMPStream_AddsTrailingNewlineWhenMissing` break-verifies
via the exact glued byte sequence observed under mutation
(`out[len(pdfBytes):]` reads `"4 0 obj\n<<\n/Type /Me"` with the guard
deleted — the metadata object's leading digit glues directly onto the
trimmed fixture's last byte with no separator). The xref-entry-swap
branch is unreachable for any spec-valid PDF (confirmed as scoped in
2a's review: `/Size` is spec-defined as one greater than the highest
object number, so `metaID` — derived from `/Size` — is always greater
than any existing object ID); reached only via `swappedIDPDF()`, a
hand-built, deliberately spec-violating fixture (`/Root 9 0 R` but
`/Size 4`) — documented as covering defensive code, not modeling a
realistic input.

`BuildXMPPacket`'s Author-default guard is masked three ways exactly
as 2a's review predicted (`x:xmptk="GoPMgr"`, `<pdf:Producer>GoPMgr
</pdf:Producer>`, and — when CreatorTool is also unset —
`<xmp:CreatorTool>GoPMgr</xmp:CreatorTool>`, none dependent on the
guard); `TestBuildXMPPacket_DefaultsAuthorWhenEmpty` uses a
tag-adjacent needle to isolate `<dc:creator>`'s own value, confirmed
by mutation that the guard-deleted output leaves `<dc:creator>`
genuinely empty while the three unconditional "GoPMgr" occurrences
remain — a bare `bytes.Contains(pkt, []byte("GoPMgr"))` would have
stayed green. All three `BuildXMPPacket` tests (Title default, Author
default, Description-when-set) were mutation-tested in this pass; this
closes a gap from the increment's first draft, where they were written
and confirmed to pass but not yet put through the mutate/run/restore
cycle. Self-identified from a pending-task recount, not from an
`/advisor` review pass — the omission was closed before writing any
documentation that would have claimed "confirmed by mutation" for
these three without it being true yet.

Two self-authored fixtures initially contained the literal keyword
they were meant to test the *absence* of (`"no trailer keyword
anywhere in this buffer"` contains "trailer"; `"no startxref marker
here"` contains "startxref"), causing false failures on the first
green-baseline run — not a mutation catching a masked guard, just a
fixture-authoring mistake caught immediately by the test failing loud.
Both reworded to neutral filler text before finalizing.

Coverage ratchet: go_default 8818/14624 (5806 uncovered) → 8833/14624
(5791 uncovered); go_duckdb 8896/14722 (5826 uncovered) → 8911/14722
(5811 uncovered) — 15 statements newly covered on both tags, matching
the per-function delta above exactly. The ratchet's known
exclude-filter flake (see `DEVELOPER_HANDBOOK.md`) reproduced twice
across this increment's work (once during the `--update` run itself,
once on a later confirmation re-run), each resolved by an immediate
clean re-run per the established handling rule — never accept a
REGRESSED result without re-running once.

### 2026-08-06 — `internal/pdfmeta` increment 3a: 89.9% → 92.2%; `MakePDFA3`'s binary-header-comment-insertion subsystem, plus a real silent-corruption bug found and fixed while covering it

Covers `ensureBinaryHeaderComment`, `hasBinaryHeaderComment`,
`shiftClassicXrefOffsets`, `parsePositiveDecimal`,
`replaceStartxrefValue` — the subsystem `MakePDFA3` calls first, which
inserts a 5-byte binary marker comment after the PDF header and
rewrites every existing classic-xref-table offset plus the trailer's
`startxref` pointer to account for the byte shift. Selected ahead of
`MakePDFA3`'s own error wraps and `readTrailerIDValue` (both scoped
out for later increments) because this cluster does the actual
byte-offset arithmetic — a bug here causes silent PDF corruption
(wrong offsets, no error), not just a missing test. All target
functions now 100% except `ensureBinaryHeaderComment` (95.7%, one
disclosed gap). 19 new tests, package 66 → 85. `pdfmeta.go` gained 17
lines (a genuine production fix, not test-only — see below);
confirmed via `git diff --stat` that nothing else in the file changed.

**Found and fixed a real bug, not just written tests for existing
code.** `/advisor`'s pre-implementation plan review flagged that
`shiftClassicXrefOffsets`'s offset-rewrite line —
`copy(pdfBytes[entryStart:entryStart+10], []byte(fmt.Sprintf("%010d",
oldOff+delta)))` — silently truncates via Go's `copy()` semantics
(copies `min(len(dst), len(src))` bytes from the start) if the shifted
offset ever needs 11+ digits instead of the classic xref format's
fixed 10. Confirmed directly, not just reasoned about: `copy()` of the
11-digit string `"10000000004"` into a 10-byte destination yields
`"1000000000"` — nearly 10x smaller than correct, with no error
raised. Only reachable on a PDF whose existing offset is within
`delta` (typically 6) of 9999999999 (~9.3GB) — astronomically
unlikely for this application's generated reports and charts — but
the fix (check the shifted value against the field's max before
writing, return an error instead of truncating) costs three lines and
closes this specific silent-truncation path.
`TestShiftClassicXrefOffsets_OffsetOverflowReturnsError`
break-verifies the new guard directly, without needing an actual
multi-gigabyte fixture: the function only parses and rewrites digit
text, it never dereferences `pdfBytes` at the parsed offset, so a
small buffer with a hand-written 10-digit offset field near the
format's boundary exercises the exact arithmetic.

**This fixes one instance, not the whole bug class — `/advisor`'s
post-implementation review caught the docs (an earlier draft of this
entry) overclaiming otherwise.** `shiftClassicXrefOffsets` is not the
only place this package writes a fixed-width classic-xref offset
field: `InjectXMPStream` (`pdfmeta.go:175,177`), `InjectOutputIntent`
(`pdfmeta.go:537,539,545`), and `InjectPAdESSignature`
(`pdfmeta.go:981`) all build new xref entries via
`fmt.Fprintf(&appended, "%010d %05d n \n", off, gen)`. Those write
into a growing `bytes.Buffer` rather than a fixed destination slice,
so they don't truncate the way `copy()` did — but `Fprintf` doesn't
truncate either; an offset `>= 10^10` would simply widen the field to
11+ digits, producing a 21-byte (or longer) entry line where the
classic xref format requires exactly 20, corrupting the fixed-width
alignment of every subsequent entry for any reader that does offset
arithmetic against table position rather than re-scanning line by
line. Same root cause (unbounded value written into a spec-fixed-width
field) and the same ~9.3GB reachability bound as the bug fixed above.
Deliberately **not** fixed in this increment — these three functions
are already in scope for the deferred byte-surgery cluster (see the
"Not started" pointer below), and bundling this fix into that pass
keeps the guard placement and error wording consistent across all four
sites rather than adding a one-off now and three more later.

*Resolved 2026-08-06, increment 4b (see below) — and the "same
~9.3GB reachability bound" claim above turned out to be only half
true.* The generation half of the field is reachable with a small
hand-built fixture, not a multi-gigabyte one: it's parsed straight out
of the trailer's `/Root <id> <gen> R` entry and flows unchanged into
the rewritten Catalog's own xref entry in all three functions. Only
the offset half needs an impractically large input. The line numbers
above (`:175,177` / `:537,539,545` / `:981`) had also drifted from
intervening edits by the time 4b landed; see
`TEST_COVERAGE_LEDGER.md`'s increment 4b entry for the fix itself
rather than this historical entry, which is left as originally
written aside from this note.

**The direct positive-path test for `ensureBinaryHeaderComment`
required a redesign after the first draft's round-trip assertion
turned out not to prove anything.** The original plan was to round-trip
the shifted output through `findLastStartxref` /
`parseTrailerSizeAndRoot` / `findObjectBody` (the same shape
`TestMakePDFA3PreservesMetadataAndOutputIntentInLatestCatalog`
already uses) and treat a successful Catalog lookup as proof the
offset shift is correct. It isn't: `findObjectBody` locates objects
via a textual `"<id> <gen> obj"` scan, never by dereferencing the
xref table's recorded byte offsets — so it reports success even if
every offset in the table is wrong, as long as the object text is
still present somewhere in the buffer. Confirmed by deliberately
mutating the shift's delta by one byte: the `findObjectBody`-based
round-trip stayed silently green under that mutation. Fixed by adding
a direct check that parses `minimalPDFWithoutBinaryComment()`'s known
xref structure, reads the shifted offset for object 1 straight out of
the table, and asserts `pdfBytes` at exactly that offset starts with
`"1 0 obj"` — re-running the same one-byte-delta mutation against this
stronger assertion correctly went red. The textual round-trip is kept
as a secondary, complementary check (it does still prove the trailer's
`/Root` reference and object body are intact), but it is documented as
not sufficient on its own — future tests validating this package's
offset arithmetic should default to reading the xref table directly,
not just checking that *some* lookup path succeeds.

`ensureBinaryHeaderComment`'s early-return path (already has the
binary comment) is asserted via pointer identity
(`&out[0] == &pdf[0]`), not just byte equality, since the function
returns `pdfBytes` itself unaliased on that path and a future
"defensive copy" refactor changing that allocation behavior should be
a deliberate, reviewed decision. `hasBinaryHeaderComment`'s
non-binary-comment guard uses a fixture chosen to pass the preceding
length/prefix guard cleanly (`"%ABCD\n"`), so a mutation of that
specific guard is what the test catches, not an accidental trip of an
earlier one — confirmed by mutation. `shiftClassicXrefOffsets`
contributes three panic-based catches (unterminated xref line,
malformed subsection header, unterminated xref entry — all
slice-bounds or index-out-of-range panics under mutation, same
category as increment 2a's `TestFindObjectBody_EndobjNotFound`) and
one coverage-only gap: the terminal `"xref trailer not found"`
fallthrough is the function's last statement, so deleting it is a
"missing return" compile error, not a testable mutation — same
category as increment 2a's `TestInsertOutputIntentsReference_InsertsFreshEntry`.
`ensureBinaryHeaderComment`'s own disclosed gap (95.7%, the propagation
of `replaceStartxrefValue`'s error) is structurally unreachable
through that call chain: `findLastStartxref` already validates a
`"startxref"`-plus-digits pair exists in the original bytes before
either downstream function runs, the comment insertion never touches
the trailer/startxref region, and both functions independently
`bytes.LastIndex` for the same literal text — so whatever
`findLastStartxref` found, `replaceStartxrefValue` finds too. Same
disclosed-gap framing as `icc.go`'s `DefaultICCProfile` from
increment 1; not contrived into false coverage. `replaceStartxrefValue`'s
own two guards are still covered directly (not only through
`ensureBinaryHeaderComment`) since it's a general-purpose helper whose
guards deserve unit-level coverage independent of what its one current
caller can reach.

Two findings from `/advisor`'s post-implementation review, both
corrected before commit: (1) `TestShiftClassicXrefOffsets_InvalidXrefOffsetReturnsError`'s
table-driven test originally shared one mutable buffer across all four
subcases — `shiftClassicXrefOffsets` rewrites entries in place, so a
subcase reaching the entry-rewrite loop under some future mutation
could leak its partial rewrite into the next subcase's input, the
same order-dependence class already fixed once this increment (map →
slice for deterministic iteration) but only half-addressed; fixed by
constructing a fresh buffer per subcase. (2) A third occurrence this
session of the self-negating-fixture mistake — a filler-text fixture
(`"no startxref here at all"`) accidentally containing the literal
keyword it existed to test the absence of — happened in this
increment's first draft, identical to two prior occurrences in
increment 2b; see `DEVELOPER_HANDBOOK.md` for the mechanical-check
note this recurrence earned.

Coverage ratchet: go_default 8833/14624 (5791 uncovered) →
8853/14627 (5774 uncovered); go_duckdb 8911/14722 (5811 uncovered) →
8931/14725 (5794 uncovered) — +20 covered, +3 denominator (the
overflow guard's three new statements: `newOff := oldOff + delta`,
the `if` condition, and its `return`), net −17 uncovered on both
tags, matching the test-coverage delta exactly (17 pre-existing
statements newly covered, 3 new statements added by the fix and all
covered by its own test). The exclude-filter flake (see
`DEVELOPER_HANDBOOK.md`, now 5 occurrences) reproduced once during
this increment's `--update` run; not accepted, re-run instead. `go
build ./...`, `gofmt -l`, `go vet`, and the full `go test ./...`
module suite pass; `internal/pdfmeta` at 85 tests, confirmed via
`grep -c "^func Test"`.

### 2026-08-06 — `internal/pdfmeta` increment 4a: 92.2% → 95.7%; the AcroForm signature-field-merging cluster, plus a second silent-corruption bug swept across the whole file, not just the one instance found

Covers `signatureFieldReferenceRewrites`, `appendSignatureFieldToFields`,
`ensureSignatureFieldFlags`, `readRefAt`, `findDictionaryEnd`,
`findArrayEnd` — the cluster that rewrites a PDF's `/AcroForm`
dictionary to register the new signature field, called from
`InjectPAdESSignature`. Selected ahead of `InjectPAdESSignature`'s own
15 uncovered blocks (deferred to increment 4b) because this cluster
does the actual dictionary rewriting for signature registration — a
bug here corrupts the signed PDF's form structure, not just misses a
test; per the workflow's priority order this ranks with 3a's
byte-offset arithmetic, not with ordinary missing coverage. All six
target functions now 100%. 31 new tests, package 85 → 116.
`pdfmeta.go` gained a net 12 statements (the `parseDigits` helper plus
its 8 call-site integrations — a genuine production fix, not
test-only; confirmed via `git diff` that nothing else in the file
changed).

**A second silent-corruption bug, same class as increment 3a's, found
while covering `ensureSignatureFieldFlags`.** `/advisor`'s
pre-implementation plan review flagged that
`ensureSignatureFieldFlags` parsed `/SigFlags`'s digit run with a bare
`flags = flags*10 + int(c-'0')` loop and no bounds check — the exact
same shape as 3a's `copy()`-truncation bug, but with a far lower
reachability bar: no 9.3GB file needed, just a 20-digit number in a
dictionary. Confirmed directly, not just reasoned about:
`ensureSignatureFieldFlags([]byte("<< /SigFlags
99999999999999999999 >>"))` returned `err=nil` and
`"<< /SigFlags 7766279631452241919 >>"` — a silently wrapped, wrong
value that would be written straight into the signed PDF's AcroForm
dict.

**Fixed by sweeping the whole file, not just the two functions this
increment's declared scope was already touching — because a follow-up
`/advisor` consultation asked "is this the only instance?" before the
plan was finalized, and the answer was no.** The first pass (by
reading) found four call sites: `readRefAt` (id, gen — already in
this increment's scope) and `readDictRef` (id, gen — an
already-shipped, already-100%-covered function from increment 2a).
`/advisor` pushed back: enumerate the class mechanically, not by
reading. `grep -n "10 + int("` surfaced 8 accumulation sites across 6
functions: `findLastStartxref`, `readDictInt`, `readDictRef` (×2),
`parsePositiveDecimal`, `ensureSignatureFieldFlags`, `readRefAt` (×2)
— four of six functions (`findLastStartxref`, `readDictInt`,
`readDictRef`, `parsePositiveDecimal`) outside this increment's
original declared scope, spanning increments 1, 2a, and 3a. Extracted a shared
`parseDigits(digits []byte) (int, error)` helper (checks
`n > (math.MaxInt-d)/10` before each multiply/add, the same overflow
math as any base-10 accumulator needs) and routed every one of the 8
sites through it. Each of the four already-100%-covered functions
(`findLastStartxref`, `readDictInt`, `readDictRef`,
`parsePositiveDecimal`) gained one new overflow test so the guard
didn't silently regress their coverage.

This is a deliberate departure from increment 3a's own precedent,
where the sibling `%010d`-field-width sites were disclosed and
deferred rather than fixed immediately (see that entry above) — worth
being explicit about why the same reasoning didn't apply here: 3a's
siblings needed a *different* guard (field-width overflow in an
`Fprintf` into a growing buffer, not `copy()` truncation into a fixed
slice), so fixing them wasn't free — each needed its own reasoning
and its own test. Here, every one of the 8 sites needed the *identical*
fix, and the shared helper was being written in this commit regardless
of scope — leaving four raw `n = n*10 + int(c-'0')` loops sitting next
to `parseDigits` in the same file would have been an invitation for a
future reader (or a future `/advisor` review) to assume the file was
now uniformly safe when it wasn't. Deferring here would have
reproduced the exact incomplete-sweep mistake the 3a-follow-up commit
(`02c797e`) had just finished correcting.

**Test design.** `readRefAt`'s three pre-existing guards (no id digit,
no gen digit, expected R after gen) had zero prior coverage —
84.2%→100% — tested directly rather than only through
`InjectPAdESSignature`, since its happy path is already exercised via
`TestInjectPAdESSignature_AppendsExistingIndirectAcroFormFields`.
`signatureFieldReferenceRewrites`'s six branches (non-dict-catalog
fallback, malformed-/AcroForm-entry, and four wrap-propagation guards
for `findDictionaryEnd`/`mergeSignatureFieldIntoAcroForm`/`readRefAt`/`findObjectBody`)
are tested directly for the same reason `replaceStartxrefValue` and
`parseTrailerSizeAndRoot`'s guards were in earlier increments: none of
these branches are reachable through any fixture this app's own PDF
generation would produce. `appendSignatureFieldToFields`'s and
`ensureSignatureFieldFlags`'s "insert fresh key when absent" branches
assert the exact rebuilt byte sequence, not `bytes.Contains`: a
substring check would stay green under a mutation that instead
(wrongly) took the existing-array/existing-flags append path, since
the field reference text would still appear somewhere in the output
either way.

A dedicated interaction test,
`TestMergeSignatureFieldIntoAcroForm_InsertsBothWhenBothAbsent`
(`mergeSignatureFieldIntoAcroForm(<< >>, id)`, not the two fresh-insert
branches exercised separately), exists because `/advisor`'s
post-implementation review flagged that their *sequencing* was
untested: `appendSignatureFieldToFields` runs first and inserts
`/Fields` immediately after `<<`; `ensureSignatureFieldFlags` then runs
on that already-rebuilt body and inserts `/SigFlags` immediately after
the *same* `<<` — so the final output is
`<<\n/SigFlags 3\n/Fields [ N 0 R ] >>`, `/SigFlags` before `/Fields`,
not the reverse. Only an exact-byte-sequence assertion (not a
`bytes.Count(...) == 1` presence check) catches a mutation to that
ordering. `findDictionaryEnd`'s and `findArrayEnd`'s "unterminated"
fallthroughs are each the function's final statement — coverage-only,
not break-verifiable via deletion, same category as increment 2a's
`TestInsertOutputIntentsReference_InsertsFreshEntry`.

**A pre-existing, low-severity parsing limitation was documented, not
fixed, this increment.** `signatureFieldReferenceRewrites` and its
callees locate keys and dictionary/array boundaries via plain byte
search, not a real PDF tokenizer — they don't skip string literals. A
catalog whose `/Title` value happened to contain the literal text
`/AcroForm` inside parentheses could in principle confuse the scan.
Traced all four call sites of `InjectPAdESSignature`
(`internal/signing/pades.go` → `app_documents.go`,
`internal/documents/charter.go`, `internal/export/pdf.go`) and
confirmed every one signs a PDF this app generated itself via
`documents.BuildCombinedReport`/`fpdf`, never an externally-uploaded
PDF; user-entered text (report titles, section content) lands inside
content streams, not inside the Catalog/AcroForm structural
dictionaries these functions scan — so the collision isn't reachable
through this app's own generation path today. Recorded here as a known
limitation, not fixed and not contrived into a misleading test.

Coverage ratchet: predicted before running `--update` via a
`git stash`-isolated clean before/after package comparison
(718/779 → 757/791, +39 covered/+12 total) — the actual ratchet numbers
matched that prediction exactly on both build tags: go_default
8853/14627 (5774 uncovered) → 8892/14639 (5747 uncovered); go_duckdb
8931/14725 (5794 uncovered) → 8970/14737 (5767 uncovered). The
exclude-filter flake (`DEVELOPER_HANDBOOK.md`, now 6 occurrences)
reproduced once during this increment's `--update` run; not accepted,
re-run instead, which produced the matching numbers above. `go build
./...`, `gofmt -l`, `go vet`, `go test ./internal/pdfmeta/... -race`,
and the full `go test ./...` module suite pass; `internal/pdfmeta` at
116 tests, confirmed via `grep -c "^func Test"`.

### 2026-08-06 — `internal/pdfmeta` increment 4b: 95.7% → 96.4%; `InjectPAdESSignature`'s own coverage, plus closing the disclosed `%010d`/`%05d` xref-field-overflow debt across all three functions at once

Covers `InjectPAdESSignature` itself (88.4% → 93.8%) and the
`%010d`/`%05d` fixed-width-xref-field-overflow guard 3a disclosed and
deferred for `InjectXMPStream`, `InjectOutputIntent`, and
`InjectPAdESSignature` (see the amendment above). 15 new tests
(116 → 131; a same-day increment-5a recount found this entry had
originally miscounted itself as "14 new tests (116 → 130)" — the diff
against the parent commit shows 15 `+func Test` lines and 0 removed).
Selected as the natural close of the byte-surgery
cluster 4a explicitly deferred, and because it was already opening
the exact three functions the disclosed debt named.

Extracted one shared `writeClassicXrefEntry(w *bytes.Buffer, id,
offset, gen int) error` helper and swept it across all 6 raw-`Fprintf`
call sites rather than fixing only `InjectPAdESSignature`'s own —
same sweep-when-cheap-and-identical reasoning as 4a's `parseDigits`
sweep. Unlike `shiftClassicXrefOffsets`'s increment-3a bug (`copy()`
truncates an overflowing value), `Fprintf`'s `%010d`/`%05d` verbs
don't truncate — they silently widen the field past its declared
width, corrupting the fixed 20-byte-per-entry alignment every
subsequent xref entry depends on.

**A pre-implementation `/advisor` review corrected a reachability
assumption before any test was written.** The initial plan carried
over 3a's framing that both the offset and generation halves need an
impractical ~9.3GB input to trigger, and planned to disclose the
generation half too. `/advisor` traced that `catalogGen` is parsed
straight out of the trailer's `/Root <id> <gen> R` entry and flows
unchanged into the rewritten Catalog's own xref entry in all three
functions — a small, hand-built fixture with an oversized (but
format-fitting) `/Root` generation reaches the guard directly. This
is why the sweep didn't cost a net increase in uncovered statements
the way a disclose-only plan would have. `rootGenOverflowPDF()` and
`swappedIDGenOverflowPDF()` (modeled on 2b's `swappedIDPDF()`) are
deliberately spec-invalid in a way this codebase's own guards don't
check — the PDF spec caps generation numbers at 65535, so a fixture
generation of 100000 is spec-invalid independent of whether it fits
`writeClassicXrefEntry`'s looser 99999 format-width guard; documented
in both fixtures' docstrings so a future reader doesn't mistake the
guard for a spec-conformance check. `swappedIDGenOverflowPDF` reuses
`swappedIDPDF`'s ID-swap trick because `InjectXMPStream`'s two
xref-entry writes are sequential: a non-swapped fixture's oversized
generation always lands on the first write and returns before the
second write's error-wrap is reached — caught directly, not
predicted, when the first version of this test alone left the
package at 98.3%, a real regression on what was previously a 100%
covered function.

**A boundary gap was found and fixed via `/advisor`'s
post-implementation review, the same category as 4a's
`TestParseDigits_BoundaryValues`.** The original three
`writeClassicXrefEntry` tests (a happy path at (1234, 5), and two
overflow fixtures far past each limit) left a `>` vs `>=` off-by-one
in either guard completely undetected — confirmed by mutation, the
whole suite stayed green with both guards changed to `>=`. Fixed by
asserting the exact largest legal entry (offset 9999999999, gen
99999) still succeeds and produces the exact expected 20-byte output;
re-verified by mutation that this test alone catches the off-by-one.

`InjectPAdESSignature`'s own 7 newly-tested branches (empty-PDF
guard, nil-`signRanges` guard, three error-wrap propagations, the
trailing-newline-insertion branch, and the `signRanges` error wrap)
reuse `InjectXMPStream`'s equivalent fixture patterns from increment
2b. The trailing-newline fixture needed a fresh
`bytes.TrimRight(minimalPDF(), "\n")` input — every existing fixture
in the file already ends in `\n`, so this branch had never been
exercised. **A masking gap found in the new empty-PDF test was traced
to an identical, pre-existing gap in 2b's `InjectXMPStream` one and
both were fixed together**: asserting only `err != nil` on nil input
stays green even with the guard deleted, since nil bytes also fail
`findLastStartxref` a few lines later with a different, still-non-nil
error; fixed by asserting each guard's own error text.

5 branches remain disclosed, confirmed by `/advisor` as genuinely
unreachable (or, for one, provably impossible) rather than merely
hard to fixture, and documented with an inline why-retained comment
in `pdfmeta.go` itself, not only the ledger — see
`TEST_COVERAGE_LEDGER.md`'s increment 4b entry for the full
per-guard reachability trace.

Coverage ratchet: predicted before running `--update` via a
`git stash`-isolated clean before/after package comparison
(757/791 → 769/798, +12 covered/+7 total) — the actual ratchet
numbers matched exactly on both build tags: go_default 8892/14639
(5747 uncovered) → 8904/14646 (5742 uncovered); go_duckdb 8970/14737
(5767 uncovered) → 8982/14744 (5762 uncovered). No flake this run.
`go build ./...`, `gofmt -l`, `go vet`,
`go test ./internal/pdfmeta/... -race`, and the full `go test ./...`
module suite pass; `internal/pdfmeta` at 131 tests, confirmed via
`grep -c "^func Test"` (this entry originally read "130"; corrected
2026-08-06 during increment 5a — see that entry's own count-reconciliation
note).

`InjectOutputIntent` (86.3%, down from 89.0%) is now a disclosed
permanent floor: the two new ICC-stream/OutputIntent-dict entry
guards this increment added are offset-only (both entries always
write generation 0) and unreachable without a ~9.3GB input, the same
bound as `InjectXMPStream`'s and `InjectPAdESSignature`'s equivalent
offset-only guards. **Superseded by increment 5a below**: that
increment's fuller enumeration of `InjectOutputIntent` found five more
real, addable branches this entry didn't scope (an empty-input guard,
an empty-ICC-profile guard, three error-wrap propagations) alongside
the two offset-only guards disclosed here and one further
unconditionally-unreachable ID-swap branch — this function was not, in
fact, down to a two-guard floor at the end of 4b.

### 2026-08-06 — `internal/pdfmeta` increment 5a: 96.4% → 97.6%; `MakePDFA3`, `streamPayload`, `readTrailerIDValue` closed, plus a mispredicted-mutation correction and an `InjectOutputIntent` scope correction

Covers `MakePDFA3`'s 3 error-wrap guards (binary-header-comment,
xmp-injection, outputintent-injection propagation), `streamPayload`'s
empty-input guard, and `readTrailerIDValue`'s 5 guards (xrefOffset
out-of-range, 2 subcases; trailer-keyword-not-found;
startxref-not-found-after-trailer; ID-not-followed-by-array;
unterminated-ID-array). 9 new tests, package 131 → 140. All three
functions now 100%; `pdfmeta.go` itself is untouched by this declared
scope — the `InjectOutputIntent` disclosure below is the increment's
one production-code edit, outside the three functions named above.

**A docstring's panic prediction was written before running the
mutation, then found wrong when actually run.**
`readTrailerIDValue`'s guard 5 (`i >= len(block)`, no closing `]`
before the block ends) was assumed, by analogy with the function's
other four guards (three of which do panic on deletion — confirmed by
mutation: `slice bounds out of range [-1:]` ×2, `[:-1]`), to panic the
same way. It does not: `block[start:i+1]` with `i == len(block)` is
legal because `block` is itself a sub-slice of the input `b`, and Go
slice capacity extends to the parent's backing array, not just the
sub-slice's own declared length — the expression reads one byte past
`block`'s logical end without exceeding its *capacity*. Confirmed via
a temporary debug test (`t.Logf`, deleted before finalizing): the
mutated function returns `("[<abc><def>\ns", true)` — a wrong,
non-empty value with `ok=true`, ending in the first byte of the
following `startxref` keyword — not a panic. The real test's docstring
was rewritten to describe this observed mechanism, with an explicit
note that an earlier draft assumed the wrong one, rather than silently
replacing it.

**`streamPayload`'s untouched branches are copy-not-strip, not a
bug — recorded because the new test's docstring points here.** Of its
three branches (empty input; input already ending in `\n`/`\r`; input
not ending in either), only the third defensively copies via
`append([]byte(nil), data...)`. Both call sites (`InjectXMPStream`'s
`xmpStream`, `InjectOutputIntent`'s `iccStream`) only read the result
(`len()` for `/Length`, then `bytes.Buffer.Write`, which copies
internally regardless), so the copy isn't load-bearing. Both call
sites also write an unconditional `\n` immediately after the stream
bytes and before `endstream` — the PDF-required EOL, computed and
written *after* `/Length` was already set from `len(streamPayload(...))`,
so it's correctly never counted. Nothing to fix.

**`InjectOutputIntent`'s remaining scope is larger than increment 4b
disclosed.** 4b described the function's remaining gap as "two
disclosed offset-only guards." A full `go tool cover -func` /
profile-span enumeration this cycle, done while checking what
remained in the package after the three declared functions above,
found five more real, addable branches 4b's framing didn't scope: an
empty-`pdfBytes` guard, an empty-`iccProfile` guard, and three
error-wrap propagations (`locate startxref`, `parse trailer`,
`locate Catalog object`) — the same guard shape already covered in
`InjectXMPStream` (2b) and `MakePDFA3` (this entry), just never
written for this function.

**A first-draft disclosure comment on the xref-entry-order swap guard
claimed unconditional unreachability; an adversarial `/advisor` pass
found the claim wrong before commit.** `iccID := trailerSize` and
`oiID := trailerSize + 1` are assigned unconditionally, three lines
apart, and the first draft reasoned that `if first > second` is
therefore false for every possible input — stronger than any other
disclosed guard in this package. `/advisor` asked whether
`trailerSize + 1` can overflow. `trailerSize` comes from
`readDictInt(block, "/Size")`, routed through `parseDigits`
(increment 4a's overflow guard) — which rejects any digit string that
would exceed `math.MaxInt`, but lets `math.MaxInt` itself through.
Confirmed by direct execution, not reasoning: a fixture with a literal
`/Size 9223372036854775807` parses to `trailerSize == math.MaxInt`,
then `oiID := trailerSize + 1` overflows to `math.MinInt`, flipping
`first > second` to true and producing a silently corrupted PDF
(negative object IDs in both the rewritten Catalog and the xref
table) with `err == nil` — the same silent-corruption shape as the
bugs fixed in increments 3a and 4a, one step downstream of an
already-guarded parse instead of an unguarded accumulator, gated
behind an exact 19-digit magic literal instead of an ordinary large
input. Traced every `InjectOutputIntent` call site to `MakePDFA3`,
and every `MakePDFA3` caller (`internal/documents`, `internal/export`)
passes bytes this app generated with fpdf moments earlier, never a
foreign or re-parsed trailer — not reachable through this app's own
generation path today, so the guard is disclosed inline in
`pdfmeta.go` (corrected comment, not the original "unreachable" one)
and flagged here, not fixed. Worth noting for whoever picks up the
five real branches below: this is the second "provably unreachable"
claim in this package's history to be wrong on a first pass (the
first was 3a's now-superseded "~9.3GB bound applies to both offset
and generation halves," corrected in 4b) — treat "unconditionally"/
"provably impossible" claims in this file as something to actively
try to break before committing them, not just state.

**Not implemented this increment**: the five real branches, and a fix
for the overflow above, remain "not started," scoped below as the
next bounded task rather than folded in here, per this workflow's
pre-implementation-review-before-scope-expansion rule.

Coverage ratchet: predicted before running `--update` via a
`git stash`-isolated clean before/after package comparison
(8904/14646 → 8914/14646 default, 8982/14744 → 8992/14744 duckdb,
+10 covered/+0 total on both tags — total unchanged because the one
production-code edit this increment makes is a comment, not a new
statement) — the actual ratchet run matched exactly on both build
tags: go_default 8904/14646 (5742 uncovered) → 8914/14646 (5732
uncovered) IMPROVED; go_duckdb 8982/14744 (5762 uncovered) →
8992/14744 (5752 uncovered) IMPROVED. No flake this run. `go build
./...`, `gofmt -l`, `go vet`, `go test ./internal/pdfmeta/... -race`,
and the full `go test ./...` module suite pass; `internal/pdfmeta` at
140 tests, confirmed via `grep -c "^func Test"`.

**Not started:** `InjectOutputIntent`'s five real branches enumerated
above (empty-`pdfBytes`, empty-`iccProfile`, 3 error-wraps), and a
decision on whether to fix the `math.MaxInt` overflow above (a cheap
guard, same shape as `writeClassicXrefEntry`'s existing offset/gen
checks, but gated behind an input this app's own generation path
never produces) — the natural next bounded `internal/pdfmeta`
increment. After that, `internal/pdfmeta` is down to
disclosed-permanent-floor guards only (the two offset-only xref
guards named above, the swap guard just disclosed, `InjectPAdESSignature`'s
five from 4b, `ensureBinaryHeaderComment`'s single disclosed gap
[`replaceStartxrefValue`'s error-propagation branch, unreachable
because both it and `findLastStartxref` independently locate the same
literal `startxref` text — confirmed as the *only* uncovered span in
the function via the coverage profile, not just cited from 3a's
prose], and `icc.go`'s single disclosed `//go:embed`-makes-it-always-
populated gap, likewise confirmed as its only uncovered span) — that
next increment is plausibly the package's actual practical ceiling, a
claim this entry makes narrower than 4b's now-corrected one by naming
the exact remaining branches rather than asserting "final." Then the
rest of
Phase 2 (remaining Go completion-tier packages at 70–99%: `fonts`
81.6%, `rfc3161` 81.6%, `signing` 81.6%, `sigma/service` 85.0%, and
others),
Phase 3 (Go construction tier: `charts/pdfrender`, `documents`, `update`,
`db`, `agile`, `export`, `money`, `scripts`, `tools/update-manifest`),
Phase 4 (root/App layer, 48.2%), Phase 5 (remaining frontend pure-logic
modules), Phase 6 (frontend component coverage — the dominant cost,
estimated 80–280 more interaction-tested component tests across ~233
files from measured per-test statement rates, likely 100+ hours spanning
many sessions), Phase 7 (convert the ratchet to a hard 100% floor once
reached).

**Also flagged, not yet actioned:** a modernization/lint sweep of
`internal/pdfmeta/pdfmeta.go` surfaced by editor diagnostics during
increment 2b's mutation testing, unrelated to coverage and pre-existing
(not introduced by 2b) — 10 `fmtappendf` findings (`[]byte(fmt.Sprintf(...))`
sites that could use `fmt.Appendf` directly), 1 `rangeint` finding (a
`for i := 0; i < n; i++` loop modernizable to `range n`), and 1
`stringsbuilder` finding (a `string +=` in a loop). One `stringscut`
finding in `pdfmeta_test.go` (`bytes.IndexByte` in
`TestMakePDFA3PreservesMetadataAndOutputIntentInLatestCatalog`, also
pre-existing) belongs with it. A second `stringscut` finding introduced
by 2b itself was fixed immediately rather than deferred (see 2b's own
entry above). Low priority — style-only, no behavior change — but
worth a dedicated small pass rather than silent accumulation.

## What Is Not on the Roadmap

Anything that violates the principles in `VISION.md`. Specifically: mandatory
cloud sync, server-side AI processing, proprietary export formats, and
real-time collaborative editing over a network are not planned.
