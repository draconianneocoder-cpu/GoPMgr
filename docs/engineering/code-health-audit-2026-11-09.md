<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Dead and inefficient code audit, 2026-11-09

Audited tree: `main` plus PR #193 (dependency updates and the scanner and
coverage repairs). This records what was found, how, and what was done about
each finding.

## Method

| Question | Tool | Scope |
|---|---|---|
| Go functions no program reaches | `golang.org/x/tools/cmd/deadcode` v0.45.0, `-tags production` | `.` and `./tools/...` for darwin and windows on macOS, linux on the Ubuntu 26.04 host; repeated with `-test` |
| Wails-bound `App` methods the frontend never calls (deadcode cannot see reflection-bound methods) | `code-map/app-api.json` compared with `frontend/src` call sites, tests excluded | all 209 methods |
| Frontend modules and exports nothing uses | import and identifier scan of `frontend/src`, tests excluded | `.ts` and `.svelte` |
| Unexported dead code | golangci-lint `unused`, already part of CI | whole module |
| Inefficient code | one-off golangci-lint run with `gocritic` (performance tag), `prealloc` and `perfsprint`; a scan for SQL calls inside loops | `.`, `./internal/...`, `./tools/...` |
| Broken documentation | relative-link and `make` target checks over tracked Markdown | `docs/`, `AGENTS.md`, `README.md` |

`deadcode` reported the same 25 functions on all three platforms, and none
with `-test`: every candidate is reached only from tests.

## Findings

### Removed: code no product path uses

| Code | What it was | Action |
|---|---|---|
| `crypto.Signer.SignPDFHash` | A raw PKCS#1 v1.5 signature over SHA-256, kept "for callers (or future code paths)". All signing goes through `SignPDFCMS`. | Removed; its tests now exercise `SignPDFCMS`. |
| `fonts.Manager.Register` | A one-line wrapper for `RegisterAs(r, family, "")`, called only by a test. | Removed; the test calls `RegisterAs`. |
| `pdfmeta.HasDefaultICC` | Reported whether the sRGB profile was embedded; only a test called it. | Removed; the test checks `DefaultICCProfile` directly. |
| `documents.SetFontApplier` | A test-only hook exported from production code. | Moved to `export_test.go`, so it is no longer part of the package API. |
| `App.UpdateProjectIndustry` (frontend API) | No frontend caller since `28caa1e`; Project Settings saves through `UpdateProjectMeta`. It silently reset a mismatched time zone, where `UpdateProjectMeta` rejects one. | Removed with its bindings; the frontend API is now 208 methods. |

### Kept: staged work, not dead code

`internal/wbs` and `internal/db/wbs_store.go` (19 of the 25 `deadcode`
results) are the owner-approved, isolated WBS model and storage foundation.
They are deliberately not wired to a production caller yet; see
[WBS model validation](../testing/wbs-model-validation-2026-10-13.md) and
[WBS storage boundary](../testing/wbs-storage-boundary-2026-10-14.md).
`fonts.newManagerWithBundledFS` is a documented test seam.

### Frontend

- `src/lib/terminology.ts` (methodology-specific words such as "user story"
  for Scrum) has existed since the initial commit but no component imports
  it. The Help screen, the user guide and the coverage ledger said the
  classification fields drive terminology across the GUI, which was not
  true. See [the decision recorded below](#terminology-decision).
- Other unreferenced exports are types and helpers used inside their own
  module and exported for tests, or `FONT_MONO`, which tests keep in step
  with the CSS token. No action.
- Five `__test_harness__` components are test-only by design.
- `tslib` was a development dependency from the original project template.
  No code or TypeScript setting uses it, and the one package that needs it
  (Tailwind's WebAssembly fallback) bundles its own copy, so it was removed.
  `@vitest/coverage-v8` and `typescript` are used without an import (by
  `vitest --coverage` and the type checkers).
- Every script under `scripts/` and every `tools/` program is referenced by
  the Makefile, a workflow, another script or the documentation.

### Corrected: a defect found while auditing

`desktop.runHeadless` (the `--check`, `--repair`, `--vacuum`, `--export*`
and `--stats` command-line mode) deferred `Database.Close` and then left
through `log.Fatalf` or `os.Exit(1)`. Deferred calls do not run on exit, so
every error path and a `--check` that found corruption exited without
closing the project database. Only the `--repair` branch closed it first. It
now returns an exit status, and `Main` exits after the deferred close has
run. A test checks each status and that a read-write run leaves no `-wal`
file; seeding the old exits back in, or dropping the close, fails it.

### Inefficient code

- The timestamp retrofit that runs on every database open wrote each legacy
  row in its own autocommit transaction: one commit and disk sync per row on
  the first open of a large legacy project. Each column's updates now run in
  one transaction.
- Three loops built strings with `+=`. Their inputs are tiny (one row's
  keys, a file name, a keyword list), so the cost was negligible; they now
  use `strings` helpers, and a comment claiming the loop avoided importing
  `strings` is gone.
- The scan for SQL written directly inside a loop found only fixed
  migration lists, schema objects and a handful of accounts inside one
  transaction. It does not see a loop that calls a `Database` method which
  queries once per item, so N+1 patterns behind such calls were not
  audited.
- The remaining linter output (176 large-value copies, 190 `fmt` calls a
  cheaper function could replace) was not changed: these are micro-costs in
  a desktop app, and no profiling was done to show any of them on a hot
  path.

### Documentation

- Fixed two broken links: `docs/ERROR_HANDLING.md` to the App API map, and
  the 2026-09-01 Cornucopia notes to the moved `internal/desktop` files.
- Left as recorded: links inside the frozen ADR-010 v2.1 snapshot, and
  `make` targets named only in superseded plans or closed reviews.

## Terminology decision

The owner chose to wire `terminology.ts` into the interface rather than
delete it, through ADR-008's catalog. Before building it, the owner asked how
other tools handle methodology wording. Azure DevOps, Jira, Microsoft
Project, Primavera P6, monday.com and ClickUp never rename schedule rows by
methodology; only Azure DevOps varies wording per process, and only in the
work-item layer, from a fixed, translated set (Story Points, Effort, Size).
Applied as written, the table would have labelled Six Sigma schedule rows
"measurement" and the Scrum milestone checkbox "sprint goal". The owner chose
that model, and ADR-008 slice 3 (the same day) implements it: the work-item
editor's estimate field reads "Story points" for Scrum and Scrumban, schedule
editors keep "task", and `terminology.ts` was replaced by
`i18n/methodology.ts`.

## Second pass: flaky tests, inefficiency and data integrity

A second pass the same day looked for flaky tests and the gaps the first
pass left open. Priorities: **P1** fix now (correctness or data integrity),
**P2** fix soon or needs an owner decision, **P3** record only.

### Flaky tests: none found

| Check | Result |
|---|---|
| CI history, 200 runs (2026-10-11 to 2026-11-09) | 149 passed, 36 cancelled, 15 failed. No commit both passed and failed. Every failure has a deterministic cause: generated notices left stale by a dependency bump, coverage-ledger drift, DataFusion spike files that config-check did not classify (since fixed), a Dependabot Wails bump that did not match the documented pins, a Go standard-library advisory (fixed by a toolchain update), or a real test failure on a feature branch. The two re-runs followed a cancelled job and a repeated real failure. |
| Go: every package three times with `-race -shuffle=on` | 51 packages pass; no data race. This run used `main` at `ae76111`: the checkout was switched to `main` outside this work while it ran. The packages this branch changes were then rerun on the branch, twice with `-race -shuffle=on` and once at UTC+14, and passed. |
| Frontend: five shuffled runs with different seeds, under load (on `main`), then three more on this branch | 742 of 742 pass each time |
| Time zones UTC+14 and UTC−10 (both suites); German locale (frontend) | all pass |
| Static patterns | No fixed network ports; no frontend test that waits on real timers or reads the clock without fake timers; no test that swaps a process-wide value while running in parallel; no date-relative assertion on a hard-coded date |
| Time bombs | The committed test certificates expire in August 2036 (**P3**: renew before then) |

The only sleep-based timing assertions are in the ADR-010 transport spike
(`spikes/adr010-g0-transport`), which CI runs only for pull requests that
touch it (**P3**).

### Fixed

| Priority | Finding | Fix |
|---|---|---|
| P1 | The agile store saved any string as a work-item type or priority, or as a sprint status; the columns are plain TEXT and nothing else checked. A misbehaving caller could store values the board, filters and metrics do not recognize. | `SaveWorkItem` and `SaveSprint` refuse unsupported values with `ErrWorkItemType`, `ErrWorkItemPriority` and `ErrSprintStatus`. State stays free-form because it names a board column. A test covers every supported value and each refusal; removing any check fails it. Existing records stay saveable: these value sets have not changed since they were added (frontend option lists, backend constants and seeds), and only the store writes these columns. |
| P2 | Password hashing made the account tests slow: `internal/users` took about 63 s (287 tests, about 0.2 s each, run one at a time) and about 400 s under the race detector. Each account hashes its password with the production Argon2id cost (64 MiB, three passes), and each key wrap and recovery code adds a one-pass 64 MiB derivation. Measured: creating an account costs 44 ms, the shared escrow setup 54 ms, and issuing recovery codes 300 ms. `internal/desktop` has the same problem and is not yet addressed. | No production change, and the test-only cost override was not adopted. First, 254 of the 287 tests run in parallel; the 33 that replace a package hook, `crypto/rand.Reader` or the environment stay serial, enforced by `requireSerial`. Then, the owner's choice: `TestMain` builds five shared setups once and the helpers hand each test a copy (`fixture_test.go`). Totals: 63 s to 33 s, 50 s to 40 s run one at a time, and 397 s to 231 s under the race detector. Argon2 is limited by memory bandwidth, and the remaining time is recovery-code tests, where hashing each code is what they verify. Peak memory: 1.7 GB for a normal run (what CI's `make verify` runs) and 5.4 GB under the race detector, which only `make race` uses, one package at a time. |

### Recorded, not changed

| Priority | Finding | Why not now |
|---|---|---|
| P3 | Activating a sprint is two saves from the GUI (complete the active sprint, then activate the new one). If the second fails, no sprint is active. | Recoverable by retrying; an atomic "activate" store method would remove it. |
| P3 | Seven exported enum members are never referenced (for example `PrioUrgent`, `AssignInformed`). | They complete value sets that the frontend sends as strings, and the store now validates against them. |

Also checked: no `Database` query method runs once per item inside a loop
over one database. The project list and portfolio loops open each project
file once and run one to three queries on it, which is inherent to listing
projects. All 107 interface catalog messages are used. svelte-check, which
reports unused CSS, passes with no warnings.

## Third pass: security scans

A third pass the same day, on `main` after PR #194, ran the security
scanners the first two passes had not.

| Check | Result |
|---|---|
| `govulncheck -tags production . ./internal/... ./tools/...` | No known vulnerabilities |
| `npm audit --omit=dev` | No advisories in production packages |
| `npm audit` (development tools) | 10 low or moderate advisories, all in ESLint's `minimatch` and the build-time `deepmerge` (**P3**). npm's suggested fix downgrades `@typescript-eslint`; do not run `npm audit fix --force`. Dependabot updates will clear them. |
| `gosec -tags production ./...`, excluding the gitignored `.tmp/` scratch tree, spikes and tests | No actionable finding. Decompression (G110) in backup restore is bounded: each entry's declared size and the archive total are capped first, and Go's zip reader fails with `ErrFormat` once an entry decodes past its declared size, which `TestRestoreArchivalBundleRejectsUnderstatedDecodedSize` proves. The SQL concatenations (G202) insert fixed column lists with bound values. The integer conversions (G115) in `money` and `analytics` follow explicit overflow checks. The printer subprocess (G204) uses a fixed path and a validated printer name. The rest are in build tools or vendored SQLCipher. |

## Fourth pass: performance (2026-11-10)

Existing benchmarks show overallocation detection is already linear and
fast (100,000 tasks in about 107 ms). The Monte Carlo schedule-risk
simulation was not.

### Fixed

| Priority | Finding | Fix |
|---|---|---|
| P2 | Every Monte Carlo iteration cloned every task, re-sorted the unchanged network, re-derived each task's links four times (each a fresh slice and map), and returned its results in two new maps; the percentiles then copied and sorted each task's samples three times, and the finish CDF sorted the finishes 21 times. 500 tasks at 10,000 iterations (the interface's maximum) took 1.5 s and allocated 7.2 GB. | `CalculateCPM` is split into a plan (topological order and resolved links, built once) and a run (the passes, unchanged in substance). Each worker clones the network and builds the plan once, and iterations write into preallocated slices; each sample list is sorted once. Medians of five runs: 42.9 ms to 5.7 ms (100 tasks × 1,000), 165 ms to 25.9 ms (500 × 1,000), and 1.50 s to 0.29 s with 7.2 GB to 49 MB allocated (500 × 10,000). One schedule pass at 5,000 tasks: 3.25 ms to 2.87 ms. Results are bit-identical to verbatim copies of the old code on 300 seeded random networks and with any worker count. |

### Recorded, not changed

| Priority | Finding | Why not now |
|---|---|---|
| P3 | Resource leveling (`kernel/resources.go`) calls `effectiveLinks` inside its loops and re-runs `CalculateCPM`. | Not measured as a hot path; the plan type now gives it a way to reuse sorted links if a benchmark shows the need. |
| P3 | A simulation keeps every task's samples for every iteration (tasks × iterations × 8 bytes): 40 MB at the interface's 10,000-iteration cap for 500 tasks, but 400 MB at the backend's 100,000-iteration limit. | Unchanged by this work; percentiles need the samples. A streaming quantile estimate would change results, so it needs a decision. |
| P3 | The frontend was not profiled in this pass. | Out of scope; candidates need measurements in the packaged app. |

## Fifth pass: multi-agent security scan (2026-11-10)

The code-modernization harden scan ran 34 agents: one finder per
vulnerability class (injection, authentication, secrets, dependencies,
input handling), a refuter per finding, and a second judge for High
findings. 28 findings became 22 after refutation (1 High, 5 Medium,
16 Low); no credentials were found. The full report stays out of the
repository.

| Priority | Finding | Status |
|---|---|---|
| High | The vendored SQLCipher 4.4.2 used SQLite 3.33.0 (2020), with memory-corruption CVEs reachable from a crafted database file (for example CVE-2021-20227 and CVE-2025-6965). `govulncheck` cannot see C code built through cgo. | **Fixed** 2026-11-10 at the owner's request: upgraded to SQLCipher 4.20.0 (SQLite 3.54.1), the newest stable release; see [the upgrade evidence](../testing/sqlcipher-4.20-upgrade.md). Old files need no migration. The Windows build is unverified. Still open as hardening: `PRAGMA trusted_schema=OFF` and `SQLITE_DBCONFIG_DEFENSIVE` on every connection, and a check that flags the vendored SQLite version against new advisories. |
| Medium | A restored `.pmba` backup's own SQLite schema (planted triggers or views) is trusted: the archive's SHA-256 digests come from its own manifest, so they authenticate nothing. | **Fixed** 2026-11-10: restore opens the file read-only first and refuses any trigger, view or virtual table (GoPMgr has never created one in a project file); with the check removed, a planted trigger deleted the restored project row. Restore is the only path that opens a project file from outside the account for writing (`OpenProject` takes only encrypted files in the user's own folder; the CLI `--project` mode opens a file the user names). Limits: SQLite still parses the file's schema, so the engine's parser vulnerabilities (the High row) stay reachable; and tables the archive defines itself are still trusted, so a crafted table with cascading foreign keys or CHECK constraints passes `CREATE TABLE IF NOT EXISTS` (closing that needs a full schema comparison across versions). |
| Medium | Nothing capped a schedule's length: an imported or typed duration of about 10^9 days made `AnchorSchedule` and resource profiles allocate memory in proportion to it. | **Fixed** 2026-11-10 by the owner's decision of a 50-year maximum (`kernel.MaxScheduleDays`, 18,250 working days), enforced in the scheduler. `ScheduleCPM` refuses any duration, lag or estimate beyond it, any value that is not a finite number, and any schedule that adds up past it; saving a chart and importing a file fail with a message naming the task. The functions that size memory from dates (`AnchorSchedule`, resource profiles, leveling) also refuse out-of-range dates themselves, because baselines hand them tasks that never went through CPM. A chart stored before the limit still opens for editing. Not covered: the import's assignment units and percent-complete are still unbounded (neither sizes memory). |
| Medium | The project's stored certificate path is read into backups with no regular-file or size checks. | Open. |
| Medium | Schedule CSV export wrote the task ID without formula neutralization; an MSPDI import keeps each task's UID as its ID. | **Fixed** 2026-11-10, with a test that fails on the old code. |
| Low | 16 findings: recovery codes left on the clipboard, a plaintext `.pre-encryption.bak` kept after encryption, no sign-in attempt limiting, sign-in timing that reveals whether a username exists, the SQLCipher key kept in the connection's DSN, development-only npm advisories, and others. | Recorded for prioritisation. |

## Re-running the audit

```sh
GOBIN="$PWD/.tmp/deadcode" go install golang.org/x/tools/cmd/deadcode@v0.45.0
.tmp/deadcode/deadcode -tags production . ./tools/...
GOOS=windows .tmp/deadcode/deadcode -tags production . ./tools/...
.tmp/deadcode/deadcode -test -tags production . ./internal/... ./tools/...
```

Flake checks from the second pass:

```sh
python3 scripts/native-go-env.py go -- go test -race -shuffle=on -count=3 . ./internal/... ./tools/...
TZ=Pacific/Kiritimati python3 scripts/native-go-env.py go -- go test -count=1 . ./internal/... ./tools/...
(cd frontend && npx vitest run --sequence.shuffle --sequence.seed=1001)
gh run list --workflow ci.yml --limit 200   # look for one commit that both passed and failed
```

Run the linux pass on a Linux host; the cgo GTK and WebKit packages do not
load from macOS. `deadcode` cannot see methods Wails binds by reflection, so
compare `code-map/app-api.json` with the frontend's `desktopApi.` calls too.
