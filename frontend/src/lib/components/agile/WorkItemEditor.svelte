<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // WorkItemEditor is a modal-style panel for editing one work item.
  // The parent owns the open/close state and passes the work item
  // (or null for "create new") via the `item` prop. When the user
  // saves or deletes, the parent gets notified via callback props.
  //
  // Used by KanbanBoard and Backlog. The component does NOT
  // self-fetch; the parent supplies an in-memory work item so
  // unsaved edits don't disappear if the user toggles columns.

  import { onDestroy } from 'svelte';

  type Status = 'idle' | 'saving' | 'deleting';

  // Props (Svelte 5 runes)
  let {
    item,
    sprints = [],
    columns = [],
    onClose,
    onSaved,
    onDeleted,
  }: {
    item: AgileWorkItem | null;
    sprints?: AgileSprint[];
    columns?: AgileColumn[];
    onClose: () => void;
    onSaved: (saved: AgileWorkItem) => void;
    onDeleted: (id: string) => void;
  } = $props();

  // Local mutable copy so the parent's reference isn't mutated until
  // save succeeds. `null` means the modal is closed.
  let draft = $state<AgileWorkItem | null>(null);
  let status = $state<Status>('idle');
  let error = $state('');

  // Re-seed the draft when `item` changes from null → record (modal
  // opens) or to a different record. Skip when only inner fields
  // changed (parent's optimistic update).
  let lastItemID: string | null = null;
  $effect(() => {
    if (!item) {
      draft = null;
      lastItemID = null;
      return;
    }
    if (item.id !== lastItemID) {
      draft = { ...item };
      lastItemID = item.id;
      error = '';
    }
  });

  async function save() {
    if (!draft) return;
    status = 'saving';
    error = '';
    try {
      const saved = await window.go.main.App.SaveWorkItem(draft);
      onSaved(saved);
      onClose();
    } catch (err: any) {
      error = String(err?.message ?? err);
    } finally {
      status = 'idle';
    }
  }

  async function destroy() {
    if (!draft) return;
    if (!confirm(`Delete "${draft.title}"?`)) return;
    status = 'deleting';
    try {
      await window.go.main.App.DeleteWorkItem(draft.id);
      onDeleted(draft.id);
      onClose();
    } catch (err: any) {
      error = String(err?.message ?? err);
    } finally {
      status = 'idle';
    }
  }

  function onBackdropClick(e: MouseEvent) {
    // Only close when the click is on the backdrop itself, not the
    // panel. The check is by ID rather than target/currentTarget
    // identity so React-style nested events work.
    if ((e.target as HTMLElement).dataset.role === 'backdrop') {
      onClose();
    }
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      onClose();
    } else if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
      void save();
    }
  }

  // Concurrency hardening: no timers in this component, but the
  // keydown listener is attached via the modal's `tabindex` element
  // and unmounted by Svelte automatically.
  onDestroy(() => {
    // No-op; the modal has no timers or external listeners.
  });

  const TYPES: AgileWorkItemType[] = ['story', 'bug', 'task', 'epic'];
  const PRIOS: AgilePriority[] = ['low', 'medium', 'high', 'urgent'];
</script>

{#if draft}
  <div
    data-role="backdrop"
    class="fixed inset-0 bg-black/60 z-40 flex items-center justify-center p-6"
    onclick={onBackdropClick}
    onkeydown={onKey}
    role="dialog"
    aria-modal="true"
    aria-label="Edit work item"
    tabindex="-1"
  >
    <div class="w-full max-w-2xl bg-slate-900 border border-slate-700 rounded-xl shadow-2xl overflow-hidden">
      <header class="px-6 py-3 border-b border-slate-800 flex items-center justify-between">
        <h2 class="text-sm font-bold tracking-widest uppercase text-white">
          {draft.id ? 'Edit work item' : 'New work item'}
        </h2>
        <button
          onclick={onClose}
          class="text-slate-500 hover:text-slate-200"
          aria-label="Close"
        >
          ×
        </button>
      </header>

      <main class="p-6 space-y-4 max-h-[70vh] overflow-y-auto">
        {#if error}
          <p class="text-xs text-red-400" role="alert">{error}</p>
        {/if}

        <label class="block">
          <span class="text-xs text-slate-500 uppercase">Title</span>
          <input
            bind:value={draft.title}
            class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          />
        </label>

        <div class="grid grid-cols-2 gap-3">
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Type</span>
            <select
              bind:value={draft.type}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded"
            >
              {#each TYPES as t (t)}
                <option value={t}>{t}</option>
              {/each}
            </select>
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Priority</span>
            <select
              bind:value={draft.priority}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded"
            >
              {#each PRIOS as p (p)}
                <option value={p}>{p}</option>
              {/each}
            </select>
          </label>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Points</span>
            <input
              type="number"
              step="0.5"
              bind:value={draft.points}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Assignee</span>
            <input
              bind:value={draft.assignee}
              placeholder="(unassigned)"
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">State / Column</span>
            <select
              bind:value={draft.state}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded"
            >
              <option value="backlog">backlog</option>
              {#each columns as c (c.id)}
                <option value={c.id}>{c.name}</option>
              {/each}
            </select>
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Sprint</span>
            <select
              bind:value={draft.sprint_id}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded"
            >
              <option value="">(none)</option>
              {#each sprints as s (s.id)}
                <option value={s.id}>{s.name}</option>
              {/each}
            </select>
          </label>
        </div>

        <label class="block">
          <span class="text-xs text-slate-500 uppercase">Description</span>
          <textarea
            bind:value={draft.description}
            rows="5"
            class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          ></textarea>
        </label>

        {#if draft.id}
          <p class="text-[10px] text-slate-500">
            ID: <span class="font-mono">{draft.id}</span>
            {#if draft.closed_at}· closed at {draft.closed_at.slice(0, 10)}{/if}
          </p>
        {/if}
      </main>

      <footer class="px-6 py-3 border-t border-slate-800 flex items-center justify-between">
        <button
          onclick={destroy}
          disabled={status !== 'idle' || !draft.id}
          class="text-xs text-red-400 hover:text-red-300 disabled:opacity-30"
        >
          Delete
        </button>
        <div class="flex gap-2">
          <button
            onclick={onClose}
            class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded"
          >
            Cancel
          </button>
          <button
            onclick={save}
            disabled={status !== 'idle' || !draft.title}
            class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-3 py-1 rounded"
          >
            {status === 'saving' ? 'Saving...' : 'Save'}
          </button>
        </div>
      </footer>
    </div>
  </div>
{/if}
