<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { formatList, t } from '../../i18n/i18n.svelte';
  // PieEditor edits a list of slices. Percentages are computed by the
  // backend and rendered in the StatsChart tooltip.
  import StatsEditorShell from './_stats_editor_shell.svelte';
  import Button from '../Button.svelte';

  interface Slice {
    label: string;
    value: number;
    color?: string;
  }
  interface PieDoc {
    title?: string;
    slices: Slice[];
  }

  let doc = $state<PieDoc>({ title: '', slices: [] });

  function addSlice() {
    doc.slices = [...doc.slices, { label: 'Slice ' + (doc.slices.length + 1), value: 1 }];
  }
  function removeSlice(i: number) {
    doc.slices = doc.slices.filter((_, idx) => idx !== i);
  }
  // The layout leaves negative slices out of the pie; name them here so a
  // slice never disappears without a reason.
  const leftOut = $derived(
    doc.slices.flatMap((s, i) => (s.value < 0 ? [s.label || String(i + 1)] : [])),
  );
</script>

<StatsEditorShell
  headingLabel="Pie Chart"
  initialDoc={{ title: '', slices: [] } as PieDoc}
  bind:doc
>
  {#snippet dataEditor()}
    <label class="block mb-4 max-w-md">
      <span class="text-xs text-slate-500 uppercase">Title</span>
      <input bind:value={doc.title} class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none" />
    </label>

    <table class="text-sm border border-slate-800 w-full max-w-2xl">
      <thead class="bg-slate-950">
        <tr>
          <th class="p-2 text-left text-[10px] text-slate-500 uppercase border-b border-slate-800">Label</th>
          <th class="p-2 text-left text-[10px] text-slate-500 uppercase border-b border-l border-slate-800 w-32">Value</th>
          <th class="p-2 text-left text-[10px] text-slate-500 uppercase border-b border-l border-slate-800 w-32">Color (hex)</th>
          <th class="p-2 border-b border-l border-slate-800 w-12"></th>
        </tr>
      </thead>
      <tbody>
        {#each doc.slices as _, i}
          <tr>
            <td class="p-1 border-b border-slate-800">
              <input bind:value={doc.slices[i].label} aria-label={t('a11y.rowField', { field: t('a11y.col.label'), row: i + 1 })} class="w-full bg-transparent text-sm px-2 py-1 focus:bg-slate-950 rounded" />
            </td>
            <td class="p-1 border-b border-l border-slate-800">
              <input type="number" min="0" bind:value={doc.slices[i].value} aria-invalid={doc.slices[i].value < 0} aria-label={t('a11y.rowField', { field: t('a11y.col.value'), row: i + 1 })} class="w-full bg-transparent text-sm px-2 py-1 focus:bg-slate-950 rounded text-right" />
            </td>
            <td class="p-1 border-b border-l border-slate-800">
              <input bind:value={doc.slices[i].color} placeholder="#22d3ee" class="w-full bg-transparent text-xs px-2 py-1 focus:bg-slate-950 rounded font-mono" />
            </td>
            <td class="p-1 border-b border-l border-slate-800 text-center">
              <Button variant="remove" class="text-xs" onclick={() => removeSlice(i)} aria-label="Remove slice" title="Remove slice">×</Button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
    <button onclick={addSlice} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-2 rounded mt-3">+ Slice</button>
    <p class="text-xs text-amber-300 mt-3 max-w-2xl" role="status">
      {#if leftOut.length}{t('pie.negativeLeftOut', { labels: formatList(leftOut) })}{/if}
    </p>
  {/snippet}
</StatsEditorShell>
