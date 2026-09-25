<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { showToast } from '../../toast.svelte';

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
  let copied = $state(false);
  // DEVELOPER_HANDBOOK.md §10.5: every timer must be cleared on destroy.
  let copiedTimer: ReturnType<typeof setTimeout> | null = null;

  onMount(loadStatus);
  onDestroy(() => {
    if (copiedTimer) clearTimeout(copiedTimer);
    if (step === 'codes') void window.go.main.App.DiscardRecoveryCodes();
  });

  async function loadStatus() {
    statusError = '';
    try {
      status = await window.go.main.App.RecoveryCodeStatus();
    } catch (err: any) {
      statusError = `Could not check your recovery codes: ${err?.message ?? err}`;
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
    busy = true;
    error = '';
    try {
      await window.go.main.App.ConfirmRecoveryCodes();
      showToast('New recovery codes saved. Your old codes no longer work.', 'success');
      finish();
    } catch (err: any) {
      const message = String(err?.message ?? err);
      error = message.charAt(0).toUpperCase() + message.slice(1) + '.';
      step = 'idle';
      codes = [];
    } finally {
      busy = false;
      await loadStatus();
    }
  }

  async function cancel() {
    if (step === 'codes') await window.go.main.App.DiscardRecoveryCodes();
    finish();
  }

  function finish() {
    step = 'idle';
    password = '';
    codes = [];
    saved = false;
  }

  async function copyCodes() {
    try {
      await navigator.clipboard.writeText(codes.join('\n'));
      copied = true;
      if (copiedTimer) clearTimeout(copiedTimer);
      copiedTimer = setTimeout(() => (copied = false), 2000);
    } catch {
      // Clipboard may be unavailable; the codes stay visible to copy by hand.
    }
  }
</script>

<div class="space-y-3" aria-labelledby="recovery-codes-heading" role="group">
  <h3 id="recovery-codes-heading" class="text-xs font-semibold text-slate-300">Recovery codes</h3>

  {#if statusError}
    <p class="text-xs text-red-400" role="alert">{statusError}</p>
  {:else if status}
    <p class="text-xs text-slate-400">{status.unused} of {status.total} recovery codes unused.</p>
    {#if status.legacy}
      <p class="text-xs text-amber-300 bg-amber-950/40 border border-amber-700/50 rounded p-2" role="status">
        Your recovery codes are from an older version of GoPMgr and can't recover encrypted projects.
        Create new ones.
      </p>
    {:else if status.unused === 0}
      <p class="text-xs text-red-300 bg-red-950/40 border border-red-800/60 rounded p-2" role="status">
        You have no unused recovery codes. If you forget your password, you can't get back into your
        encrypted projects. Create new ones now.
      </p>
    {:else if status.unused === 1}
      <p class="text-xs text-amber-300 bg-amber-950/40 border border-amber-700/50 rounded p-2" role="status">
        Only one recovery code is left. Create new ones before you need it.
      </p>
    {/if}
  {/if}

  {#if step === 'idle'}
    <button
      type="button"
      onclick={start}
      class="bg-slate-800 hover:bg-slate-700 border border-slate-700 text-xs px-4 py-2 rounded"
    >Create new recovery codes</button>
  {:else if step === 'password'}
    <form class="space-y-2" onsubmit={prepare}>
      <p class="text-xs text-slate-400">
        Your current codes keep working until you save the new ones.
      </p>
      <label for="rc-password" class="block text-[11px] font-semibold text-slate-500 uppercase">Current password</label>
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
        >{busy ? 'Creating…' : 'Create codes'}</button>
        <button type="button" onclick={cancel} class="text-xs text-slate-400 hover:text-slate-200 underline">Cancel</button>
      </div>
    </form>
  {:else}
    <div class="space-y-2">
      <p class="text-xs text-slate-300">
        Save these codes somewhere safe, such as a password manager or a printed copy. Each works
        once, and they are shown only now. Your current codes keep working until you save these.
      </p>
      <ol class="grid grid-cols-2 gap-1 font-mono text-xs text-slate-200 bg-slate-950 border border-slate-800 rounded p-3">
        {#each codes as code (code)}
          <li>{code}</li>
        {/each}
      </ol>
      <button type="button" onclick={copyCodes} class="text-xs text-cyan-400 hover:text-cyan-300 underline">Copy codes</button>
      <span class="text-xs text-emerald-400" aria-live="polite">{copied ? 'Copied.' : ''}</span>
      <label class="flex items-start gap-2 text-xs text-slate-300 select-none">
        <input type="checkbox" bind:checked={saved} class="mt-0.5 accent-cyan-500" />
        I have saved these codes somewhere safe.
      </label>
      <div class="flex items-center gap-2">
        <button
          type="button"
          onclick={confirm}
          disabled={busy || !saved}
          class="bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 disabled:cursor-not-allowed text-white text-xs font-bold px-4 py-2 rounded"
        >{busy ? 'Saving…' : 'Use the new codes'}</button>
        <button type="button" onclick={cancel} class="text-xs text-slate-400 hover:text-slate-200 underline">Keep my current codes</button>
      </div>
    </div>
  {/if}

  {#if error}
    <p class="text-xs text-red-400" role="alert">{error}</p>
  {/if}
</div>
