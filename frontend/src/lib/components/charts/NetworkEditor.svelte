<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // NetworkEditor renders an activity-on-node Network Diagram. The
  // shell handles node/edge CRUD; this component supplies only the
  // kind-specific bits (heading + on-canvas node decoration).
  import LayeredEditorShell from './_layered_editor_shell.svelte';
</script>

<LayeredEditorShell chartKind="network" headingLabel="Network Diagram">
  {#snippet nodeContent(data, n)}
    <text x="8" y="20" font-size="12" fill="#f1f5f9" font-weight="bold">
      {(data.label as string).length > 22
        ? (data.label as string).slice(0, 21) + '…'
        : data.label as string}
    </text>
    {#if (data.duration as number) > 0}
      <text x="8" y="40" font-size="10" fill="#94a3b8">
        Duration: {data.duration}
      </text>
    {/if}
    {#if data.owner}
      <text x="8" y="56" font-size="10" fill="#67e8f9">
        {data.owner as string}
      </text>
    {/if}
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
    <p class="text-[10px] text-slate-500 mt-2">
      A Network Diagram only records duration as a label. For float and
      critical-path math, use a <strong>CPM Chart</strong>.
    </p>
  {/snippet}
</LayeredEditorShell>
