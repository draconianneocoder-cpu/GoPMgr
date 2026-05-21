<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // BurnDownEditor edits days and remaining-work counts. The backend
  // computes the ideal linear trajectory from remaining[0] to 0 and
  // adds it as a second series.
  import StatsEditorShell from './_stats_editor_shell.svelte';

  interface BurnDownDoc {
    title?: string;
    y_label?: string;
    days: number[];
    remaining: number[];
  }

  let doc = $state<BurnDownDoc>({ title: '', y_label: 'Remaining', days: [], remaining: [] });

  function addRow() {
    const nextDay = doc.days.length === 0 ? 0 : doc.days[doc.days.length - 1] + 1;
    doc.days.push(nextDay);
    doc.remaining.push(doc.remaining.length > 0 ? doc.remaining[doc.remaining.length - 1] : 0);
    doc.days = [...doc.days];
    doc.remaining = [...doc.remaining];
  }
  function removeRow(i: number) {
    doc.days = doc.days.filter((_, idx) => idx !== i);
    doc.remaining = doc.remaining.filter((_, idx) => idx !== i);
  }
</script>

<StatsEditorShell
  headingLabel="Burn-Down Chart"
  initialDoc={{ title: '', y_label: 'Remaining', days: [], remaining: [] } as BurnDownDoc}
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

    <table class="text-sm border border-slate-800 w-full max-w-xl">
      <thead class="bg-slate-950">
        <tr>
          <th class="p-2 text-left text-[10px] text-slate-500 uppercase border-b border-slate-800 w-24">Day</th>
          <th class="p-2 text-left text-[10px] text-cyan-400 uppercase border-b border-l border-slate-800">Remaining</th>
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
              <input type="number" bind:value={doc.remaining[i]} class="w-full bg-transparent text-sm px-2 py-1 focus:bg-slate-950 rounded text-right" />
            </td>
            <td class="p-1 border-b border-l border-slate-800 text-center">
              <button onclick={() => removeRow(i)} class="text-slate-500 hover:text-red-400 text-xs" aria-label="Remove day">×</button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
    <button onclick={addRow} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-2 rounded mt-3">+ Day</button>

    <p class="text-[10px] text-slate-500 mt-3 max-w-xl">
      The dashed grey line is the ideal trajectory — a straight line from
      the initial remaining count down to zero across all days. If the
      cyan actual line stays above the ideal, the team is behind schedule.
    </p>
  {/snippet}
</StatsEditorShell>
