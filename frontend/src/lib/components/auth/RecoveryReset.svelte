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

  import { onMount } from 'svelte';
  import { goto } from '../../session.svelte';
  import Logo from '../Logo.svelte';

  let username = $state('');
  let code = $state('');
  let password = $state('');
  let confirm = $state('');
  let error = $state('');
  let busy = $state(false);
  let done = $state(false);
  let showPassword = $state(false);
  let showConfirm = $state(false);
  let usernameEl = $state<HTMLInputElement>();

  // Live, non-blocking validation cues (parity with Create Account).
  const passwordLongEnough = $derived(password.length >= 8);
  const passwordsMatch = $derived(confirm.length > 0 && password === confirm);
  const confirmMismatch = $derived(confirm.length > 0 && password !== confirm);
  const canSubmit = $derived(!busy && !!username && !!code && !!password && !!confirm);

  onMount(() => usernameEl?.focus());

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

<div class="min-h-screen flex items-center justify-center bg-slate-950 p-4">
  <form
    class="w-full max-w-sm p-8 bg-slate-900 border border-slate-800 rounded-xl shadow-xl space-y-4"
    onsubmit={submit}
  >
    <div class="space-y-2 text-center">
      <Logo class="h-9 w-auto mx-auto text-slate-50" />
      <h1 class="text-sm font-bold text-slate-300 tracking-widest uppercase">Reset password</h1>
      <p class="text-xs text-slate-500">
        Enter one of the recovery codes you saved when the account was created.
      </p>
    </div>

    {#if done}
      <p class="text-center text-sm text-emerald-300" role="status" aria-live="polite">
        Password reset. You can now sign in with your new password.
      </p>
      <button
        type="button"
        onclick={() => goto('login')}
        class="w-full bg-cyan-600 hover:bg-cyan-500 text-white font-bold py-2 rounded transition-colors"
      >
        BACK TO SIGN IN
      </button>
    {:else}
      <div>
        <label for="rr-username" class="block text-xs font-semibold text-slate-500 uppercase">Username</label>
        <input
          id="rr-username"
          type="text"
          autocomplete="username"
          bind:this={usernameEl}
          bind:value={username}
          class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        />
      </div>

      <div>
        <label for="rr-code" class="block text-xs font-semibold text-slate-500 uppercase">Recovery code</label>
        <input
          id="rr-code"
          type="text"
          bind:value={code}
          placeholder="JBSWY3DP-EHPK3PXP"
          class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none font-mono"
        />
      </div>

      <div>
        <label for="rr-password" class="block text-xs font-semibold text-slate-500 uppercase">New password</label>
        <div class="relative mt-1">
          <input
            id="rr-password"
            type={showPassword ? 'text' : 'password'}
            autocomplete="new-password"
            bind:value={password}
            aria-describedby="rr-password-hint"
            class="w-full bg-slate-950 border border-slate-800 p-2 pr-11 rounded focus:border-cyan-500 outline-none"
          />
          <button
            type="button"
            onclick={() => (showPassword = !showPassword)}
            aria-label={showPassword ? 'Hide password' : 'Show password'}
            aria-pressed={showPassword}
            class="absolute inset-y-0 right-0 flex items-center px-3 text-slate-500 hover:text-slate-300 rounded-r"
          >
            {#if showPassword}
              <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24" stroke-linecap="round" stroke-linejoin="round" />
                <line x1="1" y1="1" x2="23" y2="23" stroke-linecap="round" />
              </svg>
            {:else}
              <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8Z" stroke-linecap="round" stroke-linejoin="round" />
                <circle cx="12" cy="12" r="3" />
              </svg>
            {/if}
          </button>
        </div>
        <p id="rr-password-hint" class="text-[10px] mt-1 {password.length === 0 ? 'text-slate-500' : passwordLongEnough ? 'text-emerald-400' : 'text-amber-400'}">
          {passwordLongEnough ? '✓ ' : ''}At least 8 characters
        </p>
      </div>

      <div>
        <label for="rr-confirm" class="block text-xs font-semibold text-slate-500 uppercase">Confirm new password</label>
        <div class="relative mt-1">
          <input
            id="rr-confirm"
            type={showConfirm ? 'text' : 'password'}
            autocomplete="new-password"
            bind:value={confirm}
            aria-invalid={confirmMismatch}
            aria-describedby="rr-confirm-hint"
            class="w-full bg-slate-950 border border-slate-800 p-2 pr-11 rounded focus:border-cyan-500 outline-none"
          />
          <button
            type="button"
            onclick={() => (showConfirm = !showConfirm)}
            aria-label={showConfirm ? 'Hide password' : 'Show password'}
            aria-pressed={showConfirm}
            class="absolute inset-y-0 right-0 flex items-center px-3 text-slate-500 hover:text-slate-300 rounded-r"
          >
            {#if showConfirm}
              <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24" stroke-linecap="round" stroke-linejoin="round" />
                <line x1="1" y1="1" x2="23" y2="23" stroke-linecap="round" />
              </svg>
            {:else}
              <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8Z" stroke-linecap="round" stroke-linejoin="round" />
                <circle cx="12" cy="12" r="3" />
              </svg>
            {/if}
          </button>
        </div>
        <p id="rr-confirm-hint" class="text-[10px] mt-1 {passwordsMatch ? 'text-emerald-400' : 'text-amber-400'}" class:hidden={confirm.length === 0}>
          {passwordsMatch ? '✓ Passwords match' : 'Passwords don’t match yet'}
        </p>
      </div>

      {#if error}
        <p class="text-xs text-red-400" role="alert" aria-live="assertive">{error}</p>
      {/if}

      <button
        type="submit"
        disabled={!canSubmit}
        class="w-full bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 disabled:cursor-not-allowed text-white font-bold py-2 rounded transition-colors"
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
