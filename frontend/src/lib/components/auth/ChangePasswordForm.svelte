<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { showToast } from '../../toast.svelte';

  // Its own <form>, separate from App Settings' Save, so Enter changes the
  // password rather than saving settings. The fields are cleared after a
  // change and when the component is destroyed (they are component state).
  let current = $state('');
  let next = $state('');
  let confirm = $state('');
  let showPasswords = $state(false);
  let busy = $state(false);
  let error = $state('');

  const longEnough = $derived(next.length >= 8);
  const matches = $derived(confirm.length > 0 && next === confirm);
  const canSubmit = $derived(!busy && !!current && longEnough && matches);

  async function submit(e: Event) {
    e.preventDefault();
    if (!canSubmit) return;
    busy = true;
    error = '';
    try {
      await window.go.main.App.ChangePassword(current, next);
      current = '';
      next = '';
      confirm = '';
      showToast('Password changed. Your projects and recovery codes are unaffected.', 'success');
    } catch (err: any) {
      const message = String(err?.message ?? err);
      error = message.charAt(0).toUpperCase() + message.slice(1) + '.';
    } finally {
      busy = false;
    }
  }
</script>

<form class="space-y-3" onsubmit={submit} aria-labelledby="change-password-heading">
  <h3 id="change-password-heading" class="text-xs font-semibold text-slate-300">Change password</h3>

  <div>
    <label for="cp-current" class="block text-[11px] font-semibold text-slate-500 uppercase">Current password</label>
    <input
      id="cp-current"
      type={showPasswords ? 'text' : 'password'}
      autocomplete="current-password"
      bind:value={current}
      class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
    />
  </div>

  <div>
    <label for="cp-new" class="block text-[11px] font-semibold text-slate-500 uppercase">New password</label>
    <input
      id="cp-new"
      type={showPasswords ? 'text' : 'password'}
      autocomplete="new-password"
      bind:value={next}
      aria-describedby="cp-new-hint"
      class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
    />
    <p id="cp-new-hint" class="text-[10px] mt-1 {next.length === 0 ? 'text-slate-500' : longEnough ? 'text-emerald-400' : 'text-amber-400'}">
      {longEnough ? '✓ ' : ''}At least 8 characters
    </p>
  </div>

  <div>
    <label for="cp-confirm" class="block text-[11px] font-semibold text-slate-500 uppercase">Confirm new password</label>
    <input
      id="cp-confirm"
      type={showPasswords ? 'text' : 'password'}
      autocomplete="new-password"
      bind:value={confirm}
      aria-describedby="cp-confirm-hint"
      class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
    />
    <p id="cp-confirm-hint" class="text-[10px] mt-1 {matches ? 'text-emerald-400' : 'text-amber-400'}" class:hidden={confirm.length === 0}>
      {matches ? '✓ Passwords match' : 'Passwords don’t match yet'}
    </p>
  </div>

  <label class="flex items-center gap-2 text-[11px] text-slate-400 select-none">
    <input type="checkbox" bind:checked={showPasswords} class="accent-cyan-500" />
    Show passwords
  </label>

  {#if error}
    <p class="text-xs text-red-400" role="alert">{error}</p>
  {/if}

  <button
    type="submit"
    disabled={!canSubmit}
    class="bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 disabled:cursor-not-allowed text-white text-xs font-bold uppercase tracking-wider px-4 py-2 rounded"
  >
    {busy ? 'Changing…' : 'Change password'}
  </button>
</form>
