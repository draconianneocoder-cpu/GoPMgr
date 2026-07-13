<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // BurnUpEditor edits three aligned series: days (x-axis),
  // completed (cumulative work delivered), scope (total work in the
  // backlog, which can grow with scope creep). All three arrays must
  // have the same length; the editor enforces this when adding rows.
  import StatsEditorShell from './_stats_editor_shell.svelte';

  interface BurnUpDoc {
    title?: string;
    y_label?: string;
    days: number[];
    completed: number[];
    scope: number[];
  }

  let doc = $state<BurnUpDoc>({ title: '', y_label: 'Story points', days: [], completed: [], scope: [] });

  function addRow() {
    const nextDay = doc.days.length === 0 ? 1 : doc.days[doc.days.length - 1] + 1;
    doc.days.push(nextDay);
    doc.completed.push(0);
    doc.scope.push(doc.scope.length > 0 ? doc.scope[doc.scope.length - 1] : 0);
    doc.days = [...doc.days];
    doc.completed = [...doc.completed];
    doc.scope = [...doc.scope];
  }
  function removeRow(i: number) {
    doc.days = doc.days.filter((_, idx) => idx !== i);
    doc.completed = doc.completed.filter((_, idx) => idx !== i);
    doc.scope = doc.scope.filter((_, idx) => idx !== i);
  }
</script>

<StatsEditorShell
  headingLabel="Burn-Up Chart"
  initialDoc={{ title: '', y_label: 'Story points', days: [], completed: [], scope: [] } as BurnUpDoc}
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

    <table class="text-sm border border-slate-800 w-full max-w-3xl">
      <thead class="bg-slate-950">
        <tr>
          <th class="p-2 text-left text-[10px] text-slate-500 uppercase border-b border-slate-800 w-24">Day</th>
          <th class="p-2 text-left text-[10px] text-cyan-400 uppercase border-b border-l border-slate-800">Completed</th>
          <th class="p-2 text-left text-[10px] text-amber-400 uppercase border-b border-l border-slate-800">Total scope</th>
          <th class="p-2 border-b border-l border-slate-800 w-12"></th>
        </tr>
      </thead>
      <tbody>
        {#each doc.days as _, i}
          <tr>
            <td class="p-1 border-b border-slate-800">
              <input type="number" bind:value={doc.days[i]} class="w-full bg-transparent text-sm px-2 py-1 focus:bg-slate-950 rounded text-right" />
            </td>
            <td class="p-1 border-b border-l border-slate-800">
              <input type="number" bind:value={doc.completed[i]} class="w-full bg-transparent text-sm px-2 py-1 focus:bg-slate-950 rounded text-right" />
            </td>
            <td class="p-1 border-b border-l border-slate-800">
              <input type="number" bind:value={doc.scope[i]} class="w-full bg-transparent text-sm px-2 py-1 focus:bg-slate-950 rounded text-right" />
            </td>
            <td class="p-1 border-b border-l border-slate-800 text-center">
              <button onclick={() => removeRow(i)} class="text-slate-500 hover:text-red-400 text-xs" aria-label="Remove day" title="Remove day">×</button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
    <button onclick={addRow} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-2 rounded mt-3">+ Day</button>

    <p class="text-[10px] text-slate-500 mt-3 max-w-xl">
      Completed is cumulative — the total work delivered through that day.
      Scope can grow when new work is added to the backlog. The team is
      done when the two lines converge.
    </p>
  {/snippet}
</StatsEditorShell>
