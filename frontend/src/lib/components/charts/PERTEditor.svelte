<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // PERTEditor adds Optimistic / Most Likely / Pessimistic estimates
  // per node. The backend computes Expected duration and variance
  // (beta-distribution approximation) and writes them back into the
  // node before the layout pass; we render them read-only here.
  import LayeredEditorShell from './_layered_editor_shell.svelte';

  function fmt(n: unknown, digits = 2): string {
    return typeof n === 'number' ? n.toFixed(digits) : '—';
  }
</script>

<LayeredEditorShell chartKind="pert" headingLabel="PERT Chart">
  {#snippet nodeContent(data, n)}
    <text x="8" y="18" font-size="11" fill="#f1f5f9" font-weight="bold">
      {(data.label as string).length > 22
        ? (data.label as string).slice(0, 21) + '…'
        : data.label as string}
    </text>
    <text x="8" y="34" font-size="9" fill="#94a3b8">
      O: {fmt(data.o, 1)} · M: {fmt(data.m, 1)} · P: {fmt(data.p, 1)}
    </text>
    <text x="8" y="50" font-size="9" fill="#67e8f9" font-weight="bold">
      E = {fmt(data.expected)}
    </text>
    <text x="8" y="64" font-size="9" fill="#94a3b8">
      σ = {fmt(data.std_dev)}
    </text>
  {/snippet}

  {#snippet nodeDetailPanel(node)}
    <div class="grid grid-cols-3 gap-2">
      <label class="block">
        <span class="text-[10px] text-slate-500 uppercase">Optimistic</span>
        <input
          type="number"
          bind:value={node.o}
          class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 text-xs rounded focus:border-cyan-500 outline-none"
        />
      </label>
      <label class="block">
        <span class="text-[10px] text-slate-500 uppercase">Most likely</span>
        <input
          type="number"
          bind:value={node.m}
          class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 text-xs rounded focus:border-cyan-500 outline-none"
        />
      </label>
      <label class="block">
        <span class="text-[10px] text-slate-500 uppercase">Pessimistic</span>
        <input
          type="number"
          bind:value={node.p}
          class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 text-xs rounded focus:border-cyan-500 outline-none"
        />
      </label>
    </div>
    <div class="mt-3 p-2 bg-slate-950 rounded text-xs space-y-1">
      <div class="flex justify-between">
        <span class="text-slate-500">Expected duration</span>
        <span class="text-cyan-300 font-mono">{fmt(node.expected)}</span>
      </div>
      <div class="flex justify-between">
        <span class="text-slate-500">Variance</span>
        <span class="text-cyan-300 font-mono">{fmt(node.variance, 3)}</span>
      </div>
      <div class="flex justify-between">
        <span class="text-slate-500">Std deviation</span>
        <span class="text-cyan-300 font-mono">{fmt(node.std_dev)}</span>
      </div>
    </div>
    <p class="text-[10px] text-slate-500 mt-2">
      Expected = (O + 4M + P) / 6 · Variance = ((P − O) / 6)². Computed
      server-side on every save.
    </p>
  {/snippet}
</LayeredEditorShell>
