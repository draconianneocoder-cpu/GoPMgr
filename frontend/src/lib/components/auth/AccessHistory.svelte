<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Every recorded time an administrator opened the signed-in user's data
  // (ADR-004), for App Settings, under Account.
  import { onMount } from 'svelte';
  import { formatDateTime, t, tParts } from '../../i18n/i18n.svelte';

  let history = $state<AccountEvent[] | null>(null);
  let error = $state('');

  onMount(async () => {
    try {
      history = (await window.go.main.App.MyDataAccessHistory()) ?? [];
    } catch (err: any) {
      error = t('adminAccess.history.loadFailed', { error: String(err?.message ?? err) });
    }
  });

  function formatTime(value: string): string {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value : formatDateTime(date);
  }
</script>

<div class="space-y-2" role="group" aria-labelledby="access-history-heading">
  <h3 id="access-history-heading" class="text-xs font-semibold text-slate-300">{t('adminAccess.title')}</h3>
  {#if error}
    <p class="text-xs text-red-400" role="alert">{error}</p>
  {:else if history === null}
    <p class="text-xs text-slate-500">{t('common.loading')}</p>
  {:else if history.length === 0}
    <p class="text-xs text-slate-500">{t('adminAccess.history.none')}</p>
  {:else}
    <ul class="text-xs text-slate-300 space-y-1">
      {#each history as h (h.id)}
        <li>
          {#each tParts('adminAccess.history.entry', { time: formatTime(h.occurred_at), actor: h.actor }) as part}{#if part.param === 'time'}<span class="text-slate-500 font-mono">{part.text}</span>{:else}{part.text}{/if}{/each}
          <span class="block text-[11px] text-slate-400">{t('adminAccess.reason', { reason: h.detail })}</span>
        </li>
      {/each}
    </ul>
  {/if}
</div>
