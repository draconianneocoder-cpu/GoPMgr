<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Stakeholder errors in the exit guard

## Contract and acceptance

Baseline: `39676948772b57c2e2b977c99b5a9e277de5937c`.
`StakeholderManager` retained specific inline errors but returned only `false`
to the autosave coordinator, which substituted its generic foreground error.
The registered callback now forwards the existing nonempty message by throwing
an `Error` after an attempted save fails. Absent or busy drafts refuse first,
without interpreting a retained message as a new save failure.

All acceptance criteria passed in the bounded checks below:

- Name validation, unsafe hourly-rate/contract-value minor units and backend
  failures reach `navigation.error` with their existing exact text.
- Failure retains the draft, dirty state and pending destination, without
  navigation or native-close completion. Unsafe money never reaches the writer.
- Cancel/rearm and corrected retry recover; stale backend details do not replace
  a subsequent validation error.
- A busy manual save refuses an overlapping guarded write with the generic
  fallback, even if a late list failure has populated an unrelated inline error.
  Its eventual completion does not itself grant pending native-close permission.
- Existing canonical-money, per-field normalization, immutable payload,
  late-edit/rebase and per-open registration tests remain passing.

Only the private registered callback changes. Manual boolean save behavior,
shared coordinator/session interfaces, payloads, IDs, defaults, schemas,
backend transactions, markup and styling are unchanged. No migration or new
dependency is needed.

## Regression evidence

The stakeholder suite grows from 22 to 28 cases. Eight exact-message assertions
failed against the original callback, then passed with the adapter. Tests check
the foreground `navigation.error`, not merely text elsewhere in the DOM.
New cases cover new-draft Error/string/empty-Error rejections, retry, a previous
backend failure followed by name validation, and a controlled busy save with
an unrelated late refresh error. Existing native-close recovery assertions now
include Cancel/rearm; unsafe-money rejection covers both money fields.

Removing only the registered readiness check in an isolated test transform
made the busy test fail: it received the late list error instead of the generic
refusal. Removing only message forwarding made eight exact-message checks
fail again. The production source was not mutated. The normal focused suite also
passed with shuffled order (seed `20261021`). Deferred test operations are
settled and awaited in `finally`.

Reproduce the focused 48-case component/coordinator check:

```sh
npm --prefix frontend test -- --run src/lib/components/project/StakeholderManager.test.ts src/lib/autosave.test.ts src/lib/session.test.ts src/lib/native-close.test.ts
```

Fresh October 21 validation also passed: 637 frontend tests across 81 files;
Svelte check (zero errors/warnings); frontend lint; `make verify` including
root/internal/tools Go tests and repository contracts; production `make build`;
strict deep bundle-signature verification; frontend runtime smoke (607 rendered
bytes); focused native-close Go race tests; `go mod verify`; dependency audit
(zero advisories); REUSE lint; and release-scope checks. Official Svelte analysis
reported no issues, with an unchanged refresh-effect advisory.

## Browser trial and limits

Six saved, opened screenshots cover baseline entry, invalid name, generic
foreground failure, candidate specific message, Cancel/correction, and guarded
retry to one corrected list row. The private fixture uses the real App, router,
editor and guard with a disposable in-memory backend. Visible money values
remain unchanged. This is not native quit or database commit/reopen evidence.
Rejected fixture setup failures and a replaced intermediate capture are not
accepted product evidence; private screenshots/configuration are not release
artifacts.

Six canvas diagnostics were reproduced by unchanged DORA component tests under
jsdom; they do not establish chart rendering. Existing Wails `time.Time` binding
messages and the macOS 13-object/11-link target warning remain. Ad-hoc signature
verification does not establish publisher trust. No native GUI trial, full Go
race suite, Go advisory scan, Linux/Windows native trial, assistive-technology
audit or complete release/export matrix was run for this frontend-only adapter.

Lesson: preserve error details at the boolean-to-coordinator boundary, and
assert both the current foreground message and prohibited side effects. A busy
refusal must not turn an unrelated retained error into a claimed save failure.
