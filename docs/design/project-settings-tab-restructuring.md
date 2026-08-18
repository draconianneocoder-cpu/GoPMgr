# Project Settings tab restructuring

Status: implemented 2026-08-18.

## 1. Problem statement

Two findings from the design-critique pass that produced the Dashboard IA
proposal (`dashboard-ia-restructuring-proposal.md`), both on
`project/ProjectSettings.svelte`, neither fixed yet:

1. **Flat form.** The screen is one 1976-line vertical scroll of 10
   sections (Identity, Classification, Lifecycle, What-if Scenarios,
   Resource Capacity, Schedule Reports/CPM, Project Backup, Database
   Encryption, Export & Signature Settings, Document Font) with no
   grouping. `dashboard-ia-restructuring-proposal.md` §4.1/§6 already named
   this screen as `Tabs.svelte`'s intended second consumer.
2. **Methodology field inconsistency.** `ProjectLaunchpad.svelte`'s
   creation wizard presents methodology as a 3-card picker scoped to the
   chosen industry (`METHODOLOGIES[industry]`); this screen renders the
   same field as a bare freeform `<input>` with a placeholder hint,
   inconsistent with how the value was originally captured.

## 2. Requirements / constraints

- No Go/Wails-bound method signature changes.
- No change to any section's own internal logic — this is IA + one
  targeted field-type fix, not a rewrite.
- `Tabs.svelte` reused as-is (no API changes) — this is its first
  validation as a second consumer.
- `methodology` is stored server-side as an **unvalidated free string**
  (`UpdateProjectIndustry`, `app_foundation.go:201` — confirmed by
  reading the Go source, no whitelist). A project's stored value can
  legitimately be outside any industry's recommended list (older data,
  direct API/db use, or the industry changed after creation). The fix
  must never silently drop or rewrite an out-of-list value on save.

## 3. Tab grouping

| Tab | Contents | Rationale |
|---|---|---|
| General (default) | Identity + Classification + Lifecycle | Binds to `draft`/`save()`/`revert()`; the "go back and reclassify" surface, so it's the natural landing tab. |
| Scenarios | What-if Scenarios | Largest section (~324 lines), fully self-contained state. |
| Resources | Resource Capacity | Self-contained (resource calendars). |
| Exports & Signing | Schedule Reports (CPM) + Export & Signature Settings + Document Font | All three answer "how do exported artifacts look/get signed." |
| Data Protection | Project Backup + Database Encryption | Isolated deliberately, not folded into Exports & Signing, so the two highest-stakes actions on this screen (encrypt the database, create a backup) stay easy to find rather than buried in a larger tab — the opposite of the original "hard to find things" critique. |

Header (title, error/status messages, Save/Revert) stays outside all tab
panels, matching the Dashboard R3/R4 precedent for cross-cutting state.

## 4. Three decisions forced by pre-implementation review

**4.1 Save/Revert scope and labels.** `revert()` discards every field
`draft` covers (name, owner, description, industry, sub_category,
methodology, country, timezone, status, phase, dates, budget) — all of
which live on the General tab. Kept the buttons global (simplest, no new
per-tab dirty-indicator UI) but **relabeled them** so their scope survives
a tab switch: "Revert" → "Revert details", "Save changes" → "Save
details" (busy state "Saving…" → "Saving details…"). A user on the
Scenarios tab now sees "Revert details" / "Save details", not a
scope-ambiguous "Revert" / "Save changes".

**4.2 `$state` persistence across `{#if activeTab === 'x'}`.** Confirmed
before implementing, not assumed: every non-`draft` section's state
(`scenarios`, `scenarioDraft`, `scenarioChartDraft`, `scenarioComparisons`,
`resourceCalendars`, `resourceCalendarDraft`, `exportTheme`,
`signatureMethod`, encryption state, `fonts`, etc.) is declared at
`ProjectSettings.svelte`'s own top-level `<script>` scope, not inside a
child component. Wrapping a `<section>` in `{#if activeTab === 'x'}` only
conditionally renders a DOM subtree within the *same* component
instance — the component itself never unmounts, so its `$state` fields
are untouched by a tab switch. This is the opposite situation from
`ChartCatalog.svelte` (a separate child component whose own internal
`$state` really did reset on remount, fixed via `$bindable()` in the prior
cycle). No `$bindable()` lifting is needed here. This is guaranteed by
Svelte's component model, not something that could be wrong in this file
as written, so it's confirmed by the structural argument above rather
than by a fault-seeded test — there is no way to fault-seed a framework
guarantee. §6 adds a regression-guard test anyway, for the different
purpose of catching a *future* refactor that extracts a tab's content
into its own child component (the exact failure mode `ChartCatalog.svelte`
had before its own `$bindable()` fix).

**4.3 Methodology clears on Industry change, mirroring the wizard.**
`ProjectLaunchpad.svelte`'s `selectIndustry()` sets `methodology = ''`
whenever industry changes, since the recommended set is industry-scoped.
Settings mirrors this exactly on the Industry `<select>`'s `onchange` —
deliberate, not incidental, since the whole point of this fix is
consistency with the wizard. This only fires on a genuine user-driven
industry change (`bind:value` does not fire `onchange` on initial mount),
so loading an existing project never clears its methodology. Because nothing
is persisted until the user clicks "Save details" (and "Revert details"
undoes an accidental industry change before that), clearing is safe here
in a way it wouldn't be with an autosave-on-change field.

## 5. Methodology field fix

`METHODOLOGIES` (industry → recommended methodology list) previously
lived only in `ProjectLaunchpad.svelte`. Extracted to
`frontend/src/lib/methodologies.ts` as the single source of truth,
imported by both files — this closes the exact drift risk the original
finding was about (two screens describing the same constrained value
differently) at the data level, not just the widget level.

`ProjectSettings.svelte`'s Methodology field becomes a `<select>`:
- Options are `METHODOLOGIES[draft.industry]`, i.e. scoped to the
  currently-selected industry, same as the wizard.
- A `(none)` option is always present (Settings, unlike the wizard, must
  allow clearing methodology entirely — the wizard requires a selection to
  proceed past step 3, but an already-open project may have been created
  before methodology was tracked, or may legitimately have none).
- If the draft's current `methodology` value doesn't match any option in
  the current industry's list, it is prepended as an extra option labeled
  `"<value> (current)"` so it stays selected and visible — the value is
  never silently dropped or changed by merely opening the select.

A mid-implementation review raised a related risk: if `draft.methodology`
(or `draft.industry`) were ever `undefined` rather than `''` on load, the
`<select>` would bind to no matching option, which could read as an
unintended dirty state with no user edit. Checked directly rather than
assumed either way: `db.Project`'s `Industry`/`Methodology` fields have no
`omitempty` (`internal/db/project.go:39`), so a real `GetProjectMeta`
response always includes them, even empty — this specific failure mode
cannot occur through the real Wails binding. `onMount` still coerces both
fields with `?? ''` as defensive insurance (covers a stale mock or a
future field removal, not an observed defect), and a fault-seeded test
proves the coercion is what keeps the draft non-dirty on load, not
incidental behavior.

## 6. Test plan

Measure the existing suite's actual edit surface by running
`ProjectSettings.test.ts`'s 2 tests unmodified against the restructured
component first (both live in Export & Signature Settings, which moves to
the non-default "Exports & Signing" tab) — insert exactly the tab-switches
that measurement shows are needed, not more.

New tests:
- Tab wiring: default tab is General; each tab's content becomes reachable
  on switch; General's content is hidden while another tab is active.
- `$state` persistence: edit a field in a non-General tab (e.g. the
  Scenario name draft), switch to another tab and back, confirm the value
  survived. This is **not** a fault-seeded proof — a mid-implementation
  review asked for one, and there is no way to construct it: the
  persistence isn't a fix protecting against a bug in code that could
  regress, it's guaranteed by Svelte's own component model (state
  declared at a component's top-level `<script>` scope outlives any
  `{#if}` block toggling within that same component instance; there is no
  code path in this file that could accidentally break it while it stays
  a single component). The test's real purpose is a regression guard
  against a *future* change that extracts one of these tabs' content into
  its own child component — the exact failure mode `ChartCatalog.svelte`
  had before the prior cycle's `$bindable()` fix — not evidence for a
  claim about the current code.
- Methodology: an out-of-list current value renders as a selectable
  `"<value> (current)"` option and round-trips unchanged if the user
  doesn't touch it; changing Industry clears Methodology; reverting after
  an Industry change restores both fields together with the restored
  methodology selectable, not orphaned; loading a project whose
  `methodology` is missing entirely (defensive case — `db.Project`'s
  `Methodology`/`Industry` fields have no `omitempty`, confirmed by
  reading `internal/db/project.go:39`, so a real `GetProjectMeta` payload
  always includes them; this guards a mock/future-field-removal case, not
  an observed defect) does not read the draft as dirty. This last case
  was fault-seeded (temporarily removed the `?? ''` coercion in
  `onMount`, confirmed the test fails without it, restored) — unlike the
  `$state` persistence test above, this one *is* protecting a real code
  path and the fault-seed is meaningful.

## 7. What does NOT change

- Any section's internal script logic, beyond the Methodology field and
  the Industry `onchange` clear.
- Any Go/Wails method signature.
- `Tabs.svelte` itself.
- The Industry field's own option list (already consistent between this
  screen and the wizard — checked, not assumed: both list
  business/administration/engineering/software/construction/custom).

## 8. Residual-risk plan

This file is 48% test-covered per `TEST_COVERAGE_LEDGER.md` — materially
higher blind-spot risk than `Dashboard.svelte` had going into its own R3/R4
cycle. Live-verification in a real `wails dev` build is planned as part of
*this* cycle, not deferred, specifically because static verification alone
is weaker evidence here.
