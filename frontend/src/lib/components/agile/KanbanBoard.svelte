<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // KanbanBoard renders the project's default board: N columns laid
  // out left-to-right with work-item cards in each column. Cards drag
  // between columns; the backend's MoveWorkItem persists the new
  // state + order in one round-trip.
  //
  // WIP indicators: each column header shows `count / limit`. When
  // count > limit > 0 the badge tints red.

  import { onMount, onDestroy } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import WorkItemEditor from './WorkItemEditor.svelte';

  let board = $state<AgileBoard | null>(null);
  let columns = $state<AgileColumn[]>([]);
  let items = $state<AgileWorkItem[]>([]);
  let sprints = $state<AgileSprint[]>([]);
  let wipCounts = $state<Record<string, number>>({});
  let loading = $state(true);
  let error = $state('');

  // Editor modal state.
  let editing = $state<AgileWorkItem | null>(null);

  // Drag state.
  let draggingID = $state<string | null>(null);

  onMount(async () => {
    loading = true;
    try {
      const [b, cols] = await window.go.main.App.EnsureDefaultBoard();
      board = b;
      columns = cols;
      await refresh();
    } catch (err: any) {
      error = `Could not load board: ${err}`;
    } finally {
      loading = false;
    }
  });

  async function refresh() {
    if (!board) return;
    const [its, sps, wips] = await Promise.all([
      window.go.main.App.ListWorkItems('', '', ''),
      window.go.main.App.ListSprints(),
      window.go.main.App.WIPCounts(),
    ]);
    // Items shown on the board: everything that is NOT in the backlog.
    items = (its ?? []).filter((i) => i.state !== 'backlog');
    sprints = sps ?? [];
    wipCounts = wips ?? {};
  }

  function itemsInColumn(colID: string): AgileWorkItem[] {
    return items
      .filter((i) => i.state === colID)
      .sort((a, b) => a.order_idx - b.order_idx);
  }

  function openNew() {
    if (columns.length === 0) return;
    editing = {
      id: '',
      project_id: session.project!.id,
      type: 'story',
      title: '',
      description: '',
      state: columns[0].id,
      points: 0,
      assignee: '',
      sprint_id: '',
      priority: 'medium',
      order_idx: 0,
      created_at: '',
      updated_at: '',
    };
  }

  function openExisting(item: AgileWorkItem) {
    editing = item;
  }

  function onSaved(saved: AgileWorkItem) {
    const idx = items.findIndex((i) => i.id === saved.id);
    if (idx >= 0) {
      items[idx] = saved;
    } else if (saved.state !== 'backlog') {
      items = [...items, saved];
    }
    void refresh(); // re-fetch counts
  }

  function onDeleted(id: string) {
    items = items.filter((i) => i.id !== id);
    void refresh();
  }

  // ----- Drag-and-drop -----

  function onDragStart(e: DragEvent, id: string) {
    draggingID = id;
    e.dataTransfer?.setData('text/plain', id);
  }
  function onDragOver(e: DragEvent) {
    e.preventDefault();
  }
  async function onDrop(e: DragEvent, targetCol: string) {
    e.preventDefault();
    const id = draggingID;
    draggingID = null;
    if (!id) return;
    const item = items.find((i) => i.id === id);
    if (!item || item.state === targetCol) return;

    // New order_idx: append at the end of the target column.
    const newOrder = itemsInColumn(targetCol).length;
    try {
      await window.go.main.App.MoveWorkItem(id, targetCol, newOrder);
      // Optimistic update.
      item.state = targetCol;
      item.order_idx = newOrder;
      items = [...items];
      void refresh();
    } catch (err: any) {
      error = `Move failed: ${err}`;
    }
  }

  // Priority → tint class.
  function priorityTint(p: AgilePriority): string {
    switch (p) {
      case 'urgent': return 'border-l-4 border-red-500';
      case 'high':   return 'border-l-4 border-amber-500';
      case 'medium': return 'border-l-4 border-cyan-500';
      default:       return 'border-l-4 border-slate-700';
    }
  }

  // WIP badge state.
  function wipState(col: AgileColumn): { text: string; tone: string } {
    const count = wipCounts[col.id] ?? 0;
    if (col.wip_limit <= 0) {
      return { text: String(count), tone: 'bg-slate-800 text-slate-300' };
    }
    const breached = count > col.wip_limit;
    return {
      text: `${count} / ${col.wip_limit}`,
      tone: breached ? 'bg-red-900 text-red-200' : 'bg-slate-800 text-slate-300',
    };
  }

  // No timers in this component, but follow the pattern documented
  // in AGENT.md §6 for consistency.
  onDestroy(() => {});
</script>

<div class="min-h-screen bg-slate-950 text-slate-200 flex flex-col">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <button onclick={() => goto('dashboard')} class="text-xs text-slate-400 hover:text-cyan-400">
        &larr; Dashboard
      </button>
      <h1 class="text-sm font-bold tracking-widest uppercase text-white">
        Kanban · {board?.name ?? '...'}
      </h1>
    </div>
    <div class="flex gap-2">
      <button
        onclick={() => goto('backlog')}
        class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded"
      >
        Backlog
      </button>
      <button
        onclick={openNew}
        disabled={columns.length === 0}
        class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-3 py-1 rounded"
      >
        + Work item
      </button>
    </div>
  </header>

  <main class="flex-1 overflow-x-auto p-6">
    {#if error}
      <p class="text-xs text-red-400 mb-3" role="alert">{error}</p>
    {/if}

    {#if loading}
      <p class="text-sm text-slate-500">Loading board...</p>
    {:else if columns.length === 0}
      <p class="text-sm text-slate-500">No columns on this board yet.</p>
    {:else}
      <div class="flex gap-4 min-h-[60vh]">
        {#each columns as col (col.id)}
          {@const wip = wipState(col)}
          <section
            class="flex-shrink-0 w-72 bg-slate-900 border border-slate-800 rounded-lg flex flex-col"
            ondragover={onDragOver}
            ondrop={(e) => onDrop(e, col.id)}
          >
            <header class="px-3 py-2 border-b border-slate-800 flex items-center justify-between">
              <span class="text-xs font-bold tracking-widest uppercase text-cyan-400">
                {col.name}
              </span>
              <span class="text-[10px] px-2 py-0.5 rounded {wip.tone}">{wip.text}</span>
            </header>
            <ul class="flex-1 p-2 space-y-2 overflow-y-auto">
              {#each itemsInColumn(col.id) as item (item.id)}
                <li>
                  <button
                    draggable="true"
                    ondragstart={(e) => onDragStart(e, item.id)}
                    onclick={() => openExisting(item)}
                    class="w-full text-left p-2 bg-slate-950 hover:bg-slate-800 rounded {priorityTint(item.priority)} cursor-grab active:cursor-grabbing"
                  >
                    <div class="text-sm font-bold text-white">
                      {item.title || '(untitled)'}
                    </div>
                    <div class="flex items-center justify-between mt-1">
                      <span class="text-[10px] text-slate-500 uppercase">
                        {item.type}{item.points > 0 ? ` · ${item.points}pt` : ''}
                      </span>
                      {#if item.assignee}
                        <span class="text-[10px] text-cyan-400">{item.assignee}</span>
                      {/if}
                    </div>
                  </button>
                </li>
              {/each}
              {#if itemsInColumn(col.id).length === 0}
                <li class="text-[10px] text-slate-600 text-center py-4">
                  Drag a card here, or click + Work item.
                </li>
              {/if}
            </ul>
          </section>
        {/each}
      </div>
    {/if}
  </main>
</div>

<WorkItemEditor
  item={editing}
  {sprints}
  {columns}
  onClose={() => (editing = null)}
  {onSaved}
  {onDeleted}
/>
