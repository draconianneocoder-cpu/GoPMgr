<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // SprintList: create, edit, activate, and complete sprints. The
  // active-sprint constraint (only one at a time) is GUI-enforced —
  // when the user clicks "Start" on a planning sprint, any other
  // active sprint is auto-moved to "complete" first.

  import { onMount, onDestroy } from 'svelte';
  import { session, goto } from '../../session.svelte';

  let sprints = $state<AgileSprint[]>([]);
  let workItemsBySprint = $state<Record<string, AgileWorkItem[]>>({});
  let editing = $state<AgileSprint | null>(null);
  let loading = $state(true);
  let error = $state('');
  let status = $state('');

  onMount(async () => {
    await refresh();
    loading = false;
  });

  async function refresh() {
    try {
      const list = await window.go.main.App.ListSprints();
      sprints = list ?? [];

      // Index work items by sprint so we can show counts + sums.
      const all = (await window.go.main.App.ListWorkItems('', '', '')) ?? [];
      const map: Record<string, AgileWorkItem[]> = {};
      for (const i of all) {
        if (!i.sprint_id) continue;
        (map[i.sprint_id] ??= []).push(i);
      }
      workItemsBySprint = map;
    } catch (err: any) {
      error = `Could not load sprints: ${err}`;
    }
  }

  function openNew() {
    const today = new Date().toISOString().slice(0, 10);
    const inTwoWeeks = new Date(Date.now() + 14 * 86400 * 1000)
      .toISOString()
      .slice(0, 10);
    editing = {
      id: '',
      project_id: session.project!.id,
      name: '',
      goal: '',
      status: 'planning',
      start_date: today,
      end_date: inTwoWeeks,
      capacity: 0,
      created_at: '',
    };
  }

  function openExisting(s: AgileSprint) {
    editing = { ...s };
  }

  async function save() {
    if (!editing) return;
    try {
      const saved = await window.go.main.App.SaveSprint(editing);
      const idx = sprints.findIndex((s) => s.id === saved.id);
      if (idx >= 0) sprints[idx] = saved;
      else sprints = [saved, ...sprints];
      editing = null;
      status = 'Sprint saved.';
    } catch (err: any) {
      error = `Save failed: ${err}`;
    }
  }

  async function activate(s: AgileSprint) {
    // Move any currently-active sprint to "complete" first, so the
    // invariant "at most one active sprint" holds.
    try {
      for (const other of sprints) {
        if (other.id !== s.id && other.status === 'active') {
          await window.go.main.App.SaveSprint({ ...other, status: 'complete' });
        }
      }
      const saved = await window.go.main.App.SaveSprint({ ...s, status: 'active' });
      await refresh();
      status = `${saved.name} is now active.`;
    } catch (err: any) {
      error = `Activate failed: ${err}`;
    }
  }

  async function complete(s: AgileSprint) {
    if (!confirm(`Complete sprint "${s.name}"?`)) return;
    try {
      await window.go.main.App.SaveSprint({ ...s, status: 'complete' });
      await refresh();
    } catch (err: any) {
      error = `Complete failed: ${err}`;
    }
  }

  async function destroy(s: AgileSprint) {
    if (!confirm(`Delete sprint "${s.name}"? Work items will return to the backlog (sprint link only).`)) return;
    try {
      await window.go.main.App.DeleteSprint(s.id);
      await refresh();
    } catch (err: any) {
      error = `Delete failed: ${err}`;
    }
  }

  // Status → tint class for the badge.
  function statusTint(s: AgileSprintStatus): string {
    switch (s) {
      case 'active':   return 'bg-emerald-900 text-emerald-200';
      case 'complete': return 'bg-slate-700 text-slate-300';
      default:         return 'bg-cyan-900 text-cyan-200';
    }
  }

  // Sum of committed points for a sprint (read-only).
  function committedPoints(sprintID: string): number {
    return (workItemsBySprint[sprintID] ?? []).reduce(
      (sum, i) => sum + (i.points || 0),
      0,
    );
  }

  function doneItems(sprintID: string): { done: number; total: number } {
    const items = workItemsBySprint[sprintID] ?? [];
    const done = items.filter((i) => i.state === 'done').length;
    return { done, total: items.length };
  }

  // No timers in this component, but the pattern from AGENT.md §6
  // applies to anything we might add later.
  onDestroy(() => {});
</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <button onclick={() => goto('dashboard')} class="text-xs text-slate-400 hover:text-cyan-400">
        &larr; Dashboard
      </button>
      <h1 class="text-sm font-bold tracking-widest uppercase text-white">Sprints</h1>
    </div>
    <button
      onclick={openNew}
      class="text-xs bg-cyan-600 hover:bg-cyan-500 text-white font-bold uppercase px-3 py-1 rounded"
    >
      + Sprint
    </button>
  </header>

  <main class="p-6 max-w-4xl mx-auto space-y-4">
    {#if status}
      <p class="text-xs text-cyan-400">{status}</p>
    {/if}
    {#if error}
      <p class="text-xs text-red-400" role="alert">{error}</p>
    {/if}

    {#if loading}
      <p class="text-sm text-slate-500">Loading sprints...</p>
    {:else if sprints.length === 0}
      <p class="text-sm text-slate-500 text-center py-12">
        No sprints yet. Click <strong>+ Sprint</strong> to start one.
      </p>
    {:else}
      <ul class="space-y-3">
        {#each sprints as s (s.id)}
          {@const dp = doneItems(s.id)}
          {@const cp = committedPoints(s.id)}
          <li class="p-4 bg-slate-900 border border-slate-800 rounded">
            <div class="flex items-start justify-between gap-3">
              <button onclick={() => openExisting(s)} class="flex-1 text-left min-w-0">
                <div class="flex items-center gap-2">
                  <span class="font-bold text-white">{s.name || '(untitled sprint)'}</span>
                  <span class="text-[10px] px-2 py-0.5 rounded uppercase tracking-widest {statusTint(s.status)}">
                    {s.status}
                  </span>
                </div>
                {#if s.goal}
                  <p class="text-xs text-slate-400 mt-1">{s.goal}</p>
                {/if}
                <p class="text-[10px] text-slate-500 mt-1 uppercase tracking-widest">
                  {s.start_date || '?'} → {s.end_date || '?'} ·
                  {dp.done}/{dp.total} done ·
                  {cp.toFixed(1)}{s.capacity > 0 ? ` / ${s.capacity.toFixed(1)}` : ''} pts
                </p>
              </button>
              <div class="flex flex-col gap-1">
                {#if s.status === 'planning'}
                  <button
                    onclick={() => activate(s)}
                    class="text-xs bg-emerald-700 hover:bg-emerald-600 px-3 py-1 rounded"
                  >
                    Start
                  </button>
                {:else if s.status === 'active'}
                  <button
                    onclick={() => complete(s)}
                    class="text-xs bg-slate-700 hover:bg-slate-600 px-3 py-1 rounded"
                  >
                    Complete
                  </button>
                {/if}
                <button
                  onclick={() => destroy(s)}
                  class="text-xs text-slate-500 hover:text-red-400"
                >
                  Delete
                </button>
              </div>
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  </main>
</div>

<!-- Inline editor modal (lightweight; sprints have few fields) -->
{#if editing}
  <div
    class="fixed inset-0 bg-black/60 z-40 flex items-center justify-center p-6"
    role="dialog"
    aria-modal="true"
    aria-label="Edit sprint"
    onclick={(e) => {
      if ((e.target as HTMLElement).dataset?.role === 'backdrop') editing = null;
    }}
    onkeydown={(e) => { if (e.key === 'Escape') editing = null; }}
    tabindex="-1"
  >
    <div data-role="backdrop" class="absolute inset-0"></div>
    <div class="relative w-full max-w-lg bg-slate-900 border border-slate-700 rounded-xl shadow-2xl">
      <header class="px-5 py-3 border-b border-slate-800 flex items-center justify-between">
        <h2 class="text-sm font-bold tracking-widest uppercase text-white">
          {editing.id ? 'Edit sprint' : 'New sprint'}
        </h2>
        <button onclick={() => (editing = null)} class="text-slate-500 hover:text-slate-200">×</button>
      </header>
      <div class="p-5 space-y-3">
        <label class="block">
          <span class="text-xs text-slate-500 uppercase">Name</span>
          <input
            bind:value={editing.name}
            placeholder="e.g. Sprint 14"
            class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          />
        </label>
        <label class="block">
          <span class="text-xs text-slate-500 uppercase">Goal</span>
          <textarea
            bind:value={editing.goal}
            rows="2"
            class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          ></textarea>
        </label>
        <div class="grid grid-cols-2 gap-3">
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Start date</span>
            <input
              type="date"
              bind:value={editing.start_date}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded"
            />
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">End date</span>
            <input
              type="date"
              bind:value={editing.end_date}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded"
            />
          </label>
        </div>
        <label class="block">
          <span class="text-xs text-slate-500 uppercase">Capacity (story points)</span>
          <input
            type="number"
            step="0.5"
            bind:value={editing.capacity}
            class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          />
        </label>
      </div>
      <footer class="px-5 py-3 border-t border-slate-800 flex justify-end gap-2">
        <button
          onclick={() => (editing = null)}
          class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded"
        >
          Cancel
        </button>
        <button
          onclick={save}
          disabled={!editing.name}
          class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-3 py-1 rounded"
        >
          Save
        </button>
      </footer>
    </div>
  </div>
{/if}
