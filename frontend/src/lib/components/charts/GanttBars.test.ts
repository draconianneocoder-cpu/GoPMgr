// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/svelte';
import GanttBars from './GanttBars.svelte';
import type { GanttLayout } from './gantt_geometry';

function mount(layout: GanttLayout, pxPerDay = 10) {
  return render(GanttBars, { props: { layout, pxPerDay } });
}

describe('GanttBars', () => {
  it('renders an empty-state message with no rows', () => {
    const { getByText } = mount({ rows: [], deps: [], horizon: 0 });
    expect(getByText(/No tasks yet/i)).toBeInTheDocument();
  });

  it('draws a single bar for a contiguous task', () => {
    const layout: GanttLayout = {
      rows: [{ id: 'A', label: 'A', es: 1, ef: 4, percent_complete: 0 }],
      deps: [],
      horizon: 4,
    };
    const { container } = mount(layout);
    const bar = container.querySelector('[data-testid="bar-A"]');
    expect(bar).not.toBeNull();
    // ES=1 -> x=10, width=(4-1)*10=30. No split segments present.
    expect(bar!.getAttribute('x')).toBe('10');
    expect(bar!.getAttribute('width')).toBe('30');
    expect(container.querySelector('[data-testid^="split-seg"]')).toBeNull();
  });

  it('draws one interrupted bar piece per split segment plus a connector', () => {
    const layout: GanttLayout = {
      rows: [
        {
          id: 'S',
          label: 'Split',
          es: 0,
          ef: 3,
          percent_complete: 0,
          work_segments: [
            { start: 0, end: 1 },
            { start: 2, end: 3 },
            { start: 4, end: 5 },
          ],
        },
      ],
      deps: [],
      horizon: 5,
    };
    const { container } = mount(layout);
    const segs = container.querySelectorAll('[data-testid="split-seg-S"]');
    expect(segs).toHaveLength(3);
    // Absolute segment x-positions at pxPerDay=10: 0, 20, 40.
    expect(Array.from(segs).map((s) => s.getAttribute('x'))).toEqual(['0', '20', '40']);
    // A dashed connector spans the whole interrupted range (0 -> 5*10=50).
    const conn = container.querySelector('[data-testid="split-connector-S"]');
    expect(conn).not.toBeNull();
    expect(conn!.getAttribute('x1')).toBe('0');
    expect(conn!.getAttribute('x2')).toBe('50');
    // The single-bar path must not also render for a split task.
    expect(container.querySelector('[data-testid="bar-S"]')).toBeNull();
  });

  it('starts a dependency arrow at a split predecessor’s real finish', () => {
    const layout: GanttLayout = {
      rows: [
        { id: 'A', label: 'A', es: 0, ef: 3, percent_complete: 0,
          work_segments: [{ start: 0, end: 1 }, { start: 2, end: 3 }, { start: 4, end: 5 }] },
        { id: 'B', label: 'B', es: 6, ef: 8, percent_complete: 0 },
      ],
      deps: [{ from: 'A', to: 'B' }],
      horizon: 8,
    };
    const { container } = mount(layout);
    const arrow = container.querySelector('[data-testid="dep-A-B"]');
    expect(arrow).not.toBeNull();
    // Path must originate at rowEnd(A)=5 -> x=50, not EF(3)=30.
    expect(arrow!.getAttribute('d')!.startsWith('M 50 ')).toBe(true);
  });
});
