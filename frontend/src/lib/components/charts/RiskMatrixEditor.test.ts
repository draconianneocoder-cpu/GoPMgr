// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor } from '@testing-library/svelte';

// autosave.register sets up a timer; stub it so the mounted editor doesn't
// leave a live interval running through the test.
vi.mock('../../autosave.svelte', () => ({
  autosave: { register: () => () => {} },
}));

import RiskMatrixEditor from './RiskMatrixEditor.svelte';
import { session } from '../../session.svelte';

// Added 2026-08-19 alongside migrating this file's header "&larr;
// Dashboard" nav link to the new `Button variant="nav"` (design-critique
// Priority #2 continuation — see `Button.test.ts`'s row for the full
// 28-site/26-file inventory and Gate A scoping). This file had zero prior
// test coverage. Narrow by design: pins only the migrated button's exact
// class string, reachable on mount once GetChart/LayoutChart resolve — does
// not cover risk/issue/opportunity item CRUD, layout rendering, or
// autosave.

type AppMock = Record<string, ReturnType<typeof vi.fn>>;

function installApp(overrides: Partial<AppMock> = {}): AppMock {
  const chart = {
    id: 'chart-1',
    project_id: 'p1',
    kind: 'risk_matrix',
    title: 'Risk Matrix',
    data: JSON.stringify({ items: [] }),
  };
  const app: AppMock = {
    GetChart: vi.fn(async () => chart),
    SaveChart: vi.fn(async (c: unknown) => c),
    LayoutChart: vi.fn(async () => ({ body: { cells: [], validation: { error_count: 0 } } })),
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

describe('RiskMatrixEditor migrated header "&larr; Dashboard" button', () => {
  it('Button variant="nav"', async () => {
    const app = installApp();
    const { getByText } = render(RiskMatrixEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());

    const btn = getByText('← Dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'text-xs text-slate-400 hover:text-cyan-400 disabled:opacity-50'.split(/\s+/).sort(),
    );
  });
});
