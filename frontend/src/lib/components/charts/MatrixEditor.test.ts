// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor } from '@testing-library/svelte';

// autosave.register sets up a timer; stub it so the mounted editor doesn't
// leave a live interval running through the test.
vi.mock('../../autosave.svelte', () => ({
  autosave: { register: () => () => {} },
}));

import MatrixEditor from './MatrixEditor.svelte';
import { session } from '../../session.svelte';

// Added 2026-08-19 alongside migrating this file's header "&larr;
// Dashboard" nav link to the new `Button variant="nav"` (design-critique
// Priority #2 continuation — see `Button.test.ts`'s row for the full
// 28-site/26-file inventory and Gate A scoping). This file had zero prior
// test coverage. Narrow by design: pins only the migrated button's exact
// class string. Unlike every other chart editor in this family,
// `MatrixEditor.svelte`'s own `refreshLayout()` calls only `SaveChart`, not
// `LayoutChart` (confirmed by reading the component, not assumed) — the
// mount-settled gate below waits on `SaveChart`, not `LayoutChart`, or the
// `waitFor` would hang forever waiting for a call this component never
// makes.

type AppMock = Record<string, ReturnType<typeof vi.fn>>;

function installApp(overrides: Partial<AppMock> = {}): AppMock {
  const chart = {
    id: 'chart-1',
    project_id: 'p1',
    kind: 'matrix',
    title: 'Matrix',
    data: JSON.stringify({ title: '', rows_label: '', cols_label: '', rows: [], cols: [], cells: [] }),
  };
  const app: AppMock = {
    GetChart: vi.fn(async () => chart),
    SaveChart: vi.fn(async (c: unknown) => c),
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

describe('MatrixEditor migrated header "&larr; Dashboard" button', () => {
  it('Button variant="nav"', async () => {
    const app = installApp();
    const { getByText } = render(MatrixEditor);
    await waitFor(() => expect(app.SaveChart).toHaveBeenCalled());

    const btn = getByText('← Dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'text-xs text-slate-400 hover:text-cyan-400 disabled:opacity-50'.split(/\s+/).sort(),
    );
  });
});
