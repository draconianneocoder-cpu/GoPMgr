<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import { showToast } from '../../toast.svelte';

  let projects = $state<SigmaProject[]>([]);
  let loading = $state(true);
  let creating = $state(false);
  let newTitle = $state('');
  let newBelt = $state('green');

  onMount(async () => {
    await loadProjects();
  });

  async function loadProjects() {
    loading = true;
    try {
      projects = await window.go.main.App.SigmaListProjects();
    } catch (err: any) {
      showToast(`Failed to load projects: ${err}`, 'error');
    } finally {
      loading = false;
    }
  }

  async function createProject() {
    if (!newTitle.trim()) return;
    creating = true;
    try {
      const p = await window.go.main.App.SigmaCreateProject(newTitle, '', newBelt);
      showToast(`Project "${p.title}" created`, 'success');
      newTitle = '';
      await loadProjects();
      goto('sigma_project', p.id);
    } catch (err: any) {
      showToast(`Create failed: ${err}`, 'error');
    } finally {
      creating = false;
    }
  }
</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <header class="border-b border-slate-800 px-6 py-4 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <button onclick={() => goto('dashboard')} class="text-xs text-slate-400 hover:text-cyan-400">
        &larr; Dashboard
      </button>
      <h1 class="text-lg font-bold tracking-widest uppercase text-white">Process Excellence Suite</h1>
    </div>
  </header>

  <main class="p-6 max-w-5xl mx-auto space-y-8">
    <!-- Create Project Card -->
    <section class="bg-slate-900 border border-slate-800 rounded-lg p-6">
      <h2 class="text-sm font-bold uppercase tracking-widest text-cyan-400 mb-4">Start New DMAIC Project</h2>
      <div class="flex flex-col md:flex-row gap-4">
        <input
          bind:value={newTitle}
          placeholder="Project Title (e.g., Reduce Defect Rate in Assembly Line A)"
          class="flex-1 bg-slate-950 border border-slate-800 p-3 rounded text-sm focus:border-cyan-500 outline-none"
        />
        <select bind:value={newBelt} class="bg-slate-950 border border-slate-800 p-3 rounded text-sm">
          <option value="green">Green Belt</option>
          <option value="black">Black Belt</option>
        </select>
        <button
          onclick={createProject}
          disabled={creating || !newTitle}
          class="bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-6 py-3 rounded text-sm"
        >
          {creating ? 'Creating...' : 'Create Project'}
        </button>
      </div>
    </section>

    <!-- Active Projects List -->
    <section>
      <h2 class="text-sm font-bold uppercase tracking-widest text-slate-500 mb-4">Active Projects</h2>
      {#if loading}
        <p class="text-sm text-slate-500">Loading projects...</p>
      {:else if projects.length === 0}
        <p class="text-sm text-slate-500">No active Six Sigma projects yet. Create one above to get started.</p>
      {:else}
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {#each projects as p (p.id)}
            <button
              onclick={() => goto('sigma_project', p.id)}
              class="text-left bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded-lg p-5 transition"
            >
              <div class="font-bold text-white mb-1">{p.title}</div>
              <div class="text-xs text-slate-400 mb-2">
                {p.belt_level.toUpperCase()} BELL · {p.phase.toUpperCase()} PHASE
              </div>
              <div class="flex items-center gap-2">
                <span class="text-[10px] uppercase px-2 py-0.5 rounded bg-emerald-900/50 text-emerald-300 border border-emerald-800">
                  {p.status}
                </span>
                <span class="text-[10px] text-slate-500">
                  Updated {p.updated_at?.slice(0, 10) ?? '—'}
                </span>
              </div>
            </button>
          {/each}
        </div>
      {/if}
    </section>
  </main>
</div>
