<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { showToast } from '../../toast.svelte';

  let {
    projectId = '',
    data = $bindable<FishboneData>({ problem_statement: '', branches: [] }),
  }: {
    projectId?: string;
    data?: FishboneData;
  } = $props();

  let selectedCause = $state<{ branchIdx: number; causeIdx: number } | null>(null);
  let whysInput = $state('');
  let saving = $state(false);

  const defaultCategories = ['Man', 'Machine', 'Method', 'Material', 'Measurement', 'Environment'];

  onMount(async () => {
    if (data.branches.length === 0) {
      data.branches = defaultCategories.map(c => ({ category: c, causes: [] }));
    }
  });

  function nextCauseID(): string {
    const usedIDs = new Set(data.branches.flatMap((branch) => branch.causes.map((cause) => cause.id)));
    let suffix = 1;
    while (usedIDs.has(`cause-${suffix}`)) suffix += 1;
    return `cause-${suffix}`;
  }

  function addCause(branchIdx: number) {
    const desc = prompt('Enter cause description:');
    if (desc) {
      data.branches[branchIdx].causes.push({
        id: nextCauseID(),
        description: desc,
        is_root_cause: false,
        five_whys: [],
        evidence: ''
      });
    }
  }

  function openFiveWhys(branchIdx: number, causeIdx: number) {
    selectedCause = { branchIdx, causeIdx };
    whysInput = data.branches[branchIdx].causes[causeIdx].five_whys.join(' → ') || '';
  }

  function saveFiveWhys() {
    if (selectedCause) {
      const { branchIdx, causeIdx } = selectedCause;
      data.branches[branchIdx].causes[causeIdx].five_whys = whysInput.split('→').map(s => s.trim()).filter(Boolean);
      selectedCause = null;
    }
  }

  function activateSvgAction(event: KeyboardEvent, action: () => void) {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      action();
    }
  }

  async function saveAll() {
    saving = true;
    try {
      await window.go.main.App.SigmaSaveFishbone(projectId, data);
      showToast('Fishbone diagram saved', 'success');
    } catch (err: any) {
      showToast(`Save failed: ${err}`, 'error');
    } finally {
      saving = false;
    }
  }

  // SVG Layout constants
  const width = 900;
  const height = 500;
  const centerX = width / 2;
  const centerY = height / 2;
  const spineLength = 350;
</script>

<div class="w-full bg-slate-900 border border-slate-800 rounded-lg p-4 overflow-x-auto">
  <div class="flex justify-between items-center mb-4">
    <h3 class="text-sm font-bold uppercase tracking-widest text-cyan-400">Fishbone Diagram</h3>
    <button onclick={saveAll} disabled={saving} class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-3 py-1.5 rounded">
      {saving ? 'Saving...' : 'Save Diagram'}
    </button>
  </div>

  <input
    bind:value={data.problem_statement}
    class="w-full mb-4 bg-slate-950 border border-slate-800 p-2 rounded text-sm text-center font-bold text-red-400"
    placeholder="Effect / Problem Statement"
  />

  <svg viewBox="0 0 {width} {height}" class="w-full h-auto select-none">
    <!-- Main Spine -->
    <line x1="50" y1={centerY} x2={centerX + spineLength} y2={centerY} stroke="#94a3b8" stroke-width="3" />
    <!-- Arrow Head -->
    <polygon points="{centerX + spineLength},{centerY} {centerX + spineLength - 15},{centerY - 10} {centerX + spineLength - 15},{centerY + 10}" fill="#94a3b8" />

    {#each data.branches as branch, bIdx (branch.category)}
      {@const isTop = bIdx < 3}
      {@const yOffset = isTop ? -120 : 120}
      {@const xStart = 150 + bIdx * 120}

      <!-- Branch Line -->
      <line x1={xStart} y1={centerY} x2={xStart + 60} y2={centerY + yOffset} stroke="#64748b" stroke-width="2" />

      <!-- Category Label -->
      <text x={xStart + 70} y={centerY + yOffset - 10} fill="#cbd5e1" font-size="12" font-weight="bold">{branch.category}</text>

      <!-- Causes -->
      {#each branch.causes as cause, cIdx (cause.id)}
        {@const causeY = centerY + yOffset + (cIdx * 25) - (branch.causes.length * 12)}
        <line x1={xStart + 20} y1={centerY + yOffset} x2={xStart + 60} y2={causeY} stroke="#475569" stroke-width="1" />
        <text
          x={xStart + 65}
          y={causeY + 4}
          fill={cause.is_root_cause ? '#f87171' : '#e2e8f0'}
          font-size="11"
          class="cursor-pointer hover:fill-white"
          role="button"
          tabindex="0"
          onclick={() => openFiveWhys(bIdx, cIdx)}
          onkeydown={(event) => activateSvgAction(event, () => openFiveWhys(bIdx, cIdx))}
        >
          {cause.description}
        </text>
        {#if cause.is_root_cause}
          <circle cx={xStart + 60} cy={causeY} r="3" fill="#f87171" />
        {/if}
      {/each}

      <!-- Add Cause Button -->
      <text
        x={xStart + 20}
        y={centerY + yOffset + 40}
        fill="#3b82f6"
        font-size="10"
        class="cursor-pointer hover:fill-blue-400"
        role="button"
        tabindex="0"
        onclick={() => addCause(bIdx)}
        onkeydown={(event) => activateSvgAction(event, () => addCause(bIdx))}
      >
        + Add Cause
      </text>
    {/each}
  </svg>

  {#if selectedCause}
    <div class="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
      <div class="bg-slate-900 border border-slate-700 rounded-lg p-6 w-full max-w-md mx-4">
        <h4 class="text-sm font-bold uppercase tracking-widest text-amber-400 mb-2">5 Whys Drill-Down</h4>
        <p class="text-xs text-slate-400 mb-4">Cause: {data.branches[selectedCause.branchIdx].causes[selectedCause.causeIdx].description}</p>

        <textarea
          bind:value={whysInput}
          rows="4"
          class="w-full bg-slate-950 border border-slate-800 p-3 rounded text-sm focus:border-amber-500 outline-none"
          placeholder="Why 1 → Why 2 → Why 3..."
        ></textarea>

        <div class="flex justify-end gap-3 mt-4">
          <button onclick={() => selectedCause = null} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1.5 rounded">Cancel</button>
          <button onclick={saveFiveWhys} class="text-xs bg-amber-600 hover:bg-amber-500 text-white font-bold uppercase px-3 py-1.5 rounded">Save Drill-Down</button>
        </div>
      </div>
    </div>
  {/if}
</div>
