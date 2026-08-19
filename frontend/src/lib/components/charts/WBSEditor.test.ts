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
// disclosed gap.
//
// 2026-08-19 (second pass, same date): "+ Child"/"+ Sibling"/"Save" also
// migrated (secondary/sm and primary/sm respectively — the always-enabled
// "+ Child"/"+ Sibling" pair was deliberately excluded from the first pass
// above as a different, unreviewed pattern; that review has now happened).
//
// 2026-08-19 (third pass, same date): the load-error "Back to dashboard"
// button also migrated (primary/md — the same pattern already used by
// WorkflowEditor/ActivityEditor's equivalent buttons).
//
// 2026-08-19 (fourth pass, same date): the header "&larr; Dashboard" nav
// link also migrated, to the new `nav` Button variant (28 grep-confirmed
// app-wide call sites of this exact idiom before this cycle's migration —
// see Button.svelte's `nav` branch comment).

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

describe('WBSEditor migrated "+ Child"/"+ Sibling"/"Save" buttons', () => {
  it('"+ Child"/"+ Sibling": Button variant="secondary" size="sm"; "Save": variant="primary" size="sm"', async () => {
    const app = installApp();
    const { getByText } = render(WBSEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());

    const secondaryExpected = 'rounded text-xs px-3 py-1 bg-slate-800 hover:bg-slate-700 disabled:opacity-50'
      .split(/\s+/)
      .sort();
    for (const label of ['+ Child', '+ Sibling']) {
      expect(getByText(label).className.split(/\s+/).filter(Boolean).sort()).toEqual(secondaryExpected);
    }

    const save = getByText('Save') as HTMLButtonElement;
    expect(save.disabled).toBe(false);
    expect(save.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'rounded text-xs disabled:opacity-50 px-3 py-1 bg-cyan-600 hover:bg-cyan-500 text-white font-bold uppercase'
        .split(/\s+/)
        .sort(),
    );
  });
});

describe('WBSEditor migrated load-error "Back to dashboard" button', () => {
  it('Button variant="primary" size="md"', async () => {
    installApp({ GetChart: vi.fn(async () => { throw new Error('boom'); }) });
    const { findByText } = render(WBSEditor);

    const btn = await findByText('Back to dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'rounded text-xs disabled:opacity-50 px-3 py-2 bg-cyan-600 hover:bg-cyan-500 text-white font-bold uppercase'
        .split(/\s+/)
        .sort(),
    );
  });
});

// Added 2026-08-19 (fourth pass, same date): the header "&larr; Dashboard"
// nav link — a distinct button from the load-error one above — migrated to
// the new `nav` Button variant.
describe('WBSEditor migrated header "&larr; Dashboard" button', () => {
  it('Button variant="nav"', async () => {
    const app = installApp();
    const { getByText } = render(WBSEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());

    const btn = getByText('← Dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'text-xs text-slate-400 hover:text-cyan-400 disabled:opacity-50'.split(/\s+/).sort(),
    );
  });
});
