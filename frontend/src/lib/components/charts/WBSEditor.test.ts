// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';
import { tick } from 'svelte';
import { autosave } from '../../autosave.svelte';

import WBSEditor from './WBSEditor.svelte';
import { cancelNavigation, session } from '../../session.svelte';

// Preserve the migrated Button class contracts alongside persistence tests.
// Persistence fixtures use the real coordinator and commit at release time.

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
  autosave.setInterval(0);
});

afterEach(() => {
  cleanup();
  autosave.discardAll();
  cancelNavigation();
  vi.restoreAllMocks();
  vi.clearAllTimers();
  vi.useRealTimers();
});

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((done, fail) => { resolve = done; reject = fail; });
  return { promise, resolve, reject };
}

function installPersistedApp() {
  let persisted: ChartRecord = {
    id: 'chart-1', project_id: 'p1', kind: 'wbs', title: 'WBS', config: '{}',
    template_id: '', created_at: '', updated_at: '',
    data: JSON.stringify({ root: { id: 'r', title: 'Project', children: [] } }),
  };
  const commit = (record: ChartRecord) => {
    persisted = { ...record, updated_at: 'committed' };
    return { ...persisted };
  };
  const app = installApp({
    GetChart: vi.fn(async () => ({ ...persisted })),
    SaveChart: vi.fn(async (record: ChartRecord) => commit(record)),
    LayoutChart: vi.fn(async () => {
      const root = JSON.parse(persisted.data).root;
      return { body: { nodes: [{ ...root, number: '1', x: 0, y: 0, width: 150, height: 60 }],
        edges: [], width: 150, height: 60 } };
    }),
  });
  return { app, commit, read: () => JSON.parse(persisted.data).root.title };
}

async function mountSelected() {
  const fixture = installPersistedApp();
  const utils = render(WBSEditor);
  await waitFor(() => expect(utils.getByText('Save')).not.toBeDisabled());
  await utils.findByRole('button', { name: '1 Project' });
  vi.useFakeTimers();
  await fireEvent.click(await utils.findByRole('button', { name: '1 Project' }));
  await tick();
  // Selection schedules the same debounced preview as a field edit.
  return { ...fixture, ...utils };
}

describe('WBS registered writer ordering', () => {
  it('waits for a guarded writer before previewing the latest edit', async () => {
    const f = await mountSelected();
    const release = deferred<void>();
    f.app.SaveChart.mockImplementationOnce(async (record: ChartRecord) => {
      await release.promise;
      return f.commit(record);
    });
    await fireEvent.input(f.getByLabelText('Title'), { target: { value: 'guarded' } });
    const guarded = autosave.saveAll();
    await fireEvent.input(f.getByLabelText('Title'), { target: { value: 'latest preview' } });
    await vi.advanceTimersByTimeAsync(300);
    const calls = f.app.SaveChart.mock.calls.length;
    release.resolve();
    expect(await guarded).toBe(false);
    await vi.advanceTimersByTimeAsync(0);
    expect(f.app.SaveChart).toHaveBeenCalledTimes(calls + 1);
    expect(f.read()).toBe('latest preview');
    expect(f.getByRole('button', { name: '1 latest preview' })).toBeInTheDocument();
    expect(autosave.hasDirty()).toBe(false);
  });

  it('acknowledges the toolbar save through the registered baseline', async () => {
    const f = await mountSelected();
    await fireEvent.input(f.getByLabelText('Title'), { target: { value: 'toolbar edit' } });
    await fireEvent.click(f.getByText('Save'));
    await vi.advanceTimersByTimeAsync(0);
    expect(f.read()).toBe('toolbar edit');
    expect(autosave.hasDirty()).toBe(false);
  });

  it('blocks a guarded save behind an older preview and coalesces the newest edit', async () => {
    const f = await mountSelected();
    const release = deferred<void>();
    f.app.SaveChart.mockImplementationOnce(async (record: ChartRecord) => {
      await release.promise;
      return f.commit(record);
    });
    await fireEvent.input(f.getByLabelText('Title'), { target: { value: 'older' } });
    await vi.advanceTimersByTimeAsync(300);
    const calls = f.app.SaveChart.mock.calls.length;
    await fireEvent.input(f.getByLabelText('Title'), { target: { value: 'newest' } });
    await vi.advanceTimersByTimeAsync(300);
    expect(await autosave.saveAll()).toBe(false);
    expect(f.app.SaveChart).toHaveBeenCalledTimes(calls);
    expect(f.read()).toBe('Project');
    release.resolve();
    await vi.advanceTimersByTimeAsync(0);
    expect(f.app.SaveChart).toHaveBeenCalledTimes(calls + 1);
    expect(f.read()).toBe('newest');
    expect(f.getByRole('button', { name: '1 newest' })).toBeInTheDocument();
    expect(await autosave.saveAll()).toBe(true);
    expect(autosave.hasDirty()).toBe(false);
  });

  it('retains edits restored to the original value during a preview write', async () => {
    const f = await mountSelected();
    const release = deferred<void>();
    f.app.SaveChart.mockImplementationOnce(async (record: ChartRecord) => {
      await release.promise;
      return f.commit(record);
    });
    await fireEvent.input(f.getByLabelText('Title'), { target: { value: 'submitted' } });
    await vi.advanceTimersByTimeAsync(300);
    await fireEvent.input(f.getByLabelText('Title'), { target: { value: 'Project' } });
    expect(autosave.hasDirty()).toBe(true);
    expect(await autosave.saveAll()).toBe(false);
    release.resolve();
    await vi.advanceTimersByTimeAsync(0);
    expect(f.read()).toBe('submitted');
    expect(f.getByLabelText('Title')).toHaveValue('Project');
    expect(autosave.hasDirty()).toBe(true);
    expect(await autosave.saveAll()).toBe(true);
    expect(f.read()).toBe('Project');
  });

  it('does not acknowledge a failed write and permits an explicit retry', async () => {
    const f = await mountSelected();
    f.app.SaveChart.mockRejectedValueOnce(new Error('write unavailable'));
    await fireEvent.input(f.getByLabelText('Title'), { target: { value: 'edited' } });
    await vi.advanceTimersByTimeAsync(300);
    expect(f.getByRole('status')).toHaveTextContent('write unavailable');
    expect(f.read()).toBe('Project');
    expect(autosave.hasDirty()).toBe(true);
    expect(await autosave.saveAll()).toBe(true);
    expect(f.read()).toBe('edited');
  });

  it('keeps a committed write acknowledged when its layout fails', async () => {
    const f = await mountSelected();
    f.app.LayoutChart.mockRejectedValueOnce(new Error('layout unavailable'));
    await fireEvent.input(f.getByLabelText('Title'), { target: { value: 'edited' } });
    expect(await autosave.saveAll()).toBe(true);
    await tick();
    expect(f.read()).toBe('edited');
    expect(autosave.hasDirty()).toBe(false);
    expect(f.getByRole('status')).toHaveTextContent('layout unavailable');
  });

  it('does not submit a coalesced preview after destruction', async () => {
    const f = await mountSelected();
    const release = deferred<void>();
    f.app.SaveChart.mockImplementationOnce(async (record: ChartRecord) => {
      await release.promise;
      return f.commit(record);
    });
    await fireEvent.input(f.getByLabelText('Title'), { target: { value: 'submitted' } });
    await vi.advanceTimersByTimeAsync(300);
    await fireEvent.input(f.getByLabelText('Title'), { target: { value: 'discarded' } });
    await vi.advanceTimersByTimeAsync(300);
    const writes = f.app.SaveChart.mock.calls.length;
    const layouts = f.app.LayoutChart.mock.calls.length;
    f.unmount();
    release.resolve();
    await vi.advanceTimersByTimeAsync(300);
    expect(f.app.SaveChart).toHaveBeenCalledTimes(writes);
    expect(f.app.LayoutChart).toHaveBeenCalledTimes(layouts);
    expect(autosave.hasDirty()).toBe(false);
  });

  it('does not register or write after a load completes on a destroyed editor', async () => {
    const f = installPersistedApp();
    const load = deferred<ChartRecord>();
    const register = vi.spyOn(autosave, 'register');
    f.app.GetChart.mockReturnValueOnce(load.promise);
    const utils = render(WBSEditor);
    utils.unmount();
    load.resolve(await f.app.GetChart());
    await tick();
    expect(register).not.toHaveBeenCalled();
    expect(f.app.SaveChart).not.toHaveBeenCalled();
    // Clean up any orphan produced while demonstrating old behavior.
    for (const result of register.mock.results) if (result.type === 'return') result.value();
  });

  it('handles a load rejection after destruction without registering', async () => {
    const f = installPersistedApp();
    const load = deferred<ChartRecord>();
    const register = vi.spyOn(autosave, 'register');
    f.app.GetChart.mockReturnValueOnce(load.promise);
    const utils = render(WBSEditor);
    utils.unmount();
    load.reject(new Error('late load failure'));
    await tick();
    expect(register).not.toHaveBeenCalled();
    expect(f.app.SaveChart).not.toHaveBeenCalled();
  });

  it('does not write again after destruction during layout', async () => {
    const f = await mountSelected();
    const pending = deferred<unknown>();
    f.app.LayoutChart.mockReturnValueOnce(pending.promise);
    await fireEvent.input(f.getByLabelText('Title'), { target: { value: 'submitted' } });
    await vi.advanceTimersByTimeAsync(300);
    await fireEvent.input(f.getByLabelText('Title'), { target: { value: 'discarded' } });
    await vi.advanceTimersByTimeAsync(300);
    const writes = f.app.SaveChart.mock.calls.length;
    f.unmount();
    pending.reject(new Error('late layout failure'));
    await vi.advanceTimersByTimeAsync(0);
    expect(f.app.SaveChart).toHaveBeenCalledTimes(writes);
    expect(autosave.hasDirty()).toBe(false);
  });

  for (const data of ['{broken', '']) it(`keeps recovered content dirty after an initial failure (${JSON.stringify(data)})`, async () => {
    const f = installPersistedApp();
    const record = await f.app.GetChart();
    f.app.GetChart.mockResolvedValueOnce({ ...record, data });
    f.app.SaveChart.mockRejectedValueOnce(new Error('write unavailable'));
    const utils = render(WBSEditor);
    await utils.findByText(/write unavailable/);
    expect(autosave.hasDirty()).toBe(true);
    expect(await autosave.saveAll()).toBe(true);
    expect(autosave.hasDirty()).toBe(false);
  });
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
