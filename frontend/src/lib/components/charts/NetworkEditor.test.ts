// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';

// autosave.register sets up a timer; stub it so the mounted editor doesn't
// leave a live interval running through the test.
vi.mock('../../autosave.svelte', () => ({
  autosave: { register: () => () => {} },
}));

import NetworkEditor from './NetworkEditor.svelte';
import { session } from '../../session.svelte';

// Added 2026-08-19. NetworkEditor is already exercised as the mount target
// of _layered_editor_shell.test.ts, so a second file re-asserting that same
// shell-toolbar-button coverage through the same mount path would be a
// duplicate, not new evidence — caught at Gate A before this file was
// written (the prior "next work" note calling this "the mirror of
// PERTEditor.test.ts" was imprecise: PERTEditor had never been mounted by
// anything before its own test; NetworkEditor already had been). This file
// instead covers what neither _layered_editor_shell.test.ts nor any other
// test exercises: NetworkEditor's own two snippets, nodeContent and
// nodeDetailPanel, which only render once a real node exists in the doc and
// (for nodeDetailPanel) is selected — the disclosed gap this ledger has
// named three times ("does not cover node/edge CRUD, layout rendering").

type AppMock = Record<string, ReturnType<typeof vi.fn>>;

function installApp(overrides: Partial<AppMock> = {}): AppMock {
  const chart = {
    id: 'chart-1',
    project_id: 'p1',
    kind: 'network',
    title: 'N',
    data: JSON.stringify({
      nodes: [{ id: 'n1', label: 'Design review', duration: 5, owner: 'Alice' }],
      edges: [],
    }),
  };
  const layoutNode = { id: 'n1', title: 'Design review', depth: 0, x: 0, y: 0, width: 160, height: 70 };
  const app: AppMock = {
    GetChart: vi.fn(async () => chart),
    SaveChart: vi.fn(async (c: unknown) => c),
    LayoutChart: vi.fn(async () => ({
      body: { layout: { nodes: [layoutNode], edges: [], width: 200, height: 100 } },
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

describe('NetworkEditor own snippets (nodeContent / nodeDetailPanel)', () => {
  it('nodeContent renders label, duration, and owner inside the node box', async () => {
    const app = installApp();
    const { getByText, findByRole } = render(NetworkEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());

    await findByRole('button', { name: 'Design review' });
    expect(getByText('Duration: 5')).toBeInTheDocument();
    expect(getByText('Alice')).toBeInTheDocument();
  });

  it('nodeDetailPanel renders the Duration input, bound to the selected node, on selection', async () => {
    const app = installApp();
    const { findByRole, getByText, container } = render(NetworkEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());

    // Not selected yet: the detail panel (and its CPM-chart cross-reference
    // hint) is absent.
    expect(container.querySelector('input[type="number"]')).toBeNull();

    const nodeBox = await findByRole('button', { name: 'Design review' });
    await fireEvent.click(nodeBox);

    const input = container.querySelector('input[type="number"]') as HTMLInputElement;
    expect(input).not.toBeNull();
    expect(input.value).toBe('5');
    expect(getByText('Duration (days)')).toBeInTheDocument();
    expect(getByText(/only records duration as a label/)).toBeInTheDocument();
  });
});
