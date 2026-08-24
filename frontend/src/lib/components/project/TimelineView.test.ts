// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor } from '@testing-library/svelte';

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
