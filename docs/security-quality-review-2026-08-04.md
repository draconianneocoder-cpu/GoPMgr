<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# GoPMgr security, quality & stability review — 2026-08-04

Follow-up to the ROADMAP's "Next Recommended Audit" entry left by the
2026-08-04 PMForge → GoPMgr rename work: whether the frontend's hand-typed
copies of the project's four frozen persistence-boundary literals (the
`PMForge` data-root directory name, the `.pmforge` project-file extension,
the `project.pmforge` backup-archive entry name, and the `PMForge_Archive_`
backup-filename prefix) are pinned against drift the way the Go-side
originals already are.

## Verdict

**Two coverage gaps found and closed: one on the frontend, one on the
backend.** No literal had actually drifted — this is a coverage audit, not
an incident report — but before this session, four separate places had
nothing that would fail if one did: three frontend copies of the data-root
and `.pmforge` strings (two read directly by users in Help Guide text, one
a test fixture that exercises the same extension) and the backend's
`PMForge_Archive_` success path.

## Findings and fixes

### Frontend copies of the data-root and `.pmforge` strings were unpinned

`frontend/src/lib/components/HelpGuide.svelte` documents the real macOS
(`~/Library/Application Support/PMForge/`) and Linux/Windows
(`~/Documents/PMForge/`) data-root paths for users, plus the `.pmforge`
maintenance-CLI usage; `frontend/src/lib/components/project/Dashboard.svelte`
and `frontend/src/lib/components/project/ProjectLaunchpad.test.ts` each
reference the `.pmforge` extension once more. None of these were checked
against `internal/users/store.go`'s `DefaultRootDir()`, which is the actual
source of truth. A rename or find/replace pass here would send a user
looking at Help Guide text to a directory that doesn't exist, or a
frontend test fixture out of sync with what the backend actually accepts —
and nothing in CI would notice.

**Checked for the better fix first:** whether the app already exposes
`DefaultRootDir()`'s resolved value to the frontend over a Wails binding,
which would let the frontend render the real value instead of hardcoding a
second copy — the fix that removes the drift class rather than merely
detecting it. It does not: `main.go`'s `App` type binds only `Greet`,
`SetUnsavedChanges`, and the lifecycle hooks (`shutdown`, `beforeClose`); no
root-directory or log-path getter exists. Wiring one up is future work, not
in scope here (it would need a new bound method, a `wailsjs` bindings
regen, and a frontend call-site change — a larger change than a pinning
test, and arguably better scoped alongside the "add a real update-channel
binding" work already tracked elsewhere).

**Fix:** added `frontend/src/lib/persistence-boundary-strings.test.ts`,
following the top-level `frontend/src/lib/*.test.ts` convention (alongside
`autosave.test.ts`, `session.test.ts`, `theme.test.ts`) rather than living
under one specific component's directory, since it spans two component
directories. It imports each source file as raw text via Vite's `?raw`
suffix and asserts the exact literals are present. Like the Go pinning
tests, the expected strings are hardcoded rather than imported from a
shared constant — there is no single source importable from both a `.go`
file and a `.svelte` file, so hardcoding-with-a-comment is the same
trade-off `internal/db/backup_test.go` already makes for `project.pmforge`.

### The `PMForge_Archive_` prefix was referenced but never positively pinned

`internal/admin/workflow.go`'s `SecureArchive` generates backup filenames
with a `PMForge_Archive_` prefix, documented in `DEVELOPER_HANDBOOK.md` §9
as the fourth frozen literal (kept for continuity, not compatibility,
since nothing reads it back). The only existing reference to the literal
in tests was `TestSecureArchiveRemovesArchiveWhenCreatedAuditLogFails`
(`internal/admin/admin_test.go`), which globs for
`PMForge_Archive_*.pmba` but only ever asserts **zero** matches, on a path
where `SecureArchive` is expected to fail and remove its own output. An
absence check can't distinguish "renamed, and cleanup still worked" from
"still PMForge_Archive_, and cleanup still worked" — so this test would
have kept passing unchanged through a rename of the prefix.

**Fix:** added `TestSecureArchiveUsesPMForgeArchivePrefix`
(`internal/admin/admin_test.go`), which exercises `SecureArchive`'s success
path and asserts `PMForge_Archive_` against the real archive filename
returned, rather than checking for the absence of a pattern.

### Verification method

Every assertion added in this pass — the four frontend literals and the
one backend prefix — was verified to fail under a deliberate break of the
exact string it pins (temporarily renaming the literal at its source,
confirming the new/updated test goes red with the expected message, then
reverting), per the standard this codebase already holds Go pinning tests
to (`DEVELOPER_HANDBOOK.md` §9, 2026-08-04 first entry: "a pinning test has
to fail under the mutation it claims to catch, or it isn't pinning
anything"). No production code changed; `go build ./...`, `go test ./...`,
and the full `npx vitest run` frontend suite are green (61/61 frontend
tests, up from 57).

## Scope decision, initially deferred — resolved and implemented same day

**Update:** the decomposition below was worked through with the user later
the same session, who confirmed "include everything, add a migration"
covers all four literals with per-literal compatibility handling. All four
were renamed; see `DEVELOPER_HANDBOOK.md` §9's "PMForge → GoPMgr
persistence-literal rename, implemented" entry for what shipped, how each
literal's compatibility branch was verified (old-format fixtures, not
round-trip tests), and ROADMAP's "Next Recommended Audit" for the
user-facing summary. The original deferral reasoning is kept below
unedited — it's the record of *why* a plain "rename them" instruction
needed decomposing before acting on it, and every distinction it draws
still describes what was actually built.

---

A same-day follow-up request proposed renaming all four frozen literals to
`GoPMgr`/`gopmgr` outright as part of finishing the rebrand, which this
review's own fixes would then need to track. That request was **not**
implemented pending further scoping, because "rename with a migration" does
not mean the same thing for all four literals:

- **Data-root directory name** (`PMForge` → `GoPMgr`): has a real
  precedent — `users.MigrateLegacyRoot` already performs this shape of
  copy-based move for the pre-relocation macOS path. Tractable.
- **`.pmforge` extension** and **`project.pmforge` archive entry**: these
  name files that already exist in arbitrary user-chosen locations —
  Downloads folders, external drives, users' own cron jobs and scripts (see
  `HelpGuide.svelte`'s own documented `--check`/`--export` CLI usage). No
  migration routine run at app startup can reach a `.pmforge` file sitting
  on a USB drive. The only safe design is permanent dual-extension /
  dual-entry-name read support — a new compatibility surface maintained
  forever, not a one-time migration.
- **`PMForge_Archive_` prefix**: genuinely cosmetic (nothing reads it back
  per the existing handbook note), safe to rename on its own.

Proceeding on an unscoped "rename everything" instruction would reproduce
the exact class of change `DEVELOPER_HANDBOOK.md` §9 already documents as a
near-miss during the original rename. See ROADMAP's "Next Recommended
Audit" section for the per-literal decomposition offered back for a
decision.

## Priority

None blocking — all findings in this pass were coverage gaps, not active
defects, and both were closed within this review. The rename-scope
question was a product/data-safety decision, not a code-quality finding;
it was resolved with the user and implemented the same day (see the
addendum above).

## Scope / limits

Static review of the four frozen literals' non-Go-test references only
(frontend help text/fixtures, one backend test file). Did not re-audit the
Go-side pinning tests already covered by the prior 2026-08-04 entry, and
did not review the "Next Recommended Audit" candidates further down that
list (Wails root-dir binding, broader frontend test coverage). The rename
itself (all four literals, with compatibility handling) was implemented
as a separate, later step the same day — see `DEVELOPER_HANDBOOK.md` §9
rather than this document for that implementation's own scope and limits.
