<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // DORADashboard renders the four DORA KPI cards (Deployment
  // Frequency, Lead Time, Change Failure Rate, MTTR), classification
  // badges, a daily-deploy trend line (powered by StatsChart), and a
  // deployment log with an inline "+ Record deployment" form.

  import { onMount, onDestroy } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import StatsChart from '../charts/StatsChart.svelte';
  import type { StatsLayout } from '../charts/_stats_types';

  let windowDays = $state(30);
  let dora = $state<DORAResult | null>(null);
  let deploys = $state<AgileDeployment[]>([]);
  let loading = $state(true);
  let error = $state('');

  // Inline form for "+ Record deployment".
  let showForm = $state(false);
  let formVersion = $state('');
  let formSuccessful = $state(true);
  let formLead = $state(0);
  let formRestore = $state(0);
  let formNotes = $state('');

  onMount(async () => {
    await refresh();
    loading = false;
  });

  // When windowDays changes, re-compute.
  $effect(() => {
    if (windowDays > 0) {
      void refresh();
    }
  });

  async function refresh() {
    try {
      const [d, list] = await Promise.all([
        window.go.main.App.ComputeDORA(windowDays),
        window.go.main.App.ListDeployments(''),
      ]);
      dora = d;
      deploys = (list ?? []).slice(0, 50); // show most recent 50
    } catch (err: any) {
      error = `Could not load DORA: ${err}`;
    }
  }

  async function recordDeployment() {
    if (!formVersion.trim()) {
      error = 'Version is required.';
      return;
    }
    try {
      await window.go.main.App.SaveDeployment({
        id: '',
        project_id: session.project!.id,
        ts: new Date().toISOString(),
        version: formVersion.trim(),
        successful: formSuccessful,
        lead_time_hours: formLead,
        restore_time_hours: formRestore,
        notes: formNotes,
      });
      formVersion = '';
      formSuccessful = true;
      formLead = 0;
      formRestore = 0;
      formNotes = '';
      showForm = false;
      await refresh();
    } catch (err: any) {
      error = `Save failed: ${err}`;
    }
  }

  async function deleteDeployment(id: string) {
    if (!confirm('Delete this deployment record?')) return;
    try {
      await window.go.main.App.DeleteDeployment(id);
      await refresh();
    } catch (err: any) {
      error = `Delete failed: ${err}`;
    }
  }

  // Map DORA class → palette so the four KPI cards tint consistently.
  function classTone(c: DORAClass): { bar: string; text: string } {
    switch (c) {
      case 'elite':   return { bar: 'bg-emerald-500', text: 'text-emerald-300' };
      case 'high':    return { bar: 'bg-cyan-500',    text: 'text-cyan-300' };
      case 'medium':  return { bar: 'bg-amber-500',   text: 'text-amber-300' };
      case 'low':     return { bar: 'bg-red-500',     text: 'text-red-300' };
      default:        return { bar: 'bg-slate-600',   text: 'text-slate-400' };
    }
  }

  // Build a StatsLayout for the daily-deploy trend so we can reuse
  // the existing StatsChart component.
  let trendLayout = $derived<StatsLayout | null>(buildTrend());
  function buildTrend(): StatsLayout | null {
    if (!dora) return null;
    return {
      kind: 'line',
      title: 'Daily deployments (last ' + dora.window_days + ' days)',
      x_axis: { label: 'Day', type: 'category' },
      y_axis: { label: 'Deploys', type: 'linear' },
      categories: dora.daily_deploy_trend.map((p) => p.date.slice(5)), // MM-DD
      series: [
        {
          name: 'Deploys',
          values: dora.daily_deploy_trend.map((p) => p.count),
          type: 'line',
          color: '#22d3ee',
        },
      ],
    };
  }

  onDestroy(() => {});
</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <button onclick={() => goto('dashboard')} class="text-xs text-slate-400 hover:text-cyan-400">
        &larr; Dashboard
      </button>
      <h1 class="text-sm font-bold tracking-widest uppercase text-white">DORA Metrics</h1>
    </div>
    <div class="flex items-center gap-2">
      <label class="text-xs text-slate-500 flex items-center gap-2">
        Window:
        <select
          bind:value={windowDays}
          class="bg-slate-900 border border-slate-800 px-2 py-1 rounded"
        >
          <option value={7}>7 days</option>
          <option value={30}>30 days</option>
          <option value={90}>90 days</option>
          <option value={180}>180 days</option>
        </select>
      </label>
      <button
        onclick={() => (showForm = !showForm)}
        class="text-xs bg-cyan-600 hover:bg-cyan-500 text-white font-bold uppercase px-3 py-1 rounded"
      >
        {showForm ? 'Cancel' : '+ Deployment'}
      </button>
    </div>
  </header>

  <main class="p-6 space-y-6 max-w-6xl mx-auto">
    {#if error}
      <p class="text-xs text-red-400" role="alert">{error}</p>
    {/if}

    {#if loading || !dora}
      <p class="text-sm text-slate-500">Loading metrics...</p>
    {:else}
      {@const df = classTone(dora.deploy_frequency.class)}
      {@const lt = classTone(dora.lead_time.class)}
      {@const cfr = classTone(dora.change_failure_rate.class)}
      {@const mt = classTone(dora.mttr.class)}

      <!-- Four KPI tiles -->
      <section class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <article class="bg-slate-900 border border-slate-800 rounded-lg overflow-hidden">
          <div class="h-1 {df.bar}"></div>
          <div class="p-4">
            <h3 class="text-[10px] font-bold uppercase tracking-widest text-slate-500">
              Deployment frequency
            </h3>
            <p class="text-2xl font-bold text-white mt-1">{dora.deploy_frequency.label}</p>
            <p class="text-[10px] uppercase tracking-widest mt-1 {df.text}">
              {dora.deploy_frequency.class}
            </p>
            <p class="text-xs text-slate-500 mt-2">{dora.deploy_frequency.caption}</p>
          </div>
        </article>

        <article class="bg-slate-900 border border-slate-800 rounded-lg overflow-hidden">
          <div class="h-1 {lt.bar}"></div>
          <div class="p-4">
            <h3 class="text-[10px] font-bold uppercase tracking-widest text-slate-500">
              Lead time for changes
            </h3>
            <p class="text-2xl font-bold text-white mt-1">{dora.lead_time.label}</p>
            <p class="text-[10px] uppercase tracking-widest mt-1 {lt.text}">
              {dora.lead_time.class}
            </p>
            <p class="text-xs text-slate-500 mt-2">{dora.lead_time.caption}</p>
          </div>
        </article>

        <article class="bg-slate-900 border border-slate-800 rounded-lg overflow-hidden">
          <div class="h-1 {cfr.bar}"></div>
          <div class="p-4">
            <h3 class="text-[10px] font-bold uppercase tracking-widest text-slate-500">
              Change failure rate
            </h3>
            <p class="text-2xl font-bold text-white mt-1">{dora.change_failure_rate.label}</p>
            <p class="text-[10px] uppercase tracking-widest mt-1 {cfr.text}">
              {dora.change_failure_rate.class}
            </p>
            <p class="text-xs text-slate-500 mt-2">{dora.change_failure_rate.caption}</p>
          </div>
        </article>

        <article class="bg-slate-900 border border-slate-800 rounded-lg overflow-hidden">
          <div class="h-1 {mt.bar}"></div>
          <div class="p-4">
            <h3 class="text-[10px] font-bold uppercase tracking-widest text-slate-500">
              MTTR
            </h3>
            <p class="text-2xl font-bold text-white mt-1">{dora.mttr.label}</p>
            <p class="text-[10px] uppercase tracking-widest mt-1 {mt.text}">
              {dora.mttr.class}
            </p>
            <p class="text-xs text-slate-500 mt-2">{dora.mttr.caption}</p>
          </div>
        </article>
      </section>

      <!-- Totals strip -->
      <section class="text-xs text-slate-500 flex gap-6">
        <span><strong class="text-slate-300">{dora.total_deploys}</strong> total deployments</span>
        <span><strong class="text-emerald-300">{dora.successful_deploys}</strong> successful</span>
        <span><strong class="text-red-300">{dora.failed_deploys}</strong> failed</span>
      </section>

      <!-- Trend chart -->
      {#if trendLayout}
        <section class="bg-slate-900 border border-slate-800 rounded-lg p-4">
          <StatsChart layout={trendLayout} height={280} />
        </section>
      {/if}
    {/if}

    <!-- Inline record-deployment form -->
    {#if showForm}
      <section class="bg-slate-900 border border-cyan-700 rounded-lg p-4 space-y-3">
        <h2 class="text-xs font-bold tracking-widest uppercase text-cyan-400">
          Record a deployment
        </h2>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Version / tag</span>
            <input
              bind:value={formVersion}
              placeholder="e.g. v1.4.2"
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="flex items-center gap-2 mt-5">
            <input type="checkbox" bind:checked={formSuccessful} class="accent-cyan-500" />
            <span class="text-sm">Successful deployment</span>
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Lead time (hours, commit → prod)</span>
            <input
              type="number"
              step="0.1"
              bind:value={formLead}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Restore time (hours, failure → recovery)</span>
            <input
              type="number"
              step="0.1"
              bind:value={formRestore}
              disabled={formSuccessful}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none disabled:opacity-40"
            />
          </label>
          <label class="block md:col-span-2">
            <span class="text-xs text-slate-500 uppercase">Notes</span>
            <textarea
              bind:value={formNotes}
              rows="2"
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            ></textarea>
          </label>
        </div>
        <button
          onclick={recordDeployment}
          disabled={!formVersion.trim()}
          class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-3 py-1 rounded"
        >
          Save deployment
        </button>
      </section>
    {/if}

    <!-- Deployment log -->
    <section>
      <h2 class="text-xs font-bold tracking-widest uppercase text-slate-500 mb-3">
        Deployment log ({deploys.length})
      </h2>
      {#if deploys.length === 0}
        <p class="text-sm text-slate-500">
          No deployments recorded yet. Click <strong>+ Deployment</strong> to add one.
        </p>
      {:else}
        <ul class="divide-y divide-slate-800 border border-slate-800 rounded">
          {#each deploys as d (d.id)}
            <li class="px-3 py-2 flex items-center gap-3">
              <span class="text-[10px] font-mono text-slate-500 w-32 shrink-0">
                {d.ts.slice(0, 19).replace('T', ' ')}
              </span>
              <span class="text-xs font-bold text-white truncate flex-1">
                {d.version}
              </span>
              <span class="text-[10px] px-2 py-0.5 rounded {d.successful ? 'bg-emerald-900 text-emerald-200' : 'bg-red-900 text-red-200'}">
                {d.successful ? 'OK' : 'FAILED'}
              </span>
              {#if d.lead_time_hours > 0}
                <span class="text-[10px] text-slate-400 w-20 text-right">{d.lead_time_hours.toFixed(1)}h lead</span>
              {/if}
              {#if !d.successful && d.restore_time_hours > 0}
                <span class="text-[10px] text-amber-400 w-24 text-right">{d.restore_time_hours.toFixed(1)}h restore</span>
              {/if}
              <button
                onclick={() => deleteDeployment(d.id)}
                class="text-xs text-slate-500 hover:text-red-400"
                aria-label="Delete deployment"
              >
                ×
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </section>
  </main>
</div>
