// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor } from '@testing-library/svelte';

// autosave.register sets up a timer; stub it so the mounted editor doesn't
// leave a live interval running through the test.
vi.mock('../../autosave.svelte', () => ({
  autosave: { register: () => () => {} },
}));

import NetworkEditor from './NetworkEditor.svelte';
import { session } from '../../session.svelte';

// Added 2026-08-19 alongside migrating this shell's "Connect…"/"Clear
// edges"/"Delete node" buttons to the canvas/canvas-danger Button variants.
// This file (shared by NetworkEditor, PERTEditor, and CPMEditor) had zero
// prior test coverage, and — unlike CPMEditor, which has its own dedicated
// test file — neither NetworkEditor nor PERTEditor has ever had any test
// coverage at all. Gate C on the prior cycle explicitly flagged this as the
// risk to address before migrating this shared file: "migrating its three
// canvas-toolbar buttons changes two untested consumers at once."
//
// Mounted through NetworkEditor specifically (not PERTEditor) because it is
// the thinner of the two real consumers (45 lines, no own onMount/state —
// see its own file header comment: "the shell handles node/edge CRUD; this
// component supplies only the kind-specific bits"), making it the more
// representative, lower-noise way to exercise the shell's own markup. The
// shell owns all three migrated buttons identically regardless of which
// chart kind wraps it, so this is not NetworkEditor-specific coverage — it
// is coverage of the shared file, reached through the thinnest available
// door. PERTEditor remains untested by this pass; migrating shared markup
// is what this row closes, not adding first-time coverage to every
// consumer.

type AppMock = Record<string, ReturnType<typeof vi.fn>>;

function installApp(overrides: Partial<AppMock> = {}): AppMock {
  const chart = {
    id: 'chart-1',
    project_id: 'p1',
    kind: 'network',
    title: 'N',
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

describe('_layered_editor_shell migrated canvas-toolbar buttons (via NetworkEditor)', () => {
  it('"Connect…"/"Clear edges": Button variant="canvas" size="sm"; "Delete node": variant="canvas-danger" — all disabled by default with no node selected', async () => {
    const app = installApp();
    const { getByText } = render(NetworkEditor);
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

// Added 2026-08-19 (second pass, same date): the header "&larr; Dashboard"
// nav link, previously a raw <button>, migrated to the new `nav` Button
// variant. Same disposition as the describe block above: coverage of the
// shared shell file, reached through NetworkEditor as the thinnest door.
describe('_layered_editor_shell migrated header "&larr; Dashboard" button (via NetworkEditor)', () => {
  it('Button variant="nav"', async () => {
    const app = installApp();
    const { getByText } = render(NetworkEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());

    const btn = getByText('← Dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'text-xs text-slate-400 hover:text-cyan-400 disabled:opacity-50'.split(/\s+/).sort(),
    );
  });
});

// Added 2026-08-19 (third pass, same date): the load-error "Back to
// dashboard" button also migrated (variant="primary" size="md"), found in
// passing via a completeness grep after the second pass above. Same
// disposition: coverage of the shared shell file, reached through
// NetworkEditor as the thinnest door.
describe('_layered_editor_shell migrated load-error "Back to dashboard" button (via NetworkEditor)', () => {
  it('Button variant="primary" size="md"', async () => {
    installApp({ GetChart: vi.fn(async () => { throw new Error('boom'); }) });
    const { findByText } = render(NetworkEditor);

    const btn = await findByText('Back to dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'rounded text-xs disabled:opacity-50 px-3 py-2 bg-cyan-600 hover:bg-cyan-500 text-white font-bold uppercase'
        .split(/\s+/)
        .sort(),
    );
  });
});
