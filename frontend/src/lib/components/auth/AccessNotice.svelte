<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Tells the signed-in user that an administrator opened their data
  // (ADR-004). Shown on the first screen after every sign-in until the user
  // chooses "I've read this", which is recorded. Unlike advice, it never
  // hides itself when it can't load: the user is told the check failed.
  import { onMount } from 'svelte';

  let notices = $state<AccountEvent[]>([]);
  let loadError = $state('');
  let acknowledging = $state(false);
  let ackError = $state('');

  onMount(load);

  async function load() {
    loadError = '';
    try {
      notices = (await window.go.main.App.MyDataAccessNotices()) ?? [];
    } catch (err: any) {
      loadError = String(err?.message ?? err);
    }
  }

  async function acknowledge() {
    if (notices.length === 0 || acknowledging) return;
    acknowledging = true;
    ackError = '';
    // Acknowledge only what was shown: an access recorded since stays unread.
    const throughID = Math.max(...notices.map((n) => n.id));
    try {
      await window.go.main.App.AcknowledgeDataAccessNotices(throughID);
      await load();
    } catch (err: any) {
      ackError = `Could not save that you've read this: ${String(err?.message ?? err)}`;
    } finally {
      acknowledging = false;
    }
  }

  function formatTime(value: string): string {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
  }
</script>

{#if loadError}
  <section
    aria-labelledby="access-notice-heading"
    class="mb-6 p-4 bg-red-950/30 border border-red-800/60 rounded-lg space-y-2"
  >
    <h2 id="access-notice-heading" class="text-xs font-bold uppercase tracking-widest text-red-300">
      Administrator access to your data
    </h2>
    <p class="text-xs text-slate-300" role="alert">
      GoPMgr couldn't check whether an administrator opened your data: {loadError}
    </p>
    <button type="button" onclick={load} class="text-xs text-slate-300 hover:text-white underline">Try again</button>
  </section>
{:else if notices.length > 0}
  <section
    aria-labelledby="access-notice-heading"
    class="mb-6 p-4 bg-amber-950/30 border border-amber-700/50 rounded-lg space-y-3"
  >
    <h2 id="access-notice-heading" class="text-xs font-bold uppercase tracking-widest text-amber-300">
      {notices.length === 1 ? 'An administrator opened your data' : 'Administrators opened your data'}
    </h2>
    <p class="text-xs text-slate-300">
      They could see your projects read only and could not change anything. App Settings, under Account,
      lists every time this has happened.
    </p>
    <ul class="text-xs text-slate-200 space-y-2">
      {#each notices as n (n.id)}
        <li>
          <span class="font-semibold">{n.actor}</span> opened your data on {formatTime(n.occurred_at)}.
          <span class="block text-slate-400">Reason: {n.detail}</span>
        </li>
      {/each}
    </ul>
    {#if ackError}
      <p class="text-xs text-red-400" role="alert">{ackError}</p>
    {/if}
    <button
      type="button"
      onclick={acknowledge}
      disabled={acknowledging}
      class="bg-amber-700 hover:bg-amber-600 disabled:opacity-50 text-white text-xs font-bold uppercase tracking-wider px-3 py-1.5 rounded"
    >
      {acknowledging ? 'Saving…' : "I've read this"}
    </button>
  </section>
{/if}
