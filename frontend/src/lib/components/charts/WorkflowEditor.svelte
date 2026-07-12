<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // WorkflowEditor renders a classic process flowchart with six node
  // shapes:
  //
  //   start      — green oval
  //   end        — red oval
  //   action     — rounded rectangle
  //   decision   — yellow diamond
  //   io         — blue parallelogram
  //   subprocess — purple rectangle with double vertical bars
  //
  // Layout is top-to-bottom, rank-based. Edges support optional labels
  // ("Yes" / "No" / etc.) drawn near the midpoint of the connector.
  //
  // UX pattern matches NetworkEditor: pick a node, edit its label and
  // shape in the side panel; connect-mode (click source, click target)
  // creates edges; selected node can be deleted.

  import { onMount, onDestroy } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import { showToast } from '../../toast.svelte';
  import { autosave } from '../../autosave.svelte';
  import {
    shapePath,
    shapeFill,
    shapeTextFill,
    edgePath,
    edgeLabelPosition,
    type FlowNode,
  } from './_flow_shapes';

  interface WfNode {
    id: string;
    label: string;
    shape: string;
  }
  interface WfEdge {
    from: string;
    to: string;
    label?: string;
  }
  interface WfDoc {
    nodes: WfNode[];
    edges: WfEdge[];
  }
  interface Layout {
    nodes: FlowNode[];
    edges: { from: string; to: string; label?: string }[];
    width: number;
    height: number;
  }

  const SHAPES: { value: string; label: string }[] = [
    { value: 'start', label: 'Start (oval)' },
    { value: 'end', label: 'End (oval)' },
    { value: 'action', label: 'Action (rectangle)' },
    { value: 'decision', label: 'Decision (diamond)' },
    { value: 'io', label: 'Input/Output (parallelogram)' },
    { value: 'subprocess', label: 'Subprocess' },
  ];

  let chart = $state<ChartRecord | null>(null);
  let doc = $state<WfDoc>({ nodes: [], edges: [] });
  let layout = $state<Layout>({ nodes: [], edges: [], width: 0, height: 0 });
  let selectedId = $state<string | null>(null);
  let pendingEdge = $state<string | null>(null);
  let status = $state('');
  let layoutError = $state('');
  let saving = $state(false);
  // Set on every successful SaveChart (auto-persist and manual save alike).
  let lastSavedAt = $state<Date | null>(null);

  let stopAutosave: (() => void) | null = null;

  onMount(async () => {
    if (!session.editingId) return;
    chart = await window.go.main.App.GetChart(session.editingId);
    try {
      doc = JSON.parse(chart.data) as WfDoc;
      doc.nodes ??= [];
      doc.edges ??= [];
    } catch {
      doc = { nodes: [], edges: [] };
    }
    await refreshLayout();
    // Register for timed auto-save now the saved doc is loaded.
    stopAutosave = autosave.register(
      () => JSON.stringify(doc),
      () => save(),
    );
  });

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
      layout = res.body as Layout;
    } catch (err: any) {
      layoutError = String(err?.message ?? err);
    }
  }

  function newID(): string {
    return 'wf_' + Math.random().toString(36).slice(2, 8);
  }

  function addNode(shape: string) {
    const id = newID();
    doc.nodes.push({
      id,
      label: defaultLabel(shape),
      shape,
    });
    doc.nodes = [...doc.nodes];
    selectedId = id;
    void refreshLayout();
  }

  function defaultLabel(shape: string): string {
    switch (shape) {
      case 'start': return 'Start';
      case 'end': return 'End';
      case 'decision': return 'Decision?';
      case 'io': return 'Input';
      case 'subprocess': return 'Subprocess';
      default: return 'Action';
    }
  }

  function deleteNode() {
    if (!selectedId) return;
    const before = JSON.parse(JSON.stringify(doc)) as typeof doc;
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

  function startConnect() {
    if (!selectedId) return;
    pendingEdge = selectedId;
    status = 'Connect mode: click the destination node.';
  }

  function handleNodeClick(id: string) {
    if (pendingEdge && pendingEdge !== id) {
      if (!doc.edges.some((e) => e.from === pendingEdge && e.to === id)) {
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

  function deleteEdge(idx: number) {
    doc.edges = doc.edges.filter((_, i) => i !== idx);
    void refreshLayout();
  }

  async function save() {
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

  // Edges relating to the selected node (so they can be relabelled
  // or deleted from the side panel).
  let selectedEdges = $derived(
    selectedId
      ? doc.edges
          .map((e, i) => ({ ...e, idx: i }))
          .filter((e) => e.from === selectedId || e.to === selectedId)
      : [],
  );

  let debounceTimer: ReturnType<typeof setTimeout> | null = null;
  $effect(() => {
    if (!selectedNode) return;
    selectedNode.label;
    selectedNode.shape;
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => void refreshLayout(), 300);
  });

  // Concurrency hardening: cancel pending debounce on unmount.
  onDestroy(() => {
    stopAutosave?.();
    if (debounceTimer) {
      clearTimeout(debounceTimer);
      debounceTimer = null;
    }
  });
</script>

<div class="min-h-screen bg-slate-950 text-slate-200 flex flex-col">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <button onclick={() => goto('dashboard')} class="text-xs text-slate-400 hover:text-cyan-400">
        &larr; Dashboard
      </button>
      <h1 class="text-sm font-bold tracking-widest uppercase text-slate-50">Workflow Diagram</h1>
    </div>
    <div class="flex items-center gap-2">
      {#if lastSavedAt}
        <span class="text-[10px] text-slate-500 tabular-nums" title="Charts save automatically as you edit">
          Saved {lastSavedAt.toLocaleTimeString()}
        </span>
      {/if}
      <details class="relative">
        <summary class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded cursor-pointer list-none">
          + Node
        </summary>
        <div class="absolute right-0 mt-1 z-10 w-56 bg-slate-900 border border-slate-700 rounded shadow-xl">
          {#each SHAPES as s (s.value)}
            <button
              onclick={() => addNode(s.value)}
              class="w-full text-left px-3 py-2 text-xs hover:bg-slate-800"
            >
              {s.label}
            </button>
          {/each}
        </div>
      </details>
      <button
        onclick={startConnect}
        disabled={!selectedId}
        class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-30 px-3 py-1 rounded"
      >
        Connect…
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
          Empty workflow. Click <strong>+ Node</strong> and pick a shape.
        </p>
      {:else}
        <svg
          role="application"
          aria-label="Workflow diagram"
          width={Math.max(layout.width + 40, 600)}
          height={Math.max(layout.height + 60, 400)}
          class="bg-slate-900 border border-slate-800 rounded"
        >
          <defs>
            <marker id="wf-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
              <path d="M 0 0 L 10 5 L 0 10 z" fill="#64748b" />
            </marker>
          </defs>

          <g transform="translate(20,20)">
            <!-- Edges + labels -->
            {#each layout.edges as e, i (i)}
              {@const from = layout.nodes.find((n) => n.id === e.from)}
              {@const to = layout.nodes.find((n) => n.id === e.to)}
              {#if from && to}
                <path
                  d={edgePath(from, to)}
                  stroke="#64748b"
                  stroke-width="1.5"
                  fill="none"
                  marker-end="url(#wf-arrow)"
                />
                {#if e.label}
                  {@const pos = edgeLabelPosition(from, to)}
                  <text x={pos.x} y={pos.y} font-size="10" fill="#cbd5e1">
                    {e.label}
                  </text>
                {/if}
              {/if}
            {/each}

            <!-- Nodes -->
            {#each layout.nodes as n (n.id)}
              <g
                transform={`translate(${n.x},${n.y})`}
                onclick={() => handleNodeClick(n.id)}
                onkeydown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    handleNodeClick(n.id);
                  }
                }}
                role="button"
                tabindex="0"
                aria-label={n.label || n.shape}
                aria-pressed={selectedId === n.id}
                class="cursor-pointer"
              >
                <path
                  d={shapePath(n)}
                  fill={shapeFill(n.shape, selectedId === n.id)}
                  stroke={selectedId === n.id ? '#22d3ee' : '#334155'}
                  stroke-width="1.5"
                />
                {#if n.shape === 'subprocess'}
                  <line x1="8" y1="6" x2="8" y2={n.height - 6} stroke="#a5b4fc" stroke-width="1.5" />
                  <line x1={n.width - 8} y1="6" x2={n.width - 8} y2={n.height - 6} stroke="#a5b4fc" stroke-width="1.5" />
                {/if}
                <text
                  x={n.width / 2}
                  y={n.height / 2 + 4}
                  font-size="12"
                  font-weight="bold"
                  fill={shapeTextFill(n.shape)}
                  text-anchor="middle"
                >
                  {n.label.length > 20 ? n.label.slice(0, 19) + '…' : n.label}
                </text>
              </g>
            {/each}
          </g>
        </svg>
      {/if}
    </main>

    <aside class="w-96 border-l border-slate-800 p-4 bg-slate-900 overflow-y-auto">
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
            <span class="text-xs text-slate-500 uppercase">Shape</span>
            <select
              bind:value={selectedNode.shape}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded"
            >
              {#each SHAPES as s (s.value)}
                <option value={s.value}>{s.label}</option>
              {/each}
            </select>
          </label>

          {#if selectedEdges.length > 0}
            <div class="border-t border-slate-800 pt-3 mt-3">
              <h3 class="text-[10px] text-slate-500 uppercase tracking-widest mb-2">
                Connected edges
              </h3>
              <ul class="space-y-2">
                {#each selectedEdges as e (e.idx)}
                  <li class="bg-slate-950 border border-slate-800 rounded p-2 text-xs">
                    <div class="flex justify-between items-center">
                      <span class="font-mono text-slate-400">
                        {e.from} → {e.to}
                      </span>
                      <button
                        onclick={() => deleteEdge(e.idx)}
                        class="text-slate-500 hover:text-red-400"
                        aria-label="Delete edge" title="Delete edge"
                      >
                        ×
                      </button>
                    </div>
                    <input
                      placeholder="Edge label (e.g. Yes / No)"
                      bind:value={doc.edges[e.idx].label}
                      onblur={refreshLayout}
                      class="w-full mt-1 bg-slate-900 border border-slate-800 p-1 text-xs rounded focus:border-cyan-500 outline-none"
                    />
                  </li>
                {/each}
              </ul>
            </div>
          {/if}

          <p class="text-[10px] text-slate-500 mt-2">ID: {selectedNode.id}</p>
        </div>
      {:else}
        <p class="text-xs text-slate-500">
          Click a node to edit it, or click <strong>+ Node</strong> in the
          header and pick a shape to add one.
        </p>
      {/if}
    </aside>
  </div>
</div>
