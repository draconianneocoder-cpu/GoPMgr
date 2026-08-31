// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor, fireEvent } from '@testing-library/svelte';

import TimelineView from './TimelineView.svelte';

const milestoneEntry: TimelineEntry = {
  kind: 'milestone',
  title: 'Kickoff',
  date: '2026-01-10T00:00:00Z',
  source_id: 'charter-1-0',
  editable: false,
  // milestone_source deliberately omitted: pins the "undefined source reads
  // as charter" default (matches every real backend response before this
  // field existed, and any future entry the backend fails to populate).
};

const chartMilestoneEntry: TimelineEntry = {
  kind: 'milestone',
  title: 'Delivery plan: Ship',
  date: '2026-01-09T00:00:00Z',
  source_id: 'chart:gantt-1:task:2:ship',
  editable: false,
  milestone_source: 'chart',
};

const editableEntry: TimelineEntry = {
  kind: 'project_start',
  title: 'Project start',
  date: '2026-01-15T00:00:00Z',
  source_id: 'p1',
  editable: true,
  edit_field: 'start_date',
};

const deploymentEntry: TimelineEntry = {
  kind: 'deployment',
  title: 'Deploy v1.0',
  date: '2026-01-20T00:00:00Z',
  source_id: 'd1',
};

let app: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  app = {
    BuildTimeline: vi.fn(async () => []),
    ListHolidays: vi.fn(async () => []),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('milestone entries', () => {
  it('render read-only: listed, colored distinctly, not draggable', async () => {
    app.BuildTimeline = vi.fn(async () => [milestoneEntry]);
    const { getAllByText, getByText, container } = render(TimelineView);

    // Shown twice: once as the SVG tick label, once in the detail list below
    // the strip.
    await waitFor(() => expect(getAllByText('Kickoff').length).toBe(2));
    // "milestone · charter": the label is not optional, and an entry with
    // no milestone_source (this fixture) is pinned to read as charter --
    // see the fixture's own comment.
    expect(getByText('milestone · charter')).toBeInTheDocument();

    // No date <input> for it (editable=false means no drag/edit affordance) --
    // only a plain text span with the ISO date.
    expect(container.querySelector('input[type="date"]')).not.toBeInTheDocument();

    // Uses its own distinct color, not the generic-kind fallback (#94a3b8)
    // or any other kind's color.
    const dot = container.querySelector('.rounded-full') as HTMLElement | null;
    expect(dot).toBeTruthy();
    expect(dot?.style.background).toBe('rgb(168, 85, 247)'); // #a855f7

    // A charter-sourced milestone is a plain circle in the strip, not the
    // chart-sourced diamond (see the next test).
    expect(container.querySelector('svg polygon')).not.toBeInTheDocument();

    // The SVG tick for a non-editable entry has no role="button" (only
    // draggable entries get pointer/keyboard handlers).
    expect(container.querySelector('svg [role="button"]')).not.toBeInTheDocument();
  });

  it('chart-sourced milestone renders with a distinct color, shape, and label', async () => {
    app.BuildTimeline = vi.fn(async () => [chartMilestoneEntry]);
    const { getByText, container } = render(TimelineView);

    await waitFor(() => expect(getByText('milestone · chart')).toBeInTheDocument());

    // Distinct color from the charter case above (#ec4899, not #a855f7).
    const dot = container.querySelector('.rounded-full') as HTMLElement | null;
    expect(dot?.style.background).toBe('rgb(236, 72, 153)'); // #ec4899

    // Distinct marker shape in the strip: a diamond (polygon), not the
    // charter case's circle -- the distinction doesn't rely on color alone.
    const marker = container.querySelector('svg polygon');
    expect(marker).toBeTruthy();
    expect(marker?.getAttribute('fill')).toBe('#ec4899');
    expect(container.querySelector('svg circle[fill="#ec4899"]')).not.toBeInTheDocument();
  });
});

// Covers the fix for a real production bug: a point-in-time entry's
// unset end_date leaked as the Go zero value "0001-01-01T00:00:00Z"
// (json's `omitempty` is a no-op on non-pointer struct fields), and the
// old refreshHolidays computed its own independent min/max over both
// date and end_date, so that ancient value corrupted the holiday-lookup
// range into a ~2000-year window (confirmed via a reproduction: a US
// calendar over that exact range returns 22,572 holidays, matching a
// real user report of "22573 holidays" almost exactly). The backend fix
// (Entry.EndDate is now *time.Time) means a poisoned end_date should no
// longer reach the frontend at all -- but this suite tests the
// frontend's own defense-in-depth independently of that backend
// guarantee, since a future regression anywhere in the pipeline should
// not be able to reproduce the same corrupted query.
describe('refreshHolidays date range (regression coverage for the corrupted-range bug)', () => {
  it('stays within days of the real entry dates for ordinary entries', async () => {
    app.BuildTimeline = vi.fn(async () => [
      { kind: 'project_start', title: 'P start', date: '2026-01-01T00:00:00Z', source_id: 'p1', editable: true },
      { kind: 'project_end', title: 'P end', date: '2026-01-31T00:00:00Z', source_id: 'p1', editable: true },
    ]);
    render(TimelineView);
    await waitFor(() => expect(app.ListHolidays).toHaveBeenCalled());

    const [from, to] = app.ListHolidays.mock.calls[0];
    // minMS = 2026-01-01, padded -30d; maxMS = 2026-01-31, padded +30d.
    expect(from).toBe('2025-12-02');
    expect(to).toBe('2026-03-02');
  });

  it('a single entry with end_date earlier than its own date cannot drag the range back centuries', async () => {
    // Simulates a hypothetical future regression reintroducing a
    // corrupted end_date (e.g. the Go zero value), not the current
    // backend's actual behavior -- proving the frontend guard holds on
    // its own. With only one entry, nothing else could out-rank the
    // corrupted value in Math.max, which is exactly why the naive
    // "it never wins Math.max" reasoning doesn't generalize.
    app.BuildTimeline = vi.fn(async () => [
      {
        kind: 'sprint_start',
        title: 'Sprint 1 starts',
        date: '2026-02-01T00:00:00Z',
        end_date: '0001-01-01T00:00:00Z',
        source_id: 's1',
        editable: true,
        edit_field: 'start_date',
      },
    ]);
    render(TimelineView);
    await waitFor(() => expect(app.ListHolidays).toHaveBeenCalled());

    const [from, to] = app.ListHolidays.mock.calls[0];
    expect(from).toBe('2026-01-02'); // 2026-02-01 - 30d
    expect(to).toBe('2026-03-03'); // 2026-02-01 + 30d (end_date ignored: 0001 < date)
    expect(from.startsWith('0000') || from.startsWith('0001')).toBe(false);
    expect(to.startsWith('0000') || to.startsWith('0001')).toBe(false);
  });
});

// Pins the strip background to the theme token, not a literal hex --
// the original bug report ("time line is extremely dark, almost
// black"): the strip used fill="#1e293b" (slate-800's dark-theme value
// as a raw SVG presentation attribute), which never inverted for light
// theme regardless of data-theme. A future edit reverting the fill back
// to a literal hex would render correctly by coincidence in dark theme
// (the default) and only break in light theme, which nothing else in
// this suite would catch.
describe('timeline strip background', () => {
  it('binds to the slate-800 theme token, not a hardcoded hex', async () => {
    app.BuildTimeline = vi.fn(async () => [milestoneEntry]);
    const { container, getAllByText } = render(TimelineView);
    await waitFor(() => expect(getAllByText('Kickoff').length).toBe(2));

    const strip = container.querySelector('svg rect');
    expect(strip).toBeTruthy();
    expect(strip?.getAttribute('style')).toContain('fill: rgb(var(--slate-800))');
    expect(strip?.getAttribute('style')).toContain('stroke: rgb(var(--slate-700))');
    expect(strip?.getAttribute('fill')).toBeNull();
  });
});

// The strip background's own token-binding test (above, "timeline strip
// background") covers the #1 named complaint directly. These cover the
// rest of TimelineView's SVG presentation attributes that were converted
// from hardcoded hex to CSS custom-property tokens in the same fix, so a
// future edit can't silently revert any of them back to a literal hex
// (which renders identically in dark theme -- the app default -- and
// only breaks, invisibly to a dark-theme-only glance, in light theme).
//
// Drag-preview (the line + date label shown while an entry is being
// pointer-dragged) is deliberately NOT covered here: it binds to the
// same --amber-400 token the "drag-affordance ring" test below already
// pins via a path that needs no pointer simulation, and jsdom has no
// setPointerCapture -- this suite has no existing pointer-drag harness,
// and building one to cover a second call site of an already-pinned
// token isn't worth the new test infrastructure.
describe('SVG token bindings (regression coverage for hardcoded-color reversion)', () => {
  it('axis tick line and label bind to slate-500 / slate-400', async () => {
    app.BuildTimeline = vi.fn(async () => [editableEntry]);
    const { container, getAllByText } = render(TimelineView);
    await waitFor(() => expect(getAllByText('Project start')[0]).toBeInTheDocument());

    // stroke-width="0.5" is unique to tick lines among every <line> in the
    // strip (kindColor lines are 2/1.5, holiday stripes 1, drag preview 1).
    const tickLine = container.querySelector('svg line[stroke-width="0.5"]');
    expect(tickLine).toBeTruthy();
    expect(tickLine?.getAttribute('style')).toContain('stroke: rgb(var(--slate-500))');

    const tickLabels = Array.from(container.querySelectorAll('svg text')).filter(
      (t) => t.getAttribute('text-anchor') === 'middle',
    );
    expect(tickLabels.length).toBeGreaterThan(0);
    expect(tickLabels[0].getAttribute('style')).toContain('fill: rgb(var(--slate-400))');
  });

  it('holiday stripe and legend swatch bind to orange-300', async () => {
    app.BuildTimeline = vi.fn(async () => [editableEntry]);
    app.ListHolidays = vi.fn(async () => [{ date: '2026-01-16T00:00:00Z', name: 'Test Holiday' }]);
    const { container, getByText } = render(TimelineView);
    await waitFor(() => expect(getByText(/public holiday/)).toBeInTheDocument());

    // stroke-dasharray="2 2" is unique to holiday stripes (drag preview
    // uses "4 3").
    const stripe = container.querySelector('svg line[stroke-dasharray="2 2"]');
    expect(stripe).toBeTruthy();
    expect(stripe?.getAttribute('style')).toContain('stroke: rgb(var(--orange-300))');

    const swatch = container.querySelector('.inline-block.w-2.h-2');
    expect(swatch?.getAttribute('style')).toContain('rgb(var(--orange-300))');
  });

  it('marker outlines bind to slate-900 across editable, non-editable, and chart-milestone shapes', async () => {
    app.BuildTimeline = vi.fn(async () => [editableEntry, deploymentEntry, chartMilestoneEntry]);
    const { container, getAllByText } = render(TimelineView);
    await waitFor(() => expect(getAllByText('Project start')[0]).toBeInTheDocument());

    const editableCircle = container.querySelector('svg circle[r="5"]'); // editable branch
    expect(editableCircle?.getAttribute('style')).toContain('stroke: rgb(var(--slate-900))');

    const nonEditableCircle = container.querySelector('svg circle[r="3.5"]'); // non-editable, non-chart-milestone
    expect(nonEditableCircle?.getAttribute('style')).toContain('stroke: rgb(var(--slate-900))');

    const diamond = container.querySelector('svg polygon'); // chart-sourced milestone
    expect(diamond?.getAttribute('style')).toContain('stroke: rgb(var(--slate-900))');
  });

  it('entry title labels bind to slate-300', async () => {
    app.BuildTimeline = vi.fn(async () => [editableEntry]);
    const { container, getAllByText } = render(TimelineView);
    await waitFor(() => expect(getAllByText('Project start')[0]).toBeInTheDocument());

    // Entry labels are the <text> elements WITHOUT text-anchor (tick
    // labels are the only ones that set it, to "middle").
    const entryLabels = Array.from(container.querySelectorAll('svg text')).filter(
      (t) => t.getAttribute('text-anchor') === null,
    );
    expect(entryLabels.length).toBeGreaterThan(0);
    expect(entryLabels[0].getAttribute('style')).toContain('fill: rgb(var(--slate-300))');
  });

  it('the draggable-entry affordance ring is cyan-300 at rest', async () => {
    app.BuildTimeline = vi.fn(async () => [editableEntry]);
    const { container, getAllByText } = render(TimelineView);
    await waitFor(() => expect(getAllByText('Project start')[0]).toBeInTheDocument());

    const ring = container.querySelector('svg circle[r="8"]');
    expect(ring?.getAttribute('style')).toContain('stroke: rgb(var(--cyan-300))');
  });

  it('the affordance ring switches to amber-400 while a move is in flight', async () => {
    app.BuildTimeline = vi.fn(async () => [editableEntry]);
    let resolveMove!: (v: TimelineEntry[]) => void;
    app.MoveTimelineEntry = vi.fn(
      () => new Promise<TimelineEntry[]>((resolve) => { resolveMove = resolve; }),
    );
    const { container, getAllByText, getByLabelText } = render(TimelineView);
    await waitFor(() => expect(getAllByText('Project start')[0]).toBeInTheDocument());

    await fireEvent.change(getByLabelText('Project start date'), { target: { value: '2026-01-16' } });

    await waitFor(() => {
      const ring = container.querySelector('svg circle[r="8"]');
      expect(ring?.getAttribute('style')).toContain('stroke: rgb(var(--amber-400))');
    });

    // Let the pending move resolve so the test doesn't leak a dangling
    // update into the next test.
    resolveMove(await app.BuildTimeline());
  });
});

// Added 2026-08-19 alongside migrating the header "&larr; Dashboard" nav
// link (previously a raw <button>) to the `nav` Button variant — see
// Button.svelte's `nav` branch comment (tier-3 deferred file, closed this
// cycle). Not independently fault-seeded on this file — relies on
// Button.test.ts's single shared seed; this file's own query resolving is
// proven by the assertion passing rather than throwing.
describe('TimelineView migrated header "&larr; Dashboard" button', () => {
  it('Button variant="nav"', async () => {
    const { getByText } = render(TimelineView);
    await waitFor(() => expect(app.BuildTimeline).toHaveBeenCalled());

    const btn = getByText('← Dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'text-xs text-slate-400 hover:text-cyan-400 disabled:opacity-50'.split(/\s+/).sort(),
    );
  });
});
