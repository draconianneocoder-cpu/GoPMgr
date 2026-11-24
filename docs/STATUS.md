<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# GoPMgr current status

**Last reviewed:** 2026-11-14, against `main` at `0952c8f` (PR 214).
Unmerged work is named below.

This file says what is true now. Where it disagrees with an older document,
this file wins for *status*; the older document still stands as the record of
what was decided or measured on its own date. Design records, ADRs and
evidence files are not rewritten to match it.

Change a row only with something to point at: a merged commit, a decision
record or an evidence file. Adopted on 2026-11-11
([decision](review/evaluation-2026-11-10-response-plan.md)).

## Vocabulary

| Word | Meaning |
| --- | --- |
| Implemented | The code is on `main`. Says nothing about qualification on a platform. |
| Recorded evidence | Tests or measurements are written down with their date and commit. |
| Authorized | The owner approved this bounded slice for implementation. |
| Proposed | A design exists; implementing it is not approved. |
| Open | A known problem with no fix on `main`. |
| Blocked | Cannot start until the named decision or prerequisite exists. |
| Unverified | Implemented, but never run on that platform or against that input. |

## Platform and toolchain

| Item | Status | Pointer |
| --- | --- | --- |
| Go 1.27.4 required | Implemented (PR 199) | [DEPENDENCIES.md](DEPENDENCIES.md) |
| Wails 3.1.0, Svelte 5, GTK 4 / WebKitGTK 6.0 on Linux | Implemented 2026-11-09 | [migration evidence](testing/wails-v3-migration-evidence.md) |
| Native qualification on macOS 26 and Ubuntu 26.04 | Open: installed-app lifecycle, recovery, dialogs, accessibility not qualified | [ROADMAP.md](ROADMAP.md) current focus 1 to 3 |
| Windows | Unsupported preview by decision (2026-11-14): the installer is still built; the notes of every release the release workflow creates start with `docs/release-notes/platform-notice.md`, which calls it an unverified preview and says Windows is not supported yet (`make release-scope` fails if it is dropped); INSTALL.md and the in-app help say the same. No Windows host has built or run the current tree. The signed update manifest still offers the Windows installer as an update, by owner decision of 2026-11-16 | [next-alpha plan](next-alpha-release-plan.md) |
| SQLCipher 4.20.0 / SQLite 3.54.1 engine, go-sqlcipher 4.4.2 Go bindings | Implemented (PR 196) | [upgrade evidence](testing/sqlcipher-4.20-upgrade.md) |
| Engine distrusts file schemas (`trusted_schema` off, defensive mode) | Implemented (PR 199) | `internal/sqlitedriver/hardening_test.go` |
| Analytics: Go-native, Arrow-Go for Parquet | Implemented. DuckDB retired 2026-11-02; DataFusion not adopted 2026-11-09 | [DF0-G report](testing/adr-012-df0-g-report.md) |
| Launchpad rules evaluated in Go; `zen-go` removed | Implemented (PR 199) | `internal/templates` |
| Account data (`system.db`) marked with a reader level; a build refuses data marked by a newer one | Implemented (PR 201). Unverified on Windows; v1.1.0-alpha.2 and earlier ignore the mark. Every release's notes must say that going back is unsupported and untested | `internal/users/schema_gate.go`, [platform notice](release-notes/platform-notice.md) |
| Package versions come from version tags only (`v[0-9]*`) | Implemented (PR 200) | `scripts/package-version-lib.sh` |
| Signing, notarization, live update channel | Open: mechanism built, no keys provisioned | [ROADMAP.md](ROADMAP.md) current focus 4 |

## Guide reference navigation

The [user guide](user-guide.md) and
[developer handbook](DEVELOPER_HANDBOOK.md) provide linked contents,
alphabetical subject indexes and cross-linked glossaries.
The in-app Help Guide shares the user glossary and provides direct definition
lookup. Local frontend tests and browser keyboard checks passed; native
installed-app and screen-reader qualification remain unverified.

## Companion website and repository controls

| Item | Status | Pointer |
| --- | --- | --- |
| Companion website source (`website/`, Hugo 0.170.0 unified edition, Hextra v0.13.0) | Implemented (PR 203, PR 204); owner-approved Hugo security update prepared locally on 2026-11-15. Opt-in `website-*` Make targets only; no application or release target depends on it | [website/README.md](../website/README.md) |
| Website publication on GitHub Pages | Blocked: nothing is published and Pages is off. Needs the acceptance record and separate owner authorization | [website/README.md](../website/README.md), acceptance stages |
| Website conditional publication safeguards | Implemented locally on 2026-11-15 under owner approval; exact artifact-bound owner grant, authenticated publisher/destination checks, immutable commit and forward-only lease. No live grant, push or Pages change; full publication acceptance still pending | `website/tools/publisher.py`, [contract](../website/README.md) |
| Website pages pin the repository files they cite by commit and hash | Implemented (PR 203). A change to a cited file fails the site build until the page is reviewed and re-pinned | `website/quality/source-dependencies.json` |
| Website page policy: scripts, styles and data from the site only, no inline script | Implemented (PR 203). A meta tag, because GitHub Pages cannot send headers: another site can still show these pages in a frame. The checker also rejects automatic meta refresh and duplicate attributes (PR 207) | `website/tools/site.py` |
| Website accessibility | Recorded evidence 2026-11-12: axe 4.14.0 reports no violation on 11 pages, light and dark, at four widths; keyboard use of the skip link, search and menus checked. Unverified: a screen reader, browser zoom at 200% and 400% | PR 204 |
| Website tests in hosted CI | Checker CI implemented (PR 206). Production build implemented (PR 207). All 14 hosted checks passed on its final revision `f27c989` on 2026-11-13 (run 43336827515). A configuration guard preserves the reviewed execution contract. Website check is not required for merging | `.github/workflows/ci.yml` |
| Website build on Linux | Historical verification: Ubuntu 26.04.1 and PR 207 hosted CI at `709a31d`, with Go 1.27.4 and Hugo 0.169.0. Hugo 0.170.0 archive and executable provenance is recorded; that binary has not been executed on Linux in this acceptance session | `website/quality/dependencies.json` |
| `main` protected on GitHub: pull requests only, merge commits only, no force-push or deletion, four required checks | Implemented 2026-11-12 as repository ruleset "Protect main", with no bypass list. A setting, not a file in this repository | GitHub repository settings |
| GitHub Actions limited to pinned or allow-listed actions | Open: the repository allows any action | GitHub repository settings |
| Vulnerability reports go to GitHub's private form | Implemented (PR 203) | [SECURITY.md](SECURITY.md), Reporting |

## Limits that are easy to confuse

| Limit | Value | Where |
| --- | --- | --- |
| Schedule length | 18,250 **working days** per schedule. At least 50 years on any calendar: about 70 on Monday to Friday, exactly 50 if every day is worked. | `kernel.MaxScheduleDays` |
| Tabular import result | 10,000 rows | `internal/analytics` |
| WBS v2 aggregate | 10,000 nodes | `internal/wbs` |
| Editable schedule activities | Target 50,000, stress 100,000. **Not qualified.** | [OWN-10 plan](testing/ps04-own10-qualification-plan.md) |

## Security findings

From the [2026-11-09 audit](engineering/code-health-audit-2026-11-09.md) and
the [2026-11-10 evaluation](review/independent-evaluation-2026-11-10.md).

| ID | Finding | Status |
| --- | --- | --- |
| — | 2020 SQLite core | Fixed, PR 196 |
| — | Unbounded schedule length | Fixed, PR 196 |
| — | CSV formula in task ID | Fixed, PR 196 |
| — | Planted triggers, views, virtual tables in a restored backup | Fixed, PR 196 |
| — | The Sigma table migration turned foreign keys back on ignoring any error, so a pooled project connection could keep running without them | Fixed, PR 209 (2026-11-13). Found by a sweep for discarded errors; the restore is read back and a connection that fails it is discarded. Test fails without the fix |
| — | A database read that stops part-way was accepted as complete: the administrator session's sealed-key and attestation checks, the takeover-protection list, and cost baseline approval, which stored a partial sum | Fixed, PR 208 (2026-11-13). Each fix has a test that fails without it, and the lint job rejects the pattern (`rowserrcheck`). Hosted CI passed on macOS 26 and Ubuntu 26.04; not qualified on a native host |
| — | Certificate path read into backups unchecked | Fixed, PR 198 |
| SEC-01 | Engine trusts file schemas | Fixed, PR 199 |
| SEC-02 | Tables a restored backup defines are trusted | Fixed 2026-11-11: restore compares the file's tables, columns, column types, constraints, conflict and collation clauses and indexes with the schema this build creates, read-only, before opening it for writing. Empty project files made by v0.9.0-alpha, v0.9.1-alpha, v1.1.0-alpha.1 and v1.1.0-alpha.2 pass, before and after migration (run once on 2026-11-11; not kept as fixtures). Not covered: fuzzed malformed files (NEW-01), backups of projects with data from old releases, and a constraint the backup has removed, which passes by design ([note](../internal/db/schema_allowlist.go)) |
| SEC-03 | Backups contain the private signing key | Fixed 2026-11-11: excluded unless the user ticks the option for that backup; inclusion is audited. Backups made earlier still contain it |
| SEC-04 | 16 low-severity scan items | Triaged 2026-11-13, all sixteen ([triage](review/sec-04-triage.md)). Four were fixed by earlier work, three small defects are fixed (a named-pipe hang in attachment import, `#` in project names, the plaintext opener's `?` guard), the plaintext-copy notice is corrected, one is refuted and one is SEC-09. Decided 2026-11-14 ([decisions](review/owner-decisions-2026-11-14.md)): an increasing sign-in delay without lock-out, the same Argon2id work for an unknown username as for a wrong password, and clearing copied recovery codes after 60 s (or at quit) if the clipboard still holds them are implemented (the clipboard round trip is unverified on the release hosts), and so is offering a confirmed deletion of the plaintext copy once the encrypted project has opened. Open: two further confirmed findings held privately |
| SEC-05 | Windows build of the engine and its random source | Unverified |
| SEC-06 | No advisory check for the vendored C engine | Check implemented, PR 209 (2026-11-13) ([procedure](engineering/engine-advisories.md)). Offline: `make test` fails when the engine is not the reviewed one. Online: `make engine-advisories` fails on an NVD advisory without a disposition; passed 2026-11-13 with one `not_affected` (CVE-2026-119586, CLI shell only). Since 2026-11-14 ([decision](review/owner-decisions-2026-11-14.md)) the scan is required in `make check-release`, so in every tag's hosted preflight too, and fails on a review more than 90 UTC days old or dated after today; `make release-scope` fails if it is removed or simply weakened (a Makefile written to evade the check is left to review). Not yet run as part of a real tag. Not covered: CVEs NVD has not analysed |
| SEC-07 | Power-loss and hostile-input qualification of migration and restore | Open; depends on the new format |
| SEC-08 | FIDO2 / FIPS security keys | Proposed, approved in principle 2026-10-28 |
| SEC-09 | minimatch, deepmerge (development only) | Open by decision: no compatible fix exists |

## Architecture foundations

All from [ADR-011](design/ADR-011-normalized-schedule-store.md) and
[ADR-010](design/ADR-010-plugin-architecture.md). Nothing in this table is
authorized for implementation.

| ID | Item | Status |
| --- | --- | --- |
| ARC-01 | Normalized schedule storage | Design for ratification is [ADR-011 Amendment 2](design/ADR-011-normalized-schedule-store.md). Schedules still live in chart JSON. No schema is activated until DEC-FORMAT is ratified and a slice is approved |
| ARC-02 | New file format and older-writer protection | Same amendment, including the 2026-10-27 layered protection. Awaiting ratification. No format change |
| ARC-03 | Compact audit bridge | Design written in that amendment under DEC-AUDIT. Implementation needs a separate approval |
| ARC-04 | Typed, revision-checked commands | Contract is ADR-011 §6 plus Amendment 2. Needs a ratified ARC-02 and an approved ARC-03 implementation. Not started |
| ARC-05 | 50,000 editable activities end to end | Open. Needs ARC-01 to ARC-04 and DEC-HARDWARE. Amendment 2 sets no numeric budget |
| ARC-06 | One authority linking schedule, WBS, assignments, cost, baselines | Proposed |
| ARC-07 | Import lineage and custom fields in normalized data | Proposed |
| ARC-08 | Paged or streamed large results | Proposed |

## Professional scheduling work packages

Priorities are in [ROADMAP.md](ROADMAP.md); detail is in the
[modernization plan](design/professional-scheduling-modernization-plan.md).

| Package | Status |
| --- | --- |
| PS-00 Verified baseline | Partial. Hardware profiles, fixtures and budgets undecided (DEC-HARDWARE); MR-0 resource contract and per-slice MEM evidence required |
| PS-01 Normalized connected model | Design for ratification is ADR-011 Amendment 2 (ARC-01). Implementation waits for ratification. Schedules still live in chart JSON |
| PS-02 Schedule-quality audit | **Slice 1 authorized 2026-11-11.** Part a, the 18 rules in `internal/schedaudit`, and part b, the read-only `App.AuditSchedule` method, implemented 2026-11-13. Part c, the findings panel in the CPM editor and the schedule-audit section of combined reports, implemented 2026-11-18 ([§11](testing/PS-02-schedule-quality-audit-spec.md#11-ps-02c-as-implemented-2026-11-18)). Signed reports refuse an audit section until they are built through the reporting service. SQA-024 (the 50,000-task measurement on the S0 hardware profile) is not done: it needs DEC-HARDWARE. Known limit: charts have no way to designate start and finish tasks, so SQ-01 and SQ-02 report every schedule's open start and end. Later slices proposed |
| PS-03 File inspection and interoperability | Partial: MSPDI quick import and export, including task constraints since 2026-11-18. Worker slices need DEC-INTEROP |
| PS-04 Large-schedule storage, audit, Gantt | Design gate is ADR-011 Amendment 2. Implementation waits for ratification and for ARC-01 through ARC-05. Overallocation rescan fixed by RES-A-1; since 2026-11-21 demand counts the share of each day used, and since 2026-11-23 overallocation is judged over time within the day with assignment calendars (OR-01 contract changes). This design does not specify Gantt rendering |
| PS-05 Connected scheduling workspace | May rely on Amendment 2 for one active generation, one generation revision, and a chart that is a projection after activation. Not started |
| PS-06 Task and Resource Usage | May rely on the sparse timephased segments in Amendment 2. The kernel computes usage. No views |
| PS-07 Resource and financial planning | Partial. May rely on Amendment 2 for cost-owned rates, a schedule reference, and the rule that a forecast is not an actual. Rate-table columns are not specified |
| PS-08 Advanced scheduling | Partial: all 8 constraint types (MSO, SNLT and FNET since 2026-11-18); Monday-to-Friday country calendars only |
| PS-09 Analysis, comparison, reporting | Partial. Go-native analytics; **no DataFusion stage** |
| PS-10 Advanced local-first capabilities | Proposed |
| Provider-neutral integrations | Proposed, approved in principle 2026-10-28. C-01 onward unauthorized |
| Localization (ADR-008) | Phase 1 in progress: catalog, account screens, work-item editors done |

## Resource resilience

Planning direction integrated 2026-11-14; no runtime behavior changed.

| Item | Status | Pointer |
| --- | --- | --- |
| Global/subsystem memory admission and bounded queues | Not implemented. Required development contract MEM-01–12, stages MR-0–5; numeric profiles remain DEC-HARDWARE/OWN-10 decisions | [contract](design/resource-resilience.md) |
| Existing Monte Carlo path | In-process tasks × iterations allocation with fixed input/worker caps; machine-aware preflight and cancellation are the first proposed MR-1 pilot, not implemented protection | `internal/desktop/app_charts.go`, `internal/kernel/montecarlo.go` |
| Heavy-work worker isolation and safe retry | Accepted ADR-010 foundation and proposed ADR-015 detail; production global supervisor/admission and installed OOM containment not established. MR-1 host protection does not wait for G1 | [work sequence](design/resource-resilience.md#work-sequence-and-delivery-gates) |
| Durability and qualification | Per-slice MEM-T01–12 are NOT RUN. No new queued-job journal, every-keystroke guarantee or cross-platform memory claim; existing format/audit/retention gates remain | [assurance](testing/resource-resilience-assurance.md) |

## Reporting and extension planning

Integrated 2026-11-13 against `ac20cbe711d9c03479cf937d82c5b7da991454d3`.
This is documentary reconciliation; no new product behavior was implemented.

| Item | Status | Pointer |
| --- | --- | --- |
| RILD-01 reporting and lifecycle workstream | Workstream establishment recorded in the supplied plan; detailed ADR-014 and RILD-0–7 implementation slices proposed. Existing reports remain partial foundations, not immutable-issue qualification | [ADR-014](design/ADR-014-reporting-integrity-and-lifecycle.md) |
| Secure extension profiles and controls | ADR-015 proposed, source alias ADR-EXT-001. No Wasm runtime, public SDK, new package trust mechanism or community-native execution adopted | [ADR-015](design/ADR-015-secure-extension-platform.md) |
| Requirement preservation and agent sequencing | Documentation integrated: 120 explicit reporting IDs, eight catalog families, 17 architecture contracts and 31 EXT IDs; future qualification states NOT RUN | [sequence](design/reporting-extension/implementation-plan.md), [assurance](testing/reporting-extension-assurance.md) |

## Build assurance (ADR-013)

| Phase | Status |
| --- | --- |
| P0 to P5 | Implemented; see [ROADMAP.md](ROADMAP.md) |
| P6 | Partly implemented: checkout guard, licensing, pinned govulncheck, native Make environments |
| P7 to P9 | Proposed |

## Documentation structure

Owner decisions of 2026-11-17 (D1–D8, every recommendation accepted) in the
[reorganization plan](design/docs-reorganization-plan.md#7-owner-decisions).

| Phase | Status |
| --- | --- |
| P1: documentation ledger, checker, folder indexes | Implemented (PR 228): every file under `docs/` has an entry in [`ledger.json`](ledger.json) ([readable view](LEDGER.md)); `make docs-ledger-check` enforces the rules. CI also runs the history check (D8) |
| P2a: unpinned merges | Implemented in PR 229: five testing groups combined into `editor-save-atomicity.md`, `editor-guard-errors.md`, `editor-timestamp-boundary.md`, `sqlcipher-cleanup-corrections.md` and `wbs-foundation-evidence-2026-10.md`; the superseded FX options note is an appendix of `governed-presentation-only-fx.md`. Each record is kept as written: the ledger pins each section, `make docs-ledger-check` keeps checking it, and `make docs-ledger-history` compares it with its source at `9ab5d91` |
| P2b: merges and renames that change the pinned `ROADMAP.md` | Implemented in PR 230: the security-keys (v4) proposal fragments combined into `security-keys-v4-design.md` and the provider-integrations (v2) roadmap and test plan into `provider-integrations-v2-plan.md`, records kept as written and pinned in the ledger; the two critical reviews renamed `security-keys-v4-critical-review.md` and `provider-integrations-v2-critical-review.md`, unchanged; `ROADMAP.md` re-pinned and its `/roadmap/` and `/features/` pages re-reviewed |
| P3: the check joins `make verify` | Implemented in PR 231: `make verify`, which CI's Verify job runs, now runs `make docs-ledger-check`, so a document without a ledger entry, a broken link or a rewritten merged record fails the build |
| D8: the history check in CI | Implemented: CI's Verify job checks out full history and runs `make docs-ledger-history` against a pull request's target branch, or the commit a push started from |

## Known defects

| ID | Defect | Status |
| --- | --- | --- |
| BL-2 | `internal/db` and `internal/agile` generated only 32 random bits for new IDs | Generation repaired with 128 random bits with the existing prefix/hex encoding. Existing IDs stay unchanged. Collision-safe creation implemented 2026-11-18: a create whose generated ID is taken is refused (`ErrGeneratedIDCollision`) and writes nothing, in all ten upsert paths. Full Slice C is incomplete. See [scope, regression evidence and follow-up](testing/id-generation-128-bit.md) |
| KRN-01 | A constraint can be out of force without anyone being told. (a) MSO, SNLT and FNET were scheduled as ASAP. (b) `kernel.ApplyConstraintDates` leaves a date constraint unapplied when its date cannot be read or placed | **(a) Fixed 2026-11-18** by owner decision ("support properly"): the kernel arms and schedules all six date constraints, links still win and violations are flagged; both chart mappers carry them; the editor offers all eight types. This changes scheduling results for charts holding these types; baselines are unchanged; no migration. **(b) Fixed 2026-11-18**: the kernel records why a constraint is not applied (invalid date, out of range, no project start); the CPM node and task panel and the Gantt bar show it (`TestApplyConstraintDatesSaysWhyAConstraintIsNotApplied`, `cpm_anchor_test.go`). **Limits fixed the same day** (owner: "fix all identified still open issues"): latest limits read a non-working or pre-start date as the previous working day, and MSO/MFO there are flagged (owner decision "flag as violated"); part-day tasks show on the day their work starts; levelling keeps delays as a separate levelled start, never edits constraints, and flags dates it breaks (owner decision "separate field"; this also fixed a second levelling run removing pins still needed); MSPDI import and export carry constraints. **Follow-ups fixed 2026-11-21** (owner decisions the same day): Monte Carlo applies constraints and levelled starts as the schedule does and refuses what scheduling refuses; editors warn when the schedule changed since levelling; resource demand counts the share of each day a task uses (OR-01 contract change, see `docs/testing/resource-overallocation-contract.md`); levelling works in part days; SQ-09 names levelling only when the levelled start binds. **Fixed 2026-11-23** (owner decisions the same day, recorded in `docs/testing/resource-overallocation-contract.md` and the commits): detection and levelling judge resource use over time within the day, not by the day's total (D-TIME), and both read an assignment's own calendar (D-CAL, resolving F-09); levelled starts keep the share of the day passed (D-STORE) and are stored only where the saved chart needs them; split tasks keep their split days in every schedule, report and export while the split still fits; MSPDI export and import carry levelling delays as LevelingDelay (D-MSPDI) in tenths of elapsed minutes, as MPXJ reads Microsoft Project's files (owner decision the same day; Microsoft's documentation example shows the display unit instead; not checked against a file Microsoft Project saved); a levelled ALAP task keeps its levelled position (owner decision the same day), though MSPDI does not carry ALAP delays; the stale-levelling note also covers the project start, calendar and capacity. Details in the user guide, Scheduling Constraints |
| KRN-02 | A project start stored as RFC3339 with a time of day moved date constraints. `kernel.DayOffset` walks from the start instant and compares it with midnight-UTC constraint dates, so unless the start was midnight UTC a SNET, FNLT or MFO date landed a working day early or late, while the task dates shown still began on the stored calendar date. The report-export earned value, which passes the current time, also counted any time after midnight UTC as one more completed day. Reproduced in PS-02b review: start `2026-06-01T23:00:00-05:00` gives an MFO on 2026-06-03 offset 2 instead of 3, float -6 instead of -5; `2026-06-01T00:30:00+02:00` gives -4. Plain `YYYY-MM-DD` starts, which the MSPDI import and Project Settings write, are not affected | **Fixed 2026-11-13** on `fix/krn02-start-calendar-date`: `kernel.DayOffset` now compares calendar dates, each read in its own zone, which fixes every caller at once (the schedule, the chart's CPM view, levelling, the split preview and the status-date offsets). Scheduling results change only for a start stored with a time of day; earned value dated today (report exports, the combined report and the CPM editor's EVM panel) no longer counts the current day as completed after midnight UTC, so planned value, schedule variance and SPI drop by one day's planned work there and now match the portfolio, which already stripped the clock. Tests: `TestDayOffsetIgnoresTheStartsTimeOfDay`, `TestDayOffsetReadsTheDateInItsOwnZone`, and `TestAuditScheduleJudgesTheScheduleTheAppReports` over offset starts with hand-computed float |

## Owner decisions

Decided 2026-11-14 ([record](review/owner-decisions-2026-11-14.md)). Each
authorizes only the slice named; the last column says which are implemented.

| Decision | Chosen | Authorizes |
| --- | --- | --- |
| DEC-ADVISORY-SCAN | Release gate, review at most 90 days old | `make engine-advisories` in `check-release`: implemented |
| DEC-AUDIT | Approve the bridge design | Design written in ADR-011 Amendment 2. Implementation needs approval |
| DEC-FORMAT | Finish the design, then ratify | Amendment 2 is the specification. Not ratified. No format change |
| DEC-HARDWARE | Two profiles: mid-range laptop minimum, Apple Silicon reference | PS-00 fixtures and measurement; budgets after measuring |
| DEC-INTEROP | G1 only | MPXJ worker supervisor and bridge, inactive behind a flag |
| DEC-WINDOWS | Unsupported preview | Windows artifacts labelled unverified; no support claims: implemented |

SEC-04's security choices of the same day are in the SEC-04 row above.

Still waiting: SEC-04 items 14 and 15 (held privately), numeric performance
budgets, and approval of G2, G3 and the audit bridge's implementation.

## Proposed additions not yet taken up

NEW-02, the operation and provenance envelope, and NEW-09, the SQLite
feature boundary, are specified in ADR-011 Amendment 2 for ratification.
They are not implemented. NEW-04 (power-loss laboratory), NEW-05
(diagnostic bundle), NEW-08 (export fidelity corpus) and NEW-10
(mutation drills as a programme) stay proposed. NEW-01 (hostile-file
corpus) starts with response plan step A3.

## Documents known to be dated

Read these as records of their date, not as current status.

| Document | Dated because |
| --- | --- |
| [PS capability matrix](review/PS-capability-matrix.md) (2026-10-26) | Predates the engine upgrade, Wails v3 and Go-native analytics |
| [ADR-010 S0 baseline](testing/adr-010-s0-baseline.md) (2026-10-25) | Synthetic Linux probe; its 61-second Gantt layout predates RES-A-1 |
| ADR-011, Draft 1 and Amendment 1, before Amendment 2 (2026-11-18) | Those sections stay as written. Amendment 2 is the design for ratification. The earlier sections discuss SQLite 3.33. The engine is now 3.54.1. Amendment 2 still excludes STRICT, RETURNING, JSON1, FTS5 and DROP COLUMN |
| [Connected project model](design/professional-scheduling-domain-model.md), open-decision table | Those rows are the questions as they stood before Amendment 2 |
| ADR-010, status lines written before the 2026-10-28 G1 clarification | Later decisions authorize bounded G1 work |
| [beta-release-backlog.md](beta-release-backlog.md) | A running log; a later row can supersede an earlier one |
