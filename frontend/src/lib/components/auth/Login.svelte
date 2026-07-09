<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import Logo from '../Logo.svelte';

  let username = $state('');
  let password = $state('');
  let error = $state('');
  let busy = $state(false);
  let showPassword = $state(false);
  let usernameEl = $state<HTMLInputElement>();
  let hasAdmin = $state(true); // optimistic default

  // Focus the first field on load so the user can type immediately.
  onMount(async () => {
    usernameEl?.focus();
    try {
      hasAdmin = await window.go.main.App.HasAnyAdmin();
    } catch {
      hasAdmin = true;
    }
  });

  async function submit(e: Event) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      const acc = await window.go.main.App.Login(username, password);
      session.user = acc;
      goto('portfolio');
    } catch (err) {
      // Generic message — never reveal whether username or password was wrong.
      error = 'Invalid username or password.';
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
      <Logo class="h-10 w-auto mx-auto text-slate-50" />
      <p class="text-xs text-slate-500">Local-first project controls</p>
    </div>

    {#if !hasAdmin}
      <div class="bg-amber-950/40 border border-amber-700/50 rounded-lg p-2.5 text-xs text-amber-300">
        No administrator is configured. The first user to create an account can claim the administrator role.
      </div>
    {/if}

    <div>
      <label for="lg-username" class="block text-xs font-semibold text-slate-500 uppercase">Username</label>
      <input
        id="lg-username"
        type="text"
        autocomplete="username"
        bind:this={usernameEl}
        bind:value={username}
        class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
      />
    </div>

    <div>
      <label for="lg-password" class="block text-xs font-semibold text-slate-500 uppercase">Password</label>
      <div class="relative mt-1">
        <input
          id="lg-password"
          type={showPassword ? 'text' : 'password'}
          autocomplete="current-password"
          bind:value={password}
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
    </div>

    {#if error}
      <p class="text-xs text-red-400" role="alert" aria-live="assertive">{error}</p>
    {/if}

    <button
      type="submit"
      disabled={busy || !username || !password}
      class="w-full bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 disabled:cursor-not-allowed text-white font-bold py-2 rounded transition-colors"
    >
      {busy ? 'Signing in…' : 'SIGN IN'}
    </button>

    {#if !hasAdmin}
      <button
        type="button"
        onclick={() => goto('create_account')}
        class="w-full text-xs text-cyan-400 hover:text-cyan-300 underline"
      >
        Create a new account
      </button>
    {:else}
      <p class="text-center text-xs text-slate-500">
        Contact your administrator to create an account on this machine.
      </p>
    {/if}

    <button
      type="button"
      onclick={() => goto('recovery_reset')}
      class="w-full text-xs text-slate-500 hover:text-slate-300 underline"
    >
      Forgot password? Use a recovery code
    </button>
  </form>
</div>
