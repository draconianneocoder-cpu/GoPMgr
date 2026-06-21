<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { getToasts, dismissToast, pauseToast, resumeToast } from '../toast.svelte';
  import type { Toast } from '../toast.svelte';

  let toasts = $derived(getToasts());

  function handleUndo(toast: Toast) {
    if (toast.undo) {
      toast.undo();
    }
    dismissToast(toast.id);
  }
</script>

<div class="fixed bottom-4 right-4 z-[9999] flex flex-col gap-2 items-end">
  {#each toasts as toast (toast.id)}
    <div
      role="status"
      aria-live="polite"
      onmouseenter={() => pauseToast(toast.id)}
      onmouseleave={() => resumeToast(toast.id)}
      class="group flex items-start gap-3 px-4 py-3 rounded-xl shadow-2xl text-sm max-w-sm border backdrop-blur
        transition-all duration-150
        {toast.type === 'success'
          ? 'bg-emerald-950/95 border-emerald-800 text-emerald-100'
          : toast.type === 'error'
          ? 'bg-red-950/95 border-red-800 text-red-100'
          : 'bg-slate-900/95 border-slate-700 text-slate-200'}"
    >
      <!-- Icon -->
      <div class="mt-0.5 text-lg select-none">
        {#if toast.type === 'success'}
          ✅
        {:else if toast.type === 'error'}
          ❌
        {:else}
          ℹ️
        {/if}
      </div>

      <div class="flex-1 min-w-0">
        <div class="pr-1 leading-snug">{toast.message}</div>

        {#if toast.undo}
          <button
            onclick={() => handleUndo(toast)}
            class="mt-1.5 text-xs font-medium underline underline-offset-2 hover:no-underline opacity-80 hover:opacity-100 transition"
          >
            {toast.undoLabel}
          </button>
        {/if}
      </div>

      <!-- Close button -->
      <button
        onclick={() => dismissToast(toast.id)}
        class="mt-0.5 -mr-1 opacity-40 hover:opacity-100 transition text-lg leading-none"
        aria-label="Dismiss"
      >
        ×
      </button>
    </div>
  {/each}
</div>
