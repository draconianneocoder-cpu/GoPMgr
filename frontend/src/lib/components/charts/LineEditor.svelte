<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // LineEditor edits a Line chart: a list of x-axis labels (or
  // numbers) and one or more named series of y-values aligned to
  // that x-axis.
  import StatsEditorShell from './_stats_editor_shell.svelte';

  interface SeriesDef {
    name: string;
    y: number[];
    color?: string;
    dashed?: boolean;
  }
  interface LineDoc {
    title?: string;
    x_label?: string;
    y_label?: string;
    x_str?: string[]; // category labels
    series: SeriesDef[];
  }

  let doc = $state<LineDoc>({
    title: '',
    x_label: '',
    y_label: '',
    x_str: [],
    series: [],
  });

  function addX() {
    doc.x_str = [...(doc.x_str ?? []), ''];
    for (const s of doc.series) s.y.push(0);
  }
  function removeX(i: number) {
    doc.x_str = (doc.x_str ?? []).filter((_, idx) => idx !== i);
    for (const s of doc.series) s.y = s.y.filter((_, idx) => idx !== i);
  }
  function addSeries() {
    const n = (doc.x_str ?? []).length;
    doc.series = [...doc.series, { name: 'Series ' + (doc.series.length + 1), y: new Array(n).fill(0) }];
  }
  function removeSeries(i: number) {
    doc.series = doc.series.filter((_, idx) => idx !== i);
  }
</script>

<StatsEditorShell
  headingLabel="Line Chart"
  initialDoc={{ title: '', x_label: '', y_label: '', x_str: [], series: [] } as LineDoc}
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
              x \ series
            </th>
            {#each doc.series as s, i (i)}
              <th class="p-2 border-b border-l border-slate-800 min-w-[140px]">
                <div class="flex items-center gap-1">
                  <input bind:value={doc.series[i].name} class="flex-1 bg-transparent text-xs px-1 py-1 focus:bg-slate-800 rounded" />
                  <button onclick={() => removeSeries(i)} class="text-slate-500 hover:text-red-400" aria-label="Remove series">×</button>
                </div>
                <label class="flex items-center gap-1 mt-1 text-[10px]">
                  <input type="checkbox" bind:checked={doc.series[i].dashed} />
                  dashed
                </label>
              </th>
            {/each}
            <th class="p-2 border-b border-l border-slate-800">
              <button onclick={addSeries} class="text-xs bg-slate-800 hover:bg-slate-700 px-2 py-1 rounded">+ Series</button>
            </th>
          </tr>
        </thead>
        <tbody>
          {#each (doc.x_str ?? []) as _, xi}
            <tr>
              <td class="p-1 border-b border-slate-800 bg-slate-950">
                <div class="flex items-center gap-1">
                  <input bind:value={doc.x_str![xi]} class="flex-1 bg-transparent text-xs px-2 py-1 focus:bg-slate-900 rounded" />
                  <button onclick={() => removeX(xi)} class="text-slate-500 hover:text-red-400 text-xs" aria-label="Remove row">×</button>
                </div>
              </td>
              {#each doc.series as _, si}
                <td class="p-1 border-b border-l border-slate-800">
                  <input
                    type="number"
                    bind:value={doc.series[si].y[xi]}
                    class="w-full bg-transparent text-xs px-2 py-1 focus:bg-slate-950 rounded text-right"
                  />
                </td>
              {/each}
              <td class="border-b border-l border-slate-800"></td>
            </tr>
          {/each}
          <tr>
            <td class="p-2 border-t border-slate-800 bg-slate-950">
              <button onclick={addX} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded">+ x point</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  {/snippet}
</StatsEditorShell>
