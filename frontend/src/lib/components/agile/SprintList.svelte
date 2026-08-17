<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // SprintList: create, edit, activate, and complete sprints. The
  // active-sprint constraint (only one at a time) is GUI-enforced —
  // when the user clicks "Start" on a planning sprint, any other
  // active sprint is auto-moved to "complete" first.

  import { onMount, onDestroy } from 'svelte';
  import { autosave } from '../../autosave.svelte';
  import { session, goto, requestNavigation } from '../../session.svelte';

  let sprints = $state<AgileSprint[]>([]);
  let workItemsBySprint = $state<Record<string, AgileWorkItem[]>>({});
  let editing = $state<AgileSprint | null>(null);
  let loading = $state(true);
  let error = $state('');
  let status = $state('');
  let saving = $state(false);

  // Snapshot of `editing` as of the last open. Compared against the live
  // value to decide whether closing needs confirmation.
  let original = $state<string | null>(null);
  let dirty = $derived(editing !== null && original !== null && JSON.stringify(editing) !== original);
  let stopDirtyGuard: (() => void) | null = null;

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
    openEditor({
      id: '',
      project_id: session.project!.id,
      name: '',
      goal: '',
      status: 'planning',
      start_date: today,
      end_date: inTwoWeeks,
      capacity: 0,
      created_at: '',
    });
  }

  function openExisting(s: AgileSprint) {
    openEditor({ ...s });
  }

  // The shared Save/Discard/Cancel guard only knows about registered drafts.
  // Tear its entry down before clearing or replacing `editing`; otherwise its
  // snapshot can briefly change to `null` and falsely block a continuation.
  function stopEditing() {
    stopDirtyGuard?.();
    stopDirtyGuard = null;
    editing = null;
    original = null;
  }

  function openEditor(next: AgileSprint) {
    stopEditing();
    editing = next;
    original = JSON.stringify(next);
    error = '';
    // A successful save closes this lightweight modal, so timed autosave
    // would dismiss it unexpectedly. It remains registered for shared
    // navigation, sign-out, project-close, and native-close decisions.
    stopDirtyGuard = autosave.register(
      () => JSON.stringify(editing),
      () => save(),
      false,
    );
  }

  function rebaseEditableChanges(
    saved: AgileSprint,
    savingSprint: AgileSprint,
    latestSprint: AgileSprint,
  ): AgileSprint {
    return {
      ...saved,
      name: latestSprint.name !== savingSprint.name ? latestSprint.name : saved.name,
      goal: latestSprint.goal !== savingSprint.goal ? latestSprint.goal : saved.goal,
      start_date: latestSprint.start_date !== savingSprint.start_date
        ? latestSprint.start_date
        : saved.start_date,
      end_date: latestSprint.end_date !== savingSprint.end_date
        ? latestSprint.end_date
        : saved.end_date,
      capacity: latestSprint.capacity !== savingSprint.capacity
        ? latestSprint.capacity
        : saved.capacity,
    };
  }

  async function save(): Promise<boolean> {
    if (!editing || saving) return false;
    if (!editing.name.trim()) {
      error = 'Sprint name is required.';
      return false;
    }
    // Persist an immutable start snapshot. Disabled controls rule out normal
    // user changes while saving, but a programmatic update must still survive
    // rather than being silently overwritten by the backend response.
    const savingSprint = { ...editing };
    const savingSnapshot = JSON.stringify(savingSprint);
    saving = true;
    error = '';
    try {
      const saved = await window.go.main.App.SaveSprint(savingSprint);
      const idx = sprints.findIndex((s) => s.id === saved.id);
      if (idx >= 0) sprints[idx] = saved;
      else sprints = [saved, ...sprints];
      const latestSprint = editing;
      original = JSON.stringify(saved);
      if (latestSprint && JSON.stringify(latestSprint) !== savingSnapshot) {
        editing = rebaseEditableChanges(saved, savingSprint, latestSprint);
        return true;
      }
      stopEditing();
      status = 'Sprint saved.';
      return true;
    } catch (err: any) {
      error = `Save failed: ${err}`;
      return false;
    } finally {
      saving = false;
    }
  }

  // Every path that discards `editing` (header close, Cancel, Escape,
  // backdrop click) must route through here rather than assigning
  // `editing = null` directly, so an edited-but-unsaved sprint is never
  // dropped silently. No-ops while a save is in flight so a close
  // requested mid-save can't confirm a "discard" for edits the in-flight
  // request is about to persist anyway.
  // Routes through the shared Save/Discard/Cancel guard (the same modal the
  // native-close guard uses) rather than `confirm()`: Wails v2.13.0's darwin
  // WKUIDelegate (WailsContext, in
  // github.com/wailsapp/wails/v2@v2.13.0/internal/frontend/desktop/darwin/
  // WailsContext.m) declares conformance to WKUIDelegate but implements none
  // of the JS confirm/alert/prompt panel methods (verified: no
  // `runJavaScript*Panel` implementation anywhere in the vendored module).
  // Observed result in a packaged build: `confirm()` produces no dialog and
  // the close silently no-ops (2026-08-16 GUI evidence) — the exact value
  // WebKit's `confirm()` returns in that case was not independently
  // confirmed, only that no dialog appears.
  function requestClose() {
    if (saving) return;
    if (!dirty) {
      stopEditing();
      return;
    }
    requestNavigation(session.view, session.editingId, async () => {
      stopEditing();
      return true;
    });
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

  onDestroy(() => stopDirtyGuard?.());
</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <button onclick={() => goto('dashboard')} class="text-xs text-slate-400 hover:text-cyan-400">
        &larr; Dashboard
      </button>
      <h1 class="text-sm font-bold tracking-widest uppercase text-slate-50">Sprints</h1>
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
                  <span class="font-bold text-slate-50">{s.name || '(untitled sprint)'}</span>
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
      if ((e.target as HTMLElement).dataset?.role === 'backdrop') requestClose();
    }}
    onkeydown={(e) => { if (e.key === 'Escape') requestClose(); }}
    tabindex="-1"
  >
    <div data-role="backdrop" class="absolute inset-0"></div>
    <div class="relative w-full max-w-lg bg-slate-900 border border-slate-700 rounded-xl shadow-2xl">
      <header class="px-5 py-3 border-b border-slate-800 flex items-center justify-between">
        <h2 class="text-sm font-bold tracking-widest uppercase text-slate-50">
          {editing.id ? 'Edit sprint' : 'New sprint'}
        </h2>
        <button
          onclick={requestClose}
          disabled={saving}
          class="text-slate-500 hover:text-slate-200"
          aria-label="Close"
        >×</button>
      </header>
      <div class="p-5 space-y-3">
        <label class="block">
          <span class="text-xs text-slate-500 uppercase">Name</span>
          <input
            bind:value={editing.name}
            disabled={saving}
            placeholder="e.g. Sprint 14"
            class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          />
        </label>
        <label class="block">
          <span class="text-xs text-slate-500 uppercase">Goal</span>
          <textarea
            bind:value={editing.goal}
            disabled={saving}
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
              disabled={saving}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded"
            />
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">End date</span>
            <input
              type="date"
              bind:value={editing.end_date}
              disabled={saving}
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
            disabled={saving}
            class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          />
        </label>
      </div>
      <footer class="px-5 py-3 border-t border-slate-800 flex justify-end gap-2">
        <button
          onclick={requestClose}
          disabled={saving}
          class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded"
        >
          Cancel
        </button>
        <button
          onclick={save}
          disabled={saving || !editing.name.trim()}
          class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-3 py-1 rounded"
        >
          {saving ? 'Saving...' : 'Save'}
        </button>
      </footer>
    </div>
  </div>
{/if}
