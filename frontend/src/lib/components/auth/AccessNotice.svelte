<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Tells the signed-in user that an administrator opened their data
  // (ADR-004), or made a new administrator key because nobody held the old
  // one (ADR-005 part 3c). Shown on the first screen after every sign-in
  // until the user chooses "I've read this", which is recorded. Unlike
  // advice, it never hides itself when it can't load: the user is told the
  // check failed.
  import { onMount } from 'svelte';
  import { formatDateTime, t, tParts } from '../../i18n/i18n.svelte';

  let notices = $state<AccountEvent[]>([]);
  const accesses = $derived(notices.filter((n) => n.action !== 'escrow_key_new'));
  const newKeys = $derived(notices.filter((n) => n.action === 'escrow_key_new'));
  const heading = $derived(
    newKeys.length === 0
      ? t('adminAccess.notice.openedTitle', { count: accesses.length })
      : accesses.length === 0
        ? t('adminAccess.notice.newKeyTitle', { count: newKeys.length })
        : t('adminAccess.title')
  );
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
      ackError = t('adminAccess.notice.ackFailed', { error: String(err?.message ?? err) });
    } finally {
      acknowledging = false;
    }
  }

  function formatTime(value: string): string {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value : formatDateTime(date);
  }
</script>

{#if loadError}
  <section
    aria-labelledby="access-notice-heading"
    class="mb-6 p-4 bg-red-950/30 border border-red-800/60 rounded-lg space-y-2"
  >
    <h2 id="access-notice-heading" class="text-xs font-bold uppercase tracking-widest text-red-300">
      {t('adminAccess.title')}
    </h2>
    <p class="text-xs text-slate-300" role="alert">
      {t('adminAccess.notice.checkFailed', { error: loadError })}
    </p>
    <button type="button" onclick={load} class="text-xs text-slate-300 hover:text-white underline">{t('common.tryAgain')}</button>
  </section>
{:else if notices.length > 0}
  <section
    aria-labelledby="access-notice-heading"
    class="mb-6 p-4 bg-amber-950/30 border border-amber-700/50 rounded-lg space-y-3"
  >
    <h2 id="access-notice-heading" class="text-xs font-bold uppercase tracking-widest text-amber-300">
      {heading}
    </h2>
    {#if accesses.length > 0}
      <p class="text-xs text-slate-300">{t('adminAccess.notice.readOnly')}</p>
      <ul class="text-xs text-slate-200 space-y-2">
        {#each accesses as n (n.id)}
          <li>
            {#each tParts('adminAccess.notice.opened', { actor: n.actor, time: formatTime(n.occurred_at) }) as part}{#if part.param === 'actor'}<span class="font-semibold">{part.text}</span>{:else}{part.text}{/if}{/each}
            <span class="block text-slate-400">{t('adminAccess.reason', { reason: n.detail })}</span>
          </li>
        {/each}
      </ul>
    {/if}
    {#if newKeys.length > 0}
      <p class="text-xs text-slate-300">{t('adminAccess.notice.newKeyBody')}</p>
      <ul class="text-xs text-slate-200 space-y-2">
        {#each newKeys as n (n.id)}
          <li>
            {#each tParts('adminAccess.notice.newKey', { actor: n.actor, time: formatTime(n.occurred_at) }) as part}{#if part.param === 'actor'}<span class="font-semibold">{part.text}</span>{:else}{part.text}{/if}{/each}
            <span class="block text-slate-400">{t('adminAccess.reason', { reason: n.detail })}</span>
          </li>
        {/each}
      </ul>
    {/if}
    {#if ackError}
      <p class="text-xs text-red-400" role="alert">{ackError}</p>
    {/if}
    <button
      type="button"
      onclick={acknowledge}
      disabled={acknowledging}
      class="bg-amber-700 hover:bg-amber-600 disabled:opacity-50 text-white text-xs font-bold uppercase tracking-wider px-3 py-1.5 rounded"
    >
      {acknowledging ? t('common.saving') : t('adminAccess.notice.acknowledge')}
    </button>
  </section>
{/if}
