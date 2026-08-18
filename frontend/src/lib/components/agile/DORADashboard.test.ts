// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';

import DORADashboard from './DORADashboard.svelte';
import { session } from '../../session.svelte';

type AppMock = Record<string, ReturnType<typeof vi.fn>>;

const metric = (value: number): DORAMetric => ({ value, class: 'medium', label: String(value), caption: '' });

function makeDoraResult(): DORAResult {
  return {
    window_days: 30,
    from: '2026-07-18',
    to: '2026-08-17',
    total_deploys: 1,
    successful_deploys: 1,
    failed_deploys: 0,
    deploy_frequency: metric(1),
    lead_time: metric(2),
    change_failure_rate: metric(0),
    mttr: metric(0),
    daily_deploy_trend: [],
  };
}

function makeDeployment(overrides: Partial<AgileDeployment> = {}): AgileDeployment {
  return {
    id: 'dep-1',
    project_id: 'p1',
    ts: '2026-08-16T10:00:00Z',
    version: 'v1.2.3',
    successful: true,
    lead_time_hours: 4,
    restore_time_hours: 0,
    notes: '',
    ...overrides,
  };
}

function installApp(deploys: AgileDeployment[]): AppMock {
  const app: AppMock = {
    ComputeDORA: vi.fn(async () => makeDoraResult()),
    ListDeployments: vi.fn(async () => deploys),
    SaveDeployment: vi.fn(async (d: unknown) => d),
    DeleteDeployment: vi.fn(async () => undefined),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
  return app;
}

beforeEach(() => {
  session.project = { id: 'p1' } as unknown as typeof session.project;
});

afterEach(() => {
  vi.restoreAllMocks();
  session.project = null;
});

async function mountLoaded(app: AppMock) {
  const utils = render(DORADashboard);
  await waitFor(() => expect(app.ComputeDORA).toHaveBeenCalled());
  await waitFor(() => expect(app.ListDeployments).toHaveBeenCalled());
  return utils;
}

describe('DORADashboard', () => {
  it('renders the deployment log with the seeded record', async () => {
    const app = installApp([makeDeployment()]);
    const { getByText } = await mountLoaded(app);
    await waitFor(() => expect(getByText('v1.2.3')).toBeTruthy());
  });

  // Delete used `confirm()` — silently a no-op in the packaged macOS build,
  // since Wails v2.13.0's darwin WKUIDelegate implements none of the JS
  // confirm/alert/prompt panel methods. Now routed through the shared
  // ConfirmDialog instead.
  it('opens the shared confirm dialog on delete and does not call window.confirm', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm');
    const app = installApp([makeDeployment()]);
    const { container } = await mountLoaded(app);
    const deleteBtn = await waitFor(
      () => container.querySelector('[aria-label="Delete deployment"]') as HTMLButtonElement,
    );
    // Migrated from a raw <button> to `Button variant="remove"` — pins the
    // exact rendered class string so the row's appearance is provably
    // unchanged by the migration, not just assumed safe.
    expect(deleteBtn.className.trim()).toBe(
      'text-slate-500 hover:text-red-400 disabled:opacity-50 text-xs',
    );
    await fireEvent.click(deleteBtn);

    const dialog = document.querySelector('[role="alertdialog"]');
    expect(dialog).not.toBeNull();
    expect(dialog?.textContent).toContain('v1.2.3');
    expect(confirmSpy).not.toHaveBeenCalled();
    expect(app.DeleteDeployment).not.toHaveBeenCalled();
  });

  it('deletes the deployment when the dialog is confirmed', async () => {
    const app = installApp([makeDeployment()]);
    const { container } = await mountLoaded(app);
    const deleteBtn = await waitFor(
      () => container.querySelector('[aria-label="Delete deployment"]') as HTMLButtonElement,
    );
    await fireEvent.click(deleteBtn);
    const confirmBtn = Array.from(document.querySelectorAll('[role="alertdialog"] button')).find(
      (b) => b.textContent?.trim() === 'Delete',
    )!;
    await fireEvent.click(confirmBtn);

    await waitFor(() => expect(app.DeleteDeployment).toHaveBeenCalledWith('dep-1'));
    await waitFor(() => expect(document.querySelector('[role="alertdialog"]')).toBeNull());
  });

  it('keeps the deployment when the dialog is cancelled', async () => {
    const app = installApp([makeDeployment()]);
    const { container } = await mountLoaded(app);
    const deleteBtn = await waitFor(
      () => container.querySelector('[aria-label="Delete deployment"]') as HTMLButtonElement,
    );
    await fireEvent.click(deleteBtn);
    const cancelBtn = Array.from(document.querySelectorAll('[role="alertdialog"] button')).find(
      (b) => b.textContent?.trim() === 'Cancel',
    )!;
    await fireEvent.click(cancelBtn);

    expect(document.querySelector('[role="alertdialog"]')).toBeNull();
    expect(app.DeleteDeployment).not.toHaveBeenCalled();
  });
});
