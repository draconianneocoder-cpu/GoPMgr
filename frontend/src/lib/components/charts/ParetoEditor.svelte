<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // ParetoEditor: a flat list of (label, count) items. The backend
  // sorts them descending, computes cumulative percentage, and
  // overlays the 80% reference line.
  import StatsEditorShell from './_stats_editor_shell.svelte';

  interface Item {
    label: string;
    count: number;
  }
  interface ParetoDoc {
    title?: string;
    y_label?: string;
    items: Item[];
  }

  let doc = $state<ParetoDoc>({ title: '', y_label: 'Count', items: [] });

  function addItem() {
    doc.items = [...doc.items, { label: '', count: 0 }];
  }
  function removeItem(i: number) {
    doc.items = doc.items.filter((_, idx) => idx !== i);
  }
</script>

<StatsEditorShell
  headingLabel="Pareto Chart"
  initialDoc={{ title: '', y_label: 'Count', items: [] } as ParetoDoc}
  bind:doc
>
  {#snippet dataEditor()}
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4 max-w-2xl">
      <label class="block">
        <span class="text-xs text-slate-500 uppercase">Title</span>
        <input bind:value={doc.title} class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none" />
      </label>
      <label class="block">
        <span class="text-xs text-slate-500 uppercase">Y-axis label</span>
        <input bind:value={doc.y_label} placeholder="Count" class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none" />
      </label>
    </div>

    <table class="text-sm border border-slate-800 w-full max-w-2xl">
      <thead class="bg-slate-950">
        <tr>
          <th class="p-2 text-left text-[10px] text-slate-500 uppercase border-b border-slate-800">Category</th>
          <th class="p-2 text-left text-[10px] text-slate-500 uppercase border-b border-l border-slate-800 w-32">Count</th>
          <th class="p-2 border-b border-l border-slate-800 w-12"></th>
        </tr>
      </thead>
      <tbody>
        {#each doc.items as _, i}
          <tr>
            <td class="p-1 border-b border-slate-800">
              <input bind:value={doc.items[i].label} class="w-full bg-transparent text-sm px-2 py-1 focus:bg-slate-950 rounded" />
            </td>
            <td class="p-1 border-b border-l border-slate-800">
              <input type="number" bind:value={doc.items[i].count} class="w-full bg-transparent text-sm px-2 py-1 focus:bg-slate-950 rounded text-right" />
            </td>
            <td class="p-1 border-b border-l border-slate-800 text-center">
              <button onclick={() => removeItem(i)} class="text-slate-500 hover:text-red-400 text-xs" aria-label="Remove item">×</button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
    <button onclick={addItem} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-2 rounded mt-3">+ Item</button>

    <p class="text-[10px] text-slate-500 mt-3 max-w-xl">
      The backend sorts items by count descending and overlays the cumulative
      percentage on the right axis. The dashed reference line at 80% marks
      the vital-few / trivial-many threshold of the Pareto principle.
    </p>
  {/snippet}
</StatsEditorShell>
