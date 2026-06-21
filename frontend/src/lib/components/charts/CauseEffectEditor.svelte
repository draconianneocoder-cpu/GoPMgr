<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // CauseEffectEditor is the generic cause-tree editor. Unlike Fishbone
  // (which has a fixed effect + categories + causes structure), this
  // chart represents an arbitrary tree of nested causes — useful for
  // 5-Whys drill-downs and other branching analyses.
  //
  // Mirrors the WBSEditor UX: visual tree on the left, side panel
  // on the right, "add child / sibling / delete" actions in the
  // header. The backend reuses the WBS subtree-width algorithm with
  // axes swapped so the diagram grows leftward from the effect.

  import { onMount, onDestroy, untrack } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import { autosave } from '../../autosave.svelte';

  interface CauseNode {
    id: string;
    label: string;
    note?: string;
    children?: CauseNode[];
  }
  interface CausalDoc {
    effect: string;
    root: CauseNode | null;
  }
  interface NodeLayout {
    id: string;
    title: string;
    note?: string;
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
  let doc = $state<CausalDoc>({
    effect: '',
    root: { id: 'r', label: 'Root cause', children: [] },
  });
  let layout = $state<Layout>({ nodes: [], edges: [], width: 0, height: 0 });
  let selectedId = $state<string | null>('r');
  let status = $state('');
  let saving = $state(false);

  let stopAutosave: (() => void) | null = null;

  onMount(async () => {
    if (!session.editingId) return;
    chart = await window.go.main.App.GetChart(session.editingId);
    try {
      const parsed = JSON.parse(chart.data) as CausalDoc;
      doc = {
        effect: parsed.effect ?? '',
        root: parsed.root ?? { id: 'r', label: 'Root cause', children: [] },
      };
    } catch {
      doc = { effect: '', root: { id: 'r', label: 'Root cause', children: [] } };
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
    try {
      const updated = await window.go.main.App.SaveChart({
        ...chart,
        data: JSON.stringify(doc),
      });
      chart = updated;
      const res = await window.go.main.App.LayoutChart(updated.id);
      layout = res.body as Layout;
    } catch (err: any) {
      status = `Layout failed: ${err}`;
    }
  }

  function findById(n: CauseNode | null, id: string): CauseNode | null {
    if (!n) return null;
    if (n.id === id) return n;
    for (const c of n.children ?? []) {
      const f = findById(c, id);
      if (f) return f;
    }
    return null;
  }
  function findParent(n: CauseNode | null, id: string): CauseNode | null {
    if (!n) return null;
    for (const c of n.children ?? []) {
      if (c.id === id) return n;
      const f = findParent(c, id);
      if (f) return f;
    }
    return null;
  }
  function newID(): string {
    return 'c_' + Math.random().toString(36).slice(2, 8);
  }

  function addChild() {
    const id = selectedId ?? doc.root?.id ?? 'r';
    if (!doc.root) return;
    const parent = findById(doc.root, id);
    if (!parent) return;
    parent.children = parent.children ?? [];
    const c: CauseNode = { id: newID(), label: 'Why?' };
    parent.children.push(c);
    selectedId = c.id;
    void refreshLayout();
  }
  function addSibling() {
    if (!doc.root || !selectedId || selectedId === doc.root.id) return;
    const parent = findParent(doc.root, selectedId);
    if (!parent) return;
    parent.children = parent.children ?? [];
    const c: CauseNode = { id: newID(), label: 'Why?' };
    parent.children.push(c);
    selectedId = c.id;
    void refreshLayout();
  }
  function deleteNode() {
    if (!doc.root || !selectedId || selectedId === doc.root.id) return;
    const parent = findParent(doc.root, selectedId);
    if (!parent || !parent.children) return;
    parent.children = parent.children.filter((c) => c.id !== selectedId);
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
      status = `Saved at ${new Date().toLocaleTimeString()}.`;
    } catch (err: any) {
      status = `Save failed: ${err}`;
    } finally {
      saving = false;
    }
  }

  let selectedNode = $derived(
    selectedId && doc.root ? findById(doc.root, selectedId) : null,
  );

  let debounceTimer: ReturnType<typeof setTimeout> | null = null;
  $effect(() => {
    doc.effect;
    if (!selectedNode) return;
    selectedNode.label;
    selectedNode.note;
    untrack(() => {
      if (debounceTimer) clearTimeout(debounceTimer);
      debounceTimer = setTimeout(() => void refreshLayout(), 350);
    });
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
      <h1 class="text-sm font-bold tracking-widest uppercase text-slate-50">
        Cause-and-Effect Diagram
      </h1>
    </div>
    <div class="flex items-center gap-2">
      <button onclick={addChild} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded">
        + Child cause
      </button>
      <button onclick={addSibling} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded">
        + Sibling
      </button>
      <button
        onclick={deleteNode}
        disabled={!selectedId || selectedId === doc.root?.id}
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
    <main class="flex-1 overflow-auto p-6">
      {#if status}
        <p class="text-xs text-cyan-400 mb-2">{status}</p>
      {/if}
      <label class="block mb-4 max-w-md">
        <span class="text-xs text-slate-500 uppercase">Effect (final outcome)</span>
        <input
          bind:value={doc.effect}
          placeholder="e.g. Customer churn"
          class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        />
      </label>
      <svg
        role="application"
        aria-label="Cause-and-Effect diagram"
        width={Math.max(layout.width + 40, 700)}
        height={Math.max(layout.height + 40, 300)}
        class="bg-slate-900 border border-slate-800 rounded"
      >
        <g transform="translate(20,20)">
          <!-- edges -->
          {#each layout.edges as e (e.from + '-' + e.to)}
            {@const from = layout.nodes.find((n) => n.id === e.from)}
            {@const to = layout.nodes.find((n) => n.id === e.to)}
            {#if from && to}
              <path
                d={`M ${from.x} ${from.y + from.height / 2} L ${(from.x + to.x + to.width) / 2} ${from.y + from.height / 2} L ${(from.x + to.x + to.width) / 2} ${to.y + to.height / 2} L ${to.x + to.width} ${to.y + to.height / 2}`}
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
                fill={n.id === doc.root?.id ? '#0e7490' : selectedId === n.id ? '#155e75' : '#1e293b'}
                stroke={selectedId === n.id ? '#22d3ee' : '#334155'}
                stroke-width="1.5"
              />
              <text x="8" y="22" font-size="11" fill="#f1f5f9" font-weight="bold">
                {n.title.length > 22 ? n.title.slice(0, 21) + '…' : n.title}
              </text>
              {#if n.note}
                <text x="8" y="38" font-size="9" fill="#94a3b8">
                  {n.note.length > 28 ? n.note.slice(0, 27) + '…' : n.note}
                </text>
              {/if}
            </g>
          {/each}
        </g>
      </svg>
    </main>

    <aside class="w-80 border-l border-slate-800 p-4 bg-slate-900">
      <h2 class="text-xs font-bold tracking-widest uppercase text-slate-500 mb-4">
        Selected cause
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
            <span class="text-xs text-slate-500 uppercase">Note</span>
            <textarea
              bind:value={selectedNode.note}
              rows="4"
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            ></textarea>
          </label>
          <p class="text-[10px] text-slate-500">ID: {selectedNode.id}</p>
        </div>
      {:else}
        <p class="text-xs text-slate-500">Click a cause in the diagram to edit it.</p>
      {/if}
    </aside>
  </div>
</div>
