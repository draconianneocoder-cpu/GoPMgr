<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Read-only view of another account's projects, opened through recorded
  // administrator access (ADR-004). It can only look: every call it makes is
  // an AdminView* read of a copy the backend keeps apart from the
  // administrator's own project, and nothing here edits or exports. Leaving
  // it (navigating away) stops access as Stop does, so the target's key and
  // the copy never outlive the view.
  import { onDestroy, onMount } from 'svelte';
  import Spinner from '../Spinner.svelte';

  type Tab = 'summary' | 'schedule' | 'costs' | 'documents';

  let { username, onstop }: { username: string; onstop: () => void } = $props();

  let projects = $state<ProjectFile[]>([]);
  let loadingProjects = $state(true);
  let error = $state('');
  let view = $state<AdminViewProject | null>(null);
  let openPath = $state('');
  let tab = $state<Tab>('summary');
  let schedules = $state<AdminViewSchedule[] | null>(null);
  let costs = $state<AdminViewCosts | null>(null);
  let documents = $state<AdminViewDocument[] | null>(null);
  let tabLoading = $state(false);
  let tabError = $state('');
  let stopping = $state(false);
  // Set once access is stopped, so leaving afterwards does not stop again.
  let stopped = false;

  const tabs: { id: Tab; label: string }[] = [
    { id: 'summary', label: 'Summary' },
    { id: 'schedule', label: 'Schedule' },
    { id: 'costs', label: 'Costs' },
    { id: 'documents', label: 'Documents' },
  ];

  onMount(async () => {
    try {
      projects = (await window.go.main.App.AdminListUserProjects()) ?? [];
    } catch (err: any) {
      error = `Could not list ${username}'s projects: ${err}`;
    } finally {
      loadingProjects = false;
    }
  });

  async function openProject(p: ProjectFile) {
    error = '';
    tabError = '';
    try {
      view = await window.go.main.App.AdminViewUserProject(p.path);
      openPath = p.path;
      tab = 'summary';
      schedules = null;
      costs = null;
      documents = null;
    } catch (err: any) {
      error = `Could not open ${p.name}: ${err}`;
    }
  }

  async function showTab(next: Tab) {
    tab = next;
    tabError = '';
    tabLoading = true;
    try {
      if (next === 'schedule' && schedules === null) {
        schedules = (await window.go.main.App.AdminViewSchedule()) ?? [];
      } else if (next === 'costs' && costs === null) {
        costs = await window.go.main.App.AdminViewCosts();
      } else if (next === 'documents' && documents === null) {
        documents = (await window.go.main.App.AdminViewDocuments()) ?? [];
      }
    } catch (err: any) {
      tabError = `Could not load this view: ${err}`;
    } finally {
      tabLoading = false;
    }
  }

  async function stop() {
    stopping = true;
    try {
      await window.go.main.App.AdminStopUserData();
    } catch (err: any) {
      error = `Could not stop viewing: ${err}`;
      stopping = false;
      return;
    }
    stopped = true;
    stopping = false;
    onstop();
  }

  onDestroy(() => {
    if (!stopped) {
      stopped = true;
      void window.go.main.App.AdminStopUserData().catch(() => {});
    }
  });

  function formatDate(value: string): string {
    if (!value) return '—';
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value : date.toLocaleDateString();
  }
</script>

<section
  class="p-4 bg-slate-900 border border-amber-700/50 rounded-lg space-y-4"
  aria-label={`Viewing ${username}'s data`}
>
  <div class="flex items-start justify-between gap-4">
    <div>
      <h2 class="text-sm font-bold text-amber-300">Viewing {username}'s data</h2>
      <p class="text-xs text-slate-400 mt-1">
        Read only. You can look but not change, export, or print anything. This access is recorded in
        the account history with your reason.
      </p>
    </div>
    <button
      type="button"
      onclick={stop}
      disabled={stopping}
      class="bg-amber-700 hover:bg-amber-600 disabled:opacity-50 text-white text-xs font-bold uppercase tracking-wider px-3 py-1.5 rounded shrink-0"
    >
      {stopping ? 'Stopping…' : 'Stop viewing'}
    </button>
  </div>

  {#if error}
    <p class="text-xs text-red-400" role="alert">{error}</p>
  {/if}

  {#if loadingProjects}
    <Spinner label={`Loading ${username}'s projects…`} class="py-4" />
  {:else if projects.length === 0}
    <p class="text-xs text-slate-500">{username} has no projects.</p>
  {:else}
    <div>
      <h3 class="text-[11px] font-bold uppercase tracking-widest text-slate-500 mb-1">Projects</h3>
      <ul class="flex flex-wrap gap-2">
        {#each projects as p (p.path)}
          <li>
            <button
              type="button"
              onclick={() => openProject(p)}
              aria-pressed={openPath === p.path}
              class="text-xs px-2 py-1 rounded border {openPath === p.path
                ? 'border-amber-500 text-amber-200 bg-amber-900/30'
                : 'border-slate-700 text-slate-300 hover:border-slate-500'}"
            >
              {p.name}
            </button>
          </li>
        {/each}
      </ul>
    </div>
  {/if}

  {#if view}
    <div class="space-y-3">
      <h3 class="text-base font-semibold text-slate-100">{view.project.name}</h3>
      {#if view.audit_warning}
        <p class="text-xs text-red-300 bg-red-950/40 border border-red-800/60 rounded p-2" role="alert">
          {view.audit_warning}
        </p>
      {/if}

      <div role="tablist" aria-label="Project views" class="flex gap-1 border-b border-slate-800">
        {#each tabs as t (t.id)}
          <button
            type="button"
            role="tab"
            aria-selected={tab === t.id}
            onclick={() => showTab(t.id)}
            class="text-xs px-3 py-1.5 -mb-px border-b-2 {tab === t.id
              ? 'border-amber-500 text-amber-200'
              : 'border-transparent text-slate-400 hover:text-slate-200'}"
          >
            {t.label}
          </button>
        {/each}
      </div>

      <div role="tabpanel" aria-label={tabs.find((t) => t.id === tab)?.label} class="text-xs text-slate-300">
        {#if tabError}
          <p class="text-red-400" role="alert">{tabError}</p>
        {:else if tabLoading}
          <Spinner label="Loading…" class="py-4" />
        {:else if tab === 'summary'}
          <dl class="grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1">
            <dt class="text-slate-500">Status</dt><dd>{view.project.status || '—'}</dd>
            <dt class="text-slate-500">Phase</dt><dd>{view.project.phase || '—'}</dd>
            <dt class="text-slate-500">Start</dt><dd>{formatDate(view.project.start_date)}</dd>
            <dt class="text-slate-500">End</dt><dd>{formatDate(view.project.end_date)}</dd>
            <dt class="text-slate-500">Budget</dt>
            <dd>{view.project.budget ? `${view.project.budget} ${view.project.currency_code}` : '—'}</dd>
            <dt class="text-slate-500">Owner</dt><dd>{view.project.owner || '—'}</dd>
            <dt class="text-slate-500">Description</dt><dd>{view.project.description || '—'}</dd>
          </dl>
        {:else if tab === 'schedule' && schedules}
          {#if schedules.length === 0}
            <p class="text-slate-500">This project has no schedule.</p>
          {/if}
          {#each schedules as s (s.chart_id)}
            <div class="space-y-1 mb-4">
              <h4 class="font-semibold text-slate-200">{s.title}</h4>
              {#if s.note}
                <p class="text-amber-300">{s.note}</p>
              {:else if s.tasks.length === 0}
                <p class="text-slate-500">No tasks.</p>
              {:else}
                <table class="w-full">
                  <thead>
                    <tr class="text-slate-500 text-left">
                      <th class="py-1 pr-2 font-normal">Task</th>
                      <th class="py-1 pr-2 font-normal">Start</th>
                      <th class="py-1 pr-2 font-normal">Finish</th>
                      <th class="py-1 font-normal text-right">Done</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each s.tasks as task (task.id)}
                      <tr class="border-t border-slate-800/60">
                        <td class="py-1 pr-2">
                          {task.title}
                          {#if task.milestone}<span class="ml-1 text-[10px] text-cyan-400">milestone</span>{/if}
                          {#if task.critical}<span class="ml-1 text-[10px] text-red-400">critical</span>{/if}
                        </td>
                        <td class="py-1 pr-2 font-mono">{formatDate(task.start_date)}</td>
                        <td class="py-1 pr-2 font-mono">{formatDate(task.finish_date)}</td>
                        <td class="py-1 text-right font-mono">{task.percent_complete}%</td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              {/if}
            </div>
          {/each}
        {:else if tab === 'costs' && costs}
          <h4 class="font-semibold text-slate-200 mb-1">Cost entries</h4>
          {#if costs.entries.length === 0}
            <p class="text-slate-500 mb-3">No cost entries.</p>
          {:else}
            <table class="w-full mb-3">
              <thead>
                <tr class="text-slate-500 text-left">
                  <th class="py-1 pr-2 font-normal">Date</th>
                  <th class="py-1 pr-2 font-normal">Kind</th>
                  <th class="py-1 pr-2 font-normal">Description</th>
                  <th class="py-1 pr-2 font-normal">Supplier</th>
                  <th class="py-1 font-normal text-right">Amount</th>
                </tr>
              </thead>
              <tbody>
                {#each costs.entries as entry (entry.id)}
                  <tr class="border-t border-slate-800/60">
                    <td class="py-1 pr-2 font-mono">{formatDate(entry.cost_date)}</td>
                    <td class="py-1 pr-2">{entry.kind}</td>
                    <td class="py-1 pr-2">{entry.description}</td>
                    <td class="py-1 pr-2">{entry.supplier_name || '—'}</td>
                    <td class="py-1 text-right font-mono">{entry.amount}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          {/if}
          <h4 class="font-semibold text-slate-200 mb-1">Approved cost baselines</h4>
          {#if costs.baselines.length === 0}
            <p class="text-slate-500">No approved baselines.</p>
          {:else}
            <ul class="space-y-1">
              {#each costs.baselines as b (b.version)}
                <li>
                  Version {b.version}: {b.cost_baseline} {b.currency_code}, approved by {b.approved_by || '—'}
                  on {formatDate(b.approved_at)}
                </li>
              {/each}
            </ul>
          {/if}
        {:else if tab === 'documents' && documents}
          {#if documents.length === 0}
            <p class="text-slate-500">No documents.</p>
          {:else}
            <p class="text-slate-500 mb-1">Titles only; document contents are not shown.</p>
            <table class="w-full">
              <thead>
                <tr class="text-slate-500 text-left">
                  <th class="py-1 pr-2 font-normal">Title</th>
                  <th class="py-1 pr-2 font-normal">Kind</th>
                  <th class="py-1 pr-2 font-normal">Version</th>
                  <th class="py-1 pr-2 font-normal">Status</th>
                  <th class="py-1 font-normal">Updated</th>
                </tr>
              </thead>
              <tbody>
                {#each documents as d (d.id)}
                  <tr class="border-t border-slate-800/60">
                    <td class="py-1 pr-2">{d.title}</td>
                    <td class="py-1 pr-2">{d.kind}</td>
                    <td class="py-1 pr-2 font-mono">{d.version}</td>
                    <td class="py-1 pr-2">{d.status || '—'}</td>
                    <td class="py-1 font-mono">{formatDate(d.updated_at)}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          {/if}
        {/if}
      </div>
    </div>
  {/if}
</section>
