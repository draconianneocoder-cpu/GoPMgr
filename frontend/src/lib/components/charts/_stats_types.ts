// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Frontend mirrors of the StatsLayout types from
// internal/charts/stats/common.go. Kept here (rather than autogen) so
// editors and the StatsChart component share one source of truth.

export interface StatsSeries {
  name: string;
  values: number[];
  type?: 'line' | 'bar' | 'area' | '';
  color?: string;
  y_axis?: '' | 'left' | 'right';
  dashed?: boolean;
}

export interface AxisConfig {
  label?: string;
  type?: 'category' | 'linear' | 'time';
  min?: number;
  max?: number;
}

export interface Annotation {
  type: 'horizontal_line';
  value: number;
  label?: string;
  color?: string;
  dashed?: boolean;
}

export interface PointFlag {
  series: number;
  point: number;
  color: string;
  reason?: string;
}

export interface PieSlice {
  label: string;
  value: number;
  pct: number;
  color?: string;
}

export interface StatsLayout {
  kind: string;
  title?: string;
  x_axis: AxisConfig;
  y_axis: AxisConfig;
  y_axis_right?: AxisConfig;
  categories?: string[];
  series?: StatsSeries[];
  stacked?: boolean;
  slices?: PieSlice[];
  annotations?: Annotation[];
  flags?: PointFlag[];
}

// Series colours for a Series or PieSlice that doesn't name one, in fixed
// slot order (blue, orange, aqua, yellow, magenta, green, violet, red).
// Each theme has its own steps of the same eight hues, checked with the
// dataviz palette validator against the chart surface (slate-900):
// lightness band, chroma floor, colour-vision-deficiency separation of
// adjacent slots (worst ΔE 9.1 light, 8.4 dark) and the normal-vision
// floor all pass. In the light theme four slots are below 3:1 against the
// surface, so stats charts always show a legend and per-point tooltips.
export const SERIES_PALETTE = {
  dark: ['#3987e5', '#d95926', '#199e70', '#c98500', '#d55181', '#0ca30c', '#9085e9', '#e66767'],
  light: ['#2a78d6', '#eb6834', '#1baf7a', '#eda100', '#e87ba4', '#008300', '#6250d6', '#e34948'],
} as const;

// The dark-only colours the app used before the themed palette. The Go
// chart layouts (burndown, burnup, control, Pareto) still send some of
// them, so each resolves to the matching slot in the active theme; null
// means the neutral reference colour (ideal line, mean). Any other colour
// is a user's choice and is drawn as given.
export const LEGACY_SERIES_SLOTS: Readonly<Record<string, number | null>> = {
  '#22d3ee': 0, // cyan → blue
  '#f59e0b': 1, // amber → orange
  '#22c55e': 5, // green → green
  '#a855f7': 6, // purple → violet
  '#ef4444': 7, // red → red
  '#0ea5e9': 2, // sky → aqua
  '#eab308': 3, // yellow → yellow
  '#94a3b8': null, // slate → neutral
};
