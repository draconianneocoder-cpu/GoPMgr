<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# WBS foundation evidence, October 2026

Consecutive evidence for one WBS foundation: model validation, write ordering, storage boundary.

Each section below is a record kept as written, with its headings one
level lower and its links updated; nothing else changed.

## What this file holds

- [Unified WBS model assurance](#unified-wbs-model-assurance): formerly `docs/testing/wbs-model-validation-2026-10-13.md`, 2026-10-13, from commit `9ab5d91`.
- [WBS write ordering assurance](#wbs-write-ordering-assurance): formerly `docs/testing/wbs-ordering-2026-10-13.md`, 2026-10-13, from commit `9ab5d91`.
- [WBS storage boundary evidence](#wbs-storage-boundary-evidence): formerly `docs/testing/wbs-storage-boundary-2026-10-14.md`, 2026-10-14, from commit `9ab5d91`.

## Unified WBS model assurance

Scope: the approved pure model/identity/revision/validation slice, based on
`5e97f75`. The two design documents already belonged to this initiative; no
unrelated source edits or staged changes existed at entry. No runtime path,
legacy parser, schema migration, dependency or chart registry changed.

### Requirements and evidence

| Requirement | Evidence |
| --- | --- |
| Preserve scope identity, order and definitions | Literal malformed-input fixtures and semantic round trips retain IDs, duplicate display numbers, dictionary text, classification and finite negative legacy effort |
| Fail closed on ambiguous writable data | Missing required fields/IDs, unknown or case-aliased members, escaped duplicate keys, nulls, wrong shapes, invalid UTF-8, unpaired surrogates and trailing values return errors without partial models |
| Bound resource use | Exact/over-limit payload, text, identifier, node, hierarchy and combined association cases; decoded Unicode/escaped strings; wide/deep malformed input; encoded escaping expansion |
| Reject invalid topology and assignments | Nil/cyclic/aliased nodes, repeated IDs, missing/nonleaf scope targets and repeated sources; distinct schedule/activity tuples remain distinct |
| Preserve exact revision/context semantics | Zero, above-JavaScript-range and maximum `uint64` tokens; noncanonical strings and overflow fail; each copy/project/chart/lifetime axis and expected revision checked separately |

Independent plan review required explicit required/optional fields, hierarchy
versus container-depth accounting and common validation of Go-created models.
These were resolved before implementation. Classification remains uninterpreted;
empty text is valid; unknown fields are rejected rather than silently stripped.

### Reproduction

Local macOS arm64, Go 1.27.2, no toolchain/dependency change:

- Baseline `go test ./internal/charts/dag ./internal/db` and
  `go test . ./internal/...`: passed.
- Final `go test . ./internal/...`: passed; unchanged packages reused the
  baseline cache, while the new package executed its tests.
- `go test -race -shuffle=20261013 -count=10 ./internal/wbs
  ./internal/charts/dag ./internal/db`: passed, zero observed failures.
- New-package repetition with seed `20261014` (10 runs) and final expanded
  boundary fixtures with seed `20261016` (3 runs), also race-enabled: passed.
- `go test ./internal/wbs -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s
  -parallel=2`: passed, 269,293 executions, zero observed failures.
- `go vet . ./internal/...`, `golangci-lint run ./internal/wbs/...`, `make lint-go` and
  `go build ./internal/wbs`: passed.
- `go test -count=1 -cover ./internal/wbs`: passed. Coverage is diagnostic,
  not proof of safety; tests comprise 15 test functions and one fuzz target.
- `make code-map-current coverage-ledger-current`, `make release-scope` and
  `reuse lint`: passed.
- `make config-check` and `make coverage-ledger-drift`: passed; the latter
  checked default builds and DuckDB-tagged analytics, with 46 matching package
  headings and all 49 ledger headings accounted for.

Five compile-valid fault seeds were detected by targeted tests: per-array instead of
combined association limits; disabled leaf eligibility; activity identity missing
its schedule chart; context matching only copy identity; ignored unknown members.
Targeted tests failed for each and passed after restoration. An initial leaf seed
failed compilation and was discarded, not counted as behavioral sensitivity.
Self-review removed a fixture helper that could obscure encoding failures.

The independent final reviewer found no material issues. Its fresh package
tests, race check, vet and additional dictionary/link, exact encoded-size and
branching-tree probes passed. Exact encoded 8 MiB and branching 10,000-node
counterexamples were retained in the regression tests. Extended repetition and
fuzz totals above are this task's evidence, not claimed independent reruns.

### Boundaries and lessons

Strict standard-library decoding avoids a second JSON grammar. Explicit null
rejection and presence/zero-value rules are still necessary. Encoded size can
exceed decoded-text size through escaping. Hierarchy levels are not JSON depth.
Association keys must be tuples, not concatenated strings or display codes.

At this model milestone the package had no production caller. The later
[dormant storage foundation](../design/wbs-storage-boundary.md) now uses it
internally, without production application activation. Pure context matching
alone is not authorization or transactional compare-and-swap. The storage
foundation now covers receipts and atomic audit writes; external source
existence/ownership, unresolved-link warnings, migration, old-binary protection,
file exchange and UI behavior remain unimplemented. Native Wails,
Linux/Windows runtime and comparative usability were not tested for this slice;
no integrated parity or release-readiness claim follows from these results.

## WBS write ordering assurance

Baseline: `2cd31f4`, clean `main`. Selected priority: preventing an older WBS
preview from overwriting a newer acknowledged save. Signing/package-lifecycle
blockers require unavailable authority or infrastructure; UI migration is lower
risk. Public research found no relevant GoPMgr WBS reports, so frequency is
unknown. Repository behavior, not anecdotes, supplies the requirement.

### Mechanism and correction

Previously, preview and manual saves independently replaced the whole chart
payload. The installed Wails v2.16.0 Darwin dispatcher runs calls in separate
goroutines; `App.SaveChart` validates/scopes data, while `db.SaveChart` replaces
it without revision checking and returns only after commit. An older call
arriving last can therefore replace newer data. No backend revision policy,
schema, bridge signature, dependency, or presentation changed.

All WBS writes now use the registered autosave gate. Preview requests coalesce
around the active writer, then submit the latest draft. The registered callback
returns the exact writer promise: its coordinator waiter acknowledges first,
before a later preview waiter submits again. An async wrapper breaks that
handoff. Submitted JSON is captured synchronously; live edits are never replaced
by a response. In-flight writes remain dirty even after edit-back-to-original.

Registration precedes initial rendering and is removed on destruction. Activity
checks prevent late loads, writes and layouts from updating the editor or
starting queued writes. A committed write remains successful if layout fails,
with a separate visible error. Autosave registration accepts an optional stored
baseline for malformed-content recovery; existing callers retain their default.

### Evidence

Local macOS arm64, Go 1.27.2, Node 24.18.0, Vitest 5.0.3:

- Baseline: four WBS tests and the complete 594-test frontend suite passed;
  existing App chart-scope tests passed under the race detector.
- Before repair, six of seven new component cases failed, including guarded
  navigation succeeding during a pending older preview and edit-back-to-original
  appearing clean. A later fault seed wrapping `persist` in `async` failed the
  guarded-writer-to-preview test (two writes instead of the required three);
  the exact-promise implementation was restored. Fixtures apply persistence
  at release, not before a delayed response. Twelve new cases plus four existing
  styling cases now pass, including malformed and empty recovery baselines.
- Three affected frontend suites: 37 cases, 20 executions each, zero failures:
  `npm --prefix frontend test -- src/lib/components/charts/WBSEditor.test.ts
  src/lib/autosave.test.ts src/lib/components/documents/CharterEditor.test.ts
  --sequence.shuffle --sequence.seed=20261013 --repeats=19`.
  The restored implementation also passed 37 × 20 with seed `20261015`;
  after adding empty-content recovery, 38 × 20 passed with seed `20261016`.
- Full frontend: 81 files, 605 tests passed with seed `20261014`. Existing jsdom
  canvas-not-implemented diagnostics do not establish canvas rendering coverage.
  The final suite passed 606 tests with seed `20261016`.
- `go test -race -shuffle=20261013 -count=20 . -run
  'TestWBSChartAppCommitAndLayoutRoundTrip|TestChartAppMethodsRejectForeignProjectRowsInSameFile|TestSaveChartBindsNewChartToOpenProject'`
  passed. File-backed App tests verify committed payloads/layouts, reopen the
  database, and verify its five-event audit chain. These are not native transport
  tests or encrypted-database recovery tests.
- `go test -race -shuffle=20261013 -count=1 ./internal/db ./internal/charts/...`,
  `go vet . ./internal/...`, frontend check/lint/build and `make frontend-smoke`
  passed. Type checking reported zero errors and warnings.

Independent plan and mid-work reviews approved the bounded design. The plan
review exposed promise-identity and uncommitted-recovery-baseline hazards, both
addressed with regression coverage. Repeated samples are evidence, not proof of
zero flakiness. A fresh final adversarial reviewer approved with these residual
risks, independently passing the final 38 frontend cases and three targeted Go
race tests. Its stale-count documentation finding was corrected. Configuration,
code-map, coverage inventory, release-scope, REUSE (832/832), and diff whitespace
checks passed. Final self-review found no unrelated changes or temporary faults.

### Limits and lessons

Native Wails end-to-end execution, Linux/Windows runs and complete release gates
were not performed. The evidence confirms component scheduling and actual
App/database semantics separately, not a native-bridge reproduction. An already
submitted write may commit after Discard; destruction prevents subsequent queued
writes, not rollback. A hung bridge/layout keeps the guard busy; this change does
not add timeouts or cancellation. The next priority is investigating
`CauseEffectEditor.svelte`, which still has independent preview and manual
`SaveChart` calls. That source pattern is strong evidence, not a reproduced
native failure; it was not changed by this task.

Keep commit acknowledgement separate from rendering success. Control the commit
boundary in fixtures, not just response timing. Preserve exact promise ownership
when handing an operation from a coordinator to a later waiter.

Existing ADRs, schema/API documentation and immutable alpha release notes were
not changed because their contracts are unchanged. This report records the
bounded repair and verification for subsequent release planning.

## WBS storage boundary evidence

Scope: explicit internal storage initialization and revision-checked writes;
no application/session/renderer activation, upgrade or source association.
Baseline main `70a3e51` was clean with no staged or unrelated public work.

### Acceptance and counterexamples

| Requirement | Test evidence |
| --- | --- |
| One winner for stale concurrent saves | Independent database handles start at revision 1; exactly one commits and one conflicts; valid audit chain has only one additional event |
| Retried writes do not commit twice | Replay after a newer save and SQLCipher/plaintext reopen returns original revision/audit ID; current content remains newer |
| Identity reuse is refused | Same request ID with changed actor or title fails; copy/lifetime mismatch fails; closed store cannot save |
| Atomic content/revision/receipt/audit | SQLite triggers fail audit/receipt insertion and pruning at save257; no advanced chart or partial audit survives |
| Bounded receipts | Latest256 retained; replay does not refresh receipt1; expired create request conflicts rather than recreating |
| Fail-closed initialization | Two initializers preserve one copy ID; missing singleton, inconsistent revisions and partial schema fail |
| Semantic request identity | Empty dictionary/arrays and omitted equivalents replay; exact title/Unicode content persists |
| Protected generic paths | Submitted v2 kind/envelope and stored-record membership block generic update/retype/delete |
| Boundary safety | Cancelled create writes no chart; canonical max uint64 overflow refuses; lifetime Close leaves shared pool usable |
| Failed commit cleanup | Deferred foreign-key constraint forces COMMIT failure; discarded connection leaves no chart, receipt or audit mutation |

The commit-cleanup test initially failed three race-enabled runs: after the
reported failure, the pool still observed an uncommitted chart. Source inspection
confirmed `SQLiteTx.Commit` in the pinned driver rolls back only `SQLITE_BUSY`.
Go already marks `sql.Tx` finished, so deferred `Rollback` cannot rescue another
failure. The WBS writer now retains a dedicated `sql.Conn` through commit and
discards it via the documented `Raw`/`driver.ErrBadConn` mechanism on failure.
The unchanged regression passes after correction. The subsequent shared-driver
repair is tracked separately in [failed-COMMIT cleanup](sqlcipher-cleanup-corrections.md#failed-commit-cleanup-evidence);
the WBS-specific connection-disposal defense remains in place.

### Recorded local verification

- Baseline `go test -count=1 ./internal/wbs ./internal/db` and the application
  WBS commit/layout round-trip test passed.
- Initial focused WBS storage tests passed; ten shuffled race-enabled runs
  with seed20261014 passed before the deferred-constraint counterexample.
- Expanded post-correction race suite: seed20261015, three runs, passed.
- Final nine-test storage suite: seed20261017, three race-enabled runs, passed;
  final database regression suite passed at 77.6% statement coverage after
  dead-assignment cleanup. `make lint-go` passed with zero issues.
- Final `go vet . ./internal/...`, map and test-inventory checks passed.
- Production `make build` passed, retaining the existing macOS13-object versus
  macOS11-link-target warning.
- Broad Go regression suite passed before the dedicated-connection correction;
  final coverage-ledger drift reran the complete default Go coverage suites and
  DuckDB analytics successfully, with all 46 package headings matching.
- Static analysis, code-map generation/check, test inventory, configuration,
  release-scope and REUSE passed during implementation.

### Evidence limits and lessons

Internal caller-provided actor/lifetime IDs are a trusted boundary, not
authentication. Account incarnation, source incarnation, restored-copy reset,
old-application protection, runtime conflict UX and whole-file mutation paths
must precede production activation. Source associations deliberately fail closed.
Current native evidence is local macOS arm64; Windows/Linux runtime, bridge
transport, power-loss durability and full release gates were not exercised.

Keep replay lookup ahead of today's revision check; never label current content
as the original receipt's snapshot. Failed COMMIT is a resource-lifetime case,
not just an error return. Test it with a real deferred constraint rather than a
mock, and verify subsequent pooled operations as well as returned errors.
