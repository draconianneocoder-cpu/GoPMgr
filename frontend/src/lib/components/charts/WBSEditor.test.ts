// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor } from '@testing-library/svelte';

// autosave.register sets up a timer; stub it so the mounted editor doesn't
// leave a live interval running through the test.
vi.mock('../../autosave.svelte', () => ({
  autosave: { register: () => () => {} },
}));

import WBSEditor from './WBSEditor.svelte';
import { session } from '../../session.svelte';

// Added 2026-08-19 alongside migrating this file's "Delete" button to the
// canvas-danger Button variant (the app-wide canvas-toolbar disabled:
// opacity-30 idiom — see Button.test.ts's row in TEST_COVERAGE_LEDGER.md).
// This file had zero prior test coverage. Narrow by design: pins only the
// migrated button's exact class string in its default, no-node-selected
// disabled state (reachable on mount, no canvas interaction needed) — the
// same disposition Gate A required for WorkflowEditor/ActivityEditor's
// canvas/canvas-danger migration. Does not cover tree CRUD, layout
// rendering, or autosave — unchanged by this migration and a pre-existing,
// disclosed gap; "+ Child"/"+ Sibling" are a different, always-enabled
// pattern (no disabled/opacity at all) and were deliberately left as raw
// <button> elements, not part of this migration's named scope.

type AppMock = Record<string, ReturnType<typeof vi.fn>>;

function installApp(overrides: Partial<AppMock> = {}): AppMock {
  const chart = {
    id: 'chart-1',
    project_id: 'p1',
    kind: 'wbs',
    title: 'WBS',
    data: JSON.stringify({ root: { id: 'r', title: 'Project', children: [] } }),
  };
  const app: AppMock = {
    GetChart: vi.fn(async () => chart),
    SaveChart: vi.fn(async (c: unknown) => c),
    LayoutChart: vi.fn(async () => ({ body: { nodes: [], edges: [], width: 0, height: 0 } })),
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

describe('WBSEditor migrated "Delete" button', () => {
  it('Button variant="canvas-danger" size="sm", disabled by default with no node selected', async () => {
    const app = installApp();
    const { getByText } = render(WBSEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());

    const btn = getByText('Delete') as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'rounded text-xs px-3 py-1 bg-slate-800 hover:bg-red-900 disabled:opacity-30'.split(/\s+/).sort(),
    );
  });
});
