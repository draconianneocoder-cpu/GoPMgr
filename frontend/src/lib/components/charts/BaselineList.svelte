<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import ConfirmDialog from '../ConfirmDialog.svelte';

  // Lists a schedule chart's baselines, newest first, and deletes one after
  // confirmation. The list is fetched each time the panel opens, so a
  // baseline set from the editor since the last look always appears.
  // onchange lets the editor refresh its variance view, which follows the
  // newest remaining baseline.
  let { chartId, onchange }: { chartId: string; onchange?: () => void } = $props();

  let open = $state(false);
  let loading = $state(false);
  let baselines = $state<BaselineRecord[]>([]);
  let error = $state('');
  let pending = $state<BaselineRecord | null>(null);
  let deleting = $state(false);

  function label(b: BaselineRecord): string {
    const when = new Date(b.created_at).toLocaleString();
    return b.name ? `${b.name} · ${when}` : when;
  }

  async function load() {
    loading = true;
    error = '';
    try {
      baselines = (await window.go.main.App.ListScheduleBaselines(chartId)) ?? [];
    } catch (err: any) {
      error = `Could not load baselines: ${err?.message ?? err}`;
    } finally {
      loading = false;
    }
  }

  async function toggle() {
    open = !open;
    if (open) await load();
  }

  async function confirmDelete() {
    if (!pending || deleting) return;
    deleting = true;
    error = '';
    try {
      await window.go.main.App.DeleteScheduleBaseline(pending.id);
      pending = null;
      await load();
      onchange?.();
    } catch (err: any) {
      pending = null;
      error = String(err?.message ?? err);
    } finally {
      deleting = false;
    }
  }
</script>

<div class="relative">
  <button
    type="button"
    onclick={toggle}
    aria-expanded={open}
    aria-controls="baseline-list-panel"
    class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded"
  >Baselines</button>

  {#if open}
    <div
      id="baseline-list-panel"
      role="region"
      aria-label="Baselines"
      class="absolute right-0 z-20 mt-1 w-80 max-h-80 overflow-y-auto bg-slate-900 border border-slate-700 rounded shadow-xl p-3 space-y-2"
    >
      <p class="text-[11px] text-slate-500">
        Variances compare against the newest baseline. Scenarios already branched from a
        baseline keep their own copy.
      </p>
      {#if loading}
        <p class="text-xs text-slate-500">Loading…</p>
      {:else if baselines.length === 0}
        <p class="text-xs text-slate-500">No baselines yet. Use "Set baseline" to take one.</p>
      {:else}
        <ul class="space-y-1">
          {#each baselines as b, i (b.id)}
            <li class="flex items-center justify-between gap-2 text-xs">
              <span class="min-w-0 truncate text-slate-200" title={label(b)}>
                {label(b)}{#if i === 0}<span class="ml-1 text-cyan-400">(compared)</span>{/if}
              </span>
              <button
                type="button"
                onclick={() => (pending = b)}
                aria-label={`Delete baseline ${label(b)}`}
                class="shrink-0 text-red-400 hover:text-red-300 underline"
              >Delete</button>
            </li>
          {/each}
        </ul>
      {/if}
      {#if error}
        <p class="text-xs text-red-400" role="alert">{error}</p>
      {/if}
    </div>
  {/if}
</div>

<ConfirmDialog
  open={pending !== null}
  title="Delete this baseline?"
  message={pending
    ? `The baseline from ${label(pending)} will be removed. Variances will compare against the next newest baseline. This can't be undone.`
    : ''}
  confirmLabel="Delete baseline"
  cancelLabel="Keep baseline"
  busy={deleting}
  onConfirm={confirmDelete}
  onCancel={() => (pending = null)}
/>
