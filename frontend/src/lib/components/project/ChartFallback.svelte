<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Landing screen for session.view === 'charts' -- reached only when
  // Dashboard.svelte's newChart()/importMSPDI() save a chart whose kind has
  // no entry in chartRoutes (`goto(chartRoutes[kind] ?? 'charts', c.id)`).
  // This is deliberate, tested drift-handling for the case where the
  // backend's chart registry (internal/charts/registry.go) gains a new kind
  // before the frontend has a dedicated editor view and chartRoutes entry
  // for it -- see docs/design/dashboard-ia-restructuring-proposal.md §3.2.
  // Both call sites only navigate here AFTER their SaveChart call has
  // already succeeded, so the chart genuinely exists; this screen's job is
  // to say so, not to apologize for a failure that didn't happen.
  import { onMount } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import Button from '../Button.svelte';
  import Spinner from '../Spinner.svelte';

  let chart = $state<ChartRecord | null>(null);
  let loadFailed = $state(false);
  let loading = $state(true);

  onMount(async () => {
    // Read once: this view is only ever reached via a fresh goto('charts', id)
    // from Dashboard.svelte, so the component always mounts new rather than
    // being reused in place across a stale editingId.
    const id = session.editingId;
    if (!id) {
      loading = false;
      return;
    }
    try {
      chart = await window.go.main.App.GetChart(id);
    } catch {
      // The chart still exists -- only this lookup failed. Fall back to
      // generic copy below rather than claiming something went wrong with
      // the save itself.
      loadFailed = true;
    } finally {
      loading = false;
    }
  });
</script>

<div class="min-h-screen bg-slate-950 text-slate-200 flex items-center justify-center p-8">
  <div class="max-w-md text-center space-y-4">
    {#if loading}
      <Spinner label="Loading…" />
    {:else}
      <p class="text-sm text-slate-200">
        {#if chart}
          <span class="font-semibold text-slate-50">"{chart.title}"</span> was saved, but this chart type
        {:else}
          Your chart was saved, but its type
        {/if}
        doesn't have a dedicated view in this version of GoPMgr yet.
      </p>
      <p class="text-xs text-slate-500">
        It's safe in your project — nothing was lost. Check back after the next update, or return to
        the dashboard for now.
      </p>
      {#if loadFailed}
        <p class="text-xs text-amber-400">(Could not look up the chart's title, but the save itself succeeded.)</p>
      {/if}
    {/if}
    <Button variant="primary" onclick={() => goto('dashboard')}>Back to dashboard</Button>
  </div>
</div>
