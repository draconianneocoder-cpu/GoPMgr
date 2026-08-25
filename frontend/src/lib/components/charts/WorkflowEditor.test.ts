// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';

// autosave.register sets up a timer; stub it so the mounted editor doesn't
// leave a live interval running through the test.
vi.mock('../../autosave.svelte', () => ({
  autosave: { register: () => () => {} },
}));

import WorkflowEditor from './WorkflowEditor.svelte';
import { session } from '../../session.svelte';

// Narrow, targeted coverage added alongside the 2026-08-19 Button migration
// (see TEST_COVERAGE_LEDGER.md): this file previously had zero test
// coverage. Rather than backfill the whole editor, this pins the exact
// rendered class string of each button migrated from a raw <button> to the
// shared Button component, so a silent divergence (missing text-xs, wrong
// opacity, etc.) fails a test instead of only showing up as a visual
// regression.
//
// 2026-08-25: the "Delete edge" (variant="remove") button now has its own
// reachability test below, closing the gap this comment used to disclose —
// see that describe block.

type AppMock = Record<string, ReturnType<typeof vi.fn>>;

function installApp(overrides: Partial<AppMock> = {}): AppMock {
  const chart = {
    id: 'chart-1',
    project_id: 'p1',
    kind: 'workflow',
    title: 'W',
    data: JSON.stringify({ nodes: [], edges: [] }),
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

describe('WorkflowEditor migrated buttons', () => {
  it('load-error "Back to dashboard": Button variant="primary" size="md"', async () => {
    installApp({ GetChart: vi.fn(async () => { throw new Error('boom'); }) });
    const { findByText } = render(WorkflowEditor);

    const btn = await findByText('Back to dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'rounded text-xs disabled:opacity-50 px-3 py-2 bg-cyan-600 hover:bg-cyan-500 text-white font-bold uppercase'
        .split(/\s+/)
        .sort(),
    );
  });

  it('"Save": Button variant="primary" size="sm"', async () => {
    const app = installApp();
    const { getByText } = render(WorkflowEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());

    const btn = getByText('Save');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'rounded text-xs disabled:opacity-50 px-3 py-1 bg-cyan-600 hover:bg-cyan-500 text-white font-bold uppercase'
        .split(/\s+/)
        .sort(),
    );

    await fireEvent.click(btn);
    await waitFor(() => expect(app.SaveChart).toHaveBeenCalled());
  });

  // Added 2026-08-19: these two were previously excluded from the initial
  // migration pass (Button had no way to express the app-wide
  // canvas-toolbar disabled:opacity-30 idiom), now closed by adding the
  // `canvas`/`canvas-danger` variants. Pinned in their default,
  // no-node-selected state (disabled — reachable on mount without any
  // canvas interaction), per Gate A: an enabled-state-only assertion
  // cannot prove the migration preserved the opacity-30, not opacity-50,
  // dimming these two buttons actually rely on.
  it('"Connect…"/"Delete node": Button variant="canvas"/"canvas-danger" size="sm", disabled by default with no node selected', async () => {
    const app = installApp();
    const { getByText } = render(WorkflowEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());

    const connect = getByText('Connect…') as HTMLButtonElement;
    expect(connect.disabled).toBe(true);
    expect(connect.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'rounded text-xs px-3 py-1 bg-slate-800 hover:bg-slate-700 disabled:opacity-30'.split(/\s+/).sort(),
    );

    const deleteNode = getByText('Delete node') as HTMLButtonElement;
    expect(deleteNode.disabled).toBe(true);
    expect(deleteNode.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'rounded text-xs px-3 py-1 bg-slate-800 hover:bg-red-900 disabled:opacity-30'.split(/\s+/).sort(),
    );
  });

  // Added 2026-08-19 (second pass, same date): the header "&larr; Dashboard"
  // nav link, previously a raw <button>, migrated to the new `nav` Button
  // variant.
  it('"&larr; Dashboard": Button variant="nav"', async () => {
    const app = installApp();
    const { getByText } = render(WorkflowEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());

    const btn = getByText('← Dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'text-xs text-slate-400 hover:text-cyan-400 disabled:opacity-50'.split(/\s+/).sort(),
    );
  });
});

// Added 2026-08-25: closes the reachability gap the file header above used
// to disclose. Same technique as ActivityEditor's equivalent test: the
// chart is seeded pre-loaded with two nodes and one edge (via a custom
// GetChart/LayoutChart mock), so clicking the first node's canvas element
// selects it directly and renders the "Connected edges" panel immediately —
// no "+ Node"/"Connect…" canvas flow needed.
describe('WorkflowEditor "Delete edge" button (variant="remove")', () => {
  it('reachable by selecting a node with a connected edge; correct accessible name/class, removes the edge on click', async () => {
    const chart = {
      id: 'chart-1',
      project_id: 'p1',
      kind: 'workflow',
      title: 'W',
      data: JSON.stringify({
        nodes: [
          { id: 'n1', label: 'Start', shape: 'start' },
          { id: 'n2', label: 'End', shape: 'end' },
        ],
        edges: [{ from: 'n1', to: 'n2' }],
      }),
    };
    const layoutNode = (id: string, label: string, x: number) => ({
      id, label, shape: 'start', rank: 0, x, y: 10, width: 80, height: 40,
    });
    const app = installApp({
      GetChart: vi.fn(async () => chart),
      LayoutChart: vi.fn(async () => ({
        body: {
          nodes: [layoutNode('n1', 'Start', 10), layoutNode('n2', 'End', 200)],
          edges: [{ from: 'n1', to: 'n2' }],
          width: 300,
          height: 100,
        },
      })),
    });
    const { getByRole, findByRole, queryByRole } = render(WorkflowEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());

    await fireEvent.click(getByRole('button', { name: 'Start' }));

    const deleteEdgeBtn = await findByRole('button', { name: 'Delete edge' });
    expect(deleteEdgeBtn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'text-slate-500 hover:text-red-400 disabled:opacity-50'.split(/\s+/).sort(),
    );

    await fireEvent.click(deleteEdgeBtn);
    expect(queryByRole('button', { name: 'Delete edge' })).toBeNull();
  });
});
