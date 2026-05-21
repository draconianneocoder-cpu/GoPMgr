<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Backlog: every work item with state === 'backlog', shown as a
  // prioritized list. The user can:
  //   - create new items
  //   - drag items up/down to reorder priority
  //   - assign to a sprint (sets sprint_id, leaves state in backlog
  //     until the user starts work — convention matches Jira/Linear)
  //   - "Start work" → moves to the first column of the default board
  //   - click any item to open WorkItemEditor

  import { onMount, onDestroy } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import WorkItemEditor from './WorkItemEditor.svelte';

  let items = $state<AgileWorkItem[]>([]);
  let sprints = $state<AgileSprint[]>([]);
  let columns = $state<AgileColumn[]>([]);
  let loading = $state(true);
  let error = $state('');
  let editing = $state<AgileWorkItem | null>(null);

  // Persistence-debounced reorder: when the user drags, the in-memory
  // order updates immediately; we batch-save order_idx changes after
  // a short pause so quick drags don't fire N round-trips.
  let reorderTimer: ReturnType<typeof setTimeout> | null = null;

  onMount(async () => {
    loading = true;
    try {
      // Use EnsureDefaultBoard to learn the columns; we need the
      // first column ID for "Start work".
      const [, cols] = await window.go.main.App.EnsureDefaultBoard();
      columns = cols;
      await refresh();
    } catch (err: any) {
      error = `Could not load backlog: ${err}`;
    } finally {
      loading = false;
    }
  });

  async function refresh() {
    const [its, sps] = await Promise.all([
      window.go.main.App.ListWorkItems('', 'backlog', ''),
      window.go.main.App.ListSprints(),
    ]);
    items = (its ?? []).sort((a, b) => a.order_idx - b.order_idx);
    sprints = sps ?? [];
  }

  function openNew() {
    editing = {
      id: '',
      project_id: session.project!.id,
      type: 'story',
      title: '',
      description: '',
      state: 'backlog',
      points: 0,
      assignee: '',
      sprint_id: '',
      priority: 'medium',
      order_idx: items.length,
      created_at: '',
      updated_at: '',
    };
  }

  function openExisting(it: AgileWorkItem) {
    editing = it;
  }

  function onSaved(saved: AgileWorkItem) {
    if (saved.state !== 'backlog') {
      // The user moved the item out of the backlog from the editor;
      // drop it from this view.
      items = items.filter((i) => i.id !== saved.id);
      return;
    }
    const idx = items.findIndex((i) => i.id === saved.id);
    if (idx >= 0) items[idx] = saved;
    else items = [...items, saved];
  }

  function onDeleted(id: string) {
    items = items.filter((i) => i.id !== id);
  }

  async function assignSprint(item: AgileWorkItem, sprintID: string) {
    try {
      const saved = await window.go.main.App.SaveWorkItem({ ...item, sprint_id: sprintID });
      const idx = items.findIndex((i) => i.id === saved.id);
      if (idx >= 0) items[idx] = saved;
    } catch (err: any) {
      error = `Sprint assignment failed: ${err}`;
    }
  }

  async function startWork(item: AgileWorkItem) {
    if (columns.length === 0) {
      error = 'No board columns configured; cannot start work.';
      return;
    }
    try {
      await window.go.main.App.MoveWorkItem(item.id, columns[0].id, 0);
      items = items.filter((i) => i.id !== item.id);
    } catch (err: any) {
      error = `Could not start work: ${err}`;
    }
  }

  // Drag-to-reorder within the backlog.
  let draggingIdx = $state<number | null>(null);

  function onDragStart(e: DragEvent, idx: number) {
    draggingIdx = idx;
    e.dataTransfer?.setData('text/plain', String(idx));
  }
  function onDragOver(e: DragEvent) {
    e.preventDefault();
  }
  function onDrop(e: DragEvent, targetIdx: number) {
    e.preventDefault();
    const src = draggingIdx;
    draggingIdx = null;
    if (src === null || src === targetIdx) return;

    const next = [...items];
    const [moved] = next.splice(src, 1);
    next.splice(targetIdx, 0, moved);
    // Re-stamp order_idx so the persistence step writes contiguous ints.
    for (let i = 0; i < next.length; i++) next[i].order_idx = i;
    items = next;

    if (reorderTimer) clearTimeout(reorderTimer);
    reorderTimer = setTimeout(persistOrder, 400);
  }

  async function persistOrder() {
    try {
      // Save each item that changed. Cheap because the backlog is
      // typically dozens, not thousands, of items.
      for (const it of items) {
        await window.go.main.App.SaveWorkItem(it);
      }
    } catch (err: any) {
      error = `Could not persist order: ${err}`;
    }
  }

  // Concurrency hardening (AGENT.md §6): cancel pending reorder save.
  onDestroy(() => {
    if (reorderTimer) {
      clearTimeout(reorderTimer);
      reorderTimer = null;
    }
  });

  function priorityTint(p: AgilePriority): string {
    switch (p) {
      case 'urgent': return 'border-l-4 border-red-500';
      case 'high':   return 'border-l-4 border-amber-500';
      case 'medium': return 'border-l-4 border-cyan-500';
      default:       return 'border-l-4 border-slate-700';
    }
  }
</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <button onclick={() => goto('dashboard')} class="text-xs text-slate-400 hover:text-cyan-400">
        &larr; Dashboard
      </button>
      <h1 class="text-sm font-bold tracking-widest uppercase text-white">Backlog</h1>
      <span class="text-xs text-slate-500">{items.length} item{items.length === 1 ? '' : 's'}</span>
    </div>
    <div class="flex gap-2">
      <button
        onclick={() => goto('kanban')}
        class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded"
      >
        Board
      </button>
      <button
        onclick={openNew}
        class="text-xs bg-cyan-600 hover:bg-cyan-500 text-white font-bold uppercase px-3 py-1 rounded"
      >
        + Work item
      </button>
    </div>
  </header>

  <main class="p-6 max-w-4xl mx-auto">
    {#if error}
      <p class="text-xs text-red-400 mb-3" role="alert">{error}</p>
    {/if}

    {#if loading}
      <p class="text-sm text-slate-500">Loading backlog...</p>
    {:else if items.length === 0}
      <p class="text-sm text-slate-500 text-center py-12">
        Backlog is empty. Click <strong>+ Work item</strong> to add one.
      </p>
    {:else}
      <ol class="space-y-2">
        {#each items as item, i (item.id)}
          <li
            draggable="true"
            ondragstart={(e) => onDragStart(e, i)}
            ondragover={onDragOver}
            ondrop={(e) => onDrop(e, i)}
            class="p-3 bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded {priorityTint(item.priority)} flex items-start gap-3 cursor-move"
          >
            <span class="text-slate-600 font-mono text-xs select-none mt-1">
              {i + 1}.
            </span>
            <button
              onclick={() => openExisting(item)}
              class="flex-1 text-left min-w-0"
            >
              <div class="font-bold text-white truncate">
                {item.title || '(untitled)'}
              </div>
              <div class="text-[10px] text-slate-500 uppercase mt-0.5">
                {item.type} · {item.priority}
                {item.points > 0 ? ` · ${item.points}pt` : ''}
                {item.assignee ? ` · ${item.assignee}` : ''}
              </div>
            </button>
            <select
              value={item.sprint_id}
              onchange={(e) => assignSprint(item, (e.target as HTMLSelectElement).value)}
              onclick={(e) => e.stopPropagation()}
              class="text-xs bg-slate-950 border border-slate-800 rounded px-2 py-1"
            >
              <option value="">(no sprint)</option>
              {#each sprints as s (s.id)}
                <option value={s.id}>{s.name}</option>
              {/each}
            </select>
            <button
              onclick={() => startWork(item)}
              class="text-xs bg-slate-800 hover:bg-cyan-700 px-2 py-1 rounded"
              aria-label="Start work on {item.title}"
            >
              Start →
            </button>
          </li>
        {/each}
      </ol>
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
