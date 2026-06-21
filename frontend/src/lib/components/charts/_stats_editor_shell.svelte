<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later

Shared editor shell for every Stats-family chart. Handles:

  - Loading the ChartRecord from the backend on mount.
  - Saving (header button + debounced auto-save when the data changes).
  - Calling LayoutChart and feeding the result to StatsChart.
  - Status / error display.

Each kind-specific editor (LineEditor, BarEditor, ...) supplies:

  - `headingLabel`     -- the chart-type name shown in the header
  - `dataEditor`       -- a snippet that renders the kind-specific
                          form fields (series management, items, etc.)
  - parses/serialises its own `doc` shape via the bind:doc prop

The shell is generic over the doc shape because each kind's storage
JSON differs; the doc is treated as an opaque object that gets
JSON.stringified back to db.charts.data.
-->
<script lang="ts" generics="TDoc">
  import { onMount, onDestroy } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import { autosave } from '../../autosave.svelte';
  import StatsChart from './StatsChart.svelte';
  import type { Snippet } from 'svelte';
  import type { StatsLayout } from './_stats_types';

  let {
    headingLabel,
    initialDoc,
    doc = $bindable<TDoc>(),
    dataEditor,
  }: {
    headingLabel: string;
    initialDoc: TDoc;
    doc?: TDoc;
    dataEditor: Snippet;
  } = $props();

  let chart = $state<ChartRecord | null>(null);
  let layout = $state<StatsLayout | null>(null);
  let saving = $state(false);
  let status = $state('');
  let layoutError = $state('');

  function initialDocValue() {
    return initialDoc;
  }

  // Initialise doc from the prop's current initial value. Editors pass
  // a structurally-complete blank document so the form fields all bind
  // to defined slots.
  if (doc === undefined) {
    doc = initialDocValue();
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
    if (!session.editingId) return;
    chart = await window.go.main.App.GetChart(session.editingId);
    try {
      const parsed = JSON.parse(chart.data) as TDoc;
      doc = { ...initialDocValue(), ...parsed };
    } catch {
      doc = initialDocValue();
    }
    await refreshLayout();
    // Register after load so the baseline snapshot is the saved doc.
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
      layout = res.body as StatsLayout;
    } catch (err: any) {
      layoutError = String(err?.message ?? err);
    }
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

  // Debounced refresh on any doc change. Editors call `bumpDoc()`
  // through the bound doc; we watch the serialised form so additions
  // and removals at arbitrary depths trigger reactivity.
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;
  let serialised = $derived(JSON.stringify(doc));
  $effect(() => {
    serialised;
    if (!chart) return;
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => void refreshLayout(), 350);
  });

  // Concurrency hardening: cancel pending debounce on unmount.
  // (AGENT.md §6 — every editor with a setTimeout MUST clean up.)
  onDestroy(() => {
    window.removeEventListener('keydown', handleKeyDown);
    stopAutosave?.();
    if (debounceTimer) {
      clearTimeout(debounceTimer);
      debounceTimer = null;
    }
  });
</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <button onclick={() => goto('dashboard')} class="text-xs text-slate-400 hover:text-cyan-400">
        &larr; Dashboard
      </button>
      <h1 class="text-sm font-bold tracking-widest uppercase text-slate-50">{headingLabel}</h1>
    </div>
    <button
      onclick={save}
      disabled={saving}
      class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-3 py-1 rounded"
    >
      {saving ? 'Saving...' : 'Save'}
    </button>
  </header>

  <main class="p-6 space-y-6">
    {#if status}
      <p class="text-xs text-cyan-400">{status}</p>
    {/if}
    {#if layoutError}
      <p class="text-xs text-red-400">Layout error: {layoutError}</p>
    {/if}

    <!-- Chart preview -->
    <section class="bg-slate-900 border border-slate-800 rounded-lg p-4">
      {#if layout}
        <StatsChart {layout} height={380} />
      {:else}
        <p class="text-sm text-slate-500 text-center py-12">Loading chart...</p>
      {/if}
    </section>

    <!-- Kind-specific data editor -->
    <section class="bg-slate-900 border border-slate-800 rounded-lg p-4">
      {@render dataEditor()}
    </section>
  </main>
</div>
