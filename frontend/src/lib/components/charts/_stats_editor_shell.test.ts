// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor } from '@testing-library/svelte';

// autosave.register sets up a timer; stub it so the mounted editor doesn't
// leave a live interval running through the test.
vi.mock('../../autosave.svelte', () => ({
  autosave: { register: () => () => {} },
}));

import PieEditor from './PieEditor.svelte';
import { session } from '../../session.svelte';

// Added 2026-08-19 alongside migrating this shell's header "&larr;
// Dashboard" nav link to the new `Button variant="nav"` (design-critique
// Priority #2 continuation — see `Button.test.ts`'s row for the full
// 28-site/26-file inventory and Gate A scoping). This shared file (`import
// StatsEditorShell from './_stats_editor_shell.svelte'`, consumed by
// BarEditor, BurnDownEditor, BurnUpEditor, ControlChartEditor,
// CumulativeFlowEditor, LineEditor, ParetoEditor, and PieEditor) had zero
// prior test coverage — and unlike `_layered_editor_shell.svelte` (which
// had `CPMEditor.test.ts` covering one consumer before this same migration
// reached it), *none* of this shell's 8 consumers have ever had any test
// coverage at all. Mounted through `PieEditor` — the thinnest of the 8 (72
// lines, no own `onMount`/`$state` beyond a local doc object, no side
// effects) — so this is coverage of the shared shell reached through the
// thinnest available door, not `PieEditor`-specific coverage; the shell
// owns this header markup identically regardless of which chart kind wraps
// it. `LayoutChart`'s mock resolves `{ body: null }` deliberately: the
// shell's own template only renders `<StatsChart>` `{#if layout}`, and a
// null layout keeps this test from exercising `StatsChart`'s Chart.js/
// canvas rendering path (a separate, unrelated component this migration
// does not touch) — jsdom has no real `<canvas>` implementation, so
// forcing that path here would only add noise, not coverage. Narrow by
// design: pins only the migrated button's exact class string, reachable on
// mount once `GetChart`/`SaveChart`/`LayoutChart` resolve — does not cover
// slice CRUD, chart rendering, or autosave.
//
// 2026-08-19 (second pass, same date): the load-error "Back to dashboard"
// button also migrated (variant="primary" size="md" — the same pattern
// already used by WorkflowEditor/ActivityEditor/WBSEditor/CauseEffectEditor's
// equivalent buttons), also mounted via `PieEditor` as the shared shell's
// thinnest door.

type AppMock = Record<string, ReturnType<typeof vi.fn>>;

function installApp(overrides: Partial<AppMock> = {}): AppMock {
  const chart = {
    id: 'chart-1',
    project_id: 'p1',
    kind: 'pie',
    title: 'Pie',
    data: JSON.stringify({ title: '', slices: [] }),
  };
  const app: AppMock = {
    GetChart: vi.fn(async () => chart),
    SaveChart: vi.fn(async (c: unknown) => c),
    LayoutChart: vi.fn(async () => ({ body: null })),
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

describe('_stats_editor_shell migrated header "&larr; Dashboard" button (via PieEditor)', () => {
  it('Button variant="nav"', async () => {
    const app = installApp();
    const { getByText } = render(PieEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());

    const btn = getByText('← Dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'text-xs text-slate-400 hover:text-cyan-400 disabled:opacity-50'.split(/\s+/).sort(),
    );
  });
});

describe('_stats_editor_shell migrated load-error "Back to dashboard" button (via PieEditor)', () => {
  it('Button variant="primary" size="md"', async () => {
    installApp({ GetChart: vi.fn(async () => { throw new Error('boom'); }) });
    const { findByText } = render(PieEditor);

    const btn = await findByText('Back to dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'rounded text-xs disabled:opacity-50 px-3 py-2 bg-cyan-600 hover:bg-cyan-500 text-white font-bold uppercase'
        .split(/\s+/)
        .sort(),
    );
  });
});
