<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // GanttEditor: the first-class Gantt chart kind. Shares the
  // layered/CPM data model ({nodes, edges}); the backend computes the
  // full schedule (typed links + lag, constraints, overallocation,
  // anchored dates) via dag.LayoutGantt[Scheduled]. The editor is an
  // editable task grid on the left and a scaled bar canvas on the
  // right with dependency arrows, critical colouring, progress
  // overlay, and baseline ghost bars.
  import { onMount, onDestroy } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import { autosave } from '../../autosave.svelte';

  interface GanttRow {
    id: string;
    label: string;
    es: number;
    ef: number;
    float: number;
    is_critical: boolean;
    milestone: boolean;
    percent_complete: number;
    start_date?: string;
    finish_date?: string;
    overallocated?: boolean;
    constraint_violated?: boolean;
  }
  interface GanttLayout {
    rows: GanttRow[];
    deps: { from: string; to: string; label?: string }[];
    horizon: number;
    anchored: boolean;
  }
  interface GNode {
    id: string;
    label: string;
    duration?: number;
    milestone?: boolean;
    percent_complete?: number;
    [k: string]: unknown;
  }
  interface GDoc {
    nodes: GNode[];
    edges: { from: string; to: string; label?: string }[];
  }

  let chart = $state<ChartRecord | null>(null);
  let doc = $state<GDoc>({ nodes: [], edges: [] });
  let layout = $state<GanttLayout>({ rows: [], deps: [], horizon: 0, anchored: false });
  let variances = $state<Record<string, ScheduleVariance>>({});
  let status = $state('');
  let saving = $state(false);
  let pxPerDay = $state(28);

  const rowH = 30;
  const labelW = 0; // labels live in the grid; canvas is bars only

  async function loadChart() {
    if (!session.editingId) return;
    chart = await window.go.main.App.GetChart(session.editingId);
    try {
      doc = JSON.parse(chart.data) as GDoc;
      doc.nodes ??= [];
      doc.edges ??= [];
    } catch {
      doc = { nodes: [], edges: [] };
    }
    await refreshLayout();
    void refreshBaseline();
  }

  async function refreshLayout() {
    if (!chart) return;
    status = '';
    try {
      const updated = await window.go.main.App.SaveChart({
        ...chart,
        data: JSON.stringify(doc),
      });
      chart = updated;
      const res = await window.go.main.App.LayoutChart(updated.id);
      const body = res.body as { layout?: GanttLayout } | GanttLayout;
      layout = (body as any).layout ?? (body as GanttLayout);
    } catch (err: any) {
      status = String(err?.message ?? err);
    }
  }

  async function refreshBaseline() {
    if (!session.editingId) return;
    try {
      const list = await window.go.main.App.ListScheduleBaselines(session.editingId);
      variances = (list?.length ?? 0) > 0
        ? await window.go.main.App.CompareScheduleBaseline(session.editingId, '')
        : {};
    } catch {
      variances = {};
    }
  }

  async function save() {
    saving = true;
    await refreshLayout();
    saving = false;
    if (!status) {
      status = 'Saved';
      setTimeout(() => (status = ''), 2000);
    }
  }

  async function setBaseline() {
    if (!session.editingId) return;
    try {
      await window.go.main.App.SetScheduleBaseline(session.editingId, '');
      await refreshBaseline();
      status = 'Baseline set';
      setTimeout(() => (status = ''), 2000);
    } catch (err: any) {
      status = String(err?.message ?? err);
    }
  }

  function addTask() {
    const id = 't' + (Date.now() % 1e7).toString(36);
    doc.nodes.push({ id, label: 'New task', duration: 1, percent_complete: 0 });
    void refreshLayout();
  }

  function deleteTask(id: string) {
    doc.nodes = doc.nodes.filter((n) => n.id !== id);
    doc.edges = doc.edges.filter((e) => e.from !== id && e.to !== id);
    void refreshLayout();
  }

  // Link editing
  let linkFrom = $state('');
  let linkTo = $state('');
  let linkLabel = $state('');

  function addLink() {
    if (!linkFrom || !linkTo || linkFrom === linkTo) return;
    if (doc.edges.some((e) => e.from === linkFrom && e.to === linkTo)) return;
    doc.edges.push({ from: linkFrom, to: linkTo, label: linkLabel || undefined });
    linkLabel = '';
    void refreshLayout();
  }

  function deleteLink(i: number) {
    doc.edges = doc.edges.filter((_, j) => j !== i);
    void refreshLayout();
  }

  function nodeFor(id: string): GNode | undefined {
    return doc.nodes.find((n) => n.id === id);
  }

  function labelFor(id: string): string {
    return nodeFor(id)?.label ?? id;
  }

  const rowIndex = $derived(new Map(layout.rows.map((r, i) => [r.id, i])));
  const canvasW = $derived(Math.max(300, layout.horizon * pxPerDay + 60));
  const canvasH = $derived(Math.max(60, layout.rows.length * rowH + 24));

  function baselineBar(r: GanttRow): { x: number; w: number } | null {
    const v = variances[r.id];
    if (!v) return null;
    const bes = r.es - v.start_var_days;
    const bef = r.ef - v.finish_var_days;
    if (bef <= bes) return null;
    return { x: bes * pxPerDay, w: (bef - bes) * pxPerDay };
  }

  function depPath(d: { from: string; to: string }): string | null {
    const fi = rowIndex.get(d.from);
    const ti = rowIndex.get(d.to);
    const fr = layout.rows[fi ?? -1];
    const tr = layout.rows[ti ?? -1];
    if (fi === undefined || ti === undefined || !fr || !tr) return null;
    const x1 = fr.ef * pxPerDay;
    const y1 = fi * rowH + rowH / 2;
    const x2 = tr.es * pxPerDay;
    const y2 = ti * rowH + rowH / 2;
    const elbow = Math.max(x1 + 8, x2 - 8);
    return `M ${x1} ${y1} L ${elbow} ${y1} L ${elbow} ${y2} L ${x2} ${y2}`;
  }

  // The bar canvas is a visual view of data that is fully editable in the
  // task grid on the left, so it stays a labelled image rather than a second
  // set of tab stops. Give it a description that summarises the schedule, and
  // a per-bar <title> so pointer users get a tooltip identifying each bar.
  const ganttSummary = $derived.by(() => {
    const n = layout.rows.length;
    const critical = layout.rows.filter((r) => r.is_critical).length;
    const days = Math.max(0, Math.ceil(layout.horizon));
    let s = `Gantt chart: ${n} task${n === 1 ? '' : 's'} across ${days} day${days === 1 ? '' : 's'}`;
    if (critical > 0) s += `, ${critical} on the critical path`;
    return s + '.';
  });

  function barTitle(r: GanttRow): string {
    const parts = [r.label || '(untitled task)'];
    if (r.milestone) {
      parts.push(`milestone at day ${r.es}`);
    } else {
      parts.push(`day ${r.es} to ${r.ef}`);
      if (r.percent_complete > 0) parts.push(`${Math.min(100, r.percent_complete)}% complete`);
    }
    if (r.is_critical) parts.push('critical path');
    if (r.overallocated) parts.push('resource overallocated');
    if (r.constraint_violated) parts.push('constraint violated');
    if (r.start_date && r.finish_date) parts.push(`${r.start_date} to ${r.finish_date}`);
    return parts.join(', ') + '.';
  }

  function handleKeyDown(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key === 's') {
      e.preventDefault();
      void save();
    }
  }

  let stopAutosave: (() => void) | null = null;

  onMount(() => {
    window.addEventListener('keydown', handleKeyDown);
    // Register after the chart loads so the baseline snapshot is the saved
    // doc and auto-save only fires on real edits.
    void loadChart().then(() => {
      stopAutosave = autosave.register(
        () => JSON.stringify(doc),
        () => save(),
      );
    });
  });
  onDestroy(() => {
    window.removeEventListener('keydown', handleKeyDown);
    stopAutosave?.();
  });
</script>

<div class="min-h-screen bg-slate-950 text-slate-200 flex flex-col">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <button onclick={() => goto('dashboard')} class="text-xs text-slate-400 hover:text-cyan-400">
        &larr; Dashboard
      </button>
      <h1 class="text-sm font-bold tracking-widest uppercase text-slate-50">Gantt Chart</h1>
      {#if status}<span class="text-xs text-cyan-300" role="status" aria-live="polite">{status}</span>{/if}
    </div>
    <div class="flex items-center gap-2">
      <button onclick={() => (pxPerDay = Math.max(8, pxPerDay - 6))} class="text-xs bg-slate-800 hover:bg-slate-700 px-2 py-1 rounded" title="Zoom out">−</button>
      <button onclick={() => (pxPerDay = Math.min(80, pxPerDay + 6))} class="text-xs bg-slate-800 hover:bg-slate-700 px-2 py-1 rounded" title="Zoom in">+</button>
      <button onclick={setBaseline} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded" title="Snapshot for baseline ghost bars">Set baseline</button>
      <button onclick={addTask} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded">+ Task</button>
      <button onclick={save} disabled={saving} class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-3 py-1 rounded">
        {saving ? 'Saving...' : 'Save'}
      </button>
    </div>
  </header>

  <div class="flex-1 flex overflow-hidden">
    <!-- Task grid -->
    <aside class="w-[420px] border-r border-slate-800 overflow-y-auto">
      <table class="w-full text-xs">
        <thead class="text-slate-500 uppercase text-[10px] sticky top-0 bg-slate-950">
          <tr>
            <th class="text-left p-2">Task</th>
            <th class="w-14 p-1">Days</th>
            <th class="w-12 p-1">%</th>
            <th class="w-8 p-1" title="Milestone">◆</th>
            <th class="w-8"></th>
          </tr>
        </thead>
        <tbody>
          {#each layout.rows as r (r.id)}
            {@const n = nodeFor(r.id)}
            {#if n}
              <tr class="border-t border-slate-900" style="height: {rowH}px">
                <td class="px-2">
                  <input
                    bind:value={n.label}
                    onchange={refreshLayout}
                    class="w-full bg-transparent border border-transparent hover:border-slate-800 focus:border-cyan-500 rounded p-1 outline-none"
                  />
                </td>
                <td class="px-1">
                  <input
                    type="number"
                    min="0"
                    bind:value={n.duration}
                    onchange={refreshLayout}
                    class="w-full bg-slate-900 border border-slate-800 rounded p-1 font-mono text-right outline-none focus:border-cyan-500"
                  />
                </td>
                <td class="px-1">
                  <input
                    type="number"
                    min="0"
                    max="100"
                    bind:value={n.percent_complete}
                    onchange={refreshLayout}
                    class="w-full bg-slate-900 border border-slate-800 rounded p-1 font-mono text-right outline-none focus:border-cyan-500"
                  />
                </td>
                <td class="text-center">
                  <input type="checkbox" bind:checked={n.milestone} onchange={refreshLayout} class="accent-cyan-500" />
                </td>
                <td class="text-center">
                  <button onclick={() => deleteTask(r.id)} class="text-slate-600 hover:text-red-400" title="Delete task">✕</button>
                </td>
              </tr>
            {/if}
          {/each}
        </tbody>
      </table>

      <div class="p-3 border-t border-slate-800">
        <span class="text-[10px] uppercase text-slate-500">Links (FS/SS/FF/SF ± lag)</span>
        {#each doc.edges as e, i (e.from + '>' + e.to)}
          <div class="flex items-center gap-1 mt-1 text-xs">
            <span class="flex-1 truncate text-slate-400">{labelFor(e.from)} → {labelFor(e.to)}</span>
            <input
              bind:value={e.label}
              onchange={refreshLayout}
              placeholder="FS"
              class="w-16 bg-slate-900 border border-slate-800 rounded p-1 font-mono outline-none focus:border-cyan-500"
            />
            <button onclick={() => deleteLink(i)} class="text-slate-600 hover:text-red-400">✕</button>
          </div>
        {/each}
        <div class="flex items-center gap-1 mt-2">
          <select bind:value={linkFrom} class="flex-1 bg-slate-900 border border-slate-800 rounded p-1 text-xs outline-none">
            <option value="">from…</option>
            {#each doc.nodes as n (n.id)}<option value={n.id}>{n.label}</option>{/each}
          </select>
          <select bind:value={linkTo} class="flex-1 bg-slate-900 border border-slate-800 rounded p-1 text-xs outline-none">
            <option value="">to…</option>
            {#each doc.nodes as n (n.id)}<option value={n.id}>{n.label}</option>{/each}
          </select>
          <input bind:value={linkLabel} placeholder="FS+1" class="w-16 bg-slate-900 border border-slate-800 rounded p-1 text-xs font-mono outline-none" />
          <button onclick={addLink} class="text-xs bg-slate-800 hover:bg-slate-700 px-2 py-1 rounded">+</button>
        </div>
      </div>
    </aside>

    <!-- Bar canvas -->
    <main class="flex-1 overflow-auto p-4">
      {#if layout.rows.length === 0}
        <p class="text-slate-500 text-sm">No tasks yet. Click <strong>+ Task</strong> to add one.</p>
      {:else}
        <svg width={canvasW} height={canvasH} role="img" aria-label={ganttSummary}>
          <!-- day grid -->
          {#each Array(Math.ceil(layout.horizon) + 1) as _, d (d)}
            <line x1={d * pxPerDay} y1="0" x2={d * pxPerDay} y2={layout.rows.length * rowH} stroke="#1e293b" stroke-width="1" />
            {#if pxPerDay >= 20 || d % 5 === 0}
              <text x={d * pxPerDay + 2} y={layout.rows.length * rowH + 14} font-size="9" fill="#475569">{d}</text>
            {/if}
          {/each}

          <!-- dependency arrows under the bars -->
          {#each layout.deps as dep (dep.from + '>' + dep.to)}
            {@const p = depPath(dep)}
            {#if p}
              <path d={p} fill="none" stroke="#475569" stroke-width="1.2" />
            {/if}
          {/each}

          {#each layout.rows as r, i (r.id)}
            {@const y = i * rowH + 6}
            {@const bb = baselineBar(r)}
            <g>
              <title>{barTitle(r)}</title>
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
              <rect
                x={r.es * pxPerDay}
                y={y}
                width={Math.max(2, (r.ef - r.es) * pxPerDay)}
                height="14"
                rx="3"
                fill={r.is_critical ? '#ef4444' : '#0ea5e9'}
                stroke={r.overallocated ? '#fb923c' : 'none'}
                stroke-width={r.overallocated ? 2 : 0}
              />
              {#if r.percent_complete > 0}
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
            </g>
          {/each}
        </svg>
        <p class="text-[10px] text-slate-500 mt-2 max-w-xl">
          Red bars are critical; teal strip = % complete; grey ghost =
          baseline; orange outline = overallocated resource; amber ! =
          constraint violated. Real dates appear when the project has a
          start date. Link labels accept FS/SS/FF/SF with ±lag days.
        </p>
      {/if}
    </main>
  </div>
</div>
