<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import AppHeader from '../AppHeader.svelte';

  let projects = $state<ProjectFile[]>([]);
  let creating = $state(false);
  let newName = $state('');
  let newDesc = $state('');
  let error = $state('');
  // Path of the project whose Delete button is awaiting a confirming second
  // click (two-step delete so a destructive action can't fire by accident).
  let confirmingDelete = $state<string | null>(null);
  // Path of a project with a clone/delete request in flight (disables its row).
  let busyPath = $state<string | null>(null);

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

  async function clone(p: ProjectFile) {
    error = '';
    busyPath = p.path;
    try {
      await window.go.main.App.CloneProject(p.path);
      await refresh();
    } catch (err: any) {
      error = `Clone failed: ${err}`;
    } finally {
      busyPath = null;
    }
  }

  async function confirmDelete(p: ProjectFile) {
    error = '';
    busyPath = p.path;
    try {
      await window.go.main.App.DeleteProject(p.path);
      confirmingDelete = null;
      // If the deleted project was the open one, drop it from the session.
      if (session.projectPath === p.path) {
        session.project = null;
        session.projectPath = null;
      }
      await refresh();
    } catch (err: any) {
      error = `Delete failed: ${err}`;
    } finally {
      busyPath = null;
    }
  }
</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <AppHeader active="projects" />

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
          <li class="flex items-stretch gap-2" class:opacity-50={busyPath === p.path}>
            <button
              onclick={() => open(p)}
              disabled={busyPath === p.path}
              class="flex-1 min-w-0 text-left p-4 bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded-lg flex items-center justify-between gap-4"
            >
              <div class="min-w-0">
                <div class="font-bold text-slate-50 truncate">{p.name}</div>
                <div class="text-xs text-slate-500 truncate">{p.path}</div>
              </div>
              <div class="text-xs text-slate-500 shrink-0">{p.modified}</div>
            </button>

            <div class="flex flex-col justify-center gap-1 shrink-0">
              {#if confirmingDelete === p.path}
                <button
                  onclick={() => confirmDelete(p)}
                  disabled={busyPath === p.path}
                  class="text-[11px] font-bold uppercase tracking-wider px-3 py-1.5 rounded bg-red-600 hover:bg-red-500 disabled:opacity-50 text-white"
                  aria-label={`Confirm delete ${p.name}`}
                >
                  Confirm
                </button>
                <button
                  onclick={() => (confirmingDelete = null)}
                  class="text-[11px] uppercase tracking-wider px-3 py-1.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-300"
                >
                  Cancel
                </button>
              {:else}
                <button
                  onclick={() => clone(p)}
                  disabled={busyPath === p.path}
                  class="text-[11px] uppercase tracking-wider px-3 py-1.5 rounded bg-slate-800 hover:bg-slate-700 disabled:opacity-50 text-slate-300"
                  aria-label={`Clone ${p.name}`}
                >
                  Clone
                </button>
                <button
                  onclick={() => (confirmingDelete = p.path)}
                  disabled={busyPath === p.path}
                  class="text-[11px] uppercase tracking-wider px-3 py-1.5 rounded bg-slate-800 hover:bg-red-600/80 hover:text-white disabled:opacity-50 text-slate-400"
                  aria-label={`Delete ${p.name}`}
                >
                  Delete
                </button>
              {/if}
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  </main>
</div>
