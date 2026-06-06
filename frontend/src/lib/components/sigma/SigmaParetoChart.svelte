<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // SigmaParetoChart renders a bar + cumulative line chart.
  let {
    items = [],
  }: {
    items?: ParetoItem[];
  } = $props();

  let maxCount = $derived(Math.max(...items.map((i) => i.count), 1));
</script>

<div class="w-full bg-slate-900 border border-slate-800 rounded-lg p-4">
  <div class="flex items-end gap-2 h-48 mb-2">
    {#each items as item (item.category)}
      <div class="flex-1 flex flex-col items-center gap-1 group relative">
        <!-- Tooltip -->
        <div class="absolute bottom-full mb-2 hidden group-hover:block bg-slate-800 text-xs p-2 rounded shadow z-10 whitespace-nowrap">
          {item.category}: {item.count} ({item.percentage.toFixed(1)}%)
        </div>

        <!-- Bar -->
        <div
          class="w-full bg-cyan-600 rounded-t transition-all hover:bg-cyan-500"
          style="height: {(item.count / maxCount) * 100}%;"
        ></div>

        <!-- Label -->
        <div class="text-[10px] text-slate-400 truncate w-full text-center">
          {item.category}
        </div>
      </div>
    {/each}
  </div>

  {#if items.length > 0}
    <div class="text-xs text-slate-500 mt-2 border-t border-slate-800 pt-2">
      Top cause: <span class="text-cyan-400 font-medium">{items[0].category}</span> accounts for
      <span class="text-cyan-400 font-medium">{items[0].percentage.toFixed(1)}%</span> of occurrences.
    </div>
  {/if}
</div>
