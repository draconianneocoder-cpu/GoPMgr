// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor } from '@testing-library/svelte';

// autosave.register sets up a timer; stub it so the mounted editor doesn't
// leave a live interval running through the test.
vi.mock('../../autosave.svelte', () => ({
  autosave: { register: () => () => {} },
}));

import PERTEditor from './PERTEditor.svelte';
import { session } from '../../session.svelte';

// Added 2026-08-19 to close the residual gap the prior cycle disclosed:
// _layered_editor_shell.test.ts (see that file's row in
// TEST_COVERAGE_LEDGER.md) closed the "two untested consumers" risk Gate C
// flagged by mounting the shared shell through NetworkEditor, but PERTEditor
// itself still had zero *direct* coverage — a shared-pattern inference isn't
// the same as this file's own test suite actually mounting it. This file
// closes that specifically, mirroring _layered_editor_shell.test.ts's
// NetworkEditor-based test exactly, substituting PERTEditor as the mount
// target: same installApp/mountLoaded pattern (GetChart/SaveChart/
// LayoutChart on window.go.main.App are the shell's only 3 backend calls,
// confirmed via grep — PERTEditor itself has no onMount/backend calls of its
// own, only its nodeContent/nodeDetailPanel snippets, which never execute
// with an empty node list on initial mount). Does not cover PERTEditor's own
// O/M/P estimate fields or expected/variance display — those render only
// once a node exists and is selected, out of scope for this pass, whose
// purpose is closing the untested-mount gap, not full PERTEditor coverage.

type AppMock = Record<string, ReturnType<typeof vi.fn>>;

function installApp(overrides: Partial<AppMock> = {}): AppMock {
  const chart = {
    id: 'chart-1',
    project_id: 'p1',
    kind: 'pert',
    title: 'P',
    data: JSON.stringify({ nodes: [], edges: [] }),
  };
  const app: AppMock = {
    GetChart: vi.fn(async () => chart),
    SaveChart: vi.fn(async (c: unknown) => c),
    LayoutChart: vi.fn(async () => ({ body: { layout: { nodes: [], edges: [], width: 0, height: 0 } } })),
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

describe('PERTEditor mounts and renders the shared shell\'s canvas-toolbar buttons', () => {
  it('"Connect…"/"Clear edges": Button variant="canvas" size="sm"; "Delete node": variant="canvas-danger" — all disabled by default with no node selected', async () => {
    const app = installApp();
    const { getByText } = render(PERTEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());

    const canvasExpected = 'rounded text-xs px-3 py-1 bg-slate-800 hover:bg-slate-700 disabled:opacity-30'
      .split(/\s+/)
      .sort();
    for (const label of ['Connect…', 'Clear edges']) {
      const btn = getByText(label) as HTMLButtonElement;
      expect(btn.disabled).toBe(true);
      expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(canvasExpected);
    }

    const deleteNode = getByText('Delete node') as HTMLButtonElement;
    expect(deleteNode.disabled).toBe(true);
    expect(deleteNode.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'rounded text-xs px-3 py-1 bg-slate-800 hover:bg-red-900 disabled:opacity-30'.split(/\s+/).sort(),
    );
  });
});
