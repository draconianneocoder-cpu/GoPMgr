<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
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
  // DEVELOPER_HANDBOOK.md §10.5: every timer must be cleared on destroy.
  let copiedTimer: ReturnType<typeof setTimeout> | null = null;
  onDestroy(() => {
    if (copiedTimer) clearTimeout(copiedTimer);
  });

  // Per-row action state
  let pendingRoleChange = $state<string | null>(null);
  let pendingDisable = $state<string | null>(null);
  // Permanent deletion needs the username typed exactly (the backend
  // checks it too), so it opens its own panel instead of a two-click confirm.
  let purgeTarget = $state<string | null>(null);
  let purgeTyped = $state('');
  let purging = $state(false);

  let events = $state<AccountEvent[]>([]);
  let eventsError = $state('');

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
    await loadEvents();
  }

  async function loadEvents() {
    eventsError = '';
    try {
      events = (await window.go.main.App.AdminListAccountEvents()) ?? [];
    } catch (err: any) {
      eventsError = `Could not load account history: ${err}`;
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
      if (copiedTimer) clearTimeout(copiedTimer);
      copiedTimer = setTimeout(() => (copied = false), 2000);
    } catch {
      // Clipboard may be unavailable; the codes stay visible for manual copy.
    }
  }

  function downloadCodes() {
    const body = `GoPMgr recovery codes for ${createdFor}\n\n${createdCodes.join('\n')}\n`;
    const url = URL.createObjectURL(new Blob([body], { type: 'text/plain' }));
    const a = document.createElement('a');
    a.href = url;
    a.download = `gopmgr-recovery-codes-${createdFor}.txt`;
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

  async function toggleDisabled(user: Account) {
    if (pendingDisable !== user.username) {
      cancelPending(user.username);
      pendingDisable = user.username;
      return;
    }
    pendingDisable = null;
    const disable = !user.disabled;
    try {
      await window.go.main.App.AdminSetUserDisabled(user.username, disable);
      showToast(
        disable
          ? `${user.username} is disabled. Their projects are kept.`
          : `${user.username} can sign in again.`,
        'success'
      );
      await load();
    } catch (err: any) {
      showToast(`Could not ${disable ? 'disable' : 'enable'} ${user.username}: ${err}`, 'error');
    }
  }

  function startPurge(username: string) {
    cancelPending(username);
    purgeTarget = username;
    purgeTyped = '';
  }

  async function purge(username: string) {
    if (purgeTyped !== username || purging) return;
    purging = true;
    try {
      await window.go.main.App.AdminPurgeUser(username, purgeTyped);
      showToast(`${username} and their folder were permanently deleted.`, 'success');
      purgeTarget = null;
      purgeTyped = '';
    } catch (err: any) {
      const message = String(err?.message ?? err);
      // The account is gone but part of its folder is not: say so plainly
      // rather than calling it a failed delete.
      if (message.startsWith('the account was deleted')) {
        showToast(message.charAt(0).toUpperCase() + message.slice(1), 'error');
        purgeTarget = null;
        purgeTyped = '';
      } else {
        showToast(`Delete failed: ${message}`, 'error');
      }
    } finally {
      purging = false;
      await load();
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
    if (pendingRoleChange === username) pendingRoleChange = null;
    if (pendingDisable === username) pendingDisable = null;
    if (purgeTarget === username) {
      purgeTarget = null;
      purgeTyped = '';
    }
  }

  const eventLabels: Record<string, string> = {
    disabled: 'disabled',
    enabled: 'enabled',
    purged: 'permanently deleted',
    folder_not_removed: 'could not fully remove the folder of',
  };

  function formatEventTime(value: string): string {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
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
          Administrators create, disable, and delete accounts and manage roles on this machine.
        </p>
      </div>
      <button
        onclick={() => { showCreateForm = !showCreateForm; pendingRoleChange = null; pendingDisable = null; purgeTarget = null; }}
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
                  {#if user.disabled}
                    <span class="ml-1 inline-flex items-center text-[11px] font-semibold text-slate-300 bg-slate-800 border border-slate-700 rounded px-2 py-0.5">
                      Disabled
                    </span>
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
                          class="text-[11px] bg-amber-800 hover:bg-amber-700 text-white px-2 py-0.5 rounded"
                        >Confirm</button>
                        <button
                          onclick={() => cancelPending(user.username)}
                          class="text-[11px] text-slate-400 hover:text-slate-200 underline"
                        >Cancel</button>
                      {:else if pendingDisable === user.username}
                        <span class="text-[11px] text-amber-400">
                          {user.disabled ? 'Let them sign in again?' : 'Disable? Projects are kept.'}
                        </span>
                        <button
                          onclick={() => toggleDisabled(user)}
                          class="text-[11px] bg-amber-800 hover:bg-amber-700 text-white px-2 py-0.5 rounded"
                        >Confirm</button>
                        <button
                          onclick={() => cancelPending(user.username)}
                          class="text-[11px] text-slate-400 hover:text-slate-200 underline"
                        >Cancel</button>
                      {:else if purgeTarget !== user.username}
                        <button
                          onclick={() => toggleRole(user)}
                          class="text-[11px] text-slate-400 hover:text-amber-400 underline"
                          title={user.is_admin ? 'Remove administrator' : 'Grant administrator'}
                        >
                          {user.is_admin ? 'Remove admin' : 'Grant admin'}
                        </button>
                        <button
                          onclick={() => toggleDisabled(user)}
                          class="text-[11px] text-slate-400 hover:text-amber-400 underline"
                          aria-label={`${user.disabled ? 'Enable' : 'Disable'} account ${user.username}`}
                        >{user.disabled ? 'Enable' : 'Disable'}</button>
                        <button
                          onclick={() => startPurge(user.username)}
                          class="text-[11px] text-slate-400 hover:text-red-400 underline"
                          aria-label={`Delete account ${user.username} permanently`}
                        >Delete permanently</button>
                      {/if}
                    </div>
                  {/if}
                </td>
              </tr>
              {#if purgeTarget === user.username}
                <tr class="border-b border-slate-800/60 bg-red-950/20">
                  <td colspan="5" class="px-4 py-3">
                    <div class="space-y-2" role="group" aria-label={`Permanently delete ${user.username}`}>
                      <p class="text-xs font-semibold text-red-300">Permanently delete {user.username}?</p>
                      <p class="text-xs text-slate-300">
                        This deletes the account and its folder: all of {user.username}'s projects
                        (including encrypted ones), certificates, exports, and recovery codes. It can't be
                        undone. To keep the data, disable the account instead.
                      </p>
                      <label for={`purge-confirm-${user.username}`} class="block text-[11px] text-slate-400">
                        Type <span class="font-mono text-slate-200">{user.username}</span> to confirm
                      </label>
                      <div class="flex items-center gap-2">
                        <input
                          id={`purge-confirm-${user.username}`}
                          type="text"
                          autocomplete="off"
                          spellcheck="false"
                          bind:value={purgeTyped}
                          class="w-48 bg-slate-950 border border-slate-800 p-1.5 rounded text-xs font-mono focus:border-red-500 outline-none"
                        />
                        <button
                          onclick={() => purge(user.username)}
                          disabled={purgeTyped !== user.username || purging}
                          class="text-[11px] bg-red-700 hover:bg-red-600 disabled:opacity-50 disabled:cursor-not-allowed text-white px-2 py-1 rounded"
                        >{purging ? 'Deleting…' : 'Delete permanently'}</button>
                        <button
                          onclick={() => cancelPending(user.username)}
                          class="text-[11px] text-slate-400 hover:text-slate-200 underline"
                        >Cancel</button>
                      </div>
                    </div>
                  </td>
                </tr>
              {/if}
            {/each}
          </tbody>
        </table>
        {#if allUsers.length === 0}
          <p class="text-center text-slate-500 text-xs py-6">No accounts found.</p>
        {/if}
      </div>

      <section class="space-y-2" aria-labelledby="account-history-heading">
        <h2 id="account-history-heading" class="text-xs font-bold uppercase tracking-widest text-slate-400">Account history</h2>
        {#if eventsError}
          <p class="text-xs text-red-400" role="alert">{eventsError}</p>
        {:else if events.length === 0}
          <p class="text-xs text-slate-500">No accounts have been disabled, enabled, or deleted.</p>
        {:else}
          <ul class="text-xs text-slate-300 space-y-1">
            {#each events as event (event.id)}
              <li>
                <span class="text-slate-500 font-mono">{formatEventTime(event.occurred_at)}</span>
                — {event.actor} {eventLabels[event.action] ?? event.action} {event.username}
                {#if event.detail}
                  <span class="block text-[11px] text-slate-500 font-mono break-all">{event.detail}</span>
                {/if}
              </li>
            {/each}
          </ul>
        {/if}
      </section>
    {/if}
  </main>
</div>
