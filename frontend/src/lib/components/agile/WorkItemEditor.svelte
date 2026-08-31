<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
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

  import { onDestroy, untrack } from 'svelte';
  import { autosave } from '../../autosave.svelte';
  import { session, requestNavigation } from '../../session.svelte';
  import { rebaseEditableChanges } from '../../rebase-editable-changes';
  import ConfirmDialog from '../ConfirmDialog.svelte';

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

  // Snapshot of `draft` as of the last load or successful save. Compared
  // against the live draft to decide whether closing needs confirmation.
  let original = $state<string | null>(null);
  let dirty = $derived(draft !== null && original !== null && JSON.stringify(draft) !== original);

  // Re-seed the draft and its guard when `item` changes from null → record
  // (modal opens) or to a different record. This component stays mounted
  // while its modal opens and closes, so a component-lifetime registration
  // would retain the previous item's baseline and report a false dirty state.
  // Skip when only inner fields changed (parent's optimistic update).
  let lastItemID: string | null = null;
  let stopDirtyGuard: (() => void) | null = null;
  $effect(() => {
    const currentItem = item;
    if (!currentItem) {
      stopDirtyGuard?.();
      stopDirtyGuard = null;
      draft = null;
      original = null;
      lastItemID = null;
      return;
    }
    if (currentItem.id !== lastItemID) {
      stopDirtyGuard?.();
      stopDirtyGuard = null;
      const nextDraft = untrack(() => ({ ...currentItem }));
      draft = nextDraft;
      original = JSON.stringify(nextDraft);
      lastItemID = currentItem.id;
      error = '';
      // register() snapshots immediately. Keep that read untracked so typing
      // into the draft does not tear down and recreate the per-item guard.
      stopDirtyGuard = untrack(() => autosave.register(
        () => JSON.stringify(draft),
        () => save(),
        false,
      ));
    }
  });

  // Fields the backend owns/mutates independently of this form -- always
  // taken from the server response, never diffed against a stale draft
  // copy. order_idx is drag-drop-managed by KanbanBoard/Backlog; closed_at
  // is server-stamped on a state transition to done, never form-written.
  const WORK_ITEM_BACKEND_OWNED_KEYS: readonly (keyof AgileWorkItem)[] = [
    'id',
    'project_id',
    'order_idx',
    'created_at',
    'updated_at',
    'closed_at',
  ];

  async function save(): Promise<boolean> {
    if (!draft || status !== 'idle') return false;
    if (!draft.title.trim()) {
      error = 'Title is required.';
      return false;
    }
    // Persist a stable copy. The editable controls are disabled while this is
    // in flight, but the snapshot keeps this boundary correct if a future
    // caller changes draft state programmatically during the await.
    const savingDraft = { ...draft };
    const savingSnapshot = JSON.stringify(savingDraft);
    status = 'saving';
    error = '';
    try {
      const saved = await window.go.main.App.SaveWorkItem(savingDraft);
      // Mark clean before the callbacks run: `onSaved` can trigger a
      // parent refresh that awaits before this modal actually unmounts,
      // and a stale `original` would make a post-save `requestClose`
      // wrongly prompt to discard.
      const latestDraft = draft;
      original = JSON.stringify(saved);
      if (latestDraft && JSON.stringify(latestDraft) !== savingSnapshot) {
        // Keep a late edit open, but retain backend-owned fields such as a
        // newly assigned ID and timestamps from the successful save.
        draft = rebaseEditableChanges(saved, savingDraft, latestDraft, WORK_ITEM_BACKEND_OWNED_KEYS);
        onSaved(saved);
        return true;
      }
      onSaved(saved);
      onClose();
      return true;
    } catch (err: any) {
      error = String(err?.message ?? err);
      return false;
    } finally {
      status = 'idle';
    }
  }

  // Every path that discards the draft (header close, Cancel, Escape,
  // backdrop click) must route through here rather than calling `onClose`
  // directly, so an edited-but-unsaved work item is never dropped silently.
  // Ignored while a save/delete is in flight — same as the Save/Delete
  // buttons' own `disabled` guard — so a close requested mid-save can't
  // discard a draft the in-flight request is about to persist anyway.
  // Routes through the shared Save/Discard/Cancel guard (the same modal the
  // native-close guard uses) rather than `confirm()`: Wails v2.15.0's darwin
  // WKUIDelegate (WailsContext, in
  // github.com/wailsapp/wails/v2@v2.15.0/internal/frontend/desktop/darwin/
  // WailsContext.m) declares conformance to WKUIDelegate but implements none
  // of the JS confirm/alert/prompt panel methods (verified: no
  // `runJavaScript*Panel` implementation anywhere in the vendored module).
  // Observed result in a packaged build: `confirm()` produces no dialog and
  // the close silently no-ops (2026-08-16 GUI evidence) — the exact value
  // WebKit's `confirm()` returns in that case was not independently
  // confirmed, only that no dialog appears.
  function requestClose() {
    if (status !== 'idle') return;
    if (!dirty) {
      onClose();
      return;
    }
    requestNavigation(session.view, session.editingId, async () => {
      onClose();
      return true;
    });
  }

  // Deleting used `confirm()` — silently a no-op in the packaged macOS
  // build, since Wails v2.15.0's darwin WKUIDelegate implements none of the
  // JS confirm/alert/prompt panel methods (verified: no `runJavaScript*Panel`
  // implementation anywhere in the vendored module). Now routed through the
  // shared ConfirmDialog (a real DOM modal) instead, matching the fix
  // already applied to this file's unsaved-changes close guard.
  let confirmingDelete = $state(false);

  function requestDelete() {
    if (!draft || status !== 'idle' || !draft.id) return;
    confirmingDelete = true;
  }

  function cancelDelete() {
    confirmingDelete = false;
  }

  async function destroy() {
    if (!draft || status !== 'idle' || !draft.id) return;
    const deletingID = draft.id;
    confirmingDelete = false;
    status = 'deleting';
    try {
      await window.go.main.App.DeleteWorkItem(deletingID);
      onDeleted(deletingID);
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
      requestClose();
    }
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      requestClose();
    } else if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
      void save();
    }
  }

  onDestroy(() => stopDirtyGuard?.());

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
        <h2 class="text-sm font-bold tracking-widest uppercase text-slate-50">
          {draft.id ? 'Edit work item' : 'New work item'}
        </h2>
        <button
          onclick={requestClose}
          disabled={status !== 'idle'}
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
            disabled={status !== 'idle'}
            class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          />
        </label>

        <div class="grid grid-cols-2 gap-3">
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Type</span>
            <select
              bind:value={draft.type}
              disabled={status !== 'idle'}
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
              disabled={status !== 'idle'}
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
              disabled={status !== 'idle'}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Assignee</span>
            <input
              bind:value={draft.assignee}
              placeholder="(unassigned)"
              disabled={status !== 'idle'}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">State / Column</span>
            <select
              bind:value={draft.state}
              disabled={status !== 'idle'}
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
              disabled={status !== 'idle'}
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
            disabled={status !== 'idle'}
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
          onclick={requestDelete}
          disabled={status !== 'idle' || !draft.id}
          class="text-xs text-red-400 hover:text-red-300 disabled:opacity-30"
        >
          Delete
        </button>
        <div class="flex gap-2">
          <button
            onclick={requestClose}
            disabled={status !== 'idle'}
            class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded"
          >
            Cancel
          </button>
          <button
            onclick={save}
            disabled={status !== 'idle' || !draft.title.trim()}
            class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-3 py-1 rounded"
          >
            {status === 'saving' ? 'Saving...' : 'Save'}
          </button>
        </div>
      </footer>
    </div>
  </div>
{/if}

<ConfirmDialog
  open={confirmingDelete}
  title="Delete work item"
  message={`Delete "${draft?.title ?? ''}"? This cannot be undone.`}
  busy={status === 'deleting'}
  onConfirm={destroy}
  onCancel={cancelDelete}
/>
