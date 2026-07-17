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
  import { showToast } from '../../toast.svelte';
  import GanttBars from './GanttBars.svelte';
  import { GANTT_ROW_H as rowH, type GanttLayout, type GanttRow } from './gantt_geometry';
  import { splitLevelStatus, splitPreviewMessage, clearWorkSegments } from './leveling_messages';

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
  // Set on every successful SaveChart (auto-persist and manual save alike).
  let lastSavedAt = $state<Date | null>(null);
  // Set when the initial GetChart fails: renders a full-screen error with
  // a way back instead of a stuck editor + unhandled promise rejection.
  let loadError = $state('');
  let pxPerDay = $state(28);
  // AGENT.md §6: every timer must be cleared on destroy.
  let statusTimer: ReturnType<typeof setTimeout> | null = null;

  async function loadChart() {
    if (!session.editingId) return;
    try {
      chart = await window.go.main.App.GetChart(session.editingId);
    } catch (err: any) {
      loadError = `Could not load this chart: ${err?.message ?? err}`;
      return;
    }
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
      lastSavedAt = new Date();
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
      if (statusTimer) clearTimeout(statusTimer);
      statusTimer = setTimeout(() => (status = ''), 2000);
    }
  }

  async function setBaseline() {
    if (!session.editingId) return;
    try {
      await window.go.main.App.SetScheduleBaseline(session.editingId, '');
      await refreshBaseline();
      status = 'Baseline set';
      if (statusTimer) clearTimeout(statusTimer);
      statusTimer = setTimeout(() => (status = ''), 2000);
    } catch (err: any) {
      status = String(err?.message ?? err);
    }
  }

  // previewSplit reports (read-only) whether interrupting tasks across
  // non-contiguous days would resolve overallocation, without changing the
  // saved schedule — the non-destructive counterpart to levelSplit.
  async function previewSplit() {
    if (!session.editingId || saving) return;
    saving = true;
    status = '';
    try {
      const p = await window.go.main.App.PreviewSplitLeveling(session.editingId);
      status = splitPreviewMessage(p).msg;
      setTimeout(() => (status = ''), 4000);
    } catch (err: any) {
      status = String(err?.message ?? err);
    } finally {
      saving = false;
    }
  }

  // levelSplit runs resource leveling with activity splitting on this chart
  // and reloads the layout so split tasks render as interrupted bars. The
  // split working-day runs are persisted on each node as work_segments.
  async function levelSplit() {
    if (!session.editingId || saving) return;
    saving = true;
    status = '';
    try {
      const res = await window.go.main.App.LevelChartResources(
        session.editingId,
        'ltf',
        false,
        true // allow splitting
      );
      chart = await window.go.main.App.GetChart(session.editingId);
      doc = JSON.parse(chart.data) as GDoc;
      await refreshLayout();
      status = splitLevelStatus(res);
      setTimeout(() => (status = ''), 4000);
    } catch (err: any) {
      status = String(err?.message ?? err);
    } finally {
      saving = false;
    }
  }

  // onEdit handles any manual change to the schedule. A manual edit
  // invalidates the leveled split snapshot, so it drops persisted
  // work_segments before re-laying-out; stale interrupted bars can't render
  // until the user re-runs "Level (split)".
  function onEdit() {
    clearWorkSegments(doc.nodes);
    void refreshLayout();
  }

  function addTask() {
    const id = 't' + (Date.now() % 1e7).toString(36);
    doc.nodes.push({ id, label: 'New task', duration: 1, percent_complete: 0 });
    onEdit();
  }

  function deleteTask(id: string) {
    const before = JSON.parse(JSON.stringify(doc)) as typeof doc;
    doc.nodes = doc.nodes.filter((n) => n.id !== id);
    doc.edges = doc.edges.filter((e) => e.from !== id && e.to !== id);
    onEdit();
    showToast('Task deleted', {
      type: 'info',
      undo: () => {
        doc = before;
        void refreshLayout();
      },
    });
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
    onEdit();
  }

  function deleteLink(i: number) {
    doc.edges = doc.edges.filter((_, j) => j !== i);
    onEdit();
  }

  function nodeFor(id: string): GNode | undefined {
    return doc.nodes.find((n) => n.id === id);
  }

  function labelFor(id: string): string {
    return nodeFor(id)?.label ?? id;
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

  onMount(async () => {
    window.addEventListener('keydown', handleKeyDown);
    // Await (rather than a floating .then chain) so a load failure can't
    // become an unhandled rejection, and register auto-save only after a
    // successful load so the baseline snapshot is the saved doc.
    await loadChart();
    if (loadError) return; // failed load: no editor to auto-save
    stopAutosave = autosave.register(
      () => JSON.stringify(doc),
      () => save(),
    );
  });
  onDestroy(() => {
    window.removeEventListener('keydown', handleKeyDown);
    stopAutosave?.();
    if (statusTimer) clearTimeout(statusTimer);
  });
</script>

{#if loadError}
  <div class="min-h-screen bg-slate-950 text-slate-200 flex items-center justify-center">
    <div class="text-center space-y-4 px-6">
      <p class="text-sm text-red-400 break-words" role="alert">{loadError}</p>
      <button
        onclick={() => goto('dashboard')}
        class="text-xs bg-cyan-600 hover:bg-cyan-500 text-white font-bold uppercase px-3 py-2 rounded"
      >
        Back to dashboard
      </button>
    </div>
  </div>
{:else}
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
      {#if lastSavedAt}
        <span class="text-[10px] text-slate-500 tabular-nums" title="Charts save automatically as you edit">
          Saved {lastSavedAt.toLocaleTimeString()}
        </span>
      {/if}
      <button onclick={() => (pxPerDay = Math.max(8, pxPerDay - 6))} class="text-xs bg-slate-800 hover:bg-slate-700 px-2 py-1 rounded" title="Zoom out">−</button>
      <button onclick={() => (pxPerDay = Math.min(80, pxPerDay + 6))} class="text-xs bg-slate-800 hover:bg-slate-700 px-2 py-1 rounded" title="Zoom in">+</button>
      <button onclick={setBaseline} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded" title="Snapshot for baseline ghost bars">Set baseline</button>
      <button onclick={previewSplit} disabled={saving} class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-3 py-1 rounded" title="Check (read-only) whether splitting tasks across non-contiguous days would clear overallocation. Nothing is saved.">Preview splitting</button>
      <button onclick={levelSplit} disabled={saving} class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-3 py-1 rounded" title="Level resources allowing tasks to be split across non-contiguous days; split tasks render as interrupted bars">Level (split)</button>
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
                    onchange={onEdit}
                    class="w-full bg-transparent border border-transparent hover:border-slate-800 focus:border-cyan-500 rounded p-1 outline-none"
                  />
                </td>
                <td class="px-1">
                  <input
                    type="number"
                    min="0"
                    bind:value={n.duration}
                    onchange={onEdit}
                    class="w-full bg-slate-900 border border-slate-800 rounded p-1 font-mono text-right outline-none focus:border-cyan-500"
                  />
                </td>
                <td class="px-1">
                  <input
                    type="number"
                    min="0"
                    max="100"
                    bind:value={n.percent_complete}
                    onchange={onEdit}
                    class="w-full bg-slate-900 border border-slate-800 rounded p-1 font-mono text-right outline-none focus:border-cyan-500"
                  />
                </td>
                <td class="text-center">
                  <input type="checkbox" bind:checked={n.milestone} onchange={onEdit} class="accent-cyan-500" />
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
              onchange={onEdit}
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
      <GanttBars {layout} {pxPerDay} {variances} ariaLabel={ganttSummary} {barTitle} />
      {#if layout.rows.length > 0}
        <p class="text-[10px] text-slate-500 mt-2 max-w-xl">
          Red bars are critical; teal strip = % complete; grey ghost =
          baseline; orange outline = overallocated resource; amber ! =
          constraint violated. Split tasks show interrupted bars. Real dates
          appear when the project has a start date. Link labels accept
          FS/SS/FF/SF with ±lag days.
        </p>
      {/if}
    </main>
  </div>
</div>
{/if}
