# GoPMgr Test Coverage Ledger

SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later

## Purpose

This is an itemized inventory of every test file in the repository: what
production code it exercises, how (the testing technique used), and why
that coverage exists (the regression class it guards against). It answers
"what tests do we have and what are they actually for", as a companion to:

- **`ROADMAP.md`**'s "100% Full-Repo Test Coverage" section — the ongoing
  effort to *close gaps*, phase by phase, with cost estimates and
  decisions.
- **`DEVELOPER_HANDBOOK.md`**'s dated entries — the *why* behind
  non-obvious testing decisions (fault-injection patterns, break-verify
  discipline, scope exclusions) as they were made.

This document is the *what exists*, itemized. It is a snapshot as of
2026-08-05 and will drift as tests are added — update the relevant
section whenever a test file is added, removed, or its purpose changes
materially. It is not auto-generated; treat stale entries as bugs the
same way stale code comments are treated elsewhere in this repo.

**Methodology and confidence**: entries were written from test function
names, the production code each test file targets, and package doc
comments — not from executing every test individually and diffing its
assertions against this table row by row. A sample was verified directly
against source during the `a0360b5` adversarial review and held up, but
any single row is more likely to be stale or imprecise than the tests
themselves; treat a row as correctable, not authoritative, if it
disagrees with the code.

**Numbers:** package percentages below are `go test -cover` statement
coverage under the default build (no `-tags duckdb`), current as of the
`coverage-baseline.json` this session ended with. Reproduce with
`scripts/coverage-go.sh default` / `scripts/coverage-go.sh duckdb` (Go) or
`npm --prefix frontend run test:coverage` (frontend) — see
`DEVELOPER_HANDBOOK.md`'s "100% coverage: scope and exclusions" entry for
what each of those excludes and why.

**Technique legend** (used in the "How" column throughout):

| Tag | Meaning |
| --- | --- |
| unit | Direct call into a pure function; no I/O, no fixtures. |
| table-driven | One test function, many input/expected-output cases in a loop or `t.Run` subtests. |
| fixture/golden | Builds a real file/document/DB and asserts on its structure or byte content. |
| fault-injection | Forces an otherwise-unreachable error path — a SQLite `RAISE(ABORT)` trigger, a broken loader/reader, a zeroed entropy source — to prove error propagation. |
| round-trip | Encode then decode (or encrypt/decrypt, wrap/unwrap) and assert the result matches the input. |
| integration | Exercises multiple real collaborators together (e.g. a real `*db.Database` + `*agile.Store` + `Seeder`), not a single function in isolation. |
| e2e | End-to-end: builds a realistic multi-step scenario (an install, a migration, a full export pipeline) and asserts on the final observable outcome. |
| property | Asserts an invariant that must hold across many/all inputs (monotonicity, idempotence, determinism) rather than one fixed expected value. |

---

## Root package (`gopmgr`, package `main`) — 48.4%

The Wails desktop app entry point, App struct (bound to the frontend via
`window.go.main.App`), and CLI/headless dispatch. Most of this package's
~90 test files are integration tests against a real `*App` wired to a
real (temp-dir) `*users.Store` and `*db.Database` — this package has no
network or GUI dependency to mock, so tests build real state and assert
on real outcomes rather than mocking collaborators.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `main_test.go` | 10 | `logTail`, `GenerateBugReport`, `buildAppMenu` (darwin + non-darwin), `buildAppOptions`, `beforeClose` | unit, fixture, table-driven | Diagnostic-report generation must include the right log tail without unbounded growth; `buildAppMenu`'s two tests (added 2026-08-05, see `DEVELOPER_HANDBOOK.md`) pin the platform-specific menu structure — App/Edit/Window role menus vs. GoPMgr's own Window submenu, and where Quit lives — via an injectable `goos` parameter so both branches run on any single host. |
| `migration_e2e_test.go` | 1 | `users.DefaultRootDir`, `users.MigrateLegacyRoot`, `App.Login`, `App.ListProjects` | e2e | Proves a pre-rename ("PMForge") install is fully *usable* after migration — not just that files got copied — including deleting the old root first to simulate real post-migration cleanup. Guards the `f1a5501` data-loss class of bug (stale `DataDir` pointing at a deleted root). |
| `admin_recovery_test.go`, `admin_test.go` | 9 | `App.AdminIssueRecoveryCodes`, `App.CreateAccount`, `App.BecomeAdmin`, `App.AdminDeleteUser`, `App.AdminSetUserRole` | integration | Admin-only operations must reject non-admin callers and forbid self-modification (can't delete/demote yourself) — the access-control boundary between regular and admin accounts. |
| `app_documents_split_evm_test.go` | 3 | CPM→kernel task conversion for split/segmented tasks, EVM schedule loading | unit, table-driven | A task split across non-contiguous date ranges must preserve every segment through the kernel conversion, and overlapping segments must be rejected — silent data loss or double-counted effort otherwise. |
| `app_pades_export_test.go` | 2 | `App.ExportDocumentPDF` / `ExportCombinedReport` with runtime PAdES signing | fixture/golden | A "signed" export must actually contain a verifiable PAdES signature, not just succeed — checked by parsing the produced PDF's signature structure. |
| `app_settings_test.go` | 6 | `App` settings load/save/reset, timestamp-config validation | unit, table-driven | Settings persistence must round-trip correctly and reject an invalid combination (timestamping enabled without PAdES) before it's saved, not after. |
| `audit_actions_test.go` | 10 | Tamper-evident audit log appends across chart/document/work-item delete, PAdES export outcomes, checkpoint ID stability | integration | Every mutating action must leave an audit trail; this is the compliance/forensics guarantee the app markets — a missing audit row here is a silent compliance gap. |
| `encryption_migration_test.go` | 2 | `App` plaintext→encrypted project migration | integration | Migrating an at-rest-unencrypted project to SQLCipher must require reissued recovery codes first and actually re-encrypt on completion — half-migrated state would be a data-loss risk. |
| `encryption_project_test.go` | 9 | `App.CreateProject`, `OpenProject`, DEK-based access control | integration | A project encrypted under one user's DEK must be unopenable by another user's session; opening a plaintext project must force migration rather than silently working. |
| `headless_cli_test.go`, `headless_encryption_test.go` | 12 | `--export`, `--stats`, `--repair` CLI flags against real (including encrypted) project files | integration, table-driven | The headless/scriptable CLI path is a fully separate code path from the GUI — every format and credential-failure mode is exercised without launching Wails. |
| `import_formats_test.go`, `mspdi_import_test.go` | 4 | MS Project (MSPDI) XML import | fixture/golden | A malformed or binary (non-MSPDI) file must be rejected with a clear error, not silently corrupt project state. |
| `montecarlo_action_test.go` | 4 | `App.RunChartMonteCarlo`, worker-count stability | property, table-driven | Monte Carlo risk simulation must reject a non-CPM chart and produce worker-count-independent (deterministic) results. |
| `portfolio_evm_test.go` | 3 | Portfolio-level EVM rollup across multiple projects | unit | Committed cost and EVM actual cost must stay in separate columns (a real bug class: conflating them double-counts spend); a cyclic schedule must be rejected rather than hang. |
| `project_path_confinement_test.go` | 3 | Path-taking IPC methods, encrypted DSN path handling | fixture | Every App method that takes a file path from the (untrusted) frontend must confine reads/writes to the user's own projects directory — a path-traversal guard. |
| `project_storage_test.go` | 3 | `enumerateProjects`, `CreateProject` subfolder layout | table-driven | Must find projects across every layout this app has ever used (flat vs. subfolder, `.pmforge` vs. `.gopmgr`) — see the PMForge→GoPMgr rename work. |
| `report_evm_test.go`, `resources_action_test.go` | 13 | CPM-referenced EVM resolution, resource leveling/histogram actions | integration, table-driven | Resource leveling must honor calendars, priority strategy, and split-task segments, and report tasks it genuinely cannot place rather than silently dropping them. |
| `risk_sync_test.go` | 3 | Risk Register document → Risk Matrix chart sync | unit | Syncing must map rows correctly, refuse a wrong chart kind, and leave the chart untouched (not partially overwritten) on invalid input. |
| `scenarios_app_test.go` | 6 | Scenario/what-if chart branching, baseline promotion | integration | Scenario charts are isolated copies — App methods must scope strictly to the currently-open project and never leak another project's scenario data. |
| `timeline_move_test.go` | 2 | `MoveTimelineEntry` | unit | Dragging a timeline entry must update project/sprint dates correctly and reject moves on read-only entries (deployments). |
| `user_isolation_test.go` | 1 | Per-user project directory isolation | integration | Two users on the same machine must never see each other's projects — the core multi-user security boundary. |

---

## `internal/admin` — 97.7%

Administrative Pack: secure archive creation and tamper-evident signature
event logging.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `admin_test.go` | 13 | `NewService`, `SecureArchive` (incl. the `GoPMgr_Archive_` prefix — renamed from `PMForge_Archive_`, see `b1c8336`; settings-load failure; `CertPath`-bundling failure), `LogSignatureEvent`, `LogDocumentSignatureOutcome` (incl. empty-string defaulting) | fault-injection, fixture, closed-DB | `SecureArchive` must fail closed on every stage that can fail: if settings can't be loaded, if the saved certificate can't be bundled, or if the success audit row can't be written (in which case the just-created archive file must be deleted rather than left as an unaudited artifact). Signature-event logging must never panic regardless of success/failure/nil-error combinations, since it runs on the signing hot path, and a blocked tamper-evident audit write must leave no partial row behind (this last assertion proves the write was reached and failed, not that the surrounding `if err != nil` guard specifically ran — that guard is log-only with no propagation, so no assertion can tell it apart from being deleted, the same disclosed limit as `applog`'s `pruneOldLogs` test). One `os.Remove` double-fault line (removing an unaudited archive fails too) is knowingly untested: forcing it needs a directory permission that would also block archive creation itself, so no portable test reaches it — see the in-code comment at `workflow.go`'s `SecureArchive`. |

## `internal/agile` — 50.3%

Agile/Software-Dev Pack: Kanban boards, backlog, sprints, DORA metrics.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `dora_test.go` | 14 | DORA metric classification (deploy frequency, lead time, CFR, MTTR), windowed aggregation | unit, table-driven | DORA's four key metrics have industry-standard elite/high/medium/low bands — misclassifying a band would show a team the wrong performance signal. |
| `ids_test.go` | 1 | Entity ID generation | fault-injection | ID generation must fail loudly (not silently produce a weak/predictable ID) when the entropy source is unavailable. |
| `store_test.go` | 1 | `EnsureDefaultBoard` | fixture | A board missing its default columns (e.g. from a partial migration) must be self-repaired rather than left broken. |

## `internal/analytics` — 100% (default build) / 80.6% (`-tags duckdb`)

Optional in-memory analytics engine; `stub.go` (default build, always
reports unavailable) vs. `duckdb.go` (real DuckDB-backed portfolio
rollups and CSV import) are two genuinely different implementations
behind the same interface — see `DEVELOPER_HANDBOOK.md`'s coverage-scope
entry for why both build tags are measured independently.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `duckdb_test.go` | 8 | `PortfolioRollup` (aggregation, minor-units preference, idempotence), `ImportTabular` (CSV) | unit, table-driven, fixture | Portfolio rollups must aggregate correctly and be safe to call repeatedly (idempotent) since the GUI may re-trigger it; CSV import must reject unsupported extensions and missing files before touching DuckDB. |
| `stub_test.go` | 1 | Stub engine's "unavailable" reporting | unit | The non-duckdb build must report unavailability cleanly, not panic or silently no-op. |

## `internal/applog` — 100% of `applog.go` (95.7% of package; `dialog_darwin.go`/`opendir_darwin.go` are platform-narrow and excluded from the ratchet, see `coverage-exclude-go.txt`)

Process-level diagnostic logging (dated log files, log pruning, fatal-error dialogs).

`Init`'s three failure branches (unresolvable log dir, `MkdirAll` failure,
`OpenFile` failure) and `Fatal`'s dialog/exit sequence were previously
untested because they depend on filesystem failure conditions or on a
real OS dialog + `os.Exit`. Phase 2 closed this by (a) forcing filesystem
failures with real-but-controlled obstacles (a blocker file/directory
sitting where `Init` needs to create one) instead of relying on
permission bits, which behave inconsistently when tests run as root in
CI containers, and (b) adding `userHomeDir`/`tempDir`/`showError`/`osExit`
as package-level func vars — the same injectable-seam pattern used for
`runtime.GOOS` in Phase 1 — so tests can force the "nothing is writable"
path and observe `Fatal`'s behavior without touching the real filesystem,
popping a native dialog, or terminating the test binary.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `applog_test.go` | 14 | `Init` (success, append-across-calls, unresolvable-dir, `MkdirAll`-fails, `OpenFile`-fails, prune-on-startup), `resolveLogDir`/`LogDir` (explicit dir, home fallback, temp fallback, fully-unresolvable), `pruneOldLogs` (normal sweep, unreadable-dir no-op — this one is coverage-only, see `DEVELOPER_HANDBOOK.md`), `Fatal`, `formatFatal`, `dialogMessage` | fixture, fault-injection (real filesystem obstacles), injectable-seam (`userHomeDir`/`tempDir`/`showError`/`osExit` func vars) | Logs must append across process restarts (not truncate), old logs must be pruned (unbounded disk growth otherwise), a fatal error's dialog text must include enough context to be useful without a debugger, and every one of `Init`'s "never fail outright" fallback paths must actually degrade to stderr-only logging rather than panicking or crashing a GUI launch with no window and no trace — the exact failure mode this package exists to prevent. |

## `internal/auth` — 97.7%

Argon2id password hashing/verification — the core of local authentication.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `password_test.go` | 21 | `HashPassword`, `VerifyPassword`, `NeedsRehash` | table-driven, fault-injection, round-trip | Extremely high test density for a security-critical primitive: every malformed-hash-string shape (wrong part count, bad version, bad base64, zero memory/time) must be rejected explicitly rather than causing undefined behavior, and a hash created under weaker parameters must be flagged for transparent rehashing on next login. |

## `internal/budget` — 100%

Cost-rollup engine (vendor contracts + labour estimates).

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `budget_test.go` | 6 | `Compute` | unit, table-driven | Vendor and labour costs must sum correctly including fractional story-point rates (via minor-units arithmetic) and a negative remaining budget (over-budget) must be representable, not clamped to zero. |

## `internal/calendar` — 92.7%

Working-day/holiday calendars per country, used by scheduling.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `calendar_test.go` | 9 | `For` (country lookup + fallback), `IsWorkday`, `WorkdaysFrom` (forward/backward) | table-driven | Every supported country's policy must actually validate against its declared regions (not silently accept garbage), and workday-walking must be correct in both directions (used by both forward scheduling and backward float calculation). |

## `internal/charts` — 94.8%

Chart taxonomy/dispatch layer: 22 chart kinds routed to one of 4 layout engines (DAG, Stats, Matrix, Flow).

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `charts_test.go` | 26 | `All`, `Get`, `ByEngine`, `Layout` dispatch for every kind | table-driven | The registry is the single source of truth routing a chart kind string to its layout function — an unrouted or mis-routed kind would silently render nothing or the wrong chart. Every kind's example data is round-tripped through `Layout` to prove the registry and the engines agree on schema. |
| `engines_schedule_test.go` | 3 | `LayoutWithSchedule` (CPM date-anchoring dispatch) | unit | Only CPM charts anchor to real dates; other DAG-family charts must delegate to plain layout, and a zero project start must fall back safely rather than producing garbage dates. |

## `internal/charts/dag` — 92.6%

DAG-family chart engines: WBS, CPM/Network/PERT, layered (Workflow-adjacent), Fishbone, causal tree.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `dag_test.go` | 48 | `Parse`/`Layout`/`Encode` round-trips for WBS, layered graphs, Fishbone, causal tree; cycle detection | table-driven, round-trip | The largest single test file in the repo — every DAG-family chart type must survive a parse→layout→encode round-trip unchanged, and every layout function must detect and reject cycles rather than infinite-looping. |
| `cpm_anchor_test.go` | 6 | `AnchorCPMDates`, calendar-constrained scheduled layout | table-driven | Anchoring abstract CPM day-offsets to real calendar dates must skip weekends/holidays correctly and flag resource overallocation without silently ignoring it. |
| `gantt_test.go` | 5 | Gantt-specific layout (incl. split/segmented tasks) | unit | A Gantt chart's cycle detection and absolute-work-segment emission (for split tasks) must match the underlying CPM data exactly. |
| `link_label_test.go` | 2 | Dependency-link label parsing (`FS+5d` etc.) | unit | Malformed or unusual link labels must parse to a sane default rather than crash the layout. |
| `pert_test.go` | 6 | PERT three-point estimation (optimistic/likely/pessimistic → expected + std dev) | table-driven | PERT's statistical formulas (expected duration, variance) must match the textbook formula exactly, including the all-zero-variance edge case. |

## `internal/charts/flow` — 94.9%

Flow-family chart engines: Workflow, Activity diagrams.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `flow_test.go` | 32 | `Parse`/`Layout`/`Encode` for Workflow and Activity diagrams, swimlane assignment, shape resolution | table-driven, round-trip | Layering (rank assignment) must handle diamonds and cycles correctly; nodes with no assigned swimlane must get a default lane rather than vanish from the rendered diagram. |

## `internal/charts/matrix` — 91.4%

Matrix-family chart engines: RACI, Risk Matrix, Stakeholder, SWOT, generic matrix.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `generic_test.go` | 9 | Generic matrix parse/layout, row/header padding mismatches | table-driven | A matrix with fewer data rows than headers (or vice versa) must be padded/truncated predictably, not panic on an index-out-of-range. |
| `raci_test.go` | 12 | RACI parse/layout, accountability-count validation | table-driven | A RACI chart with zero or multiple "Accountable" owners per task is a real process smell — the layout must flag it as an issue, not just render it silently. |
| `risk_test.go` | 3 | Risk Matrix canonical 5×5 grid, invalid-coordinate/duplicate-ID detection | table-driven | The risk grid's geometry is fixed (5×5); out-of-range coordinates or duplicate risk IDs must be reported, since they'd otherwise silently overlap on the rendered matrix. |
| `stakeholder_test.go` | 10 | Stakeholder power/interest quadrant placement | table-driven, property | Every stakeholder must land in exactly one of four labeled quadrants and stay within the unit canvas regardless of input scale; sort order must be deterministic (alphabetical) so re-renders don't jitter. |
| `swot_test.go` | 6 | SWOT four-quadrant layout | table-driven | The four SWOT categories must always render in the same canonical grid position regardless of how many items are in each. |

## `internal/charts/pdfrender` — 15.8%

Renders chart layouts directly into PDF via `gofpdf`. **Known low-coverage package — Phase 3 (Go construction tier) candidate**, not yet worked in this session's coverage effort.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `gantt_test.go` | 4 | Gantt PDF rendering (incl. split tasks), grid-step sizing | fixture/golden | A split-task Gantt bar must render as multiple disjoint segments, not one bar spanning the gap. |
| `pdfrender_test.go` | 13 | `Fit` (scale-to-frame math), `parseBody`, `isEngineNotImpl` | unit, table-driven | `Fit`'s scale math has 7 distinct geometric cases (exact match, width-constrained, height-constrained, oversized, undersized) each independently verified. |
| `risk_test.go` | 1 | Risk Matrix PDF rendering | fixture/golden | Confirms a Risk Matrix chart actually produces PDF bytes, not just that the Go call doesn't error. |

## `internal/charts/stats` — 95.4%

Stats-family chart engines: Pareto, Line, Bar, Pie, Burn-up/down, Cumulative Flow, Control chart.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `stats_test.go` | 26 | Pareto (sort, cumulative %, 80% annotation), `computeMean`/`computeStdDev`, Control chart (auto vs. explicit limits, out-of-bounds flagging) | table-driven | A Control chart's UCL/LCL auto-computation must not silently override explicitly-set limits, and points outside limits must be flagged — the entire point of a control chart is catching out-of-control processes, so a missed flag is a functional defect, not cosmetic. |
| `stats_remaining_test.go` | 41 | Line/Bar/Pie/Burn-up/Burn-down/Cumulative-Flow parse+layout | table-driven | Burn-down's ideal-trajectory computation is independently verified for N=0, N=1, and N=5 (a known trajectory) since it's a closed-form formula easy to get off-by-one on. |

## `internal/cli` — 100%

GNU-style CLI flag parsing (`--version`, `--export`, `--repair`, etc.).

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `parser_test.go` | 9 | `ParseFlags` (all 18 flags + positional project-path arg), `PrintVersion` | table-driven | Added 2026-08-04 (`dd9bf01`) — `ParseFlags` itself had zero tests despite `Config`/`PrintVersion` being tested in the same file. Uses the standard `flag.CommandLine`-reset technique to test code built on Go's package-level `flag` state without any production code changes. |

## `internal/crypto` — 91.7% (up from 72.9% at the start of Phase 2; see `DEVELOPER_HANDBOOK.md`'s two dated entries — the `LoadCertificate` bug fix, then `encrypt.go`/`keywrap.go`/`pdf_cms*.go`. Remaining gap is entirely accepted-and-disclosed lines: value-relationship guards that aren't dead by type (`aes.NewCipher`, a `DecryptBuffer` length check), a process-global FIPS-140 branch (`cipher.NewGCM`), and `asn1.Marshal`/`rsa.SignPKCS1v15` error-propagation wrappers whose underlying failure is proven real by direct helper tests but not reachable through this package's own well-formed call sites — see the handbook for the full, itemized breakdown rather than repeating it here.)

Symmetric encryption (project-at-rest), key wrapping (ADR-001 DEK hierarchy), PDF/CMS digital signing.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `encrypt_test.go` | 8 | `EncryptBuffer`/`DecryptBuffer` (round-trip, wrong-password, too-short, empty-password, salt-generation failure, nonce-generation failure) | round-trip, table-driven, `rand.Reader` swap (stdlib's own indirection point, restored via `t.Cleanup`, never `t.Parallel()` — a process-global var swapped mid-test would corrupt any concurrently-running crypto elsewhere in the package) | A fresh nonce must be used per call (reused nonces break AEAD security entirely) — explicitly asserted, not just assumed. The two `rand.Reader`-failure tests must isolate exactly one of `EncryptBuffer`'s two `io.ReadFull` calls each, or a mutation to one check silently passes by tripping the other's error instead — see the handbook for the two independent bugs (a self-referential delegation causing infinite recursion, then a cascading-failure design) break-verification caught in the first draft of the fake reader used here. |
| `keywrap_test.go` | 9 | DEK wrap/unwrap (ADR-001); `GenerateDEK` (`rand.Reader` failure); `WrapKey` (bad DEK, `EncryptBuffer` error propagation via empty secret); `UnwrapKey` (non-base64 input, post-decrypt wrong-length rejection) | round-trip, `rand.Reader` swap | Wrapping with the wrong secret must fail, and each wrap must produce fresh ciphertext (no nonce reuse), same rationale as `encrypt_test.go`. `UnwrapKey`'s wrong-length test deliberately bypasses `WrapKey` (which would reject a short DEK before ever reaching `UnwrapKey`'s own check) by building the wrapped blob directly via `EncryptBuffer` + base64 — break-verified to confirm it exercises `UnwrapKey`'s check specifically, not `WrapKey`'s. |
| `pdf_cms_test.go` | 3 | `cmsSignedAttributes`/`sortCMSAttributes` (`asn1.Marshal` error propagation via deliberately malformed inputs — an unsupported Go type, an empty object identifier); `sortCMSAttributes`'s DER-canonical ordering | direct unit tests against unexported helpers, using malformed-input shapes confirmed reachable by direct experimentation before writing the tests | These are real, general-purpose error paths that no real call site in this package's production code ever triggers (every real caller passes well-formed certificates/digests/attributes) — tested directly against the helper so the error-handling code itself is proven correct, while the higher-level callers' propagation of that same error is left as a disclosed, unreachable-via-realistic-input gap (see handbook) rather than forced via an artificial seam. |
| `pdf_cms_timestamp_test.go` | 5 | RFC 3161 timestamp token embedding into CMS; `parseDetachedPAdESCMS`'s structural validation (trailing data after either ASN.1 layer, wrong ContentInfo type, non-detached content, empty signature value) | fixture/golden, hand-crafted DER (decode a real signed CMS, mutate one field via `asn1.Unmarshal`/mutate/`asn1.Marshal`, re-encode) | Embedding an unsigned timestamp token must NOT change the already-computed signature bytes (it would invalidate the signature it's supposed to timestamp) — and multiple independent tokens must coexist. The 5 malformed-CMS cases were break-verified individually; one required discovering that `asn1.RawValue.Marshal` echoes a cached `FullBytes` field verbatim and ignores a mutated `.Bytes` field unless `FullBytes` is explicitly cleared first — noted in the handbook so a future mutate-and-reencode test doesn't silently no-op the same way. |
| `pdf_sign_test.go` | 16 | `SignPDFCMS`; `LoadCertificate` (chain-bundled P12 — the bug fix's own regression test, plain 2-bag P12, file-not-found, wrong password, non-RSA key rejection); `parseP12Blocks`/`splitLeafCertificate` (multi-key, unparseable key/cert, missing key/cert, non-RSA-cert skip, no-match-found, all as direct pure-function tests needing no P12 fixture); `SignPDFHash` (no-key error, sign+verify round-trip) | fixture/golden, real PKCS#12 fixtures (`testdata/`, see its `README.md` for provenance and regeneration), direct pure-function unit tests | A PAdES Baseline-B signature must specifically omit the CMS `signing-time` attribute (required by the PAdES spec, easy to get wrong by reusing generic CMS code). Separately: `LoadCertificate` previously called `pkcs12.Decode`, which hard-requires exactly one key and one certificate bag and errors on anything else — meaning any commercially-issued signing certificate exported with its issuing chain bundled in (the normal case) failed to load at all with a confusing "expected exactly two safe bags" error. Fixed by switching to `pkcs12.ToPEM` (no such limit) and classifying blocks explicitly, matching the signer's certificate to its key by public-key identity rather than trusting the P12's optional (and sometimes-omitted) `localKeyId` attribute. |

## `internal/db` — 65.4%

Persistence kernel: single SQLite file (optionally SQLCipher-encrypted) holding every project entity, plus the tamper-evident audit chain.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `audit_events_test.go` | 12 | Tamper-evident audit chain (hash-linked events) across every mutating operation | fault-injection, fixture | `VerifyAuditChain` must detect tampering (a modified historical row breaks the hash chain) — this is the compliance guarantee's actual enforcement mechanism, tested by deliberately corrupting a row and confirming detection. |
| `audit_test.go` | 1 | CSV audit export | fixture | Exported audit CSV must be private (correct file permissions) and complete (no silently dropped rows). |
| `backup_test.go` | 10 | `InitDB`, archival bundle create (current/v2 `.gopmgr` format, exercised by every create test), restore (v2 round-trip via `TestRestoreArchivalBundleValidatesAndPublishesProjectOnly`; legacy v1 `.pmforge` read path specifically via `TestRestoreArchivalBundleReadsSchemaVersion1Archive`) | fixture/golden, table-driven | Restoring a bundle must reject path-traversal entries in the zip and must not publish a partial archive if bundling fails partway — both are real safety properties, not incidental. Legacy v1 archives (pre-rename) must still restore correctly since users may have old backups on disk. |
| `baselines_test.go` | 2 | Baseline CRUD | fixture | Basic create/list roundtrip for schedule baselines. |
| `conn_pragmas_test.go` | 2 | SQLite pragma consistency across the connection pool | fixture | `PRAGMA foreign_keys=ON` must apply to every pooled connection, not just the one that set it — a per-connection setting that silently doesn't propagate would let orphaned rows slip through undetected. |
| `encryption_test.go` | 4 | `InitEncryptedDB`, plaintext→encrypted migration | fixture, fault-injection | Opening an encrypted DB with the wrong key must fail cleanly (not corrupt or silently open with garbage data). |
| `ids_test.go` | 2 | Project/entity ID generation | fault-injection | Same class as `internal/agile/ids_test.go`: ID generation must fail loudly under starved entropy. |
| `introspect_test.go` | 1 | Schema introspection (`DumpSchema`) | fixture | Used by `--schema-dump`; must enumerate every real table. |
| `money_columns_test.go` | 4 | Minor-units money columns, project timezone round-trip | round-trip | Money must round-trip through SQLite as exact minor units (cents), never floating point — a lossy round-trip here is a silent financial-accuracy bug. |
| `repair_encryption_test.go`, `repair_test.go` | 4 | `SwapInSnapshot` (self-healing repair workflow) | fault-injection | A corrupt or invalid snapshot must be rejected *before* the live database is closed — closing first and then failing to swap in would leave the user with no working database at all. |
| `resource_calendars_test.go` | 2 | Resource-specific calendar overrides | fixture | Basic CRUD + existence check for per-resource calendar overrides used by leveling. |
| `scenarios_test.go` | 6 | Scenario/what-if branching, baseline promotion, isolated-copy semantics | fixture, fault-injection | A scenario chart must be a genuinely isolated copy — editing it must never mutate the original chart it branched from. |
| `settings_test.go` | 10 | Project settings persistence, legacy-column backfill | fixture, round-trip | `TestSaveSettingsDerivesLegacySignatureEnabledFromMethod` / `TestGetSettingsBackfillsLegacySignatureEnabledToPAdES`: an old boolean signature-enabled flag must derive correctly to/from the newer PAdES-method-based settings, so upgrading doesn't silently disable signing for existing projects. |
| `sigma_test.go` | 3 | Six Sigma charter/fishbone JSON storage | fault-injection, round-trip | Malformed JSON in a Sigma JSON-blob column must be rejected on read, not silently return a zero-value struct that looks like empty user data. |

## `internal/debug` — 94.1%

Structured error-report wrapping (file/line/stack capture) for diagnostics.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `report_test.go` | 11 | `Wrap`, `ToError`, `Report` extraction | unit, table-driven | A wrapped error must capture the real call site (file/line/stack) at wrap time, not at some later unwrap time; wrapping `nil` must be a safe no-op, not panic. |

## `internal/documents` — 44.9%

25 document kinds (Charter, Risk Register, Status Report, etc.): registry, default content, PDF rendering.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `documents_test.go` | 9 | `All`, `Get`, `ByPhase`, `DefaultContent`, `Render` for every kind | table-driven | Every one of the 25 document kinds must produce valid default JSON content AND a valid rendered PDF — a kind that's registered but can't render would be a create-then-crash bug reachable directly from the GUI's "new document" button. |
| `helpers_test.go` | 12 | Date parsing, project-window computation, cost rollups, issue partitioning/sorting | unit, table-driven | Project-window computation (`ComputeProjectWindow`) must correctly extend the window from start-only tasks (milestones with no end date) — an easy off-by-one. |
| `project_budget_test.go` | 1 | Money formatting with thousands separators | unit | Display formatting correctness for budget figures shown to the user. |
| `report_evm_test.go` | 3 | EVM summary lines in document reports, combined-report chart-ref resolution | fixture | A status report's schedule chart reference must resolve to real EVM data, not a placeholder. |
| `report_quality_test.go` | 2 | Document-set preflight validation (certified vs. custom profile) | table-driven | "Certified" export mode must block on any missing required document; a custom profile must allow intentional omissions — different validation strictness for different user intents. |
| `risk_matrix_ref_test.go` | 1 | Risk Register ↔ Risk Matrix chart cross-reference | unit | The Risk Register document must correctly expose which Risk Matrix chart it's linked to. |
| `signed_pipeline_test.go` | 2 | `RenderSignedWithSigner` (shared PAdES signing pipeline) | unit | If signing fails, the function must return zero bytes (not partial/unsigned PDF bytes that could be mistaken for a valid signed document). |

## `internal/export` — 51.2%

Converts internal data models to interchange formats (PDF, CSV, HTML, MSPDI, Six Sigma reports).

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `csv_test.go` | 1 | CSV formula-injection neutralization | fixture | A cell value starting with `=`/`+`/`-`/`@` must be neutralized before writing CSV — otherwise opening the export in Excel executes attacker-controlled formulas (a real, named CSV-injection vulnerability class). |
| `evm_section_test.go` | 4 | EVM section inclusion/suppression in schedule reports | table-driven | Reports without EVM data configured must render unchanged (no empty EVM section clutter); reports with it must carry the section. |
| `html_test.go` | 1 | HTML schedule export | fixture | Basic generation sanity for the HTML export path. |
| `montecarlo_report_test.go` | 1 | Monte Carlo risk report PDF/A generation | fixture/golden | Confirms the risk report is a genuinely valid PDF/A document, not just non-empty bytes. |
| `mspdi_import_test.go`, `mspdi_test.go` | 7 | MSPDI (MS Project XML) import/export round-trip | round-trip, table-driven | Anchored-date export must fall back gracefully when no anchor is available, and task order must be deterministic across exports (non-deterministic order would make diffs across exports meaningless). |
| `pdf_eof_test.go` | 1 | Schedule PDF has no trailing data after `%%EOF` | fixture/golden | Trailing bytes after a PDF's EOF marker is a known malformed-PDF/steganography-adjacent smell some validators reject — verified absent. |
| `pdf_signing_test.go` | 3 | `RenderPDFWithSignerLoader` (PAdES embedding pipeline) | fixture/golden, fault-injection | A certificate-load failure must propagate as an error, not silently produce an unsigned PDF that looks signed. |
| `sigma_report_test.go` | 2 | `GenerateSigmaReport` (per-user export directory + permission tightening) | fixture | See `ea454d5`: proves the report writes into the caller-supplied `outDir` (the per-user isolation fix) and tightens an over-permissive existing directory to `0700`. |

## `internal/exportsafe` — 100%

Shared CSV/spreadsheet formula-injection neutralization, used by every CSV-producing exporter.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `exportsafe_test.go` | 2 | `Cell` neutralization | unit, table-driven | Same vulnerability class as `internal/export/csv_test.go` but at the shared-helper level; also asserts safe values pass through completely unmodified (no over-aggressive escaping that would corrupt legitimate data starting with, e.g., a minus sign). |

## `internal/fonts` — 81.5%

TrueType font catalog/embedding for generated PDFs.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `fonts_test.go` | 12 | Font family/style resolution, TTF validation, import + directory permission tightening | fixture, fault-injection | An unregistered font family must fall back to a known-good default rather than producing a PDF with missing glyphs; `ImportFont` rejects non-TTF files by content, not just by file extension. |

## `internal/kernel` — 92.0%

Scheduling math core: Critical Path Method, resource leveling, EVM, Monte Carlo simulation.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `anchor_test.go` | 6 | `AnchorSchedule` (day-offset → calendar date) | table-driven | A pathological calendar (e.g. every day a holiday) must still terminate rather than loop forever searching for a valid workday. |
| `baseline_test.go` | 3 | Baseline comparison, CPM-clamped progress | unit | Progress reported against a baseline must be clamped by what CPM actually computed — can't show more progress than the schedule allows. |
| `constraints_test.go` | 9 | SNET/MFO/FNLT/ALAP date constraints | table-driven | Each constraint type has distinct violate/satisfy semantics (e.g. FNLT violation produces negative float, MFO pulls the task rather than pushing it) — each independently verified since they're easy to conflate. |
| `evm_test.go` | 7 | `ComputeEVM` (Planned Value, split-task PV pausing during gaps) | table-driven | A split task's Planned Value must pause accruing during the gap between segments — a linear PV calculation across the whole span would overstate planned progress during idle time. |
| `leveling_options_test.go` | 12 | `LevelResourcesWithOptions` (strategy, horizon, splitting) | table-driven | A leveling horizon that's exceeded, or a leveling cycle, must return an explicit sentinel rather than silently truncating results — the caller needs to know leveling didn't fully succeed. |
| `links_test.go` | 10 | CPM dependency link types (FS/SS/FF/SF, lag/lead), cycle detection | table-driven | Negative lag/lead must clamp at the project start (can't schedule before day one); legacy untyped "precedents" must still work for backward compatibility with older project files. |
| `montecarlo_test.go` | 8 | `RunMonteCarlo` (finish-date distribution, tornado chart drivers) | property, table-driven | Results must be worker-count-independent (same simulation, different parallelism, same statistical output) — a determinism property, not a fixed expected value, since the simulation is inherently probabilistic. |
| `resources_test.go` | 15 | Resource usage profiles, overallocation detection, contention-based leveling | table-driven, fixture | Overallocation detection must honor per-resource calendar overrides (a resource unavailable Tuesdays isn't "overallocated" for not working Tuesday); leveling must serialize genuinely contending assignments without touching unrelated ones. |
| `scheduler_test.go` | 10 | `CalculateCPM`, topological sort | table-driven | Diamond-shaped and parallel-equal-path dependency graphs are specifically tested since they're the classic cases naive CPM implementations get wrong (picking the wrong critical path). |

## `internal/money` — 63.3%

Exact monetary arithmetic (minor units, avoiding floating-point currency bugs).

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `money_test.go` | 3 | `AmountFromMajorFloat`, `RateTimesQuantity` (exact rational rounding) | unit | Rate×quantity must use exact rational arithmetic, not float multiplication — float rounding errors compounding across many line items is a classic real-world accounting bug class. |

## `internal/pdfmeta` — 81.7%

Byte-level PDF metadata operations: XMP injection, PAdES signature embedding, PDF/A compliance.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `pdfmeta_test.go` | 28 | Xref/trailer parsing, incremental-revision object lookup, PAdES signature embedding, XML escaping, ICC profile handling | fixture/golden, fault-injection | `TestFindObjectBody_ReturnsLatestIncrementalRevision` and `TestInjectPAdESSignature_RejectsCMSLargerThanReservedCapacity` are the two highest-stakes cases: PDF incremental updates mean the *latest* revision of an object must be found (an earlier one would silently apply metadata to a stale object), and a CMS blob that overflows its reserved byte-range must be rejected rather than corrupt the PDF structure by overwriting adjacent bytes. |

## `internal/rfc3161` — 81.6%

RFC 3161 timestamp-authority client (fail-closed by design).

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `client_test.go` | 8 | `ClientTimestamp`, trust-root verification, redirect handling, nonce entropy | fault-injection, table-driven | `TestClientTimestampDoesNotFollowRedirects`: a timestamp authority response must not be followed through an HTTP redirect (an attacker-controlled redirect could substitute a different TSA) — a specific SSRF/substitution-class guard, not generic HTTP-client behavior. |

## `internal/sigma/charts` — 100%

Six Sigma Pareto chart calculation.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `pareto_test.go` | 10 | `CalculatePareto` | table-driven | Ties (equal counts) must sort stably — an unstable sort here would make the chart's item order non-deterministic across identical re-renders. |

## `internal/sigma/domain` — no statements (types + enums only)

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `enums_test.go` | 1 | Sigma enum string values | unit | Enum string values are persisted to the DB and JSON — a renamed constant's *string value* changing would silently corrupt or orphan existing stored data even though the Go identifier looks unchanged. |

## `internal/sigma/service` — 85.0%

Six Sigma project service layer (DMAIC phase data, tool-completion status).

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `service_test.go` | 9 | Input validation (empty title/project ID) across every Save* method | table-driven | Every mutating method must reject an empty required ID/title before touching the database — a consistent validation contract checked uniformly. |
| `status_test.go` | 14 | `GetToolStatus` per DMAIC phase (Define/Analyze/Improve/Control), `GetProjectReportData` | table-driven | Each phase's "not started / active / completed" status has distinct, phase-specific criteria (e.g. Analyze needs a Fishbone branch *with* a cause, not just an empty branch) — each threshold independently verified since a wrong status misleads the user about real project readiness. |

## `internal/sigma/stats` — 100%

Six Sigma descriptive statistics and process capability (Cp/Cpk, DPMO).

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `basic_test.go` | 10 | `CalculateDescriptive`, `CalculateCapability` | table-driven | `TestCalculateCapability_CpkLessThanCpWhenOffCenter`: Cpk must be strictly less than Cp when the process mean is off-center — the entire point of Cpk vs. Cp is capturing centering, so this is the one assertion that would catch a formula transposition bug. |

## `internal/sigma/tollgate` — 100%

DMAIC phase-gate readiness checks (can a project advance from Define to Measure, etc.).

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `readiness_test.go` | 24 | `CheckDefineReadiness`/`CheckAnalyzeReadiness`/`CheckImproveReadiness`/`CheckControlReadiness`, `CheckPhase` dispatch | table-driven | `TestCheckDefineReadiness_CanAdvanceThresholdIs80Pct`: the 80% pass threshold is pinned as an explicit literal so it can't silently drift; Measure phase is confirmed auto-approved (no gate) since it has no readiness criteria of its own. |

## `internal/signing` — 81.6%

Document signing: GnuPG detached signatures, PAdES timestamp preparation and application.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `gnupg_test.go` | 2 | `SignDetachedASCIIArmored` GnuPG command construction | unit | The constructed GnuPG CLI command must include the configured key and fall back to GnuPG's default key correctly when none is configured. |
| `pades_test.go` | 7 | `PrepareTimestamp`, `ApplyPAdES` | fault-injection, table-driven | `TestApplyPAdESFailsClosedWhenTimestampRequestFails`: if the configured timestamp authority can't be reached, signing must fail closed (no signature produced) rather than silently falling back to an unsigned or wrongly-classified (Baseline-B claimed as Baseline-T) document. |

## `internal/sqlitedriver` — no statements (registration only)

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `driver_test.go` | 1 | SQLCipher driver registration | unit | Confirms the registered driver is actually the SQLCipher-capable one, not a plain SQLite driver silently substituted (which would mean "encrypted" databases are actually plaintext). |

## `internal/templates` — 97.3%

Project Launchpad seeding: JDM (JSON Decision Model) rule evaluation + seed-action dispatch.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `jdm_test.go` | 5 | `NewEngine`, `Evaluate` (fallback row, known decision-table row, nil-engine guard, loader-error wrapping) | fixture, fault-injection | `TestEvaluate_SoftwareScrumRow` (added 2026-08-05 after adversarial review) is the only test that actually proves the `"industry"`/`"methodology"` map keys reach the JDM correctly — the fallback-only test would stay green even if both keys were misspelled, since a misspelled lookup and an unknown pair hit the same fallback row. |
| `jdm_windows_test.go` | 1 | Windows build of the JDM engine | fixture | Windows-only build (`jdm_windows.go`, excluded from the coverage ratchet as platform-narrow — see `DEVELOPER_HANDBOOK.md`) gets its own minimal smoke test since it can't be exercised on this (macOS) CI/dev host. |
| `seeds_test.go` | 14 | `Seeder.Apply`/`applyOne` for all 15 seed kinds, error propagation, partial-success contract | integration, fault-injection | Added 2026-08-04 (`41be798`). `TestSeeder_ApplyReturnsPartialReceiptsOnFailure` is the one genuinely non-obvious behavior: `Apply` must return receipts for every seed that succeeded *before* a failure, not discard them — pinned via a SQLite trigger blocking one specific seed's insert mid-batch. 4 more trigger-based tests pin each individual seed handler's error propagation. |

## `internal/timeline` — 96.7%

Assembles every dated entity in a project (tasks, sprints, deployments) into one timeline view.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `timeline_test.go` | 8 | `Build`, `ParseDate` | table-driven | Entries with empty or zero-value dates must be skipped from the timeline (not rendered as bogus 1970-01-01 entries); a failed deployment's title must be visually distinguishable. |

## `internal/update` — 42.0%

Signed release-manifest fetch and verification (Ed25519), semver comparison.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `check_test.go` | 28 | `CheckLatest`, `verifyManifest` (Ed25519 signature checks), `isNewer` (semver comparison) | fault-injection, table-driven | Security-critical: `CheckLatest` must reject a non-HTTPS manifest URL outright, and every possible manifest-tampering shape (wrong key, bad base64, invalid JSON, channel mismatch) has its own dedicated rejection test — an update mechanism is a prime supply-chain-attack target, hence the density here despite auto-update not yet being enabled in any shipped release (see `ROADMAP.md`). |

## `internal/users` — 89.5%

Local multi-user account system: Argon2id-backed accounts, ADR-001 DEK hierarchy, PMForge→GoPMgr data-root migration. Coverage sweep in progress (2026-08-05): `dek.go` closed to 90.2%; `recovery.go` closed to 92.9% (`IssueRecoveryCodes` 94.7%, `ResetWithRecoveryCode` 92.2%, `migrateRecoveryTable`/`canonicalise` 100%); `store.go` closed to 87.7% in its first of two sub-increments — `Open`/`Close`/`RootDir`/`migrate`/`migrateAdminColumn`/`SetAdmin`/`DeleteAccount`/`CreateAccount`/`Authenticate`/`List` (all DB-backed, `CreateAccount`/`Authenticate`/`List` now 100%); the second sub-increment (`MigrateLegacyRoot`/`migrateLegacyRoot`/`copyTree`/`copyFile`/`ensurePrivateDir`/`ensurePrivateSQLiteFiles`, all filesystem-backed) is the final remaining increment for the whole package.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `conn_pragmas_test.go` | 1 | `system.db` connection-pool pragmas | fixture | Same class as `internal/db/conn_pragmas_test.go`, for the separate system (accounts) database. |
| `dek_test.go` | 13 | ADR-001 DEK unlock (incl. `migrateDEKColumns`'s closed-DB and dropped-table failure branches, `UnlockDEK`'s invalid-username/entropy-failure/empty-password/blocked-persist branches, `HasLegacyRecoveryCodeWraps`'s invalid-username branch), recovery-code-based reset, legacy-code migration | fixture, round-trip, fault-injection, closed-DB | `TestUnlockDEKLazyGenerationAndStability`: accounts created before the DEK hierarchy existed must lazily generate one on first unlock, and it must be *stable* — unlocking twice must yield the same key, not a fresh one each time (which would make previously-encrypted data unrecoverable). The new fault-injection tests pin each of `UnlockDEK`'s four internal failure points individually (invalid username, entropy source, empty-password rejection, blocked persist) rather than relying on `err != nil`, since a bare check masks which specific branch actually ran — see `DEVELOPER_HANDBOOK.md`'s matching entry for a mutation that was initially missed this way and fixed. Two branches in `migrateDEKColumns` (the `PRAGMA table_info` row-scan and post-loop `rows.Err()` checks) stay disclosed-untested: SQLite guarantees that pragma's six-column output shape, so no forcing method exists short of a corrupted SQLite build. |
| `migrate_root_test.go` | 6 | `MigrateLegacyRoot` across every historical data-root layout (current PMForge, pre-2026-06 relocation, XDG override) | fixture, e2e | `TestMigrateLegacyRoot_FindsXDGInstall`: covers a real-world Linux desktop-environment configuration (`$XDG_DATA_HOME` set), not just a test convenience — see the `legacyRootCandidates` doc comment for why this matters. |
| `recovery_test.go` | 24 | Recovery-code reset (canonicalisation), entropy-failure propagation, `migrateRecoveryTable`'s closed-DB failure, `RemainingRecoveryCodes` (positive counting + invalid-username), `IssueRecoveryCodes` (invalid-username, user-existence closed-DB and nonexistent-user branches, `generateCode`-vs-`HashPassword` entropy failures distinguished via a call-indexed fake reader, wrong-length-DEK rejection, DELETE/INSERT trigger-blocked branches with distinct abort messages), `ResetWithRecoveryCode` (short-password rejection, closed-DB `tx.Begin()` failure, dropped-table `SELECT` failure, non-matching-code rejection, corrupted-`wrapped_dek` "recovery wrap corrupt" detection, legacy-vs-non-legacy `GenerateDEK`/`WrapKey` entropy failures, used-flag/password-update trigger-blocked branches, `HashPassword` entropy failure at call index 3) | fault-injection, unit, closed-DB, call-indexed fault-injection | Pasted recovery codes must be canonicalized (whitespace-tolerant) since users copy-paste them from a document. `TestIssueRecoveryCodes_HashPasswordEntropyFailure` uses a call-indexed `rand.Reader` fake (fails only the 2nd read call) rather than an always-failing one, so `generateCode`'s own guard is proven to have run and succeeded first. `TestResetWithRecoveryCode_RejectsCorruptedWrap` is the most security-critical test in this file: a recovery code whose stored hash matches but whose `wrapped_dek` has been corrupted (e.g. by disk corruption or a bug elsewhere) must be rejected explicitly ("recovery wrap corrupt"), not silently treated as a legacy no-wrap code, which would generate a fresh DEK and orphan the user's encrypted projects. `TestResetWithRecoveryCode_RejectsInvalidUsername` is deliberately NOT a guard-presence test — break-verification showed deleting `ValidateUsername`'s check inside `ResetWithRecoveryCode` produces the identical `ErrInvalidRecoveryCode` value via the `matchID < 0` fallback, because the function unifies "invalid username" and "no matching code" into one opaque error by design (anti-enumeration); the test still pins the real behavioral guarantee, just not that specific guard. `TestResetWithRecoveryCode_WrapKeyEntropyFailure` asserts the exact bare injected-error text rather than `err != nil`, because break-verification also showed a deleted `WrapKey` guard cascades into `HashPassword`'s own, independently-guarded (and differently-worded) entropy failure a few lines later. Four branches stay disclosed-untested: `IssueRecoveryCodes`' `tx.Begin()` (preceded by an earlier DB call a closed-DB fault trips first) and `tx.Commit()`, plus `ResetWithRecoveryCode`'s `rows.Scan`/`rows.Err()` on its own SELECT cursor (same class as `migrateDEKColumns`'s PRAGMA-cursor disclosures — this table has only one writer in this codebase, so a type-mismatched Scan has no realistic trigger) and its own `tx.Commit()` (no portable forcing method for either function's COMMIT). |
| `root_dir_test.go` | 7 | `DefaultRootDir`, `legacyRootCandidates`, and (added 2026-08-05) the pure `defaultRootDirForGOOS`/`legacyRootCandidatesForGOOS` helpers | unit, table-driven | The GOOS-conditional pure-helper tests (`*_BothBranches`) are the ones that actually exercise both the darwin and non-darwin branches in a single run — see `DEVELOPER_HANDBOOK.md`'s 2026-08-05 entry for why the older `runtime.GOOS`-dependent tests alone couldn't do this on a macOS host. `TestDefaultRootDirAndLegacyRootCandidates_PropagateHomeDirError` covers the shared `os.UserHomeDir()` failure path via `HOME=""`. |
| `store_test.go` | 40 | `Store.Open` (private-permission enforcement, root-as-file, system.db-as-directory), `Close`/`RootDir` (trivial branches), `migrate`/`migrateAdminColumn` (closed-DB, already-migrated idempotency), `SetAdmin`/`DeleteAccount` (last-admin protection, no-such-user), `CreateAccount` (invalid username, duplicate-check closed-DB, entropy failure, blocked provisioning, blocked INSERT trigger), `Authenticate` (ValidateUsername short-circuit, no-such-user, corrupted-column Scan failure, wrong password, last-login parse, rehash entropy failure), `List` (closed-DB, corrupted-column Scan failure, last-login parse) | fault-injection, fixture, closed-DB, corrupted-column, call-indexed fault-injection | `TestAuthenticateReturnsLastLoginUpdateError`/`...PasswordRehashUpdateError` are the SQLite-trigger fault-injection pattern this session reused repeatedly elsewhere (`internal/templates`, etc.) — it originated here. `TestSetAdmin_DemoteSoleAdminReturnsErrLastAdmin`/`TestDeleteAccount_SoleAdminReturnsErrLastAdmin`: the system must never be left with zero administrators, a lockout-prevention guarantee. `TestOpen_RootDirIsFile` asserts the specific "mkdir root" wrapper text, not a bare error check: break-verification confirmed deleting `Open`'s `ensurePrivateDir` guard doesn't make `Open` fail differently — `sql.Open` connects lazily, so execution falls through to `s.migrate()`'s first `Exec`, which fails with an unrelated "unable to open database file" error against the same broken path, the same cascading-fallible-path shape as this session's other masked-mutation findings elsewhere in this package. `TestAuthenticate_ValidateUsernameShortCircuitsBeforeDBAccess` is a second instance of the anti-enumeration-unification shape first found in `recovery.go`'s `ResetWithRecoveryCode`: an open store can't break-verify this guard, because a present guard and a deleted one both ultimately return the identical `ErrNoSuchUser` value (present: the guard itself; deleted: `sql.ErrNoRows` from the following SELECT, since `ErrNoSuchUser`'s own doc comment mandates the merge). The fix here differs from `recovery.go`'s, though: closing the store first makes the two paths diverge on *when* the DB gets touched — a present guard still returns `ErrNoSuchUser` untouched, while a deleted one reaches the closed connection and surfaces "database is closed" instead, a value `ErrNoSuchUser` can never equal — so this one IS break-verifiable, unlike `ResetWithRecoveryCode`'s. `TestAuthenticate_ScanFailsOnCorruptedIsAdminColumn`/`TestList_ScanFailsOnCorruptedIsAdminColumn` use a new-to-this-package forcing technique: `UPDATE users SET is_admin = 'not-an-int'` writes a non-convertible value via SQLite's type affinity (the column stays `INTEGER`-typed but SQLite stores whatever is given), so the subsequent `Scan(&isAdmin)` fails deterministically without needing a closed connection or a trigger. Ten branches stay disclosed-untested in `store.go`, all with in-code comments at the point of the check: `Open`'s `sql.Open` own error and both migrate/private-file close-error cascades; `migrate`'s `migrateRecoveryTable`/`migrateDEKColumns` propagation checks (no hook between them and the prior successful Exec on the same live connection); `migrateAdminColumn`'s PRAGMA-cursor `Scan`/`rows.Err()` (same class as `dek.go`'s `migrateDEKColumns` disclosure); `SetAdmin`'s and `DeleteAccount`'s admin-COUNT query errors (same live-connection, no-DML-trigger-possible reasoning as `migrate`'s propagation checks). |

---

## `scripts` (package `scripts`, config-check helper) — 63.4%

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `config_check_test.go` | 2 | Repository config-file validation, version-control-candidate path selection | fixture | Backs the `make config-check` gate — must correctly identify which config paths are actually tracked before validating them. |

## `tools/update-manifest` — 42.4%

Standalone CLI that produces the signed release manifest consumed by `internal/update`.

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `main_test.go` | 1 | `run` (manifest generation + signing) | fixture/golden | Confirms a generated manifest is actually verifiable — round-trips through the same signature-checking logic `internal/update` uses on the client side, so manifest-generation and manifest-verification can't silently drift apart. |

---

## Frontend (`frontend/src`) — 14.69% of statements (see `DEVELOPER_HANDBOOK.md`)

Vitest + `@testing-library/svelte`. Coverage here is intentionally far
lower than Go — see `ROADMAP.md`'s Phase 6 for the reasoning (most of the
233 `.svelte` files are presentational and haven't been reached yet; the
tests below are either pure-logic modules or the components judged
highest-value so far).

| File | Tests | Covers | How | Why |
| --- | --- | --- | --- | --- |
| `src/lib/autosave.test.ts` | 2 | Autosave debounce/trigger logic | unit | Autosave must actually debounce (not fire on every keystroke) and must fire on the trailing edge. |
| `src/lib/components/AppSettings.test.ts` | 3 | App-level settings form | component | Basic render + save-callback wiring for global app preferences. |
| `src/lib/components/charts/GanttBars.test.ts` | 4 | Gantt bar rendering geometry | component | Bar position/width must match the underlying task's date range. |
| `src/lib/components/charts/GanttEditor.test.ts` | 5 | Gantt chart editor interactions | component | User edits (drag/resize) must produce the correct updated task data. |
| `src/lib/components/charts/_flow_shapes.test.ts` | 13 | `shapePath`/`shapeFill`/`shapeTextFill`/`edgePath`/`edgeLabelPosition` (SVG geometry for Workflow/Activity editors) | unit, table-driven | Added 2026-08-04. Pins exact SVG path strings for each shape (oval/diamond/parallelogram) and edge-routing offset math — geometry bugs here render silently wrong (a lopsided diamond) rather than throwing, so nothing else would catch a regression. `a_decision`/`decision` share geometry; only the `decision` case's exact-path assertion actually break-verifies it (documented in the test comments). |
| `src/lib/components/charts/gantt_geometry.test.ts` | 11 | Gantt geometry helpers | unit, table-driven | Pixel-position math for Gantt rendering, tested independent of the Svelte component. |
| `src/lib/components/charts/leveling_messages.test.ts` | 13 | Resource-leveling result → user-facing message text | unit, table-driven | Each leveling outcome (horizon exceeded, cycle detected, fully levelled, etc.) must map to a distinct, correct user-facing message — a mismatch here would show the user a misleading explanation of what leveling actually did. |
| `src/lib/components/project/ChartCatalog.test.ts` | 3 | Chart-kind catalog/picker | component | Renders the correct set of creatable chart kinds. |
| `src/lib/components/project/DocumentCatalog.test.ts` | 3 | Document-kind catalog/picker | component | Renders the correct set of creatable document kinds. |
| `src/lib/components/project/ProjectLaunchpad.test.ts` | 2 | Project-creation launchpad (industry/methodology selection → seed preview) | component | Confirms explicit user confirmation is required before advancing, and that the selected timezone is sent to the backend on creation. |
| `src/lib/components/project/ProjectSettings.test.ts` | 2 | Project settings — PAdES timestamp configuration section only | component | Narrow, targeted coverage: this file is one section of a much larger (1,900-line) component — see `DEVELOPER_HANDBOOK.md`'s coverage entries for why the rest of `ProjectSettings.svelte` is a known, tracked gap (48% covered) rather than assumed complete. |
| `src/lib/components/sigma/SigmaFishbone.test.ts` | 1 | Six Sigma Fishbone diagram editor | component | Basic render sanity for the Fishbone editor. |
| `src/lib/persistence-boundary-strings.test.ts` | 4 | `.gopmgr`/`.pmforge` extension strings, GoPMgr/PMForge data-root path strings across `HelpGuide.svelte`, `Dashboard.svelte`, `ProjectLaunchpad.test.ts` (via Vite `?raw` imports) | fixture, unit | Added during the PMForge→GoPMgr rename (`7d8a699`/`b1c8336`) specifically to pin these literals against find/replace drift — these strings are a real persistence boundary (existing user files on disk), not branding, so a careless rename would orphan real installs. |
| `src/lib/session.test.ts` | 2 | Session state store | unit | Login/logout state transitions. |
| `src/lib/terminology.test.ts` | 7 | `term`/`capitalised` (methodology-specific vocabulary: "task" → "user story" for Scrum, etc.) | unit, table-driven | Added 2026-08-04. This lookup table drives user-visible labels across the whole GUI — a wrong entry silently mislabels every work item for that methodology with no error and no visual signal it's wrong. Covers per-methodology overrides, fallback to the generic word, case-insensitive matching, and unknown-methodology fallback. |
| `src/lib/theme.test.ts` | 6 | Light/dark theme resolution | unit | Theme preference resolution (explicit setting vs. OS preference) must be correct, since a wrong theme reads as a visual bug immediately. |

---

## Known gaps (cross-reference)

This ledger records what exists; it does not restate what's missing —
that's `ROADMAP.md`'s job (see the "100% Full-Repo Test Coverage"
section for the phased plan, cost estimates, and the exclusion list).
The two package rows above marked "Known low-coverage package" are the
exception, flagged here because their coverage number would otherwise
read as unexplained given how thorough this ledger is everywhere else.
