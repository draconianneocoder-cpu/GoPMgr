<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# WBS write ordering assurance

Baseline: `2cd31f4`, clean `main`. Selected priority: preventing an older WBS
preview from overwriting a newer acknowledged save. Signing/package-lifecycle
blockers require unavailable authority or infrastructure; UI migration is lower
risk. Public research found no relevant GoPMgr WBS reports, so frequency is
unknown. Repository behavior, not anecdotes, supplies the requirement.

## Mechanism and correction

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

## Evidence

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

## Limits and lessons

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
