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
// regression. The "Delete edge" (variant="remove") button is not covered
// here — reaching it requires selecting a node with a connected edge,
// which is disproportionate setup for this pass; its class was instead
// verified by reading Button.svelte's `remove` branch directly (interpolates
// `{klass}`, spreads `{...rest}`), and it takes no class/title override here
// so there is nothing bespoke to regress.

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
    expect(btn.className.trim()).toBe(
      'rounded text-xs disabled:opacity-50 px-3 py-2 bg-cyan-600 hover:bg-cyan-500 text-white font-bold uppercase',
    );
  });

  it('"Save": Button variant="primary" size="sm"', async () => {
    const app = installApp();
    const { getByText } = render(WorkflowEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());

    const btn = getByText('Save');
    expect(btn.className.trim()).toBe(
      'rounded text-xs disabled:opacity-50 px-3 py-1 bg-cyan-600 hover:bg-cyan-500 text-white font-bold uppercase',
    );

    await fireEvent.click(btn);
    await waitFor(() => expect(app.SaveChart).toHaveBeenCalled());
  });
});
