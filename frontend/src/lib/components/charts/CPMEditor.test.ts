// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor } from '@testing-library/svelte';

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
// <button> elements. The "Remove assignment" button (variant="remove",
// class="px-1 text-xs" — the one place a real bug was caught and fixed
// during this migration: the original omitted "text-xs", which the
// `remove` variant does not bake in) is not covered here — reaching it
// needs a selected node with an assignment row added first, which goes
// through LayeredDiagram's canvas selection and is disproportionate setup
// for this pass. Its class was instead verified by reading Button.svelte's
// `remove` branch directly (interpolates `{klass}` verbatim, spreads
// `{...rest}` so `title` also passes through) — see TEST_COVERAGE_LEDGER.md
// for the full disposition.

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

    const expected = 'rounded text-xs disabled:opacity-50 px-3 py-1 bg-slate-800 hover:bg-slate-700';
    for (const label of ['Level', 'Preview splitting', 'Histogram', 'Set baseline']) {
      expect(getByText(label).className.trim()).toBe(expected);
    }
  });
});
