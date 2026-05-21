<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // ControlChartEditor: a time series of measurements with optional
  // explicit Mean / UCL / LCL. If left at 0/0/0, the backend computes
  // mean ± 3σ from the data. Out-of-control points are flagged red
  // in the rendered chart automatically.
  import StatsEditorShell from './_stats_editor_shell.svelte';

  interface ControlDoc {
    title?: string;
    y_label?: string;
    x: number[];
    y: number[];
    mean?: number;
    ucl?: number;
    lcl?: number;
  }

  let doc = $state<ControlDoc>({
    title: '',
    y_label: '',
    x: [],
    y: [],
    mean: 0,
    ucl: 0,
    lcl: 0,
  });

  function addRow() {
    const next = doc.x.length === 0 ? 1 : doc.x[doc.x.length - 1] + 1;
    doc.x = [...doc.x, next];
    doc.y = [...doc.y, 0];
  }
  function removeRow(i: number) {
    doc.x = doc.x.filter((_, idx) => idx !== i);
    doc.y = doc.y.filter((_, idx) => idx !== i);
  }
  function resetLimits() {
    doc.mean = 0;
    doc.ucl = 0;
    doc.lcl = 0;
  }
</script>

<StatsEditorShell
  headingLabel="Control Chart"
  initialDoc={{ title: '', y_label: '', x: [], y: [], mean: 0, ucl: 0, lcl: 0 } as ControlDoc}
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
        <input bind:value={doc.y_label} class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none" />
      </label>
    </div>

    <section class="grid grid-cols-1 md:grid-cols-4 gap-3 mb-4 max-w-2xl">
      <label class="block">
        <span class="text-[10px] text-slate-500 uppercase">Mean</span>
        <input type="number" bind:value={doc.mean} class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 text-xs rounded focus:border-cyan-500 outline-none" />
      </label>
      <label class="block">
        <span class="text-[10px] text-slate-500 uppercase">UCL</span>
        <input type="number" bind:value={doc.ucl} class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 text-xs rounded focus:border-cyan-500 outline-none" />
      </label>
      <label class="block">
        <span class="text-[10px] text-slate-500 uppercase">LCL</span>
        <input type="number" bind:value={doc.lcl} class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 text-xs rounded focus:border-cyan-500 outline-none" />
      </label>
      <button
        onclick={resetLimits}
        class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-2 rounded self-end"
      >
        Auto (mean ± 3σ)
      </button>
    </section>

    <table class="text-sm border border-slate-800 w-full max-w-xl">
      <thead class="bg-slate-950">
        <tr>
          <th class="p-2 text-left text-[10px] text-slate-500 uppercase border-b border-slate-800 w-24">Sample</th>
          <th class="p-2 text-left text-[10px] text-cyan-400 uppercase border-b border-l border-slate-800">Measurement</th>
          <th class="p-2 border-b border-l border-slate-800 w-12"></th>
        </tr>
      </thead>
      <tbody>
        {#each doc.x as _, i}
          <tr>
            <td class="p-1 border-b border-slate-800">
              <input type="number" bind:value={doc.x[i]} class="w-full bg-transparent text-sm px-2 py-1 focus:bg-slate-950 rounded text-right" />
            </td>
            <td class="p-1 border-b border-l border-slate-800">
              <input type="number" bind:value={doc.y[i]} class="w-full bg-transparent text-sm px-2 py-1 focus:bg-slate-950 rounded text-right" />
            </td>
            <td class="p-1 border-b border-l border-slate-800 text-center">
              <button onclick={() => removeRow(i)} class="text-slate-500 hover:text-red-400 text-xs" aria-label="Remove sample">×</button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
    <button onclick={addRow} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-2 rounded mt-3">+ Sample</button>

    <p class="text-[10px] text-slate-500 mt-3 max-w-xl">
      Leave Mean / UCL / LCL at 0 to have the backend compute them as
      mean ± 3σ from your samples. Out-of-control points (outside
      [LCL, UCL]) are highlighted red in the chart.
    </p>
  {/snippet}
</StatsEditorShell>
