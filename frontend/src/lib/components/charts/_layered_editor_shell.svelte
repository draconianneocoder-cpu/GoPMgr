<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later

Shared editor shell for the three layered-DAG editors (Network, PERT,
CPM). Provides node/edge CRUD, save, and a slot for kind-specific
node-detail panels.

This is a helper composed inside NetworkEditor/PERTEditor/CPMEditor;
not routed directly.
-->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import { autosave } from '../../autosave.svelte';
  import { showToast } from '../../toast.svelte';
  import LayeredDiagram from './LayeredDiagram.svelte';
  import type { Snippet } from 'svelte';

  interface LayeredNode {
    id: string;
    label: string;
    [k: string]: unknown;
    note?: string;
    owner?: string;
    duration?: number;
    duration_estimate?: DurationEstimate;
    o?: number;
    m?: number;
    p?: number;
    expected?: number;
    variance?: number;
    std_dev?: number;
    es?: number;
    ef?: number;
    ls?: number;
    lf?: number;
    float?: number;
    is_critical?: boolean;
    start_date?: string;
    finish_date?: string;
    constraint?: string;
    constraint_date?: string;
    constraint_violated?: boolean;
    percent_complete?: number;
    milestone?: boolean;
    actual_start?: string;
    actual_finish?: string;
    budgeted_cost?: number;
    budgeted_cost_minor_units?: number;
    actual_cost?: number;
    actual_cost_minor_units?: number;
    assignments?: {
      resource: string;
      units?: number;
      calendar_id?: string;
      skill_tags?: string[];
      max_units?: number;
    }[];
    overallocated?: boolean;
  }
  interface LayeredEdge {
    from: string;
    to: string;
    label?: string;
  }
  interface LayeredLayoutNode {
    id: string;
    title: string;
    note?: string;
    owner?: string;
    depth: number;
    x: number;
    y: number;
    width: number;
    height: number;
  }
  interface LayeredDoc {
    nodes: LayeredNode[];
    edges: LayeredEdge[];
  }

  // Props
  let {
    chartKind,
    headingLabel,
    nodeDetailPanel,
    nodeContent,
    toolbarExtra,
    asideExtra,
  }: {
    chartKind: string; // 'network' | 'pert' | 'cpm'
    headingLabel: string;
    nodeDetailPanel?: Snippet<[LayeredNode]>;
    nodeContent?: Snippet<[LayeredNode, LayeredLayoutNode]>;
    /** Optional kind-specific toolbar buttons (e.g. CPM's Set baseline). */
    toolbarExtra?: Snippet<[]>;
    /** Optional chart-level panel at the bottom of the aside (e.g. CPM's EVM card). */
    asideExtra?: Snippet<[]>;
  } = $props();

  // State
  let chart = $state<ChartRecord | null>(null);
  let doc = $state<LayeredDoc>({ nodes: [], edges: [] });
  let layout = $state<{ nodes: any[]; edges: any[]; width: number; height: number }>({
    nodes: [],
    edges: [],
    width: 0,
    height: 0,
  });
  let selectedId = $state<string | null>(null);
  let status = $state('');
  let saving = $state(false);
  // Set on every successful SaveChart (auto-persist and manual save alike).
  let lastSavedAt = $state<Date | null>(null);
  // Set when the initial GetChart fails: renders a full-screen error with
  // a way back instead of a stuck editor + unhandled promise rejection.
  let loadError = $state('');
  let layoutError = $state('');
  let pendingEdge = $state<string | null>(null); // ID of from-node when picking the to-node

  function handleKeyDown(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key === 's') {
      e.preventDefault();
      void save();
    }
  }

  // Load
  async function loadChart() {
    if (!session.editingId) return;
    try {
      chart = await window.go.main.App.GetChart(session.editingId);
    } catch (err: any) {
      loadError = `Could not load this chart: ${err?.message ?? err}`;
      return;
    }
    try {
      doc = JSON.parse(chart.data) as LayeredDoc;
      doc.nodes ??= [];
      doc.edges ??= [];
    } catch {
      doc = { nodes: [], edges: [] };
    }
    await refreshLayout();
  }

  let stopAutosave: (() => void) | null = null;

  onMount(async () => {
    window.addEventListener('keydown', handleKeyDown);
    await loadChart();
    if (loadError) return; // failed load: no editor to auto-save
    // Register after load so the baseline snapshot is the saved doc and
    // auto-save only fires on real edits.
    stopAutosave = autosave.register(
      () => JSON.stringify(doc),
      () => save(),
    );
  });

  // reloadFromDB lets kind-specific toolbars (e.g. CPM's Level
  // resources, which mutates the chart server-side) refresh the
  // in-memory doc so a later Ctrl+S can't clobber backend changes.
  export async function reloadFromDB() {
    await loadChart();
  }

  async function refreshLayout() {
    if (!chart) return;
    layoutError = '';
    try {
      const updated = await window.go.main.App.SaveChart({
        ...chart,
        data: JSON.stringify(doc),
      });
      chart = updated;
      lastSavedAt = new Date();
      const res = await window.go.main.App.LayoutChart(updated.id);
      // For layered kinds the body shape is {layout, doc}.
      const body = res.body as { layout?: any; doc?: LayeredDoc } | any;
      if (body && body.layout) {
        layout = body.layout;
        if (body.doc) doc = body.doc; // accept backend annotations (PERT, CPM)
      } else {
        layout = body;
      }
    } catch (err: any) {
      layoutError = String(err?.message ?? err);
    }
  }

  // Node CRUD
  function newID(): string {
    return 'n_' + Math.random().toString(36).slice(2, 8);
  }
  function addNode() {
    const id = newID();
    doc.nodes.push({ id, label: 'New activity' });
    doc.nodes = [...doc.nodes];
    selectedId = id;
    void refreshLayout();
  }
  // Destructive edits persist immediately (refreshLayout saves), so each
  // offers an undo toast holding a pre-delete snapshot of the whole doc.
  // Undo restores that snapshot; edits made in the few seconds between
  // delete and undo are rolled back with it — an acceptable trade for a
  // single-user editor with a short toast window.
  function deleteNode() {
    if (!selectedId) return;
    const before = JSON.parse(JSON.stringify(doc)) as LayeredDoc;
    doc.nodes = doc.nodes.filter((n) => n.id !== selectedId);
    doc.edges = doc.edges.filter((e) => e.from !== selectedId && e.to !== selectedId);
    selectedId = null;
    void refreshLayout();
    showToast('Node deleted', {
      type: 'info',
      undo: () => {
        doc = before;
        void refreshLayout();
      },
    });
  }

  // Edge CRUD via two-click "connect" mode
  function startConnect() {
    if (!selectedId) return;
    pendingEdge = selectedId;
    status = 'Connect mode: click the destination node.';
  }
  function handleNodeClick(id: string) {
    if (pendingEdge && pendingEdge !== id) {
      // Avoid duplicates and self-loops.
      const exists = doc.edges.some((e) => e.from === pendingEdge && e.to === id);
      if (!exists) {
        doc.edges.push({ from: pendingEdge, to: id });
        doc.edges = [...doc.edges];
      }
      pendingEdge = null;
      status = '';
      void refreshLayout();
    } else {
      selectedId = id;
    }
  }
  function clearEdgesFromSelected() {
    if (!selectedId) return;
    const before = JSON.parse(JSON.stringify(doc)) as LayeredDoc;
    doc.edges = doc.edges.filter((e) => e.from !== selectedId && e.to !== selectedId);
    void refreshLayout();
    showToast('Edges cleared', {
      type: 'info',
      undo: () => {
        doc = before;
        void refreshLayout();
      },
    });
  }

  export async function save() {
    if (!chart) return;
    saving = true;
    status = '';
    try {
      const updated = await window.go.main.App.SaveChart({
        ...chart,
        data: JSON.stringify(doc),
      });
      chart = updated;
      lastSavedAt = new Date();
      status = `Saved at ${new Date().toLocaleTimeString()}.`;
    } catch (err: any) {
      status = `Save failed: ${err}`;
    } finally {
      saving = false;
    }
  }

  let selectedNode = $derived(
    selectedId ? doc.nodes.find((n) => n.id === selectedId) : null,
  );

  // Re-layout when the selected node's user-editable fields change.
  // (PERT/CPM annotations come back from the backend.)
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;
  $effect(() => {
    if (!selectedNode) return;
    selectedNode.label;
    selectedNode.duration;
    selectedNode.duration_estimate?.optimistic;
    selectedNode.duration_estimate?.most_likely;
    selectedNode.duration_estimate?.pessimistic;
    selectedNode.duration_estimate?.distribution;
    selectedNode.o;
    selectedNode.m;
    selectedNode.p;
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => void refreshLayout(), 300);
  });

  // Concurrency hardening: cancel pending debounce on unmount so
  // navigation away from a half-edited chart doesn't fire a save
  // call on an unmounted component. (AGENT.md §6.)
  onDestroy(() => {
    window.removeEventListener('keydown', handleKeyDown);
    stopAutosave?.();
    if (debounceTimer) {
      clearTimeout(debounceTimer);
      debounceTimer = null;
    }
  });

  // Bridge LayeredDiagram's selection events back through handleNodeClick
  // so the connect-mode flow gets a chance to claim the click.
  let diagSelectedId: string | null = $state(null);
  $effect(() => {
    if (diagSelectedId && diagSelectedId !== selectedId) {
      handleNodeClick(diagSelectedId);
      diagSelectedId = null;
    }
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
      <h1 class="text-sm font-bold tracking-widest uppercase text-slate-50">{headingLabel}</h1>
    </div>
    <div class="flex items-center gap-2">
      {#if lastSavedAt}
        <span class="text-[10px] text-slate-500 tabular-nums" title="Charts save automatically as you edit">
          Saved {lastSavedAt.toLocaleTimeString()}
        </span>
      {/if}
      {#if toolbarExtra}
        {@render toolbarExtra()}
      {/if}
      <button onclick={addNode} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded">
        + Node
      </button>
      <button
        onclick={startConnect}
        disabled={!selectedId}
        class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-30 px-3 py-1 rounded"
      >
        Connect…
      </button>
      <button
        onclick={clearEdgesFromSelected}
        disabled={!selectedId}
        class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-30 px-3 py-1 rounded"
      >
        Clear edges
      </button>
      <button
        onclick={deleteNode}
        disabled={!selectedId}
        class="text-xs bg-slate-800 hover:bg-red-900 disabled:opacity-30 px-3 py-1 rounded"
      >
        Delete node
      </button>
      <button
        onclick={save}
        disabled={saving}
        class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-3 py-1 rounded"
      >
        {saving ? 'Saving...' : 'Save'}
      </button>
    </div>
  </header>

  <div class="flex-1 flex">
    <main class="flex-1 overflow-auto p-6">
      {#if status}
        <p class="text-xs text-cyan-400 mb-2" role="status" aria-live="polite">{status}</p>
      {/if}
      {#if layoutError}
        <p class="text-xs text-red-400 mb-2" role="alert">Layout error: {layoutError}</p>
      {/if}
      {#if doc.nodes.length === 0}
        <p class="text-sm text-slate-500 text-center mt-12">
          Empty diagram. Click <strong>+ Node</strong> to start.
        </p>
      {:else}
        <LayeredDiagram
          {layout}
          nodes={doc.nodes}
          bind:selectedId={diagSelectedId}
          {nodeContent}
        />
      {/if}
    </main>

    <aside class="w-96 border-l border-slate-800 p-4 bg-slate-900">
      <h2 class="text-xs font-bold tracking-widest uppercase text-slate-500 mb-4">
        Selected node
      </h2>
      {#if selectedNode}
        <div class="space-y-3 text-sm">
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Label</span>
            <input
              bind:value={selectedNode.label}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Owner</span>
            <input
              bind:value={selectedNode.owner}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Note</span>
            <textarea
              bind:value={selectedNode.note}
              rows="3"
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            ></textarea>
          </label>

          {#if doc.edges.some((e) => e.to === selectedNode.id)}
            <div class="border-t border-slate-800 pt-3">
              <span class="text-xs text-slate-500 uppercase">Incoming links</span>
              {#each doc.edges.filter((e) => e.to === selectedNode.id) as edge (edge.from)}
                <div class="flex items-center gap-2 mt-1">
                  <span class="flex-1 text-xs text-slate-400 truncate">
                    from {doc.nodes.find((n) => n.id === edge.from)?.label ?? edge.from}
                  </span>
                  <input
                    bind:value={edge.label}
                    placeholder="FS"
                    class="w-24 bg-slate-950 border border-slate-800 p-1 rounded text-xs font-mono focus:border-cyan-500 outline-none"
                  />
                </div>
              {/each}
              {#if chartKind === 'cpm'}
                <p class="text-[10px] text-slate-500 mt-1">
                  Link type and lag in days: FS, SS, FF, SF with optional
                  +n/-n (e.g. SS+2, FS-1). Blank = FS. Drives the schedule.
                </p>
              {/if}
            </div>
          {/if}

          {#if nodeDetailPanel}
            <div class="border-t border-slate-800 pt-3">
              {@render nodeDetailPanel(selectedNode)}
            </div>
          {/if}

          <p class="text-[10px] text-slate-500">ID: {selectedNode.id}</p>
        </div>
      {:else}
        <p class="text-xs text-slate-500">
          Click a node in the diagram to edit it. Click <strong>+ Node</strong> to add one.
        </p>
      {/if}

      {#if asideExtra}
        <div class="border-t border-slate-800 mt-4 pt-4">
          {@render asideExtra()}
        </div>
      {/if}
    </aside>
  </div>
</div>
{/if}
