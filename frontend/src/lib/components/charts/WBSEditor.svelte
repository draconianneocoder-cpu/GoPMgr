<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount, onDestroy, untrack } from 'svelte';
  import { session, goto } from '../../session.svelte';

  // ---------- types & state ----------
  interface WBSNode {
    id: string;
    number?: string;
    title: string;
    note?: string;
    owner?: string;
    effort?: number;
    children?: WBSNode[];
  }
  interface WBSDoc {
    root: WBSNode;
  }
  interface NodeLayout {
    id: string;
    number: string;
    title: string;
    note?: string;
    owner?: string;
    depth: number;
    x: number;
    y: number;
    width: number;
    height: number;
  }
  interface EdgeLayout {
    from: string;
    to: string;
  }
  interface Layout {
    nodes: NodeLayout[];
    edges: EdgeLayout[];
    width: number;
    height: number;
  }

  let chart = $state<ChartRecord | null>(null);
  let doc = $state<WBSDoc>({ root: { id: 'r', title: 'Project', children: [] } });
  let layout = $state<Layout>({ nodes: [], edges: [], width: 0, height: 0 });
  let selectedId = $state<string | null>(null);
  let saving = $state(false);
  let status = $state('');

  // ---------- load ----------
  onMount(async () => {
    if (!session.editingId) return;
    chart = await window.go.main.App.GetChart(session.editingId);
    try {
      doc = JSON.parse(chart.data) as WBSDoc;
    } catch {
      doc = { root: { id: 'r', title: chart.title, children: [] } };
    }
    await refreshLayout();
  });

  async function refreshLayout() {
    if (!chart) return;
    try {
      // Persist the edits so the backend's Renumber + LayoutWBS sees them.
      const updated = await window.go.main.App.SaveChart({
        ...chart,
        data: JSON.stringify(doc),
      });
      chart = updated;
      const res = await window.go.main.App.LayoutChart(updated.id);
      layout = (res.body as unknown) as Layout;
    } catch (err: any) {
      status = `Layout failed: ${err}`;
    }
  }

  // ---------- tree edit operations ----------
  function findById(node: WBSNode, id: string): WBSNode | null {
    if (node.id === id) return node;
    for (const c of node.children ?? []) {
      const f = findById(c, id);
      if (f) return f;
    }
    return null;
  }

  function findParent(node: WBSNode, id: string): WBSNode | null {
    for (const c of node.children ?? []) {
      if (c.id === id) return node;
      const f = findParent(c, id);
      if (f) return f;
    }
    return null;
  }

  function newID(): string {
    return 'n_' + Math.random().toString(36).slice(2, 9);
  }

  function addChild() {
    const id = selectedId ?? doc.root.id;
    const parent = findById(doc.root, id);
    if (!parent) return;
    parent.children = parent.children ?? [];
    const child: WBSNode = { id: newID(), title: 'New work package' };
    parent.children.push(child);
    selectedId = child.id;
    void refreshLayout();
  }

  function addSibling() {
    if (!selectedId || selectedId === doc.root.id) return;
    const parent = findParent(doc.root, selectedId);
    if (!parent) return;
    const sib: WBSNode = { id: newID(), title: 'New work package' };
    parent.children!.push(sib);
    selectedId = sib.id;
    void refreshLayout();
  }

  function deleteNode() {
    if (!selectedId || selectedId === doc.root.id) return;
    const parent = findParent(doc.root, selectedId);
    if (!parent) return;
    parent.children = parent.children!.filter((c) => c.id !== selectedId);
    selectedId = null;
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
      status = `Saved. Version updated at ${new Date().toLocaleTimeString()}.`;
    } catch (err: any) {
      status = `Save failed: ${err}`;
    } finally {
      saving = false;
    }
  }

  // ---------- derived: editor panel binds ----------
  let selectedNode = $derived(
    selectedId ? findById(doc.root, selectedId) : null,
  );

  // When the user edits the selected node's fields, re-layout
  // (debounce-via-untrack so we don't loop on every keystroke).
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;
  $effect(() => {
    if (!selectedNode) return;
    // Touch reactive fields so the effect re-runs on any edit.
    selectedNode.title;
    selectedNode.note;
    selectedNode.owner;
    selectedNode.effort;
    untrack(() => {
      if (debounceTimer) clearTimeout(debounceTimer);
      debounceTimer = setTimeout(() => void refreshLayout(), 300);
    });
  });

  // Concurrency hardening: cancel pending debounce on unmount.
  onDestroy(() => {
    if (debounceTimer) {
      clearTimeout(debounceTimer);
      debounceTimer = null;
    }
  });
</script>

<div class="min-h-screen bg-slate-950 text-slate-200 flex flex-col">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <button
        onclick={() => goto('dashboard')}
        class="text-xs text-slate-400 hover:text-cyan-400"
      >
        &larr; Dashboard
      </button>
      <h1 class="text-sm font-bold tracking-widest uppercase text-white">
        {chart?.title ?? 'Work Breakdown Structure'}
      </h1>
    </div>
    <div class="flex items-center gap-2">
      <button onclick={addChild} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded">
        + Child
      </button>
      <button onclick={addSibling} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded">
        + Sibling
      </button>
      <button
        onclick={deleteNode}
        disabled={!selectedId || selectedId === doc.root.id}
        class="text-xs bg-slate-800 hover:bg-red-900 disabled:opacity-30 px-3 py-1 rounded"
      >
        Delete
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
    <!-- Canvas -->
    <main class="flex-1 overflow-auto p-6">
      {#if status}
        <p class="text-xs text-cyan-400 mb-2">{status}</p>
      {/if}
      <svg
        role="application"
        aria-label="Work Breakdown Structure editor"
        width={Math.max(layout.width + 40, 600)}
        height={Math.max(layout.height + 80, 400)}
        class="bg-slate-900 border border-slate-800 rounded"
      >
        <!-- edges -->
        {#each layout.edges as e (e.from + '-' + e.to)}
          {@const from = layout.nodes.find((n) => n.id === e.from)}
          {@const to = layout.nodes.find((n) => n.id === e.to)}
          {#if from && to}
            <path
              d={`M ${from.x + from.width / 2} ${from.y + from.height} L ${from.x + from.width / 2} ${from.y + from.height + 12} L ${to.x + to.width / 2} ${from.y + from.height + 12} L ${to.x + to.width / 2} ${to.y}`}
              stroke="#475569"
              stroke-width="1.5"
              fill="none"
            />
          {/if}
        {/each}

        <!-- nodes -->
        {#each layout.nodes as n (n.id)}
          <g
            transform={`translate(${n.x},${n.y})`}
            onclick={() => (selectedId = n.id)}
            onkeydown={(e) => e.key === 'Enter' && (selectedId = n.id)}
            role="button"
            tabindex="0"
            class="cursor-pointer"
          >
            <rect
              width={n.width}
              height={n.height}
              rx="6"
              fill={selectedId === n.id ? '#0e7490' : '#1e293b'}
              stroke={selectedId === n.id ? '#22d3ee' : '#334155'}
              stroke-width="1.5"
            />
            <text x="8" y="18" font-size="10" fill="#67e8f9" font-weight="bold">
              {n.number}
            </text>
            <text x="8" y="34" font-size="12" fill="#f1f5f9">
              {n.title.length > 24 ? n.title.slice(0, 23) + '…' : n.title}
            </text>
            {#if n.owner}
              <text x="8" y="49" font-size="9" fill="#94a3b8">
                {n.owner}
              </text>
            {/if}
          </g>
        {/each}
      </svg>
    </main>

    <!-- Side panel -->
    <aside class="w-80 border-l border-slate-800 p-4 bg-slate-900">
      <h2 class="text-xs font-bold tracking-widest uppercase text-slate-500 mb-4">
        Selected node
      </h2>
      {#if selectedNode}
        <div class="space-y-3 text-sm">
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Title</span>
            <input
              bind:value={selectedNode.title}
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
            <span class="text-xs text-slate-500 uppercase">Effort (units)</span>
            <input
              type="number"
              bind:value={selectedNode.effort}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Note</span>
            <textarea
              bind:value={selectedNode.note}
              rows="4"
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            ></textarea>
          </label>
          <p class="text-[10px] text-slate-500">
            ID: {selectedNode.id} · Number: {selectedNode.number ?? '—'}
          </p>
        </div>
      {:else}
        <p class="text-xs text-slate-500">
          Click a node in the diagram to edit its fields.
        </p>
      {/if}
    </aside>
  </div>
</div>
