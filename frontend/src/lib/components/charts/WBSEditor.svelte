<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount, onDestroy, untrack } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import { showToast } from '../../toast.svelte';
  import { autosave } from '../../autosave.svelte';
  import Button from '../Button.svelte';

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
  // Set on every successful SaveChart (auto-persist and manual save alike).
  let lastSavedAt = $state<Date | null>(null);
  // Set when the initial GetChart fails: renders a full-screen error with
  // a way back instead of a stuck editor + unhandled promise rejection.
  let loadError = $state('');
  let status = $state('');

  // ---------- load ----------
  let stopAutosave: (() => void) | null = null;
  let active = true;
  let pendingWrite: Promise<boolean> | null = null;
  let refreshing = false;
  let refreshAgain = false;

  function snapshot(): string {
    return JSON.stringify(doc);
  }

  onMount(async () => {
    if (!session.editingId) return;
    try {
      const loaded = await window.go.main.App.GetChart(session.editingId);
      if (!active) return;
      chart = loaded;
    } catch (err: any) {
      if (active) loadError = `Could not load this chart: ${err?.message ?? err}`;
      return;
    }
    let initialSnapshot: string | undefined;
    try {
      doc = JSON.parse(chart.data) as WBSDoc;
    } catch {
      initialSnapshot = chart.data;
      doc = { root: { id: 'r', title: chart.title, children: [] } };
    }
    stopAutosave = autosave.register(snapshot, persist, true, initialSnapshot);
    await refreshLayout();
  });

  async function refreshLayout() {
    if (!active || !chart) return;
    refreshAgain = true;
    if (refreshing) return;
    refreshing = true;
    try {
      while (refreshAgain && active) {
        // The coordinator awaits this exact promise first. Its acknowledgement
        // releases the gate before this later waiter submits the newest draft.
        if (pendingWrite) await pendingWrite;
        if (!active) return;
        refreshAgain = false;
        await save();
      }
    } finally {
      refreshing = false;
    }
  }

  // Keep the promise identity: an async wrapper would acknowledge later than
  // the preview waiter and could cause the next refresh to be refused as busy.
  function persist(): Promise<boolean> {
    pendingWrite = writeChart();
    return pendingWrite;
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

  function normalizeEffort(value: number | undefined): number | undefined {
    if (value === undefined || value === null || Number.isNaN(value)) return undefined;
    return Math.max(0, value);
  }

  function selectNode(id: string) {
    selectedId = id;
    const node = findById(doc.root, id);
    if (node) node.effort = normalizeEffort(node.effort);
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
    // Removing a WBS node deletes its whole subtree — snapshot for undo.
    const before = JSON.parse(JSON.stringify(doc)) as typeof doc;
    parent.children = parent.children!.filter((c) => c.id !== selectedId);
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

  async function save(): Promise<boolean> {
    return active && await autosave.saveRegistered(snapshot);
  }

  async function writeChart(): Promise<boolean> {
    if (!active || !chart) return false;
    const submission = { ...chart, data: snapshot() };
    saving = true;
    status = '';
    try {
      const updated = await window.go.main.App.SaveChart(submission);
      if (!active) return true;
      chart = updated;
      lastSavedAt = new Date();
      status = `Saved. Version updated at ${new Date().toLocaleTimeString()}.`;
      try {
        const res = await window.go.main.App.LayoutChart(updated.id);
        if (active) layout = (res.body as unknown) as Layout;
      } catch (err: unknown) {
        if (active) status = `Saved, but layout failed: ${err}`;
      }
      return true;
    } catch (err: unknown) {
      if (active) status = `Save failed: ${err}`;
      return false;
    } finally {
      if (active) saving = false;
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
    active = false;
    refreshAgain = false;
    stopAutosave?.();
    if (debounceTimer) {
      clearTimeout(debounceTimer);
      debounceTimer = null;
    }
  });
</script>

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
      <h1 class="text-sm font-bold tracking-widest uppercase text-slate-50">
        {chart?.title ?? 'Work Breakdown Structure'}
      </h1>
    </div>
    <div class="flex items-center gap-2">
      {#if lastSavedAt}
        <span class="text-[10px] text-slate-500 tabular-nums" title="Charts save automatically as you edit">
          Saved {lastSavedAt.toLocaleTimeString()}
        </span>
      {/if}
      <Button variant="secondary" size="sm" onclick={addChild}>+ Child</Button>
      <Button variant="secondary" size="sm" onclick={addSibling}>+ Sibling</Button>
      <Button
        variant="canvas-danger"
        size="sm"
        onclick={deleteNode}
        disabled={!selectedId || selectedId === doc.root.id}
      >
        Delete
      </Button>
      <Button variant="primary" size="sm" onclick={save} disabled={saving}>
        {saving ? 'Saving...' : 'Save'}
      </Button>
    </div>
  </header>

  <div class="flex-1 flex">
    <!-- Canvas -->
    <main class="flex-1 overflow-auto p-6">
      {#if status}
        <p class="text-xs text-cyan-400 mb-2" role="status" aria-live="polite">{status}</p>
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
            onclick={() => selectNode(n.id)}
            onkeydown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                selectNode(n.id);
              }
            }}
            role="button"
            tabindex="0"
            aria-label={`${n.number} ${n.title}`}
            aria-pressed={selectedId === n.id}
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
              min="0"
              step="0.25"
              bind:value={selectedNode.effort}
              oninput={() => (selectedNode!.effort = normalizeEffort(selectedNode!.effort))}
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
{/if}
