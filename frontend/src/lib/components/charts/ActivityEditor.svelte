<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // ActivityEditor renders a UML 2.5 activity diagram with horizontal
  // swimlanes. Each swimlane is an actor / role / team; activities
  // assigned to a swimlane render in its column.
  //
  // Node shapes:
  //   initial     — small filled circle (the "start" of the activity)
  //   final       — bullseye (the "end")
  //   activity    — rounded rectangle
  //   a_decision  — diamond (the "a_" prefix avoids clashing with
  //                 workflow's `decision` shape constant)
  //   fork        — horizontal bar (splits flow into parallel paths)
  //   join        — horizontal bar (merges parallel paths)

  import { onMount, onDestroy } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import { autosave } from '../../autosave.svelte';
  import {
    shapePath,
    shapeFill,
    shapeTextFill,
    edgePath,
    edgeLabelPosition,
    type FlowNode,
  } from './_flow_shapes';

  interface Swimlane {
    id: string;
    name: string;
  }
  interface AcNode {
    id: string;
    label: string;
    shape: string;
    swimlane_id: string;
  }
  interface AcEdge {
    from: string;
    to: string;
    label?: string;
  }
  interface AcDoc {
    swimlanes: Swimlane[];
    nodes: AcNode[];
    edges: AcEdge[];
  }
  interface SwimlaneLayout {
    id: string;
    name: string;
    x: number;
    y: number;
    width: number;
    height: number;
  }
  interface Layout {
    nodes: FlowNode[];
    edges: { from: string; to: string; label?: string }[];
    swimlanes: SwimlaneLayout[];
    width: number;
    height: number;
  }

  const NODE_SHAPES: { value: string; label: string }[] = [
    { value: 'initial', label: 'Initial (●)' },
    { value: 'activity', label: 'Activity' },
    { value: 'a_decision', label: 'Decision (◆)' },
    { value: 'fork', label: 'Fork (━)' },
    { value: 'join', label: 'Join (━)' },
    { value: 'final', label: 'Final (◉)' },
  ];

  let chart = $state<ChartRecord | null>(null);
  let doc = $state<AcDoc>({ swimlanes: [], nodes: [], edges: [] });
  let layout = $state<Layout>({ nodes: [], edges: [], swimlanes: [], width: 0, height: 0 });
  let selectedId = $state<string | null>(null);
  let pendingEdge = $state<string | null>(null);
  let status = $state('');
  let layoutError = $state('');
  let saving = $state(false);

  let stopAutosave: (() => void) | null = null;

  onMount(async () => {
    if (!session.editingId) return;
    chart = await window.go.main.App.GetChart(session.editingId);
    try {
      doc = JSON.parse(chart.data) as AcDoc;
      doc.swimlanes ??= [];
      doc.nodes ??= [];
      doc.edges ??= [];
    } catch {
      doc = { swimlanes: [], nodes: [], edges: [] };
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
      const res = await window.go.main.App.LayoutChart(updated.id);
      layout = res.body as Layout;
    } catch (err: any) {
      layoutError = String(err?.message ?? err);
    }
  }

  // Swimlane CRUD
  function newSwimID(): string {
    return 'sw_' + Math.random().toString(36).slice(2, 6);
  }
  function newNodeID(): string {
    return 'ac_' + Math.random().toString(36).slice(2, 8);
  }

  function addSwimlane() {
    doc.swimlanes.push({ id: newSwimID(), name: 'New role' });
    doc.swimlanes = [...doc.swimlanes];
    void refreshLayout();
  }
  function removeSwimlane(id: string) {
    doc.swimlanes = doc.swimlanes.filter((s) => s.id !== id);
    // Re-home any node previously in that lane.
    for (const n of doc.nodes) {
      if (n.swimlane_id === id) n.swimlane_id = '';
    }
    doc.nodes = [...doc.nodes];
    void refreshLayout();
  }

  // Node CRUD
  function addNode(shape: string) {
    // Default to the selected swimlane, or the first one if none.
    const defaultLane = selectedSwimlane()?.id ?? doc.swimlanes[0]?.id ?? '';
    const id = newNodeID();
    doc.nodes.push({
      id,
      label: defaultLabel(shape),
      shape,
      swimlane_id: defaultLane,
    });
    doc.nodes = [...doc.nodes];
    selectedId = id;
    void refreshLayout();
  }

  function defaultLabel(shape: string): string {
    switch (shape) {
      case 'initial': return '';
      case 'final': return '';
      case 'fork': return 'fork';
      case 'join': return 'join';
      case 'a_decision': return 'Decision?';
      default: return 'Activity';
    }
  }

  function deleteNode() {
    if (!selectedId) return;
    doc.nodes = doc.nodes.filter((n) => n.id !== selectedId);
    doc.edges = doc.edges.filter((e) => e.from !== selectedId && e.to !== selectedId);
    selectedId = null;
    void refreshLayout();
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
  function selectedSwimlane(): Swimlane | undefined {
    return selectedNode
      ? doc.swimlanes.find((s) => s.id === selectedNode!.swimlane_id)
      : undefined;
  }
  let selectedEdges = $derived(
    selectedId
      ? doc.edges
          .map((e, i) => ({ ...e, idx: i }))
          .filter((e) => e.from === selectedId || e.to === selectedId)
      : [],
  );

  let debounceTimer: ReturnType<typeof setTimeout> | null = null;
  $effect(() => {
    for (const s of doc.swimlanes) void s.name;
    if (selectedNode) {
      selectedNode.label;
      selectedNode.shape;
      selectedNode.swimlane_id;
    }
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => void refreshLayout(), 350);
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
      <h1 class="text-sm font-bold tracking-widest uppercase text-slate-50">Activity Diagram</h1>
    </div>
    <div class="flex items-center gap-2">
      <button onclick={addSwimlane} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded">
        + Swimlane
      </button>
      <details class="relative">
        <summary class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded cursor-pointer list-none">
          + Node
        </summary>
        <div class="absolute right-0 mt-1 z-10 w-44 bg-slate-900 border border-slate-700 rounded shadow-xl">
          {#each NODE_SHAPES as s (s.value)}
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
        <p class="text-xs text-red-400 mb-2">Layout error: {layoutError}</p>
      {/if}
      {#if doc.swimlanes.length === 0 && doc.nodes.length === 0}
        <p class="text-sm text-slate-500 text-center mt-12">
          Empty diagram. Start by adding one or more <strong>+ Swimlane</strong>s
          (one per actor or role), then add nodes.
        </p>
      {:else}
        <svg
          role="application"
          aria-label="Activity diagram"
          width={Math.max(layout.width + 40, 700)}
          height={Math.max(layout.height + 60, 400)}
          class="bg-slate-900 border border-slate-800 rounded"
        >
          <defs>
            <marker id="ac-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
              <path d="M 0 0 L 10 5 L 0 10 z" fill="#64748b" />
            </marker>
          </defs>

          <g transform="translate(20,20)">
            <!-- Swimlane bands (drawn behind everything) -->
            {#each layout.swimlanes as s, i (s.id || '__default_' + i)}
              <rect
                x={s.x}
                y={s.y}
                width={s.width}
                height={s.height}
                fill={i % 2 === 0 ? '#0f172a' : '#111827'}
                stroke="#334155"
                stroke-width="1"
              />
              <rect
                x={s.x}
                y={s.y}
                width={s.width}
                height="30"
                fill="#1e293b"
                stroke="#334155"
                stroke-width="1"
              />
              <text
                x={s.x + s.width / 2}
                y={s.y + 19}
                font-size="11"
                font-weight="bold"
                fill="#67e8f9"
                text-anchor="middle"
                style="text-transform: uppercase; letter-spacing: 1px;"
              >
                {s.name}
              </text>
            {/each}

            <!-- Edges -->
            {#each layout.edges as e, i (i)}
              {@const from = layout.nodes.find((n) => n.id === e.from)}
              {@const to = layout.nodes.find((n) => n.id === e.to)}
              {#if from && to}
                <path
                  d={edgePath(from, to)}
                  stroke="#64748b"
                  stroke-width="1.5"
                  fill="none"
                  marker-end="url(#ac-arrow)"
                />
                {#if e.label}
                  {@const pos = edgeLabelPosition(from, to)}
                  <text x={pos.x} y={pos.y} font-size="10" fill="#cbd5e1">
                    [{e.label}]
                  </text>
                {/if}
              {/if}
            {/each}

            <!-- Nodes -->
            {#each layout.nodes as n (n.id)}
              <g
                transform={`translate(${n.x},${n.y})`}
                onclick={() => handleNodeClick(n.id)}
                onkeydown={(e) => e.key === 'Enter' && handleNodeClick(n.id)}
                role="button"
                tabindex="0"
                class="cursor-pointer"
              >
                <path
                  d={shapePath(n)}
                  fill={shapeFill(n.shape, selectedId === n.id)}
                  stroke={selectedId === n.id ? '#22d3ee' : '#334155'}
                  stroke-width={n.shape === 'fork' || n.shape === 'join' ? 0 : 1.5}
                />
                <!-- Inner dot for final node (bullseye) -->
                {#if n.shape === 'final'}
                  <circle
                    cx={n.width / 2}
                    cy={n.height / 2}
                    r={Math.min(n.width, n.height) / 4}
                    fill="#f1f5f9"
                  />
                {/if}
                {#if n.label && n.shape !== 'initial' && n.shape !== 'final'}
                  <text
                    x={n.shape === 'fork' || n.shape === 'join' ? -6 : n.width / 2}
                    y={n.shape === 'fork' || n.shape === 'join' ? n.height + 2 : n.height / 2 + 4}
                    font-size="11"
                    font-weight="bold"
                    fill={shapeTextFill(n.shape)}
                    text-anchor={n.shape === 'fork' || n.shape === 'join' ? 'end' : 'middle'}
                  >
                    {n.label.length > 20 ? n.label.slice(0, 19) + '…' : n.label}
                  </text>
                {/if}
              </g>
            {/each}
          </g>
        </svg>
      {/if}
    </main>

    <aside class="w-96 border-l border-slate-800 p-4 bg-slate-900 overflow-y-auto">
      <!-- Swimlane manager -->
      <h2 class="text-xs font-bold tracking-widest uppercase text-slate-500 mb-3">
        Swimlanes
      </h2>
      <ul class="space-y-2 mb-6">
        {#each doc.swimlanes as s (s.id)}
          <li class="flex gap-2 items-center">
            <input
              bind:value={s.name}
              class="flex-1 bg-slate-950 border border-slate-800 p-2 text-xs rounded focus:border-cyan-500 outline-none"
            />
            <button
              onclick={() => removeSwimlane(s.id)}
              class="text-xs text-slate-500 hover:text-red-400 px-1"
              aria-label="Remove swimlane"
            >
              ×
            </button>
          </li>
        {/each}
      </ul>

      <h2 class="text-xs font-bold tracking-widest uppercase text-slate-500 mb-3">
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
              {#each NODE_SHAPES as s (s.value)}
                <option value={s.value}>{s.label}</option>
              {/each}
            </select>
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Swimlane</span>
            <select
              bind:value={selectedNode.swimlane_id}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded"
            >
              <option value="">(unassigned)</option>
              {#each doc.swimlanes as s (s.id)}
                <option value={s.id}>{s.name}</option>
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
                        aria-label="Delete edge"
                      >
                        ×
                      </button>
                    </div>
                    <input
                      placeholder="Edge condition (e.g. approved)"
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
          Pick a node to edit it, or use <strong>+ Node</strong> to add one
          to the selected swimlane.
        </p>
      {/if}
    </aside>
  </div>
</div>
