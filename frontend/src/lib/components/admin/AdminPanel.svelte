<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import AppHeader from '../AppHeader.svelte';
  import RecoveryCodeList from '../auth/RecoveryCodeList.svelte';
  import AdminUserDataViewer from './AdminUserDataViewer.svelte';
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

  // Recorded access to another account's data (ADR-004): the row whose
  // reason form is open, and the account being viewed.
  const maxReasonLength = 500;
  let accessTarget = $state<string | null>(null);
  let accessReason = $state('');
  let openingAccess = $state(false);
  let viewing = $state<string | null>(null);

  // The super administrator (ADR-005): the only administrator who changes
  // administrators and opens users' data; every other one is a subordinate
  // who manages standard accounts.
  let superAdmin = $state('');
  let pendingHandOver = $state<string | null>(null);
  let handingOver = $state(false);

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
    try {
      superAdmin = (await window.go.main.App.AdminRoles()).super;
    } catch {
      superAdmin = '';
    }
    await loadEvents();
  }

  function startAccess(username: string) {
    cancelPending(username);
    accessTarget = username;
    accessReason = '';
  }

  async function openAccess(username: string) {
    if (!accessReason.trim() || accessReason.length > maxReasonLength || openingAccess) return;
    openingAccess = true;
    try {
      await window.go.main.App.AdminOpenUserData(username, accessReason);
      accessTarget = null;
      accessReason = '';
      viewing = username;
    } catch (err: any) {
      showToast(`Could not open ${username}'s data: ${err}`, 'error');
    } finally {
      openingAccess = false;
      await loadEvents();
    }
  }

  async function stoppedViewing() {
    viewing = null;
    await loadEvents();
  }

  async function handOver(username: string) {
    if (pendingHandOver !== username) {
      cancelPending(username);
      pendingHandOver = username;
      return;
    }
    handingOver = true;
    try {
      const result = await window.go.main.App.AdminHandOverSuper(username);
      // Any data this administrator had open was closed with the role.
      viewing = null;
      showToast(
        result.key_passed
          ? `${username} is now the super administrator. You're a subordinate administrator.`
          : `${username} is now the super administrator, but the administrator key couldn't be passed, so no one can open users' data until it is replaced.`,
        result.key_passed ? 'success' : 'error'
      );
      pendingHandOver = null;
      await load();
    } catch (err: any) {
      showToast(`Could not hand over the role: ${err}`, 'error');
    } finally {
      handingOver = false;
    }
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
      // fails, and the user can create codes in App Settings, under Account.
      try {
        createdCodes = (await window.go.main.App.AdminIssueRecoveryCodes(uname, pw)) ?? [];
        createdFor = uname;
      } catch (err: any) {
        showToast(`Account created, but recovery codes could not be generated: ${err}. The user can create them in App Settings, under Account.`, 'error');
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

  function dismissCodes() {
    createdCodes = [];
    createdFor = '';
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
    if (pendingHandOver === username) pendingHandOver = null;
    if (accessTarget === username) {
      accessTarget = null;
      accessReason = '';
    }
    if (purgeTarget === username) {
      purgeTarget = null;
      purgeTyped = '';
    }
  }

  // One sentence per history entry. An action this version does not know
  // (written by a newer GoPMgr) falls back to its raw name.
  function describeEvent(e: AccountEvent): string {
    const self = e.actor === e.username;
    switch (e.action) {
      case 'created':
        return self
          ? `${e.username} created the first account (${e.detail})`
          : `${e.actor} created ${e.username} (${e.detail})`;
      case 'promoted':
        return self
          ? `${e.username} claimed the administrator role`
          : `${e.actor} made ${e.username} an administrator`;
      case 'demoted':
        return `${e.actor} removed ${e.username}'s administrator role`;
      case 'disabled':
        return `${e.actor} disabled ${e.username}`;
      case 'enabled':
        return `${e.actor} enabled ${e.username}`;
      case 'purged':
        return `${e.actor} permanently deleted ${e.username}`;
      case 'folder_not_removed':
        return `${e.actor} could not fully remove the folder of ${e.username}`;
      case 'admin_access':
        return `${e.actor} opened ${e.username}'s data as an administrator`;
      case 'escrow_key_mismatch':
        return `A key check failed for ${e.username}; GoPMgr's key records may have been changed outside GoPMgr`;
      case 'escrow_reenrolled':
        return `${e.username}'s key records were missing and were made again at sign-in`;
      case 'personal_key_repaired':
        return `${e.username}'s account key was changed outside GoPMgr and was repaired at sign-in`;
      case 'personal_key_trusted':
        return `${e.actor} accepted ${e.username}'s account key without an earlier check`;
      case 'escrow_rotated':
        return `${e.actor} replaced the administrator key`;
      case 'super_admin_assigned':
        return `${e.username} became the super administrator (${e.detail})`;
      case 'super_admin_handed_over':
        return `${e.actor} handed the super administrator role to ${e.username}`;
      case 'access_notice_read':
        return `${e.username} read the notice that their data was opened (${e.detail})`;
      default:
        return `${e.actor} ${e.action} ${e.username}`;
    }
  }

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
  // Your own row has no actions: the backend refuses changes to your own
  // account, which is also what keeps at least one administrator able to
  // sign in. Counted like the backend's guard: disabled admins don't count.
  const iAmSuper = $derived(superAdmin !== '' && session.user?.username === superAdmin);
  // Subordinates manage standard accounts; administrators are the super
  // administrator's to change.
  const canManage = (u: Account) => iAmSuper || !u.is_admin;
</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <AppHeader active="admin" />

  <main class="max-w-3xl mx-auto p-8 space-y-6">
    <div class="flex items-center justify-between gap-4">
      <div>
        <h1 class="text-xl font-bold">User management</h1>
        <p class="text-xs text-slate-500 mt-0.5">
          Administrators create, disable, and delete accounts on this machine.
          {#if superAdmin && !iAmSuper}
            Only the super administrator, {superAdmin}, changes administrators and can open users' data.
          {/if}
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
        {#if iAmSuper}
          <label class="flex items-center gap-2">
            <input type="checkbox" bind:checked={newIsAdmin} class="accent-cyan-500" />
            <span class="text-xs text-slate-300">Administrator account</span>
          </label>
        {/if}
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
        <!-- Keyed so a second account's codes start without the last save's message. -->
        {#key createdCodes}
          <RecoveryCodeList username={createdFor} codes={createdCodes} />
        {/key}
        <button
          type="button"
          onclick={dismissCodes}
          class="text-xs font-semibold uppercase tracking-wide bg-cyan-600 hover:bg-cyan-500 text-white px-3 py-1.5 rounded transition-colors"
        >
          Done
        </button>
      </div>
    {/if}

    {#if viewing}
      <AdminUserDataViewer username={viewing} onstop={stoppedViewing} />
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
                  {#if user.username === superAdmin}
                    <span class="inline-flex items-center gap-1 text-[11px] font-semibold text-amber-300 bg-amber-900/50 border border-amber-500/60 rounded px-2 py-0.5">
                      Super administrator
                    </span>
                  {:else if user.is_admin}
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
                  {#if !isSelf(user) && !canManage(user)}
                    <p class="text-[11px] text-slate-500 text-right">Only the super administrator can change administrators.</p>
                  {:else if !isSelf(user)}
                    <div class="flex items-center justify-end gap-2">
                      {#if pendingHandOver === user.username}
                        <span class="text-[11px] text-amber-400">
                          Make {user.username} super administrator? You'll become a subordinate administrator.
                        </span>
                        <button
                          onclick={() => handOver(user.username)}
                          disabled={handingOver}
                          class="text-[11px] bg-amber-800 hover:bg-amber-700 disabled:opacity-50 text-white px-2 py-0.5 rounded"
                        >Confirm</button>
                        <button
                          onclick={() => cancelPending(user.username)}
                          class="text-[11px] text-slate-400 hover:text-slate-200 underline"
                        >Cancel</button>
                      {:else if pendingRoleChange === user.username}
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
                      {:else if purgeTarget !== user.username && accessTarget !== user.username}
                        {#if iAmSuper && user.is_admin && !user.disabled}
                          <button
                            onclick={() => handOver(user.username)}
                            class="text-[11px] text-slate-400 hover:text-amber-400 underline"
                            aria-label={`Make ${user.username} super administrator`}
                          >Make super administrator</button>
                        {/if}
                        {#if iAmSuper && !viewing}
                          <!-- One account at a time: stop viewing before opening another. -->
                          <button
                            onclick={() => startAccess(user.username)}
                            class="text-[11px] text-slate-400 hover:text-amber-400 underline"
                            aria-label={`Open ${user.username}'s data`}
                          >Open data</button>
                        {/if}
                        {#if iAmSuper}
                          <button
                            onclick={() => toggleRole(user)}
                            class="text-[11px] text-slate-400 hover:text-amber-400 underline"
                            title={user.is_admin ? 'Remove administrator' : 'Grant administrator'}
                          >
                            {user.is_admin ? 'Remove admin' : 'Grant admin'}
                          </button>
                        {/if}
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
                  {:else}
                    <p class="text-[11px] text-slate-500 text-right">
                      {iAmSuper
                        ? "You're the super administrator, so no one can change your account. To step down, make another administrator who has signed in the super administrator."
                        : 'Only the super administrator can change your account.'}
                    </p>
                  {/if}
                </td>
              </tr>
              {#if accessTarget === user.username}
                <tr class="border-b border-slate-800/60 bg-amber-950/10">
                  <td colspan="5" class="px-4 py-3">
                    <div class="space-y-2" role="group" aria-label={`Open ${user.username}'s data`}>
                      <p class="text-xs font-semibold text-amber-300">Open {user.username}'s data?</p>
                      <p class="text-xs text-slate-300">
                        You'll see {user.username}'s projects read only: you can't change, export, or print
                        anything. Opening is recorded in the account history with your reason.
                      </p>
                      <label for={`access-reason-${user.username}`} class="block text-[11px] text-slate-400">
                        Reason (required)
                      </label>
                      <textarea
                        id={`access-reason-${user.username}`}
                        bind:value={accessReason}
                        maxlength={maxReasonLength}
                        rows="2"
                        class="w-full bg-slate-950 border border-slate-800 p-1.5 rounded text-xs focus:border-amber-500 outline-none"
                      ></textarea>
                      <div class="flex items-center gap-2">
                        <button
                          onclick={() => openAccess(user.username)}
                          disabled={!accessReason.trim() || openingAccess}
                          class="text-[11px] bg-amber-700 hover:bg-amber-600 disabled:opacity-50 disabled:cursor-not-allowed text-white px-2 py-1 rounded"
                        >{openingAccess ? 'Opening…' : 'Open data'}</button>
                        <button
                          onclick={() => cancelPending(user.username)}
                          class="text-[11px] text-slate-400 hover:text-slate-200 underline"
                        >Cancel</button>
                        <span class="ml-auto text-[10px] text-slate-500">{accessReason.length}/{maxReasonLength}</span>
                      </div>
                    </div>
                  </td>
                </tr>
              {/if}
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
          <p class="text-xs text-slate-500">No account changes yet.</p>
        {:else}
          <ul class="text-xs text-slate-300 space-y-1">
            {#each events as event (event.id)}
              <li>
                <span class="text-slate-500 font-mono">{formatEventTime(event.occurred_at)}</span>
                — {describeEvent(event)}
                {#if event.action === 'admin_access' && event.detail}
                  <span class="block text-[11px] text-slate-400 break-words">Reason: {event.detail}</span>
                {:else if (event.action === 'folder_not_removed' || event.action === 'escrow_key_mismatch' || event.action === 'personal_key_repaired') && event.detail}
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
