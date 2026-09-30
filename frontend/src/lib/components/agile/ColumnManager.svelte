<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import ConfirmDialog from '../ConfirmDialog.svelte';

  // Rename, reorder, add, and set WIP limits on the board's columns, then
  // save them together; delete an empty column the user added. Every save
  // renumbers the columns 0..n, so a save that stops partway is put right by
  // the next one. The built-in columns come back whenever they are missing
  // (agile.DefaultColumns) and "done" feeds DORA, so the backend refuses to
  // delete them; they are marked here so the button is not offered.
  const BUILT_IN = new Set(['todo', 'doing', 'review', 'done']);

  let {
    boardId,
    columns,
    itemCounts,
    onchange,
    onclose,
  }: {
    boardId: string;
    columns: AgileColumn[];
    itemCounts: Record<string, number>;
    onchange: (columns: AgileColumn[]) => void;
    onclose: () => void;
  } = $props();

  type Row = { id: string; name: string; wip_limit: number };
  const toRows = (cols: AgileColumn[]): Row[] =>
    cols.map((c) => ({ id: c.id, name: c.name, wip_limit: c.wip_limit }));

  // The draft starts from the columns shown when the panel opens and is
  // rebuilt from the backend after every save or delete.
  let rows = $state<Row[]>([]);
  let saved = $state<Row[]>([]);
  onMount(() => {
    rows = toRows(columns);
    saved = toRows(columns);
  });
  let busy = $state(false);
  let error = $state('');
  let pendingDelete = $state<Row | null>(null);

  const dirty = $derived(JSON.stringify(rows) !== JSON.stringify(saved));

  function move(i: number, by: -1 | 1) {
    const j = i + by;
    if (j < 0 || j >= rows.length) return;
    const next = [...rows];
    [next[i], next[j]] = [next[j], next[i]];
    rows = next;
  }

  function addColumn() {
    rows = [...rows, { id: '', name: '', wip_limit: 0 }];
  }

  async function reload() {
    const res = await window.go.main.App.EnsureDefaultBoard();
    rows = toRows(res.columns);
    saved = toRows(res.columns);
    onchange(res.columns);
  }

  async function save() {
    if (busy) return;
    error = '';
    if (rows.some((r) => !r.name.trim())) {
      error = 'Give every column a name.';
      return;
    }
    if (rows.some((r) => !Number.isInteger(r.wip_limit) || r.wip_limit < 0)) {
      error = 'A WIP limit must be a whole number, 0 for no limit.';
      return;
    }
    busy = true;
    try {
      for (const [i, r] of rows.entries()) {
        await window.go.main.App.SaveColumn({
          id: r.id,
          board_id: boardId,
          name: r.name.trim(),
          order_idx: i,
          wip_limit: r.wip_limit,
        });
      }
    } catch (err: any) {
      error = String(err?.message ?? err);
    }
    try {
      await reload();
    } catch (err: any) {
      error ||= `Could not reload the columns: ${err?.message ?? err}`;
    } finally {
      busy = false;
    }
  }

  async function confirmDelete() {
    if (!pendingDelete || busy) return;
    busy = true;
    error = '';
    try {
      await window.go.main.App.DeleteColumn(pendingDelete.id);
      await reload();
    } catch (err: any) {
      error = String(err?.message ?? err);
    } finally {
      pendingDelete = null;
      busy = false;
    }
  }

  function deleteBlockedReason(r: Row): string {
    if (BUILT_IN.has(r.id)) return 'Built-in columns can be renamed but not deleted.';
    const n = itemCounts[r.id] ?? 0;
    if (n > 0) return `Move its ${n === 1 ? '1 work item' : `${n} work items`} out first.`;
    if (dirty) return 'Save or discard your changes first.';
    return '';
  }
</script>

<section
  aria-labelledby="column-manager-heading"
  class="mx-6 mt-4 border border-slate-800 bg-slate-900 rounded-lg p-4 space-y-3"
>
  <div class="flex items-center justify-between">
    <h2 id="column-manager-heading" class="text-xs font-bold tracking-widest uppercase text-cyan-400">Columns</h2>
    <button type="button" onclick={onclose} class="text-xs text-slate-400 hover:text-slate-200 underline">Close</button>
  </div>
  <p class="text-[11px] text-slate-500">
    Cards sit in columns from left to right. A WIP limit of 0 means no limit.
  </p>

  <ol class="space-y-2">
    {#each rows as r, i (i)}
      {@const reason = r.id ? deleteBlockedReason(r) : ''}
      <li class="flex flex-wrap items-center gap-2">
        <label class="sr-only" for={`col-name-${i}`}>Column {i + 1} name</label>
        <input
          id={`col-name-${i}`}
          bind:value={r.name}
          placeholder="Column name"
          class="flex-1 min-w-40 bg-slate-950 border border-slate-800 px-2 py-1 rounded text-sm focus:border-cyan-500 outline-none"
        />
        <label class="text-[11px] text-slate-500 flex items-center gap-1">
          WIP
          <input
            type="number"
            min="0"
            step="1"
            bind:value={r.wip_limit}
            aria-label={`Column ${i + 1} WIP limit`}
            class="w-16 bg-slate-950 border border-slate-800 px-2 py-1 rounded text-sm focus:border-cyan-500 outline-none"
          />
        </label>
        <button type="button" onclick={() => move(i, -1)} disabled={i === 0} aria-label={`Move column ${i + 1} left`} class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-40 px-2 py-1 rounded">←</button>
        <button type="button" onclick={() => move(i, 1)} disabled={i === rows.length - 1} aria-label={`Move column ${i + 1} right`} class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-40 px-2 py-1 rounded">→</button>
        {#if !r.id}
          <button type="button" onclick={() => (rows = rows.filter((_, k) => k !== i))} class="text-xs text-slate-400 hover:text-slate-200 underline">Remove</button>
        {:else if BUILT_IN.has(r.id)}
          <span class="text-[11px] text-slate-500" title={reason}>Built-in</span>
        {:else}
          <button
            type="button"
            onclick={() => (pendingDelete = r)}
            disabled={reason !== '' || busy}
            title={reason || undefined}
            aria-label={`Delete column ${r.name}`}
            class="text-xs text-red-400 hover:text-red-300 disabled:opacity-40 underline"
          >Delete</button>
        {/if}
      </li>
    {/each}
  </ol>

  <div class="flex flex-wrap items-center gap-2">
    <button type="button" onclick={addColumn} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded">+ Column</button>
    <button
      type="button"
      onclick={save}
      disabled={!dirty || busy}
      class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold px-3 py-1 rounded"
    >{busy ? 'Saving…' : 'Save columns'}</button>
    <button
      type="button"
      onclick={() => (rows = saved.map((r) => ({ ...r })))}
      disabled={!dirty || busy}
      class="text-xs text-slate-400 hover:text-slate-200 disabled:opacity-40 underline"
    >Discard changes</button>
  </div>

  {#if error}
    <p class="text-xs text-red-400" role="alert">{error}</p>
  {/if}
</section>

<ConfirmDialog
  open={pendingDelete !== null}
  title="Delete this column?"
  message={pendingDelete ? `The empty column "${pendingDelete.name}" will be removed from the board.` : ''}
  confirmLabel="Delete column"
  cancelLabel="Keep column"
  busy={busy}
  onConfirm={confirmDelete}
  onCancel={() => (pendingDelete = null)}
/>
