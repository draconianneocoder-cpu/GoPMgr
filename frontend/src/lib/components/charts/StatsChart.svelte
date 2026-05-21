<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // StatsChart is the shared Chart.js host used by every editor in
  // the Stats family. It takes a StatsLayout (produced by the Go
  // backend) and turns it into a Chart.js config, then mounts it on
  // a canvas. Reactive updates re-render the chart in place.
  //
  // The component imports Chart.js's `Chart` and registers all
  // controllers/elements/scales/plugins needed by the eight kinds.
  // Doing this in the shared component (rather than in every editor)
  // means each editor stays minimal.

  import { onMount, onDestroy } from 'svelte';
  import {
    Chart,
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
    type ChartConfiguration,
    type ChartDataset,
  } from 'chart.js';

  import type { StatsLayout, StatsSeries } from './_stats_types';
  import { DEFAULT_PALETTE } from './_stats_types';

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
  );

  let { layout, height = 400 }: { layout: StatsLayout; height?: number } = $props();

  let canvas = $state<HTMLCanvasElement | null>(null);
  let chart: Chart | null = null;

  onMount(() => {
    rebuild();
  });
  onDestroy(() => {
    chart?.destroy();
    chart = null;
  });

  // Rebuild when the layout changes. We destroy and re-create rather
  // than mutating chart.data because the chart *type* itself can
  // change between kinds (line ↔ bar ↔ pie), and Chart.js does not
  // support swapping a chart's controller in place.
  $effect(() => {
    layout; // trigger reactivity
    rebuild();
  });

  function rebuild() {
    if (!canvas) return;
    chart?.destroy();
    const config = buildConfig(layout);
    if (!config) return;
    chart = new Chart(canvas, config);
  }

  function buildConfig(l: StatsLayout): ChartConfiguration | null {
    switch (l.kind) {
      case 'pie': return buildPieConfig(l);
      case 'pareto': return buildParetoConfig(l);
      default: return buildCartesianConfig(l);
    }
  }

  // ---------- Pie ----------
  function buildPieConfig(l: StatsLayout): ChartConfiguration {
    const slices = l.slices ?? [];
    const labels = slices.map((s) => s.label);
    const values = slices.map((s) => s.value);
    const colors = slices.map((s, i) => s.color || DEFAULT_PALETTE[i % DEFAULT_PALETTE.length]);
    return {
      type: 'pie',
      data: {
        labels,
        datasets: [{ data: values, backgroundColor: colors, borderColor: '#0f172a', borderWidth: 1 }],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          title: { display: !!l.title, text: l.title ?? '', color: '#cbd5e1' },
          legend: { position: 'right', labels: { color: '#cbd5e1' } },
          tooltip: {
            callbacks: {
              label: (ctx) => {
                const s = slices[ctx.dataIndex];
                return `${s.label}: ${s.value} (${s.pct.toFixed(1)}%)`;
              },
            },
          },
        },
      },
    };
  }

  // ---------- Pareto (bar + line on dual y-axis) ----------
  function buildParetoConfig(l: StatsLayout): ChartConfiguration {
    const series = l.series ?? [];
    const datasets: ChartDataset[] = series.map((s, i) => baseDataset(s, i, true));
    return {
      type: 'bar', // mixed; the line series declares type: 'line' itself
      data: {
        labels: l.categories ?? [],
        datasets,
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: commonPlugins(l),
        scales: {
          x: cartesianX(l),
          y: { ...cartesianY(l), position: 'left' },
          y1: {
            position: 'right',
            min: l.y_axis_right?.min ?? 0,
            max: l.y_axis_right?.max ?? 100,
            grid: { drawOnChartArea: false, color: '#1e293b' },
            ticks: { color: '#94a3b8', callback: (v) => v + '%' },
            title: {
              display: !!l.y_axis_right?.label,
              text: l.y_axis_right?.label ?? '',
              color: '#94a3b8',
            },
          },
        },
      },
    };
  }

  // ---------- Cartesian (line / bar / cum-flow / burn / control) ----------
  function buildCartesianConfig(l: StatsLayout): ChartConfiguration {
    const series = l.series ?? [];
    const datasets: ChartDataset[] = series.map((s, i) => baseDataset(s, i, false, l));
    const isBar = l.kind === 'bar';
    return {
      type: isBar ? 'bar' : 'line',
      data: {
        labels: l.categories ?? [],
        datasets,
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: commonPlugins(l),
        scales: {
          x: { ...cartesianX(l), stacked: !!l.stacked },
          y: { ...cartesianY(l), stacked: !!l.stacked },
        },
      },
    };
  }

  // ---------- Dataset builders ----------
  function baseDataset(s: StatsSeries, i: number, allowDualY: boolean, layout?: StatsLayout): ChartDataset {
    const color = s.color || DEFAULT_PALETTE[i % DEFAULT_PALETTE.length];
    const type = (s.type || 'line') as 'line' | 'bar' | 'area';

    const ds: ChartDataset = {
      type: type === 'area' ? 'line' : (type as 'line' | 'bar'),
      label: s.name,
      data: s.values,
      borderColor: color,
      backgroundColor: type === 'bar' ? color : type === 'area' ? hexToRgba(color, 0.45) : color,
      borderDash: s.dashed ? [6, 4] : undefined,
      pointBackgroundColor: pointColors(s.values, i, color, layout),
      pointBorderColor: pointColors(s.values, i, color, layout),
      pointRadius: 4,
      borderWidth: 2,
      fill: type === 'area' ? (s.y_axis === 'right' ? false : true) : false,
      tension: type === 'line' || type === 'area' ? 0.2 : 0,
    } as ChartDataset;

    if (allowDualY && s.y_axis === 'right') {
      (ds as any).yAxisID = 'y1';
    }
    return ds;
  }

  // For Control charts: paint flagged points red. Layout.flags carries
  // (series, point, color) tuples; we resolve them per-series.
  function pointColors(values: number[], seriesIdx: number, baseColor: string, layout?: StatsLayout): string[] {
    if (!layout || !layout.flags || layout.flags.length === 0) {
      return values.map(() => baseColor);
    }
    return values.map((_, pointIdx) => {
      const flag = layout.flags?.find((f) => f.series === seriesIdx && f.point === pointIdx);
      return flag ? flag.color : baseColor;
    });
  }

  function commonPlugins(l: StatsLayout) {
    const annotations = l.annotations ?? [];
    return {
      title: { display: !!l.title, text: l.title ?? '', color: '#cbd5e1' },
      legend: { display: true, labels: { color: '#cbd5e1' } },
      tooltip: {
        callbacks: {
          afterLabel: (ctx: any) => {
            // For Control charts: surface the flag's reason in the tooltip.
            const flag = l.flags?.find(
              (f) => f.series === ctx.datasetIndex && f.point === ctx.dataIndex,
            );
            return flag?.reason ?? '';
          },
        },
      },
      // Annotations are drawn as fake horizontal-line "datasets"
      // because Chart.js's annotation plugin is a separate package.
      // For zero-dependency simplicity, ControlChart's mean/UCL/LCL
      // are layered via an inline plugin below.
      pmforgeAnnotations: { annotations },
    } as any;
  }

  function cartesianX(l: StatsLayout) {
    return {
      type: 'category' as const,
      title: { display: !!l.x_axis?.label, text: l.x_axis?.label ?? '', color: '#94a3b8' },
      ticks: { color: '#94a3b8' },
      grid: { color: '#1e293b' },
    };
  }
  function cartesianY(l: StatsLayout) {
    return {
      type: 'linear' as const,
      title: { display: !!l.y_axis?.label, text: l.y_axis?.label ?? '', color: '#94a3b8' },
      ticks: { color: '#94a3b8' },
      grid: { color: '#1e293b' },
      min: l.y_axis?.min,
      max: l.y_axis?.max,
    };
  }

  function hexToRgba(hex: string, alpha: number): string {
    const m = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex);
    if (!m) return `rgba(34,211,238,${alpha})`;
    return `rgba(${parseInt(m[1], 16)}, ${parseInt(m[2], 16)}, ${parseInt(m[3], 16)}, ${alpha})`;
  }

  // Inline plugin: render any horizontal_line annotations on top of
  // the chart. Registered globally so every Chart instance picks it up.
  const annotationPlugin = {
    id: 'pmforgeAnnotations',
    afterDatasetsDraw(c: Chart) {
      const opts = (c.config.options?.plugins as any)?.pmforgeAnnotations;
      const annotations = opts?.annotations as Annotation[] | undefined;
      if (!annotations || annotations.length === 0) return;
      const yScale: any = c.scales['y'];
      if (!yScale) return;
      const ctx = c.ctx;
      ctx.save();
      for (const a of annotations) {
        if (a.type !== 'horizontal_line') continue;
        const y = yScale.getPixelForValue(a.value);
        ctx.strokeStyle = a.color ?? '#94a3b8';
        ctx.setLineDash(a.dashed ? [6, 4] : []);
        ctx.lineWidth = 1.25;
        ctx.beginPath();
        ctx.moveTo(c.chartArea.left, y);
        ctx.lineTo(c.chartArea.right, y);
        ctx.stroke();
        if (a.label) {
          ctx.fillStyle = a.color ?? '#94a3b8';
          ctx.font = '10px sans-serif';
          ctx.fillText(a.label, c.chartArea.left + 4, y - 4);
        }
      }
      ctx.restore();
    },
  };
  type Annotation = { type: string; value: number; label?: string; color?: string; dashed?: boolean };
  Chart.register(annotationPlugin);
</script>

<div style={`height: ${height}px;`} class="w-full">
  <canvas bind:this={canvas}></canvas>
</div>
