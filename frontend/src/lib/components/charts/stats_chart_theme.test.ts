// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import type { Chart } from 'chart.js';
import { describe, expect, it, vi } from 'vitest';

import { buildStatsChartConfig, drawStatsChartAnnotations } from './stats_chart_config';
import { DEFAULT_STATS_CHART_THEME, readStatsChartTheme } from './stats_chart_theme';

function style(values: Record<string, string>): Pick<CSSStyleDeclaration, 'getPropertyValue'> {
  return { getPropertyValue: (property) => values[property] ?? '' };
}

describe('readStatsChartTheme', () => {
  it('reads the current CSS-variable palette used by the canvas renderer', () => {
    expect(readStatsChartTheme(style({
      '--slate-300': '51 65 85',
      '--slate-400': '71 85 105',
      '--slate-800': '226 232 240',
      '--slate-900': '241 245 249',
    }))).toEqual({
      mode: 'light',
      text: 'rgb(51 65 85)',
      mutedText: 'rgb(71 85 105)',
      grid: 'rgb(226 232 240)',
      surface: 'rgb(241 245 249)',
    });
  });

  it('falls back per token when computed styles are absent or malformed', () => {
    expect(readStatsChartTheme(style({
      '--slate-300': 'not-a-color',
      '--slate-400': '300 1 2',
      '--slate-800': '30 41',
    }))).toEqual(DEFAULT_STATS_CHART_THEME);
  });
});

describe('buildStatsChartConfig', () => {
  const lightTheme = {
    mode: 'light' as const,
    text: 'rgb(51 65 85)',
    mutedText: 'rgb(71 85 105)',
    grid: 'rgb(226 232 240)',
    surface: 'rgb(241 245 249)',
  };

  it('threads the active theme through cartesian text, grid, and annotations', () => {
    const config = buildStatsChartConfig({
      kind: 'control',
      title: 'Cycle time',
      x_axis: { label: 'Day' },
      y_axis: { label: 'Hours' },
      categories: ['Mon'],
      series: [{ name: 'Observed', values: [4] }],
      annotations: [
        { type: 'horizontal_line', value: 5, label: 'UCL' },
        { type: 'horizontal_line', value: 1, label: 'LCL' },
      ],
    }, lightTheme);

    expect(config).toMatchObject({
      options: {
        plugins: {
          title: { color: lightTheme.text },
          legend: { labels: { color: lightTheme.text } },
          gopmgrAnnotations: { fallbackColor: lightTheme.mutedText },
        },
        scales: {
          x: { ticks: { color: lightTheme.mutedText }, grid: { color: lightTheme.grid } },
          // The value axis reaches both control limits, so neither is drawn
          // outside the plot.
          y: { ticks: { color: lightTheme.mutedText }, grid: { color: lightTheme.grid }, suggestedMin: 1, suggestedMax: 5 },
        },
      },
    });
  });

  it('uses the theme surface for pie borders and retains Pareto dual-axis wiring', () => {
    const pie = buildStatsChartConfig({
      kind: 'pie',
      x_axis: {},
      y_axis: {},
      slices: [{ label: 'Done', value: 1, pct: 100 }],
    }, lightTheme);
    expect(pie).toMatchObject({ data: { datasets: [{ borderColor: lightTheme.surface }] } });

    const pareto = buildStatsChartConfig({
      kind: 'pareto',
      x_axis: {},
      y_axis: {},
      y_axis_right: { label: 'Cumulative' },
      series: [{ name: 'Count', values: [3], type: 'bar' }, { name: 'Cumulative', values: [100], y_axis: 'right' }],
      annotations: [{ type: 'horizontal_line', value: 80, label: '80%' }],
    }, lightTheme);
    // The 80% line belongs to the percentage axis; it must not stretch the
    // count axis (3 here) to 80.
    const countAxis = (pareto.options?.scales as Record<string, { suggestedMax?: number }>).y;
    expect(countAxis.suggestedMax).toBeUndefined();
    expect(pareto).toMatchObject({
      data: { datasets: [{}, { yAxisID: 'y1' }] },
      options: {
        plugins: { gopmgrAnnotations: { scaleID: 'y1' } },
        scales: { y1: { ticks: { color: lightTheme.mutedText }, grid: { color: lightTheme.grid } } },
      },
    });
  });

  it('draws series in the active theme: unset colours by slot, the former dark-only defaults by hue, custom colours as given', () => {
    const layout = {
      kind: 'burnup' as const,
      x_axis: {},
      y_axis: {},
      categories: ['1'],
      series: [
        { name: 'Completed', values: [1], color: '#22d3ee' },
        { name: 'Ideal', values: [1], color: '#94a3b8' },
        { name: 'Unset', values: [1] },
        { name: 'Custom', values: [1], color: '#123456' },
      ],
      annotations: [{ type: 'horizontal_line' as const, value: 1, label: 'Mean', color: '#ef4444' }],
    };
    const colours = (theme: typeof lightTheme | typeof DEFAULT_STATS_CHART_THEME) =>
      (buildStatsChartConfig(layout, theme).data.datasets as { borderColor: string }[]).map((d) => d.borderColor);

    expect(colours(lightTheme)).toEqual(['#2a78d6', lightTheme.mutedText, '#1baf7a', '#123456']);
    // A ninth unset series takes the first slot again rather than no colour.
    const nine = buildStatsChartConfig({
      kind: 'line', x_axis: {}, y_axis: {}, categories: ['1'],
      series: Array.from({ length: 9 }, (_, i) => ({ name: `S${i}`, values: [i] })),
    }, lightTheme).data.datasets as { borderColor: string }[];
    expect(nine[8].borderColor).toBe('#2a78d6');
    expect(colours(DEFAULT_STATS_CHART_THEME)).toEqual(['#3987e5', DEFAULT_STATS_CHART_THEME.mutedText, '#199e70', '#123456']);
    expect(buildStatsChartConfig(layout, lightTheme)).toMatchObject({
      options: { plugins: { gopmgrAnnotations: { annotations: [{ color: '#e34948' }], labelColor: lightTheme.text } } },
    });
  });

  it('draws Pareto annotations against the configured percentage axis', () => {
    const leftPixel = vi.fn(() => 12);
    const rightPixel = vi.fn(() => 34);
    const context = {
      save: vi.fn(),
      restore: vi.fn(),
      setLineDash: vi.fn(),
      beginPath: vi.fn(),
      moveTo: vi.fn(),
      lineTo: vi.fn(),
      stroke: vi.fn(),
      fillText: vi.fn(),
    };
    const chart = {
      scales: {
        y: { getPixelForValue: leftPixel },
        y1: { getPixelForValue: rightPixel },
      },
      ctx: context,
      chartArea: { left: 5, right: 105 },
    } as unknown as Chart;

    drawStatsChartAnnotations(chart, {
      annotations: [{ type: 'horizontal_line', value: 80, label: '80%' }],
      scaleID: 'y1',
    });

    expect(rightPixel).toHaveBeenCalledWith(80);
    expect(leftPixel).not.toHaveBeenCalled();
    expect(context.moveTo).toHaveBeenCalledWith(5, 34);
    expect(context.lineTo).toHaveBeenCalledWith(105, 34);
  });
});
