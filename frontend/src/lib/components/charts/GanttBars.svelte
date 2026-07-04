<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Presentational Gantt bar canvas: pure function of the layout, zoom, and
  // baseline variances (no Wails bridge, no session), so it can be mounted
  // and asserted in component tests. GanttEditor renders through this.
  import {
    GANTT_ROW_H as rowH,
    baselineBar,
    barPieces,
    depPath,
    type GanttLayout,
  } from './gantt_geometry';

  let {
    layout,
    pxPerDay,
    variances = {},
  }: {
    layout: GanttLayout;
    pxPerDay: number;
    variances?: Record<string, { start_var_days: number; finish_var_days: number }>;
  } = $props();

  const canvasW = $derived(Math.max(300, layout.horizon * pxPerDay + 60));
  const canvasH = $derived(Math.max(60, layout.rows.length * rowH + 24));
</script>

{#if layout.rows.length === 0}
  <p class="text-slate-500 text-sm">No tasks yet. Click <strong>+ Task</strong> to add one.</p>
{:else}
  <svg width={canvasW} height={canvasH} role="img" aria-label="Gantt schedule bars">
    <!-- day grid -->
    {#each Array(Math.ceil(layout.horizon) + 1) as _, d (d)}
      <line x1={d * pxPerDay} y1="0" x2={d * pxPerDay} y2={layout.rows.length * rowH} stroke="#1e293b" stroke-width="1" />
      {#if pxPerDay >= 20 || d % 5 === 0}
        <text x={d * pxPerDay + 2} y={layout.rows.length * rowH + 14} font-size="9" fill="#475569">{d}</text>
      {/if}
    {/each}

    <!-- dependency arrows under the bars -->
    {#each layout.deps as dep (dep.from + '>' + dep.to)}
      {@const p = depPath(dep, layout.rows, pxPerDay)}
      {#if p}
        <path d={p} data-testid="dep-{dep.from}-{dep.to}" fill="none" stroke="#475569" stroke-width="1.2" />
      {/if}
    {/each}

    {#each layout.rows as r, i (r.id)}
      {@const y = i * rowH + 6}
      {@const bb = baselineBar(r, variances[r.id], pxPerDay)}
      <!-- baseline ghost -->
      {#if bb}
        <rect x={bb.x} y={y + rowH - 16} width={bb.w} height="5" rx="2" fill="#475569" opacity="0.6" />
      {/if}
      {#if r.milestone}
        <rect
          x={r.es * pxPerDay - 7}
          y={y + 2}
          width="14"
          height="14"
          transform="rotate(45 {r.es * pxPerDay} {y + 9})"
          fill="#22d3ee"
        />
      {:else}
        {@const split = !!(r.work_segments && r.work_segments.length)}
        {#if split && r.work_segments}
          <!-- split (interrupted) task: a dashed connector spans the gaps
               between the working-day bar pieces below. -->
          <line
            data-testid="split-connector-{r.id}"
            x1={r.work_segments[0].start * pxPerDay}
            y1={y + 7}
            x2={r.work_segments[r.work_segments.length - 1].end * pxPerDay}
            y2={y + 7}
            stroke={r.is_critical ? '#ef4444' : '#0ea5e9'}
            stroke-width="1"
            stroke-dasharray="2 2"
            opacity="0.5"
          />
        {/if}
        {#each barPieces(r, pxPerDay) as piece (piece.x)}
          <rect
            data-testid={split ? `split-seg-${r.id}` : `bar-${r.id}`}
            x={piece.x}
            y={y}
            width={piece.w}
            height="14"
            rx="3"
            fill={r.is_critical ? '#ef4444' : '#0ea5e9'}
            stroke={r.overallocated ? '#fb923c' : 'none'}
            stroke-width={r.overallocated ? 2 : 0}
          />
        {/each}
        {#if !split && r.percent_complete > 0}
          <rect
            x={r.es * pxPerDay}
            y={y + 10}
            width={Math.max(0, (r.ef - r.es) * pxPerDay * Math.min(100, r.percent_complete) / 100)}
            height="4"
            rx="2"
            fill="#0f766e"
          />
        {/if}
      {/if}
      {#if r.constraint_violated}
        <text x={r.ef * pxPerDay + 4} y={y + 12} font-size="11" font-weight="bold" fill="#f59e0b">!</text>
      {/if}
      {#if layout.anchored && r.start_date}
        <text x={r.ef * pxPerDay + (r.constraint_violated ? 14 : 4)} y={y + 12} font-size="9" fill="#64748b">
          {r.start_date} → {r.finish_date}
        </text>
      {/if}
    {/each}
  </svg>
{/if}
