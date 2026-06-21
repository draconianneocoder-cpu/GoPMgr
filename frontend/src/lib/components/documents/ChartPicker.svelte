<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // ChartPicker is a self-contained dropdown that lists the charts
  // in the currently-open project and lets the user pick one. Used
  // by DocumentFieldEditor for every FieldChartRef field.
  //
  // Props
  // -----
  //   value          — bound ID of the selected chart (or "")
  //   chartKind      — optional filter; only charts of this kind are
  //                    shown when set. Empty string means "all kinds".
  //
  // The component loads its chart list once on mount and re-loads if
  // chartKind changes. We do not subscribe to chart-list mutations
  // because the parent editor is doing one focused edit; refresh on
  // next mount.

  import { onMount } from 'svelte';

  let {
    value = $bindable<string>(''),
    chartKind = '',
  }: {
    value?: string;
    chartKind?: string;
  } = $props();

  let charts = $state<ChartRecord[]>([]);
  let loading = $state(true);
  let error = $state('');

  async function load() {
    loading = true;
    error = '';
    try {
      // The backend's ListCharts(projectID, kind) filters server-side
      // when kind is non-empty. The Wails binding takes only `kind`
      // because the currently-open project is on the Go side.
      const all = (await window.go.main.App.ListCharts(chartKind || '')) ?? [];
      charts = all;
    } catch (err: any) {
      error = String(err?.message ?? err);
      charts = [];
    } finally {
      loading = false;
    }
  }

  onMount(load);

  // Re-load if the constraint changes (rare, but supports future
  // dynamic-kind use cases).
  $effect(() => {
    chartKind;
    void load();
  });

  let selectedLabel = $derived(
    value ? charts.find((c) => c.id === value)?.title ?? '(unknown)' : '',
  );
</script>

<div class="space-y-1">
  {#if loading}
    <p class="text-[10px] text-slate-500">Loading charts…</p>
  {:else if error}
    <p class="text-[10px] text-red-400">Error: {error}</p>
  {:else if charts.length === 0}
    <p class="text-[10px] text-slate-500">
      No {chartKind ? chartKind : ''} charts in this project yet. Create one from the dashboard.
    </p>
  {:else}
    <select
      bind:value
      class="w-full bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none text-sm"
    >
      <option value="">(none)</option>
      {#each charts as c (c.id)}
        <option value={c.id}>{c.title} · {c.kind}</option>
      {/each}
    </select>
    {#if value && selectedLabel}
      <p class="text-[10px] text-cyan-400">Linked: {selectedLabel}</p>
    {/if}
  {/if}
</div>
