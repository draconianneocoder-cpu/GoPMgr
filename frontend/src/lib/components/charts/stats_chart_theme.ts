// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { LEGACY_SERIES_SLOTS, SERIES_PALETTE } from './_stats_types';

export interface StatsChartTheme {
  mode: 'dark' | 'light';
  text: string;
  mutedText: string;
  grid: string;
  surface: string;
}

export const DEFAULT_STATS_CHART_THEME: StatsChartTheme = {
  mode: 'dark',
  text: 'rgb(203 213 225)',
  mutedText: 'rgb(148 163 184)',
  grid: 'rgb(30 41 59)',
  surface: 'rgb(15 23 42)',
};

// seriesColor returns the colour to draw series or slice `index` in:
// the theme's palette slot when no colour is set, the theme's step of
// the same hue for one of the app's former dark-only defaults, and any
// other colour unchanged. Slots past the eighth repeat.
export function seriesColor(theme: StatsChartTheme, index: number, color?: string): string {
  const palette = SERIES_PALETTE[theme.mode];
  if (!color) return palette[index % palette.length];
  const slot = LEGACY_SERIES_SLOTS[color.toLowerCase()];
  if (slot === undefined) return color;
  return slot === null ? theme.mutedText : palette[slot];
}

type CSSPropertyReader = Pick<CSSStyleDeclaration, 'getPropertyValue'>;

function cssTriplet(style: CSSPropertyReader, property: string, fallback: string): string {
  const channels = style.getPropertyValue(property).trim().split(/\s+/).map(Number);
  if (channels.length !== 3 || channels.some((channel) => !Number.isInteger(channel) || channel < 0 || channel > 255)) {
    return fallback;
  }
  return `rgb(${channels.join(' ')})`;
}

export function readStatsChartTheme(style: CSSPropertyReader): StatsChartTheme {
  // The light theme inverts the slate scale, so a light slate-900 surface
  // means the light theme.
  const surface = cssTriplet(style, '--slate-900', DEFAULT_STATS_CHART_THEME.surface);
  const channels = surface.match(/\d+/g)?.map(Number) ?? [];
  const mode = channels.length === 3 && channels[0] + channels[1] + channels[2] > 382 ? 'light' : 'dark';
  return {
    mode,
    text: cssTriplet(style, '--slate-300', DEFAULT_STATS_CHART_THEME.text),
    mutedText: cssTriplet(style, '--slate-400', DEFAULT_STATS_CHART_THEME.mutedText),
    grid: cssTriplet(style, '--slate-800', DEFAULT_STATS_CHART_THEME.grid),
    surface,
  };
}
