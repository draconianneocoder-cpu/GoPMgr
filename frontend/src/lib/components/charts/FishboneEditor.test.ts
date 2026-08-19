// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';

// autosave.register sets up a timer; stub it so the mounted editor doesn't
// leave a live interval running through the test.
vi.mock('../../autosave.svelte', () => ({
  autosave: { register: () => () => {} },
}));

import FishboneEditor from './FishboneEditor.svelte';
import { session } from '../../session.svelte';

type AppMock = Record<string, ReturnType<typeof vi.fn>>;

const emptyLayout = { nodes: [], edges: [], width: 0, height: 0 };

// installApp wires a mock Wails bridge onto window.go.main.App. `data` is
// the chart's stored FishboneDoc JSON, so tests can start from either an
// empty or a populated categories list.
function installApp(data: string): AppMock {
  const chart = { id: 'chart-1', project_id: 'p1', kind: 'fishbone', title: 'F', data };
  const app: AppMock = {
    GetChart: vi.fn(async () => chart),
    SaveChart: vi.fn(async (c: unknown) => c),
    LayoutChart: vi.fn(async () => ({ body: emptyLayout })),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
  return app;
}

async function mountLoaded(app: AppMock) {
  const utils = render(FishboneEditor);
  await waitFor(() => expect(app.GetChart).toHaveBeenCalled());
  await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());
  return utils;
}

beforeEach(() => {
  session.editingId = 'chart-1';
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('FishboneEditor "Apply 6 Ms preset"', () => {
  it('applies the preset immediately with no dialog when there are no existing categories', async () => {
    const app = installApp(JSON.stringify({ effect: '', categories: [] }));
    const { getByText } = await mountLoaded(app);
    const saveCallsBefore = app.SaveChart.mock.calls.length;

    await fireEvent.click(getByText('Apply 6 Ms preset'));

    expect(document.querySelector('[role="alertdialog"]')).toBeNull();
    await waitFor(() => expect(app.SaveChart.mock.calls.length).toBeGreaterThan(saveCallsBefore));
    const saved = JSON.parse(app.SaveChart.mock.calls.at(-1)![0].data);
    expect(saved.categories.map((c: { name: string }) => c.name)).toEqual([
      'People',
      'Process',
      'Equipment',
      'Materials',
      'Environment',
      'Measurement',
    ]);
  });

  // Used `confirm()` — silently a no-op in the packaged macOS build, since
  // Wails v2.13.0's darwin WKUIDelegate implements none of the JS
  // confirm/alert/prompt panel methods. Now routed through the shared
  // ConfirmDialog instead.
  it('opens the shared confirm dialog and does not call window.confirm or replace categories when existing categories would be overwritten', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm');
    const app = installApp(
      JSON.stringify({ effect: 'Late deliveries', categories: [{ name: 'Custom category', causes: [] }] }),
    );
    const { getByText } = await mountLoaded(app);
    const saveCallsBefore = app.SaveChart.mock.calls.length;

    await fireEvent.click(getByText('Apply 6 Ms preset'));

    const dialog = document.querySelector('[role="alertdialog"]');
    expect(dialog).not.toBeNull();
    expect(dialog?.textContent).toContain('Replace existing categories');
    expect(confirmSpy).not.toHaveBeenCalled();
    expect(app.SaveChart.mock.calls.length).toBe(saveCallsBefore);
    expect((document.querySelectorAll('aside input')[1] as HTMLInputElement).value).toBe('Custom category');
  });

  it('replaces categories with the preset when the dialog is confirmed', async () => {
    const app = installApp(
      JSON.stringify({ effect: 'Late deliveries', categories: [{ name: 'Custom category', causes: [] }] }),
    );
    const { getByText } = await mountLoaded(app);

    await fireEvent.click(getByText('Apply 6 Ms preset'));
    await fireEvent.click(getByText('Replace'));

    await waitFor(() => expect(document.querySelector('[role="alertdialog"]')).toBeNull());
    await waitFor(() => {
      const last = app.SaveChart.mock.calls.at(-1)![0];
      const saved = JSON.parse(last.data);
      expect(saved.categories.map((c: { name: string }) => c.name)).toContain('People');
    });
  });

  // Post-implementation advisor review (M1): confirmSixMs() fires an
  // unawaited refreshLayout() and the dialog closes immediately, so nothing
  // originally stopped a second click from reopening the dialog mid-save.
  // Proves the trigger button is disabled for the duration of that in-flight
  // save, using a controllable SaveChart promise to observe the mid-flight
  // state deterministically instead of racing a real async resolution.
  it('disables the trigger button while the confirmed preset is still saving', async () => {
    const app = installApp(
      JSON.stringify({ effect: 'Late deliveries', categories: [{ name: 'Custom category', causes: [] }] }),
    );
    // installApp's default auto-resolving SaveChart runs the initial mount's
    // own refreshLayout() (onMount saves once on load) — only swap in a
    // controllable promise afterward, or the mount itself never completes.
    const { getByText } = await mountLoaded(app);
    let resolveSave!: (v: unknown) => void;
    app.SaveChart.mockImplementation(
      (c: unknown) =>
        new Promise((resolve) => {
          resolveSave = () => resolve(c);
        }),
    );

    await fireEvent.click(getByText('Apply 6 Ms preset'));
    await fireEvent.click(getByText('Replace'));

    const trigger = getByText('Apply 6 Ms preset').closest('button') as HTMLButtonElement;
    expect(trigger.disabled).toBe(true);

    resolveSave(undefined);
    await waitFor(() => expect(trigger.disabled).toBe(false));
  });

  it('keeps the existing categories when the dialog is cancelled', async () => {
    const app = installApp(
      JSON.stringify({ effect: 'Late deliveries', categories: [{ name: 'Custom category', causes: [] }] }),
    );
    const { getByText } = await mountLoaded(app);
    const saveCallsBefore = app.SaveChart.mock.calls.length;

    await fireEvent.click(getByText('Apply 6 Ms preset'));
    await fireEvent.click(getByText('Cancel'));

    expect(document.querySelector('[role="alertdialog"]')).toBeNull();
    expect(app.SaveChart.mock.calls.length).toBe(saveCallsBefore);
    expect((document.querySelectorAll('aside input')[1] as HTMLInputElement).value).toBe('Custom category');
  });
});

// Added 2026-08-19 alongside migrating the header "&larr; Dashboard" nav
// link (previously a raw <button>) to the new `nav` Button variant — a
// distinct, unpadded exempt-branch idiom, like `link`/`remove` — see
// Button.svelte's `nav` branch comment.
describe('FishboneEditor migrated header "&larr; Dashboard" button', () => {
  it('Button variant="nav"', async () => {
    const app = installApp(JSON.stringify({ effect: '', categories: [] }));
    const { getByText } = await mountLoaded(app);

    const btn = getByText('← Dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'text-xs text-slate-400 hover:text-cyan-400 disabled:opacity-50'.split(/\s+/).sort(),
    );
  });
});

// Added 2026-08-19 (second pass, same date): the load-error "Back to
// dashboard" button also migrated (variant="primary" size="md" — the same
// pattern already used by WorkflowEditor/ActivityEditor/WBSEditor/
// CauseEffectEditor's equivalent buttons, found in passing via a completeness
// grep after the cycle 9/10 nav-variant migration).
describe('FishboneEditor migrated load-error "Back to dashboard" button', () => {
  it('Button variant="primary" size="md"', async () => {
    const app: AppMock = {
      GetChart: vi.fn(async () => { throw new Error('boom'); }),
      SaveChart: vi.fn(async (c: unknown) => c),
      LayoutChart: vi.fn(async () => ({ body: emptyLayout })),
    };
    (window as unknown as { go: unknown }).go = { main: { App: app } };
    const { findByText } = render(FishboneEditor);

    const btn = await findByText('Back to dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'rounded text-xs disabled:opacity-50 px-3 py-2 bg-cyan-600 hover:bg-cyan-500 text-white font-bold uppercase'
        .split(/\s+/)
        .sort(),
    );
  });
});
