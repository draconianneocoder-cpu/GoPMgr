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
   1280×800 viewport, mixing five distinct kinds of content in sequence with
   no tabs, sections, or in-page navigation: project-meta cards
   (Stakeholders/Timeline/Budget) → a Documents list → a 24-item
   chart-creation tool catalog (2-column grid, lightly filterable by 4
   category chips, all 24 shown unsorted by default) → a 25-item
   document-template catalog (behind one collapsed accordion) → a
   Software-Dev Pack section (the enable/disable control for the
   agile/Sigma tooling fixed under the "Fix launchpad wizard" backlog item).
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
- **R3.** A user must be able to reach any one of the five content groups
  currently on `project/Dashboard.svelte` (meta cards, Documents, chart
  catalog, document-template catalog, Software-Dev Pack) without scrolling
  past unrelated content to do it.
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

### 3.1 Naming collision (R1)

Two independent screens currently share the word "dashboard." The fix is a
naming decision, not a structural one:

| View | Current nav label | Current heading | Proposed nav label | Proposed heading |
| --- | --- | --- | --- | --- |
| `portfolio` | "DASHBOARD" | "Portfolio dashboard" | "PORTFOLIO" | "Portfolio" |
| `dashboard` (per-project) | *(no nav entry — reached from inside a project)* | "Project dashboard" | *(unchanged)* | "Dashboard" |

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

### 3.2 `'charts'` fallback route (R2) — corrected during drafting

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

### 3.3 Dashboard restructuring (R3, R4)

Proposed structure: a tabbed shell wrapping the five existing content
groups, with the meta cards promoted out of the tab set entirely (they're
the orientation content every visit needs, not one option among several).

```
┌─────────────────────────────────────────────────────────┐
│  ← Back to portfolio          Project Name          ⋮   │  (existing header, unchanged)
├─────────────────────────────────────────────────────────┤
│  Stakeholders: 12   Timeline: Mar–Sep   Budget: $42,000  │  (meta cards, always visible)
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
| **Overview** (default) | The existing Documents list. This is the tab a user lands on — the highest-frequency task (checking recent documents / project state) shouldn't be behind a click, matching R3's "reach without scrolling past unrelated content" for the single most common case. |
| **Charts** | The 24-item chart-creation tool catalog, category chips unchanged. |
| **Documents** | The 25-item document-template catalog, no longer behind a collapsed accordion — the accordion was doing the job a tab now does (hiding low-frequency content), so a plain always-expanded grid is simpler once it has its own tab. |
| **Dev Tools** | The Software-Dev Pack section (R4) — reachable in one click from the tab row regardless of scroll position, closing the gap the "Fix launchpad wizard" backlog item already flagged from the enable-toggle's *positioning*, not just its default state. This tab should only render in the tab row when `session.project.methodology` is one that has agile/Sigma tooling available, matching whatever condition currently gates the section's visibility — not shown as a fourth tab that's always empty for e.g. a Waterfall project with no Software-Dev Pack applicable. **This condition needs confirming against `Dashboard.svelte`'s actual current gating logic before implementation**, not assumed from this document.

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
| Dev Tools as a conditional tab vs. always-present with an empty state | An always-present tab is simpler to implement (no conditional tab-row logic) but reintroduces exactly the kind of "feature that doesn't apply to this project shown anyway" clutter the original critique flagged the Software-Dev Pack contradiction as an instance of. | Conditional, matching the existing gating logic (to be confirmed, §3.3). |
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

## 7. Explicitly out of scope this cycle

- Implementing any of the above in code. This document is the design
  artifact for a future implementation cycle, per this cycle's own scoping
  decision (a full `Dashboard.svelte` restructuring without a dedicated
  implementation-and-review cycle was judged too high-risk for the app's
  most-visited screen to bundle into the same cycle as the residual-risk
  closures — see the Executive Summary for the explicit reasoning).
- Migrating `Dashboard.svelte`'s existing markup to the Button/Card library
  (tracked separately, C5).
- Project Settings' own flat-form restructuring (a distinct, independently
  flagged defect from the same design-critique pass, not addressed here).
