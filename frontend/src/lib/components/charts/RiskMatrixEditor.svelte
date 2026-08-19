<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // The editor keeps the complete risk/issue/opportunity record in chart data;
  // the backend is authoritative for validation, score bands, and cell order.
  // Keeping scoring out of the component prevents screen, PDF, and report
  // exports from drifting into subtly different risk classifications.
  import { onDestroy, onMount } from 'svelte';
  import { autosave } from '../../autosave.svelte';
  import { goto, session } from '../../session.svelte';
  import { showToast } from '../../toast.svelte';
  import Button from '../Button.svelte';

  interface RiskItem {
    id: string;
    title: string;
    kind: 'risk' | 'issue' | 'opportunity';
    probability: number;
    impact: number;
    owner?: string;
    status?: string;
    mitigation?: string;
    linked_task?: string;
  }
  interface RiskDocument { items: RiskItem[] }
  interface RiskCell {
    probability: number;
    impact: number;
    score: number;
    band: string;
    items: RiskItem[];
  }
  interface RiskLayout {
    cells: RiskCell[];
    validation: { issues?: string[]; error_count: number };
  }

  let chart = $state<ChartRecord | null>(null);
  let doc = $state<RiskDocument>({ items: [] });
  let layout = $state<RiskLayout>({ cells: [], validation: { error_count: 0 } });
  // Selection is index-based because IDs are user-editable. Keying selection
  // by ID would jump to the wrong row while a duplicate-ID validation error is
  // being corrected.
  let selectedIndex = $state<number | null>(null);
  let status = $state('');
  let saving = $state(false);
  let loadError = $state('');
  let lastSavedAt = $state<Date | null>(null);
  let stopAutosave: (() => void) | null = null;
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;

  let selected = $derived(selectedIndex === null ? null : doc.items[selectedIndex]);

  onMount(async () => {
    if (!session.editingId) return;
    try {
      chart = await window.go.main.App.GetChart(session.editingId);
      const parsed = JSON.parse(chart.data) as RiskDocument;
      doc = { items: parsed.items ?? [] };
      await refreshLayout();
      stopAutosave = autosave.register(() => JSON.stringify(doc), () => save());
    } catch (err: any) {
      loadError = `Could not load this risk matrix: ${err?.message ?? err}`;
    }
  });

  async function persistAndLayout(showSavedStatus = false) {
    if (!chart) return;
    const updated = await window.go.main.App.SaveChart({ ...chart, data: JSON.stringify(doc) });
    chart = updated;
    lastSavedAt = new Date();
    const result = await window.go.main.App.LayoutChart(updated.id);
    layout = result.body as RiskLayout;
    if (showSavedStatus) status = `Saved at ${lastSavedAt.toLocaleTimeString()}.`;
  }

  async function refreshLayout() {
    try {
      await persistAndLayout();
      status = '';
    } catch (err: any) {
      status = `Layout failed: ${err?.message ?? err}`;
    }
  }

  async function save() {
    if (!chart) return false;
    saving = true;
    status = '';
    try {
      await persistAndLayout(true);
      return true;
    } catch (err: any) {
      status = `Save failed: ${err?.message ?? err}`;
      return false;
    } finally {
      saving = false;
    }
  }

  function nextID(): string {
    let n = doc.items.length + 1;
    while (doc.items.some((item) => item.id === `R-${n}`)) n++;
    return `R-${n}`;
  }

  function addItem() {
    const item: RiskItem = {
      id: nextID(),
      title: 'New risk',
      kind: 'risk',
      probability: 3,
      impact: 3,
      status: 'open',
    };
    doc.items.push(item);
    doc.items = [...doc.items];
    selectedIndex = doc.items.length - 1;
    void refreshLayout();
  }

  function removeItem(index: number) {
    const before = structuredClone(doc);
    doc.items = doc.items.filter((_, itemIndex) => itemIndex !== index);
    selectedIndex = null;
    void refreshLayout();
    showToast('Risk item deleted', {
      type: 'info',
      undo: () => {
        doc = before;
        void refreshLayout();
      },
    });
  }

  $effect(() => {
    if (!selected) return;
    selected.id;
    selected.title;
    selected.kind;
    selected.probability;
    selected.impact;
    selected.owner;
    selected.status;
    selected.mitigation;
    selected.linked_task;
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => void refreshLayout(), 300);
  });

  onDestroy(() => {
    stopAutosave?.();
    if (debounceTimer) clearTimeout(debounceTimer);
  });

  function cellTone(band: string): string {
    if (band === 'extreme') return 'bg-red-950 border-red-700';
    if (band === 'severe') return 'bg-red-900/70 border-red-600';
    if (band === 'high') return 'bg-amber-900/70 border-amber-600';
    if (band === 'medium') return 'bg-cyan-900/70 border-cyan-600';
    return 'bg-slate-800 border-slate-700';
  }
</script>

{#if loadError}
  <div class="min-h-screen bg-slate-950 text-slate-200 flex items-center justify-center">
    <div class="text-center space-y-4 px-6">
      <p class="text-sm text-red-400" role="alert">{loadError}</p>
      <Button variant="primary" size="md" onclick={() => goto('dashboard')}>Back to dashboard</Button>
    </div>
  </div>
{:else}
  <div class="min-h-screen bg-slate-950 text-slate-200">
    <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
      <div class="flex items-center gap-4">
        <Button variant="nav" onclick={() => goto('dashboard')}>&larr; Dashboard</Button>
        <h1 class="text-sm font-bold tracking-widest uppercase text-slate-50">Risk Matrix</h1>
      </div>
      <div class="flex items-center gap-2">
        {#if lastSavedAt}<span class="text-[10px] text-slate-500 tabular-nums">Saved {lastSavedAt.toLocaleTimeString()}</span>{/if}
        <button onclick={addItem} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded">+ Item</button>
        <button onclick={save} disabled={saving} class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-3 py-1 rounded">{saving ? 'Saving...' : 'Save'}</button>
      </div>
    </header>

    <div class="flex">
      <aside class="w-[26rem] border-r border-slate-800 p-4 overflow-y-auto h-[calc(100vh-50px)]">
        <h2 class="text-xs font-bold tracking-widest uppercase text-slate-500 mb-3">Register items</h2>
        {#if doc.items.length === 0}
          <p class="text-xs text-slate-500">No items yet. Add a risk, active issue, or opportunity.</p>
        {:else}
          <ul class="space-y-2">
            {#each doc.items as item, index}
              <li>
                <button onclick={() => (selectedIndex = index)} class="w-full text-left p-2 rounded border {selectedIndex === index ? 'border-cyan-500 bg-slate-900' : 'border-slate-800 hover:bg-slate-900'}">
                  <div class="flex justify-between gap-2"><span class="font-bold text-sm">{item.id}</span><span class="text-[10px] uppercase text-slate-500">{item.kind}</span></div>
                  <div class="text-xs text-slate-300 truncate">{item.title}</div>
                  <div class="text-[10px] text-slate-500">P{item.probability} × I{item.impact} = {item.probability * item.impact}</div>
                </button>
              </li>
            {/each}
          </ul>
        {/if}

        {#if selected}
          <div class="mt-6 border-t border-slate-800 pt-4 space-y-3 text-sm">
            <div class="grid grid-cols-2 gap-2">
              <label><span class="text-xs text-slate-500 uppercase">ID</span><input bind:value={selected.id} class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none" /></label>
              <label><span class="text-xs text-slate-500 uppercase">Type</span><select bind:value={selected.kind} class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded"><option value="risk">Risk</option><option value="issue">Issue</option><option value="opportunity">Opportunity</option></select></label>
            </div>
            <label class="block"><span class="text-xs text-slate-500 uppercase">Title</span><input bind:value={selected.title} class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none" /></label>
            <div class="grid grid-cols-2 gap-2">
              <label><span class="text-xs text-slate-500 uppercase">Probability</span><select bind:value={selected.probability} class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded">{#each [1,2,3,4,5] as value}<option value={value}>{value}</option>{/each}</select></label>
              <label><span class="text-xs text-slate-500 uppercase">Impact</span><select bind:value={selected.impact} class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded">{#each [1,2,3,4,5] as value}<option value={value}>{value}</option>{/each}</select></label>
            </div>
            <div class="grid grid-cols-2 gap-2">
              <label><span class="text-xs text-slate-500 uppercase">Owner</span><input bind:value={selected.owner} class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded" /></label>
              <label><span class="text-xs text-slate-500 uppercase">Status</span><input bind:value={selected.status} class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded" /></label>
            </div>
            <label class="block"><span class="text-xs text-slate-500 uppercase">Linked task</span><input bind:value={selected.linked_task} class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded" /></label>
            <label class="block"><span class="text-xs text-slate-500 uppercase">Response / mitigation</span><textarea bind:value={selected.mitigation} rows="3" class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded"></textarea></label>
            <button onclick={() => removeItem(selectedIndex!)} class="text-xs text-red-400 hover:text-red-300">Remove item</button>
          </div>
        {/if}
      </aside>

      <main class="flex-1 p-6 overflow-auto">
        {#if status}<p class="text-xs text-cyan-400 mb-2" role="status" aria-live="polite">{status}</p>{/if}
        {#if layout.validation.error_count > 0}
          <div class="mb-4 rounded border border-amber-700 bg-amber-950/40 p-3" role="alert">
            <p class="text-xs font-bold text-amber-300">Correct {layout.validation.error_count} register issue(s)</p>
            <ul class="mt-1 list-disc pl-4 text-xs text-amber-200">{#each layout.validation.issues ?? [] as issue}<li>{issue}</li>{/each}</ul>
          </div>
        {/if}
        <div class="max-w-4xl">
          <div class="mb-2 text-center text-xs font-bold uppercase tracking-widest text-cyan-300">Impact →</div>
          <div class="flex">
            <div class="w-8 flex items-center justify-center text-xs font-bold uppercase tracking-widest text-cyan-300 [writing-mode:vertical-rl] rotate-180">Probability →</div>
            <div class="flex-1 grid grid-cols-5 gap-1" role="grid" aria-label="Five by five probability and impact risk matrix">
              {#each layout.cells as cell (`${cell.probability}-${cell.impact}`)}
                <div role="gridcell" aria-label={`Probability ${cell.probability}, impact ${cell.impact}, score ${cell.score}, ${cell.items.length} items`} class="relative min-h-28 rounded border p-2 {cellTone(cell.band)}">
                  <span class="absolute right-2 top-1 text-[10px] text-white/60">{cell.score}</span>
                  <div class="mt-4 flex flex-wrap gap-1">
                    {#each cell.items as item (item.id)}
                      <button onclick={() => (selectedIndex = doc.items.findIndex((candidate) => candidate.id === item.id))} title={item.title} class="rounded bg-slate-950/80 px-2 py-1 text-[10px] font-bold text-white ring-1 ring-white/20 hover:ring-cyan-300">{item.kind === 'opportunity' ? '+' : item.kind === 'issue' ? '!' : ''}{item.id}</button>
                    {/each}
                  </div>
                </div>
              {/each}
            </div>
          </div>
          <div class="ml-8 mt-2 grid grid-cols-5 text-center text-xs text-slate-500">{#each [1,2,3,4,5] as value}<span>{value}</span>{/each}</div>
        </div>
      </main>
    </div>
  </div>
{/if}
