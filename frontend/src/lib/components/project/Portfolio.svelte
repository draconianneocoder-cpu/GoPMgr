<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Portfolio dashboard: the post-login landing screen. Shows every project
  // in the user's folder with its status, phase, dates, and chart/document
  // counts, sorted so in-progress work appears first. Clicking a card opens
  // the project and routes to its per-project dashboard.
  import { onMount } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import AppHeader from '../AppHeader.svelte';

  let projects = $state<ProjectSummary[]>([]);
  let loading = $state(true);
  let error = $state('');
  let query = $state('');
  let tab = $state<'all' | 'active' | 'done'>('all');

  onMount(load);

  async function load() {
    loading = true;
    error = '';
    try {
      projects = (await window.go.main.App.ProjectsOverview()) ?? [];
    } catch (err: any) {
      error = `Could not load projects: ${err}`;
    } finally {
      loading = false;
    }
  }

  // "In-progress" = not yet finished. Unknown ("") is treated as active so
  // an unreadable project is never hidden by the Active tab.
  const activeStatuses = new Set(['planning', 'active', 'on_hold', '']);
  const isActive = (p: ProjectSummary) => activeStatuses.has(p.status);
  const isDone   = (p: ProjectSummary) => p.status === 'complete' || p.status === 'cancelled';

  const sorted = $derived(
    [...projects].sort((a, b) => {
      const rank = (isActive(a) ? 0 : 1) - (isActive(b) ? 0 : 1);
      if (rank !== 0) return rank;
      return (b.modified || '').localeCompare(a.modified || '');
    }),
  );

  const filtered = $derived(
    sorted.filter(p => {
      if (query && !p.name.toLowerCase().includes(query.toLowerCase())) return false;
      if (tab === 'active') return isActive(p);
      if (tab === 'done')   return isDone(p);
      return true;
    }),
  );

  const counts = $derived({
    all:    projects.length,
    active: projects.filter(isActive).length,
    done:   projects.filter(isDone).length,
  });

  const statusStyles: Record<string, string> = {
    active: 'bg-emerald-600/20 text-emerald-300 border-emerald-700/40',
    planning: 'bg-cyan-600/20 text-cyan-300 border-cyan-700/40',
    on_hold: 'bg-amber-600/20 text-amber-300 border-amber-700/40',
    complete: 'bg-slate-600/20 text-slate-300 border-slate-600/40',
    cancelled: 'bg-red-600/20 text-red-300 border-red-700/40',
  };
  const statusLabel = (s: string) => (s ? s.replace('_', ' ') : 'unknown');

  async function open(p: ProjectSummary) {
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

  // ----- Portfolio analytics (DuckDB, optional build) -----
  let rollup = $state<PortfolioSummary | null>(null);
  let rollupErr = $state('');
  let rollupLoading = $state(false);

  async function runRollup() {
    rollupLoading = true;
    rollupErr = '';
    try {
      rollup = await window.go.main.App.RunPortfolioAnalytics();
    } catch (err: any) {
      rollup = null;
      rollupErr = String(err?.message ?? err);
    } finally {
      rollupLoading = false;
    }
  }

  const fmtNum = (n: number) => n.toLocaleString(undefined, { maximumFractionDigits: 0 });

  // ----- Local data-file import (DuckDB) -----
  let dataset = $state<Dataset | null>(null);
  let importErr = $state('');
  let importing = $state(false);

  async function importDataset() {
    importing = true;
    importErr = '';
    try {
      const ds = await window.go.main.App.ImportDatasetForAnalysis();
      // An empty result means the user cancelled the file picker.
      dataset = ds && ds.columns && ds.columns.length ? ds : null;
    } catch (err: any) {
      dataset = null;
      importErr = String(err?.message ?? err);
    } finally {
      importing = false;
    }
  }
</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <AppHeader active="portfolio" />

  <main class="max-w-5xl mx-auto p-8">
    <div class="flex items-center justify-between mb-5">
      <div>
        <h2 class="text-xl font-bold">Portfolio dashboard</h2>
        <p class="text-xs text-slate-500 mt-1">
          {counts.active} active · {counts.all} total
        </p>
      </div>
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

    <section class="mb-5 p-4 bg-slate-900 border border-slate-800 rounded-lg">
      <div class="flex items-center justify-between gap-3">
        <div>
          <h3 class="text-sm font-bold text-slate-200">Portfolio analytics</h3>
          <p class="text-xs text-slate-500">Cross-project cost rollup, aggregated with DuckDB.</p>
        </div>
        <div class="flex items-center gap-2">
          <button
            onclick={runRollup}
            disabled={rollupLoading}
            aria-busy={rollupLoading}
            class="bg-slate-800 hover:bg-slate-700 disabled:opacity-50 text-slate-200 text-xs font-bold uppercase tracking-wider px-3 py-2 rounded"
          >
            {rollupLoading ? 'Computing…' : 'Run rollup'}
          </button>
          <button
            onclick={importDataset}
            disabled={importing}
            aria-busy={importing}
            class="bg-slate-800 hover:bg-slate-700 disabled:opacity-50 text-slate-200 text-xs font-bold uppercase tracking-wider px-3 py-2 rounded"
          >
            {importing ? 'Importing…' : 'Import data file'}
          </button>
        </div>
      </div>

      {#if rollupErr}
        <p class="mt-3 text-xs text-amber-400" role="status">
          {rollupErr.includes('not built in')
            ? 'The analytics engine is not included in this build.'
            : `Analytics failed: ${rollupErr}`}
        </p>
      {:else if rollup}
        <dl class="mt-3 grid grid-cols-2 sm:grid-cols-4 gap-3">
          <div>
            <dt class="text-[10px] uppercase tracking-wider text-slate-500">Projects</dt>
            <dd class="text-slate-100 font-bold">{rollup.project_count}</dd>
          </div>
          <div>
            <dt class="text-[10px] uppercase tracking-wider text-slate-500">Total budget</dt>
            <dd class="text-slate-100 font-bold">{fmtNum(rollup.total_budgeted_cost)}</dd>
          </div>
          <div>
            <dt class="text-[10px] uppercase tracking-wider text-slate-500">Committed</dt>
            <dd class="text-slate-100 font-bold">{fmtNum(rollup.total_actual_cost)}</dd>
          </div>
          <div>
            <dt class="text-[10px] uppercase tracking-wider text-slate-500">Remaining</dt>
            <dd class="font-bold {rollup.total_budgeted_cost - rollup.total_actual_cost < 0 ? 'text-red-400' : 'text-emerald-300'}">
              {fmtNum(rollup.total_budgeted_cost - rollup.total_actual_cost)}
            </dd>
          </div>
        </dl>
      {/if}

      {#if importErr}
        <p class="mt-3 text-xs text-amber-400" role="status">
          {importErr.includes('not built in')
            ? 'The analytics engine is not included in this build.'
            : `Import failed: ${importErr}`}
        </p>
      {:else if dataset}
        <div class="mt-3">
          <p class="text-xs text-slate-500 mb-2">
            {dataset.columns.length} columns × {dataset.rows.length} rows{dataset.rows.length > 50 ? ' (first 50 shown)' : ''}
          </p>
          <div class="overflow-auto max-h-72 border border-slate-800 rounded">
            <table class="w-full text-xs text-slate-300" aria-label="Imported data preview">
              <thead class="sticky top-0 bg-slate-800/70">
                <tr>
                  {#each dataset.columns as c (c)}
                    <th scope="col" class="text-left font-semibold px-2 py-1 whitespace-nowrap">{c}</th>
                  {/each}
                </tr>
              </thead>
              <tbody>
                {#each dataset.rows.slice(0, 50) as row, i (i)}
                  <tr class="border-t border-slate-800/60">
                    {#each row as cell, j (j)}
                      <td class="px-2 py-1 whitespace-nowrap">{cell === null || cell === undefined ? '' : String(cell)}</td>
                    {/each}
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </div>
      {/if}
    </section>

    {#if !loading && projects.length > 0}
      <div class="flex items-center gap-3 mb-4">
        <input
          bind:value={query}
          type="search"
          placeholder="Search projects…"
          aria-label="Search projects"
          class="flex-1 bg-slate-900 border border-slate-800 text-sm px-3 py-1.5 rounded focus:border-cyan-500 outline-none"
        />
        <div class="flex gap-1" role="tablist" aria-label="Filter by status">
          {#each ([['all','All'],['active','Active'],['done','Done']] as const) as [t, label] (t)}
            <button
              role="tab"
              aria-selected={tab === t}
              onclick={() => (tab = t)}
              class="text-xs px-3 py-1.5 rounded {tab === t
                ? 'bg-cyan-600 text-white'
                : 'bg-slate-800 text-slate-400 hover:text-slate-200'}"
            >
              {label} <span class="opacity-60">{counts[t]}</span>
            </button>
          {/each}
        </div>
      </div>
    {/if}

    {#if loading}
      <p class="text-sm text-slate-500 text-center py-12" role="status" aria-live="polite">Loading…</p>
    {:else if projects.length === 0}
      <p class="text-sm text-slate-500 text-center py-12">
        No projects yet. Click <strong>+ New Project</strong> to get started.
      </p>
    {:else if filtered.length === 0}
      <p class="text-sm text-slate-500 text-center py-12">No projects match your search.</p>
    {:else}
      <ul class="grid grid-cols-1 md:grid-cols-2 gap-3">
        {#each filtered as p (p.path)}
          <li>
            <button
              onclick={() => open(p)}
              class="w-full h-full text-left p-4 bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded-lg"
            >
              <div class="flex items-start justify-between gap-3">
                <div class="font-bold text-slate-50 truncate">{p.name}</div>
                <span
                  class={`shrink-0 text-[10px] font-bold uppercase tracking-wider px-2 py-0.5 rounded border ${
                    statusStyles[p.status] ?? 'bg-slate-700/30 text-slate-300 border-slate-600/40'
                  }`}
                >
                  {statusLabel(p.status)}
                </span>
              </div>
              <div class="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-slate-500">
                {#if p.phase}<span>Phase: {p.phase}</span>{/if}
                {#if p.start_date}<span>Start: {p.start_date}</span>{/if}
                {#if p.end_date}<span>End: {p.end_date}</span>{/if}
                <span>{p.charts} charts · {p.documents} docs</span>
              </div>
              {#if !p.readable}
                <div class="mt-2 text-[11px] text-amber-400">Could not read project details.</div>
              {/if}
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </main>
</div>
