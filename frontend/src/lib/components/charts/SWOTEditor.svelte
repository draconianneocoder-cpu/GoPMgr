<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // SWOTEditor renders the canonical 2×2 SWOT grid as four cards:
  //
  //   Strengths       | Weaknesses
  //   Opportunities   | Threats
  //
  // The top row is internal factors, bottom row external. Left column
  // is positive, right column negative. The colour palette is set
  // server-side via the `tone` field on each quadrant so the GUI and
  // any future PDF export agree on the styling.

  import { onMount, onDestroy } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import { showToast } from '../../toast.svelte';
  import { autosave } from '../../autosave.svelte';

  interface SWOTDoc {
    title?: string;
    strengths: string[];
    weaknesses: string[];
    opportunities: string[];
    threats: string[];
  }
  interface SWOTQuadrant {
    key: string;
    title: string;
    items: string[];
    row: number;
    col: number;
    tone: string;
  }
  interface SWOTLayout {
    title?: string;
    quadrants: SWOTQuadrant[];
  }

  let chart = $state<ChartRecord | null>(null);
  let doc = $state<SWOTDoc>({
    title: '',
    strengths: [],
    weaknesses: [],
    opportunities: [],
    threats: [],
  });
  let layout = $state<SWOTLayout>({ quadrants: [] });
  let status = $state('');
  let saving = $state(false);
  // Set on every successful SaveChart (auto-persist and manual save alike).
  let lastSavedAt = $state<Date | null>(null);

  let stopAutosave: (() => void) | null = null;

  onMount(async () => {
    if (!session.editingId) return;
    chart = await window.go.main.App.GetChart(session.editingId);
    try {
      const parsed = JSON.parse(chart.data) as SWOTDoc;
      doc = {
        title: parsed.title ?? '',
        strengths: parsed.strengths ?? [],
        weaknesses: parsed.weaknesses ?? [],
        opportunities: parsed.opportunities ?? [],
        threats: parsed.threats ?? [],
      };
    } catch {
      doc = { strengths: [], weaknesses: [], opportunities: [], threats: [] };
    }
    await refreshLayout();
    // Register for timed auto-save now the saved doc is loaded.
    stopAutosave = autosave.register(
      () => JSON.stringify(doc),
      () => save(),
    );
  });

  onDestroy(() => {
    stopAutosave?.();
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
      layout = res.body as SWOTLayout;
    } catch (err: any) {
      status = `Layout failed: ${err}`;
    }
  }

  // Each quadrant's items live in a specific slice on `doc`. Look it
  // up by quadrant key so add/remove operations route correctly.
  function listFor(key: string): string[] {
    switch (key) {
      case 'S': return doc.strengths;
      case 'W': return doc.weaknesses;
      case 'O': return doc.opportunities;
      case 'T': return doc.threats;
      default:  return [];
    }
  }
  function setList(key: string, items: string[]) {
    switch (key) {
      case 'S': doc.strengths = items; break;
      case 'W': doc.weaknesses = items; break;
      case 'O': doc.opportunities = items; break;
      case 'T': doc.threats = items; break;
    }
  }

  function addItem(key: string) {
    setList(key, [...listFor(key), '']);
    void refreshLayout();
  }
  function removeItem(key: string, idx: number) {
    const before = JSON.parse(JSON.stringify(doc)) as SWOTDoc;
    setList(key, listFor(key).filter((_, i) => i !== idx));
    void refreshLayout();
    showToast('Item deleted', {
      type: 'info',
      undo: () => {
        doc = before;
        void refreshLayout();
      },
    });
  }
  function updateItem(key: string, idx: number, value: string) {
    const list = listFor(key);
    list[idx] = value;
    setList(key, list);
  }

  // Tone → Tailwind class map. Kept here so the four quadrants share
  // one palette definition.
  function paneClass(tone: string): string {
    switch (tone) {
      case 'positive':          return 'border-emerald-900 bg-emerald-950/30';
      case 'negative':          return 'border-rose-900 bg-rose-950/30';
      case 'external_positive': return 'border-sky-900 bg-sky-950/30';
      case 'external_negative': return 'border-amber-900 bg-amber-950/30';
      default:                  return 'border-slate-800 bg-slate-900';
    }
  }
  function headerClass(tone: string): string {
    switch (tone) {
      case 'positive':          return 'text-emerald-300';
      case 'negative':          return 'text-rose-300';
      case 'external_positive': return 'text-sky-300';
      case 'external_negative': return 'text-amber-300';
      default:                  return 'text-slate-300';
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
      lastSavedAt = new Date();
      status = `Saved at ${new Date().toLocaleTimeString()}.`;
    } catch (err: any) {
      status = `Save failed: ${err}`;
    } finally {
      saving = false;
    }
  }
</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <button onclick={() => goto('dashboard')} class="text-xs text-slate-400 hover:text-cyan-400">
        &larr; Dashboard
      </button>
      <h1 class="text-sm font-bold tracking-widest uppercase text-slate-50">SWOT Matrix</h1>
    </div>
    <div class="flex items-center gap-3">
      {#if lastSavedAt}
        <span class="text-[10px] text-slate-500 tabular-nums" title="Charts save automatically as you edit">
          Saved {lastSavedAt.toLocaleTimeString()}
        </span>
      {/if}
      <button
        onclick={save}
        disabled={saving}
        class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-3 py-1 rounded"
      >
        {saving ? 'Saving...' : 'Save'}
      </button>
    </div>
  </header>

  <main class="p-6 space-y-6">
    {#if status}
      <p class="text-xs text-cyan-400" role="status" aria-live="polite">{status}</p>
    {/if}

    <label class="block max-w-md">
      <span class="text-xs font-semibold text-slate-500 uppercase">Title (optional)</span>
      <input
        bind:value={doc.title}
        onblur={refreshLayout}
        placeholder="e.g. Q3 Product Launch SWOT"
        class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
      />
    </label>

    <!-- Axis labels above and to the left of the grid -->
    <div class="grid grid-cols-[140px_1fr_1fr] gap-2 items-center">
      <div></div>
      <div class="text-center text-[10px] text-slate-500 uppercase tracking-widest">
        Positive
      </div>
      <div class="text-center text-[10px] text-slate-500 uppercase tracking-widest">
        Negative
      </div>

      <div class="text-right text-[10px] text-slate-500 uppercase tracking-widest pr-2">
        Internal
      </div>
      {#each layout.quadrants.filter((q) => q.row === 0) as q (q.key)}
        <article class="rounded-lg border p-4 min-h-[200px] {paneClass(q.tone)}">
          <h2 class="text-sm font-bold tracking-widest uppercase mb-3 {headerClass(q.tone)}">
            {q.key} · {q.title}
          </h2>
          <ul class="space-y-1.5">
            {#each listFor(q.key) as _, i}
              <li class="flex gap-2 items-start">
                <span class="text-slate-500 mt-2">·</span>
                <textarea
                  rows="1"
                  value={listFor(q.key)[i]}
                  oninput={(e) => updateItem(q.key, i, (e.target as HTMLTextAreaElement).value)}
                  onblur={refreshLayout}
                  aria-label={`${q.title} item ${i + 1}`}
                  class="flex-1 bg-transparent border-b border-slate-800 text-sm py-1 focus:border-cyan-500 outline-none resize-none"
                ></textarea>
                <button
                  onclick={() => removeItem(q.key, i)}
                  class="text-slate-500 hover:text-red-400 text-xs mt-1"
                  aria-label="Remove item" title="Remove item"
                >
                  ×
                </button>
              </li>
            {/each}
            <li>
              <button
                onclick={() => addItem(q.key)}
                class="text-xs text-cyan-400 hover:text-cyan-300 mt-2"
              >
                + Add
              </button>
            </li>
          </ul>
        </article>
      {/each}

      <div class="text-right text-[10px] text-slate-500 uppercase tracking-widest pr-2">
        External
      </div>
      {#each layout.quadrants.filter((q) => q.row === 1) as q (q.key)}
        <article class="rounded-lg border p-4 min-h-[200px] {paneClass(q.tone)}">
          <h2 class="text-sm font-bold tracking-widest uppercase mb-3 {headerClass(q.tone)}">
            {q.key} · {q.title}
          </h2>
          <ul class="space-y-1.5">
            {#each listFor(q.key) as _, i}
              <li class="flex gap-2 items-start">
                <span class="text-slate-500 mt-2">·</span>
                <textarea
                  rows="1"
                  value={listFor(q.key)[i]}
                  oninput={(e) => updateItem(q.key, i, (e.target as HTMLTextAreaElement).value)}
                  onblur={refreshLayout}
                  aria-label={`${q.title} item ${i + 1}`}
                  class="flex-1 bg-transparent border-b border-slate-800 text-sm py-1 focus:border-cyan-500 outline-none resize-none"
                ></textarea>
                <button
                  onclick={() => removeItem(q.key, i)}
                  class="text-slate-500 hover:text-red-400 text-xs mt-1"
                  aria-label="Remove item" title="Remove item"
                >
                  ×
                </button>
              </li>
            {/each}
            <li>
              <button
                onclick={() => addItem(q.key)}
                class="text-xs text-cyan-400 hover:text-cyan-300 mt-2"
              >
                + Add
              </button>
            </li>
          </ul>
        </article>
      {/each}
    </div>
  </main>
</div>
