<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Dashboard information-architecture restructuring — design proposal

**Status:** Proposed, not implemented
**Author's note:** This is a design document, not a change log. Nothing in
`frontend/src/lib/components/project/Dashboard.svelte`, `App.svelte`, or
`AppHeader.svelte` has changed as a result of this document. It exists to be
reviewed and either approved for its own implementation cycle or revised.

## 1. Problem statement

A 2026-08-17 design-critique pass (`docs/beta-release-backlog.md`'s Priority
#2 row; screenshots and live click-through, not a static read) found three
concrete defects in the app's navigation and its most-visited screen:

1. **Naming collision.** `AppHeader`'s nav highlights a tab labeled
   "DASHBOARD" for the `portfolio` view, but that view's own page heading
   reads "Portfolio dashboard." A *separate* view (`dashboard`, reached only
   from inside an open project) is independently labeled "Project
   dashboard." Two different screens are both, in different words, called
   "dashboard" — a user who bookmarks "the dashboard" in memory has no
   reliable way to know which one they mean, and the nav's own label doesn't
   match the page it points to.
2. **Unhandled drift fallback.** `session.view`'s type is a string union
   that includes `'charts'`, which `Dashboard.svelte` deliberately routes
   to when a saved chart's `kind` has no entry in its `chartRoutes` map —
   confirmed during drafting (§3.2) to be tested, intentional
   registry-drift infrastructure, not dead code as originally assumed.
   `App.svelte`'s `routeLoaders` map has no entry for `'charts'`, so
   reaching it renders the generic "Unknown view" safety screen, which
   gives no indication that the chart was, in fact, already saved
   successfully. Verified with zero drift today (§3.2) between the
   backend's chart registry and the frontend's `chartRoutes` map — every
   one of the 22 registered chart kinds' string values matches a
   `chartRoutes` key exactly, not just in count — so this fallback is not
   currently reachable through normal use, but the app has no mechanism
   preventing that from changing.
3. **Unstructured single-project dashboard.** `project/Dashboard.svelte` is
   one flat vertical scroll, confirmed live at roughly 2500px tall on a
   1280×800 viewport, with no tabs, sections, or in-page navigation.
   **Correction, made while implementing this document's own R1/R2 items
   and re-reading the file**: earlier drafts of this document described
   this as "five distinct kinds of content" and mapped "Documents list" to
   a single existing-work list. The file actually has seven content
   groups (plus a loading/error state that gates two of them but isn't
   itself content), in order: project-meta cards
   (Stakeholders/Timeline/Budget) → a Process Excellence quick-link
   (shown only for `six_sigma`-methodology projects) → *(loading/error
   state)* → an **existing charts** list → a separate **existing
   documents** list → a 24-item chart-creation tool catalog (2-column
   grid, lightly filterable by 4 category chips, all 24 shown unsorted by
   default) → a 25-item document-template catalog (behind one collapsed
   accordion) → a Software-Dev Pack section (the enable/disable control
   for the agile/Sigma tooling fixed under the "Fix launchpad wizard"
   backlog item) — seven, not five, once the existing-work lists are
   split out as the two distinct sections they are in the code, and the
   Sigma quick-link is counted as its own group rather than folded into
   the meta cards.
   A user opening a project to check its budget must scroll past the chart
   catalog to reach it if they landed below the meta cards, and the
   Software-Dev Pack toggle — which the Launchpad wizard's own copy tells
   users to expect enabled — is reachable only after scrolling through both
   catalogs.

None of these are new features. All three are legibility/navigability
defects in a shipped surface, which is why this is framed as an IA
restructuring, not a feature request.

## 2. Requirements

### Functional

- **R1.** Every distinct screen must have one unambiguous label, used
  consistently in both the nav (where applicable) and the page's own
  heading.
- **R2.** Every value in the `session.view` union that is reachable through
  a real code path (`goto()`/`requestNavigation()` call, not just declared
  in the type) must render a screen that explains the state it represents
  — not the generic "Unknown view" safety fallback, which is correct only
  for a value reached through a genuine bug. `'charts'` is reachable
  through intentional, tested drift-handling logic (§3.2), so it needs its
  own honest screen, not removal from the union.
- **R3.** A user must be able to reach any one of the seven content groups
  currently on `project/Dashboard.svelte` (meta cards, Process Excellence
  quick-link, existing charts, existing documents, chart catalog,
  document-template catalog, Software-Dev Pack — §1's corrected count) without
  scrolling past unrelated content to do it. **Correction, made while
  updating this document alongside the R3/R4 implementation**: this
  requirement's parenthetical previously still listed the pre-correction
  five-group taxonomy ("Documents" as one group, no separate charts/quick-link
  entries) after §1 and §3.3 had already been corrected to seven in the prior
  cycle — left stale by oversight, not a deliberate narrower requirement.
- **R4.** The Software-Dev Pack's enabled/disabled state and its toggle must
  be visible without scrolling past both catalogs, given the existing
  backlog finding that its current position contradicts the Launchpad
  wizard's own promise.

### Non-functional / constraints

- **C1. Single-maintainer, desktop app, no analytics.** There is no
  usage-telemetry pipeline to validate an IA change against real usage
  patterns after shipping — decisions here rely on the structural argument
  (content taxonomy, scroll depth) and the existing design-critique
  findings, not on click-through data. This should be stated as a limit on
  confidence, not glossed over.
- **C2. Minimize backend/wire-contract change.** This is a presentation-layer
  problem; nothing in `internal/`, `app_*.go`, or the Wails-bound method
  signatures needs to change to solve it. A design that requires new Go
  surface should be treated as over-scoped.
- **C3. Preserve existing keyboard/focus quality.** The same design-critique
  pass flagged the app's keyboard focus handling (tab order, visible
  `:focus-visible` ring) as a genuine strength. Any tabbed/sectioned
  restructuring must not regress it — tab controls need the same
  keyboard-operable pattern already used elsewhere (see §5).
- **C4. No `session.view` union churn beyond what's needed.** Every other
  view in the union is unaffected by this proposal; changes should be
  additive/corrective (fixing #2, adjusting #1's labels), not a wider
  refactor of the routing mechanism itself.
- **C5. Component-library alignment where it doesn't block the fix.** The
  in-progress Button/Card/Input/Select library (`frontend/src/lib/
  components/`) is mid-migration (9 of 77 screen files converted as of this
  writing). This proposal should use those components for any *new* UI it
  introduces (e.g., tab controls), but should not require migrating
  `Dashboard.svelte`'s existing catalog cards to `Card.svelte` as a
  precondition — that migration is already tracked separately and doing it
  as a side effect here would conflate two changes' worth of visual risk
  into one review.

## 3. High-level design

### 3.1 Naming collision (R1) — **Implemented 2026-08-17**

Two independent screens currently share the word "dashboard." The fix is a
naming decision, not a structural one:

| View | Current nav label | Current heading | Proposed nav label | Proposed heading |
| --- | --- | --- | --- | --- |
| `portfolio` | "DASHBOARD" | "Portfolio dashboard" | "PORTFOLIO" | "Portfolio" |
| `dashboard` (per-project) | *(no nav entry — reached from inside a project)* | *(none — see correction below)* | *(unchanged)* | *(unchanged)* |

**Correction to this table, made while implementing it**: the `dashboard`
row's "Current heading" cell originally said "Project dashboard," inherited
from the source design-critique without re-reading the code. That is not a
visible heading. `Dashboard.svelte`'s actual `<h1>` renders
`{session.project?.name}` — the open project's own name — not a static
string. "Project dashboard" exists only as `App.svelte`'s
`VIEW_LABELS['dashboard']`, a screen-reader-only string read by the
`aria-live` route announcer on navigation; a sighted user never sees it
printed anywhere. The real, visible collision was narrower than originally
described: the `portfolio` nav tab and its own heading, not a second
on-screen "Project dashboard" heading competing with it. Because
`VIEW_LABELS['dashboard']` doesn't visibly collide with anything once
`portfolio`'s label changes to "Portfolio," it was left unchanged — no
"Proposed heading" column entry was needed for the `dashboard` row.

Rationale: `portfolio` is the multi-project landing screen — "Portfolio" is
already the more specific, correct noun for it (a list of projects), and
dropping "dashboard" from its label removes the collision at the source
rather than renaming the screen that's actually shaped like a dashboard
(single project, KPI-style meta cards, tool catalogs). This is a two-string
change: `AppHeader.svelte`'s nav label and `Portfolio.svelte`'s heading
text. `VIEW_LABELS['portfolio']` in `App.svelte` (used for the
screen-reader route announcer) moves from `'Portfolio'` to stay consistent
— it already reads `'Portfolio'`, not `'Dashboard'`, so the announcer was
already correct; only the *visible* nav chip and page heading were wrong.

### 3.2 `'charts'` fallback route (R2) — corrected during drafting, **implemented 2026-08-17**

**The original design-critique framing of this as a "dead route" turned out
to be imprecise, caught by re-reading the code rather than trusting the
earlier inference.** `session.view`'s `'charts'` value is not unreachable
defensive code: `Dashboard.svelte`'s `newChart()`/`importMSPDI()` navigate
to `chartRoutes[kind] ?? 'charts'` after a chart is successfully saved, and
`Dashboard.test.ts` has a dedicated test —
`'falls back to the generic charts view for a chart kind with no
registered route'` — that mocks `ListChartKinds` to return a synthetic
`future_kind` entry and asserts `session.view` becomes `'charts'`. This is
deliberate, tested infrastructure for chart-kind *registry drift*: the
backend's chart registry (`internal/charts/registry.go`) and the
frontend's `chartRoutes` map (which routes a saved chart to its dedicated
editor view) are two separately-maintained lists that happen to agree
today — as of this writing, both list exactly 22 chart kinds, and every
one of the 22 *string values* matches (not just the count): extracting
each `Kind` constant's actual string literal from `registry.go` and each
key from `chartRoutes` and diffing the two sorted sets (`comm -3`) returns
empty. A count-only comparison would have missed a same-count value
mismatch — `KindStakeholder`'s value is `"stakeholder_analysis"`, not
`"stakeholder"`, for one example of how the Go constant name and the wire
string already diverge elsewhere in this same list, so checking names
instead of values would not have been reliable. But nothing enforces
these two lists stay in sync when a new chart kind is added to one and
not (yet) the other.

**So there is no dead route to remove.** What the original finding was
reaching for, more precisely: when registry drift *does* happen (a new
chart kind ships on the backend before its frontend editor and
`chartRoutes` entry exist), the chart is still saved successfully — the
`SaveChart` call already succeeded before this navigation runs — but the
user then lands on `App.svelte`'s generic "Unknown view" fallback screen,
which offers only "Back to dashboard" and gives no indication that their
new chart was, in fact, saved and still exists. That is a real (if
currently unreachable) UX gap in an intentionally-built safety net, not a
route to delete.

**Revised recommendation:** give the `'charts'` fallback its own small,
dedicated view — not a route users ever navigate to directly, only ever
landed on via this fallback — with copy along the lines of "`{title}` was
saved, but this chart type doesn't have a dedicated view yet. It's safe in
your project; check back after the next update, or return to the
dashboard." This is a small, self-contained addition: one new
`routeLoaders['charts']` entry, no change to `chartRoutes`, `SaveChart`, or
any Go surface (C2), and it turns a currently-silent "your work seems to
have vanished" moment into an honest one, for a state the codebase already
goes out of its way to test for. `Dashboard.test.ts`'s existing fallback
test needs no change — it only asserts `session.view === 'charts'`, not
what that view renders.

**Implementation note**: shipped as `ChartFallback.svelte`, fetching the
chart's title via the existing `GetChart(id)` binding (`session.editingId`
carries the id the fallback navigation already passes) so the copy can
name the specific chart rather than speaking generically — with a
graceful fallback to generic copy if that lookup itself fails, since a
failed title lookup is not evidence the chart wasn't saved. Verified the
"was saved" claim holds on both call sites that can reach this view
(`Dashboard.svelte`'s `newChart()` and `importMSPDI()`): both only call
`goto(chartRoutes[kind] ?? 'charts', c.id)` after their `SaveChart`/
`ImportMSPDIChartWithOptions` call has already resolved successfully, and
both wrap the call in `try`/`catch` that shows an error toast and never
navigates on failure.

### 3.3 Dashboard restructuring (R3, R4) — **Implemented 2026-08-17**, see §7

Proposed structure: a tabbed shell wrapping the seven existing content
groups (§1, corrected count), with the meta cards **and** the Process
Excellence quick-link promoted out of the tab set entirely — both are
orientation/navigation content every visit needs, not one option among
several, matching how they already behave today (always rendered, never
gated behind a click).

```
┌─────────────────────────────────────────────────────────┐
│  ← Back to portfolio          Project Name          ⋮   │  (existing header, unchanged)
├─────────────────────────────────────────────────────────┤
│  Stakeholders: 12   Timeline: Mar–Sep   Budget: $42,000  │  (meta cards, always visible)
│  [ DMAIC Workspace → ]                                   │  (Sigma quick-link, six_sigma only, always visible)
├─────────────────────────────────────────────────────────┤
│  [ Overview ]  [ Charts ]  [ Documents ]  [ Dev Tools ]  │  (new tab row)
├─────────────────────────────────────────────────────────┤
│                                                           │
│   (active tab's content — one of the four groups below)  │
│                                                           │
└─────────────────────────────────────────────────────────┘
```

| Tab | Content (from current `Dashboard.svelte`) |
| --- | --- |
| **Overview** (default) | **Correction**: not "the existing Documents list" (singular) — the file has two separate existing-work lists, an existing **charts** list and an existing **documents** list, both of which belong here together as "what you've already made in this project." This is the tab a user lands on — the highest-frequency task (checking recent work / project state) shouldn't be behind a click, matching R3's "reach without scrolling past unrelated content" for the single most common case. The loading/error state that currently gates both lists moves with them into this tab. |
| **Charts** | The 24-item chart-creation tool catalog, category chips unchanged. The MSPDI import controls (currently interleaved with this catalog) move here too, since they're part of the same "create a new chart" task. |
| **Documents** | The 25-item document-template catalog, no longer behind a collapsed accordion — the accordion was doing the job a tab now does (hiding low-frequency content), so a plain always-expanded grid is simpler once it has its own tab. The "Build combined report" action (currently in the same section) moves here too. |
| **Dev Tools** | The Software-Dev Pack section (R4) — reachable in one click from the tab row regardless of scroll position, closing the gap the "Fix launchpad wizard" backlog item already flagged from the enable-toggle's *positioning*, not just its default state. **Correction, made while re-reading `Dashboard.svelte` before implementation**: an earlier draft of this table proposed making this tab conditional on `session.project.methodology`. That was wrong — the Software-Dev Pack's enable/disable toggle is **universal**, not methodology-gated; any project can enable it regardless of methodology, and the section already renders unconditionally today (with its own internal enabled/disabled state, not an external visibility gate). This tab must therefore always be present in the tab row, matching current always-rendered behavior — not conditionally shown. |

**Implementation notes, corrections made during R3/R4 build-out:**

- **Loading/error placement (correction to this table's implicit framing)**:
  the table above describes the loading/error state as moving into the
  Overview tab (it "gates both lists"). Built instead with the loading
  spinner and load-error/retry block sitting **outside all four tab
  panels**, directly below the tab row, visible regardless of which tab is
  active. Rationale found only while implementing: the Charts and Documents
  tabs' catalogs also depend on the same `load()` call
  (`chartKinds`/`docKinds`); scoping the loading/error indicator to Overview
  only would leave a Charts or Documents tab switched to mid-load rendering
  blank with no explanation. Keeping it outside every panel preserves this
  section's pre-restructuring behavior (visible regardless of scroll
  position) and extends it to "regardless of tab" at no extra cost, since it
  doesn't touch content that differs between the two placements.
- **Panel mounting: `{#if}`, not `hidden`.** Considered and rejected keeping
  all four panels mounted with the `hidden` attribute (the W3C APG reference
  pattern, which would have preserved `ChartCatalog`/`DocumentCatalog`'s own
  local search/filter/expand state across tab switches). Rejected because
  `ChartCatalog`/`DocumentCatalog` render every catalog entry's name as
  text — with all four panels mounted, a kind name that also appears as an
  existing-chart's kind label in the Overview tab (e.g. "Work Breakdown
  Structure") exists twice in the DOM at once, which breaks Testing
  Library's `getByText` (throws on multiple matches) in `Dashboard.test.ts`'s
  own pre-existing load test — caught pre-implementation by tracing the
  actual fixture data, not discovered after the fact. `{#if}`-gated panels
  avoid this collision entirely, at the cost of resetting a catalog's
  search/filter state when the user leaves its tab and returns — a real but
  minor UX regression, not corrected this cycle.
- **New "nothing created yet" empty-state copy on the Overview tab** — not
  specified anywhere above. Without it, a project with zero existing charts
  and zero existing documents renders an empty Overview tab (no heading, no
  content, nothing telling the user where to go), a UX gap the pre-tab flat
  layout didn't have (the catalogs used to render directly below, so an
  empty project was never actually visually blank). Added one line of new
  user-facing copy ("Nothing created yet — use the Charts or Documents tab
  above to get started.") shown only when both lists are empty and loading
  has finished without error. Disclosed here because it's new copy this
  document didn't call for, not because it's risky — it's covered by
  `Dashboard.test.ts`'s "switching to Charts reveals the chart catalog and
  hides the Overview panel" test.

## 4. Deep dive

### 4.1 Tab control implementation

Per constraint C3, the tab row must be keyboard-operable to the same
standard as the app's existing focus handling: arrow-key navigation between
tabs, `Tab` to enter/exit the tab row as one stop (not one stop per tab),
and `aria-selected`/`role="tablist"`/`role="tab"`/`role="tabpanel"` wired
correctly — this is a well-established ARIA pattern
([APG Tabs](https://www.w3.org/WAI/ARIA/apg/patterns/tabs/)), not a novel
interaction to design from scratch. Given C5, this should be a small new
component (e.g. `Tabs.svelte`) built from the same primitives as the
existing library rather than a one-off in `Dashboard.svelte`, since the
Project Settings screen (flagged in the same design-critique pass as
"one flat, very long form" with no tabs/sections) is a second, independent
candidate for the same pattern — building it once as a shared component
means that fix is cheaper when its own cycle comes.

### 4.2 State: which tab is active

The active tab needs to live in local component state
(`let activeTab = $state('overview')`), not `session.*` — there's no
product requirement for the active dashboard tab to survive navigating
away and back, and adding it to the shared session store would be scope
beyond what R3/R4 ask for (C4's "additive, not a wider refactor" applies
here too, even though `activeTab` isn't part of `session.view`'s union).

### 4.3 What does NOT change

- The meta cards' content, data source, and visual design.
- The chart catalog's 24 entries, their category chips, or their filtering
  logic.
- The document-template catalog's 25 entries.
- The Software-Dev Pack section's own internal content and enable/disable
  logic (`app_foundation.go`'s `seedsRequestAgilePack`/`AgileEnabled`
  wiring) — only its position moves, from "bottom of a long scroll" to
  "one click away."
- Any Go/Wails-bound method signature (per C2).

## 5. Trade-off analysis

| Decision | Trade-off | Why this direction |
| --- | --- | --- |
| Tabs vs. accordion-everywhere | Tabs hide non-active content entirely (no partial-scroll preview of what's below); accordions keep a compact always-visible outline. | The catalogs are large (24/25 items) — an accordion-only approach still leaves the *expanded* catalog just as tall as it is today; tabs cap the maximum visible height for any given task, which is what R3 actually asks for. |
| Overview tab shows Documents only vs. Documents + a "recent activity" summary | A richer Overview tab could show more, but nothing in the current `Dashboard.svelte` has activity-feed data to summarize — inventing one is a real feature, not an IA fix. | Keep Overview to what already exists (Documents), consistent with C2/C5's "don't expand scope." |
| Dev Tools as a conditional tab vs. always-present | **Superseded, §3.3**: this row originally framed the choice as open, pending confirmation of the section's gating logic. Re-reading `Dashboard.svelte` found the Software-Dev Pack toggle is universal (no methodology gate), so "conditional on methodology" was never a real option — the section already renders unconditionally today. | Always-present, matching current behavior. Not a trade-off once the actual gating logic was checked. |
| Rename `portfolio`'s label vs. rename per-project `dashboard`'s label | Renaming the per-project screen (to something other than "dashboard") would also resolve the collision. | `portfolio` is the multi-project list — "Portfolio" is the more accurate existing word already used in its own heading; changing the *other* screen's long-established name has a larger blast radius (help docs, user muscle memory) for the same fix. |

## 6. What I'd revisit as this grows

- **If Project Settings gets the same tab treatment** (a second, independent
  candidate flagged in §4.1), the `Tabs.svelte` component built here should
  be validated against a *second* real use case before being called
  "the" shared pattern — one consumer isn't enough to know if its API
  generalizes.
- **If usage telemetry is ever added** (currently absent per C1), the
  Overview-tab-as-default decision and the catalog category-chip filtering
  should be revisited against real data instead of the structural argument
  this document relies on.
- **If a sixth content group is ever added to the dashboard** (e.g. a
  reporting/analytics tab), the four-tab row's width and the "always
  visible meta cards above the tabs" decision should be re-examined —
  this proposal was sized for five known groups, not designed to scale
  indefinitely.
- **If the chart-kind registry (`internal/charts/registry.go`) and the
  frontend's `chartRoutes` map are ever unified into one source of truth**
  (e.g. generating `chartRoutes` from `ListChartKinds()` at build or
  runtime instead of hand-maintaining two lists), the `'charts'` fallback
  view proposed in §3.2 becomes dead code in the literal sense the
  original critique assumed it already was — worth deleting at that point,
  not before.

## 7. Implementation status

**R1 (naming collision) and R2 (`'charts'` fallback route) were implemented
2026-08-17**, in a cycle deliberately split from R3/R4: pre-implementation
adversarial review found R3/R4 (the tab restructuring) carries a
substantial, entangled cost R1/R2 don't — `Dashboard.test.ts` had 23 tests
across 8 `describe` blocks, nearly all of which interact with content that
would move into a specific tab, and `Testing Library`'s queries exclude
elements not actually rendered/visible, so a tabbed restructuring requires
inserting a tab-switch into the majority of those tests, not just moving
markup — an estimate that measurement later corrected: 12 of 23, not
"nearly all" (see the R3/R4 paragraph below). The deferral decision itself
still held once R3/R4 were actually built; the entangled cost was real,
just smaller than this estimate suggested at the time. `Dashboard.test.ts`'s
own header comment also documents a live,
unfixed load-order race in the component being restructured — mixing a
DOM restructuring with a rewrite of the tests that currently work around
that race would make a test failure's cause ambiguous (the tab change, the
pre-existing race, or a bad test edit). R1/R2 have neither entanglement:
zero existing tests touched the strings R1 changed, and R2 added a route
`routeLoaders` didn't previously have, so nothing could regress.

**R3/R4 (the tab restructuring itself) implemented 2026-08-17**, its own
cycle as planned above. Actual test-edit surface, measured (not estimated):
12 of the 23 pre-existing tests needed a tab-switch inserted — chart
creation (3), document creation (2), Agile Pack toggle (4), MSPDI import
(3) — the other 11 (load, chart/document deletion, signed export, project
close) reach their target content on the default Overview tab or the
always-visible header, unchanged. Confirmed by running the un-edited test
suite against the restructured component first: 12 failed / 11 passed,
matching this count exactly before any test file was touched. A new shared
`Tabs.svelte` (role=tablist/tab, roving tabindex, arrow-key + Home/End
navigation, automatic activation — the W3C ARIA APG pattern) ships with its
own test file (8 tests) rather than re-testing keyboard mechanics inside
`Dashboard.test.ts`; `Dashboard.test.ts` gained 5 new tests covering its own
tab wiring only (default tab, each tab's content reachable, Dev Tools
present independent of methodology — verified by asserting the
methodology-gated Process Excellence quick-link is also present, proving
the test's methodology override actually took effect rather than
trivially passing). The pre-existing toggle-vs-load() race test was
fault-seeded after its tab-switch edit (temporarily removing
`toggleAgile()`'s `if (loading) return;` guard) and confirmed it still
fails without the guard — proof the edit didn't make it vacuous. The
load-order race itself is unchanged and intentionally not touched this
cycle, per the plan above. See §3.3 for the three implementation
corrections made during this build-out (loading/error placement, `{#if}`
vs `hidden` panel mounting, and the new "nothing created yet" empty-state
copy).

**Residual risk, not closed this cycle**: not live-verified in a running
Wails build. All coverage above is `vitest`/jsdom assertion-level — it
proves the DOM queries resolve and the right elements exist, not that the
tab row renders legibly, that panel spacing survived the restructuring, or
that the focus ring (flagged as a genuine strength in the original
design-critique pass, C3) is still visually present on the new tab
buttons. C3 is verified here by assertion only (`tabindex`,
`aria-selected`, `document.activeElement` after an arrow-key press), not
by looking at a real focus ring in a real window. This is the same
constraint disclosed for R1/R2 last cycle, but it matters more here: R1/R2
changed strings, R3/R4 changed the DOM structure of the app's most-visited
screen, and nobody has looked at the result in a browser.

Still out of scope, independent of R1–R4:

- Migrating `Dashboard.svelte`'s existing markup to the Button/Card library
  (tracked separately, C5).
- Project Settings' own flat-form restructuring (a distinct, independently
  flagged defect from the same design-critique pass, not addressed here).
- Preserving `ChartCatalog`/`DocumentCatalog`'s search/filter state across a
  tab switch away and back (see §3.3's `{#if}`-vs-`hidden` note) — the
  `{#if}` implementation resets it; fixing this would mean lifting that
  state into `Dashboard.svelte` as props, a small follow-up if it turns out
  to matter in practice.
