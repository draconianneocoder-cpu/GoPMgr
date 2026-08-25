// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';

// autosave.register sets up a timer; stub it so the mounted editor doesn't
// leave a live interval running through the test.
vi.mock('../../autosave.svelte', () => ({
  autosave: { register: () => () => {} },
}));

import CPMEditor from './CPMEditor.svelte';
import { session } from '../../session.svelte';

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
  (window as unknown as { go: unknown }).go = { main: { App: app } };
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
