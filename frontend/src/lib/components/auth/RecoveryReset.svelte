<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // RecoveryReset is the "forgot my password" flow. The user lands
  // here from the Login screen by clicking "use a recovery code".
  // They enter their username, one of the 8 unused codes generated
  // at account creation, and a new password. On success the code
  // is marked used and the password hash is rotated atomically.

  import { goto } from '../../session.svelte';

  let username = $state('');
  let code = $state('');
  let password = $state('');
  let confirm = $state('');
  let error = $state('');
  let busy = $state(false);
  let done = $state(false);

  async function submit(e: Event) {
    e.preventDefault();
    error = '';

    if (!username || !code || !password) {
      error = 'All fields are required.';
      return;
    }
    if (password.length < 8) {
      error = 'New password must be at least 8 characters.';
      return;
    }
    if (password !== confirm) {
      error = 'Passwords do not match.';
      return;
    }

    busy = true;
    try {
      await window.go.main.App.ResetWithRecoveryCode(username, code, password);
      done = true;
    } catch (err: any) {
      // Generic message — the backend collapses unknown-user and
      // invalid-code into the same error to avoid enumeration.
      error = 'Invalid username or recovery code.';
    } finally {
      busy = false;
    }
  }
</script>

<div class="min-h-screen flex items-center justify-center bg-slate-950">
  <form
    class="w-full max-w-sm p-8 bg-slate-900 border border-slate-800 rounded-xl shadow-xl space-y-4"
    onsubmit={submit}
  >
    <h1 class="text-xl font-bold text-slate-50 tracking-widest uppercase text-center">
      Reset Password
    </h1>
    <p class="text-xs text-slate-500 text-center">
      Enter one of the recovery codes you saved when the account was created.
    </p>

    {#if done}
      <p class="text-center text-sm text-emerald-300">
        Password reset. You can now sign in with your new password.
      </p>
      <button
        type="button"
        onclick={() => goto('login')}
        class="w-full bg-cyan-600 hover:bg-cyan-500 text-white font-bold py-2 rounded"
      >
        BACK TO SIGN IN
      </button>
    {:else}
      <label class="block">
        <span class="text-xs font-semibold text-slate-500 uppercase">Username</span>
        <input
          type="text"
          bind:value={username}
          class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        />
      </label>

      <label class="block">
        <span class="text-xs font-semibold text-slate-500 uppercase">Recovery code</span>
        <input
          type="text"
          bind:value={code}
          placeholder="JBSWY3DP-EHPK3PXP"
          class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none font-mono"
        />
      </label>

      <label class="block">
        <span class="text-xs font-semibold text-slate-500 uppercase">New password</span>
        <input
          type="password"
          bind:value={password}
          autocomplete="new-password"
          class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        />
      </label>

      <label class="block">
        <span class="text-xs font-semibold text-slate-500 uppercase">Confirm new password</span>
        <input
          type="password"
          bind:value={confirm}
          autocomplete="new-password"
          class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        />
      </label>

      {#if error}
        <p class="text-xs text-red-400" role="alert">{error}</p>
      {/if}

      <button
        type="submit"
        disabled={busy}
        class="w-full bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold py-2 rounded"
      >
        {busy ? 'Resetting…' : 'RESET PASSWORD'}
      </button>

      <button
        type="button"
        onclick={() => goto('login')}
        class="w-full text-xs text-cyan-400 hover:text-cyan-300 underline"
      >
        Back to sign in
      </button>
    {/if}
  </form>
</div>
