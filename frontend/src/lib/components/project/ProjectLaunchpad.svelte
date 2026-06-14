<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // ProjectLaunchpad replaces the old "name + description" inline
  // form in ProjectPicker. Four-step wizard:
  //
  //   1. Industry tile selection      (Business / Admin / Engineering / Software / Construction / Custom)
  //   2. Sub-category (industry-aware list)
  //   3. Methodology (recommended set for the industry; user can override)
  //   4. Name + description + country + seed-artifact checkboxes
  //
  // On submit calls CreateProjectFromLaunchpad. The seed list shown
  // in step 4 comes from the backend's zen-go evaluation, so adding
  // a new industry/methodology row to the JDM auto-extends the GUI
  // suggestions.

  import { onDestroy } from 'svelte';
  import { session, goto } from '../../session.svelte';

  // Props — Launchpad can be opened from ProjectPicker; on close we
  // notify the parent so it can refresh its list.
  let {
    onCreated,
    onCancel,
  }: {
    onCreated: (project: ProjectMeta, projectPath?: string) => void;
    onCancel: () => void;
  } = $props();

  type Step = 1 | 2 | 3 | 4;
  let step = $state<Step>(1);

  // Selections
  let industry = $state('');
  let subCategory = $state('');
  let methodology = $state('');
  let name = $state('');
  let description = $state('');
  let countryCode = $state('US');

  // Seed picker state
  let suggestedSeeds = $state<string[]>([]);
  let seedsChecked = $state<Record<string, boolean>>({});

  let busy = $state(false);
  let error = $state('');

  const INDUSTRIES = [
    { id: 'business',       label: 'Business',       blurb: 'Marketing, sales, finance, HR, operations.' },
    { id: 'administration', label: 'Administration', blurb: 'Legal, public sector, executive support, facilities.' },
    { id: 'engineering',    label: 'Engineering',    blurb: 'R&D, civil, mechanical, electrical, manufacturing.' },
    { id: 'software',       label: 'Software',       blurb: 'Web, mobile, AI/ML, DevOps, game dev.' },
    { id: 'construction',   label: 'Construction',   blurb: 'Residential, commercial, infrastructure, renovation.' },
    { id: 'custom',         label: 'Custom',         blurb: 'Blank slate — pick everything yourself.' },
  ];

  const SUB_CATEGORIES: Record<string, string[]> = {
    business:       ['Marketing', 'Sales', 'Finance', 'HR', 'Operations'],
    administration: ['Legal', 'Public Sector', 'Executive Support', 'Facility Management'],
    engineering:    ['R&D', 'Civil', 'Mechanical', 'Electrical', 'Manufacturing'],
    software:       ['Web Dev', 'Mobile App', 'AI/ML', 'DevOps', 'Game Dev'],
    construction:   ['Residential', 'Commercial', 'Infrastructure', 'Renovation'],
    custom:         ['General'],
  };

  // Methodology recommendations per industry (lowercase IDs match
  // the JDM's `methodology` column).
  const METHODOLOGIES: Record<string, { id: string; label: string; blurb: string }[]> = {
    business: [
      { id: 'lean',      label: 'Lean',      blurb: 'Eliminate waste; flow-based.' },
      { id: 'six_sigma', label: 'Six Sigma', blurb: 'Process improvement via DMAIC.' },
      { id: 'okrs',      label: 'OKRs',      blurb: 'Objectives & key results.' },
    ],
    administration: [
      { id: 'waterfall', label: 'Waterfall', blurb: 'Linear, sequential phases.' },
      { id: 'prince2',   label: 'PRINCE2',   blurb: 'Stage-gated governance.' },
      { id: 'pmbok',     label: 'PMBOK',     blurb: 'PMI process groups.' },
    ],
    engineering: [
      { id: 'cpm',       label: 'Critical Path', blurb: 'Network-based scheduling.' },
      { id: 'waterfall', label: 'Waterfall',     blurb: 'Sequential design / build / test.' },
      { id: 'six_sigma', label: 'Six Sigma',     blurb: 'Quality control loops.' },
    ],
    software: [
      { id: 'scrum',    label: 'Scrum',    blurb: 'Time-boxed sprints, backlog.' },
      { id: 'kanban',   label: 'Kanban',   blurb: 'Continuous flow, WIP limits.' },
      { id: 'scrumban', label: 'Scrumban', blurb: 'Hybrid: backlog + flow.' },
    ],
    construction: [
      { id: 'waterfall', label: 'Waterfall',         blurb: 'Phase-gated build.' },
      { id: 'lean',      label: 'Lean Construction', blurb: 'Pull planning; minimise waste.' },
      { id: 'cpm',       label: 'CPM',               blurb: 'Critical-path scheduling.' },
    ],
    custom: [
      { id: 'custom', label: 'Build it yourself', blurb: 'No starter artifacts.' },
    ],
  };

  // When the user finishes step 3, ask the backend (zen-go) for the
  // recommended seed list and check them all by default.
  async function loadSeeds() {
    busy = true;
    error = '';
    try {
      const seeds = (await window.go.main.App.LaunchpadEvaluate(industry, methodology)) ?? [];
      suggestedSeeds = seeds;
      const next: Record<string, boolean> = {};
      for (const s of seeds) next[s] = true;
      seedsChecked = next;
    } catch (err: any) {
      error = `Could not load seed suggestions: ${err}`;
      suggestedSeeds = [];
    } finally {
      busy = false;
    }
  }

  async function create() {
    busy = true;
    error = '';
    try {
      const seeds = suggestedSeeds.filter((s) => seedsChecked[s]);
      const [project, , projectPath] = await window.go.main.App.CreateProjectFromLaunchpad(
        name,
        description,
        industry,
        subCategory,
        methodology,
        countryCode,
        seeds,
      );
      onCreated(project, projectPath);
    } catch (err: any) {
      error = `Create failed: ${err}`;
    } finally {
      busy = false;
    }
  }

  function next() {
    if (step === 3) {
      void loadSeeds();
    }
    if (step < 4) step = (step + 1) as Step;
  }
  function prev() {
    if (step > 1) step = (step - 1) as Step;
  }

  // Pretty labels for seed strings the backend returns.
  const SEED_LABELS: Record<string, string> = {
    kanban:                   'Kanban board (default 4 columns)',
    backlog:                  '3 placeholder backlog items',
    sprint1:                  'Sprint 1 in planning state',
    wbs:                      'Work Breakdown Structure (empty root)',
    cpm:                      'CPM schedule (empty)',
    fishbone:                 'Fishbone (root-cause) diagram',
    control:                  'Control chart',
    pareto:                   'Pareto chart',
    cumulative_flow:          'Cumulative Flow diagram',
    swot:                     'SWOT matrix',
    charter:                  'Project Charter document',
    plan_word:                'Project Plan document',
    statement_of_work:        'Statement of Work',
    scope_statement:          'Scope Statement',
    risk_register:            'Risk Register',
    communication_plan:       'Communication Plan',
    status_report:            'Initial Status Report',
    stakeholder_analysis_doc: 'Stakeholder Analysis',
  };
  function seedLabel(s: string): string {
    return SEED_LABELS[s] ?? s;
  }

  onDestroy(() => {});
</script>

<div class="min-h-screen bg-slate-950 text-slate-200 flex flex-col">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <h1 class="text-sm font-bold tracking-widest uppercase text-white">
      New Project · Step {step} of 4
    </h1>
    <button onclick={onCancel} class="text-xs text-slate-400 hover:text-cyan-400">
      Cancel
    </button>
  </header>

  <!-- Progress strip -->
  <div class="flex">
    {#each [1, 2, 3, 4] as i}
      <div
        class="flex-1 h-1 {i <= step ? 'bg-cyan-500' : 'bg-slate-800'}"
      ></div>
    {/each}
  </div>

  <main class="flex-1 p-8 max-w-5xl mx-auto w-full">
    {#if error}
      <p class="text-xs text-red-400 mb-3" role="alert">{error}</p>
    {/if}

    {#if step === 1}
      <h2 class="text-lg font-bold mb-6">What kind of project is this?</h2>
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {#each INDUSTRIES as ind (ind.id)}
          <button
            onclick={() => { industry = ind.id; subCategory = ''; methodology = ''; next(); }}
            class="p-5 bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded-lg text-left"
          >
            <div class="text-base font-bold text-white">{ind.label}</div>
            <p class="text-xs text-slate-500 mt-1">{ind.blurb}</p>
          </button>
        {/each}
      </div>
    {:else if step === 2}
      <h2 class="text-lg font-bold mb-6">
        Narrow it down (<span class="text-cyan-400">{industry}</span>)
      </h2>
      <div class="grid grid-cols-2 md:grid-cols-3 gap-3">
        {#each SUB_CATEGORIES[industry] ?? [] as sub (sub)}
          <button
            onclick={() => { subCategory = sub; next(); }}
            class="p-4 bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded text-left text-sm"
          >
            {sub}
          </button>
        {/each}
      </div>
      <div class="mt-6 flex">
        <button onclick={prev} class="text-xs text-slate-400 hover:text-cyan-400">← Back</button>
      </div>
    {:else if step === 3}
      <h2 class="text-lg font-bold mb-6">
        Which methodology fits best?
      </h2>
      <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
        {#each METHODOLOGIES[industry] ?? [] as m (m.id)}
          <button
            onclick={() => { methodology = m.id; next(); }}
            class="p-5 bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded-lg text-left"
          >
            <div class="text-base font-bold text-white">{m.label}</div>
            <p class="text-xs text-slate-500 mt-1">{m.blurb}</p>
          </button>
        {/each}
      </div>
      <div class="mt-6 flex">
        <button onclick={prev} class="text-xs text-slate-400 hover:text-cyan-400">← Back</button>
      </div>
    {:else}
      <h2 class="text-lg font-bold mb-6">Project details &amp; starter artifacts</h2>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
        <label class="block">
          <span class="text-xs text-slate-500 uppercase">Project name</span>
          <input
            bind:value={name}
            class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            required
          />
        </label>
        <label class="block">
          <span class="text-xs text-slate-500 uppercase">Country (for holidays)</span>
          <select
            bind:value={countryCode}
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
      <label class="block mb-6">
        <span class="text-xs text-slate-500 uppercase">Description</span>
        <textarea
          bind:value={description}
          rows="2"
          class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        ></textarea>
      </label>

      <div class="mb-6">
        <h3 class="text-xs font-bold uppercase tracking-widest text-slate-500 mb-2">
          Starter artifacts (suggested for {industry} + {methodology})
        </h3>
        {#if busy && suggestedSeeds.length === 0}
          <p class="text-xs text-slate-500">Loading recommendations…</p>
        {:else if suggestedSeeds.length === 0}
          <p class="text-xs text-slate-500">
            No suggestions for this combination — you'll start with an empty project.
          </p>
        {:else}
          <ul class="space-y-1">
            {#each suggestedSeeds as s (s)}
              <li>
                <label class="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    bind:checked={seedsChecked[s]}
                    class="accent-cyan-500"
                  />
                  <span>{seedLabel(s)}</span>
                </label>
              </li>
            {/each}
          </ul>
        {/if}
      </div>

      <div class="flex items-center justify-between">
        <button onclick={prev} class="text-xs text-slate-400 hover:text-cyan-400">
          ← Back
        </button>
        <button
          onclick={create}
          disabled={busy || !name}
          class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-4 py-2 rounded"
        >
          {busy ? 'Creating…' : 'Create project'}
        </button>
      </div>
    {/if}
  </main>
</div>
