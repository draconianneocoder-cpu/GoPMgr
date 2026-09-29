<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { goto } from '../../session.svelte';

  // RepairAndSwap checks the open project's database and, when it finds
  // damage, replaces it with a healed copy (the damaged file is kept beside
  // it). After a swap every other view still holds data read from the old
  // file, so the user is sent back to the dashboard to reload it.
  let busy = $state(false);
  let result = $state<RepairResult | null>(null);
  let error = $state('');

  async function checkAndRepair() {
    if (busy) return;
    busy = true;
    result = null;
    error = '';
    try {
      result = await window.go.main.App.RepairAndSwap();
    } catch (err: any) {
      error = String(err?.message ?? err);
    } finally {
      busy = false;
    }
  }
</script>

<div class="border border-slate-800 bg-slate-900/60 rounded p-4 space-y-3">
  <p class="text-xs text-slate-400">
    Checks this project's database for damage and, if it finds any, repairs it from a clean
    copy. The damaged file is kept beside the project. Create a backup first if you can.
  </p>
  <button
    type="button"
    onclick={checkAndRepair}
    disabled={busy}
    class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-4 py-2 rounded border border-slate-700"
  >{busy ? 'Checking…' : 'Check and repair'}</button>

  {#if result?.swapped}
    <div class="space-y-2" role="status">
      <p class="text-xs text-emerald-300">
        Repaired. The damaged copy was kept at <span class="break-all">{result.damaged_copy}</span>.
      </p>
      <button
        type="button"
        onclick={() => goto('dashboard')}
        class="text-xs bg-cyan-600 hover:bg-cyan-500 text-white font-bold px-4 py-2 rounded"
      >Reload the project</button>
    </div>
  {:else if result?.success}
    <p class="text-xs text-emerald-300" role="status">No problems found.</p>
  {:else if result || error}
    <p class="text-xs text-red-400" role="alert">
      This project could not be repaired{error ? `: ${error}` : ''}. Restore it from a backup.
    </p>
  {/if}

  {#if result?.log?.length}
    <details class="text-[11px] text-slate-500">
      <summary class="cursor-pointer">Details</summary>
      <ol class="mt-1 space-y-0.5 font-mono break-all">
        {#each result.log as line, i (i)}
          <li>{line}</li>
        {/each}
      </ol>
    </details>
  {/if}
</div>
