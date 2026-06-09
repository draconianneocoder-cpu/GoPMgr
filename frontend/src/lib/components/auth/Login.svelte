<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { session, goto } from '../../session.svelte';

  let username = $state('');
  let password = $state('');
  let error = $state('');
  let busy = $state(false);
  let usernameEl = $state<HTMLInputElement>();

  // Focus the first field on load so the user can type immediately.
  onMount(() => usernameEl?.focus());

  async function submit(e: Event) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      const acc = await window.go.main.App.Login(username, password);
      session.user = acc;
      goto('project_picker');
    } catch (err) {
      // Generic message — never reveal whether username or password was wrong.
      error = 'Invalid username or password.';
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
    <h1 class="text-2xl font-bold text-white tracking-widest uppercase text-center">PMForge</h1>
    <p class="text-xs text-slate-500 text-center">Local-first project controls</p>

    <label class="block">
      <span class="text-xs font-semibold text-slate-500 uppercase">Username</span>
      <input
        type="text"
        autocomplete="username"
        bind:this={usernameEl}
        bind:value={username}
        class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
      />
    </label>

    <label class="block">
      <span class="text-xs font-semibold text-slate-500 uppercase">Password</span>
      <input
        type="password"
        autocomplete="current-password"
        bind:value={password}
        class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
      />
    </label>

    {#if error}
      <p class="text-xs text-red-400" role="alert">{error}</p>
    {/if}

    <button
      type="submit"
      disabled={busy || !username || !password}
      class="w-full bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold py-2 rounded transition-all"
    >
      {busy ? 'Signing in...' : 'SIGN IN'}
    </button>

    <button
      type="button"
      onclick={() => goto('create_account')}
      class="w-full text-xs text-cyan-400 hover:text-cyan-300 underline"
    >
      Create a new account
    </button>
    <button
      type="button"
      onclick={() => goto('recovery_reset')}
      class="w-full text-xs text-slate-500 hover:text-slate-300 underline"
    >
      Forgot password? Use a recovery code
    </button>
  </form>
</div>
