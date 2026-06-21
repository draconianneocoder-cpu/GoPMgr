<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { session, goto } from '../../session.svelte';

  let username = $state('');
  let displayName = $state('');
  let password = $state('');
  let confirm = $state('');
  let isAdmin = $state(false);
  let error = $state('');
  let busy = $state(false);
  let hasAdmin = $state(true); // optimistic: assume admin exists until we know otherwise

  const usernameRule = /^[A-Za-z0-9_-]{3,32}$/;

  onMount(async () => {
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

<div class="min-h-screen flex items-center justify-center bg-slate-950">
  <form
    class="w-full max-w-sm p-8 bg-slate-900 border border-slate-800 rounded-xl shadow-xl space-y-4"
    onsubmit={submit}
  >
    <h1 class="text-xl font-bold text-slate-50 tracking-widest uppercase text-center">Create Account</h1>
    <p class="text-xs text-slate-500 text-center">
      Your data is stored under ~/Documents/PMForge/&lt;username&gt;/ on this machine.
    </p>

    {#if !hasAdmin}
      <div class="bg-amber-950/40 border border-amber-700/50 rounded-lg p-3 text-xs text-amber-300 space-y-1">
        <p class="font-semibold">No administrator configured</p>
        <p class="text-amber-400/80">
          This machine has no PMForge administrator yet. You may claim that role below.
          Administrators can create and delete accounts and manage other users.
        </p>
      </div>
    {/if}

    <label class="block">
      <span class="text-xs font-semibold text-slate-500 uppercase">Username</span>
      <input
        type="text"
        autocomplete="username"
        bind:value={username}
        class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
      />
      <span class="text-[10px] text-slate-500">3–32 chars; letters, digits, _ or -</span>
    </label>

    <label class="block">
      <span class="text-xs font-semibold text-slate-500 uppercase">Display Name</span>
      <input
        type="text"
        bind:value={displayName}
        class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
      />
    </label>

    <label class="block">
      <span class="text-xs font-semibold text-slate-500 uppercase">Password</span>
      <input
        type="password"
        autocomplete="new-password"
        bind:value={password}
        class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
      />
    </label>

    <label class="block">
      <span class="text-xs font-semibold text-slate-500 uppercase">Confirm Password</span>
      <input
        type="password"
        autocomplete="new-password"
        bind:value={confirm}
        class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
      />
    </label>

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
      <p class="text-xs text-red-400" role="alert">{error}</p>
    {/if}

    <button
      type="submit"
      disabled={busy}
      class="w-full bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold py-2 rounded transition-all"
    >
      {busy ? 'Creating...' : 'CREATE ACCOUNT'}
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
