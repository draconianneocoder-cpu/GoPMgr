<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { session, goto } from '../../session.svelte';

  let chart = $state<ScenarioChart | null>(null);
  let title = $state('');
  let data = $state('');
  let config = $state('');
  let loading = $state(true);
  let saving = $state(false);
  let comparing = $state(false);
  let promoting = $state(false);
  let baselineName = $state('');
  let variances = $state<Record<string, ScheduleVariance>>({});
  let comparisonLoaded = $state(false);
  let status = $state('');
  let error = $state('');

  function pretty(raw: string): string {
    try {
      return JSON.stringify(JSON.parse(raw || '{}'), null, 2);
    } catch {
      return raw || '{}';
    }
  }

  function assertJSON(label: string, raw: string) {
    try {
      JSON.parse(raw || '{}');
    } catch (err: any) {
      throw new Error(`${label} is not valid JSON: ${err?.message ?? err}`);
    }
  }

  function varianceRows(): ScheduleVariance[] {
    return Object.values(variances).sort((a, b) => a.task_id.localeCompare(b.task_id));
  }

  function formatVariance(days: number): string {
    if (Math.abs(days) < 1e-9) return '0.0d';
    return `${days > 0 ? '+' : ''}${days.toFixed(1)}d`;
  }

  onMount(async () => {
    loading = true;
    error = '';
    try {
      if (!session.editingId) throw new Error('No scenario chart selected');
      const loaded = await window.go.main.App.GetScenarioChart(session.editingId);
      chart = loaded;
      title = loaded.title;
      data = pretty(loaded.data);
      config = pretty(loaded.config);
    } catch (err: any) {
      error = String(err?.message ?? err);
    } finally {
      loading = false;
    }
  });

  async function save() {
    if (!chart || saving) return;
    saving = true;
    status = '';
    error = '';
    try {
      assertJSON('Scenario chart data', data);
      assertJSON('Scenario chart config', config);
      const saved = await window.go.main.App.SaveScenarioChart({
        ...chart,
        title: title.trim() || chart.title,
        data,
        config,
      });
      chart = saved;
      title = saved.title;
      data = pretty(saved.data);
      config = pretty(saved.config);
      variances = {};
      comparisonLoaded = false;
      status = 'Saved.';
    } catch (err: any) {
      error = String(err?.message ?? err);
    } finally {
      saving = false;
    }
  }

  async function compareToBaseline() {
    if (!chart || comparing) return;
    comparing = true;
    status = '';
    error = '';
    try {
      variances = await window.go.main.App.CompareScenarioChart(chart.id);
      comparisonLoaded = true;
      status = 'Comparison refreshed.';
    } catch (err: any) {
      error = String(err?.message ?? err);
    } finally {
      comparing = false;
    }
  }

  async function promoteToBaseline() {
    if (!chart || promoting) return;
    promoting = true;
    status = '';
    error = '';
    try {
      const name = baselineName.trim();
      if (!name) throw new Error('Baseline name is required');
      const promoted = await window.go.main.App.PromoteScenarioChartToBaseline(chart.id, name);
      baselineName = '';
      status = `Promoted ${promoted.name}.`;
    } catch (err: any) {
      error = String(err?.message ?? err);
    } finally {
      promoting = false;
    }
  }
</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <button onclick={() => goto('project_settings')} class="text-xs text-slate-400 hover:text-cyan-400">
        &larr; Project Settings
      </button>
      <h1 class="text-sm font-bold tracking-widest uppercase text-slate-50">Scenario chart editor</h1>
    </div>
    <div class="flex flex-wrap gap-2">
      <button
        onclick={compareToBaseline}
        disabled={!chart || comparing}
        class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-3 py-2 rounded"
      >
        {comparing ? 'Comparing...' : 'Compare to baseline'}
      </button>
      <button
        onclick={save}
        disabled={!chart || saving}
        class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-4 py-2 rounded"
      >
        {saving ? 'Saving...' : 'Save scenario edits'}
      </button>
    </div>
  </header>

  <main class="p-6 max-w-6xl mx-auto space-y-5">
    {#if loading}
      <div class="border border-slate-800 rounded bg-slate-900/40 p-4 text-xs uppercase tracking-widest text-slate-500">
        Loading scenario chart
      </div>
    {:else if error && !chart}
      <div class="border border-red-900/70 rounded bg-red-950/30 p-4 text-sm text-red-200" role="alert">
        {error}
      </div>
    {:else if chart}
      <section class="border border-slate-800 rounded bg-slate-900/40 p-4 space-y-3">
        <div class="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
          <div>
            <p class="uppercase tracking-widest text-slate-500">Kind</p>
            <p class="text-slate-200">{chart.kind}</p>
          </div>
          <div>
            <p class="uppercase tracking-widest text-slate-500">Source chart</p>
            <p class="text-slate-200 break-all">{chart.source_chart_id}</p>
          </div>
          <div>
            <p class="uppercase tracking-widest text-slate-500">Source baseline</p>
            <p class="text-slate-200 break-all">{chart.source_baseline_id || 'Current chart data'}</p>
          </div>
        </div>

        <label class="block">
          <span class="text-xs text-slate-500 uppercase">Scenario chart title</span>
          <input
            bind:value={title}
            disabled={saving}
            class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none disabled:opacity-50"
          />
        </label>
      </section>

      <section class="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <label class="block">
          <span class="text-xs text-slate-500 uppercase">Scenario chart data JSON</span>
          <textarea
            bind:value={data}
            disabled={saving}
            rows="22"
            spellcheck="false"
            class="w-full mt-1 bg-slate-950 border border-slate-800 p-3 rounded focus:border-cyan-500 outline-none disabled:opacity-50 font-mono text-[11px] leading-relaxed"
          ></textarea>
        </label>
        <label class="block">
          <span class="text-xs text-slate-500 uppercase">Scenario chart config JSON</span>
          <textarea
            bind:value={config}
            disabled={saving}
            rows="22"
            spellcheck="false"
            class="w-full mt-1 bg-slate-950 border border-slate-800 p-3 rounded focus:border-cyan-500 outline-none disabled:opacity-50 font-mono text-[11px] leading-relaxed"
          ></textarea>
        </label>
      </section>

      <section class="border border-slate-800 rounded bg-slate-900/40 p-4 space-y-3">
        <div class="grid grid-cols-1 md:grid-cols-[1fr_auto_auto] gap-2">
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Promoted baseline name</span>
            <input
              bind:value={baselineName}
              disabled={promoting}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none disabled:opacity-50"
            />
          </label>
          <div class="flex items-end">
            <button
              onclick={promoteToBaseline}
              disabled={!chart || promoting || baselineName.trim() === ''}
              class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-4 py-2 rounded"
            >
              {promoting ? 'Promoting...' : 'Promote to baseline'}
            </button>
          </div>
          <div class="flex items-end">
            <button
              onclick={compareToBaseline}
              disabled={!chart || comparing}
              class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-4 py-2 rounded"
            >
              {comparing ? 'Comparing...' : 'Compare to baseline'}
            </button>
          </div>
        </div>

        {#if comparisonLoaded}
          {#if varianceRows().length > 0}
            <div class="grid grid-cols-[1fr_auto_auto] gap-x-3 gap-y-1 text-[11px] text-slate-400">
              <span class="uppercase tracking-widest text-slate-500">Task</span>
              <span class="uppercase tracking-widest text-slate-500">Start</span>
              <span class="uppercase tracking-widest text-slate-500">Finish</span>
              {#each varianceRows() as variance (variance.task_id)}
                <span class="text-slate-300">{variance.task_id}</span>
                <span class={variance.start_var_days > 0 ? 'text-amber-300' : variance.start_var_days < 0 ? 'text-cyan-300' : 'text-slate-400'}>
                  {formatVariance(variance.start_var_days)}
                </span>
                <span class={variance.finish_var_days > 0 ? 'text-amber-300' : variance.finish_var_days < 0 ? 'text-cyan-300' : 'text-slate-400'}>
                  {formatVariance(variance.finish_var_days)}
                </span>
              {/each}
            </div>
          {:else}
            <p class="text-xs uppercase tracking-widest text-slate-500">No baseline variance</p>
          {/if}
        {/if}
      </section>

      <div class="flex flex-wrap items-center gap-3">
        <button
          onclick={save}
          disabled={saving}
          class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-4 py-2 rounded"
        >
          {saving ? 'Saving...' : 'Save scenario edits'}
        </button>
        {#if status}
          <span class="text-xs text-cyan-400">{status}</span>
        {/if}
        {#if error}
          <span class="text-xs text-red-400" role="alert">{error}</span>
        {/if}
      </div>
    {/if}
  </main>
</div>
