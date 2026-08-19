// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';

// autosave.register sets up a timer; stub it so the mounted editor doesn't
// leave a live interval running through the test.
vi.mock('../../autosave.svelte', () => ({
  autosave: { register: () => () => {} },
}));

import ActivityEditor from './ActivityEditor.svelte';
import { session } from '../../session.svelte';

// Narrow, targeted coverage added alongside the 2026-08-19 Button migration
// (see TEST_COVERAGE_LEDGER.md): this file previously had zero test
// coverage. Pins the exact rendered class string of each button migrated
// from a raw <button> to the shared Button component. The "Delete edge"
// (variant="remove") button is not covered here for the same reason as
// WorkflowEditor's — reaching it needs a selected node with a connected
// edge — and takes no class/title override, so nothing bespoke to regress.

type AppMock = Record<string, ReturnType<typeof vi.fn>>;

function installApp(overrides: Partial<AppMock> = {}): AppMock {
  const chart = {
    id: 'chart-1',
    project_id: 'p1',
    kind: 'activity',
    title: 'A',
    data: JSON.stringify({ swimlanes: [], nodes: [], edges: [] }),
  };
  const app: AppMock = {
    GetChart: vi.fn(async () => chart),
    SaveChart: vi.fn(async (c: unknown) => c),
    LayoutChart: vi.fn(async () => ({
      body: { nodes: [], edges: [], swimlanes: [], width: 0, height: 0 },
    })),
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

describe('ActivityEditor migrated buttons', () => {
  it('load-error "Back to dashboard": Button variant="primary" size="md"', async () => {
    installApp({ GetChart: vi.fn(async () => { throw new Error('boom'); }) });
    const { findByText } = render(ActivityEditor);

    const btn = await findByText('Back to dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'rounded text-xs disabled:opacity-50 px-3 py-2 bg-cyan-600 hover:bg-cyan-500 text-white font-bold uppercase'
        .split(/\s+/)
        .sort(),
    );
  });

  it('"+ Swimlane": Button variant="secondary" size="sm"', async () => {
    const app = installApp();
    const { getByRole } = render(ActivityEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());

    // getByText would also match the empty-state hint text ("...adding one
    // or more <strong>+ Swimlane</strong>s..."); the button is the only
    // element with this exact accessible name.
    const btn = getByRole('button', { name: '+ Swimlane' });
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'rounded text-xs disabled:opacity-50 px-3 py-1 bg-slate-800 hover:bg-slate-700'.split(/\s+/).sort(),
    );

    await fireEvent.click(btn);
    expect(app.SaveChart).toHaveBeenCalled();
  });

  it('"Save": Button variant="primary" size="sm"', async () => {
    const app = installApp();
    const { getByText } = render(ActivityEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());

    const btn = getByText('Save');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'rounded text-xs disabled:opacity-50 px-3 py-1 bg-cyan-600 hover:bg-cyan-500 text-white font-bold uppercase'
        .split(/\s+/)
        .sort(),
    );
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
    const { getByText } = render(ActivityEditor);
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
});
