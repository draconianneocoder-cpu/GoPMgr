// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';

import Dashboard from './Dashboard.svelte';
import { getToasts } from '../../toast.svelte';
import { cancelNavigation, navigation, session } from '../../session.svelte';

let app: Record<string, ReturnType<typeof vi.fn>>;

const chartKinds: ChartDefinition[] = [
  { kind: 'wbs', name: 'Work Breakdown Structure', engine: 'dag', description: 'Decompose scope.', data_example: '{}' },
  { kind: 'raci', name: 'RACI Matrix', engine: 'matrix', description: 'Responsibility matrix.', data_example: '{}' },
];

const docKinds: DocumentDefinition[] = [
  { kind: 'charter', name: 'Project Charter', phase: 'initiation', description: 'Founding document.', fields: [] },
  { kind: 'business_case', name: 'Business Case', phase: 'initiation', description: 'Justification.', fields: [] },
];

const chart: ChartRecord = {
  id: 'chart-1',
  project_id: 'p1',
  kind: 'wbs',
  title: 'Existing WBS',
  data: '{}',
  config: '{}',
  template_id: '',
  created_at: '',
  updated_at: '2026-08-01T00:00:00Z',
};

const doc: DocumentRecord = {
  id: 'doc-1',
  project_id: 'p1',
  kind: 'business_case',
  title: 'Existing Business Case',
  content: '{}',
  template_id: '',
  version: 2,
  status: 'review',
  created_at: '',
  updated_at: '',
};

// Default mock returns an empty project (so both catalogs auto-expand, per
// Dashboard's `initiallyExpanded={charts.length === 0}` contract) unless a
// test overrides ListCharts/ListDocuments.
function makeApp(overrides: Record<string, ReturnType<typeof vi.fn>> = {}) {
  return {
    ListChartKinds: vi.fn(async () => chartKinds),
    ListDocumentKinds: vi.fn(async () => docKinds),
    ListCharts: vi.fn(async () => []),
    ListDocuments: vi.fn(async () => []),
    AgileEnabled: vi.fn(async () => false),
    SetAgileEnabled: vi.fn(async () => undefined),
    ComputeBudget: vi.fn(async () => ({
      budget: 0,
      contract_value: 0,
      labour_estimate: 0,
      committed: 0,
      remaining: 0,
      budget_minor_units: 0,
      contract_value_minor_units: 0,
      labour_estimate_minor_units: 0,
      committed_minor_units: 0,
      remaining_minor_units: 0,
      by_category: {},
      by_category_minor_units: {},
    })),
    DeleteChart: vi.fn(async () => undefined),
    DeleteDocument: vi.fn(async () => undefined),
    SaveChart: vi.fn(async (c: ChartRecord) => ({ ...c, id: 'new-chart-1' })),
    NewDocument: vi.fn(async (kind: string) => ({ id: 'new-doc-1', kind })),
    ImportMSPDIChartWithOptions: vi.fn(async () => ({ id: 'imported-cpm-1' })),
    CloseProject: vi.fn(async () => undefined),
    GetSettings: vi.fn(async () => ({ cert_path: '', signature_method: 'pades', gpg_key_id: '' })),
    ExportDocumentPDF: vi.fn(async () => '/tmp/out.pdf'),
    ExportDocumentPDFGnuPG: vi.fn(async () => ({ pdf_path: '/tmp/out.pdf', signature_path: '/tmp/out.pdf.asc', method: 'gpg' })),
    ExportDocumentPDFSigned: vi.fn(async () => '/tmp/out-signed.pdf'),
    ...overrides,
  };
}

beforeEach(() => {
  session.project = { id: 'p1', name: 'Test Project', methodology: 'agile' } as unknown as typeof session.project;
  // Non-default sentinel: assertions that check "did NOT navigate away"
  // compare against this rather than against `afterEach`'s reset value, so
  // a bug that left `session.view` at its ambient default couldn't pass by
  // accident.
  session.view = 'dashboard';
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  cancelNavigation();
  navigation.saving = false;
  navigation.error = '';
  session.view = 'portfolio';
  session.editingId = null;
  session.project = null;
});

function setApp(overrides: Record<string, ReturnType<typeof vi.fn>> = {}) {
  app = makeApp(overrides);
  (window as unknown as { go: unknown }).go = { main: { App: app } };
  return app;
}

// `load()` (Dashboard's onMount) runs five sequential backend calls ending
// with AgileEnabled(), and its results (agileEnabled, charts, docs) are
// assigned as each call resolves. Every interactive control is rendered
// immediately, not gated on `loading` — so an action dispatched before
// load() finishes races its own late assignments and can be silently
// clobbered. Confirmed empirically while writing this suite: toggling
// agileEnabled before load() reached its own AgileEnabled() call left the
// backend correctly updated but the UI reverted to the pre-toggle value
// once load() caught up. This is a real, deferred defect in Dashboard.svelte
// (see .agent_memory/dashboard-coverage-2026-08-15.md), not a test-only
// concern — every test below that interacts with the component after the
// initial render must wait out the full mount through this helper so the
// suite exercises intended behavior rather than the race. Only the spinner
// test and the load-error test deliberately observe mid-load state and
// render directly instead.
async function renderLoaded(overrides: Record<string, ReturnType<typeof vi.fn>> = {}) {
  setApp(overrides);
  const utils = render(Dashboard);
  await waitFor(() => expect(app.AgileEnabled).toHaveBeenCalled());
  return utils;
}

// Dashboard IA restructuring (design doc §3.3/R3/R4): the chart-creation
// catalog, document-creation catalog, MSPDI import, and Software-Dev Pack
// sections moved off the default Overview tab into their own tabs. Tests
// that interact with them must switch tabs first -- tab switching is local
// component state, independent of load(), so it's always safe to click
// immediately, even mid-load.
async function switchTab(getByRole: ReturnType<typeof render>['getByRole'], name: RegExp | string) {
  await fireEvent.click(getByRole('tab', { name }));
}

describe('Dashboard load', () => {
  it('shows a spinner while data is loading, then the loaded content', async () => {
    let resolveCharts: (v: ChartRecord[]) => void;
    setApp({
      ListCharts: vi.fn(() => new Promise<ChartRecord[]>((resolve) => (resolveCharts = resolve))),
    });

    const { getByText, queryByText } = render(Dashboard);
    expect(getByText(/loading charts/i)).toBeInTheDocument();

    await waitFor(() => expect(resolveCharts).toBeDefined());
    resolveCharts!([chart]);
    await waitFor(() => expect(queryByText(/loading charts/i)).not.toBeInTheDocument());
    expect(getByText('Existing WBS')).toBeInTheDocument();
  });

  it('renders existing charts and documents with backend-provided kind labels', async () => {
    const { getByText } = await renderLoaded({
      ListCharts: vi.fn(async () => [chart]),
      ListDocuments: vi.fn(async () => [doc]),
    });

    expect(getByText('Existing WBS')).toBeInTheDocument();
    expect(getByText('Work Breakdown Structure')).toBeInTheDocument();
    expect(getByText('Existing Business Case')).toBeInTheDocument();
    expect(getByText('Business Case')).toBeInTheDocument();
    expect(getByText('review')).toBeInTheDocument();
    expect(getByText('v2')).toBeInTheDocument();
  });

  it('shows a retry-able error message when loading fails', async () => {
    let attempt = 0;
    setApp({
      ListCharts: vi.fn(async () => {
        attempt += 1;
        if (attempt === 1) throw new Error('backend unavailable');
        return [chart];
      }),
    });

    const { getByText, getByRole, queryByText } = render(Dashboard);

    await waitFor(() => expect(getByText(/could not load this project/i)).toBeInTheDocument());
    expect(queryByText('Existing WBS')).not.toBeInTheDocument();

    await fireEvent.click(getByRole('button', { name: /retry/i }));

    await waitFor(() => expect(getByText('Existing WBS')).toBeInTheDocument());
    expect(app.ListCharts).toHaveBeenCalledTimes(2);
  });
});

describe('Dashboard chart deletion', () => {
  it('requires an explicit Yes before deleting, and No leaves the chart untouched', async () => {
    const { getByText, getByRole } = await renderLoaded({ ListCharts: vi.fn(async () => [chart]) });
    await waitFor(() => expect(getByText('Existing WBS')).toBeInTheDocument());

    await fireEvent.click(getByRole('button', { name: /^delete existing wbs$/i }));
    expect(getByText('Delete?')).toBeInTheDocument();

    await fireEvent.click(getByRole('button', { name: 'No' }));
    expect(app.DeleteChart).not.toHaveBeenCalled();
    expect(getByText('Existing WBS')).toBeInTheDocument();
  });

  it('deletes the chart and removes it from the list once Yes is confirmed', async () => {
    const { getByText, getByRole, queryByText } = await renderLoaded({ ListCharts: vi.fn(async () => [chart]) });
    await waitFor(() => expect(getByText('Existing WBS')).toBeInTheDocument());

    await fireEvent.click(getByRole('button', { name: /^delete existing wbs$/i }));
    await fireEvent.click(getByRole('button', { name: /^confirm delete existing wbs$/i }));

    await waitFor(() => expect(app.DeleteChart).toHaveBeenCalledWith('chart-1'));
    expect(queryByText('Existing WBS')).not.toBeInTheDocument();
  });

  it('keeps the chart and shows an error toast when the delete call fails', async () => {
    const { getByText, getByRole } = await renderLoaded({
      ListCharts: vi.fn(async () => [chart]),
      DeleteChart: vi.fn(async () => {
        throw new Error('locked by another process');
      }),
    });
    await waitFor(() => expect(getByText('Existing WBS')).toBeInTheDocument());

    await fireEvent.click(getByRole('button', { name: /^delete existing wbs$/i }));
    await fireEvent.click(getByRole('button', { name: /^confirm delete existing wbs$/i }));

    await waitFor(() => expect(getToasts().at(-1)?.type).toBe('error'));
    expect(getToasts().at(-1)?.message).toContain('locked by another process');
    expect(getByText('Existing WBS')).toBeInTheDocument();
  });
});

describe('Dashboard document deletion', () => {
  it('requires confirmation and calls DeleteDocument with the document id', async () => {
    const { getByText, getByRole, queryByText } = await renderLoaded({ ListDocuments: vi.fn(async () => [doc]) });
    await waitFor(() => expect(getByText('Existing Business Case')).toBeInTheDocument());

    await fireEvent.click(getByRole('button', { name: /^delete existing business case$/i }));
    await fireEvent.click(getByRole('button', { name: /^confirm delete existing business case$/i }));

    await waitFor(() => expect(app.DeleteDocument).toHaveBeenCalledWith('doc-1'));
    expect(queryByText('Existing Business Case')).not.toBeInTheDocument();
  });
});

describe('Dashboard chart creation', () => {
  it('creates a chart with its registered starter payload and navigates to its route', async () => {
    const { getByRole } = await renderLoaded();
    await switchTab(getByRole, /^charts$/i);

    await fireEvent.click(getByRole('button', { name: /^RACI Matrix/ }));

    await waitFor(() => expect(app.SaveChart).toHaveBeenCalledOnce());
    const saved = app.SaveChart.mock.calls[0][0];
    expect(saved.kind).toBe('raci');
    expect(JSON.parse(saved.data)).toEqual({ roles: [], tasks: [], assignments: {} });
    expect(session.view).toBe('raci');
    expect(session.editingId).toBe('new-chart-1');
  });

  it('falls back to the generic charts view for a chart kind with no registered route', async () => {
    const { getByRole } = await renderLoaded({
      ListChartKinds: vi.fn(async () => [
        { kind: 'future_kind', name: 'Future Kind', engine: 'dag', description: '', data_example: '{}' },
      ]),
    });
    await switchTab(getByRole, /^charts$/i);

    await fireEvent.click(getByRole('button', { name: /^Future Kind/ }));

    await waitFor(() => expect(app.SaveChart).toHaveBeenCalledOnce());
    expect(session.view).toBe('charts');
  });

  it('shows an error toast and does not navigate when chart creation fails', async () => {
    const { getByRole } = await renderLoaded({
      SaveChart: vi.fn(async () => {
        throw new Error('disk full');
      }),
    });
    await switchTab(getByRole, /^charts$/i);

    await fireEvent.click(getByRole('button', { name: /^RACI Matrix/ }));

    await waitFor(() => expect(getToasts().at(-1)?.message).toContain('disk full'));
    // Confirms we stayed put rather than navigating on a failed create —
    // 'dashboard' is the non-default sentinel `beforeEach` sets, not the
    // ambient default `afterEach` resets to, so this can't pass by accident.
    expect(session.view).toBe('dashboard');
  });
});

describe('Dashboard document creation', () => {
  it('routes charter-kind documents to the charter view', async () => {
    const { getByRole } = await renderLoaded();
    await switchTab(getByRole, /^documents$/i);

    await fireEvent.click(getByRole('button', { name: /^Project Charter/ }));

    await waitFor(() => expect(app.NewDocument).toHaveBeenCalledWith('charter', 'Project Charter'));
    expect(session.view).toBe('charter');
    expect(session.editingId).toBe('new-doc-1');
  });

  it('routes non-charter documents to the generic documents editor', async () => {
    const { getByRole } = await renderLoaded();
    await switchTab(getByRole, /^documents$/i);

    await fireEvent.click(getByRole('button', { name: /^Business Case/ }));

    await waitFor(() => expect(app.NewDocument).toHaveBeenCalledWith('business_case', 'Business Case'));
    expect(session.view).toBe('documents');
  });
});

describe('Dashboard Agile Pack toggle', () => {
  it('enabling shows the workspace links; disabling hides them again', async () => {
    const { getByRole, queryByRole } = await renderLoaded();
    await switchTab(getByRole, /^dev tools$/i);
    expect(queryByRole('button', { name: /^Board Kanban/ })).not.toBeInTheDocument();

    await fireEvent.click(getByRole('button', { name: /enable agile pack/i }));
    await waitFor(() => expect(getByRole('button', { name: /^Board Kanban/ })).toBeInTheDocument());
    expect(app.SetAgileEnabled).toHaveBeenCalledWith(true);
    expect(getByRole('button', { name: /^List Backlog/ })).toBeInTheDocument();
    expect(getByRole('button', { name: /^Iteration Sprints/ })).toBeInTheDocument();
    expect(getByRole('button', { name: /^Metrics DORA Dashboard/ })).toBeInTheDocument();

    await fireEvent.click(getByRole('button', { name: /^disable$/i }));
    await waitFor(() => expect(queryByRole('button', { name: /^Board Kanban/ })).not.toBeInTheDocument());
    expect(app.SetAgileEnabled).toHaveBeenCalledWith(false);
  });

  it('an older binary without the Agile binding degrades to disabled rather than failing', async () => {
    setApp({
      AgileEnabled: vi.fn(async () => {
        throw new Error('method not found');
      }),
    });
    const { getByRole, getByText, queryByRole } = render(Dashboard);
    await switchTab(getByRole, /^dev tools$/i);

    await waitFor(() => expect(getByText(/software-dev pack \(disabled\)/i)).toBeInTheDocument());
    expect(queryByRole('button', { name: /^Board Kanban/ })).not.toBeInTheDocument();
  });

  // Regression test for the race documented in
  // .agent_memory/dashboard-coverage-2026-08-15.md: load()'s own
  // `agileEnabled = await AgileEnabled()` assignment runs after
  // toggleAgile()'s assignment would if the toggle were clicked mid-load,
  // silently reverting it. The fix disables the toggle for the entire
  // window `loading` is true, which this test proves by holding
  // AgileEnabled() pending, confirming the button is inert, then
  // confirming it becomes usable (and only then) once load() settles.
  it('disables the toggle for the whole initial load, closing the toggle-vs-load() race', async () => {
    let resolveAgileEnabled: (v: boolean) => void;
    setApp({
      AgileEnabled: vi.fn(() => new Promise<boolean>((resolve) => (resolveAgileEnabled = resolve))),
    });
    const { getByRole } = render(Dashboard);
    // Tab switching is local state, independent of load() -- safe to click
    // immediately, before load() has resolved.
    await switchTab(getByRole, /^dev tools$/i);

    await waitFor(() => expect(getByRole('button', { name: /enable agile pack/i })).toBeInTheDocument());
    const toggle = getByRole('button', { name: /enable agile pack/i });
    expect(toggle).toBeDisabled();

    // A click while disabled must reach neither the backend nor the
    // clobber-prone assignment — this is what actually closes the race,
    // not just the visual disabled state.
    await fireEvent.click(toggle);
    expect(app.SetAgileEnabled).not.toHaveBeenCalled();

    await waitFor(() => expect(resolveAgileEnabled).toBeDefined());
    resolveAgileEnabled!(false);
    await waitFor(() => expect(getByRole('button', { name: /enable agile pack/i })).not.toBeDisabled());

    await fireEvent.click(getByRole('button', { name: /enable agile pack/i }));
    await waitFor(() => expect(app.SetAgileEnabled).toHaveBeenCalledWith(true));
  });

  it('leaves the toggle enabled (not stuck) when the load fails before reaching AgileEnabled', async () => {
    setApp({
      ListCharts: vi.fn(async () => {
        throw new Error('backend unavailable');
      }),
    });
    const { getByText, getByRole } = render(Dashboard);

    // The load-error message is outside every tab panel (design doc §3.3
    // correction), so it's visible without switching tabs.
    await waitFor(() => expect(getByText(/could not load this project/i)).toBeInTheDocument());
    // The toggle itself lives in the Dev Tools tab panel, so reaching it
    // needs a switch first.
    await switchTab(getByRole, /^dev tools$/i);
    // load()'s outer catch sets loadError and loading=false without ever
    // reaching the AgileEnabled() call — agileEnabled keeps its default
    // (false) and there is no pending assignment left to race, so the
    // toggle must not be left permanently disabled by this path.
    expect(getByRole('button', { name: /enable agile pack/i })).not.toBeDisabled();
    expect(app.AgileEnabled).not.toHaveBeenCalled();
  });
});

describe('Dashboard signed document export', () => {
  // Note for future edits: once the modal is open, its backdrop carries
  // aria-label="Cancel digital signature", which also matches /signature/i.
  // These tests query for the "Signature" trigger button before the modal
  // exists (backdrop absent) and never re-query it by that pattern while
  // the modal is open, so the ambiguity doesn't currently bite — but a new
  // assertion using getByRole('button', { name: /signature/i }) *after*
  // opening the modal would hit a "multiple elements" error.
  async function openSignModal(getByRole: ReturnType<typeof render>['getByRole']) {
    await fireEvent.click(getByRole('button', { name: /signature/i }));
    await waitFor(() => expect(getByRole('dialog')).toBeInTheDocument());
  }

  it('exports without a digital signature and shows a success toast', async () => {
    const { getByRole } = await renderLoaded({ ListDocuments: vi.fn(async () => [doc]) });

    await openSignModal(getByRole);
    await fireEvent.change(getByRole('combobox', { name: /signing method/i }), { target: { value: 'none' } });
    await fireEvent.click(getByRole('button', { name: /^export$/i }));

    await waitFor(() => expect(app.ExportDocumentPDF).toHaveBeenCalledWith('doc-1'));
    // The toast is pushed after the awaited export call resolves inside the
    // component's async IIFE, so the assertion must live inside its own
    // waitFor rather than run synchronously after the call above — otherwise
    // it can race and observe the pre-toast state.
    await waitFor(() => expect(getToasts().at(-1)?.type).toBe('success'));
    expect(getToasts().at(-1)?.message).toContain('without digital signature');
  });

  it('exports with a GnuPG detached signature', async () => {
    const { getByRole } = await renderLoaded({ ListDocuments: vi.fn(async () => [doc]) });

    await openSignModal(getByRole);
    await fireEvent.change(getByRole('combobox', { name: /signing method/i }), { target: { value: 'gpg' } });
    await fireEvent.click(getByRole('button', { name: /^export$/i }));

    await waitFor(() => expect(app.ExportDocumentPDFGnuPG).toHaveBeenCalledWith('doc-1', ''));
    await waitFor(() => expect(getToasts().at(-1)?.message).toContain('GnuPG signature'));
  });

  it('shows an error toast when the signed export fails', async () => {
    const { getByRole } = await renderLoaded({
      ListDocuments: vi.fn(async () => [doc]),
      ExportDocumentPDF: vi.fn(async () => {
        throw new Error('export engine crashed');
      }),
    });

    await openSignModal(getByRole);
    await fireEvent.change(getByRole('combobox', { name: /signing method/i }), { target: { value: 'none' } });
    await fireEvent.click(getByRole('button', { name: /^export$/i }));

    await waitFor(() => expect(getToasts().at(-1)?.type).toBe('error'));
    expect(getToasts().at(-1)?.message).toContain('export engine crashed');
    // The Signature button must return to its idle label, not stay stuck
    // mid-export, so the user can retry.
    expect(getByRole('button', { name: /^signature$/i })).not.toBeDisabled();
  });
});

describe('Dashboard MSPDI import', () => {
  it('imports a schedule as a new CPM chart with the default option set', async () => {
    const { getByRole } = await renderLoaded();
    await switchTab(getByRole, /^charts$/i);

    await fireEvent.click(getByRole('button', { name: /import schedule/i }));

    await waitFor(() =>
      expect(app.ImportMSPDIChartWithOptions).toHaveBeenCalledWith({
        include_dependencies: true,
        include_progress: true,
        include_assignments: true,
      }),
    );
    expect(session.view).toBe('cpm');
    expect(session.editingId).toBe('imported-cpm-1');
  });

  it('silently ignores a user-cancelled import (no error message shown)', async () => {
    const { getByRole, queryByText } = await renderLoaded({
      ImportMSPDIChartWithOptions: vi.fn(async () => {
        throw new Error('cancelled by user');
      }),
    });
    await switchTab(getByRole, /^charts$/i);

    await fireEvent.click(getByRole('button', { name: /import schedule/i }));

    await waitFor(() => expect(app.ImportMSPDIChartWithOptions).toHaveBeenCalledOnce());
    expect(queryByText(/cancelled by user/i)).not.toBeInTheDocument();
  });

  it('surfaces a non-cancellation import failure as a visible message', async () => {
    const { getByRole, getByText } = await renderLoaded({
      ImportMSPDIChartWithOptions: vi.fn(async () => {
        throw new Error('malformed MSPDI XML');
      }),
    });
    await switchTab(getByRole, /^charts$/i);

    await fireEvent.click(getByRole('button', { name: /import schedule/i }));

    await waitFor(() => expect(getByText(/malformed MSPDI XML/i)).toBeInTheDocument());
  });
});

describe('Dashboard project close', () => {
  it('closes the project, clears session state, and returns to the portfolio', async () => {
    const { getByRole } = await renderLoaded();

    await fireEvent.click(getByRole('button', { name: /close project/i }));

    await waitFor(() => expect(app.CloseProject).toHaveBeenCalledOnce());
    expect(session.project).toBeNull();
    expect(session.view).toBe('portfolio');
  });
});

// Dashboard IA restructuring (design doc §3.3/R3/R4): integration coverage
// for Dashboard's own wiring of the shared Tabs component -- which tab is
// default, that each tab reveals its own content and hides the others', and
// that Dev Tools is unconditionally present (not methodology-gated, per the
// design doc's §3.3 correction). Keyboard navigation (arrow keys, Home/End,
// roving tabindex) is Tabs.svelte's own responsibility and is covered once,
// generically, in Tabs.test.ts -- not re-tested here.
describe('Dashboard tabs', () => {
  it('defaults to the Overview tab, with the existing-work lists visible and the other tabs empty', async () => {
    const { getByRole, getByText, queryByRole } = await renderLoaded({
      ListCharts: vi.fn(async () => [chart]),
      ListDocuments: vi.fn(async () => [doc]),
    });

    expect(getByRole('tab', { selected: true })).toHaveTextContent('Overview');
    expect(getByText('Existing WBS')).toBeInTheDocument();
    expect(getByText('Existing Business Case')).toBeInTheDocument();
    // The chart/document catalogs and the Software-Dev Pack section live on
    // other tabs and must not be reachable without switching to them first.
    expect(queryByRole('button', { name: /^RACI Matrix/ })).not.toBeInTheDocument();
    expect(queryByRole('button', { name: /^Project Charter/ })).not.toBeInTheDocument();
    expect(queryByRole('button', { name: /enable agile pack/i })).not.toBeInTheDocument();
  });

  it('switching to Charts reveals the chart catalog and hides the Overview panel', async () => {
    // Default fixture (no existing charts) so ChartCatalog's own
    // initiallyExpanded={charts.length === 0} auto-opens it -- an existing
    // chart would collapse it behind its own "Browse N chart tools" toggle,
    // which is ChartCatalog's own tested behavior, not this test's concern.
    // The same empty fixture makes Overview's own "nothing created yet"
    // copy the thing to check disappears once Overview isn't active.
    const { getByRole, getByText, queryByText } = await renderLoaded();
    expect(getByText(/nothing created yet/i)).toBeInTheDocument();

    await switchTab(getByRole, /^charts$/i);

    expect(getByRole('tab', { selected: true })).toHaveTextContent('Charts');
    expect(getByRole('button', { name: /^RACI Matrix/ })).toBeInTheDocument();
    expect(queryByText(/nothing created yet/i)).not.toBeInTheDocument();
  });

  it('switching to Documents reveals the document catalog', async () => {
    const { getByRole } = await renderLoaded();

    await switchTab(getByRole, /^documents$/i);

    expect(getByRole('tab', { selected: true })).toHaveTextContent('Documents');
    expect(getByRole('button', { name: /^Project Charter/ })).toBeInTheDocument();
  });

  it('Dev Tools is present regardless of project methodology', async () => {
    session.project = { id: 'p1', name: 'Test Project', methodology: 'six_sigma' } as unknown as typeof session.project;
    const { getByRole } = await renderLoaded();

    // Confirms the methodology override above actually took effect (not
    // just that Dev Tools happens to always render) -- the Process
    // Excellence quick-link is the one piece of Dashboard that IS gated on
    // methodology === 'six_sigma', so its presence here is proof the
    // reassigned session.project was read, not stale defaults.
    expect(getByRole('button', { name: /^six sigma dmaic workspace/i })).toBeInTheDocument();

    await switchTab(getByRole, /^dev tools$/i);

    expect(getByRole('tab', { selected: true })).toHaveTextContent('Dev Tools');
    expect(getByRole('heading', { name: /software-dev pack/i })).toBeInTheDocument();
  });

  it('the load-error message renders regardless of the active tab', async () => {
    setApp({
      ListCharts: vi.fn(async () => {
        throw new Error('backend unavailable');
      }),
    });
    const { getByRole, getByText } = render(Dashboard);

    await switchTab(getByRole, /^charts$/i);

    await waitFor(() => expect(getByText(/could not load this project/i)).toBeInTheDocument());
  });

  // Regression test for the ChartCatalog/DocumentCatalog search-state
  // reset flagged as a residual risk when the tab restructuring shipped
  // (design doc §7's "out of scope" list): {#if}-gating the tab panels
  // unmounts ChartCatalog/DocumentCatalog on every tab switch, which would
  // reset their local search/filter/expansion state without the bindable
  // props fix in ChartCatalog.svelte/DocumentCatalog.svelte.
  it('preserves the chart catalog search text across a tab switch away and back', async () => {
    const { getByRole } = await renderLoaded();
    await switchTab(getByRole, /^charts$/i);

    await fireEvent.input(getByRole('searchbox', { name: /search chart tools/i }), {
      target: { value: 'earned value' },
    });
    expect((getByRole('searchbox', { name: /search chart tools/i }) as HTMLInputElement).value).toBe('earned value');

    await switchTab(getByRole, /^overview$/i);
    await switchTab(getByRole, /^charts$/i);

    expect((getByRole('searchbox', { name: /search chart tools/i }) as HTMLInputElement).value).toBe('earned value');
  });
});

// Added 2026-08-19: "Settings" and "Close project" migrated from raw
// <button class="text-xs text-slate-400 hover:text-cyan-400 underline">
// to `Button variant="nav" class="underline"` — the last 2 of the app's
// 3-site "underline" nav-family idiom (see Button.svelte's `nav` branch
// comment), folded into the existing `nav` variant via class passthrough
// rather than a fourth exempt branch. This closes the nav-family idiom
// app-wide; Dashboard.svelte's other ~20 raw <button> elements are
// unrelated (different, un-migrated patterns) and remain out of scope.
describe('Dashboard migrated header "Settings"/"Close project" buttons', () => {
  const expectedClass = 'text-xs text-slate-400 hover:text-cyan-400 disabled:opacity-50 underline'
    .split(/\s+/)
    .sort();

  it('renders "Settings" as Button variant="nav" class="underline"', async () => {
    const { getByText } = await renderLoaded();
    expect(getByText('Settings').className.split(/\s+/).filter(Boolean).sort()).toEqual(expectedClass);
  });

  it('renders "Close project" as Button variant="nav" class="underline"', async () => {
    const { getByText } = await renderLoaded();
    expect(getByText('Close project').className.split(/\s+/).filter(Boolean).sort()).toEqual(expectedClass);
  });
});
