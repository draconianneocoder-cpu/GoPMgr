// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it, vi } from 'vitest';
import { render } from '@testing-library/svelte';

import StatsChart from './StatsChart.svelte';
import type { StatsLayout } from './_stats_types';

// A layout with nothing to draw (a new chart, or a pie whose slices are all
// 0 or left out as negative) drew an empty canvas with no reason. The chart
// says so instead, and draws a canvas only when there is something on it.

const axis = { type: 'category' } as StatsLayout['x_axis'];

function pie(values: number[]): StatsLayout {
  return {
    kind: 'pie',
    x_axis: axis,
    y_axis: axis,
    slices: values.map((value, i) => ({ label: `S${i + 1}`, value, pct: 0 })),
  } as StatsLayout;
}

describe('StatsChart empty state', () => {
  it.each([
    ['a pie whose slices are all 0', pie([0, 0]), 'No slice has a value above 0, so the pie has nothing to draw.'],
    ['a pie with no slices', pie([]), 'No data to chart yet.'],
    ['a line chart with no series', { kind: 'line', x_axis: axis, y_axis: axis, series: [] } as StatsLayout, 'No data to chart yet.'],
  ])('%s shows a message, not a canvas', (_name, layout, message) => {
    const { getByRole, container } = render(StatsChart, { layout });
    expect(getByRole('status')).toHaveTextContent(message);
    expect(container.querySelector('canvas')).toBeNull();
  });

  it('a pie with a positive slice draws a canvas', () => {
    // jsdom has no 2D context; Chart.js reports that and draws nothing.
    vi.spyOn(console, 'error').mockImplementation(() => {});
    const { container, queryByRole } = render(StatsChart, { layout: pie([0, 3]) });
    expect(container.querySelector('canvas')).not.toBeNull();
    expect(queryByRole('status')).toBeNull();
  });
});
