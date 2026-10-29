<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010 test strategy

**Status:** Proposed, together with
[ADR-010](../design/ADR-010-plugin-architecture.md) and its
[implementation plan](../design/adr-010-implementation-plan.md). No test
named here exists yet.

This strategy says what must be proven before each ADR-010 phase can pass,
how to prove it, and how to record the evidence. It brings the source test
IDs together in one matrix:

- PLG-001 to PLG-030 and SCALE-001 to SCALE-030, from v2.0;
- REVIEW-001 to REVIEW-020, from the addendum;
- ADR10-001 to ADR10-030, new in v2.1.

Each source ID keeps its number and meaning. Where GoPMgr's real design
changes what a case checks, the matrix says so.

## Contents

1. [Principles](#1-principles)
2. [Test layers and commands](#2-test-layers-and-commands)
3. [Fixture corpus](#3-fixture-corpus)
4. [Acceptance matrix](#4-acceptance-matrix)
5. [Publication fault injection](#5-publication-fault-injection)
6. [Performance plan](#6-performance-plan)
7. [Accessibility and localization](#7-accessibility-and-localization)
8. [Evidence and PASS discipline](#8-evidence-and-pass-discipline)

## 1. Principles

- **Prove the invariant, not the path.** The cases that matter most are the
  negative ones:
  - no partial project is ever visible;
  - no write happens after a session change;
  - no field disappears without being counted;
  - no worker process outlives its job.
- **Test at the cheapest layer that can fail.** Most rules can be proven by
  Go table tests against a fake worker. Only the cases that depend on MPXJ's
  behaviour need the real Java worker.
- **Make the worker misbehave on purpose.** The fake worker in G1 can hang,
  crash, print to stdout, send oversized or out-of-order frames, report
  completion early, and report wrong counts. The host must survive each one.
- **Never test with customer data.** Fixtures are made for testing or have a
  licence that allows redistribution (§3).
- **Gate on evidence.** A phase passes only when each case assigned to it
  has a recorded PASS from the current HEAD (§8).

## 2. Test layers and commands

| Layer | What it proves | Command (existing, or proposed for the phase) |
| --- | --- | --- |
| Go unit and table tests | Framing, validators, the journal state machine, receipts, value rules, error mapping | `go test . ./internal/...` (`make test`) |
| Race | Supervisor, cancellation, and job state under concurrency | `make race` |
| Fuzz | Frame decoding, record validation, format sniffing | `go test -run=^$ -fuzz=Fuzz<Name> ./internal/interop/...` locally; the seed corpus in `testdata/fuzz/` runs in every `go test` |
| Protocol golden tests | Go and Java encode and decode the same bytes | Shared fixtures under `internal/interop/protocol/testdata/`, checked by both sides |
| Fake worker | Every host rule that does not need MPXJ | A Go test helper binary that speaks the protocol (G1) |
| Java bridge unit tests | MPXJ mapping, stdout capture, framing | The bridge's own build (`workers/mpxj-bridge`), chosen in G0 |
| Worker integration | Host and real worker on fixtures | `go test` behind a build tag chosen in G1, because it needs the bundled JVM |
| Encrypted storage | Staging, sealing, and publication with SQLCipher | `go test ./internal/interop/publish/...`; `make check-encrypted-db` extended |
| Frontend component | Stepper, grids, badges, focus, IME, copy keys | `npm --prefix frontend test` (Vitest) |
| Frontend load | Rune misuse and mount crashes that type checks miss | `make frontend-smoke`, `make frontend-stability` |
| Frontend types and budget | Type safety and bundle size | `npm --prefix frontend run check`, `make frontend-build-budget` |
| Localization | No literal text in new components; every error code has a key | `frontend/src/lib/i18n/hardcoded-text.test.ts`; a new code-to-key test |
| Packaging | Offline launch, digests, notices, SBOM | `make license-check`, `make release-scope`, `make package-<os>`, `make check-release` |
| Performance | Budgets in plan §7.11 | Go benchmarks plus a harness that writes JSON evidence (§6) |
| Manual | Screen readers, OS dialogs, notarization | The checklists in §7 and the G4 release checklist |

**Ledger and hygiene.** Every new test file is added to
`docs/TEST_COVERAGE_LEDGER.md`, and `make coverage-ledger-current` must pass.
`make verify` stays the fast pre-commit gate.

## 3. Fixture corpus

Fixtures are listed in `testdata/interop/fixtures.json`, created in G0. Each
entry records:

- an ID, format, producer, and producer version;
- its size and SHA-256;
- its licence or origin;
- how it was made (tool and steps);
- the expected counts for each record type.

### 3.1 Sources

| Source | Use | Rule |
| --- | --- | --- |
| Made for testing in Microsoft Project, P6, or ProjectLibre | Real MPP, XER, PMXML, and POD files | The maintainer, or a contributor with a licensed copy, makes them; the steps are recorded |
| Written by MPXJ's own writers | MSPDI, MPX, XER, PMXML variations | Generated by a pinned script, so they can be rebuilt |
| Written by GoPMgr's MSPDI export | Round trips with Quick chart import | Generated in the test |
| Synthetic scale files | T1 to T3 tiers | A seeded generator; the seed is recorded |
| Hostile files | Parser limits and errors | Generated or hand-made; small enough to commit |
| Public sample files | Extra coverage | Only when their licence allows redistribution; record it |

Large tiers (T2 and T3) are generated at test time, not committed. The
generator and its seed are committed.

### 3.2 Coverage the corpus needs

| Group | Cases |
| --- | --- |
| Formats and versions | MPP from at least three Microsoft Project versions; MSPDI; XER; PMXML; one POD |
| Structure | Deep summaries, null rows, external links, duplicate IDs with distinct UIDs, cycles |
| Calendars | 8, 12, and 6-hour days; P6 calendar inheritance; holiday exceptions; time zones |
| Resources and costs | Colliding resource names, negative costs, JPY, three-decimal currencies, overtime |
| Baselines and timephased | 11 baselines; irregular segments; sparse and dense contours |
| Custom fields | Every type MPXJ reports, plus one MPXJ does not read |
| Identity | The same project saved twice, a copied project with the same UIDs, a moved task, UID reuse |
| Hostile | Truncated, empty, wrong extension, password-protected, XML entity expansion, external entities, deep nesting, very long strings, huge counts |

## 4. Acceptance matrix

**Gate** is the earliest phase that must pass the case; later phases keep
passing it. **Layer** names the cheapest layer that can prove it (§2).

### 4.1 Worker MVP and trust (G1, G2)

| ID | Case | Pass condition | Gate | Layer |
| --- | --- | --- | --- | --- |
| PLG-001 | Worker off | Create, open, save, export, and scheduling work unchanged | G1 | Go and frontend suites with the flag off |
| PLG-002 | Worker files changed after install | Worker never starts; `PLUGIN_SIGNATURE_INVALID` | G1 | Fake worker |
| PLG-003 | Wrong protocol major or worker ID | Handshake refused; host keeps working | G1 | Fake worker |
| PLG-005 | Worker killed during a job | Job fails; process reaped; nothing written | G1 | Fake worker |
| PLG-006 | Cancel while parsing | Cancel is deterministic; scratch removed; nothing written | G1 | Fake worker |
| PLG-019 | Worker asks for database or API access | No protocol message can carry it | G1 | Protocol review and golden tests |
| PLG-020 | Worker writes notes to stderr | Log capped and scrubbed; no note text in logs | G1 | Fake worker |
| PLG-024 | Upgrade with the worker absent | Projects and Quick chart import still work | G1 | Upgrade test |
| PLG-025 | Worker files removed while running | Controlled failure; no fallback runs anything else | G1 | Fake worker |
| PLG-027 | Worker reports success before the last frame | Host rejects the job | G1 | Fake worker |
| REVIEW-017 | Worker missing, off, or incompatible | Core editing and Quick chart import stay fully usable | G1 | Go and frontend suites |
| ADR10-001 | A library prints to `System.out` during parsing | Frame stream intact; the text appears on stderr | G2 | Java bridge test |
| ADR10-002 | Random and mutated frames | No panic, hang, or unbounded allocation on either side | G1 | Fuzz |
| ADR10-003 | Lock file on disk replaced | Digests are checked against the lock embedded in the binary | G1 | Go unit |
| ADR10-004 | Symlink, extra file, or missing file in the worker folder | Launch refused | G1 | Go unit |
| ADR10-005 | `JAVA_TOOL_OPTIONS`, `_JAVA_OPTIONS`, or `CLASSPATH` set in the user's environment | Not passed to the worker; fixed argument list only | G1 | Go unit |
| ADR10-006 | Two jobs in a row; a worker that starts a child process | Each job gets a new process; the whole tree is reaped | G1 | Fake worker |
| ADR10-007 | Handshake never answered | Deadline fires; process killed; `PLUGIN_START_FAILED` after one retry | G1 | Fake worker |
| ADR10-008 | Frame over 4 MiB, sequence gap, duplicate, or completion with wrong counts | Job fails with `INPUT_INVALID` or `PLUGIN_UNHEALTHY` | G1 | Fake worker |
| ADR10-009 | Worker opens a network listener | None opened in any test run (checked by the harness) | G1 | Integration |
| ADR10-010 | Every error code | Each maps to a catalog key in `messages/en.ts`; no screen matches English text | G1 | Go and frontend unit |

### 4.2 Reading files (G2)

| ID | Case | Pass condition | Gate | Layer |
| --- | --- | --- | --- | --- |
| PLG-004 | Corrupt input, expansion bomb, malformed records | Bounded, clean error; nothing written | G2 | Integration, hostile corpus |
| PLG-009 | Source changed during the job | Digest is taken from the opened handle; the mismatch is rejected | G2 | Go unit |
| PLG-028 | Names that try path traversal or spreadsheet formulas | Sanitized; no path escape; no live formula in reports | G2 | Go unit |
| SCALE-006 | ProjectLibre files | Read through `ProjectLibreReader` with a fixture, or not advertised | G2 | Integration |
| SCALE-007 | MPP on a machine without Microsoft Project | The packaged worker reads it offline | G2 | Packaged integration |
| REVIEW-005 | T2 MPP with a seekable scratch copy | JVM heap, host memory, and scratch measured; caps enforced | G2 | Performance harness |
| ADR10-011 | Wrong extension; empty file | Detected by signature, or `INPUT_INVALID` | G2 | Integration |
| ADR10-012 | XML with external entities | Entities never resolved; no file or network read | G2 | Java bridge test |
| ADR10-013 | Password-protected MPP | `INPUT_PROTECTED`; no secret in logs, receipts, or errors | G2 | Integration |
| ADR10-014 | Estimate exceeds free memory or disk | `RESOURCE_LIMIT` before parsing; inspection still offered | G2 | Go unit |
| ADR10-015 | Inspection of every fixture | Counts equal MPXJ's own counts; no database written | G2 | Integration |

### 4.3 Import as a new project (S2-S4, G3)

| ID | Case | Pass condition | Gate | Layer |
| --- | --- | --- | --- | --- |
| PLG-007 | Sign out or switch account after review | `STALE_SESSION`; nothing published | G3 | Go unit |
| PLG-008 | Switch project after review | No write to any project | G3 | Go unit |
| PLG-010 | Duplicate approval or replayed receipt | Publishes once; the second request returns the first receipt | G3 | Go unit |
| PLG-011 | 11 baselines, custom fields, timephased data | Every field counted; unsupported ones shown | G3 | Integration |
| PLG-012 | Negative cost, unusual currencies, JPY with a fraction | Source precision kept; conversion checked (ADR-009) | S4 | Go unit |
| PLG-013 | P6 calendar inheritance and holidays | Compared accurately, or warned as unsupported | S4 | Integration |
| PLG-014 | Unresolved external predecessor | Kept as unresolved with its source; no task invented | S4 | Go unit |
| PLG-015 | Import above the WBS caps (8 MiB, 10,000 nodes) | Stored outside WBS, or refused; never truncated | S3 | Go unit |
| PLG-016 | Disk full during import | No project published; existing projects unchanged | G3 | Fault injection (§5) |
| PLG-021 | Two accounts or projects with overlapping jobs | No crossover of identity or project | G3 | Race |
| PLG-026 | Crash after publication, before the receipt reaches the screen | The receipt is inside the published project; reconciliation shows it; nothing written twice | G3 | Fault injection |
| SCALE-002 | More than 50,000 activities | Stored and reopened with exact counts; summary and leaf counted separately | S3 | Integration |
| SCALE-003 | WBS boundary | WBS v2 still refuses above its caps; imports cannot bypass it | S3 | Go unit |
| SCALE-004 | MSPDI summary and null rows | Kept or reported; no summary relation lost | S4 | Integration |
| SCALE-005 | 12-hour, 6-hour, and custom calendars | Exact durations and the source's working-time settings kept | S4 | Integration |
| SCALE-008 | Field parity across MPP, MSPDI, XER, PMXML | The evidence matrix (plan §4.2) is complete for each | S4 | Integration |
| SCALE-009 | Field the source has and GoPMgr does not model | Kept read-only, or `unsupported_by_reader`; never dropped | S4 | Integration |
| SCALE-013 | Wrong copy, scope, or session | Rejected before any read or write; receipts cannot be replayed on another copy | S3 | Go unit |
| SCALE-016 | 11 baselines and irregular timephased data | Null and absent kept distinct; units, precision, and boundaries kept | S4 | Integration |
| SCALE-017 | Exact money | Follows ADR-009; no drift from floats | S4 | Go unit |
| SCALE-018 | Resource names collide | Assignments use resource IDs, not names | S4 | Go unit |
| SCALE-021 | Crash during a large import or a WAL checkpoint | No partial project; recovery is deterministic | G3 | Fault injection |
| SCALE-022 | Replaying a finished import | Same receipt; no duplicate rows | G3 | Go unit |
| SCALE-023 | Nesting, duplicate IDs, external links | No invented relations or cycles; unresolved references explicit | S4 | Go unit |
| SCALE-028 | Upgrade, then open with an older build | Older build refuses or opens read-only, per the format gate | S2 | Upgrade test |
| REVIEW-001 | Re-import after one byte changes | Lineage kept; no duplicate entities | G3 | Integration |
| REVIEW-002 | Copied project with the same UIDs | No automatic merge; the user decides the lineage | G3 | Go unit |
| REVIEW-003 | Crash after staging commits, before publication | Restart publishes once or quarantines | G3 | Fault injection |
| REVIEW-004 | Crash after rename, before the journal says PUBLISHED | Recovery finds the project once. *GoPMgr has no catalog step; the rename itself makes the project visible, so this checks the second SEALED row of plan §6.3.* | G3 | Fault injection |
| REVIEW-011 | Older build opens a newer project | Refuses or opens read-only; never writes | S2 | Upgrade test |
| REVIEW-015 | Fields MPXJ does not read | Reported as `unsupported_by_reader`; no claim of full coverage | S4 | Integration |
| REVIEW-016 | Out of resources during import or restore | Existing project unchanged; staging cleaned or recoverable | G3 | Fault injection |
| REVIEW-018 | Cancel at every job state | No ambiguous end state; a late cancel shows the receipt | G3 | Go unit (table over states) |
| ADR10-016 | Staging on a different volume from `projects/` | Publication refused (device IDs differ) | G3 | Go unit |
| ADR10-017 | Journal found in each phase (plan §6.3), and a folder with no journal | The listed action; a folder without a journal is never deleted | G3 | Go unit |
| ADR10-018 | `ListProjects` during an import | Staging and quarantine never appear | G3 | Go unit |
| ADR10-019 | Folder name taken at publication, including an empty folder created just before the rename | The no-replace rename fails; another name chosen; nothing replaced, on each OS | G3 | Go unit, per OS |
| ADR10-020 | Cancel from `RECHECK` on | Refused; the screen says the import is finishing | G3 | Go and frontend unit |
| ADR10-021 | Receipt v2 | Today's `MSPDIImportReceipt` fields keep their meaning; legacy receipts still read | G3 | Go unit |
| ADR10-022 | Counts above 2^53 in a receipt | Exact after JSON round trip (decimal strings) | G3 | Go unit |
| ADR10-023 | Date-only values; unset, null, zero, and unsupported values | Date-only stays date-only; the four missing states stay distinct | S4 | Go unit |
| ADR10-024 | Format gate | The gate release sets and checks `user_version`; an unknown newer version is refused | S2 | Go unit and upgrade test |
| ADR10-025 | Saved import report | Leaves out notes, local paths, and values unless the user includes them | G3 | Go unit |
| ADR10-026 | Source scratch copy | Private modes; removed on success, failure, cancel, and crash at the next start | G2 | Go unit |

### 4.4 Large schedules, compare, and export (S5, S6, G4)

| ID | Case | Pass condition | Gate | Layer |
| --- | --- | --- | --- | --- |
| PLG-017 | Export check fails, or the destination exists | Nothing overwritten; destination intact | G4 | Go unit |
| PLG-018 | Ask for `.mpp` export | `EXPORT_UNSUPPORTED` with the MSPDI suggestion | G4 | Go and frontend unit |
| PLG-029 | Re-import into an existing WBS (future) | Explicit mapping and revision rules; no name match | S6 | Go unit |
| SCALE-010 | Two-way compare | Lineage matched or unresolved; no writes; stable pages | S6 | Integration |
| SCALE-011 | Three-way conflicting edit | Conflict shown; no automatic "source wins"; stale revision refused | S6 | Go unit |
| SCALE-012 | Apply selected differences | Only approved fields change, atomically | S6 | Go unit |
| SCALE-014 | Timeline at 50,000 tasks | Only the viewport is drawn; DOM, memory, and transfer bounded; keyboard and screen reader work | S5 | Frontend and manual (§7) |
| SCALE-015 | Edit one task in a 50,000-task plan | Bounded writes; no full chart JSON; audited delta | S5 | Go unit and benchmark |
| SCALE-024 | Filter, sort, and paging | One revision returns every row exactly once | S5 | Go unit |
| SCALE-029 | Offline installer | Worker and interface work offline on each OS | G4 | Packaged, manual |
| SCALE-030 | Full user acceptance | Create, inspect, compare, import, cancel, and export with honest reports | G4 | Manual checklist |
| REVIEW-006 | Timephased data in MSPDI export | Required MPXJ options on; data exported or reported | G4 | Integration |
| REVIEW-008 | Cursor across writes | `revision_changed`; no silent skip or repeat | S5 | Go unit |
| REVIEW-009 | Slow pager during imports | WAL checkpoints still progress; WAL size bounded | S5 | Integration |
| REVIEW-010 | Field edit on a real 50,000-task schedule | Bounded writes; correct cache invalidation | S5 | Benchmark |
| REVIEW-013 | Compare with filters and off-screen selections | The selection count shows hidden rows; none applied unseen | S6 | Frontend unit |
| REVIEW-014 | Screen reader and keyboard on a 50,000-row tree | Focus, position, and navigation stay predictable | S5 | Manual (§7) |
| REVIEW-020 | Imported dates against GoPMgr's recalculation | Source values kept; differences explained | S6 | Integration |
| ADR10-027 | Cursor tampered with, expired, or used on another project | Rejected | S5 | Go unit |
| ADR10-028 | Focus after paging, sorting, or folding | The selected task keeps focus | S5 | Frontend unit |

### 4.5 Analytics and audit (S7A, S7B)

| ID | Case | Pass condition | Gate | Layer |
| --- | --- | --- | --- | --- |
| SCALE-001 | `ImportTabular` with 10,001 rows | The legacy API reports its limit; the new path stores all rows and row 10,001 can be paged | S7A | Go unit |
| SCALE-019 | Old and new audit events together | Old hashes unchanged; mixed chain verifies; replay rebuilds approved versions | S7B | Go unit |
| SCALE-020 | Checkpoint or evidence deleted or altered | Verifier detects it; no PASS | S7B | Go unit |
| SCALE-025 | Query authorization | No sibling-project leak; no escrow bypass; no direct worker or AI read | S7A | Go unit |
| SCALE-026 | Analytics results | Match the kernel and `internal/money`, or block | S7A | Go unit |
| SCALE-027 | Dataset limits | Memory, size, row, and query budgets hold; a truncated result never counts as success | S7A | Go unit |
| REVIEW-012 | Old chain and new typed deltas | Old hashes unchanged; money and large integers exact | S7B | Go unit |
| REVIEW-019 | Portfolio query across projects | Only approved snapshots used; no stale user | S7A | Go unit |
| ADR10-029 | Legacy result above 10,000 rows | `result_truncated` and `dataset_fully_imported` reported separately | S7A | Go unit |
| ADR10-030 | Receipt, undo, and audit retention | Each follows its own rule; the 256-receipt limit applies to WBS only | S7B | Go unit |

### 4.6 AI and third-party workers (G5-G7)

These cases are owned by the future ADRs. They are listed so no ID is lost.

| ID | Case | Gate |
| --- | --- | --- |
| PLG-022 | MCP access after sign-out | G5 |
| PLG-023 | A model tries to approve its own edit | G6 |
| PLG-030 | A third-party worker reaches nearby files or the network | G7 |
| REVIEW-007 | MCP connection during sign-out or project switch | G5 |

## 5. Publication fault injection

The publish package reaches the file system through a small interface, so
tests can make any single step fail. A test helper process is also killed
(`SIGKILL`) at each boundary to check recovery from a real crash.

| Boundary | Fault | Expected state after restart |
| --- | --- | --- |
| B1 | After `staging/<op>` is created, before the journal | Folder with no journal: quarantined, never deleted |
| B2 | During import, inside a transaction | PREPARED: quarantined; nothing in `projects/` |
| B3 | While the receipt and audit event are written, before the integrity check | PREPARED: quarantined |
| B4 | During the WAL checkpoint or the switch to DELETE mode | PREPARED: quarantined; the staged database still opens |
| B5 | After closing, before sidecars are removed | PREPARED: quarantined |
| B6 | After fsync, before the journal says SEALED | PREPARED: quarantined |
| B7 | After SEALED, before the session recheck | SEALED with the staged project present: seal digest checked; the user is asked |
| B8 | Session changed at the recheck | `STALE_SESSION`; staging removed; nothing published |
| B9 | Rename refused (name taken, permission denied, different volume, no-replace call unsupported) | Another name tried, or the job fails; staging kept for retry; nothing replaced |
| B10 | After rename, before `projects/` is fsynced or the journal says PUBLISHED | SEALED with the staged project gone: the seal digest matches, so it is marked PUBLISHED; the project appears once |
| B11 | After PUBLISHED, before the read-back | PUBLISHED: reconciliation reads it back and verifies |
| B12 | After the read-back, before staging is removed | PUBLISHED: staging removed |

At every boundary, these must hold:

- `ListProjects` shows each project once, or not at all;
- every visible project already holds its receipt and audit event;
- existing projects are unchanged;
- no plaintext copy of project data is left behind.

The same faults are run with `SQLITE_FULL` and `EIO` in place of a crash.

**Limits.** True power loss is not simulated. Directory fsync cannot run on
Windows, and `F_FULLFSYNC` behaviour on macOS can only be checked on real
hardware. These are listed as manual G3 checks on each OS.

## 6. Performance plan

This applies P1-09.

**Profiles.** Measure on the two hardware profiles the owner sets at S0
(minimum and recommended). Each result records the machine, OS, and
toolchain.

**Workloads:**

- T0: 100-500 tasks;
- T1: 10,000;
- T2: 50,000;
- T3: 100,000 tasks with one million timephased points.

**Method:**

- Five runs per measurement, recorded as p50 and p95.
- Cold and warm runs are recorded separately.
- Memory is the peak of host plus worker, sampled every 100 ms.
- Disk is the peak use of the scratch, staging, and evidence folders.

**Metrics.** The stages in plan §7.11:

- open, convert, stage, seal, and publish;
- first viewport, scroll p95, search, and a single edit;
- recalculation and a risk run;
- export, snapshot, and restore.

**Baselines (S0).** Today's MSPDI import, WBS save, chart save, and
analytics import, measured at their current limits, so later phases can
show their change.

**Budgets.** Until the owner accepts them, budgets are reported, not
enforced. After that, a regression of more than 10% on the recommended
profile needs a note in the pull request, and a missed budget blocks the
gate.

## 7. Accessibility and localization

### 7.1 Automated (Vitest)

**ARIA attributes:**

- `Stepper`: `aria-current="step"`;
- `DataGrid`: `aria-sort`, `aria-rowcount`, and `aria-rowindex`;
- `TreeGrid`: `aria-level` and `aria-expanded`;
- `PhaseProgress`: its live region announces phase changes only.

**Keyboard and input:**

- Arrow keys move through grid cells, Home and End jump, and Enter opens.
- Focus stays on the selected row after a page loads.
- Enter does nothing while `isComposing` is true (Japanese and Korean input).

**Colour.** Every `StatusBadge` renders a text label.

**Copy:**

- Every new component has zero literal text in the ratchet
  (`hardcoded-text.test.ts`).
- Every key in plan §9.4 exists in `messages/en.ts` with the same
  placeholders.

### 7.2 Manual, per gate with interface work

**Screen readers:** VoiceOver on macOS, NVDA on Windows, and Orca on Linux.
For each, run the assistant from start to finish and the inspector filters.
At S5, navigate the 50,000-row tree.

**Zoom and keyboard:**

- At 200% zoom, nothing is clipped and buttons wrap.
- The whole import works with the keyboard alone.

**Text expansion.** A German-length test catalog shows that buttons wrap
rather than truncate.

**Plan.** Follow `docs/design/accessibility-remaining-work-plan.md` where it
overlaps.

## 8. Evidence and PASS discipline

### 8.1 Results

| Result | Meaning |
| --- | --- |
| PASS | Ran on the recorded HEAD in this session and met its condition |
| FAIL | Ran and did not meet it |
| BLOCKED | Could not run: a missing tool, OS, fixture, or licence. Says what is missing |
| SKIPPED | Deliberately not run, with the reason. Never counted as PASS |

### 8.2 Rules

- A gate passes only when every case assigned to it is PASS.
- A BLOCKED case keeps the gate open, unless the owner records an exception
  with its reason and a date to revisit it.
- A result from an earlier HEAD is restated as "last passed at `<sha>`", not
  as PASS.
- An earlier SKIPPED case is never reported as PASS.
- Each gate's evidence is a dated test plan in `docs/testing/`, as in
  `recent-work-test-plan-2026-10-23.md`, and its pull request links to it.

### 8.3 Record for each case

```json
{
  "id": "PLG-016",
  "result": "PASS",
  "head": "<sha>",
  "os": "darwin-arm64",
  "toolchain": {"go": "1.27.3", "jdk": "<version>", "mpxj": "16.10.0"},
  "command": "go test ./internal/interop/publish/ -run TestPublishFaults/B2",
  "fixtures": [{"id": "mpp-2019-small", "sha256": "<digest>"}],
  "duration_ms": 0,
  "notes": ""
}
```

Records hold no project content, no local user paths, and no secrets.
