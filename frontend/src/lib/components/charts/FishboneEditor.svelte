<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // FishboneEditor edits an Ishikawa diagram: one central Effect plus
  // N Categories, each containing a list of Causes.
  //
  // The right pane is a form. The left pane is the SVG diagram that
  // the backend's LayoutFishbone produced. We don't bind directly to
  // SVG primitives because the categories' bone geometry is computed
  // server-side from the data — every form change triggers a save +
  // re-layout, similar to the WBSEditor pattern.

  import { onMount, onDestroy } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import { showToast } from '../../toast.svelte';
  import { autosave } from '../../autosave.svelte';

  interface FishboneCategory {
    name: string;
    causes: string[];
  }
  interface FishboneDoc {
    effect: string;
    categories: FishboneCategory[];
  }
  interface FishboneNode {
    id: string;
    type: 'effect' | 'category' | 'cause' | 'spine_start';
    label: string;
    x: number;
    y: number;
    width: number;
    height: number;
    side?: 'above' | 'below';
  }
  interface FishboneEdge {
    x1: number;
    y1: number;
    x2: number;
    y2: number;
    kind: 'spine' | 'bone' | 'cause';
  }
  interface FishboneLayout {
    nodes: FishboneNode[];
    edges: FishboneEdge[];
    width: number;
    height: number;
  }

  // The canonical Ishikawa "6 Ms" preset users can apply with one click.
  const SIX_MS = ['People', 'Process', 'Equipment', 'Materials', 'Environment', 'Measurement'];

  let chart = $state<ChartRecord | null>(null);
  let doc = $state<FishboneDoc>({ effect: '', categories: [] });
  let layout = $state<FishboneLayout>({ nodes: [], edges: [], width: 0, height: 0 });
  let status = $state('');
  let saving = $state(false);
  // Set on every successful SaveChart (auto-persist and manual save alike).
  let lastSavedAt = $state<Date | null>(null);
  // Set when the initial GetChart fails: renders a full-screen error with
  // a way back instead of a stuck editor + unhandled promise rejection.
  let loadError = $state('');

  let stopAutosave: (() => void) | null = null;

  onMount(async () => {
    if (!session.editingId) return;
    try {
      chart = await window.go.main.App.GetChart(session.editingId);
    } catch (err: any) {
      loadError = `Could not load this chart: ${err?.message ?? err}`;
      return;
    }
    try {
      doc = JSON.parse(chart.data) as FishboneDoc;
      doc.effect ??= '';
      doc.categories ??= [];
    } catch {
      doc = { effect: '', categories: [] };
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
      lastSavedAt = new Date();
      const res = await window.go.main.App.LayoutChart(updated.id);
      layout = res.body as FishboneLayout;
    } catch (err: any) {
      status = `Layout failed: ${err}`;
    }
  }

  function applySixMs() {
    if (doc.categories.length > 0) {
      if (!confirm('Replace existing categories with the 6 Ms preset?')) return;
    }
    doc.categories = SIX_MS.map((name) => ({ name, causes: [] }));
    void refreshLayout();
  }

  function addCategory() {
    doc.categories.push({ name: 'New category', causes: [] });
    doc.categories = [...doc.categories];
    void refreshLayout();
  }
  function removeCategory(i: number) {
    // Removing a category deletes all its causes — snapshot for undo.
    const before = JSON.parse(JSON.stringify(doc)) as FishboneDoc;
    doc.categories = doc.categories.filter((_, idx) => idx !== i);
    void refreshLayout();
    showToast('Category deleted', {
      type: 'info',
      undo: () => {
        doc = before;
        void refreshLayout();
      },
    });
  }
  function addCause(catIdx: number) {
    doc.categories[catIdx].causes.push('');
    doc.categories = [...doc.categories];
  }
  function removeCause(catIdx: number, cIdx: number) {
    const before = JSON.parse(JSON.stringify(doc)) as FishboneDoc;
    doc.categories[catIdx].causes = doc.categories[catIdx].causes.filter((_, idx) => idx !== cIdx);
    doc.categories = [...doc.categories];
    void refreshLayout();
    showToast('Cause deleted', {
      type: 'info',
      undo: () => {
        doc = before;
        void refreshLayout();
      },
    });
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

  // Debounced re-layout on any field edit.
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;
  $effect(() => {
    doc.effect;
    for (const c of doc.categories) {
      c.name;
      for (const cause of c.causes) {
        // touch each cause so the effect re-runs on edits
        void cause;
      }
    }
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => void refreshLayout(), 400);
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
      <h1 class="text-sm font-bold tracking-widest uppercase text-slate-50">Fishbone Diagram</h1>
    </div>
    <div class="flex items-center gap-2">
      {#if lastSavedAt}
        <span class="text-[10px] text-slate-500 tabular-nums" title="Charts save automatically as you edit">
          Saved {lastSavedAt.toLocaleTimeString()}
        </span>
      {/if}
      <button onclick={applySixMs} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded">
        Apply 6 Ms preset
      </button>
      <button onclick={addCategory} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded">
        + Category
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
    <!-- Diagram -->
    <main class="flex-1 overflow-auto p-6">
      {#if status}
        <p class="text-xs text-cyan-400 mb-2" role="status" aria-live="polite">{status}</p>
      {/if}
      {#if doc.categories.length === 0}
        <p class="text-sm text-slate-500 text-center mt-12">
          Empty diagram. Use the <strong>6 Ms preset</strong> or click <strong>+ Category</strong>.
        </p>
      {:else}
        <svg
          role="application"
          aria-label="Fishbone diagram"
          width={Math.max(layout.width + 40, 700)}
          height={Math.max(layout.height + 40, 400)}
          class="bg-slate-900 border border-slate-800 rounded"
        >
          <g transform="translate(20,20)">
            <!-- edges first so nodes overlay them -->
            {#each layout.edges as e, i}
              <line
                x1={e.x1}
                y1={e.y1}
                x2={e.x2}
                y2={e.y2}
                stroke={e.kind === 'spine' ? '#22d3ee' : e.kind === 'bone' ? '#94a3b8' : '#475569'}
                stroke-width={e.kind === 'spine' ? 2.5 : e.kind === 'bone' ? 2 : 1}
              />
            {/each}
            <!-- nodes -->
            {#each layout.nodes as n (n.id)}
              {#if n.type === 'effect'}
                <rect
                  x={n.x}
                  y={n.y}
                  width={n.width}
                  height={n.height}
                  rx="6"
                  fill="#0e7490"
                  stroke="#22d3ee"
                  stroke-width="2"
                />
                <text
                  x={n.x + n.width / 2}
                  y={n.y + n.height / 2 + 5}
                  font-size="13"
                  fill="#f1f5f9"
                  text-anchor="middle"
                  font-weight="bold"
                >
                  {n.label || 'Effect'}
                </text>
              {:else if n.type === 'category'}
                <text
                  x={n.x + n.width / 2}
                  y={n.y + n.height / 2 + 4}
                  font-size="12"
                  fill="#67e8f9"
                  text-anchor="middle"
                  font-weight="bold"
                >
                  {n.label}
                </text>
              {:else}
                <text
                  x={n.x + n.width}
                  y={n.y + n.height / 2 + 4}
                  font-size="10"
                  fill="#cbd5e1"
                  text-anchor="end"
                >
                  {n.label}
                </text>
              {/if}
            {/each}
          </g>
        </svg>
      {/if}
    </main>

    <!-- Side editor -->
    <aside class="w-96 border-l border-slate-800 p-4 bg-slate-900 overflow-y-auto">
      <h2 class="text-xs font-bold tracking-widest uppercase text-slate-500 mb-4">
        Effect &amp; categories
      </h2>

      <label class="block mb-4">
        <span class="text-xs text-slate-500 uppercase">Effect (the problem)</span>
        <input
          bind:value={doc.effect}
          placeholder="e.g. Production line defects"
          class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        />
      </label>

      {#each doc.categories as cat, i (i)}
        <div class="border border-slate-800 rounded p-3 mb-3 space-y-2">
          <div class="flex items-center gap-2">
            <input
              bind:value={cat.name}
              class="flex-1 bg-slate-950 border border-slate-800 p-2 text-sm rounded focus:border-cyan-500 outline-none"
            />
            <button
              onclick={() => removeCategory(i)}
              class="text-xs text-slate-500 hover:text-red-400 px-2"
              aria-label="Remove category" title="Remove category"
            >
              ×
            </button>
          </div>

          {#each cat.causes as _, ci}
            <div class="flex gap-2">
              <input
                bind:value={cat.causes[ci]}
                placeholder="cause"
                class="flex-1 bg-slate-950 border border-slate-800 p-2 text-xs rounded focus:border-cyan-500 outline-none"
              />
              <button
                onclick={() => removeCause(i, ci)}
                class="text-xs text-slate-500 hover:text-red-400 px-1"
                aria-label="Remove cause" title="Remove cause"
              >
                ×
              </button>
            </div>
          {/each}

          <button
            onclick={() => addCause(i)}
            class="text-xs bg-slate-800 hover:bg-slate-700 px-2 py-1 rounded"
          >
            + Cause
          </button>
        </div>
      {/each}
    </aside>
  </div>
</div>
{/if}
