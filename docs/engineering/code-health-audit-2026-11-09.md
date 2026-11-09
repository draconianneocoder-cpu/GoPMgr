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
delete it. [ADR-008](../design/ADR-008-localization.md) already decides how:
its methodology words become catalog entries, and translations use full
sentences rather than joined fragments. Calling `term()` from components
would conflict with that, so the wiring is the next ADR-008 Phase 1 slice,
the extraction of the work-item editors. Until then the module stays unused,
and the Help screen, user guide and coverage ledger no longer claim
otherwise.

## Re-running the audit

```sh
GOBIN="$PWD/.tmp/deadcode" go install golang.org/x/tools/cmd/deadcode@v0.45.0
.tmp/deadcode/deadcode -tags production . ./tools/...
GOOS=windows .tmp/deadcode/deadcode -tags production . ./tools/...
.tmp/deadcode/deadcode -test -tags production . ./internal/... ./tools/...
```

Run the linux pass on a Linux host; the cgo GTK and WebKit packages do not
load from macOS. `deadcode` cannot see methods Wails binds by reflection, so
compare `code-map/app-api.json` with the frontend's `desktopApi.` calls too.
