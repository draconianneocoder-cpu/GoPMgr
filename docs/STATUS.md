<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# GoPMgr current status

**Last reviewed:** 2026-11-12, against `main` at `ea740a1` (pull request 204).

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
| Windows | Unverified: no Windows host has built or run the current tree | [next-alpha plan](next-alpha-release-plan.md) |
| SQLCipher 4.20.0 / SQLite 3.54.1 engine, go-sqlcipher 4.4.2 Go bindings | Implemented (PR 196) | [upgrade evidence](testing/sqlcipher-4.20-upgrade.md) |
| Engine distrusts file schemas (`trusted_schema` off, defensive mode) | Implemented (PR 199) | `internal/sqlitedriver/hardening_test.go` |
| Analytics: Go-native, Arrow-Go for Parquet | Implemented. DuckDB retired 2026-11-02; DataFusion not adopted 2026-11-09 | [DF0-G report](testing/adr-012-df0-g-report.md) |
| Launchpad rules evaluated in Go; `zen-go` removed | Implemented (PR 199) | `internal/templates` |
| Account data (`system.db`) marked with a reader level; a build refuses data marked by a newer one | Implemented (PR 201). Unverified on Windows; v1.1.0-alpha.2 and earlier ignore the mark | `internal/users/schema_gate.go` |
| Package versions come from version tags only (`v[0-9]*`) | Implemented (PR 200) | `scripts/package-version-lib.sh` |
| Signing, notarization, live update channel | Open: mechanism built, no keys provisioned | [ROADMAP.md](ROADMAP.md) current focus 4 |

## Companion website and repository controls

| Item | Status | Pointer |
| --- | --- | --- |
| Companion website source (`website/`, Hugo Extended 0.169.0, Hextra v0.13.0) | Implemented (PR 203, PR 204). Opt-in `website-*` Make targets only; no application or release target depends on it | [website/README.md](../website/README.md) |
| Website publication on GitHub Pages | Blocked: nothing is published and Pages is off. Needs the acceptance record and separate owner authorization | [website/README.md](../website/README.md), acceptance stages |
| Website pages pin the repository files they cite by commit and hash | Implemented (PR 203). A change to a cited file fails the site build until the page is reviewed and re-pinned | `website/quality/source-dependencies.json` |
| Website page policy: scripts, styles and data from the site only, no inline script | Implemented (PR 203). A meta tag, because GitHub Pages cannot send headers: another site can still show these pages in a frame | `website/tools/site.py` |
| Website accessibility | Recorded evidence 2026-11-12: axe 4.14.0 reports no violation on 11 pages, light and dark, at four widths; keyboard use of the skip link, search and menus checked. Unverified: a screen reader, browser zoom at 200% and 400% | PR 204 |
| Website tests in hosted CI | Implemented: the CI job "Website checks" runs `make website-tests` on Ubuntu 26.04 with full history. It tests the checkers; no hosted job runs Hugo or builds the site. Not a required check for merging | `.github/workflows/ci.yml` |
| Website build on Linux | Unverified: the Hugo executable is recorded for macOS ARM64 only | `website/quality/dependencies.json` |
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
| — | Certificate path read into backups unchecked | Fixed, PR 198 |
| SEC-01 | Engine trusts file schemas | Fixed, PR 199 |
| SEC-02 | Tables a restored backup defines are trusted | Fixed 2026-11-11: restore compares the file's tables, columns, column types, constraints, conflict and collation clauses and indexes with the schema this build creates, read-only, before opening it for writing. Empty project files made by v0.9.0-alpha, v0.9.1-alpha, v1.1.0-alpha.1 and v1.1.0-alpha.2 pass, before and after migration (run once on 2026-11-11; not kept as fixtures). Not covered: fuzzed malformed files (NEW-01), backups of projects with data from old releases, and a constraint the backup has removed, which passes by design ([note](../internal/db/schema_allowlist.go)) |
| SEC-03 | Backups contain the private signing key | Fixed 2026-11-11: excluded unless the user ticks the option for that backup; inclusion is audited. Backups made earlier still contain it |
| SEC-04 | 16 low-severity scan items | Open: not yet triaged one by one. Planned: step A5 |
| SEC-05 | Windows build of the engine and its random source | Unverified |
| SEC-06 | No advisory check for the vendored C engine | Open. Planned: step A4 |
| SEC-07 | Power-loss and hostile-input qualification of migration and restore | Open; depends on the new format |
| SEC-08 | FIDO2 / FIPS security keys | Proposed, approved in principle 2026-10-28 |
| SEC-09 | minimatch, deepmerge (development only) | Open by decision: no compatible fix exists |

## Architecture foundations

All from [ADR-011](design/ADR-011-normalized-schedule-store.md) and
[ADR-010](design/ADR-010-plugin-architecture.md). Nothing in this table is
authorized for implementation.

| ID | Item | Status |
| --- | --- | --- |
| ARC-01 | Normalized schedule storage | Proposed. Schedules live in chart JSON. Blocked by DEC-FORMAT |
| ARC-02 | New file format and older-writer protection | Proposed. Blocked by DEC-FORMAT |
| ARC-03 | Compact audit bridge | Proposed. Blocked by DEC-AUDIT (ADR-011 DEC-04) |
| ARC-04 | Typed, revision-checked commands | Proposed. Needs ARC-02 and ARC-03 |
| ARC-05 | 50,000 editable activities end to end | Open. Needs ARC-01 to ARC-04 and DEC-HARDWARE |
| ARC-06 | One authority linking schedule, WBS, assignments, cost, baselines | Proposed |
| ARC-07 | Import lineage and custom fields in normalized data | Proposed |
| ARC-08 | Paged or streamed large results | Proposed |

## Professional scheduling work packages

Priorities are in [ROADMAP.md](ROADMAP.md); detail is in the
[modernization plan](design/professional-scheduling-modernization-plan.md).

| Package | Status |
| --- | --- |
| PS-00 Verified baseline | Partial. Hardware profiles, fixtures and budgets undecided (DEC-HARDWARE) |
| PS-01 Normalized connected model | Proposed (ARC-01) |
| PS-02 Schedule-quality audit | **Slice 1 authorized 2026-11-11**, not yet implemented. Later slices proposed |
| PS-03 File inspection and interoperability | Partial: MSPDI quick import and export. Worker slices need DEC-INTEROP |
| PS-04 Large-schedule storage, audit, Gantt | Blocked (ARC-01 to ARC-05). Overallocation rescan fixed by RES-A-1 |
| PS-05 Connected scheduling workspace | Proposed |
| PS-06 Task and Resource Usage | Proposed. The kernel computes usage; no views |
| PS-07 Resource and financial planning | Partial |
| PS-08 Advanced scheduling | Partial: 5 of 8 constraint types; Monday-to-Friday country calendars only |
| PS-09 Analysis, comparison, reporting | Partial. Go-native analytics; **no DataFusion stage** |
| PS-10 Advanced local-first capabilities | Proposed |
| Provider-neutral integrations | Proposed, approved in principle 2026-10-28. C-01 onward unauthorized |
| Localization (ADR-008) | Phase 1 in progress: catalog, account screens, work-item editors done |

## Build assurance (ADR-013)

| Phase | Status |
| --- | --- |
| P0 to P5 | Implemented; see [ROADMAP.md](ROADMAP.md) |
| P6 | Partly implemented: checkout guard, licensing, pinned govulncheck, native Make environments |
| P7 to P9 | Proposed |

## Decisions waiting for the owner

| Decision | Unblocks |
| --- | --- |
| DEC-AUDIT | ARC-03, ARC-04 |
| DEC-FORMAT | ARC-01, ARC-02 |
| DEC-HARDWARE | ARC-05, PS-00 |
| DEC-INTEROP | PS-03 worker slices |
| DEC-WINDOWS | SEC-05, Windows release claims |

## Proposed additions not yet taken up

From the evaluation: NEW-02 operation and provenance envelope, NEW-04
power-loss laboratory, NEW-05 opt-in diagnostic bundle, NEW-08 export
fidelity corpus, NEW-09 SQLite feature policy for the new format, NEW-10
mutation drills as a programme. NEW-01 (hostile-file corpus) starts with
response plan step A3.

## Documents known to be dated

Read these as records of their date, not as current status.

| Document | Dated because |
| --- | --- |
| [PS capability matrix](review/PS-capability-matrix.md) (2026-10-26) | Predates the engine upgrade, Wails v3 and Go-native analytics |
| [ADR-010 S0 baseline](testing/adr-010-s0-baseline.md) (2026-10-25) | Synthetic Linux probe; its 61-second Gantt layout predates RES-A-1 |
| ADR-011, sections written before its 2026-10-27 amendment | Forbids SQLite features because the engine was 3.33; the engine is now 3.54.1. Feature use still needs the new-format decision |
| ADR-010, status lines written before the 2026-10-28 G1 clarification | Later decisions authorize bounded G1 work |
| [beta-release-backlog.md](beta-release-backlog.md) | A running log; a later row can supersede an earlier one |
