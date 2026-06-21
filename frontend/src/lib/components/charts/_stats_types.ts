// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
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

// Default palette used when a Series or PieSlice doesn't specify a
// colour. Eight values is enough for a typical PM chart (more series
// than that should usually be redesigned).
export const DEFAULT_PALETTE = [
  '#22d3ee', // cyan
  '#f59e0b', // amber
  '#22c55e', // green
  '#a855f7', // purple
  '#ef4444', // red
  '#0ea5e9', // sky
  '#eab308', // yellow
  '#94a3b8', // slate
];
