<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // TollgateChecklist shows readiness score and individual checks.
  let {
    score = 0,
    canAdvance = false,
    checks = [],
    missingList = '',
  }: {
    score?: number;
    canAdvance?: boolean;
    checks?: { name: string; passed: boolean; message: string }[];
    missingList?: string;
  } = $props();

  const circumference = 2 * Math.PI * 40;
  let offset = $derived(circumference - (score / 100) * circumference);
  // One band decides both the arc and the percentage, so a failing score
  // no longer pairs a red arc with amber text.
  let scoreTone = $derived(
    score >= 80
      ? { token: '--emerald-400', text: 'text-emerald-400' }
      : score >= 50
        ? { token: '--amber-400', text: 'text-amber-400' }
        : { token: '--red-400', text: 'text-red-400' },
  );
</script>

<div class="bg-slate-900 border border-slate-800 rounded-lg p-5">
  <div class="flex items-center gap-6">
    <!-- Circular Progress -->
    <div class="relative w-24 h-24 flex-shrink-0">
      <svg class="w-full h-full transform -rotate-90" viewBox="0 0 100 100">
        <!-- Theme tokens through style (fill=/stroke= attributes cannot resolve
             var()): the fixed dark-theme hexes drew a near-black track and
             a 2:1 amber arc on the light theme's pale card. -->
        <circle cx="50" cy="50" r="40" style="stroke: rgb(var(--slate-800))" stroke-width="8" fill="none" />
        <circle
          cx="50" cy="50" r="40"
          style="stroke: rgb(var({scoreTone.token}))"
          stroke-width="8"
          fill="none"
          stroke-dasharray={circumference}
          stroke-dashoffset={offset}
          class="transition-all duration-500"
        />
      </svg>
      <div class="absolute inset-0 flex items-center justify-center">
        <span class="text-xl font-bold {scoreTone.text}">
          {Math.round(score)}%
        </span>
      </div>
    </div>

    <!-- Checklist -->
    <div class="flex-1 space-y-2">
      {#each checks as check (check.name)}
        <div class="flex items-start gap-3 text-sm">
          <span class="mt-0.5 text-lg">
            {check.passed ? '✅' : '❌'}
          </span>
          <div>
            <div class="font-medium {check.passed ? 'text-emerald-300' : 'text-slate-300'}">
              {check.name}
            </div>
            {#if !check.passed}
              <div class="text-xs text-slate-500">{check.message}</div>
            {/if}
          </div>
        </div>
      {/each}
    </div>
  </div>

  {#if !canAdvance}
    <div class="mt-4 p-3 bg-red-900/30 border border-red-800 rounded text-xs text-red-300">
      ⚠️ Tollgate not ready. Complete missing items before advancing phase.
      {#if missingList}
        <br/>Missing: <span class="font-medium">{missingList}</span>
      {/if}
    </div>
  {:else}
    <div class="mt-4 p-3 bg-emerald-900/30 border border-emerald-800 rounded text-xs text-emerald-300">
      ✅ Tollgate passed. You may advance to the next phase.
    </div>
  {/if}
</div>
