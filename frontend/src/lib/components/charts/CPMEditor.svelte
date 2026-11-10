<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { definedEntries, normaliseSimulation } from '../../desktop-dtos';
  import { desktopApi } from '../../desktop-api';
  // CPMEditor records each activity's Duration. The backend runs the
  // full CPM forward/backward pass (kernel.CalculateCPM) and writes
  // ES/EF/LS/LF/Float plus IsCritical back per node. Critical-path
  // activities are tinted red on the canvas.
  import { onMount, onDestroy } from 'svelte';
  import { session } from '../../session.svelte';
  import LayeredEditorShell from './_layered_editor_shell.svelte';
  import Button from '../Button.svelte';
  import BaselineList from './BaselineList.svelte';
  import { levelResourcesMessages, splitPreviewMessage } from './leveling_messages';
  import { formatNumber, t } from '../../i18n/i18n.svelte';

  function fmt(n: unknown, digits = 1): string {
    return typeof n === 'number' ? n.toFixed(digits) : '—';
  }

  // ----- Schedule baseline (roadmap item 17) -----
  let baselineCount = $state(0);
  let variances = $state<Record<string, ScheduleVariance>>({});
  let baselineBusy = $state(false);
  let baselineMsg = $state('');
  // DEVELOPER_HANDBOOK.md §10.5: every timer must be cleared on destroy.
  let baselineMsgTimer: ReturnType<typeof setTimeout> | null = null;
  let resourceMsgTimer: ReturnType<typeof setTimeout> | null = null;
  onDestroy(() => {
    if (baselineMsgTimer) clearTimeout(baselineMsgTimer);
    if (resourceMsgTimer) clearTimeout(resourceMsgTimer);
  });

  async function refreshBaseline() {
    if (!session.editingId) return;
    try {
      const list = await desktopApi.ListScheduleBaselines(session.editingId);
      baselineCount = list?.length ?? 0;
      variances = baselineCount > 0
        ? definedEntries(await desktopApi.CompareScheduleBaseline(session.editingId, ''))
        : {};
    } catch {
      // Baseline data is auxiliary; never block the editor on it.
      baselineCount = 0;
      variances = {};
    }
  }

  async function setBaseline() {
    if (!session.editingId || baselineBusy) return;
    baselineBusy = true;
    baselineMsg = '';
    try {
      await desktopApi.SetScheduleBaseline(session.editingId, '');
      await refreshBaseline();
      baselineMsg = t('chart.baselineSet');
      if (baselineMsgTimer) clearTimeout(baselineMsgTimer);
      baselineMsgTimer = setTimeout(() => (baselineMsg = ''), 2500);
    } catch (err: any) {
      baselineMsg = String(err?.message ?? err);
    } finally {
      baselineBusy = false;
    }
  }

  function varFmt(days: number): string {
    if (Math.abs(days) < 1e-9) return t('cpm.variance.onPlan');
    return days > 0 ? t('cpm.variance.late', { days: oneDecimal(days) }) : t('cpm.variance.early', { days: oneDecimal(days) });
  }

  // ----- Earned Value (roadmap item 18) -----
  let evm = $state<EVMetrics | null>(null);
  let evmAsOf = $state(''); // YYYY-MM-DD; '' = today
  let evmError = $state('');
  let evmBusy = $state(false);

  async function computeEVM() {
    if (!session.editingId || evmBusy) return;
    evmBusy = true;
    evmError = '';
    try {
      const loaded = await desktopApi.ComputeScheduleEVM(session.editingId, evmAsOf);
      evm = { ...loaded, tasks: loaded.tasks ?? [] };
    } catch (err: any) {
      evm = null;
      evmError = String(err?.message ?? err);
    } finally {
      evmBusy = false;
    }
  }

  function money(n: number): string {
    return n.toLocaleString(undefined, { maximumFractionDigits: 0 });
  }

  function idx(n: number): string {
    return n > 0 ? n.toFixed(2) : 'n/a';
  }

  // ----- Monte Carlo schedule risk -----
  let monteCarlo = $state<SimResult | null>(null);
  let monteCarloIterations = $state(1000);
  let monteCarloBusy = $state(false);
  let monteCarloError = $state('');
  let monteCarloReportBusy = $state(false);
  let monteCarloReportStatus = $state('');
  let monteCarloReportError = $state('');

  // oneDecimal formats a day count with one decimal place in the format locale.
  function oneDecimal(n: number): string {
    return formatNumber(n, { minimumFractionDigits: 1, maximumFractionDigits: 1 });
  }

  function days(n: number): string {
    return t('cpm.days', { days: oneDecimal(n) });
  }

  function pct(n: number): string {
    return formatNumber(n, { style: 'percent', maximumFractionDigits: 0 });
  }

  function cdfDomain(result: SimResult): [number, number] {
    const days = [
      ...(result.finish_cdf ?? []).map((point) => point.day),
      result.p50,
      result.p80,
      result.p90,
    ].filter((value) => Number.isFinite(value));
    if (days.length === 0) return [0, 1];
    let min = Math.min(...days);
    let max = Math.max(...days);
    if (Math.abs(max - min) < 1e-9) {
      min -= 0.5;
      max += 0.5;
    }
    return [min, max];
  }

  function cdfX(result: SimResult, day: number): number {
    const [min, max] = cdfDomain(result);
    return 14 + ((day - min) / (max - min)) * 214;
  }

  function cdfY(probability: number): number {
    return 82 - Math.max(0, Math.min(1, probability)) * 64;
  }

  function cdfPath(result: SimResult): string {
    const points = result.finish_cdf ?? [];
    if (points.length === 0) return '';
    return points
      .map((point, i) => `${i === 0 ? 'M' : 'L'} ${cdfX(result, point.day).toFixed(1)} ${cdfY(point.probability).toFixed(1)}`)
      .join(' ');
  }

  function confidenceMarkers(result: SimResult) {
    return [
      { label: 'cpm.risk.p50' as const, day: result.p50, probability: 0.5, color: '#67e8f9' },
      { label: 'cpm.risk.p80' as const, day: result.p80, probability: 0.8, color: '#fbbf24' },
      { label: 'cpm.risk.p90' as const, day: result.p90, probability: 0.9, color: '#fca5a5' },
    ];
  }

  function tornadoRows(): TornadoDriver[] {
    return monteCarlo?.tornado_drivers ?? [];
  }

  function tornadoMaxScore(): number {
    return tornadoRows().reduce((max, driver) => Math.max(max, driver.score), 0);
  }

  function tornadoBarWidth(driver: TornadoDriver): string {
    const maxScore = tornadoMaxScore();
    const value = maxScore > 0 ? (driver.score / maxScore) * 100 : driver.critical_frequency * 100;
    return `${Math.max(2, Math.min(100, value))}%`;
  }

  async function runMonteCarlo() {
    if (!session.editingId || monteCarloBusy) return;
    monteCarloBusy = true;
    monteCarloError = '';
    monteCarloReportStatus = '';
    monteCarloReportError = '';
    monteCarlo = null;
    try {
      await shellRef?.save();
      const iterations = Math.max(100, Math.min(10000, Math.floor(Number(monteCarloIterations) || 1000)));
      monteCarloIterations = iterations;
      monteCarlo = normaliseSimulation(await desktopApi.RunChartMonteCarlo(session.editingId, iterations, 0));
    } catch (err: any) {
      monteCarloError = String(err?.message ?? err);
    } finally {
      monteCarloBusy = false;
    }
  }

  async function exportMonteCarloReport() {
    if (!session.editingId || monteCarloReportBusy) return;
    monteCarloReportBusy = true;
    monteCarloReportStatus = '';
    monteCarloReportError = '';
    try {
      await shellRef?.save();
      const iterations = Math.max(100, Math.min(10000, Math.floor(Number(monteCarloIterations) || 1000)));
      monteCarloIterations = iterations;
      const path = await desktopApi.ExportChartMonteCarloRiskReport(session.editingId, iterations, 0);
      monteCarloReportStatus = t('cpm.exportedTo', { path });
    } catch (err: any) {
      if (!String(err?.message ?? err).includes('export cancelled')) monteCarloReportError = t('cpm.exportFailed', { error: String(err?.message ?? err) });
    } finally {
      monteCarloReportBusy = false;
    }
  }

  function syncMinorUnits(node: any, valueKey: 'budgeted_cost' | 'actual_cost', raw: string) {
    const minorKey = valueKey === 'budgeted_cost'
      ? 'budgeted_cost_minor_units'
      : 'actual_cost_minor_units';
    const value = Number(raw || 0);
    node[minorKey] = Number.isFinite(value) ? Math.round(value * 100) : 0;
  }

  // ----- Resource assignments (roadmap item 19) -----
  let stakeholders = $state<Stakeholder[]>([]);
  let resourceBusy = $state(false);
  let resourceMsg = $state('');
  // Persistent (not auto-cleared) warning for tasks the leveller could not
  // fit within capacity. resourceWarnTitle holds the full task list for the
  // tooltip when the inline text is truncated.
  let resourceWarn = $state('');
  let resourceWarnTitle = $state('');
  // Leveling heuristic: 'ltf' (least total float, default) or 'edf'
  // (earliest deadline). Passed through to App.LevelChartResources.
  let levelStrategy = $state('ltf');
  // When on, critical-path tasks win resource contention ahead of floating
  // tasks so leveling never delays the critical path.
  let priorityCritical = $state(false);
  // Read-only splitting preview message (activity splitting is analysis-only
  // because a split task can't be stored as a single start pin).
  let splitPreviewMsg = $state('');
  let splitPreviewTitle = $state('');

  async function loadStakeholders() {
    try {
      stakeholders = (await desktopApi.ListStakeholders('')) ?? [];
    } catch {
      stakeholders = []; // suggestions only; free text still works
    }
  }

  function flashResourceMsg(msg: string) {
    resourceMsg = msg;
    if (resourceMsgTimer) clearTimeout(resourceMsgTimer);
    resourceMsgTimer = setTimeout(() => (resourceMsg = ''), 4000);
  }

  function skillTagsText(assignment: { skill_tags?: string[] }): string {
    return (assignment.skill_tags ?? []).join(', ');
  }

  function setSkillTags(assignment: { skill_tags?: string[] }, raw: string) {
    assignment.skill_tags = raw
      .split(',')
      .map((tag) => tag.trim())
      .filter(Boolean);
  }

  function seedDurationEstimate(node: any) {
    const duration = Number(node.duration || 0);
    const optimistic = duration > 0 ? Math.max(0, duration * 0.8) : 0;
    const pessimistic = duration > 0 ? duration * 1.25 : 0;
    node.duration_estimate = {
      optimistic: Number(optimistic.toFixed(2)),
      most_likely: Number(duration.toFixed(2)),
      pessimistic: Number(pessimistic.toFixed(2)),
      distribution: 'triangular',
    };
  }

  function clearDurationEstimate(node: any) {
    delete node.duration_estimate;
  }

  let shellRef = $state<{ reloadFromDB: () => Promise<void>; save: () => Promise<boolean> } | null>(null);

  async function levelResources() {
    if (!session.editingId || resourceBusy) return;
    resourceBusy = true;
    resourceWarn = '';
    resourceWarnTitle = '';
    try {
      const res = await desktopApi.LevelChartResources(
        session.editingId,
        levelStrategy,
        priorityCritical,
        false // CPM editor applies pins only; splitting is applied from the Gantt view
      );
      // Reload the shell's doc from the DB so the editor shows the
      // new SNET pins and a later save can't clobber them.
      await shellRef?.reloadFromDB();
      // Success flash plus a persistent warning for tasks whose demand
      // exceeds capacity (still overallocated; badges also show it).
      const m = levelResourcesMessages(res);
      flashResourceMsg(m.flash);
      resourceWarn = m.warn;
      resourceWarnTitle = m.warnTitle;
    } catch (err: any) {
      flashResourceMsg(String(err?.message ?? err));
    } finally {
      resourceBusy = false;
    }
  }

  // previewSplitting reports (read-only) whether interrupting tasks across
  // non-contiguous days would resolve overallocation, without changing the
  // saved schedule. Splitting isn't persisted: a split task can't be stored
  // as the single SNET start pin the chart model uses.
  async function previewSplitting() {
    if (!session.editingId || resourceBusy) return;
    resourceBusy = true;
    splitPreviewMsg = '';
    splitPreviewTitle = '';
    try {
      const p = await desktopApi.PreviewSplitLeveling(session.editingId);
      const m = splitPreviewMessage(p);
      splitPreviewMsg = m.msg;
      splitPreviewTitle = m.title;
    } catch (err: any) {
      splitPreviewMsg = String(err?.message ?? err);
    } finally {
      resourceBusy = false;
    }
  }

  async function generateHistogram() {
    if (!session.editingId || resourceBusy) return;
    resourceBusy = true;
    try {
      const chart = await desktopApi.GenerateResourceHistogram(session.editingId);
      flashResourceMsg(t('cpm.histogramSaved', { title: chart.title }));
    } catch (err: any) {
      flashResourceMsg(String(err?.message ?? err));
    } finally {
      resourceBusy = false;
    }
  }

  onMount(() => {
    void refreshBaseline();
    void loadStakeholders();
  });

  // The router can reuse this editor instance across chart IDs (same "cpm"
  // view, different session.editingId), so onMount won't re-run. Clear the
  // persistent leveling warning when the edited chart changes so a warning
  // from one schedule never bleeds into another.
  $effect(() => {
    session.editingId;
    resourceWarn = '';
    resourceWarnTitle = '';
    splitPreviewMsg = '';
    splitPreviewTitle = '';
  });
</script>

<LayeredEditorShell bind:this={shellRef} chartKind="cpm" headingLabel={t('cpm.title')}>
  {#snippet toolbarExtra()}
    {#if baselineMsg}
      <span class="text-[10px] text-cyan-300">{baselineMsg}</span>
    {/if}
    {#if resourceMsg}
      <span class="text-[10px] text-orange-300">{resourceMsg}</span>
    {/if}
    {#if resourceWarn}
      <button
        type="button"
        class="text-[10px] text-amber-400 hover:text-amber-300"
        title={resourceWarnTitle}
        onclick={() => {
          resourceWarn = '';
          resourceWarnTitle = '';
        }}
      >{t('cpm.resourceWarn.dismiss', { warning: resourceWarn })}</button>
    {/if}
    <select
      bind:value={levelStrategy}
      disabled={resourceBusy}
      title={t('cpm.level.strategy.hint')}
      class="text-[10px] bg-slate-800 border border-slate-700 rounded px-1 py-0.5 text-slate-200"
    >
      <option value="ltf">{t('cpm.level.leastFloat')}</option>
      <option value="edf">{t('cpm.level.earliestDeadline')}</option>
    </select>
    <label
      class="text-[10px] text-slate-300 flex items-center gap-1"
      title={t('cpm.level.protectCritical.hint')}
    >
      <input type="checkbox" bind:checked={priorityCritical} disabled={resourceBusy} />
      {t('cpm.level.protectCritical')}
    </label>
    <Button
      variant="secondary"
      size="sm"
      onclick={levelResources}
      disabled={resourceBusy}
      title={t('cpm.level.hint')}
    >
      {t('cpm.level')}
    </Button>
    <Button
      variant="secondary"
      size="sm"
      onclick={previewSplitting}
      disabled={resourceBusy}
      title={t('cpm.previewSplitting.hint')}
    >
      {t('chart.previewSplitting')}
    </Button>
    {#if splitPreviewMsg}
      <span class="text-[10px] text-sky-300" title={splitPreviewTitle}>{splitPreviewMsg}</span>
    {/if}
    <Button
      variant="secondary"
      size="sm"
      onclick={generateHistogram}
      disabled={resourceBusy}
      title={t('cpm.histogram.hint')}
    >
      {t('cpm.histogram')}
    </Button>
    <Button
      variant="secondary"
      size="sm"
      onclick={setBaseline}
      disabled={baselineBusy}
      title={t('cpm.baseline.hint')}
    >
      {baselineCount > 0 ? t('cpm.rebaseline', { count: baselineCount }) : t('chart.setBaseline')}
    </Button>
    {#if session.editingId}
      <BaselineList chartId={session.editingId} onchange={refreshBaseline} />
    {/if}
  {/snippet}
  {#snippet nodeContent(data, n)}
    <!-- Critical-path tint overrides the shell's default fill. -->
    {#if data.is_critical}
      <rect
        width={n.width as number}
        height={n.height as number}
        rx="6"
        fill="#7f1d1d"
        opacity="0.6"
      />
    {/if}
    {#if data.constraint_violated}
      <!-- Amber dashed outline + marker: constraint unsatisfiable. -->
      <rect
        width={n.width as number}
        height={n.height as number}
        rx="6"
        fill="none"
        stroke="#f59e0b"
        stroke-width="2"
        stroke-dasharray="4 3"
      />
      <text
        x={(n.width as number) - 12}
        y="16"
        font-size="12"
        font-weight="bold"
        fill="#f59e0b"
      >!</text>
    {/if}
    {#if data.milestone}
      <text
        x={(n.width as number) - 16}
        y={(n.height as number) - 8}
        font-size="10"
        fill="#67e8f9"
      >◆</text>
    {/if}
    {#if (data.percent_complete ?? 0) > 0}
      <!-- Progress strip along the node's bottom edge. -->
      <rect
        x="2"
        y={(n.height as number) - 5}
        width={((n.width as number) - 4) * Math.min(100, Math.max(0, data.percent_complete as number)) / 100}
        height="3"
        rx="1.5"
        fill="#22d3ee"
      />
    {/if}
    {#if data.overallocated}
      <!-- Orange strip on the left edge: a resource is over capacity. -->
      <rect x="0" y="2" width="3" height={(n.height as number) - 4} rx="1.5" fill="#fb923c" />
    {/if}
    <text x="8" y="18" font-size="11" fill="#f1f5f9" font-weight="bold">
      {(data.label as string).length > 22
        ? (data.label as string).slice(0, 21) + '…'
        : data.label as string}
    </text>
    <text x="8" y="34" font-size="9" fill="#94a3b8">
      {t('cpm.node.durationFloat', { duration: fmt(data.duration), float: fmt(data.float, 2) })}
    </text>
    <text x="8" y="48" font-size="9" fill="#67e8f9">
      {#if data.start_date}
        {data.start_date} → {data.finish_date}
      {:else}
        {t('cpm.node.early', { es: fmt(data.es), ef: fmt(data.ef) })}
      {/if}
    </text>
    <text x="8" y="62" font-size="9" fill="#fbbf24">
      {t('cpm.node.late', { ls: fmt(data.ls), lf: fmt(data.lf) })}
    </text>
  {/snippet}

  {#snippet nodeDetailPanel(node)}
    <label class="block">
      <span class="text-xs text-slate-500 uppercase">{t('cpm.duration')}</span>
      <input
        type="number"
        bind:value={node.duration}
        class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
      />
    </label>
    <div class="mt-3 rounded border border-slate-800 bg-slate-950/40 p-2">
      <div class="flex items-center justify-between gap-2">
        <span class="text-xs text-slate-500 uppercase">{t('cpm.estimate')}</span>
        {#if node.duration_estimate}
          <button
            onclick={() => clearDurationEstimate(node)}
            class="text-[10px] text-slate-500 hover:text-red-300"
          >
            {t('cpm.estimate.clear')}
          </button>
        {:else}
          <button
            onclick={() => seedDurationEstimate(node)}
            class="text-[10px] bg-slate-800 hover:bg-slate-700 px-2 py-1 rounded"
          >
            {t('cpm.estimate.use')}
          </button>
        {/if}
      </div>
      {#if node.duration_estimate}
        <div class="mt-2 grid grid-cols-3 gap-2">
          <label class="block">
            <span class="text-[10px] text-slate-500 uppercase">{t('cpm.estimate.optimistic')}</span>
            <input
              type="number"
              min="0"
              step="0.25"
              bind:value={node.duration_estimate.optimistic}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-1.5 rounded text-xs font-mono focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-[10px] text-slate-500 uppercase">{t('cpm.estimate.likely')}</span>
            <input
              type="number"
              min="0"
              step="0.25"
              bind:value={node.duration_estimate.most_likely}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-1.5 rounded text-xs font-mono focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-[10px] text-slate-500 uppercase">{t('cpm.estimate.pessimistic')}</span>
            <input
              type="number"
              min="0"
              step="0.25"
              bind:value={node.duration_estimate.pessimistic}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-1.5 rounded text-xs font-mono focus:border-cyan-500 outline-none"
            />
          </label>
        </div>
        <label class="block mt-2">
          <span class="text-[10px] text-slate-500 uppercase">{t('cpm.estimate.distribution')}</span>
          <select
            bind:value={node.duration_estimate.distribution}
            class="w-full mt-1 bg-slate-950 border border-slate-800 p-1.5 rounded text-xs focus:border-cyan-500 outline-none"
          >
            <option value="triangular">{t('cpm.estimate.triangular')}</option>
            <option value="beta-pert">{t('cpm.estimate.betaPert')}</option>
            <option value="normal">{t('cpm.estimate.normal')}</option>
          </select>
        </label>
      {:else}
        <p class="mt-2 text-[10px] text-slate-500">
          {t('cpm.estimate.hint')}
        </p>
      {/if}
    </div>
    <div class="flex gap-2 mt-2">
      <label class="block flex-1">
        <span class="text-xs text-slate-500 uppercase">{t('cpm.percentComplete')}</span>
        <input
          type="number"
          min="0"
          max="100"
          bind:value={node.percent_complete}
          class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        />
      </label>
      <label class="flex items-end gap-2 pb-2">
        <input type="checkbox" bind:checked={node.milestone} class="accent-cyan-500" />
        <span class="text-xs text-slate-500 uppercase">{t('chart.milestone')}</span>
      </label>
    </div>
    <div class="flex gap-2 mt-2">
      <label class="block flex-1">
        <span class="text-xs text-slate-500 uppercase">{t('cpm.budgetedCost')}</span>
        <input
          type="number"
          min="0"
          bind:value={node.budgeted_cost}
          oninput={(e) => syncMinorUnits(node, 'budgeted_cost', (e.target as HTMLInputElement).value)}
          class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        />
      </label>
      <label class="block flex-1">
        <span class="text-xs text-slate-500 uppercase">{t('cpm.actualCost')}</span>
        <input
          type="number"
          min="0"
          bind:value={node.actual_cost}
          oninput={(e) => syncMinorUnits(node, 'actual_cost', (e.target as HTMLInputElement).value)}
          class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        />
      </label>
    </div>
    <div class="flex gap-2 mt-2">
      <label class="block flex-1">
        <span class="text-xs text-slate-500 uppercase">{t('cpm.actualStart')}</span>
        <input
          type="date"
          bind:value={node.actual_start}
          class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        />
      </label>
      <label class="block flex-1">
        <span class="text-xs text-slate-500 uppercase">{t('cpm.actualFinish')}</span>
        <input
          type="date"
          bind:value={node.actual_finish}
          class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        />
      </label>
    </div>
    <div class="mt-3">
      <span class="text-xs text-slate-500 uppercase">{t('cpm.assignments')}</span>
      {#each node.assignments ?? [] as assignment, i (i)}
        <div class="mt-1 rounded border border-slate-800 bg-slate-950/40 p-2">
          <div class="flex items-center gap-2">
            <input
              bind:value={assignment.resource}
              list="cpm-stakeholders"
              placeholder={t('cpm.assignment.resource')}
              class="flex-1 bg-slate-950 border border-slate-800 p-1.5 rounded text-xs focus:border-cyan-500 outline-none"
            />
            <input
              type="number"
              min="0.1"
              step="0.1"
              bind:value={assignment.units}
              title={t('cpm.assignment.units')}
              class="w-16 bg-slate-950 border border-slate-800 p-1.5 rounded text-xs font-mono focus:border-cyan-500 outline-none"
            />
            <Button
              variant="remove"
              class="px-1 text-xs"
              onclick={() => {
                node.assignments = (node.assignments ?? []).filter((_, j) => j !== i);
              }}
              aria-label={t('cpm.assignment.remove')} title={t('cpm.assignment.remove')}
            >✕</Button>
          </div>
          <div class="mt-2 grid grid-cols-3 gap-2">
            <input
              bind:value={assignment.calendar_id}
              placeholder={t('cpm.assignment.calendar')}
              title={t('cpm.assignment.calendar.hint')}
              class="bg-slate-950 border border-slate-800 p-1.5 rounded text-xs focus:border-cyan-500 outline-none"
            />
            <input
              type="number"
              min="0"
              step="0.1"
              bind:value={assignment.max_units}
              placeholder={t('cpm.assignment.max')}
              title={t('cpm.assignment.max.hint')}
              class="bg-slate-950 border border-slate-800 p-1.5 rounded text-xs font-mono focus:border-cyan-500 outline-none"
            />
            <input
              value={skillTagsText(assignment)}
              oninput={(e) => setSkillTags(assignment, (e.target as HTMLInputElement).value)}
              placeholder={t('cpm.assignment.skills')}
              title={t('cpm.assignment.skills.hint')}
              class="bg-slate-950 border border-slate-800 p-1.5 rounded text-xs focus:border-cyan-500 outline-none"
            />
          </div>
        </div>
      {/each}
      <button
        onclick={() => {
          node.assignments = [...(node.assignments ?? []), { resource: '', units: 1 }];
        }}
        class="mt-1 text-xs bg-slate-800 hover:bg-slate-700 px-2 py-1 rounded"
      >
        {t('cpm.assignment.add')}
      </button>
      <datalist id="cpm-stakeholders">
        {#each stakeholders as s (s.id)}
          <option value={s.name}></option>
        {/each}
      </datalist>
    </div>
    {#if node.overallocated}
      <p class="mt-2 p-2 bg-orange-950 border border-orange-600 rounded text-xs text-orange-300">
        {t('cpm.overallocated')}
      </p>
    {/if}
    <label class="block mt-3">
      <span class="text-xs text-slate-500 uppercase">{t('cpm.constraint')}</span>
      <select
        bind:value={node.constraint}
        class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
      >
        <option value={undefined}>{t('cpm.constraint.asap')}</option>
        <option value="ALAP">{t('cpm.constraint.alap')}</option>
        <option value="SNET">{t('cpm.constraint.snet')}</option>
        <option value="FNLT">{t('cpm.constraint.fnlt')}</option>
        <option value="MFO">{t('cpm.constraint.mfo')}</option>
      </select>
    </label>
    {#if node.constraint === 'SNET' || node.constraint === 'FNLT' || node.constraint === 'MFO'}
      <label class="block mt-2">
        <span class="text-xs text-slate-500 uppercase">{t('cpm.constraint.date')}</span>
        <input
          type="date"
          bind:value={node.constraint_date}
          class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        />
      </label>
    {/if}
    {#if node.constraint_violated}
      <p class="mt-2 p-2 bg-amber-950 border border-amber-600 rounded text-xs text-amber-300">
        {t('cpm.constraint.violated')}
      </p>
    {/if}
    <div class="mt-3 p-2 bg-slate-950 rounded text-xs space-y-1">
      {#if node.start_date}
        <div class="flex justify-between">
          <span class="text-slate-500">{t('cpm.startDate')}</span>
          <span class="text-cyan-300 font-mono">{node.start_date}</span>
        </div>
        <div class="flex justify-between border-b border-slate-800 pb-1 mb-1">
          <span class="text-slate-500">{t('cpm.finishDate')}</span>
          <span class="text-cyan-300 font-mono">{node.finish_date}</span>
        </div>
      {/if}
      <div class="flex justify-between">
        <span class="text-slate-500">{t('cpm.es')}</span>
        <span class="text-cyan-300 font-mono">{fmt(node.es)}</span>
      </div>
      <div class="flex justify-between">
        <span class="text-slate-500">{t('cpm.ef')}</span>
        <span class="text-cyan-300 font-mono">{fmt(node.ef)}</span>
      </div>
      <div class="flex justify-between">
        <span class="text-slate-500">{t('cpm.ls')}</span>
        <span class="text-amber-300 font-mono">{fmt(node.ls)}</span>
      </div>
      <div class="flex justify-between">
        <span class="text-slate-500">{t('cpm.lf')}</span>
        <span class="text-amber-300 font-mono">{fmt(node.lf)}</span>
      </div>
      <div class="flex justify-between border-t border-slate-800 pt-1 mt-1">
        <span class="text-slate-500">{t('cpm.float')}</span>
        <span class="font-mono" class:text-red-400={node.is_critical} class:text-cyan-300={!node.is_critical}>
          {node.is_critical ? t('cpm.float.critical', { float: fmt(node.float, 2) }) : fmt(node.float, 2)}
        </span>
      </div>
      {#if variances[node.id]}
        {@const v = variances[node.id]}
        <div class="border-t border-slate-800 pt-1 mt-1 space-y-1">
          {#if v.baseline_start}
            <div class="flex justify-between">
              <span class="text-slate-500">{t('cpm.baseline')}</span>
              <span class="text-slate-400 font-mono">{v.baseline_start} → {v.baseline_finish}</span>
            </div>
          {/if}
          <div class="flex justify-between">
            <span class="text-slate-500">{t('cpm.startVsBaseline')}</span>
            <span
              class="font-mono"
              class:text-red-400={v.start_var_days > 1e-9}
              class:text-emerald-400={v.start_var_days < -1e-9}
              class:text-slate-400={Math.abs(v.start_var_days) <= 1e-9}
            >{varFmt(v.start_var_days)}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-slate-500">{t('cpm.finishVsBaseline')}</span>
            <span
              class="font-mono"
              class:text-red-400={v.finish_var_days > 1e-9}
              class:text-emerald-400={v.finish_var_days < -1e-9}
              class:text-slate-400={Math.abs(v.finish_var_days) <= 1e-9}
            >{varFmt(v.finish_var_days)}</span>
          </div>
        </div>
      {/if}
    </div>
    <p class="text-[10px] text-slate-500 mt-2">
      {t('cpm.help')}
    </p>
  {/snippet}

  {#snippet asideExtra()}
    <h2 class="text-xs font-bold tracking-widest uppercase text-slate-500 mb-2">
      {t('cpm.evm.title')}
    </h2>
    <div class="flex items-center gap-2">
      <input
        type="date"
        bind:value={evmAsOf}
        title={t('cpm.evm.statusDate')}
        class="flex-1 bg-slate-950 border border-slate-800 p-1.5 rounded text-xs focus:border-cyan-500 outline-none"
      />
      <Button variant="secondary" size="compact" onclick={computeEVM} disabled={evmBusy}>
        {evmBusy ? t('cpm.evm.computing') : t('cpm.evm.compute')}
      </Button>
    </div>
    {#if evmError}
      <p class="mt-2 text-xs text-amber-300">{evmError}</p>
    {/if}
    {#if evm}
      <div class="mt-2 p-2 bg-slate-950 rounded text-xs space-y-1">
        <div class="grid grid-cols-2 gap-x-4 gap-y-1">
          <div class="flex justify-between">
            <span class="text-slate-500">{t('cpm.evm.bac')}</span>
            <span class="font-mono text-slate-300">{money(evm.bac)}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-slate-500">{t('cpm.evm.pv')}</span>
            <span class="font-mono text-slate-300">{money(evm.pv)}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-slate-500">{t('cpm.evm.ev')}</span>
            <span class="font-mono text-slate-300">{money(evm.ev)}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-slate-500">{t('cpm.evm.ac')}</span>
            <span class="font-mono text-slate-300">{money(evm.ac)}</span>
          </div>
        </div>
        <div class="grid grid-cols-2 gap-x-4 gap-y-1 border-t border-slate-800 pt-1 mt-1">
          <div class="flex justify-between">
            <span class="text-slate-500">{t('cpm.evm.sv')}</span>
            <span class="font-mono" class:text-red-400={evm.sv < 0} class:text-emerald-400={evm.sv > 0} class:text-slate-300={evm.sv === 0}>{money(evm.sv)}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-slate-500">{t('cpm.evm.cv')}</span>
            <span class="font-mono" class:text-red-400={evm.cv < 0} class:text-emerald-400={evm.cv > 0} class:text-slate-300={evm.cv === 0}>{money(evm.cv)}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-slate-500">{t('cpm.evm.spi')}</span>
            <span class="font-mono" class:text-red-400={evm.spi > 0 && evm.spi < 1} class:text-emerald-400={evm.spi >= 1} class:text-slate-300={evm.spi === 0}>{idx(evm.spi)}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-slate-500">{t('cpm.evm.cpi')}</span>
            <span class="font-mono" class:text-red-400={evm.cpi > 0 && evm.cpi < 1} class:text-emerald-400={evm.cpi >= 1} class:text-slate-300={evm.cpi === 0}>{idx(evm.cpi)}</span>
          </div>
        </div>
        <div class="grid grid-cols-2 gap-x-4 gap-y-1 border-t border-slate-800 pt-1 mt-1">
          <div class="flex justify-between">
            <span class="text-slate-500">{t('cpm.evm.eac')}</span>
            <span class="font-mono text-slate-300">{money(evm.eac)}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-slate-500">{t('cpm.evm.etc')}</span>
            <span class="font-mono text-slate-300">{money(evm.etc)}</span>
          </div>
          <div class="flex justify-between col-span-2">
            <span class="text-slate-500">{t('cpm.evm.vac')}</span>
            <span class="font-mono" class:text-red-400={evm.vac < 0} class:text-emerald-400={evm.vac > 0} class:text-slate-300={evm.vac === 0}>{money(evm.vac)}</span>
          </div>
        </div>
      </div>
      <p class="text-[10px] text-slate-500 mt-1">
        {t('cpm.evm.help')}
      </p>
    {/if}

    <div class="border-t border-slate-800 mt-4 pt-4">
      <h2 class="text-xs font-bold tracking-widest uppercase text-slate-500 mb-2">
        {t('cpm.risk.title')}
      </h2>
      <div class="flex items-center gap-2">
        <label class="flex-1">
          <span class="sr-only">{t('cpm.risk.iterations')}</span>
          <input
            type="number"
            min="100"
            max="10000"
            step="100"
            bind:value={monteCarloIterations}
            class="w-full bg-slate-950 border border-slate-800 p-1.5 rounded text-xs font-mono focus:border-cyan-500 outline-none"
            title={t('cpm.risk.iterations.hint')}
          />
        </label>
        <Button variant="secondary" size="compact" onclick={runMonteCarlo} disabled={monteCarloBusy}>
          {monteCarloBusy ? t('cpm.risk.running') : t('cpm.risk.run')}
        </Button>
      </div>
      <div class="mt-2 h-1.5 overflow-hidden rounded bg-slate-950">
        <div
          class="h-full rounded bg-cyan-500 transition-all"
          style:width={monteCarloBusy || monteCarlo ? '100%' : '0%'}
          class:animate-pulse={monteCarloBusy}
        ></div>
      </div>
      {#if monteCarloError}
        <p class="mt-2 text-xs text-amber-300">{monteCarloError}</p>
      {/if}
      {#if monteCarlo}
        <div class="mt-2 p-2 bg-slate-950 rounded text-xs space-y-2">
          <div class="grid grid-cols-3 gap-2">
            <div>
              <div class="text-[10px] uppercase text-slate-500">{t('cpm.risk.p50')}</div>
              <div class="font-mono text-cyan-300">{days(monteCarlo.p50)}</div>
            </div>
            <div>
              <div class="text-[10px] uppercase text-slate-500">{t('cpm.risk.p80')}</div>
              <div class="font-mono text-amber-300">{days(monteCarlo.p80)}</div>
            </div>
            <div>
              <div class="text-[10px] uppercase text-slate-500">{t('cpm.risk.p90')}</div>
              <div class="font-mono text-red-300">{days(monteCarlo.p90)}</div>
            </div>
          </div>
          {#if monteCarlo.finish_cdf?.length}
            <div class="rounded border border-slate-800 bg-slate-900/50 p-2">
              <div class="flex justify-between text-[10px] uppercase text-slate-500">
                <span>{t('cpm.risk.finishProbability')}</span>
                <span>{t('cpm.risk.sCurve')}</span>
              </div>
              <svg
                class="mt-1 h-28 w-full"
                viewBox="0 0 242 100"
                role="img"
                aria-label={t('cpm.risk.sCurve.label', { from: days(cdfDomain(monteCarlo)[0]), to: days(cdfDomain(monteCarlo)[1]) })}
              >
                <line x1="14" y1="82" x2="228" y2="82" stroke="#334155" stroke-width="1" />
                <line x1="14" y1="18" x2="14" y2="82" stroke="#334155" stroke-width="1" />
                <line x1="14" y1="50" x2="228" y2="50" stroke="#1e293b" stroke-width="1" stroke-dasharray="3 3" />
                <text x="14" y="95" font-size="8" fill="#64748b">{days(cdfDomain(monteCarlo)[0])}</text>
                <text x="228" y="95" font-size="8" fill="#64748b" text-anchor="end">{days(cdfDomain(monteCarlo)[1])}</text>
                <text x="5" y="84" font-size="8" fill="#64748b" text-anchor="end">0</text>
                <text x="5" y="21" font-size="8" fill="#64748b" text-anchor="end">1</text>
                <path
                  d={cdfPath(monteCarlo)}
                  fill="none"
                  stroke="#22d3ee"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
                {#each confidenceMarkers(monteCarlo) as marker (marker.label)}
                  <line
                    x1={cdfX(monteCarlo, marker.day)}
                    y1={cdfY(marker.probability)}
                    x2={cdfX(monteCarlo, marker.day)}
                    y2="82"
                    stroke={marker.color}
                    stroke-width="1"
                    stroke-dasharray="3 2"
                  />
                  <circle
                    cx={cdfX(monteCarlo, marker.day)}
                    cy={cdfY(marker.probability)}
                    r="2.5"
                    fill={marker.color}
                  />
                  <text
                    x={cdfX(monteCarlo, marker.day)}
                    y={Math.max(10, cdfY(marker.probability) - 5)}
                    font-size="8"
                    text-anchor="middle"
                    fill={marker.color}
                  >{t(marker.label)}</text>
                {/each}
              </svg>
            </div>
          {/if}
          <div class="border-t border-slate-800 pt-2">
            <div class="flex justify-between text-[10px] uppercase text-slate-500">
              <span>{t('cpm.risk.tornado')}</span>
              <span>{t('cpm.risk.runs', { count: monteCarlo.iterations })}</span>
            </div>
            <div class="mt-1 space-y-1">
              {#if tornadoRows().length}
                {#each tornadoRows() as driver (driver.task_id)}
                  <div class="grid grid-cols-[minmax(0,1fr)_4.25rem] items-center gap-2">
                    <div class="min-w-0">
                      <div class="flex items-center justify-between gap-2">
                        <span class="truncate font-mono text-slate-300">{driver.task_id}</span>
                        <span class="font-mono text-slate-400">{driver.score.toFixed(2)}</span>
                      </div>
                      <div class="mt-1 h-1.5 rounded bg-slate-800">
                        <div
                          class="h-full rounded bg-cyan-500"
                          style:width={tornadoBarWidth(driver)}
                        ></div>
                      </div>
                    </div>
                    <div class="justify-self-end text-right leading-tight">
                      <div class="font-mono text-[10px] text-slate-300">{pct(driver.critical_frequency)}</div>
                      <div class="font-mono text-[10px] text-slate-500">{days(driver.duration_spread)}</div>
                    </div>
                  </div>
                {/each}
              {:else}
                <div class="min-w-0">
                  <p class="text-[10px] text-slate-500">{t('cpm.risk.noDrivers')}</p>
                </div>
              {/if}
            </div>
          </div>
        </div>
        <div class="mt-2 flex items-center justify-between gap-2">
          <Button variant="secondary" size="compact" onclick={exportMonteCarloReport} disabled={monteCarloReportBusy}>
            {monteCarloReportBusy ? t('cpm.risk.exporting') : t('cpm.risk.export')}
          </Button>
          <span class="text-[10px] text-slate-500">{t('cpm.risk.report')}</span>
        </div>
        {#if monteCarloReportStatus}
          <p class="mt-1 break-words text-[10px] text-cyan-300">{monteCarloReportStatus}</p>
        {/if}
        {#if monteCarloReportError}
          <p class="mt-1 break-words text-[10px] text-amber-300" role="alert">{monteCarloReportError}</p>
        {/if}
        <p class="text-[10px] text-slate-500 mt-1">
          {t('cpm.risk.help')}
        </p>
      {:else if !monteCarloError}
        <p class="mt-2 text-[10px] text-slate-500">
          {t('cpm.risk.empty')}
        </p>
      {/if}
    </div>
  {/snippet}
</LayeredEditorShell>
