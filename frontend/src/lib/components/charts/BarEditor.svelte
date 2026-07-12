<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // BarEditor mirrors LineEditor closely — same categories × series
  // grid — but the chart renders as vertical bars. Same data model
  // (categories + named series of values).
  import StatsEditorShell from './_stats_editor_shell.svelte';

  interface SeriesDef {
    name: string;
    values: number[];
    color?: string;
  }
  interface BarDoc {
    title?: string;
    x_label?: string;
    y_label?: string;
    categories: string[];
    series: SeriesDef[];
  }

  let doc = $state<BarDoc>({
    title: '',
    x_label: '',
    y_label: '',
    categories: [],
    series: [],
  });

  function addCategory() {
    doc.categories = [...doc.categories, ''];
    for (const s of doc.series) s.values.push(0);
  }
  function removeCategory(i: number) {
    doc.categories = doc.categories.filter((_, idx) => idx !== i);
    for (const s of doc.series) s.values = s.values.filter((_, idx) => idx !== i);
  }
  function addSeries() {
    const n = doc.categories.length;
    doc.series = [...doc.series, { name: 'Series ' + (doc.series.length + 1), values: new Array(n).fill(0) }];
  }
  function removeSeries(i: number) {
    doc.series = doc.series.filter((_, idx) => idx !== i);
  }
</script>

<StatsEditorShell
  headingLabel="Bar Chart"
  initialDoc={{ title: '', x_label: '', y_label: '', categories: [], series: [] } as BarDoc}
  bind:doc
>
  {#snippet dataEditor()}
    <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
      <label class="block">
        <span class="text-xs text-slate-500 uppercase">Title</span>
        <input bind:value={doc.title} class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none" />
      </label>
      <label class="block">
        <span class="text-xs text-slate-500 uppercase">X-axis label</span>
        <input bind:value={doc.x_label} class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none" />
      </label>
      <label class="block">
        <span class="text-xs text-slate-500 uppercase">Y-axis label</span>
        <input bind:value={doc.y_label} class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none" />
      </label>
    </div>

    <div class="overflow-x-auto">
      <table class="text-sm border border-slate-800">
        <thead class="bg-slate-950">
          <tr>
            <th class="p-2 text-left text-[10px] text-slate-500 uppercase border-b border-slate-800">
              Category \ series
            </th>
            {#each doc.series as _, i (i)}
              <th class="p-2 border-b border-l border-slate-800 min-w-[140px]">
                <div class="flex items-center gap-1">
                  <input bind:value={doc.series[i].name} class="flex-1 bg-transparent text-xs px-1 py-1 focus:bg-slate-800 rounded" />
                  <button onclick={() => removeSeries(i)} class="text-slate-500 hover:text-red-400" aria-label="Remove series" title="Remove series">×</button>
                </div>
              </th>
            {/each}
            <th class="p-2 border-b border-l border-slate-800">
              <button onclick={addSeries} class="text-xs bg-slate-800 hover:bg-slate-700 px-2 py-1 rounded">+ Series</button>
            </th>
          </tr>
        </thead>
        <tbody>
          {#each doc.categories as _, ci}
            <tr>
              <td class="p-1 border-b border-slate-800 bg-slate-950">
                <div class="flex items-center gap-1">
                  <input bind:value={doc.categories[ci]} class="flex-1 bg-transparent text-xs px-2 py-1 focus:bg-slate-900 rounded" />
                  <button onclick={() => removeCategory(ci)} class="text-slate-500 hover:text-red-400 text-xs" aria-label="Remove category" title="Remove category">×</button>
                </div>
              </td>
              {#each doc.series as _, si}
                <td class="p-1 border-b border-l border-slate-800">
                  <input
                    type="number"
                    bind:value={doc.series[si].values[ci]}
                    class="w-full bg-transparent text-xs px-2 py-1 focus:bg-slate-950 rounded text-right"
                  />
                </td>
              {/each}
              <td class="border-b border-l border-slate-800"></td>
            </tr>
          {/each}
          <tr>
            <td class="p-2 border-t border-slate-800 bg-slate-950">
              <button onclick={addCategory} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded">+ category</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  {/snippet}
</StatsEditorShell>
