// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';

// autosave.register sets up a timer; stub it so the mounted editor doesn't
// leave a live interval running through the test.
vi.mock('../../autosave.svelte', () => ({
  autosave: { register: () => () => {} },
}));

import GanttEditor from './GanttEditor.svelte';
import { session } from '../../session.svelte';

type AppMock = Record<string, ReturnType<typeof vi.fn>>;

// installApp wires a mock Wails bridge onto window.go.main.App and returns it
// for assertions. Overrides replace individual methods (e.g. to reject).
function installApp(overrides: Partial<AppMock> = {}): AppMock {
  const chart = {
    id: 'chart-1',
    project_id: 'p1',
    kind: 'gantt',
    title: 'G',
    data: JSON.stringify({ nodes: [{ id: 'A', label: 'A', duration: 2 }], edges: [] }),
  };
  const emptyLayout = { body: { layout: { rows: [], deps: [], horizon: 0, anchored: false } } };
  const app: AppMock = {
    GetChart: vi.fn(async () => chart),
    SaveChart: vi.fn(async (c: unknown) => c),
    LayoutChart: vi.fn(async () => emptyLayout),
    ListScheduleBaselines: vi.fn(async () => []),
    CompareScheduleBaseline: vi.fn(async () => ({})),
    LevelChartResources: vi.fn(async () => ({ pinned: 0, split_labels: ['A'] })),
    PreviewSplitLeveling: vi.fn(async () => ({ split_task_labels: ['A'], resolves_overallocation: true })),
    SetScheduleBaseline: vi.fn(async () => undefined),
    ...overrides,
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
  return app;
}

async function mountLoaded(app: AppMock) {
  const utils = render(GanttEditor);
  // onMount -> loadChart hits the bridge; wait until that settles so the
  // action we trigger next isn't racing the initial layout load.
  await waitFor(() => expect(app.GetChart).toHaveBeenCalled());
  await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());
  return utils;
}

beforeEach(() => {
  session.editingId = 'chart-1';
});

// The success handlers schedule a 4s status-clear; drop any pending timer so
// it can't fire against an unmounted component in a later test.
afterEach(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
});

describe('GanttEditor split actions (handler glue)', () => {
  it('previewSplit: bridge result flows to the status line', async () => {
    const app = installApp();
    const { getByText, findByText } = await mountLoaded(app);

    await fireEvent.click(getByText('Preview splitting'));

    expect(await findByText(/Splitting 1 task\(s\) would clear overallocation: A/)).toBeInTheDocument();
    expect(app.PreviewSplitLeveling).toHaveBeenCalledWith('chart-1');
  });

  it('levelSplit: calls the bridge with allowSplitting=true and reports the split', async () => {
    const app = installApp();
    const { getByText, findByText } = await mountLoaded(app);

    await fireEvent.click(getByText('Level (split)'));

    expect(await findByText(/Levelled; split 1 task\(s\): A/)).toBeInTheDocument();
    // The 4th argument (allowSplitting) must be true for the Gantt action.
    expect(app.LevelChartResources).toHaveBeenCalledWith('chart-1', 'ltf', false, true);
  });

  it('surfaces a bridge error on the status line instead of throwing', async () => {
    const app = installApp({
      PreviewSplitLeveling: vi.fn(async () => {
        throw new Error('needs a project start date');
      }),
    });
    const { getByText, findByText } = await mountLoaded(app);

    await fireEvent.click(getByText('Preview splitting'));

    expect(await findByText(/needs a project start date/)).toBeInTheDocument();
  });

  it('cancels the split-status timeout when the editor is unmounted', async () => {
    const app = installApp();
    const { getByText, unmount } = await mountLoaded(app);
    vi.useFakeTimers();

    await fireEvent.click(getByText('Preview splitting'));

    expect(app.PreviewSplitLeveling).toHaveBeenCalledWith('chart-1');
    expect(vi.getTimerCount()).toBe(1);
    unmount();
    expect(vi.getTimerCount()).toBe(0);
  });

  it('assigns distinct task IDs when additions share a clock tick', async () => {
    const app = installApp();
    const { getByRole } = await mountLoaded(app);
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-07-21T00:00:00Z'));

    const addTask = getByRole('button', { name: '+ Task' });
    await fireEvent.click(addTask);
    await fireEvent.click(addTask);

    const savedDocs = app.SaveChart.mock.calls.slice(-2).map(([record]) =>
      JSON.parse((record as { data: string }).data) as { nodes: Array<{ id: string }> },
    );
    const addedIDs = savedDocs.map((saved) => saved.nodes.at(-1)?.id);
    expect(addedIDs[0]).toBeDefined();
    expect(addedIDs[0]).not.toBe(addedIDs[1]);
  });
});
