<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { session, goto } from '../../session.svelte';

  let projects = $state<ProjectFile[]>([]);
  let creating = $state(false);
  let newName = $state('');
  let newDesc = $state('');
  let error = $state('');

  onMount(refresh);

  async function refresh() {
    try {
      projects = (await window.go.main.App.ListProjects()) ?? [];
    } catch (err: any) {
      error = `Could not list projects: ${err}`;
    }
  }

  async function open(p: ProjectFile) {
    error = '';
    try {
      const meta = await window.go.main.App.OpenProject(p.path);
      session.project = meta;
      session.projectPath = p.path;
      goto('dashboard');
    } catch (err: any) {
      error = String(err?.message ?? err);
    }
  }

  async function createProject(e: Event) {
    e.preventDefault();
    error = '';
    try {
      const p = await window.go.main.App.CreateProject(newName, newDesc);
      newName = '';
      newDesc = '';
      creating = false;
      await refresh();
      await open(p);
    } catch (err: any) {
      error = String(err?.message ?? err);
    }
  }

  async function logout() {
    await window.go.main.App.Logout();
    session.user = null;
    session.project = null;
    session.projectPath = null;
    goto('login');
  }
</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <header class="border-b border-slate-800 px-6 py-4 flex items-center justify-between">
    <div>
      <h1 class="text-lg font-bold tracking-widest uppercase">PMForge</h1>
      <p class="text-xs text-slate-500">Signed in as {session.user?.display_name ?? session.user?.username}</p>
    </div>
    <button onclick={logout} class="text-xs text-slate-400 hover:text-cyan-400 underline">
      Sign out
    </button>
  </header>

  <main class="max-w-3xl mx-auto p-8">
    <div class="flex items-center justify-between mb-6">
      <h2 class="text-xl font-bold">Your projects</h2>
      <button
        onclick={() => goto('launchpad')}
        class="bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-bold uppercase tracking-wider px-3 py-2 rounded"
      >
        + New Project
      </button>
    </div>

    {#if error}
      <p class="text-sm text-red-400 mb-4" role="alert">{error}</p>
    {/if}

    {#if creating}
      <form
        onsubmit={createProject}
        class="p-4 bg-slate-900 border border-slate-800 rounded-lg space-y-3 mb-6"
      >
        <label class="block">
          <span class="text-xs font-semibold text-slate-500 uppercase">Project Name</span>
          <input
            type="text"
            bind:value={newName}
            required
            class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          />
        </label>
        <label class="block">
          <span class="text-xs font-semibold text-slate-500 uppercase">Description</span>
          <textarea
            bind:value={newDesc}
            rows="2"
            class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          ></textarea>
        </label>
        <button
          type="submit"
          disabled={!newName}
          class="bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white text-xs font-bold uppercase tracking-wider px-3 py-2 rounded"
        >
          Create
        </button>
      </form>
    {/if}

    {#if projects.length === 0}
      <p class="text-sm text-slate-500 text-center py-12">
        No projects yet. Click <strong>+ New Project</strong> to get started.
      </p>
    {:else}
      <ul class="space-y-2">
        {#each projects as p (p.path)}
          <li>
            <button
              onclick={() => open(p)}
              class="w-full text-left p-4 bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded-lg flex items-center justify-between"
            >
              <div>
                <div class="font-bold text-white">{p.name}</div>
                <div class="text-xs text-slate-500">{p.path}</div>
              </div>
              <div class="text-xs text-slate-500">{p.modified}</div>
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </main>
</div>
