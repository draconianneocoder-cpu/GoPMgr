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
note below), and a Mobile Companion web view.

**Local Collaboration note:** GoPMgr must acquire an exclusive SQLite lock
before opening a project and must release it on close. The sync folder must
not be transferred by Syncthing while GoPMgr holds the lock. The
recommended user workflow is: close the project in GoPMgr, allow Syncthing
to sync, reopen on the other machine. GoPMgr will detect and warn on
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

**Not started:** the rest of Phase 2 (remaining Go completion-tier
packages at 70–99%),
Phase 3 (Go construction tier: `charts/pdfrender`, `documents`, `update`,
`db`, `agile`, `export`, `money`, `scripts`, `tools/update-manifest`),
Phase 4 (root/App layer, 48.2%), Phase 5 (remaining frontend pure-logic
modules), Phase 6 (frontend component coverage — the dominant cost,
estimated 80–280 more interaction-tested component tests across ~233
files from measured per-test statement rates, likely 100+ hours spanning
many sessions), Phase 7 (convert the ratchet to a hard 100% floor once
reached).

## What Is Not on the Roadmap

Anything that violates the principles in `VISION.md`. Specifically: mandatory
cloud sync, server-side AI processing, proprietary export formats, and
real-time collaborative editing over a network are not planned.
