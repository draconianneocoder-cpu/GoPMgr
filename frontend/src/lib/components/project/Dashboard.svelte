<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
import { onMount } from 'svelte';
import { session, goto } from '../../session.svelte';
import { showToast } from '../../toast.svelte';
import SignCertificateModal from '../SignCertificateModal.svelte';
import BudgetPanel from './BudgetPanel.svelte';
import ChartCatalog from './ChartCatalog.svelte';
import DocumentCatalog from './DocumentCatalog.svelte';
import Spinner from '../Spinner.svelte';

  let chartKinds = $state<ChartDefinition[]>([]);
  let docKinds = $state<DocumentDefinition[]>([]);
  let charts = $state<ChartRecord[]>([]);
  let docs = $state<DocumentRecord[]>([]);
  let agileEnabled = $state(false);
  let loading = $state(true);
  let loadError = $state('');

  // Delete confirmation: holds the id of the item awaiting confirmation.
  let deletingChartId = $state<string | null>(null);
  let deletingDocId = $state<string | null>(null);

  // Signed export state (shared for all documents in the list)
  let signCertPath = $state('');
  let signGPGKeyID = $state('');
  let preferredSignatureMethod = $state<SignatureMethod>('pades');
  let signatureSettingsLoaded = $state(false);
  let signingDocId = $state<string | null>(null);

  async function load() {
    loading = true;
    loadError = '';
    try {
      chartKinds = (await window.go.main.App.ListChartKinds()) ?? [];
      docKinds = (await window.go.main.App.ListDocumentKinds()) ?? [];
      charts = (await window.go.main.App.ListCharts('')) ?? [];
      docs = (await window.go.main.App.ListDocuments('')) ?? [];
      try {
        agileEnabled = await window.go.main.App.AgileEnabled();
      } catch {
        // Older binary without the Agile bindings — feature stays hidden.
        agileEnabled = false;
      }
    } catch (err: any) {
      loadError = `Could not load this project's charts and documents: ${err}`;
    } finally {
      loading = false;
    }
  }

  onMount(load);

  let showSignModal = $state(false);
  let pendingCertPathForDash = $state('');
  let pendingGPGKeyIDForDash = $state('');
  let pendingSignatureMethodForDash = $state<SignatureMethod>('pades');
  let pendingDocForSign: DocumentRecord | null = $state(null);

  async function exportSignedDocument(d: DocumentRecord) {
    signingDocId = d.id;
    pendingDocForSign = d;

    if (!signatureSettingsLoaded) {
      try {
        const s = await window.go.main.App.GetSettings();
        if (s?.cert_path) signCertPath = s.cert_path;
        if (s?.signature_method) preferredSignatureMethod = s.signature_method;
        if (s?.gpg_key_id) signGPGKeyID = s.gpg_key_id;
        signatureSettingsLoaded = true;
      } catch {}
    }

    pendingCertPathForDash = signCertPath;
    pendingGPGKeyIDForDash = signGPGKeyID;
    pendingSignatureMethodForDash = preferredSignatureMethod;
    showSignModal = true;
    signingDocId = null; // modal will control final state
  }

  function handleDashboardSignedConfirm(options: SignatureExportOptions) {
    showSignModal = false;
    if (!pendingDocForSign) return;

    signingDocId = pendingDocForSign.id;
    (async () => {
      try {
        if (options.method === 'pades') {
          const path = await window.go.main.App.ExportDocumentPDFSigned(
            pendingDocForSign!.id,
            options.cert_path,
            options.cert_password,
          );
          showToast(`PAdES signed PDF exported to: ${path}`, 'success');
        } else if (options.method === 'gpg') {
          const result = await window.go.main.App.ExportDocumentPDFGnuPG(
            pendingDocForSign!.id,
            options.gpg_key_id,
          );
          showToast(`PDF exported to: ${result.pdf_path}; GnuPG signature: ${result.signature_path}`, 'success');
        } else {
          const path = await window.go.main.App.ExportDocumentPDF(pendingDocForSign!.id);
          showToast(`PDF exported without digital signature to: ${path}`, 'success');
        }
      } catch (e: any) {
        showToast(`Export failed: ${e}`, 'error');
      } finally {
        signingDocId = null;
        pendingDocForSign = null;
        pendingCertPathForDash = '';
        pendingGPGKeyIDForDash = '';
      }
    })();
  }

  async function toggleAgile() {
    const next = !agileEnabled;
    try {
      await window.go.main.App.SetAgileEnabled(next);
      agileEnabled = next;
    } catch {
      // Binding missing; do nothing.
    }
  }

  async function confirmDeleteChart(id: string) {
    try {
      await window.go.main.App.DeleteChart(id);
      charts = charts.filter((c) => c.id !== id);
    } catch (err: any) {
      showToast(`Delete failed: ${err}`, 'error');
    } finally {
      deletingChartId = null;
    }
  }

  async function confirmDeleteDoc(id: string) {
    try {
      await window.go.main.App.DeleteDocument(id);
      docs = docs.filter((d) => d.id !== id);
    } catch (err: any) {
      showToast(`Delete failed: ${err}`, 'error');
    } finally {
      deletingDocId = null;
    }
  }

  // ----- New-chart factory --------------------------------------
  //
  // Each kind has its own data-shape default. Keeping this map here
  // (rather than in the registry) lets each "starter document" be
  // expressed as native JSON literals rather than encoded strings.
  const chartStarters: Record<string, () => unknown> = {
    wbs: () => ({ root: { id: 'r', title: session.project!.name, children: [] } }),
    network: () => ({ nodes: [], edges: [] }),
    pert: () => ({ nodes: [], edges: [] }),
    cpm: () => ({ nodes: [], edges: [] }),
    gantt: () => ({ nodes: [], edges: [] }),
    fishbone: () => ({ effect: session.project!.name + ' issue', categories: [] }),
    cause_effect: () => ({
      effect: session.project!.name + ' outcome',
      root: { id: 'r', label: 'Root cause', children: [] },
    }),
    workflow: () => ({ nodes: [], edges: [] }),
    activity: () => ({ swimlanes: [], nodes: [], edges: [] }),
    raci: () => ({ roles: [], tasks: [], assignments: {} }),
    swot: () => ({ strengths: [], weaknesses: [], opportunities: [], threats: [] }),
    stakeholder_analysis: () => ({ stakeholders: [] }),
    matrix: () => ({ rows: [], cols: [], cells: [], rows_label: '', cols_label: '' }),
    risk_matrix: () => ({ items: [] }),
    line: () => ({ title: '', x_label: '', y_label: '', x_str: [], series: [] }),
    bar: () => ({ title: '', x_label: '', y_label: '', categories: [], series: [] }),
    pareto: () => ({ title: '', y_label: 'Count', items: [] }),
    pie: () => ({ title: '', slices: [] }),
    burnup: () => ({ title: '', y_label: 'Story points', days: [], completed: [], scope: [] }),
    burndown: () => ({ title: '', y_label: 'Remaining', days: [], remaining: [] }),
    cumulative_flow: () => ({ title: '', y_label: 'WIP', days: [], states: {}, state_order: [] }),
    control: () => ({ title: '', y_label: '', x: [], y: [], mean: 0, ucl: 0, lcl: 0 }),
  };

  // Which view to route to after creating a chart of a given kind.
  // Other kinds fall through to the generic 'charts' fallback.
  const chartRoutes: Record<string, typeof session.view> = {
    wbs: 'wbs',
    network: 'network',
    pert: 'pert',
    cpm: 'cpm',
    gantt: 'gantt',
    fishbone: 'fishbone',
    cause_effect: 'cause_effect',
    workflow: 'workflow',
    activity: 'activity',
    raci: 'raci',
    swot: 'swot',
    stakeholder_analysis: 'stakeholder',
    matrix: 'matrix',
    risk_matrix: 'risk_matrix',
    line: 'line',
    bar: 'bar',
    pareto: 'pareto',
    pie: 'pie',
    burnup: 'burnup',
    burndown: 'burndown',
    cumulative_flow: 'cumulative_flow',
    control: 'control',
  };

  async function newChart(kind: string, title: string) {
    try {
      const starter = chartStarters[kind]?.() ?? {};
      const c = await window.go.main.App.SaveChart({
        id: '',
        project_id: session.project!.id,
        kind,
        title,
        data: JSON.stringify(starter),
        config: '{}',
        template_id: '',
        created_at: '',
        updated_at: '',
      } as any);
      goto(chartRoutes[kind] ?? 'charts', c.id);
    } catch (err: any) {
      showToast(`Could not create chart: ${err}`, 'error');
    }
  }

  let importMsg = $state('');
  let importDependencies = $state(true);
  let importProgress = $state(true);
  let importAssignments = $state(true);

  async function importMSPDI() {
    importMsg = '';
    try {
      const c = await window.go.main.App.ImportMSPDIChartWithOptions({
        include_dependencies: importDependencies,
        include_progress: importProgress,
        include_assignments: importAssignments,
      });
      goto(chartRoutes['cpm'] ?? 'charts', c.id);
    } catch (err: any) {
      const msg = String(err?.message ?? err);
      if (!msg.includes('cancelled')) importMsg = msg;
    }
  }

  async function newDocument(kind: string, defaultTitle: string) {
    try {
      const d = await window.go.main.App.NewDocument(kind, defaultTitle);
      // Both routes use the structured document editor today, but retaining
      // the charter route preserves its specific screen-reader route label.
      goto(kind.startsWith('charter') ? 'charter' : 'documents', d.id);
    } catch (err: any) {
      showToast(`Could not create document: ${err}`, 'error');
    }
  }

  async function close() {
    await window.go.main.App.CloseProject();
    session.project = null;
    session.projectPath = null;
    goto('portfolio');
  }

  // Human-readable labels come from the backend registries used by the
  // creation catalogs, so an existing artifact and its creation tool always
  // use the same name.
  const chartKindLabel = $derived(new Map(chartKinds.map(c => [c.kind, c.name])));
  const docKindLabel   = $derived(new Map(docKinds.map(d => [d.kind, d.name])));

  const statusStyles: Record<string, string> = {
    active:    'bg-emerald-900 text-emerald-300 border-emerald-700/40',
    planning:  'bg-cyan-900/60 text-cyan-300 border-cyan-700/40',
    on_hold:   'bg-amber-900/60 text-amber-300 border-amber-700/40',
    complete:  'bg-slate-700/40 text-slate-300 border-slate-600/40',
    cancelled: 'bg-red-900/60 text-red-300 border-red-700/40',
  };
  const statusLabel = (s: string) => (s ? s.replace('_', ' ') : 'unknown');

  const docStatusStyles: Record<string, string> = {
    draft:    'bg-slate-700/40 text-slate-400 border-slate-600/40',
    review:   'bg-amber-900/60 text-amber-300 border-amber-700/40',
    approved: 'bg-emerald-900 text-emerald-300 border-emerald-700/40',
    archived: 'bg-slate-800/60 text-slate-500 border-slate-700/40',
  };
</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <header class="border-b border-slate-800 px-6 py-4 flex items-center justify-between gap-4">
    <div class="min-w-0">
      <div class="flex items-center gap-3 flex-wrap">
        <h1 class="text-lg font-bold tracking-widest uppercase truncate">
          {session.project?.name ?? 'Project'}
        </h1>
        {#if session.project?.status}
          <span class={`shrink-0 text-[10px] font-bold uppercase tracking-wider px-2 py-0.5 rounded border ${statusStyles[session.project.status] ?? 'bg-slate-700/40 text-slate-300 border-slate-600/40'}`}>
            {statusLabel(session.project.status)}
          </span>
        {/if}
      </div>
      {#if session.project}
        {@const meta = [
          session.project.phase,
          session.project.methodology,
          (session.project.start_date || session.project.end_date)
            ? `${session.project.start_date || '?'} → ${session.project.end_date || 'ongoing'}`
            : null,
        ].filter(Boolean)}
        {#if meta.length > 0}
          <p class="text-xs text-slate-500 mt-0.5">{meta.join(' · ')}</p>
        {/if}
      {/if}
    </div>
    <div class="flex items-center gap-4 shrink-0">
      <button onclick={() => goto('project_settings')} class="text-xs text-slate-400 hover:text-cyan-400 underline">
        Settings
      </button>
      <button onclick={close} class="text-xs text-slate-400 hover:text-cyan-400 underline">
        Close project
      </button>
    </div>
  </header>

  <SignCertificateModal
    bind:open={showSignModal}
    certPath={pendingCertPathForDash}
    method={pendingSignatureMethodForDash}
    gpgKeyID={pendingGPGKeyIDForDash}
    onConfirm={handleDashboardSignedConfirm}
  />

  <main class="max-w-6xl mx-auto p-8 space-y-8">
    <!-- Project navigation row: Stakeholders + Timeline are always
         available (not gated on a pack toggle). Budget panel shows
         a live summary. -->
    <section>
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <button
          onclick={() => goto('stakeholders')}
          class="p-4 bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded-lg text-left"
        >
          <div class="text-cyan-400 text-[10px] font-bold uppercase tracking-widest">People</div>
          <div class="text-base font-bold text-slate-50 mt-1">Stakeholders</div>
          <p class="text-xs text-slate-500 mt-1">
            Project-level address book; rates &amp; contracts feed the budget panel.
          </p>
        </button>
        <button
          onclick={() => goto('timeline')}
          class="p-4 bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded-lg text-left"
        >
          <div class="text-cyan-400 text-[10px] font-bold uppercase tracking-widest">Calendar</div>
          <div class="text-base font-bold text-slate-50 mt-1">Timeline</div>
          <p class="text-xs text-slate-500 mt-1">
            Sprint + deployment + milestone strip, with country holidays, exportable to iCal.
          </p>
        </button>
        <div class="md:col-span-2 lg:col-span-1">
          <BudgetPanel />
        </div>
      </div>
    </section>

    <!-- Process Excellence (Six Sigma) — shown only for six_sigma methodology projects -->
    {#if session.project?.methodology === 'six_sigma'}
      <section>
        <h2 class="text-sm font-bold uppercase tracking-widest text-slate-500 mb-3">
          Process Excellence
        </h2>
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          <button
            onclick={() => goto('sigma_dashboard')}
            class="p-5 bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded-lg text-left"
          >
            <div class="text-cyan-400 text-[10px] font-bold uppercase tracking-widest">Six Sigma</div>
            <div class="text-base font-bold text-slate-50 mt-1">DMAIC Workspace</div>
            <p class="text-xs text-slate-500 mt-1">
              Define · Measure · Analyze · Improve · Control — phase tollgates, fishbone analysis, and DMAIC project tracking.
            </p>
          </button>
        </div>
      </section>
    {/if}

    <!-- Loading / error state for the project's existing charts & documents. -->
    {#if loading}
      <Spinner label="Loading charts &amp; documents…" />
    {:else if loadError}
      <section class="text-center py-8 space-y-3" role="alert">
        <p class="text-sm text-red-400 break-words">{loadError}</p>
        <button
          onclick={load}
          class="text-xs font-bold uppercase tracking-wider bg-slate-800 hover:bg-slate-700 text-slate-200 px-4 py-2 rounded transition-colors"
        >
          Retry
        </button>
      </section>
    {/if}

    <!-- Existing charts (shown first so returning users reach their work quickly) -->
    {#if charts.length > 0}
    <section>
      <h2 class="text-sm font-bold uppercase tracking-widest text-slate-500 mb-3">Charts</h2>
      <ul class="space-y-2">
        {#each charts as c (c.id)}
          <li class="flex items-center gap-2">
            <button
              onclick={() => goto(chartRoutes[c.kind] ?? 'charts', c.id)}
              class="flex-1 text-left p-3 bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded flex items-center justify-between"
            >
              <div>
                <div class="font-bold text-slate-50">{c.title}</div>
                <div class="text-xs text-slate-500">{chartKindLabel.get(c.kind) ?? c.kind}</div>
              </div>
              <span class="text-xs text-slate-500">{c.updated_at?.slice(0, 10) ?? ''}</span>
            </button>
            {#if deletingChartId === c.id}
              <span class="text-xs text-slate-400 shrink-0">Delete?</span>
              <button
                onclick={() => confirmDeleteChart(c.id)}
                class="text-xs bg-red-700 hover:bg-red-600 text-white px-2 py-1 rounded"
                aria-label={`Confirm delete ${c.title}`}
              >Yes</button>
              <button
                onclick={() => (deletingChartId = null)}
                class="text-xs bg-slate-700 hover:bg-slate-600 text-slate-300 px-2 py-1 rounded"
              >No</button>
            {:else}
              <button
                onclick={() => (deletingChartId = c.id)}
                class="text-xs bg-slate-800 hover:bg-red-900/60 px-2 py-1 rounded text-slate-500 hover:text-red-300"
                aria-label={`Delete ${c.title}`}
              >Delete</button>
            {/if}
          </li>
        {/each}
      </ul>
    </section>
    {/if}

    <!-- Existing documents (shown before new-document actions for return-user flow) -->
    {#if docs.length > 0}
    <section>
      <h2 class="text-sm font-bold uppercase tracking-widest text-slate-500 mb-3">Documents</h2>
      <ul class="space-y-2">
        {#each docs as d (d.id)}
          <li class="flex items-center gap-2">
            <button
              onclick={() => goto(d.kind.startsWith('charter') ? 'charter' : 'documents', d.id)}
              class="flex-1 text-left p-3 bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded flex items-center justify-between gap-3"
            >
              <div class="min-w-0">
                <div class="font-bold text-slate-50 truncate">{d.title}</div>
                <div class="text-xs text-slate-500 mt-0.5">{docKindLabel.get(d.kind) ?? d.kind}</div>
              </div>
              <div class="flex items-center gap-2 shrink-0">
                <span class={`text-[10px] font-bold uppercase tracking-wider px-2 py-0.5 rounded border ${docStatusStyles[d.status] ?? 'bg-slate-700/40 text-slate-400 border-slate-600/40'}`}>
                  {d.status || 'draft'}
                </span>
                <span class="text-xs text-slate-500">v{d.version}</span>
              </div>
            </button>
            <button
              onclick={() => exportSignedDocument(d)}
              disabled={signingDocId === d.id}
              class="text-xs bg-emerald-800 hover:bg-emerald-700 disabled:opacity-50 px-2 py-1 rounded self-center"
              title="Choose PAdES, GnuPG, or no digital signature"
            >
              {signingDocId === d.id ? '…' : 'Signature'}
            </button>
            {#if deletingDocId === d.id}
              <span class="text-xs text-slate-400 shrink-0">Delete?</span>
              <button
                onclick={() => confirmDeleteDoc(d.id)}
                class="text-xs bg-red-700 hover:bg-red-600 text-white px-2 py-1 rounded"
                aria-label={`Confirm delete ${d.title}`}
              >Yes</button>
              <button
                onclick={() => (deletingDocId = null)}
                class="text-xs bg-slate-700 hover:bg-slate-600 text-slate-300 px-2 py-1 rounded"
              >No</button>
            {:else}
              <button
                onclick={() => (deletingDocId = d.id)}
                class="text-xs bg-slate-800 hover:bg-red-900/60 px-2 py-1 rounded text-slate-500 hover:text-red-300"
                aria-label={`Delete ${d.title}`}
              >Delete</button>
            {/if}
          </li>
        {/each}
      </ul>
    </section>
    {/if}

    <!-- One registry-backed chart catalog replaces the former duplicated
         full card grid + engine reference grid. Empty projects open it
         automatically; established projects keep it compact. -->
    <section>
      <div class="flex items-center justify-between mb-3">
        <div>
          <h2 class="text-sm font-bold uppercase tracking-widest text-slate-500">
            Create chart
          </h2>
          <p class="mt-1 text-xs text-slate-500">Choose the right visual instrument for the decision at hand.</p>
        </div>
        <div class="flex items-center gap-2">
          {#if importMsg}
            <span class="text-xs text-amber-300 break-words min-w-0">{importMsg}</span>
          {/if}
          <button
            onclick={importMSPDI}
            class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1.5 rounded"
            title="Import an MSPDI XML schedule as a new CPM chart"
          >
            Import schedule (MSPDI)
          </button>
        </div>
      </div>
      <div class="flex flex-wrap gap-3 mb-3 text-xs text-slate-400" aria-label="MSPDI import fields">
        <label class="flex items-center gap-1"><input type="checkbox" bind:checked={importDependencies} /> Dependencies</label>
        <label class="flex items-center gap-1"><input type="checkbox" bind:checked={importProgress} /> Progress</label>
        <label class="flex items-center gap-1"><input type="checkbox" bind:checked={importAssignments} /> Resource assignments</label>
        <span class="text-slate-500">A mapping receipt is retained with the imported chart.</span>
      </div>
      {#if !loading && !loadError}
        <ChartCatalog
          definitions={chartKinds}
          initiallyExpanded={charts.length === 0}
          onCreate={(definition) => newChart(definition.kind, definition.name)}
        />
      {/if}
    </section>

    <!-- Registry-backed document discovery mirrors the compact chart catalog.
         Combined Report remains a separate composition action because it
         assembles existing documents rather than creating a registry kind. -->
    <section>
      <div class="mb-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 class="text-sm font-bold uppercase tracking-widest text-slate-500">
            Create document
          </h2>
          <p class="mt-1 text-xs text-slate-500">
            Start from a controlled template aligned to the project lifecycle.
          </p>
        </div>
        <button
          onclick={() => goto('report_composer')}
          class="rounded border border-slate-700 bg-slate-900 px-3 py-2 text-left hover:border-slate-600 hover:bg-slate-800"
        >
          <span class="block text-[10px] font-bold uppercase tracking-widest text-cyan-300">Report assembly</span>
          <span class="mt-0.5 block text-xs font-semibold text-slate-100">Build combined report →</span>
        </button>
      </div>
      {#if !loading && !loadError}
        <DocumentCatalog
          definitions={docKinds}
          initiallyExpanded={docs.length === 0}
          onCreate={(definition) => newDocument(definition.kind, definition.name)}
        />
      {/if}
    </section>

    <!-- Agile Pack — opt-in via toggle -->
    <section>
      <div class="flex items-center justify-between mb-3">
        <h2 class="text-sm font-bold uppercase tracking-widest text-slate-500">
          Software-Dev Pack {agileEnabled ? '' : '(disabled)'}
        </h2>
        <button
          onclick={toggleAgile}
          class="text-xs {agileEnabled ? 'bg-slate-800 hover:bg-slate-700' : 'bg-cyan-600 hover:bg-cyan-500 text-white'} px-3 py-1 rounded"
        >
          {agileEnabled ? 'Disable' : 'Enable Agile Pack'}
        </button>
      </div>
      {#if agileEnabled}
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          <button
            onclick={() => goto('kanban')}
            class="p-5 bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded-lg text-left"
          >
            <div class="text-cyan-400 text-[10px] font-bold uppercase tracking-widest">Board</div>
            <div class="text-base font-bold text-slate-50 mt-1">Kanban</div>
            <p class="text-xs text-slate-500 mt-1">
              Drag work items between columns; WIP-limit indicators.
            </p>
          </button>
          <button
            onclick={() => goto('backlog')}
            class="p-5 bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded-lg text-left"
          >
            <div class="text-cyan-400 text-[10px] font-bold uppercase tracking-widest">List</div>
            <div class="text-base font-bold text-slate-50 mt-1">Backlog</div>
            <p class="text-xs text-slate-500 mt-1">
              Prioritized work waiting to be picked up.
            </p>
          </button>
          <button
            onclick={() => goto('sprints')}
            class="p-5 bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded-lg text-left"
          >
            <div class="text-cyan-400 text-[10px] font-bold uppercase tracking-widest">Iteration</div>
            <div class="text-base font-bold text-slate-50 mt-1">Sprints</div>
            <p class="text-xs text-slate-500 mt-1">
              Plan, activate, and complete time-boxed sprints.
            </p>
          </button>
          <button
            onclick={() => goto('dora')}
            class="p-5 bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded-lg text-left"
          >
            <div class="text-cyan-400 text-[10px] font-bold uppercase tracking-widest">Metrics</div>
            <div class="text-base font-bold text-slate-50 mt-1">DORA Dashboard</div>
            <p class="text-xs text-slate-500 mt-1">
              Deploy frequency, lead time, CFR, MTTR with classifications.
            </p>
          </button>
        </div>
      {:else}
        <p class="text-xs text-slate-500">
          Enable the Software-Dev Pack to add Kanban, Backlog, Sprints, and DORA metrics
          to this project. The pack stores its data in this project's <code>.pmforge</code>
          file; disabling hides it without deleting anything.
        </p>
      {/if}
    </section>

  </main>
</div>
