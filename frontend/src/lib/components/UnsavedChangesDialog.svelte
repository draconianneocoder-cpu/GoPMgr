<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { t } from '../i18n/i18n.svelte';
  import { modalFocus } from '../modal-focus';
  import {
    navigation,
    saveAndContinueNavigation,
    discardAndContinueNavigation,
  } from '../session.svelte';

  // The prompt shown while a navigation waits on unsaved editor changes
  // (session.svelte.ts). Cancel is the first control, so it takes focus
  // when the prompt opens; Escape cancels too, except while saving.
  let { onCancel }: { onCancel: () => void } = $props();
</script>

{#if navigation.pending}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4" role="presentation">
    <div
      class="w-full max-w-md rounded-lg border border-slate-700 bg-slate-900 p-5 shadow-2xl"
      role="alertdialog"
      aria-modal="true"
      aria-labelledby="unsaved-title"
      aria-describedby="unsaved-description"
      tabindex="-1"
      use:modalFocus={{ onEscape: () => { if (!navigation.saving) onCancel(); } }}
    >
      <h2 id="unsaved-title" class="text-base font-bold text-slate-100">{t('unsaved.title')}</h2>
      <p id="unsaved-description" class="mt-2 text-sm text-slate-400">{t('unsaved.description')}</p>
      {#if navigation.error}
        <p class="mt-3 text-sm text-red-400" role="alert">{navigation.error}</p>
      {/if}
      <div class="mt-5 flex flex-wrap justify-end gap-2">
        <button onclick={onCancel} disabled={navigation.saving} class="rounded px-3 py-2 text-xs text-slate-300 hover:bg-slate-800">{t('unsaved.cancel')}</button>
        <button onclick={discardAndContinueNavigation} disabled={navigation.saving} class="rounded border border-red-800 px-3 py-2 text-xs text-red-300 hover:bg-red-950/40">{t('unsaved.discard')}</button>
        <button onclick={saveAndContinueNavigation} disabled={navigation.saving} class="rounded bg-cyan-700 px-3 py-2 text-xs font-bold text-white hover:bg-cyan-800 disabled:opacity-50">
          {navigation.saving ? t('unsaved.saving') : t('unsaved.save')}
        </button>
      </div>
    </div>
  </div>
{/if}
