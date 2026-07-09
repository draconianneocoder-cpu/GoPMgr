<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import Logo from '../Logo.svelte';

  let username = $state('');
  let displayName = $state('');
  let password = $state('');
  let confirm = $state('');
  let isAdmin = $state(false);
  let error = $state('');
  let busy = $state(false);
  let hasAdmin = $state(true); // optimistic: assume admin exists until we know otherwise
  let showPassword = $state(false);
  let showConfirm = $state(false);
  let usernameEl = $state<HTMLInputElement>();

  const usernameRule = /^[A-Za-z0-9_-]{3,32}$/;

  // Live, non-blocking validation cues so a new user gets affirmative
  // feedback while typing rather than only an error after submitting.
  const usernameValid = $derived(usernameRule.test(username));
  const usernameTouched = $derived(username.length > 0);
  const passwordLongEnough = $derived(password.length >= 8);
  const passwordsMatch = $derived(confirm.length > 0 && password === confirm);
  const confirmMismatch = $derived(confirm.length > 0 && password !== confirm);
  const canSubmit = $derived(!busy && !!username && !!password && !!confirm);

  onMount(async () => {
    usernameEl?.focus();
    try {
      hasAdmin = await window.go.main.App.HasAnyAdmin();
    } catch {
      hasAdmin = true; // safe default: don't show admin prompt on error
    }
  });

  async function submit(e: Event) {
    e.preventDefault();
    error = '';

    if (!usernameRule.test(username)) {
      error = 'Username must be 3–32 letters, digits, _ or -.';
      return;
    }
    if (password.length < 8) {
      error = 'Password must be at least 8 characters.';
      return;
    }
    if (password !== confirm) {
      error = 'Passwords do not match.';
      return;
    }

    busy = true;
    try {
      const acc = await window.go.main.App.CreateAccount(username, displayName || username, password, isAdmin);
      session.user = acc;
      goto('portfolio');
    } catch (err: any) {
      error = String(err?.message ?? err);
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
      <h1 class="text-sm font-bold text-slate-300 tracking-widest uppercase">Create your account</h1>
      <p class="text-xs text-slate-500">
        Everything you create stays on this computer — nothing is uploaded.
      </p>
    </div>

    {#if !hasAdmin}
      <div class="bg-amber-950/40 border border-amber-700/50 rounded-lg p-3 text-xs text-amber-300 space-y-1">
        <p class="font-semibold">You're the first user on this computer</p>
        <p class="text-amber-400/80">
          No PMForge administrator exists yet. You can make this account the administrator below —
          administrators can create and remove other accounts.
        </p>
      </div>
    {/if}

    <div>
      <label for="ca-username" class="block text-xs font-semibold text-slate-500 uppercase">Username</label>
      <input
        id="ca-username"
        type="text"
        autocomplete="username"
        bind:this={usernameEl}
        bind:value={username}
        aria-invalid={usernameTouched && !usernameValid}
        aria-describedby="ca-username-hint"
        class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
      />
      <p id="ca-username-hint" class="text-[10px] mt-1 {usernameTouched && !usernameValid ? 'text-amber-400' : 'text-slate-500'}">
        3–32 characters; letters, digits, _ or -
      </p>
    </div>

    <div>
      <label for="ca-display" class="block text-xs font-semibold text-slate-500 uppercase">Display name</label>
      <input
        id="ca-display"
        type="text"
        bind:value={displayName}
        class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
      />
      <p class="text-[10px] mt-1 text-slate-500">Optional — shown in the app. Defaults to your username.</p>
    </div>

    <div>
      <label for="ca-password" class="block text-xs font-semibold text-slate-500 uppercase">Password</label>
      <div class="relative mt-1">
        <input
          id="ca-password"
          type={showPassword ? 'text' : 'password'}
          autocomplete="new-password"
          bind:value={password}
          aria-describedby="ca-password-hint"
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
      <p id="ca-password-hint" class="text-[10px] mt-1 {password.length === 0 ? 'text-slate-500' : passwordLongEnough ? 'text-emerald-400' : 'text-amber-400'}">
        {passwordLongEnough ? '✓ ' : ''}At least 8 characters
      </p>
    </div>

    <div>
      <label for="ca-confirm" class="block text-xs font-semibold text-slate-500 uppercase">Confirm password</label>
      <div class="relative mt-1">
        <input
          id="ca-confirm"
          type={showConfirm ? 'text' : 'password'}
          autocomplete="new-password"
          bind:value={confirm}
          aria-invalid={confirmMismatch}
          aria-describedby="ca-confirm-hint"
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
      <p id="ca-confirm-hint" class="text-[10px] mt-1 {passwordsMatch ? 'text-emerald-400' : 'text-amber-400'}" class:hidden={confirm.length === 0}>
        {passwordsMatch ? '✓ Passwords match' : 'Passwords don’t match yet'}
      </p>
    </div>

    {#if !hasAdmin}
      <label class="flex items-start gap-3 cursor-pointer select-none">
        <input
          type="checkbox"
          bind:checked={isAdmin}
          class="mt-0.5 accent-cyan-500"
        />
        <span class="text-xs text-slate-300">
          <span class="font-semibold text-slate-100">Make this account an administrator</span><br />
          <span class="text-slate-500">
            Grants the ability to create and delete PMForge accounts on this machine.
            This option is only available while no administrator exists.
          </span>
        </span>
      </label>
    {/if}

    {#if error}
      <p class="text-xs text-red-400" role="alert" aria-live="assertive">{error}</p>
    {/if}

    <button
      type="submit"
      disabled={!canSubmit}
      class="w-full bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 disabled:cursor-not-allowed text-white font-bold py-2 rounded transition-colors"
    >
      {busy ? 'Creating…' : 'CREATE ACCOUNT'}
    </button>

    <button
      type="button"
      onclick={() => goto('login')}
      class="w-full text-xs text-cyan-400 hover:text-cyan-300 underline"
    >
      Back to sign in
    </button>
  </form>
</div>
