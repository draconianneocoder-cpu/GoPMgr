<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Reliability repairs, 2026-10-12

Baseline: `3ac7f5b`. Scope: Charter editor save/lifecycle behavior and PAdES
harness ownership. No database, wire-format, dependency, or toolchain change.
This is bounded remediation, not a claim that the repository is flakiness-free.

## Confirmed mechanisms and repairs

| Impact | Mechanism | Repair and sensitive evidence |
| --- | --- | --- |
| High: lost document edits | A save response replaced a newer live title; content baseline used live rather than acknowledged content | Capture submitted state, rebase late edits, and retain the acknowledged title/content/status baseline. Separate A-to-B-to-A component cases fail on the original behavior. |
| High: navigation during a write | A draft restored to its old value appeared clean while another value was still being committed | All Charter writes use the same registered-entry gate/baseline; any in-flight entry is dirty. Tests require pending navigation before resolving the write and a second save of the restored value. |
| Medium: stale registration | Async document/schema loading resumed after destruction and registered an orphan draft | Activity checks precede state assignment/registration; unmount unregisters and late load failures are handled. Deferred resolve/reject cases cover both load stages. |
| Medium: stale error ownership | A save completing after unregister could clear or replace the new editor's error | A completion must still own its registered entry before acknowledgement/error reporting. Controlled success/rejection cases fail without membership checks; pending Charter saves after unmount cannot export or retain a draft. |
| High: evidence exclusion lost | External INT/TERM cleanup released the lock, continued validation, then EXIT cleanup removed a successor's lock | Signal handlers exit 130/143; only EXIT releases an owned lock. Readiness and cleanup barriers reproduce the actual double-removal schedule and prove successor/inherited-lock preservation. |
| Medium: owner report deleted | A failed contender or pre-lock rejection unconditionally removed shared temporary trust evidence | Report cleanup requires successful acquisition or inherited-lock permission. Timeout-zero and pre-lock sentinel tests preserve lock/report bytes. |
| Medium: prior-run dependency | Explicit-PDF validation skipped generation without creating its output directory | Initialize derived output under the lock. Fresh-fixture validation succeeds without a generator invocation and leaves a separate input byte-identical. |
| Medium: order-sensitive timer test | Gantt's cleanup test counted Svelte's zero-delay delegated-event task as an editor timer; prior clicks changed whether that task was already scheduled | Advance fake time by zero to execute due framework work, retaining exact one-owned-timer-before/zero-after-unmount assertions. Removing the production `clearTimeout` still fails the zero-after assertion. No Gantt runtime change remains. |

Manual Charter saves and heartbeat saves now acknowledge the submitted snapshot
through the same coordinator. Persistence success is distinct from readiness to
export/synchronize: those actions must also see a clean acknowledged draft.
An overlapping request fails closed without issuing another write; no retries,
sleep-based synchronization, or weakened assertions were introduced.

## Reproduction and verification

Run the focused lifecycle regressions with:

```sh
python3 -B scripts/pades-harness-lifecycle_test.py
npm --prefix frontend test -- src/lib/components/documents/CharterEditor.test.ts src/lib/autosave.test.ts --sequence.shuffle --sequence.seed=20261012 --repeats=19
make pades-harness-tests
```

The original PAdES scripts failed seven of eight new cases. New Charter
regressions exposed overlapping writes, premature export, missing status
dirty tracking, navigation bypass, orphan registration, and unhandled late
load rejection. A mistaken test assertion initially expected a boolean from
the void navigation API; it was corrected to assert pending navigation and
unchanged route, and failed again on the actual bypass.
Replacing the rebase with unconditional response assignment also failed all
three late-title/content/status cases; the corrected source was restored
byte-for-byte after this fault seed.

The final broader run also caught an implementation regression: successful
modal saves intentionally unregister their draft. Returning failure solely
because the entry disappeared blocked valid continuation. The correction
preserves committed success while limiting baseline/error mutation to entries
still registered. This distinguishes cancellation from successful self-cleanup
without weakening existing modal assertions.

The Python fixtures copy the real scripts into private temporary directories,
control every optional validator, synchronize by pipes rather than delays, and
reap their isolated process groups. They are lifecycle tests, not evidence of
cryptographic validity or trusted certificates; the existing real-generator
gate remains required. Component tests use the real coordinator and detached
responses with advanced server metadata, not a no-op autosave mock.

Local macOS arm64 verification (Go 1.27.2, Node 24.18.0, Python 3.14.8,
Bash 3.2.57):

- `go test -race -shuffle=20261012 -count=1 . ./internal/...` passed before
  changes; no Go source changed. A final `go test -run '^$' . ./internal/...
  ./tools/...` compiled the current frontend embed and tools without executing
  tests.
- Five affected frontend files: 70 cases, each executed 20 times with
  `--sequence.shuffle --sequence.seed=20261016 --repeats=19`; zero failures.
- Full frontend: 81 files, 594 tests passed with seed `20261016` after resolving
  the nine observed failures described above; seed `20261017` also passed.
  jsdom emitted its existing canvas-not-implemented diagnostics; canvas
  rendering is not verified here.
- PAdES lifecycle: 20 complete runs of eight cases, zero failures; an additional
  ResourceWarning-as-error run passed. `make pades-harness-tests` passed,
  including the real generator, validator matrices, locks, and parallel gates.
- `npm --prefix frontend run check`, `run lint`, `run build`,
  `make frontend-smoke`, shell syntax, ShellCheck, configuration/code-map/
  ledger-current/release-scope checks, REUSE, and diff whitespace checks passed.

Independent pre/post-change reviewers challenged A-to-B-to-A baselines,
inherited locks, successor ownership, and teardown errors. Broader tests
falsified the first modal-teardown interpretation; preserving committed success
resolved it. Local passes and repeated samples are evidence, not proof of a
zero failure probability.

## Boundaries and follow-up

- Parent-only signal tests cover the generator stage. Bash can defer a signal
  while foreground commands or the validation pipeline finish; immediate child
  cancellation, SIGKILL recovery, and hostile lock replacement are not proven.
- Supplied-input preservation is tested for a separate input path. Aliases or
  input collisions with derived-output paths need a separate confinement fix.
- WBS preview writes independently race guarded saves in a controlled-function
  probe. The subsequent [WBS ordering repair](wbs-ordering-2026-10-13.md)
  adds controlled component commits and real App/database evidence; native
  bridge execution remains a separate evidence gap.
- Other editors may have similar async lifetime/write paths. No broad rewrite
  or claim of closure follows from the Charter fix.
- The account-creation concurrency test's 300 ms scheduling window remains a
  test-assurance lead, not a reproduced production race.
- Native desktop behavior, Linux/Windows execution, signing-modal lifetime,
  and the full release-validation gate were not exercised by this task. The
  recorded Linux SSH host rejected non-interactive authentication, so no remote
  commands or tests ran. No hosted workflow was dispatched.
