<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // BudgetPanel is a Dashboard-embedded summary tile. It calls
  // ComputeBudget on mount and renders four numbers + a progress
  // bar + a per-category breakdown.
  //
  // The data sources are: project.budget (the cap), stakeholder
  // contract_values (vendors), and matched work-item points ×
  // assignee hourly_rate (labour estimate).

  import { onMount, onDestroy } from 'svelte';
  import Spinner from '../Spinner.svelte';

  let summary = $state<BudgetSummary | null>(null);
  let error = $state('');

  onMount(async () => {
    try {
      summary = await window.go.main.App.ComputeBudget();
    } catch (err: any) {
      error = String(err);
    }
  });

  function formatCompactCurrency(n: number, options: { compact?: boolean } = {}): string {
    if (n === 0) return '0';
    const compact = options.compact === true;
    return n.toLocaleString(undefined, {
      maximumFractionDigits: compact ? 1 : 0,
      notation: compact ? 'compact' : 'standard',
    });
  }

  function fmt(n: number): string {
    return formatCompactCurrency(n);
  }

  // Progress: committed as % of budget. >100% turns red.
  let pct = $derived(
    summary && summary.budget > 0 ? (summary.committed / summary.budget) * 100 : 0,
  );
  let pctClamped = $derived(Math.min(100, pct));
  let overBudget = $derived(pct > 100);

  onDestroy(() => {});
</script>

<section class="bg-slate-900 border border-slate-800 rounded-lg p-4">
  <div class="flex items-center justify-between mb-3">
    <h2 class="text-xs font-bold uppercase tracking-widest text-slate-500">Budget</h2>
    {#if summary && summary.budget > 0}
      <span class="text-[10px] text-slate-500">
        {pct.toFixed(0)}% committed
      </span>
    {/if}
  </div>

  {#if error}
    <p class="text-xs text-red-400 break-words" role="alert">{error}</p>
  {:else if !summary}
    <Spinner label="Loading budget…" class="py-2" />
  {:else if summary.budget === 0 && summary.committed === 0}
    <p class="text-xs text-slate-500">
      Set a budget on the project metadata and add stakeholder rates / contracts to see this panel populate.
    </p>
  {:else}
    <div class="grid grid-cols-2 md:grid-cols-4 gap-3 mb-3">
      <div class="min-w-0">
        <div class="text-[10px] uppercase tracking-widest text-slate-500">Budget</div>
        <div class="text-base font-bold text-slate-50 tabular-nums truncate" title={fmt(summary.budget)}>
          {formatCompactCurrency(summary.budget, { compact: true })}
        </div>
      </div>
      <div class="min-w-0">
        <div class="text-[10px] uppercase tracking-widest text-slate-500">Committed</div>
        <div class="text-base font-bold text-amber-300 tabular-nums truncate" title={fmt(summary.committed)}>
          {formatCompactCurrency(summary.committed, { compact: true })}
        </div>
      </div>
      <div class="min-w-0">
        <div class="text-[10px] uppercase tracking-widest text-slate-500">Contracts</div>
        <div class="text-base font-bold text-cyan-300 tabular-nums truncate" title={fmt(summary.contract_value)}>
          {formatCompactCurrency(summary.contract_value, { compact: true })}
        </div>
      </div>
      <div class="min-w-0">
        <div class="text-[10px] uppercase tracking-widest text-slate-500">Labour est.</div>
        <div class="text-base font-bold text-cyan-300 tabular-nums truncate" title={fmt(summary.labour_estimate)}>
          {formatCompactCurrency(summary.labour_estimate, { compact: true })}
        </div>
      </div>
    </div>

    <!-- Progress bar -->
    {#if summary.budget > 0}
      <div class="h-2 bg-slate-950 rounded overflow-hidden">
        <div
          class="h-full {overBudget ? 'bg-red-500' : 'bg-cyan-500'}"
          style="width: {pctClamped}%"
        ></div>
      </div>
      <div class="flex justify-between text-[10px] mt-1">
        <span class="text-slate-500">0</span>
        <span class="{overBudget ? 'text-red-300' : 'text-slate-400'}">
          Remaining: {fmt(summary.remaining)}
        </span>
        <span class="text-slate-500 tabular-nums truncate text-right" title={fmt(summary.budget)}>
          {formatCompactCurrency(summary.budget, { compact: true })}
        </span>
      </div>
    {/if}

    <!-- Per-category breakdown -->
    {#if Object.keys(summary.by_category).length > 0}
      <div class="mt-3 grid grid-cols-2 md:grid-cols-4 gap-2 text-[10px]">
        {#each Object.entries(summary.by_category) as [cat, val] (cat)}
          <div class="bg-slate-950 rounded p-2">
            <div class="uppercase tracking-widest text-slate-500">{cat}</div>
            <div class="font-bold text-slate-200 tabular-nums truncate" title={fmt(val as number)}>
              {formatCompactCurrency(val as number, { compact: true })}
            </div>
          </div>
        {/each}
      </div>
    {/if}
  {/if}
</section>
