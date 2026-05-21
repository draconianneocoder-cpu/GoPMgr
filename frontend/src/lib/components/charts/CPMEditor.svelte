<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // CPMEditor records each activity's Duration. The backend runs the
  // full CPM forward/backward pass (kernel.CalculateCPM) and writes
  // ES/EF/LS/LF/Float plus IsCritical back per node. Critical-path
  // activities are tinted red on the canvas.
  import LayeredEditorShell from './_layered_editor_shell.svelte';

  function fmt(n: unknown, digits = 1): string {
    return typeof n === 'number' ? n.toFixed(digits) : '—';
  }
</script>

<LayeredEditorShell chartKind="cpm" headingLabel="CPM Chart">
  {#snippet nodeContent(data, n)}
    <!-- Critical-path tint overrides the shell's default fill. -->
    {#if data.is_critical}
      <rect
        width={n.width as number}
        height={n.height as number}
        rx="6"
        fill="#7f1d1d"
        opacity="0.6"
      />
    {/if}
    <text x="8" y="18" font-size="11" fill="#f1f5f9" font-weight="bold">
      {(data.label as string).length > 22
        ? (data.label as string).slice(0, 21) + '…'
        : data.label as string}
    </text>
    <text x="8" y="34" font-size="9" fill="#94a3b8">
      Dur: {fmt(data.duration)} · Float: {fmt(data.float, 2)}
    </text>
    <text x="8" y="48" font-size="9" fill="#67e8f9">
      ES {fmt(data.es)} · EF {fmt(data.ef)}
    </text>
    <text x="8" y="62" font-size="9" fill="#fbbf24">
      LS {fmt(data.ls)} · LF {fmt(data.lf)}
    </text>
  {/snippet}

  {#snippet nodeDetailPanel(node)}
    <label class="block">
      <span class="text-xs text-slate-500 uppercase">Duration (days)</span>
      <input
        type="number"
        bind:value={node.duration}
        class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
      />
    </label>
    <div class="mt-3 p-2 bg-slate-950 rounded text-xs space-y-1">
      <div class="flex justify-between">
        <span class="text-slate-500">Earliest start (ES)</span>
        <span class="text-cyan-300 font-mono">{fmt(node.es)}</span>
      </div>
      <div class="flex justify-between">
        <span class="text-slate-500">Earliest finish (EF)</span>
        <span class="text-cyan-300 font-mono">{fmt(node.ef)}</span>
      </div>
      <div class="flex justify-between">
        <span class="text-slate-500">Latest start (LS)</span>
        <span class="text-amber-300 font-mono">{fmt(node.ls)}</span>
      </div>
      <div class="flex justify-between">
        <span class="text-slate-500">Latest finish (LF)</span>
        <span class="text-amber-300 font-mono">{fmt(node.lf)}</span>
      </div>
      <div class="flex justify-between border-t border-slate-800 pt-1 mt-1">
        <span class="text-slate-500">Float</span>
        <span class="font-mono" class:text-red-400={node.is_critical} class:text-cyan-300={!node.is_critical}>
          {fmt(node.float, 2)}{node.is_critical ? ' (critical)' : ''}
        </span>
      </div>
    </div>
    <p class="text-[10px] text-slate-500 mt-2">
      ES/EF/LS/LF computed server-side via internal/kernel CPM. Any node
      with Float = 0 lies on the critical path and is highlighted red.
    </p>
  {/snippet}
</LayeredEditorShell>
