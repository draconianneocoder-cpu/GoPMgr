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

  // ----- Resource assignments (roadmap item 19) -----
  let stakeholders = $state<Stakeholder[]>([]);
  let resourceBusy = $state(false);
  let resourceMsg = $state('');

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

  let shellRef = $state<{ reloadFromDB: () => Promise<void> } | null>(null);

  async function levelResources() {
    if (!session.editingId || resourceBusy) return;
    resourceBusy = true;
    try {
      const pinned = await window.go.main.App.LevelChartResources(session.editingId);
      // Reload the shell's doc from the DB so the editor shows the
      // new SNET pins and a later save can't clobber them.
      await shellRef?.reloadFromDB();
      flashResourceMsg(
        pinned > 0 ? `Levelled: ${pinned} task(s) pinned (SNET)` : 'Already level: nothing moved'
      );
    } catch (err: any) {
      flashResourceMsg(String(err?.message ?? err));
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
</script>

<LayeredEditorShell bind:this={shellRef} chartKind="cpm" headingLabel="CPM Chart">
  {#snippet toolbarExtra()}
    {#if baselineMsg}
      <span class="text-[10px] text-cyan-300">{baselineMsg}</span>
    {/if}
    {#if resourceMsg}
      <span class="text-[10px] text-orange-300">{resourceMsg}</span>
    {/if}
    <button
      onclick={levelResources}
      disabled={resourceBusy}
      class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-3 py-1 rounded"
      title="Delay contended tasks until resources fit capacity; delays persist as SNET constraints"
    >
      Level
    </button>
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
          class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        />
      </label>
      <label class="block flex-1">
        <span class="text-xs text-slate-500 uppercase">Actual cost</span>
        <input
          type="number"
          min="0"
          bind:value={node.actual_cost}
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
        <div class="flex items-center gap-2 mt-1">
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
        at least one scheduled day. Reduce units, move the task, or
        add capacity.
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
  {/snippet}
</LayeredEditorShell>
