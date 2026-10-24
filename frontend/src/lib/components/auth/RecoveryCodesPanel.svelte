<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { session } from '../../session.svelte';
  import { showToast } from '../../toast.svelte';
  import RecoveryCodeList from './RecoveryCodeList.svelte';
  import { t } from '../../i18n/i18n.svelte';

  // onrenewed runs after new codes are saved, once the panel has returned
  // to its idle step (Project Settings uses it to allow encryption again).
  // onpendingchange reports whether unsaved new codes are on screen, so a
  // parent can hold back actions (such as encrypting) until they are saved
  // or discarded.
  let {
    onrenewed,
    onpendingchange,
  }: { onrenewed?: () => void; onpendingchange?: (pending: boolean) => void } = $props();

  // New codes are made in two steps so the user never ends up without
  // working codes: PrepareRecoveryCodes returns them without storing
  // anything, and ConfirmRecoveryCodes replaces the old ones only after the
  // user ticks that they saved them. Leaving the page discards them.
  let status = $state<RecoveryCodeStatus | null>(null);
  let statusError = $state('');
  let step = $state<'idle' | 'password' | 'codes'>('idle');
  let password = $state('');
  let codes = $state<string[]>([]);
  let saved = $state(false);
  let busy = $state(false);
  let error = $state('');
  // Files saved while new codes are on screen hold codes that work only once
  // they are confirmed. If they are dropped instead, each file looks like a
  // good recovery file but is not, so name them for the user to delete.
  let savedFiles = $state<string[]>([]);
  let unusedFiles = $state<string[]>([]);

  onMount(loadStatus);
  $effect(() => {
    onpendingchange?.(step === 'codes');
  });
  onDestroy(() => {
    if (step === 'codes') {
      void window.go.main.App.DiscardRecoveryCodes();
      // The panel is gone, so only a toast can still reach the user.
      if (savedFiles.length > 0) showToast(unusedFileWarning(savedFiles), 'error');
    }
    onpendingchange?.(false);
  });

  function unusedFileWarning(files: string[]): string {
    return t('recoveryPanel.unusedFiles', { count: files.length, files: files.join(', ') });
  }

  function dropSavedFiles() {
    unusedFiles = [...unusedFiles, ...savedFiles];
    savedFiles = [];
  }

  async function loadStatus() {
    statusError = '';
    try {
      status = await window.go.main.App.RecoveryCodeStatus();
    } catch (err: any) {
      statusError = t('recoveryPanel.statusFailed', { error: String(err?.message ?? err) });
    }
  }

  function start() {
    step = 'password';
    password = '';
    error = '';
  }

  async function prepare(e: Event) {
    e.preventDefault();
    if (busy || !password) return;
    busy = true;
    error = '';
    try {
      codes = (await window.go.main.App.PrepareRecoveryCodes(password)) ?? [];
      password = '';
      saved = false;
      step = 'codes';
    } catch (err: any) {
      const message = String(err?.message ?? err);
      error = message.charAt(0).toUpperCase() + message.slice(1) + '.';
    } finally {
      busy = false;
    }
  }

  async function confirm() {
    if (busy || !saved) return;
    let renewed = false;
    busy = true;
    error = '';
    try {
      await window.go.main.App.ConfirmRecoveryCodes();
      showToast(t('recoveryPanel.renewed'), 'success');
      finish();
      renewed = true;
    } catch (err: any) {
      const message = String(err?.message ?? err);
      error = message.charAt(0).toUpperCase() + message.slice(1) + '.';
      dropSavedFiles();
      step = 'idle';
      codes = [];
    } finally {
      busy = false;
      await loadStatus();
    }
    if (renewed) onrenewed?.();
  }

  async function cancel() {
    if (step === 'codes') {
      await window.go.main.App.DiscardRecoveryCodes();
      dropSavedFiles();
    }
    finish();
  }

  function finish() {
    step = 'idle';
    password = '';
    codes = [];
    saved = false;
    savedFiles = [];
  }
</script>

<div class="space-y-3" aria-labelledby="recovery-codes-heading" role="group">
  <h3 id="recovery-codes-heading" class="text-xs font-semibold text-slate-300">{t('recoveryPanel.title')}</h3>

  {#if statusError}
    <p class="text-xs text-red-400" role="alert">{statusError}</p>
  {:else if status}
    <p class="text-xs text-slate-400">{t('recoveryPanel.unused', { unused: status.unused, count: status.total })}</p>
    {#if status.legacy}
      <p class="text-xs text-amber-300 bg-amber-950/40 border border-amber-700/50 rounded p-2" role="status">
        {t('recoveryPanel.legacy')}
      </p>
    {:else if status.unused === 0}
      <p class="text-xs text-red-300 bg-red-950/40 border border-red-800/60 rounded p-2" role="status">
        {t('recoveryPanel.none')}
      </p>
    {:else if status.unused === 1}
      <p class="text-xs text-amber-300 bg-amber-950/40 border border-amber-700/50 rounded p-2" role="status">
        {t('recoveryPanel.oneLeft')}
      </p>
    {/if}
  {/if}

  {#if unusedFiles.length > 0}
    <p class="text-xs text-amber-300 bg-amber-950/40 border border-amber-700/50 rounded p-2" role="alert">
      {unusedFileWarning(unusedFiles)}
    </p>
  {/if}

  {#if step === 'idle'}
    <button
      type="button"
      onclick={start}
      class="bg-slate-800 hover:bg-slate-700 border border-slate-700 text-xs px-4 py-2 rounded"
    >{t('recoveryPanel.create')}</button>
  {:else if step === 'password'}
    <form class="space-y-2" onsubmit={prepare}>
      <p class="text-xs text-slate-400">{t('recoveryPanel.keepWorking')}</p>
      <label for="rc-password" class="block text-[11px] font-semibold text-slate-500 uppercase">{t('common.password.current')}</label>
      <input
        id="rc-password"
        type="password"
        autocomplete="current-password"
        bind:value={password}
        class="w-full bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
      />
      <div class="flex items-center gap-2">
        <button
          type="submit"
          disabled={busy || !password}
          class="bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 disabled:cursor-not-allowed text-white text-xs font-bold px-4 py-2 rounded"
        >{busy ? t('common.creating') : t('recoveryPanel.createCodes')}</button>
        <button type="button" onclick={cancel} class="text-xs text-slate-400 hover:text-slate-200 underline">{t('common.cancel')}</button>
      </div>
    </form>
  {:else}
    <div class="space-y-2">
      <p class="text-xs text-slate-300">{t('recoveryPanel.saveThese', { action: t('recoveryPanel.useNew') })}</p>
      <RecoveryCodeList
        username={session.user?.username ?? ''}
        {codes}
        savedHint={t('recoveryPanel.savedHint', { action: t('recoveryPanel.useNew') })}
        onsaved={(path) => (savedFiles = [...savedFiles, path])}
      />
      <label class="flex items-start gap-2 text-xs text-slate-300 select-none">
        <input type="checkbox" bind:checked={saved} class="mt-0.5 accent-cyan-500" />
        {t('recoveryPanel.savedCheck')}
      </label>
      <div class="flex items-center gap-2">
        <button
          type="button"
          onclick={confirm}
          disabled={busy || !saved}
          class="bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 disabled:cursor-not-allowed text-white text-xs font-bold px-4 py-2 rounded"
        >{busy ? t('common.saving') : t('recoveryPanel.useNew')}</button>
        <button type="button" onclick={cancel} class="text-xs text-slate-400 hover:text-slate-200 underline">{t('recoveryPanel.keepCurrent')}</button>
      </div>
    </div>
  {/if}

  {#if error}
    <p class="text-xs text-red-400" role="alert">{error}</p>
  {/if}
</div>
