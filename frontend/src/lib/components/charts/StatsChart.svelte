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

  function rebuild() {
    if (!canvas) return;
    chart?.destroy();
    const theme = readStatsChartTheme(getComputedStyle(document.documentElement));
    const config = buildStatsChartConfig(layout, theme);
    chart = new Chart(canvas, config);
  }

  onMount(() => {
    const observer = new MutationObserver(rebuild);
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
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

<div style={`height: ${height}px;`} class="w-full" role="img" aria-label={chartLabel}>
  <canvas bind:this={canvas}></canvas>
</div>
