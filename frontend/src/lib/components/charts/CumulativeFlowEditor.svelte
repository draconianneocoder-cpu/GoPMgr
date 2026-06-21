<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // CumulativeFlowEditor: Days × States grid. Each cell is the WIP
  // count for a state on a day. State order (bottom-to-top of the
  // stack) is editable via the up/down arrows on each state header.
  import StatsEditorShell from './_stats_editor_shell.svelte';

  interface CumFlowDoc {
    title?: string;
    y_label?: string;
    days: number[];
    states: Record<string, number[]>;
    state_order: string[];
  }

  let doc = $state<CumFlowDoc>({
    title: '',
    y_label: 'WIP',
    days: [],
    states: {},
    state_order: [],
  });

  let newStateName = $state('');

  function addDay() {
    const next = doc.days.length === 0 ? 1 : doc.days[doc.days.length - 1] + 1;
    doc.days = [...doc.days, next];
    for (const name of doc.state_order) {
      doc.states[name] = [...(doc.states[name] ?? []), 0];
    }
    doc.states = { ...doc.states };
  }
  function removeDay(i: number) {
    doc.days = doc.days.filter((_, idx) => idx !== i);
    for (const name of doc.state_order) {
      doc.states[name] = (doc.states[name] ?? []).filter((_, idx) => idx !== i);
    }
    doc.states = { ...doc.states };
  }
  function addState() {
    const name = newStateName.trim();
    if (!name || doc.state_order.includes(name)) return;
    doc.state_order = [...doc.state_order, name];
    doc.states[name] = new Array(doc.days.length).fill(0);
    doc.states = { ...doc.states };
    newStateName = '';
  }
  function removeState(name: string) {
    doc.state_order = doc.state_order.filter((s) => s !== name);
    delete doc.states[name];
    doc.states = { ...doc.states };
  }
  function moveState(name: string, dir: -1 | 1) {
    const i = doc.state_order.indexOf(name);
    const j = i + dir;
    if (i < 0 || j < 0 || j >= doc.state_order.length) return;
    const next = [...doc.state_order];
    [next[i], next[j]] = [next[j], next[i]];
    doc.state_order = next;
  }
  function setCell(state: string, day: number, value: number) {
    doc.states[state] ??= new Array(doc.days.length).fill(0);
    doc.states[state][day] = value;
    doc.states = { ...doc.states };
  }
</script>

<StatsEditorShell
  headingLabel="Cumulative Flow Diagram"
  initialDoc={{ title: '', y_label: 'WIP', days: [], states: {}, state_order: [] } as CumFlowDoc}
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

    <!-- State management -->
    <section class="mb-4">
      <h3 class="text-xs font-bold uppercase tracking-widest text-slate-500 mb-2">States (bottom to top)</h3>
      <ul class="flex flex-wrap gap-2 mb-3">
        {#each doc.state_order as name, i (name)}
          <li class="flex items-center gap-1 bg-slate-950 border border-slate-800 rounded px-2 py-1 text-xs">
            <button onclick={() => moveState(name, -1)} disabled={i === 0} class="text-slate-500 hover:text-cyan-400 disabled:opacity-30" aria-label="Move down">▼</button>
            <button onclick={() => moveState(name, 1)} disabled={i === doc.state_order.length - 1} class="text-slate-500 hover:text-cyan-400 disabled:opacity-30" aria-label="Move up">▲</button>
            <span class="font-bold">{name}</span>
            <button onclick={() => removeState(name)} class="text-slate-500 hover:text-red-400 ml-1" aria-label="Remove state">×</button>
          </li>
        {/each}
      </ul>
      <form onsubmit={(e) => { e.preventDefault(); addState(); }} class="flex gap-2 max-w-md">
        <input
          bind:value={newStateName}
          placeholder="e.g. todo / doing / done"
          class="flex-1 bg-slate-950 border border-slate-800 p-2 text-sm rounded focus:border-cyan-500 outline-none"
        />
        <button class="bg-slate-800 hover:bg-slate-700 px-3 py-2 text-xs rounded">+ State</button>
      </form>
    </section>

    <!-- Daily counts grid -->
    {#if doc.state_order.length > 0}
      <div class="overflow-x-auto">
        <table class="text-sm border border-slate-800">
          <thead class="bg-slate-950">
            <tr>
              <th class="p-2 text-left text-[10px] text-slate-500 uppercase border-b border-slate-800 w-20">Day</th>
              {#each doc.state_order as name (name)}
                <th class="p-2 text-center text-[10px] text-cyan-400 uppercase border-b border-l border-slate-800 min-w-[100px]">{name}</th>
              {/each}
              <th class="p-2 border-b border-l border-slate-800 w-12"></th>
            </tr>
          </thead>
          <tbody>
            {#each doc.days as _, di}
              <tr>
                <td class="p-1 border-b border-slate-800">
                  <input type="number" bind:value={doc.days[di]} class="w-full bg-transparent text-sm px-2 py-1 focus:bg-slate-950 rounded text-right" />
                </td>
                {#each doc.state_order as name (name)}
                  <td class="p-1 border-b border-l border-slate-800">
                    <input
                      type="number"
                      value={doc.states[name]?.[di] ?? 0}
                      oninput={(e) => setCell(name, di, parseFloat((e.target as HTMLInputElement).value) || 0)}
                      class="w-full bg-transparent text-sm px-2 py-1 focus:bg-slate-950 rounded text-right"
                    />
                  </td>
                {/each}
                <td class="p-1 border-b border-l border-slate-800 text-center">
                  <button onclick={() => removeDay(di)} class="text-slate-500 hover:text-red-400 text-xs" aria-label="Remove day">×</button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
      <button onclick={addDay} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-2 rounded mt-3">+ Day</button>
    {:else}
      <p class="text-sm text-slate-500">Add at least one state to start the diagram.</p>
    {/if}
  {/snippet}
</StatsEditorShell>
