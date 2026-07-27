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
- The leveling/preview **action decision-logic** was extracted the same way: `leveling_messages.ts` (pure status/warning/preview builders + `clearWorkSegments`) is unit-tested and consumed by both the CPM and Gantt editors. This verifies the *logic* of the leveling actions. The async handler orchestration is now also covered: `GanttEditor.test.ts` mounts the real editor against a mocked `window.go.main.App` (+ stubbed autosave), clicks the Preview/Level buttons, and asserts the bridge result flows through the handler to the status DOM — including that `Level (split)` calls the bridge with `allowSplitting=true` and that a bridge rejection surfaces as an error message rather than throwing. Unifying the two editors' preview builders made the Gantt preview/level messages gain the CPM's `“+N more”` truncation — a small consistency improvement. _Still compile-only:_ the equivalent CPMEditor handlers (it wraps the heavier `LayeredEditorShell`, so a mount test would need much more bridge scaffolding).

## Other notable decisions (from `session-notes.md`)

- **2026-06-15** — Wails main package moved to the repo root (required by `wails build`); `cmd/` directory retired.
- **2026-06-15** — Per-project unique-ID subfolders adopted, with legacy flat-file layout preserved for backward compatibility (`projectPathFor` accepts both).
- **2026-06-20** — `CreateAccount` duplicate check changed from case-sensitive to `lower(username) = lower(?)` after an APFS case-insensitive collision let two accounts share one data directory.
- **2026-06-22** — Wails v2.9.2 → v2.12.0 upgrade, pulling `golang.org/x/crypto`/`x/net`/`x/sys` security-hygiene bumps via `go mod tidy`.
- **2026-07-24** — The Go `internal/calendar` policy catalogue became the
  authoritative Launchpad source for business-calendar jurisdictions and IANA
  time zones. Project creation validates the pair at the Wails boundary so
  schedules and time-series charts cannot start with contradictory settings.
- **2026-07-24** — Dashboard chart creation now uses one searchable,
  engine-filtered catalog sourced from the Go chart registry. It replaces the
  duplicated full card grid and chart-template reference grid; empty projects
  open the catalog automatically while established workspaces keep it compact.
- **2026-07-24** — Dashboard document creation now uses the same registry-first
  discovery pattern: one searchable, lifecycle-filtered controlled-document
  index replaces the duplicated Charter card and 25-template grid. Combined
  Report remains separate because it composes existing documents.
- **2026-07-25** — `go.mod` is the source of truth for the Wails runtime and
  CLI version. `make wails-version` now rejects drift in CI/release workflows,
  installation guidance, developer instructions, and build-failure recovery
  hints while allowing dated upgrade history to retain earlier versions.
  Release and macOS package gates additionally require the installed CLI to
  match through `make wails-cli-version`.
- **2026-07-25** — RFC 3161 timestamping is split into protocol validation and
  PAdES mutation. `internal/rfc3161` now owns nonce-bearing SHA-256 requests,
  HTTPS/redirect/size policy, response binding, TSA certificate constraints,
  and optional trust-root verification. `internal/crypto` hashes the existing
  signer signature value and embeds validated token DER as the canonical
  unsigned CMS attribute without changing the signature. The PDF signature
  slot reserves 32 KiB for TSA chains.
- **2026-07-25** — Project Settings now persists an opt-in, credential-free
  HTTPS TSA endpoint plus optional policy OID and PEM trust-root path.
  `internal/signing.ApplyPAdES` is the shared document and combined-report
  signing pipeline. Enabled timestamping is fail-closed, and audit statuses
  distinguish Baseline B, Baseline T with unevaluated trust, and Baseline T
  verified against the configured root.
- **2026-07-25** — Retired the legacy `%%PMForgeCMSSignature` PDF-comment
  fallback. Archive PDF signing and `documents.RenderSigned` now delegate to
  `signing.ApplyPAdES`; every successful PAdES path must contain a real `/Sig`
  dictionary and `/ByteRange`, while failures return no PDF bytes.
- **2026-07-26** — Application PAdES-T export tests use a private, per-call
  `padesExportRuntime` instead of mutable `App` fields or package globals.
  Document and combined-report tests now verify timestamped CMS structure,
  0600 file output, and verified versus unevaluated TSA audit statuses without
  contacting a live service. RFC 3161 fixture certificates cover both their
  deterministic TSTInfo instant and the current CMS signing clock so the suite
  cannot expire as wall time advances.
- **2026-07-26** — Default external PAdES validation always regenerates its
  sample under the shared lock; explicit PDF arguments remain non-mutating.
  Evidence reports identify generated versus supplied input and bind the
  checkout, scripts, PDF, CMS, and signed ByteRange bytes with provenance and
  SHA-256 hashes.
- **2026-07-26** — Configuration formats follow their consumers rather than a
  repository-wide preference. `make config-check` parses and structurally
  validates the tracked YAML/TOML inventory, rejects ambiguous YAML and
  unclassified additions, and prevents the retired GitLab pipeline from
  returning alongside the authoritative GitHub Actions workflows.
- **2026-07-26** — Native tag packaging uses one immutable installer-tool
  record: nFPM v2.47.0 and Chocolatey NSIS 3.12.0. The workflow verifies the
  installed tool provenance, while macOS stays on its built-in `hdiutil`
  release path instead of installing the optional `create-dmg`. Isolated drift
  fixtures and release-scope wiring guard the contract.
- **2026-07-26** — Published release history follows live GitHub evidence.
  Concrete candidate tags and candidate release-note files must match
  `docs/published-release-tags.txt`; isolated regressions and the release-scope
  gate reject unverified claims. The record now contains `v0.9.1-alpha` and
  the packaged `v1.1.0-alpha.1` prerelease published on 2026-07-27.
- **2026-07-26** — Windows packaging now consumes PMForge-owned NSIS/resource
  templates while Wails regenerates only derived macros and assets. The native
  command enables DuckDB and verifies binary linkage before upload; isolated
  mutations plus an NSIS 3 fixture compile guard branding, uninstall safety,
  and workflow ordering.
- **2026-07-27** — GitHub publication classification is explicit: a validated
  tag with a SemVer suffix supplies `--prerelease`, while the clean product
  version does not. Isolated helper and workflow mutations prevent an alpha or
  RC from being presented as GA. Release preflight also installs and discovers
  `ripgrep` explicitly before source contracts that depend on it, and blocking
  race-test output remains visible in hosted logs. The post-Dependabot
  lockfile pins `brace-expansion` 5.0.8 after a clean high-severity npm audit.
  Browser storage code now uses `window.localStorage`, while Vitest workers
  disable Node 26's process-wide Web Storage and the stability gate simulates
  that conflict on older runtimes. PDF/A helper tests explicitly force their
  fake CLI phase so a host-installed Docker cannot escape the hermetic fixture;
  blocking PDF/A diagnostics remain visible in release logs.
- **2026-07-27** — Strict PDF/A builds carry a deterministic Source Sans 3
  baseline instead of relying on ignored output from `make fonts`. The four
  regular/bold/italic/bold-italic faces are tracked under OFL-1.1 and bound to
  reviewed Adobe upstream bytes by `make required-font-assets`; the rest of
  the font catalog remains optional and fetch-on-demand.
- **2026-07-27** — Windows release setup verifies pinned NSIS at Chocolatey's
  explicit installation path and publishes that path through `GITHUB_PATH`.
  This avoids assuming an already-running Git Bash process sees Chocolatey's
  PATH update while retaining the exact 3.12.0 package-record check.
- **2026-07-27** — Native package workflows consume only the tracked Source
  Sans 3 baseline; optional font downloads no longer make packaged features
  depend on transient CDN success. The macOS job reclaims disposable Go module,
  build, and npm caches after its DuckDB app build because the hosted runner
  otherwise leaves too little scratch space for `hdiutil`.
- **2026-07-27** — The hermetic NSIS template compile uses non-empty fixture
  inputs and translates the dummy application to an absolute Windows path with
  `cygpath` when native `makensis.exe` runs under Git Bash. Homebrew NSIS keeps
  the script-relative POSIX path.
- **2026-07-27** — Release workflow run `30302650798` passed its full preflight,
  native Windows Wails/CGO plus NSIS build, macOS arm64 DMG build, Linux
  `.deb`/`.rpm` build, artifact upload, and prerelease publication. A transient
  Google Chrome apt mirror mismatch was cleared by rerunning only the failed
  Linux leg. GitHub published four assets for `v1.1.0-alpha.1`; real-machine
  install and lifecycle testing remain separate evidence.
- **2026-07-27** — The published alpha DMG passed checksum, `hdiutil`, strict
  ad-hoc `codesign`, architecture, and clean first-launch checks on an M4 Mac.
  With an isolated empty data root, the UI reported no configured
  administrator and exposed the first-user administrator checkbox. No account
  was submitted and existing PMForge data was not touched.

## Open / deferred (not yet decided or implemented)

- **Advanced Resource Levelling** (ROADMAP Phase 1) — _horizon slice done end-to-end._ The leveling horizon is the exported `DefaultLevelingHorizon`, configurable per schedule via `LevelResourcesWithOptions(..., LevelingOptions{Horizon})`, which returns the `ErrLevelingHorizonExceeded` sentinel plus `LevelingResult.UnplacedTaskIDs` instead of silently capping (`internal/kernel/resources.go`, 2026-07-02). The production path is wired: `App.LevelChartResources` returns a `LevelResult` (pinned + unplaceable IDs/labels), routing a cycle as a hard error but a horizon overflow as a non-fatal warning; the CPM editor shows a dismissible “still overallocated” warning (2026-07-03). EDF/LTF leveling heuristics added as a `LevelingOptions.Strategy` selector, wired through `App.LevelChartResources(chartID, strategy)` and a CPM-editor dropdown (LTF default preserves prior behaviour, 2026-07-03). Priority-override (`LevelingOptions.PriorityCritical`) protects the critical path, wired as `LevelChartResources(chartID, strategy, priorityCritical)` + a “Protect critical” checkbox. Partial-assignment splitting (`LevelingOptions.AllowSplitting` + `Task.WorkDays`, with `ResourceUsage`/`DetectOverallocations` made split-aware) interrupts tasks across non-contiguous days; surfaced read-only via `App.PreviewSplitLeveling` + a “Preview splitting” button (2026-07-04). Split schedules are now also **persistable and renderable**: `dag.WorkSegment` + `LayeredNode.WorkSegments` (task-relative offset runs) store a split, `App.LevelChartResources(chartID, strategy, priorityCritical, allowSplitting)` persists them, the Gantt layout emits absolute segments, and `GanttEditor` draws interrupted bars via a “Level (split)” action (2026-07-04). The PDF Gantt renderer (`pdfrender/gantt.go`) draws the same interrupted bars so exports match the screen (2026-07-04). EVM now maps those persisted relative segments into `kernel.Task.PlannedWorkSegments`; planned value accrues only inside worked segments and plateaus through idle gaps, with invalid overlapping segments rejected at the chart adapter and current-chart parse errors propagated instead of silently falling back to stale legacy tasks (2026-07-25). **RICE-144 Advanced Resource Levelling is complete.**
- **Risk/Issue/Opportunity workflow + Risk Matrix chart** (22nd chart kind) — _foundation and explicit document sync complete 2026-07-25._ `matrix.RiskMatrixDocument` stores risks, active issues, and opportunities with probability/impact, owner, status, mitigation, and linked task. The backend emits a validated canonical 5×5 grid; the dedicated Svelte editor and vector PDF renderer consume that layout, and Risk Register documents can link the chart into combined reports. `SyncRiskRegisterToMatrix` validates persisted rows before replacing linked chart items and preserves chart metadata/audit history. Synchronization is deliberately one-way and only runs from the Risk Register editor's **Refresh Risk Matrix** action; there are no hidden cross-artifact writes.
- **RFC 3161 PAdES timestamping** — protocol client, CMS embedding,
  DSS-classified fixture, project settings, fail-closed document and
  combined-report export wiring, and audit statuses are complete 2026-07-25.
  Remaining release evidence is a real trusted signer/TSA chain plus Acrobat
  validation.
- **Portfolio rollup SPI/CPI** — _complete 2026-07-25._ `RunPortfolioAnalytics` separates committed estimates from schedule AC, computes eligible project EVM in the Go kernel at a normalized UTC status date, and lets in-memory DuckDB aggregate exact minor units. Portfolio SPI/CPI are weighted (`ΣEV/ΣPV`, `ΣEV/ΣAC`), and included/excluded counts expose incomplete schedule evidence instead of substituting zero.
- **RPM Fedora runtime** — built on Ubuntu, cross-distro behavior unverified on a real Fedora box.
- **Windows native installer execution** — source templates, native
  Wails/CGO compilation, embedded DuckDB verification, and NSIS packaging pass
  on GitHub's Windows runner. Install/launch, first-run account creation, and
  data-preserving uninstall still require a Windows test machine.
- **PAdES trusted-chain / Acrobat validation** — blocked on a real trusted signing source; `make check-pades-trusted` reports "not configured" in the interim.
