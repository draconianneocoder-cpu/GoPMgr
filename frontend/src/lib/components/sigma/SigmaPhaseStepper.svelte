<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // SigmaPhaseStepper renders the DMAIC phase navigation and status.
  // Used in the project workspace to switch between Define, Measure, etc.

  let {
    currentPhase = $bindable<string>('define'),
    projectId = $bindable<string>(''),
  }: {
    currentPhase?: string;
    projectId?: string;
  } = $props();

  const phases = [
    { id: 'define', label: 'Define', icon: '🎯' },
    { id: 'measure', label: 'Measure', icon: '📏' },
    { id: 'analyze', label: 'Analyze', icon: '🔍' },
    { id: 'improve', label: 'Improve', icon: '🚀' },
    { id: 'control', label: 'Control', icon: '🛡️' },
  ];

  const phaseOrder = ['define', 'measure', 'analyze', 'improve', 'control'];

  function phaseIndex(id: string) {
    return phaseOrder.indexOf(id);
  }

  function isCompleted(id: string) {
    return phaseIndex(id) < phaseIndex(currentPhase);
  }

  function isCurrent(id: string) {
    return id === currentPhase;
  }

  function goToPhase(id: string) {
    currentPhase = id;
    // In a real app, we'd save this to backend
  }
</script>

<div class="w-full bg-slate-900 border-b border-slate-800 px-6 py-4">
  <div class="flex items-center justify-between max-w-5xl mx-auto">
    {#each phases as phase (phase.id)}
      <button
        onclick={() => goToPhase(phase.id)}
        class="flex flex-col items-center gap-2 group"
      >
        <div
          class="w-10 h-10 rounded-full flex items-center justify-center text-lg transition-all
            {isCompleted(phase.id)
              ? 'bg-emerald-600 text-white'
              : isCurrent(phase.id)
              ? 'bg-cyan-600 text-white ring-4 ring-cyan-900'
              : 'bg-slate-800 text-slate-400 group-hover:bg-slate-700'}"
        >
          {isCompleted(phase.id) ? '✓' : phase.icon}
        </div>
        <span
          class="text-xs font-bold uppercase tracking-wider transition-colors
            {isCurrent(phase.id)
              ? 'text-cyan-400'
              : isCompleted(phase.id)
              ? 'text-emerald-400'
              : 'text-slate-500'}"
        >
          {phase.label}
        </span>
      </button>

      {#if phase.id !== 'control'}
        <div
          class="flex-1 h-0.5 mx-2 transition-colors
            {isCompleted(phase.id) ? 'bg-emerald-600' : 'bg-slate-800'}"
        ></div>
      {/if}
    {/each}
  </div>
</div>
