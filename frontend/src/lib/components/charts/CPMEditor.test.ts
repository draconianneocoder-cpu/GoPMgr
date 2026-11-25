// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';

// The real auto-save module starts a timer. This stub keeps its rule:
// a dirty document is saved, and an unchanged document is not.
const autosaveMock = vi.hoisted(() => {
  let persist: () => Promise<boolean> = async () => true;
  let read: () => string = () => '';
  let last = '';
  return {
    register: (snapshot: () => string, save: () => Promise<boolean>) => {
      read = snapshot;
      persist = save;
      last = snapshot();
      return () => {};
    },
    saveRegisteredIfDirty: async () => {
      if (read() === last) return true;
      const ok = await persist();
      if (ok) last = read();
      return ok;
    },
    saveRegisteredNow: async () => {
      const ok = await persist();
      if (ok) last = read();
      return ok;
    },
    lastError: '',
  };
});
vi.mock('../../autosave.svelte', () => ({ autosave: autosaveMock }));

import CPMEditor from './CPMEditor.svelte';
import { session } from '../../session.svelte';

const desktopApiMock = vi.hoisted(() => {
  const state: { current: Record<string, ReturnType<typeof vi.fn>> } = { current: {} };
  return state;
});
vi.mock('../../desktop-api', () => ({ get desktopApi() { return desktopApiMock.current; } }));

// Narrow, targeted coverage added alongside the 2026-08-19 Button migration
// (see TEST_COVERAGE_LEDGER.md): this file previously had zero test
// coverage. Pins the exact rendered class string of the toolbarExtra
// buttons (all `Button variant="secondary" size="sm"`) migrated from raw
// <button> elements.
//
// 2026-08-25: the "Remove assignment" button (variant="remove", class=
// "px-1 text-xs") now has its own reachability test below — see that
// describe block for why it was previously excluded (disproportionate
// canvas-selection setup) and how the shell's own `+ Node` button turned
// out to be a much shorter path in than the LayeredDiagram canvas.

type AppMock = Record<string, ReturnType<typeof vi.fn>>;

function installApp(overrides: Partial<AppMock> = {}): AppMock {
  const chart = {
    id: 'chart-1',
    project_id: 'p1',
    kind: 'cpm',
    title: 'C',
    data: JSON.stringify({ nodes: [], edges: [] }),
  };
  const app: AppMock = {
    GetChart: vi.fn(async () => chart),
    SaveChart: vi.fn(async (c: unknown) => c),
    LayoutChart: vi.fn(async () => ({ body: { layout: { nodes: [], edges: [], width: 0, height: 0 } } })),
    ListScheduleBaselines: vi.fn(async () => []),
    CompareScheduleBaseline: vi.fn(async () => ({})),
    ListStakeholders: vi.fn(async () => []),
    ...overrides,
  };
  desktopApiMock.current = app;
  return app;
}

beforeEach(() => {
  session.editingId = 'chart-1';
});

afterEach(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
});

describe('CPMEditor toolbarExtra migrated buttons', () => {
  it('renders "Level"/"Preview splitting"/"Histogram"/"Set baseline" as Button variant="secondary" size="sm"', async () => {
    const app = installApp();
    const { getByText } = render(CPMEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());
    await waitFor(() => expect(app.ListScheduleBaselines).toHaveBeenCalled());

    const expected = 'rounded text-xs disabled:opacity-50 px-3 py-1 bg-slate-800 hover:bg-slate-700'
      .split(/\s+/)
      .sort();
    for (const label of ['Level', 'Preview splitting', 'Histogram', 'Set baseline']) {
      expect(getByText(label).className.split(/\s+/).filter(Boolean).sort()).toEqual(expected);
    }
  });

  // Added 2026-08-19: these two were previously excluded from the initial
  // migration pass ("py-1.5" had no matching Button size), now closed by
  // adding the `compact` size bucket (px-3 py-1.5, confirmed by grep as a
  // real, app-wide-reused 4th padding bucket, not invented for this pair).
  // "Export PDF/A" (the third py-1.5 button in this file) was left
  // unmigrated at the time — its original padding is px-2.5, not px-3, and
  // relying on a `class="px-2.5"` override to win over the `compact`
  // variant's own `px-3` is not safely assumable (Tailwind resolves
  // same-property utility conflicts by the utilities' fixed position in
  // the generated stylesheet, not by source/class-list order — documented
  // precedent elsewhere in this ledger's Input/Select entries). Migrated
  // in a later pass instead of overridden — see the describe block below.
  it('renders "Compute" and "Run" as Button variant="secondary" size="compact"', async () => {
    const app = installApp();
    const { getByText } = render(CPMEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());

    const expected = 'rounded text-xs disabled:opacity-50 px-3 py-1.5 bg-slate-800 hover:bg-slate-700'
      .split(/\s+/)
      .sort();
    for (const label of ['Compute', 'Run']) {
      expect(getByText(label).className.split(/\s+/).filter(Boolean).sort()).toEqual(expected);
    }
  });
});

const simResult: SimResult = {
  valid: true,
  iterations: 1000,
  workers: 1,
  p50: 10,
  p80: 12,
  p90: 14,
  finish_cdf: [],
  critical_path_frequency: {},
  duration_percentiles: {},
  tornado_drivers: [],
};

// Added 2026-08-19 (design-critique Priority #2, Option B from the
// system-design consult on this button's padding mismatch — see
// docs/beta-release-backlog.md): "Export PDF/A" migrated from a raw
// <button class="rounded bg-slate-800 px-2.5 py-1.5 ...">, whose px-2.5
// padding had no matching Button size, to `variant="secondary"
// size="compact"` (px-3 py-1.5 — 2px wider). A real, small, deliberate
// visual change, not a pure refactor — accepted explicitly as the
// trade-off of closing this file's last raw-button exclusion, per the
// decision made in that consult. The button is gated behind a completed
// Monte Carlo run (`{#if monteCarlo}`), so reaching it requires clicking
// "Run" first and letting RunChartMonteCarlo resolve.
describe('CPMEditor migrated "Export PDF/A" button', () => {
  it('Button variant="secondary" size="compact"', async () => {
    const app = installApp({ RunChartMonteCarlo: vi.fn(async () => simResult) });
    const { getByText, findByText } = render(CPMEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());

    await fireEvent.click(getByText('Run'));
    const btn = await findByText('Export PDF/A');

    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'rounded text-xs disabled:opacity-50 px-3 py-1.5 bg-slate-800 hover:bg-slate-700'.split(/\s+/).sort(),
    );
  });
});

// Added 2026-08-25: closes the reachability gap the file header above used
// to disclose. The shell's `+ Node` button (`_layered_editor_shell.svelte`)
// sets `selectedId` directly on click — no LayeredDiagram canvas
// interaction needed — which renders CPMEditor's own `nodeDetailPanel`
// snippet immediately, including this button. Also regression-covers the
// 2026-08-25 accessibility fix: this button previously had `title` but no
// `aria-label`, so its computed accessible name came from its own "✕" text
// content rather than the title (title is not used as the accessible name
// when the element has non-empty text content) — a real gap, unlike its
// "Delete edge" siblings in ActivityEditor/WorkflowEditor which always had
// both. `getByRole(..., { name: 'Remove assignment' })` below is
// load-bearing: it fails against the pre-fix markup.
describe('CPMEditor "Remove assignment" button (variant="remove")', () => {
  it('reachable via + Node -> + Assign resource; correct accessible name, class, and removes its row on click', async () => {
    const app = installApp();
    const { getByRole, findByText, queryByRole } = render(CPMEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());

    await fireEvent.click(getByRole('button', { name: '+ Node' }));
    const assignBtn = await findByText('+ Assign resource');
    await fireEvent.click(assignBtn);

    const removeBtn = await findByText('✕');
    expect(removeBtn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'text-slate-500 hover:text-red-400 disabled:opacity-50 px-1 text-xs'.split(/\s+/).sort(),
    );
    // Accessible name comes from aria-label, not the "✕" text content.
    expect(getByRole('button', { name: 'Remove assignment' })).toBe(removeBtn);

    await fireEvent.click(removeBtn);
    expect(queryByRole('button', { name: 'Remove assignment' })).toBeNull();
  });
});

// KRN-01 (2026-11-18): the editor offers all eight constraint types, shows
// the date field for the six dated ones, and reads a lower-case stored
// value ("mso" from hand-edited data) without rewriting it.
describe('CPMEditor scheduling constraints', () => {
  it('shows a stored lower-case MSO as Must start on, keeps the stored value, and shows the date only for dated types', async () => {
    const node = { id: 'n1', label: 'Pour', duration: 2, constraint: 'mso', constraint_date: '2026-06-04' };
    const app = installApp({
      GetChart: vi.fn(async () => ({ id: 'chart-1', project_id: 'p1', kind: 'cpm', title: 'C', data: JSON.stringify({ nodes: [node], edges: [] }) })),
      LayoutChart: vi.fn(async () => ({ body: { layout: { nodes: [{ id: 'n1', title: 'Pour', depth: 0, x: 0, y: 0, width: 120, height: 40 }], edges: [], width: 200, height: 80 } } })),
    });
    const { getByRole, getByLabelText, queryByLabelText } = render(CPMEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());
    await fireEvent.click(getByRole('button', { name: 'Pour' }));

    const select = (await waitFor(() => getByLabelText('Constraint'))) as HTMLSelectElement;
    expect(select.selectedOptions[0].textContent).toBe('Must start on');
    expect(Array.from(select.options, (o) => o.textContent)).toEqual([
      'ASAP (default)', 'As late as possible', 'Start no earlier than', 'Start no later than',
      'Finish no earlier than', 'Finish no later than', 'Must start on', 'Must finish on',
    ]);
    expect(queryByLabelText('Constraint date')).not.toBeNull();
    await new Promise((r) => setTimeout(r, 400));
    expect(app.SaveChart).not.toHaveBeenCalled();

    await fireEvent.change(select, { target: { value: 'FNET' } });
    await fireEvent.change(select, { target: { value: 'SNLT' } });
    expect(queryByLabelText('Constraint date')).not.toBeNull();
    await fireEvent.change(select, { target: { value: 'ALAP' } });
    await waitFor(() => expect(queryByLabelText('Constraint date')).toBeNull());
    await fireEvent.click(getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(app.SaveChart).toHaveBeenCalled());
    const saved = app.SaveChart.mock.calls.map(([chart]) => String((chart as { data: string }).data));
    expect(saved.every((data) => data.includes('"constraint":"ALAP"'))).toBe(true);
    expect(saved.every((data) => !data.includes('"constraint":"mso"'))).toBe(true);
  });

  // Selecting a task must not write the chart. The detail panel's milestone
  // checkbox used bind:checked, which writes `false` into a task stored
  // without `milestone`, so one click saved the chart twice (adding the field,
  // then removing it when layout returned the task without it).
  it('selects a task stored without `milestone` without writing the chart', async () => {
    vi.useFakeTimers();
    const node = { id: 'n1', label: 'Pour', duration: 2 };
    const app = installApp({
      GetChart: vi.fn(async () => ({ id: 'chart-1', project_id: 'p1', kind: 'cpm', title: 'C', data: JSON.stringify({ nodes: [node], edges: [] }) })),
      LayoutChart: vi.fn(async () => ({ body: { layout: { nodes: [{ id: 'n1', title: 'Pour', depth: 0, x: 0, y: 0, width: 120, height: 40 }], edges: [], width: 200, height: 80 } } })),
    });
    const { getByRole, getByLabelText } = render(CPMEditor);
    await vi.advanceTimersByTimeAsync(0);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());
    await fireEvent.click(getByRole('button', { name: 'Pour' }));
    await waitFor(() => getByLabelText('Constraint'));
    await vi.advanceTimersByTimeAsync(400);
    expect(app.SaveChart).not.toHaveBeenCalled();
  });

  it('opens a task stored as "ASAP" without rewriting it', async () => {
    const node = { id: 'n1', label: 'Pour', duration: 2, constraint: 'ASAP' };
    const app = installApp({
      GetChart: vi.fn(async () => ({ id: 'chart-1', project_id: 'p1', kind: 'cpm', title: 'C', data: JSON.stringify({ nodes: [node], edges: [] }) })),
      LayoutChart: vi.fn(async () => ({ body: { layout: { nodes: [{ id: 'n1', title: 'Pour', depth: 0, x: 0, y: 0, width: 120, height: 40 }], edges: [], width: 200, height: 80 } } })),
    });
    const { getByRole, getByLabelText } = render(CPMEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());
    await fireEvent.click(getByRole('button', { name: 'Pour' }));
    const select = (await waitFor(() => getByLabelText('Constraint'))) as HTMLSelectElement;
    expect(select.selectedOptions[0].textContent).toBe('ASAP (default)');
    await new Promise((r) => setTimeout(r, 400));
    expect(app.SaveChart).not.toHaveBeenCalled();

    await fireEvent.change(select, { target: { value: 'MSO' } });
    await fireEvent.click(getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(app.SaveChart).toHaveBeenCalled());
    const saved = app.SaveChart.mock.calls.map(([chart]) => String((chart as { data: string }).data));
    expect(saved.every((data) => data.includes('"constraint":"MSO"'))).toBe(true);
    expect(saved.every((data) => !data.includes('"constraint":"ASAP"'))).toBe(true);
  });

  // KRN-01 b: a date constraint the scheduler could not apply is shown with
  // its reason (from the backend's annotated document), not left silent.
  it('says why a constraint is not applied', async () => {
    const node = { id: 'n1', label: 'Pour', duration: 2, constraint: 'SNLT' };
    const app = installApp({
      GetChart: vi.fn(async () => ({ id: 'chart-1', project_id: 'p1', kind: 'cpm', title: 'C', data: JSON.stringify({ nodes: [node], edges: [] }) })),
      LayoutChart: vi.fn(async () => ({
        body: {
          layout: { nodes: [{ id: 'n1', title: 'Pour', depth: 0, x: 0, y: 0, width: 120, height: 40 }], edges: [], width: 200, height: 80 },
          doc: { nodes: [{ ...node, constraint_unapplied: 'invalid_date' }], edges: [] },
        },
      })),
    });
    const { getByRole, findByText } = render(CPMEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());
    await fireEvent.click(getByRole('button', { name: 'Pour' }));
    const note = await findByText(
      'Constraint not applied: the constraint date is missing or is not a valid date. Until you correct it, the task is scheduled as if it had no constraint.',
    );
    expect(note.getAttribute('role')).toBe('status');
  });
});
