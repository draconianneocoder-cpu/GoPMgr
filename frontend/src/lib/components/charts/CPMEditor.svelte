<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // CPMEditor records each activity's Duration. The backend runs the
  // full CPM forward/backward pass (kernel.CalculateCPM) and writes
  // ES/EF/LS/LF/Float plus IsCritical back per node. Critical-path
  // activities are tinted red on the canvas.
  import { onMount } from 'svelte';
  import { session } from '../../session.svelte';
  import LayeredEditorShell from './_layered_editor_shell.svelte';
  import { levelResourcesMessages, splitPreviewMessage } from './leveling_messages';

  function fmt(n: unknown, digits = 1): string {
    return typeof n === 'number' ? n.toFixed(digits) : '—';
  }

  // ----- Schedule baseline (roadmap item 17) -----
  let baselineCount = $state(0);
  let variances = $state<Record<string, ScheduleVariance>>({});
  let baselineBusy = $state(false);
  let baselineMsg = $state('');

  async function refreshBaseline() {
    if (!session.editingId) return;
    try {
      const list = await window.go.main.App.ListScheduleBaselines(session.editingId);
      baselineCount = list?.length ?? 0;
      variances = baselineCount > 0
        ? await window.go.main.App.CompareScheduleBaseline(session.editingId, '')
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
      await window.go.main.App.SetScheduleBaseline(session.editingId, '');
      await refreshBaseline();
      baselineMsg = 'Baseline set';
      setTimeout(() => (baselineMsg = ''), 2500);
    } catch (err: any) {
      baselineMsg = String(err?.message ?? err);
    } finally {
      baselineBusy = false;
    }
  }

  function varFmt(days: number): string {
    if (Math.abs(days) < 1e-9) return 'on plan';
    return days > 0 ? `+${days.toFixed(1)}d late` : `${days.toFixed(1)}d early`;
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
      evm = await window.go.main.App.ComputeScheduleEVM(session.editingId, evmAsOf);
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

  function days(n: number): string {
    return `${n.toFixed(1)}d`;
  }

  function pct(n: number): string {
    return `${Math.round(n * 100)}%`;
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
      { label: 'P50', day: result.p50, probability: 0.5, color: '#67e8f9' },
      { label: 'P80', day: result.p80, probability: 0.8, color: '#fbbf24' },
      { label: 'P90', day: result.p90, probability: 0.9, color: '#fca5a5' },
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
      monteCarlo = await window.go.main.App.RunChartMonteCarlo(session.editingId, iterations, 0);
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
      const path = await window.go.main.App.ExportChartMonteCarloRiskReport(session.editingId, iterations, 0);
      monteCarloReportStatus = `Exported to: ${path}`;
    } catch (err: any) {
      monteCarloReportError = `Export failed: ${String(err?.message ?? err)}`;
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
      stakeholders = (await window.go.main.App.ListStakeholders('')) ?? [];
    } catch {
      stakeholders = []; // suggestions only; free text still works
    }
  }

  function flashResourceMsg(msg: string) {
    resourceMsg = msg;
    setTimeout(() => (resourceMsg = ''), 4000);
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

  let shellRef = $state<{ reloadFromDB: () => Promise<void>; save: () => Promise<void> } | null>(null);

  async function levelResources() {
    if (!session.editingId || resourceBusy) return;
    resourceBusy = true;
    resourceWarn = '';
    resourceWarnTitle = '';
    try {
      const res = await window.go.main.App.LevelChartResources(
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
      const p = await window.go.main.App.PreviewSplitLeveling(session.editingId);
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
      const chart = await window.go.main.App.GenerateResourceHistogram(session.editingId);
      flashResourceMsg(`Histogram saved: ${chart.title}`);
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

<LayeredEditorShell bind:this={shellRef} chartKind="cpm" headingLabel="CPM Chart">
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
      >⚠ {resourceWarn} (dismiss)</button>
    {/if}
    <select
      bind:value={levelStrategy}
      disabled={resourceBusy}
      title="Leveling heuristic: least total float protects the project finish; earliest deadline favours per-task due dates."
      class="text-[10px] bg-slate-800 border border-slate-700 rounded px-1 py-0.5 text-slate-200"
    >
      <option value="ltf">Least float</option>
      <option value="edf">Earliest deadline</option>
    </select>
    <label
      class="text-[10px] text-slate-300 flex items-center gap-1"
      title="Protect the critical path: critical tasks win resource contention ahead of floating tasks."
    >
      <input type="checkbox" bind:checked={priorityCritical} disabled={resourceBusy} />
      Protect critical
    </label>
    <button
      onclick={levelResources}
      disabled={resourceBusy}
      class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-3 py-1 rounded"
      title="Delay contended tasks until resources fit capacity; delays persist as SNET constraints"
    >
      Level
    </button>
    <button
      onclick={previewSplitting}
      disabled={resourceBusy}
      class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-3 py-1 rounded"
      title="Preview (read-only) whether interrupting tasks across non-contiguous days would clear overallocation. Not saved."
    >
      Preview splitting
    </button>
    {#if splitPreviewMsg}
      <span class="text-[10px] text-sky-300" title={splitPreviewTitle}>{splitPreviewMsg}</span>
    {/if}
    <button
      onclick={generateHistogram}
      disabled={resourceBusy}
      class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-3 py-1 rounded"
      title="Save a bar chart of per-day resource demand (snapshot; regenerate after edits)"
    >
      Histogram
    </button>
    <button
      onclick={setBaseline}
      disabled={baselineBusy}
      class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-3 py-1 rounded"
      title="Snapshot the current schedule for planned-vs-actual comparison"
    >
      {baselineCount > 0 ? `Re-baseline (${baselineCount})` : 'Set baseline'}
    </button>
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
      Dur: {fmt(data.duration)} · Float: {fmt(data.float, 2)}
    </text>
    <text x="8" y="48" font-size="9" fill="#67e8f9">
      {#if data.start_date}
        {data.start_date} → {data.finish_date}
      {:else}
        ES {fmt(data.es)} · EF {fmt(data.ef)}
      {/if}
    </text>
    <text x="8" y="62" font-size="9" fill="#fbbf24">
      LS {fmt(data.ls)} · LF {fmt(data.lf)}
    </text>
  {/snippet}

  {#snippet nodeDetailPanel(node)}
    <label class="block">
      <span class="text-xs text-slate-500 uppercase">Duration (days)</span>
      <input
        type="number"
        bind:value={node.duration}
        class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
      />
    </label>
    <div class="mt-3 rounded border border-slate-800 bg-slate-950/40 p-2">
      <div class="flex items-center justify-between gap-2">
        <span class="text-xs text-slate-500 uppercase">Monte Carlo estimate</span>
        {#if node.duration_estimate}
          <button
            onclick={() => clearDurationEstimate(node)}
            class="text-[10px] text-slate-500 hover:text-red-300"
          >
            Clear
          </button>
        {:else}
          <button
            onclick={() => seedDurationEstimate(node)}
            class="text-[10px] bg-slate-800 hover:bg-slate-700 px-2 py-1 rounded"
          >
            Use estimate
          </button>
        {/if}
      </div>
      {#if node.duration_estimate}
        <div class="mt-2 grid grid-cols-3 gap-2">
          <label class="block">
            <span class="text-[10px] text-slate-500 uppercase">Optimistic</span>
            <input
              type="number"
              min="0"
              step="0.25"
              bind:value={node.duration_estimate.optimistic}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-1.5 rounded text-xs font-mono focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-[10px] text-slate-500 uppercase">Likely</span>
            <input
              type="number"
              min="0"
              step="0.25"
              bind:value={node.duration_estimate.most_likely}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-1.5 rounded text-xs font-mono focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-[10px] text-slate-500 uppercase">Pessimistic</span>
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
          <span class="text-[10px] text-slate-500 uppercase">Distribution</span>
          <select
            bind:value={node.duration_estimate.distribution}
            class="w-full mt-1 bg-slate-950 border border-slate-800 p-1.5 rounded text-xs focus:border-cyan-500 outline-none"
          >
            <option value="triangular">Triangular</option>
            <option value="beta-pert">Beta-PERT</option>
            <option value="normal">Normal</option>
          </select>
        </label>
      {:else}
        <p class="mt-2 text-[10px] text-slate-500">
          Leave blank for deterministic duration. Add a three-point estimate for schedule-risk simulation.
        </p>
      {/if}
    </div>
    <div class="flex gap-2 mt-2">
      <label class="block flex-1">
        <span class="text-xs text-slate-500 uppercase">% Complete</span>
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
        <span class="text-xs text-slate-500 uppercase">Milestone</span>
      </label>
    </div>
    <div class="flex gap-2 mt-2">
      <label class="block flex-1">
        <span class="text-xs text-slate-500 uppercase">Budgeted cost</span>
        <input
          type="number"
          min="0"
          bind:value={node.budgeted_cost}
          oninput={(e) => syncMinorUnits(node, 'budgeted_cost', (e.target as HTMLInputElement).value)}
          class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        />
      </label>
      <label class="block flex-1">
        <span class="text-xs text-slate-500 uppercase">Actual cost</span>
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
        <span class="text-xs text-slate-500 uppercase">Actual start</span>
        <input
          type="date"
          bind:value={node.actual_start}
          class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        />
      </label>
      <label class="block flex-1">
        <span class="text-xs text-slate-500 uppercase">Actual finish</span>
        <input
          type="date"
          bind:value={node.actual_finish}
          class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        />
      </label>
    </div>
    <div class="mt-3">
      <span class="text-xs text-slate-500 uppercase">Assignments</span>
      {#each node.assignments ?? [] as assignment, i (i)}
        <div class="mt-1 rounded border border-slate-800 bg-slate-950/40 p-2">
          <div class="flex items-center gap-2">
            <input
              bind:value={assignment.resource}
              list="cpm-stakeholders"
              placeholder="Resource"
              class="flex-1 bg-slate-950 border border-slate-800 p-1.5 rounded text-xs focus:border-cyan-500 outline-none"
            />
            <input
              type="number"
              min="0.1"
              step="0.1"
              bind:value={assignment.units}
              title="Units (1 = full-time)"
              class="w-16 bg-slate-950 border border-slate-800 p-1.5 rounded text-xs font-mono focus:border-cyan-500 outline-none"
            />
            <button
              onclick={() => {
                node.assignments = (node.assignments ?? []).filter((_, j) => j !== i);
              }}
              class="text-xs text-slate-500 hover:text-red-400 px-1"
              title="Remove assignment"
            >✕</button>
          </div>
          <div class="mt-2 grid grid-cols-3 gap-2">
            <input
              bind:value={assignment.calendar_id}
              placeholder="Calendar"
              title="Optional named resource calendar"
              class="bg-slate-950 border border-slate-800 p-1.5 rounded text-xs focus:border-cyan-500 outline-none"
            />
            <input
              type="number"
              min="0"
              step="0.1"
              bind:value={assignment.max_units}
              placeholder="Max"
              title="Optional max units for this assignment"
              class="bg-slate-950 border border-slate-800 p-1.5 rounded text-xs font-mono focus:border-cyan-500 outline-none"
            />
            <input
              value={skillTagsText(assignment)}
              oninput={(e) => setSkillTags(assignment, (e.target as HTMLInputElement).value)}
              placeholder="Skills"
              title="Comma-separated skill tags"
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
        + Assign resource
      </button>
      <datalist id="cpm-stakeholders">
        {#each stakeholders as s (s.id)}
          <option value={s.name}></option>
        {/each}
      </datalist>
    </div>
    {#if node.overallocated}
      <p class="mt-2 p-2 bg-orange-950 border border-orange-600 rounded text-xs text-orange-300">
        Overallocated: a resource on this task exceeds its capacity on
        at least one scheduled day, including Resource Capacity calendar
        overrides. Reduce units, move the task, or add capacity.
      </p>
    {/if}
    <label class="block mt-3">
      <span class="text-xs text-slate-500 uppercase">Constraint</span>
      <select
        bind:value={node.constraint}
        class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
      >
        <option value={undefined}>ASAP (default)</option>
        <option value="ALAP">As late as possible</option>
        <option value="SNET">Start no earlier than</option>
        <option value="FNLT">Finish no later than</option>
        <option value="MFO">Must finish on</option>
      </select>
    </label>
    {#if node.constraint === 'SNET' || node.constraint === 'FNLT' || node.constraint === 'MFO'}
      <label class="block mt-2">
        <span class="text-xs text-slate-500 uppercase">Constraint date</span>
        <input
          type="date"
          bind:value={node.constraint_date}
          class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        />
      </label>
    {/if}
    {#if node.constraint_violated}
      <p class="mt-2 p-2 bg-amber-950 border border-amber-600 rounded text-xs text-amber-300">
        Constraint violated: precedence links make this constraint
        unsatisfiable. Links win; adjust the constraint date, the
        durations, or the links.
      </p>
    {/if}
    <div class="mt-3 p-2 bg-slate-950 rounded text-xs space-y-1">
      {#if node.start_date}
        <div class="flex justify-between">
          <span class="text-slate-500">Start date</span>
          <span class="text-cyan-300 font-mono">{node.start_date}</span>
        </div>
        <div class="flex justify-between border-b border-slate-800 pb-1 mb-1">
          <span class="text-slate-500">Finish date</span>
          <span class="text-cyan-300 font-mono">{node.finish_date}</span>
        </div>
      {/if}
      <div class="flex justify-between">
        <span class="text-slate-500">Earliest start (ES)</span>
        <span class="text-cyan-300 font-mono">{fmt(node.es)}</span>
      </div>
      <div class="flex justify-between">
        <span class="text-slate-500">Earliest finish (EF)</span>
        <span class="text-cyan-300 font-mono">{fmt(node.ef)}</span>
      </div>
      <div class="flex justify-between">
        <span class="text-slate-500">Latest start (LS)</span>
        <span class="text-amber-300 font-mono">{fmt(node.ls)}</span>
      </div>
      <div class="flex justify-between">
        <span class="text-slate-500">Latest finish (LF)</span>
        <span class="text-amber-300 font-mono">{fmt(node.lf)}</span>
      </div>
      <div class="flex justify-between border-t border-slate-800 pt-1 mt-1">
        <span class="text-slate-500">Float</span>
        <span class="font-mono" class:text-red-400={node.is_critical} class:text-cyan-300={!node.is_critical}>
          {fmt(node.float, 2)}{node.is_critical ? ' (critical)' : ''}
        </span>
      </div>
      {#if variances[node.id]}
        {@const v = variances[node.id]}
        <div class="border-t border-slate-800 pt-1 mt-1 space-y-1">
          {#if v.baseline_start}
            <div class="flex justify-between">
              <span class="text-slate-500">Baseline</span>
              <span class="text-slate-400 font-mono">{v.baseline_start} → {v.baseline_finish}</span>
            </div>
          {/if}
          <div class="flex justify-between">
            <span class="text-slate-500">Start vs baseline</span>
            <span
              class="font-mono"
              class:text-red-400={v.start_var_days > 1e-9}
              class:text-emerald-400={v.start_var_days < -1e-9}
              class:text-slate-400={Math.abs(v.start_var_days) <= 1e-9}
            >{varFmt(v.start_var_days)}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-slate-500">Finish vs baseline</span>
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
      ES/EF/LS/LF computed server-side via internal/kernel CPM. Any node
      with Float = 0 lies on the critical path and is highlighted red.
      Real start/finish dates appear once the project has a start date
      (Project Settings); non-working days are skipped using the
      project country's calendar. Incoming-link labels set the
      dependency type and lag (FS/SS/FF/SF, e.g. SS+2); blank means FS.
      Date constraints (SNET/FNLT/MFO) also need the project start
      date; ALAP works without one. Links always win over constraints;
      impossible constraints flag the node amber instead. Resource
      assignments default to capacity 1.0 per resource; tasks whose
      resources are over capacity get an orange edge strip.
    </p>
  {/snippet}

  {#snippet asideExtra()}
    <h2 class="text-xs font-bold tracking-widest uppercase text-slate-500 mb-2">
      Earned value
    </h2>
    <div class="flex items-center gap-2">
      <input
        type="date"
        bind:value={evmAsOf}
        title="Status date (blank = today)"
        class="flex-1 bg-slate-950 border border-slate-800 p-1.5 rounded text-xs focus:border-cyan-500 outline-none"
      />
      <button
        onclick={computeEVM}
        disabled={evmBusy}
        class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-3 py-1.5 rounded"
      >
        {evmBusy ? 'Computing…' : 'Compute'}
      </button>
    </div>
    {#if evmError}
      <p class="mt-2 text-xs text-amber-300">{evmError}</p>
    {/if}
    {#if evm}
      <div class="mt-2 p-2 bg-slate-950 rounded text-xs space-y-1">
        <div class="grid grid-cols-2 gap-x-4 gap-y-1">
          <div class="flex justify-between">
            <span class="text-slate-500">BAC</span>
            <span class="font-mono text-slate-300">{money(evm.bac)}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-slate-500">PV</span>
            <span class="font-mono text-slate-300">{money(evm.pv)}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-slate-500">EV</span>
            <span class="font-mono text-slate-300">{money(evm.ev)}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-slate-500">AC</span>
            <span class="font-mono text-slate-300">{money(evm.ac)}</span>
          </div>
        </div>
        <div class="grid grid-cols-2 gap-x-4 gap-y-1 border-t border-slate-800 pt-1 mt-1">
          <div class="flex justify-between">
            <span class="text-slate-500">SV</span>
            <span class="font-mono" class:text-red-400={evm.sv < 0} class:text-emerald-400={evm.sv > 0} class:text-slate-300={evm.sv === 0}>{money(evm.sv)}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-slate-500">CV</span>
            <span class="font-mono" class:text-red-400={evm.cv < 0} class:text-emerald-400={evm.cv > 0} class:text-slate-300={evm.cv === 0}>{money(evm.cv)}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-slate-500">SPI</span>
            <span class="font-mono" class:text-red-400={evm.spi > 0 && evm.spi < 1} class:text-emerald-400={evm.spi >= 1} class:text-slate-300={evm.spi === 0}>{idx(evm.spi)}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-slate-500">CPI</span>
            <span class="font-mono" class:text-red-400={evm.cpi > 0 && evm.cpi < 1} class:text-emerald-400={evm.cpi >= 1} class:text-slate-300={evm.cpi === 0}>{idx(evm.cpi)}</span>
          </div>
        </div>
        <div class="grid grid-cols-2 gap-x-4 gap-y-1 border-t border-slate-800 pt-1 mt-1">
          <div class="flex justify-between">
            <span class="text-slate-500">EAC</span>
            <span class="font-mono text-slate-300">{money(evm.eac)}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-slate-500">ETC</span>
            <span class="font-mono text-slate-300">{money(evm.etc)}</span>
          </div>
          <div class="flex justify-between col-span-2">
            <span class="text-slate-500">VAC</span>
            <span class="font-mono" class:text-red-400={evm.vac < 0} class:text-emerald-400={evm.vac > 0} class:text-slate-300={evm.vac === 0}>{money(evm.vac)}</span>
          </div>
        </div>
      </div>
      <p class="text-[10px] text-slate-500 mt-1">
        PV from the schedule, EV from % complete × budgeted cost, AC
        from actual cost. SPI/CPI below 1 means behind schedule / over
        cost; n/a until work is planned or cost incurred.
      </p>
    {/if}

    <div class="border-t border-slate-800 mt-4 pt-4">
      <h2 class="text-xs font-bold tracking-widest uppercase text-slate-500 mb-2">
        Monte Carlo risk
      </h2>
      <div class="flex items-center gap-2">
        <label class="flex-1">
          <span class="sr-only">Iterations</span>
          <input
            type="number"
            min="100"
            max="10000"
            step="100"
            bind:value={monteCarloIterations}
            class="w-full bg-slate-950 border border-slate-800 p-1.5 rounded text-xs font-mono focus:border-cyan-500 outline-none"
            title="Simulation iterations"
          />
        </label>
        <button
          onclick={runMonteCarlo}
          disabled={monteCarloBusy}
          class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-3 py-1.5 rounded"
        >
          {monteCarloBusy ? 'Running…' : 'Run'}
        </button>
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
              <div class="text-[10px] uppercase text-slate-500">P50</div>
              <div class="font-mono text-cyan-300">{days(monteCarlo.p50)}</div>
            </div>
            <div>
              <div class="text-[10px] uppercase text-slate-500">P80</div>
              <div class="font-mono text-amber-300">{days(monteCarlo.p80)}</div>
            </div>
            <div>
              <div class="text-[10px] uppercase text-slate-500">P90</div>
              <div class="font-mono text-red-300">{days(monteCarlo.p90)}</div>
            </div>
          </div>
          {#if monteCarlo.finish_cdf?.length}
            <div class="rounded border border-slate-800 bg-slate-900/50 p-2">
              <div class="flex justify-between text-[10px] uppercase text-slate-500">
                <span>Finish probability</span>
                <span>S-curve</span>
              </div>
              <svg
                class="mt-1 h-28 w-full"
                viewBox="0 0 242 100"
                role="img"
                aria-label={`Monte Carlo finish probability S-curve from ${days(cdfDomain(monteCarlo)[0])} to ${days(cdfDomain(monteCarlo)[1])}`}
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
                  >{marker.label}</text>
                {/each}
              </svg>
            </div>
          {/if}
          <div class="border-t border-slate-800 pt-2">
            <div class="flex justify-between text-[10px] uppercase text-slate-500">
              <span>Tornado drivers</span>
              <span>{monteCarlo.iterations.toLocaleString()} runs</span>
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
                  <p class="text-[10px] text-slate-500">No variable risk drivers detected.</p>
                </div>
              {/if}
            </div>
          </div>
        </div>
        <div class="mt-2 flex items-center justify-between gap-2">
          <button
            onclick={exportMonteCarloReport}
            disabled={monteCarloReportBusy}
            class="rounded bg-slate-800 px-2.5 py-1.5 text-xs text-slate-100 hover:bg-slate-700 disabled:opacity-50"
          >
            {monteCarloReportBusy ? 'Exporting…' : 'Export PDF/A'}
          </button>
          <span class="text-[10px] text-slate-500">Risk report</span>
        </div>
        {#if monteCarloReportStatus}
          <p class="mt-1 break-words text-[10px] text-cyan-300">{monteCarloReportStatus}</p>
        {/if}
        {#if monteCarloReportError}
          <p class="mt-1 break-words text-[10px] text-amber-300" role="alert">{monteCarloReportError}</p>
        {/if}
        <p class="text-[10px] text-slate-500 mt-1">
          P50/P80/P90 are finish-day confidence points. Tornado drivers rank
          critical-path frequency multiplied by P90-P50 duration spread.
        </p>
      {:else if !monteCarloError}
        <p class="mt-2 text-[10px] text-slate-500">
          Add optional task estimates, then run a simulation. Tasks without estimates use their fixed duration.
        </p>
      {/if}
    </div>
  {/snippet}
</LayeredEditorShell>
