<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import {
    ArcElement,
    BarController,
    BarElement,
    CategoryScale,
    Chart,
    Filler,
    Legend,
    LinearScale,
    LineController,
    LineElement,
    PieController,
    PointElement,
    Title,
    Tooltip,
  } from 'chart.js';

  import type { StatsLayout } from './_stats_types';
  import { buildStatsChartConfig, gopmgrAnnotationsPlugin } from './stats_chart_config';
  import { readStatsChartTheme } from './stats_chart_theme';
  import { FONT_SANS } from '../../typography';
  import { t } from '../../i18n/i18n.svelte';

  Chart.register(
    LineController,
    BarController,
    PieController,
    LineElement,
    PointElement,
    BarElement,
    ArcElement,
    Filler,
    CategoryScale,
    LinearScale,
    Title,
    Tooltip,
    Legend,
    gopmgrAnnotationsPlugin,
  );
  // Chart.js defaults to Helvetica Neue/Arial; draw labels in the app's font.
  Chart.defaults.font.family = FONT_SANS;

  let { layout, height = 400 }: { layout: StatsLayout; height?: number } = $props();
  let canvas = $state<HTMLCanvasElement | null>(null);
  let chart: Chart | null = null;

  const KIND_LABELS: Record<string, string> = {
    line: 'Line chart',
    bar: 'Bar chart',
    pie: 'Pie chart',
    pareto: 'Pareto chart',
    burnup: 'Burn-up chart',
    burndown: 'Burn-down chart',
    cumulative_flow: 'Cumulative flow diagram',
    control: 'Control chart',
  };
  const chartLabel = $derived.by(() => {
    const parts = [KIND_LABELS[layout.kind] ?? 'Chart'];
    if (layout.title) parts.push(layout.title);
    if (layout.kind === 'pie') {
      const names = (layout.slices ?? []).map((slice) => slice.label).filter(Boolean);
      if (names.length) parts.push(`Segments: ${names.join(', ')}`);
    } else {
      const names = (layout.series ?? []).map((series) => series.name).filter(Boolean);
      if (names.length) parts.push(`Series: ${names.join(', ')}`);
      if (layout.categories?.length) parts.push(`${layout.categories.length} categories`);
    }
    return parts.join('. ') + '.';
  });

  // A layout with nothing to draw (a new chart, or a pie whose values are
  // all 0) showed an empty canvas with no reason; it gets a message instead.
  const emptyMessage = $derived.by(() => {
    if (layout.kind === 'pie') {
      const slices = layout.slices ?? [];
      if (slices.some((s) => s.value > 0)) return '';
      return slices.length ? t('chart.noData.pie') : t('chart.noData');
    }
    const hasValue = (layout.series ?? []).some((s) => s.values.some((v) => Number.isFinite(v)));
    return hasValue ? '' : t('chart.noData');
  });

  function rebuild() {
    if (!canvas) {
      chart?.destroy();
      chart = null;
      return;
    }
    chart?.destroy();
    // A construction that threw leaves its chart registered on the canvas,
    // and every later `new Chart` on it fails with "Canvas is already in use".
    Chart.getChart(canvas)?.destroy();
    const theme = readStatsChartTheme(getComputedStyle(document.documentElement));
    // Chart.js writes into its data arrays (it attaches a `_chartjs`
    // listener property). Editors hold the layout in `$state`, whose proxies
    // refuse that write (state_descriptors_fixed), so the chart drew nothing.
    // Chart.js gets a plain copy instead.
    const config = buildStatsChartConfig($state.snapshot(layout) as StatsLayout, theme);
    chart = new Chart(canvas, config);
  }

  onMount(() => {
    const observer = new MutationObserver(rebuild);
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
    // A canvas keeps whatever font was ready when it drew. Redraw once the
    // page's fonts finish loading so labels never stay in a fallback font.
    document.fonts?.ready.then(() => chart?.update());
    return () => observer.disconnect();
  });

  onDestroy(() => {
    chart?.destroy();
    chart = null;
  });

  $effect(() => {
    layout;
    rebuild();
  });
</script>

{#if emptyMessage}
  <div
    style={`height: ${height}px;`}
    class="w-full flex items-center justify-center rounded border border-dashed border-slate-700 text-sm text-slate-400"
    role="status"
  >
    {emptyMessage}
  </div>
{:else}
  <div style={`height: ${height}px;`} class="w-full" role="img" aria-label={chartLabel}>
    <canvas bind:this={canvas}></canvas>
  </div>
{/if}
