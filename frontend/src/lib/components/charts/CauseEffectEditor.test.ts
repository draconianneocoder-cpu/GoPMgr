// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor } from '@testing-library/svelte';

// autosave.register sets up a timer; stub it so the mounted editor doesn't
// leave a live interval running through the test.
vi.mock('../../autosave.svelte', () => ({
  autosave: { register: () => () => {} },
}));

import CauseEffectEditor from './CauseEffectEditor.svelte';
import { session } from '../../session.svelte';

// Added 2026-08-19, identical disposition to WBSEditor.test.ts's new row —
// see that file's header comment. This file mirrors WBSEditor's UX (its own
// header comment says so) and shares the exact same "Delete" button pattern.
//
// 2026-08-19 (second pass, same date): "+ Child cause"/"+ Sibling"/"Save"
// also migrated, identical disposition to WBSEditor.test.ts's second pass.
//
// 2026-08-19 (third pass, same date): the load-error "Back to dashboard"
// button also migrated, identical disposition to WBSEditor.test.ts's third
// pass.

type AppMock = Record<string, ReturnType<typeof vi.fn>>;

function installApp(overrides: Partial<AppMock> = {}): AppMock {
  const chart = {
    id: 'chart-1',
    project_id: 'p1',
    kind: 'cause_effect',
    title: 'Cause tree',
    data: JSON.stringify({ effect: '', root: { id: 'r', label: 'Root cause', children: [] } }),
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

describe('CauseEffectEditor migrated "Delete" button', () => {
  it('Button variant="canvas-danger" size="sm", disabled by default with no node selected', async () => {
    const app = installApp();
    const { getByText } = render(CauseEffectEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());

    const btn = getByText('Delete') as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'rounded text-xs px-3 py-1 bg-slate-800 hover:bg-red-900 disabled:opacity-30'.split(/\s+/).sort(),
    );
  });
});

describe('CauseEffectEditor migrated "+ Child cause"/"+ Sibling"/"Save" buttons', () => {
  it('"+ Child cause"/"+ Sibling": Button variant="secondary" size="sm"; "Save": variant="primary" size="sm"', async () => {
    const app = installApp();
    const { getByText } = render(CauseEffectEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());

    const secondaryExpected = 'rounded text-xs px-3 py-1 bg-slate-800 hover:bg-slate-700 disabled:opacity-50'
      .split(/\s+/)
      .sort();
    for (const label of ['+ Child cause', '+ Sibling']) {
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

describe('CauseEffectEditor migrated load-error "Back to dashboard" button', () => {
  it('Button variant="primary" size="md"', async () => {
    installApp({ GetChart: vi.fn(async () => { throw new Error('boom'); }) });
    const { findByText } = render(CauseEffectEditor);

    const btn = await findByText('Back to dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'rounded text-xs disabled:opacity-50 px-3 py-2 bg-cyan-600 hover:bg-cyan-500 text-white font-bold uppercase'
        .split(/\s+/)
        .sort(),
    );
  });
});
