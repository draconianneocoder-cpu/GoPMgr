<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later

Shared editor shell for the three layered-DAG editors (Network, PERT,
CPM). Provides node/edge CRUD, save, and a slot for kind-specific
node-detail panels.

This is a helper composed inside NetworkEditor/PERTEditor/CPMEditor;
not routed directly.
-->
<script lang="ts">
  import { desktopApi } from '../../desktop-api';
  import { onMount, onDestroy } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import { autosave } from '../../autosave.svelte';
  import { showToast } from '../../toast.svelte';
  import { t } from '../../i18n/i18n.svelte';
  import LayeredDiagram from './LayeredDiagram.svelte';
  import Button from '../Button.svelte';
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
    constraint_unapplied?: string;
    level_start?: string;
    level_start_fraction?: number;
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
    // Stored by resource levelling: what it read (a digest).
    levelled_inputs?: string;
    // Computed by the CPM layout: the schedule changed since levelling.
    levelling_stale?: boolean;
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
      chart = await desktopApi.GetChart(session.editingId);
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
  // Layout writes these onto the nodes it returns. They are not part of the
  // stored chart. The next layout computes them again.
  const layoutOutputKeys = [
    'expected', 'variance', 'std_dev',
    'es', 'ef', 'ls', 'lf', 'float', 'is_critical',
    'start_date', 'finish_date',
    'constraint_violated', 'constraint_unapplied',
    'overallocated',
  ] as const;

  function storedDoc(): LayeredDoc {
    const plain = $state.snapshot(doc);
    return {
      nodes: plain.nodes.map((node) => {
        const stored: LayeredNode = { ...node };
        for (const key of layoutOutputKeys) delete stored[key];
        return stored;
      }),
      edges: plain.edges,
      // Keep levelling's stored digest; levelling_stale is a layout output.
      ...(plain.levelled_inputs ? { levelled_inputs: plain.levelled_inputs } : {}),
    };
  }

  // The stale-levelling note can be dismissed; it returns the next time
  // the schedule goes stale after being current.
  let staleDismissed = $state(false);
  $effect(() => {
    if (!doc.levelling_stale) staleDismissed = false;
  });

  // One function, so saveIfDirty addresses this editor's auto-save entry.
  // The string is the stored chart, not the layout result.
  const snapshotDoc = () => JSON.stringify(storedDoc());

  onMount(async () => {
    await loadChart();
    if (loadError) return; // failed load: no editor to auto-save
    // Register after load so the baseline snapshot is the saved doc and
    // auto-save only fires on real edits.
    stopAutosave = autosave.register(snapshotDoc, () => writeChart());
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
      // Opening a chart does not save. After registration, persist a user
      // edit through auto-save before layout reads the stored chart.
      if (stopAutosave) {
        const saved = await autosave.saveRegisteredIfDirty(snapshotDoc);
        if (!saved) {
          layoutError = autosave.lastError || 'Save failed';
          return;
        }
      }
      const res = await desktopApi.LayoutChart(chart.id);
      // For layered kinds the body shape is {layout, doc}.
      const body = res.body as { layout?: any; doc?: LayeredDoc } | any;
      if (body && body.layout) {
        layout = body.layout;
        if (body.doc) doc = body.doc; // display backend annotations (PERT, CPM)
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

  // save is the manual save (Ctrl/Cmd+S, the Save button and editor
  // actions). Once registered it goes through auto-save, which waits for a
  // save in flight and records what was saved, so saveIfDirty does not
  // rewrite a chart the user has just saved.
  export async function save(): Promise<boolean> {
    if (!chart) return false;
    if (stopAutosave) return autosave.saveRegisteredNow(snapshotDoc);
    return writeChart();
  }

  // writeChart writes the editor's content to the stored chart.
  async function writeChart(): Promise<boolean> {
    if (!chart) return false;
    saving = true;
    status = '';
    try {
      const updated = await desktopApi.SaveChart({
        ...chart,
        data: snapshotDoc(),
      });
      chart = updated;
      lastSavedAt = new Date();
      status = `Saved at ${new Date().toLocaleTimeString()}.`;
      return true;
    } catch (err: any) {
      status = `Save failed: ${err}`;
      return false;
    } finally {
      saving = false;
    }
  }

  // saveIfDirty saves only edits auto-save has not recorded yet, waiting for
  // a save already in flight. A read-only check such as the schedule audit
  // calls it first, so it reads the chart as shown without rewriting an
  // unchanged chart (which would change its revision). True when the stored
  // chart now holds the editor's content.
  export async function saveIfDirty(): Promise<boolean> {
    if (!chart) return false;
    if (!stopAutosave) return writeChart();
    return autosave.saveRegisteredIfDirty(snapshotDoc);
  }

  // loadedChartId is the ID of the chart this editor holds and saves. The
  // router can reuse the editor after session.editingId changes, so callers
  // that act on the stored chart use this ID rather than the session's.
  export function loadedChartId(): string | null {
    return chart?.id ?? null;
  }

  // hasNode reports whether the chart has a node with this ID.
  export function hasNode(id: string): boolean {
    return doc.nodes.some((n) => n.id === id);
  }

  // selectNode selects an existing node, as a click on it would, and returns
  // false when the chart has no node with that ID.
  export function selectNode(id: string): boolean {
    if (!hasNode(id)) return false;
    if (pendingEdge) {
      // Leave connect mode rather than linking to the selected task.
      pendingEdge = null;
      status = '';
    }
    selectedId = id;
    return true;
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
  // call on an unmounted component. (DEVELOPER_HANDBOOK.md §10.5.)
  onDestroy(() => {
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

<svelte:window onkeydown={handleKeyDown} />

{#if loadError}
  <div class="min-h-screen bg-slate-950 text-slate-200 flex items-center justify-center">
    <div class="text-center space-y-4 px-6">
      <p class="text-sm text-red-400 break-words" role="alert">{loadError}</p>
      <Button variant="primary" size="md" onclick={() => goto('dashboard')}>
        Back to dashboard
      </Button>
    </div>
  </div>
{:else}
<div class="min-h-screen bg-slate-950 text-slate-200 flex flex-col">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <Button variant="nav" onclick={() => goto('dashboard')}>
        &larr; Dashboard
      </Button>
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
      <Button variant="canvas" size="sm" onclick={startConnect} disabled={!selectedId}>
        Connect…
      </Button>
      <Button variant="canvas" size="sm" onclick={clearEdgesFromSelected} disabled={!selectedId}>
        Clear edges
      </Button>
      <Button variant="canvas-danger" size="sm" onclick={deleteNode} disabled={!selectedId}>
        Delete node
      </Button>
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
      {#if doc.levelling_stale && !staleDismissed}
        <div class="mb-2 flex items-center gap-2 text-xs text-amber-300" role="status">
          <span>{t('leveling.stale')}</span>
          <Button variant="secondary" size="compact" onclick={() => (staleDismissed = true)}>{t('leveling.stale.dismiss')}</Button>
        </div>
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
