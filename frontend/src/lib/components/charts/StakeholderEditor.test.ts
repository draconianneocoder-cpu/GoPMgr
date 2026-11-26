// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';

// autosave.register sets up a timer; stub it so the mounted editor doesn't
// leave a live interval running through the test.
vi.mock('../../autosave.svelte', () => ({
  autosave: { register: () => () => {} },
}));

import StakeholderEditor from './StakeholderEditor.svelte';
import { session } from '../../session.svelte';

const desktopApiMock = vi.hoisted(() => {
  const state: { current: Record<string, ReturnType<typeof vi.fn>> } = { current: {} };
  return state;
});
vi.mock('../../desktop-api', () => ({ get desktopApi() { return desktopApiMock.current; } }));

// Added 2026-08-19 alongside migrating this file's header "&larr;
// Dashboard" nav link to the new `Button variant="nav"` (design-critique
// Priority #2 continuation — see `Button.test.ts`'s row for the full
// 28-site/26-file inventory and Gate A scoping). This file had zero prior
// test coverage. Narrow by design: pins only the migrated button's exact
// class string, reachable on mount once GetChart/LayoutChart resolve — does
// not cover the stakeholder-power/interest grid CRUD, layout rendering, or
// autosave.
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
    kind: 'stakeholder_map',
    title: 'Stakeholders',
    data: JSON.stringify({ stakeholders: [] }),
  };
  const app: AppMock = {
    GetChart: vi.fn(async () => chart),
    SaveChart: vi.fn(async (c: unknown) => c),
    LayoutChart: vi.fn(async () => ({ body: { points: [], quadrants: [] } })),
    ...overrides,
  };
  desktopApiMock.current = app;
  return app;
}

beforeEach(() => {
  session.editingId = 'chart-1';
});

afterEach(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
});

describe('StakeholderEditor migrated header "&larr; Dashboard" button', () => {
  it('Button variant="nav"', async () => {
    const app = installApp();
    const { getByText } = render(StakeholderEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());

    const btn = getByText('← Dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'text-xs text-slate-400 hover:text-cyan-400 disabled:opacity-50'.split(/\s+/).sort(),
    );
  });
});

describe('StakeholderEditor migrated load-error "Back to dashboard" button', () => {
  it('Button variant="primary" size="md"', async () => {
    installApp({ GetChart: vi.fn(async () => { throw new Error('boom'); }) });
    const { findByText } = render(StakeholderEditor);

    const btn = await findByText('Back to dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'rounded text-xs disabled:opacity-50 px-3 py-2 bg-cyan-700 hover:bg-cyan-800 text-white font-bold uppercase'
        .split(/\s+/)
        .sort(),
    );
  });
});

// Selecting an item must not write the chart. A <select bind:value> on a
// field the stored chart omits writes the first option back on mount
// (Svelte's bind_select_value), so one click saved an untouched chart.
describe('StakeholderEditor select', () => {
  it('selects a stakeholder stored without power and interest without writing the chart', async () => {
    vi.useFakeTimers();
    const app = installApp({
      GetChart: vi.fn(async () => ({ id: 'chart-1', project_id: 'p1', kind: 'stakeholder_analysis', title: 'S',
        data: JSON.stringify({ stakeholders: [{ id: '1', name: 'Sponsor' }] }) })),
      LayoutChart: vi.fn(async () => ({ body: { points: [{ id: '1', name: 'Sponsor', power: '', interest: '', strategy: 'Monitor', x: 0.25, y: 0.75 }], quadrants: [] } })),
    });
    const { getByRole } = render(StakeholderEditor);
    await vi.advanceTimersByTimeAsync(0);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());
    await fireEvent.click(getByRole('button', { name: 'Sponsor' }));
    await vi.advanceTimersByTimeAsync(400);
    expect(app.SaveChart).not.toHaveBeenCalled();
  });
});
