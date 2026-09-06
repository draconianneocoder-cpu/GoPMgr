// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

export interface StatsChartTheme {
  text: string;
  mutedText: string;
  grid: string;
  surface: string;
}

export const DEFAULT_STATS_CHART_THEME: StatsChartTheme = {
  text: 'rgb(203 213 225)',
  mutedText: 'rgb(148 163 184)',
  grid: 'rgb(30 41 59)',
  surface: 'rgb(15 23 42)',
};

type CSSPropertyReader = Pick<CSSStyleDeclaration, 'getPropertyValue'>;

function cssTriplet(style: CSSPropertyReader, property: string, fallback: string): string {
  const channels = style.getPropertyValue(property).trim().split(/\s+/).map(Number);
  if (channels.length !== 3 || channels.some((channel) => !Number.isInteger(channel) || channel < 0 || channel > 255)) {
    return fallback;
  }
  return `rgb(${channels.join(' ')})`;
}

export function readStatsChartTheme(style: CSSPropertyReader): StatsChartTheme {
  return {
    text: cssTriplet(style, '--slate-300', DEFAULT_STATS_CHART_THEME.text),
    mutedText: cssTriplet(style, '--slate-400', DEFAULT_STATS_CHART_THEME.mutedText),
    grid: cssTriplet(style, '--slate-800', DEFAULT_STATS_CHART_THEME.grid),
    surface: cssTriplet(style, '--slate-900', DEFAULT_STATS_CHART_THEME.surface),
  };
}
