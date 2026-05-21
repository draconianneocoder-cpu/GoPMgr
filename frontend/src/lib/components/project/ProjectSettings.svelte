<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // ProjectSettings lets the user edit project-level metadata after
  // creation: name, description, industry, sub-category, methodology,
  // country, budget, owner, dates, status, phase.
  //
  // The Launchpad sets these at creation time; this panel is the
  // canonical "go back and reclassify" entry point. Reuses existing
  // App.UpdateProjectMeta and App.UpdateProjectIndustry — no new
  // backend code.

  import { onMount, onDestroy } from 'svelte';
  import { session, goto } from '../../session.svelte';

  let draft = $state<ProjectMeta | null>(null);
  let original = $state<ProjectMeta | null>(null);
  let busy = $state(false);
  let status = $state('');
  let error = $state('');

  onMount(async () => {
    try {
      const p = await window.go.main.App.GetProjectMeta();
      draft = { ...p };
      original = p;
    } catch (err: any) {
      error = `Could not load project: ${err}`;
    }
  });

  let dirty = $derived(
    draft !== null && original !== null && JSON.stringify(draft) !== JSON.stringify(original),
  );

  async function save() {
    if (!draft) return;
    busy = true;
    error = '';
    status = '';
    try {
      // Two calls because UpdateProjectIndustry covers the four
      // Launchpad columns explicitly; UpdateProjectMeta handles
      // everything else.
      const meta = await window.go.main.App.UpdateProjectMeta(draft);
      const merged = await window.go.main.App.UpdateProjectIndustry(
        draft.industry,
        draft.sub_category,
        draft.methodology,
        draft.country_code,
      );
      original = merged;
      draft = { ...merged };
      session.project = merged;
      status = 'Saved.';
      // Suppress unused-variable warning while keeping the explicit
      // call so the metadata path is always exercised.
      void meta;
    } catch (err: any) {
      error = `Save failed: ${err}`;
    } finally {
      busy = false;
    }
  }

  function revert() {
    if (original) draft = { ...original };
  }

  onDestroy(() => {});
</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <button onclick={() => goto('dashboard')} class="text-xs text-slate-400 hover:text-cyan-400">
        &larr; Dashboard
      </button>
      <h1 class="text-sm font-bold tracking-widest uppercase text-white">Project Settings</h1>
    </div>
    <div class="flex gap-2">
      <button
        onclick={revert}
        disabled={!dirty || busy}
        class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-30 px-3 py-1 rounded"
      >
        Revert
      </button>
      <button
        onclick={save}
        disabled={!dirty || busy}
        class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-3 py-1 rounded"
      >
        {busy ? 'Saving…' : 'Save changes'}
      </button>
    </div>
  </header>

  <main class="p-6 max-w-3xl mx-auto space-y-6">
    {#if error}
      <p class="text-xs text-red-400" role="alert">{error}</p>
    {/if}
    {#if status}
      <p class="text-xs text-cyan-400">{status}</p>
    {/if}

    {#if !draft}
      <p class="text-sm text-slate-500">Loading…</p>
    {:else}
      <!-- Identity -->
      <section class="grid grid-cols-1 md:grid-cols-2 gap-3">
        <label class="block">
          <span class="text-xs text-slate-500 uppercase">Project name</span>
          <input
            bind:value={draft.name}
            class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          />
        </label>
        <label class="block">
          <span class="text-xs text-slate-500 uppercase">Owner</span>
          <input
            bind:value={draft.owner}
            class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          />
        </label>
        <label class="block md:col-span-2">
          <span class="text-xs text-slate-500 uppercase">Description</span>
          <textarea
            bind:value={draft.description}
            rows="3"
            class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          ></textarea>
        </label>
      </section>

      <!-- Classification (Launchpad fields) -->
      <section>
        <h2 class="text-xs font-bold uppercase tracking-widest text-slate-500 mb-2">
          Classification
        </h2>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Industry</span>
            <select
              bind:value={draft.industry}
              class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded"
            >
              <option value="">(none)</option>
              <option value="business">Business</option>
              <option value="administration">Administration</option>
              <option value="engineering">Engineering</option>
              <option value="software">Software</option>
              <option value="construction">Construction</option>
              <option value="custom">Custom</option>
            </select>
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Sub-category</span>
            <input
              bind:value={draft.sub_category}
              class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Methodology</span>
            <input
              bind:value={draft.methodology}
              placeholder="e.g. scrum / cpm / waterfall"
              class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Country (for holidays)</span>
            <select
              bind:value={draft.country_code}
              class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded"
            >
              <option value="US">United States</option>
              <option value="GB">United Kingdom</option>
              <option value="CA">Canada</option>
              <option value="DE">Germany</option>
              <option value="FR">France</option>
              <option value="AU">Australia</option>
              <option value="">Other / generic</option>
            </select>
          </label>
        </div>
      </section>

      <!-- Lifecycle -->
      <section>
        <h2 class="text-xs font-bold uppercase tracking-widest text-slate-500 mb-2">
          Lifecycle
        </h2>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Status</span>
            <select
              bind:value={draft.status}
              class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded"
            >
              <option value="planning">Planning</option>
              <option value="active">Active</option>
              <option value="on_hold">On hold</option>
              <option value="complete">Complete</option>
              <option value="cancelled">Cancelled</option>
            </select>
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Phase</span>
            <select
              bind:value={draft.phase}
              class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded"
            >
              <option value="initiation">Initiation</option>
              <option value="planning">Planning</option>
              <option value="execution">Execution</option>
              <option value="monitoring">Monitoring</option>
              <option value="closing">Closing</option>
            </select>
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Start date</span>
            <input
              type="date"
              bind:value={draft.start_date}
              class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">End date</span>
            <input
              type="date"
              bind:value={draft.end_date}
              class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block md:col-span-2">
            <span class="text-xs text-slate-500 uppercase">Budget</span>
            <input
              type="number"
              step="100"
              bind:value={draft.budget}
              class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
            <span class="block text-[10px] text-slate-500 mt-1">
              Feeds the Dashboard Budget panel via stakeholder rates × work-item points.
            </span>
          </label>
        </div>
      </section>
    {/if}
  </main>
</div>
