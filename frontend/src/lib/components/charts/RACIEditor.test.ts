// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor } from '@testing-library/svelte';

// autosave.register sets up a timer; stub it so the mounted editor doesn't
// leave a live interval running through the test.
vi.mock('../../autosave.svelte', () => ({
  autosave: { register: () => () => {} },
}));

import RACIEditor from './RACIEditor.svelte';
import { session } from '../../session.svelte';

// Added 2026-08-19 alongside migrating this file's header "&larr;
// Dashboard" nav link to the new `Button variant="nav"` (design-critique
// Priority #2 continuation — see `Button.test.ts`'s row for the full
// 28-site/26-file inventory and Gate A scoping). This file had zero prior
// test coverage. Narrow by design: pins only the migrated button's exact
// class string, reachable on mount once GetChart/LayoutChart resolve — does
// not cover role/task/assignment CRUD, layout rendering, or autosave.
//
// 2026-08-19 (second pass, same date): the load-error "Back to dashboard"
// button also migrated (variant="primary" size="md" — the same pattern
// already used by WorkflowEditor/ActivityEditor/WBSEditor/CauseEffectEditor's
// equivalent buttons).

type AppMock = Record<string, ReturnType<typeof vi.fn>>;

function installApp(overrides: Partial<AppMock> = {}): AppMock {
  const chart = {
    id: 'chart-1',
    project_id: 'p1',
    kind: 'raci',
    title: 'RACI',
    data: JSON.stringify({ roles: [], tasks: [], assignments: {} }),
  };
  const app: AppMock = {
    GetChart: vi.fn(async () => chart),
    SaveChart: vi.fn(async (c: unknown) => c),
    LayoutChart: vi.fn(async () => ({
      body: { roles: [], tasks: [], cells: [], validation: { error_count: 0 } },
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

describe('RACIEditor migrated header "&larr; Dashboard" button', () => {
  it('Button variant="nav"', async () => {
    const app = installApp();
    const { getByText } = render(RACIEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());

    const btn = getByText('← Dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'text-xs text-slate-400 hover:text-cyan-400 disabled:opacity-50'.split(/\s+/).sort(),
    );
  });
});

describe('RACIEditor migrated load-error "Back to dashboard" button', () => {
  it('Button variant="primary" size="md"', async () => {
    installApp({ GetChart: vi.fn(async () => { throw new Error('boom'); }) });
    const { findByText } = render(RACIEditor);

    const btn = await findByText('Back to dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'rounded text-xs disabled:opacity-50 px-3 py-2 bg-cyan-600 hover:bg-cyan-500 text-white font-bold uppercase'
        .split(/\s+/)
        .sort(),
    );
  });
});
