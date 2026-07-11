<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import AppHeader from '../AppHeader.svelte';
  import Spinner from '../Spinner.svelte';
  import { session } from '../../session.svelte';
  import { showToast } from '../../toast.svelte';

  let allUsers = $state<Account[]>([]);
  let loading = $state(true);
  let error = $state('');

  // New user form
  let showCreateForm = $state(false);
  let newUsername = $state('');
  let newDisplayName = $state('');
  let newPassword = $state('');
  let newIsAdmin = $state(false);
  let creating = $state(false);

  // Recovery codes for the just-created account, shown once for the admin to
  // hand to the user (an admin-created account otherwise gets none).
  let createdCodes = $state<string[]>([]);
  let createdFor = $state('');
  let copied = $state(false);

  // Per-row action state
  let pendingDelete = $state<string | null>(null);
  let pendingRoleChange = $state<string | null>(null);

  const usernameRule = /^[A-Za-z0-9_-]{3,32}$/;

  onMount(load);

  async function load() {
    loading = true;
    error = '';
    try {
      allUsers = await window.go.main.App.AdminListUsers();
    } catch (err: any) {
      error = `Could not load users: ${err}`;
    } finally {
      loading = false;
    }
  }

  async function createUser(e: Event) {
    e.preventDefault();
    if (!usernameRule.test(newUsername)) {
      showToast('Username must be 3–32 letters, digits, _ or -.', 'error');
      return;
    }
    if (newPassword.length < 8) {
      showToast('Password must be at least 8 characters.', 'error');
      return;
    }
    creating = true;
    const uname = newUsername;
    const pw = newPassword;
    try {
      await window.go.main.App.CreateAccount(uname, newDisplayName || uname, pw, newIsAdmin);
      // Issue recovery codes for the new account (same footing as a
      // self-registered user). Non-fatal: the account exists even if this
      // fails, and the user can generate codes later from Project Settings.
      try {
        createdCodes = (await window.go.main.App.AdminIssueRecoveryCodes(uname, pw)) ?? [];
        createdFor = uname;
      } catch (err: any) {
        showToast(`Account created, but recovery codes could not be generated: ${err}. The user can create them from Project Settings.`, 'error');
      }
      showToast(`Account "${uname}" created.`, 'success');
      newUsername = '';
      newDisplayName = '';
      newPassword = '';
      newIsAdmin = false;
      showCreateForm = false;
      await load();
    } catch (err: any) {
      showToast(`Create failed: ${err}`, 'error');
    } finally {
      creating = false;
    }
  }

  async function copyCodes() {
    try {
      await navigator.clipboard.writeText(createdCodes.join('\n'));
      copied = true;
      setTimeout(() => (copied = false), 2000);
    } catch {
      // Clipboard may be unavailable; the codes stay visible for manual copy.
    }
  }

  function downloadCodes() {
    const body = `PMForge recovery codes for ${createdFor}\n\n${createdCodes.join('\n')}\n`;
    const url = URL.createObjectURL(new Blob([body], { type: 'text/plain' }));
    const a = document.createElement('a');
    a.href = url;
    a.download = `pmforge-recovery-codes-${createdFor}.txt`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  }

  function dismissCodes() {
    createdCodes = [];
    createdFor = '';
    copied = false;
  }

  async function confirmDelete(username: string) {
    if (pendingDelete !== username) {
      pendingDelete = username;
      return;
    }
    pendingDelete = null;
    try {
      await window.go.main.App.AdminDeleteUser(username);
      showToast(`Account "${username}" deleted.`, 'success');
      await load();
    } catch (err: any) {
      showToast(`Delete failed: ${err}`, 'error');
    }
  }

  async function toggleRole(user: Account) {
    if (pendingRoleChange !== user.username) {
      pendingRoleChange = user.username;
      return;
    }
    pendingRoleChange = null;
    const newRole = !user.is_admin;
    try {
      await window.go.main.App.AdminSetUserRole(user.username, newRole);
      showToast(
        `${user.username} is now ${newRole ? 'an administrator' : 'a standard user'}.`,
        'success'
      );
      await load();
    } catch (err: any) {
      showToast(`Role change failed: ${err}`, 'error');
    }
  }

  function cancelPending(username: string) {
    if (pendingDelete === username) pendingDelete = null;
    if (pendingRoleChange === username) pendingRoleChange = null;
  }

  function formatLastLogin(value: string): string {
    if (!value) return 'Never';
    const date = new Date(value);
    if (Number.isNaN(date.getTime()) || date.getFullYear() <= 1) return 'Never';
    return date.toLocaleDateString();
  }

  const isSelf = (u: Account) => u.username === session.user?.username;
</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <AppHeader active="admin" />

  <main class="max-w-3xl mx-auto p-8 space-y-6">
    <div class="flex items-center justify-between gap-4">
      <div>
        <h1 class="text-xl font-bold">User management</h1>
        <p class="text-xs text-slate-500 mt-0.5">
          Administrators can create and delete accounts and manage roles on this machine.
        </p>
      </div>
      <button
        onclick={() => { showCreateForm = !showCreateForm; pendingDelete = null; pendingRoleChange = null; }}
        class="bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-bold uppercase tracking-wider px-4 py-2 rounded shrink-0"
      >
        {showCreateForm ? 'Cancel' : 'Create user'}
      </button>
    </div>

    {#if error}
      <p class="text-sm text-red-400" role="alert">{error}</p>
    {/if}

    {#if showCreateForm}
      <form
        onsubmit={createUser}
        class="p-4 bg-slate-900 border border-slate-800 rounded-lg space-y-4"
      >
        <h2 class="text-xs font-bold uppercase tracking-widest text-cyan-400">New account</h2>
        <div class="grid grid-cols-2 gap-4">
          <label class="block">
            <span class="text-xs font-semibold text-slate-500 uppercase">Username</span>
            <input
              type="text"
              autocomplete="off"
              bind:value={newUsername}
              placeholder="username"
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded text-sm focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-xs font-semibold text-slate-500 uppercase">Display name</span>
            <input
              type="text"
              autocomplete="off"
              bind:value={newDisplayName}
              placeholder="Full Name"
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded text-sm focus:border-cyan-500 outline-none"
            />
          </label>
        </div>
        <label class="block">
          <span class="text-xs font-semibold text-slate-500 uppercase">Initial password</span>
          <input
            type="password"
            autocomplete="new-password"
            bind:value={newPassword}
            class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded text-sm focus:border-cyan-500 outline-none"
          />
          <span class="text-[10px] text-slate-500">8 characters minimum. Share it securely; the user should change it.</span>
        </label>
        <label class="flex items-center gap-2">
          <input type="checkbox" bind:checked={newIsAdmin} class="accent-cyan-500" />
          <span class="text-xs text-slate-300">Administrator account</span>
        </label>
        <div class="flex gap-2 pt-1">
          <button
            type="submit"
            disabled={creating}
            class="bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white text-xs font-bold uppercase tracking-wider px-4 py-2 rounded"
          >
            {creating ? 'Creating…' : 'Create account'}
          </button>
        </div>
      </form>
    {/if}

    {#if createdCodes.length > 0}
      <div class="p-4 bg-cyan-950/20 border border-cyan-900/60 rounded-lg space-y-3">
        <div>
          <h2 class="text-xs font-bold uppercase tracking-widest text-cyan-300">
            Recovery codes for {createdFor}
          </h2>
          <p class="text-xs text-cyan-300/80 mt-1">
            Give these to {createdFor} to store somewhere safe. They are the only way to recover the
            account if the password is lost, and they won't be shown again.
          </p>
        </div>
        <ul class="grid grid-cols-2 sm:grid-cols-4 gap-1.5 font-mono text-xs text-slate-100 bg-slate-950 border border-slate-800 rounded p-3">
          {#each createdCodes as code (code)}
            <li>{code}</li>
          {/each}
        </ul>
        <div class="flex items-center gap-2">
          <button
            type="button"
            onclick={copyCodes}
            class="text-xs font-semibold uppercase tracking-wide bg-slate-800 hover:bg-slate-700 text-slate-100 px-3 py-1.5 rounded transition-colors"
          >
            {copied ? 'Copied ✓' : 'Copy'}
          </button>
          <button
            type="button"
            onclick={downloadCodes}
            class="text-xs font-semibold uppercase tracking-wide bg-slate-800 hover:bg-slate-700 text-slate-100 px-3 py-1.5 rounded transition-colors"
          >
            Download .txt
          </button>
          <button
            type="button"
            onclick={dismissCodes}
            class="ml-auto text-xs font-semibold uppercase tracking-wide bg-cyan-600 hover:bg-cyan-500 text-white px-3 py-1.5 rounded transition-colors"
          >
            Done
          </button>
        </div>
        <p class="text-[10px] text-slate-500" aria-live="polite">
          {copied ? 'Recovery codes copied to the clipboard.' : ''}
        </p>
      </div>
    {/if}

    {#if loading}
      <Spinner label="Loading users…" class="py-8" />
    {:else}
      <div class="bg-slate-900 border border-slate-800 rounded-lg overflow-hidden">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-slate-800 text-xs text-slate-500 uppercase tracking-wider">
              <th class="text-left px-4 py-3">Username</th>
              <th class="text-left px-4 py-3">Display name</th>
              <th class="text-left px-4 py-3">Role</th>
              <th class="text-left px-4 py-3">Last login</th>
              <th class="px-4 py-3"></th>
            </tr>
          </thead>
          <tbody>
            {#each allUsers as user (user.username)}
              <tr class="border-b border-slate-800/60 last:border-0 hover:bg-slate-800/30">
                <td class="px-4 py-3 font-mono text-xs">
                  {user.username}
                  {#if isSelf(user)}
                    <span class="ml-1 text-[10px] text-cyan-500 font-sans">(you)</span>
                  {/if}
                </td>
                <td class="px-4 py-3 text-slate-300">{user.display_name}</td>
                <td class="px-4 py-3">
                  {#if user.is_admin}
                    <span class="inline-flex items-center gap-1 text-[11px] font-semibold text-amber-400 bg-amber-900/30 border border-amber-700/40 rounded px-2 py-0.5">
                      Admin
                    </span>
                  {:else}
                    <span class="text-[11px] text-slate-500">Standard</span>
                  {/if}
                </td>
                <td class="px-4 py-3 text-[11px] text-slate-500 font-mono">
                  {formatLastLogin(user.last_login)}
                </td>
                <td class="px-4 py-3">
                  {#if !isSelf(user)}
                    <div class="flex items-center justify-end gap-2">
                      {#if pendingRoleChange === user.username}
                        <span class="text-[11px] text-amber-400">
                          {user.is_admin ? 'Remove admin?' : 'Grant admin?'}
                        </span>
                        <button
                          onclick={() => toggleRole(user)}
                          class="text-[11px] bg-amber-700 hover:bg-amber-600 text-white px-2 py-0.5 rounded"
                        >Confirm</button>
                        <button
                          onclick={() => cancelPending(user.username)}
                          class="text-[11px] text-slate-400 hover:text-slate-200 underline"
                        >Cancel</button>
                      {:else if pendingDelete === user.username}
                        <span class="text-[11px] text-red-400">Delete account?</span>
                        <button
                          onclick={() => confirmDelete(user.username)}
                          class="text-[11px] bg-red-700 hover:bg-red-600 text-white px-2 py-0.5 rounded"
                        >Confirm</button>
                        <button
                          onclick={() => cancelPending(user.username)}
                          class="text-[11px] text-slate-400 hover:text-slate-200 underline"
                        >Cancel</button>
                      {:else}
                        <button
                          onclick={() => toggleRole(user)}
                          class="text-[11px] text-slate-400 hover:text-amber-400 underline"
                          title={user.is_admin ? 'Remove administrator' : 'Grant administrator'}
                        >
                          {user.is_admin ? 'Remove admin' : 'Grant admin'}
                        </button>
                        <button
                          onclick={() => confirmDelete(user.username)}
                          class="text-[11px] text-slate-400 hover:text-red-400 underline"
                          aria-label={`Delete account ${user.username}`}
                        >Delete</button>
                      {/if}
                    </div>
                  {/if}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
        {#if allUsers.length === 0}
          <p class="text-center text-slate-500 text-xs py-6">No accounts found.</p>
        {/if}
      </div>
    {/if}
  </main>
</div>
