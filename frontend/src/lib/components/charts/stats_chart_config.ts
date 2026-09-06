// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import type { Chart, ChartConfiguration, ChartDataset, ChartType, Plugin, TooltipItem } from 'chart.js';

import type { Annotation, StatsLayout, StatsSeries } from './_stats_types';
import { DEFAULT_PALETTE } from './_stats_types';
import type { StatsChartTheme } from './stats_chart_theme';

export interface AnnotationPluginOptions {
  annotations?: Annotation[];
  fallbackColor?: string;
  scaleID?: string;
}

export function buildStatsChartConfig(l: StatsLayout, theme: StatsChartTheme): ChartConfiguration {
  switch (l.kind) {
    case 'pie': return buildPieConfig(l, theme);
    case 'pareto': return buildParetoConfig(l, theme);
    default: return buildCartesianConfig(l, theme);
  }
}

function buildPieConfig(l: StatsLayout, theme: StatsChartTheme): ChartConfiguration {
  const slices = l.slices ?? [];
  return {
    type: 'pie',
    data: {
      labels: slices.map((s) => s.label),
      datasets: [{
        data: slices.map((s) => s.value),
        backgroundColor: slices.map((s, i) => s.color || DEFAULT_PALETTE[i % DEFAULT_PALETTE.length]),
        borderColor: theme.surface,
        borderWidth: 1,
      }],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        title: { display: !!l.title, text: l.title ?? '', color: theme.text },
        legend: { position: 'right', labels: { color: theme.text } },
        tooltip: {
          callbacks: {
            label: (ctx) => {
              const slice = slices[ctx.dataIndex];
              return `${slice.label}: ${slice.value} (${slice.pct.toFixed(1)}%)`;
            },
          },
        },
      },
    },
  };
}

function buildParetoConfig(l: StatsLayout, theme: StatsChartTheme): ChartConfiguration {
  const datasets: ChartDataset<'line' | 'bar', number[]>[] = (l.series ?? []).map(
    (series, index) => baseDataset(series, index, true),
  );
  return {
    type: 'bar',
    data: { labels: l.categories ?? [], datasets },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      plugins: commonPlugins(l, theme),
      scales: {
        x: cartesianX(l, theme),
        y: { ...cartesianY(l, theme), position: 'left' },
        y1: {
          position: 'right',
          min: l.y_axis_right?.min ?? 0,
          max: l.y_axis_right?.max ?? 100,
          grid: { drawOnChartArea: false, color: theme.grid },
          ticks: { color: theme.mutedText, callback: (value) => value + '%' },
          title: { display: !!l.y_axis_right?.label, text: l.y_axis_right?.label ?? '', color: theme.mutedText },
        },
      },
    },
  };
}

function buildCartesianConfig(l: StatsLayout, theme: StatsChartTheme): ChartConfiguration {
  const datasets: ChartDataset<'line' | 'bar', number[]>[] = (l.series ?? []).map(
    (series, index) => baseDataset(series, index, false, l),
  );
  return {
    type: l.kind === 'bar' ? 'bar' : 'line',
    data: { labels: l.categories ?? [], datasets },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      plugins: commonPlugins(l, theme),
      scales: {
        x: { ...cartesianX(l, theme), stacked: !!l.stacked },
        y: { ...cartesianY(l, theme), stacked: !!l.stacked },
      },
    },
  };
}

function baseDataset(
  series: StatsSeries,
  index: number,
  allowDualY: boolean,
  layout?: StatsLayout,
): ChartDataset<'line' | 'bar', number[]> {
  const color = series.color || DEFAULT_PALETTE[index % DEFAULT_PALETTE.length];
  const type = series.type || 'line';
  const dataset: ChartDataset<'line' | 'bar', number[]> = {
    type: type === 'area' ? 'line' : type,
    label: series.name,
    data: series.values,
    borderColor: color,
    backgroundColor: type === 'bar' ? color : type === 'area' ? hexToRgba(color, 0.45) : color,
    borderDash: series.dashed ? [6, 4] : undefined,
    pointBackgroundColor: pointColors(series.values, index, color, layout),
    pointBorderColor: pointColors(series.values, index, color, layout),
    pointRadius: 4,
    borderWidth: 2,
    fill: type === 'area' && series.y_axis !== 'right',
    tension: type === 'line' || type === 'area' ? 0.2 : 0,
  };
  if (allowDualY && series.y_axis === 'right') dataset.yAxisID = 'y1';
  return dataset;
}

function pointColors(values: number[], seriesIndex: number, baseColor: string, layout?: StatsLayout): string[] {
  if (!layout?.flags?.length) return values.map(() => baseColor);
  return values.map((_, pointIndex) => {
    const flag = layout.flags?.find((candidate) => candidate.series === seriesIndex && candidate.point === pointIndex);
    return flag?.color ?? baseColor;
  });
}

function commonPlugins(l: StatsLayout, theme: StatsChartTheme) {
  return {
    title: { display: !!l.title, text: l.title ?? '', color: theme.text },
    legend: { display: true, labels: { color: theme.text } },
    tooltip: {
      callbacks: {
        afterLabel: (ctx: TooltipItem<ChartType>) => l.flags?.find(
          (flag) => flag.series === ctx.datasetIndex && flag.point === ctx.dataIndex,
        )?.reason ?? '',
      },
    },
    gopmgrAnnotations: {
      annotations: l.annotations ?? [],
      fallbackColor: theme.mutedText,
      scaleID: l.kind === 'pareto' ? 'y1' : 'y',
    },
  };
}

function cartesianX(l: StatsLayout, theme: StatsChartTheme) {
  return {
    type: 'category' as const,
    title: { display: !!l.x_axis?.label, text: l.x_axis?.label ?? '', color: theme.mutedText },
    ticks: { color: theme.mutedText },
    grid: { color: theme.grid },
  };
}

function cartesianY(l: StatsLayout, theme: StatsChartTheme) {
  return {
    type: 'linear' as const,
    title: { display: !!l.y_axis?.label, text: l.y_axis?.label ?? '', color: theme.mutedText },
    ticks: { color: theme.mutedText },
    grid: { color: theme.grid },
    min: l.y_axis?.min,
    max: l.y_axis?.max,
  };
}

function hexToRgba(hex: string, alpha: number): string {
  const match = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex);
  if (!match) return `rgba(34,211,238,${alpha})`;
  return `rgba(${parseInt(match[1], 16)}, ${parseInt(match[2], 16)}, ${parseInt(match[3], 16)}, ${alpha})`;
}

export function drawStatsChartAnnotations(chart: Chart, options: AnnotationPluginOptions): void {
  if (!options.annotations?.length) return;
  const yScale = chart.scales[options.scaleID ?? 'y'];
  if (!yScale) return;
  const ctx = chart.ctx;
  ctx.save();
  for (const annotation of options.annotations) {
    if (annotation.type !== 'horizontal_line') continue;
    const color = annotation.color ?? options.fallbackColor ?? '#94a3b8';
    const y = yScale.getPixelForValue(annotation.value);
    ctx.strokeStyle = color;
    ctx.setLineDash(annotation.dashed ? [6, 4] : []);
    ctx.lineWidth = 1.25;
    ctx.beginPath();
    ctx.moveTo(chart.chartArea.left, y);
    ctx.lineTo(chart.chartArea.right, y);
    ctx.stroke();
    if (annotation.label) {
      ctx.fillStyle = color;
      ctx.font = '10px sans-serif';
      ctx.fillText(annotation.label, chart.chartArea.left + 4, y - 4);
    }
  }
  ctx.restore();
}

export const gopmgrAnnotationsPlugin: Plugin<ChartType, AnnotationPluginOptions> = {
  id: 'gopmgrAnnotations',
  afterDatasetsDraw(chart, _args, options) {
    drawStatsChartAnnotations(chart, options);
  },
};
