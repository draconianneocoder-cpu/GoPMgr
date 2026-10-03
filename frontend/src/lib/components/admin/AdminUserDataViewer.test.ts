// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor, within } from '@testing-library/svelte';

import AdminUserDataViewer from './AdminUserDataViewer.svelte';

let app: Record<string, ReturnType<typeof vi.fn>>;

function projectMeta(name: string): ProjectMeta {
  return {
    name,
    status: 'Active',
    phase: 'Execution',
    start_date: '2026-10-05',
    end_date: '2026-12-01',
    budget: '1000.00',
    currency_code: 'USD',
    owner: 'Bob',
    description: 'Bridge works',
  } as ProjectMeta;
}

beforeEach(() => {
  app = {
    AdminListUserProjects: vi.fn(async () => [{ path: '/data/bob/projects/p1/project.gopmgr', name: 'Bridge', modified: '' }]),
    AdminViewUserProject: vi.fn(async () => ({ project: projectMeta('Bridge'), audit_warning: '' })),
    AdminViewSchedule: vi.fn(async () => [
      {
        chart_id: 'c1', title: 'Delivery', kind: 'gantt', note: '',
        tasks: [
          { id: 'a', title: 'Design', start_date: '2026-10-05', finish_date: '2026-10-07', duration: 3, percent_complete: 100, milestone: false, critical: true },
          { id: 'm', title: 'Handover', start_date: '2026-10-15', finish_date: '2026-10-15', duration: 0, percent_complete: 0, milestone: true, critical: true },
        ],
      },
      { chart_id: 'c2', title: 'Broken', kind: 'cpm', note: "This schedule's data could not be read.", tasks: [] },
    ]),
    AdminViewCosts: vi.fn(async () => ({
      entries: [{ id: 'e1', cost_type_id: 't', kind: 'actual', cost_date: '2026-10-06', description: 'Rebar', amount: '500.00', quantity: '', unit: '', item_name: '', sku: '', supplier_name: 'Acme Steel', invoice_reference: '' }],
      baselines: [],
    })),
    AdminViewDocuments: vi.fn(async () => [
      { id: 'd1', kind: 'charter_word', title: 'Charter', version: 2, status: 'draft', updated_at: '2026-10-06T10:00:00Z' },
    ]),
    AdminStopUserData: vi.fn(async () => undefined),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

async function openBridge(onstop = vi.fn()) {
  const utils = render(AdminUserDataViewer, { username: 'bob', onstop });
  await fireEvent.click(await utils.findByRole('button', { name: 'Bridge' }));
  await utils.findByRole('heading', { name: 'Bridge' });
  return utils;
}

describe('AdminUserDataViewer', () => {
  it('says the view is read only and recorded, and shows the project summary', async () => {
    const utils = await openBridge();
    const region = utils.getByRole('region', { name: "Viewing bob's data" });
    expect(within(region).getByText(/Read only\. You can look but not change, export, or print anything/)).toBeInTheDocument();
    expect(within(region).getByText(/recorded in the account history/)).toBeInTheDocument();
    expect(app.AdminViewUserProject).toHaveBeenCalledWith('/data/bob/projects/p1/project.gopmgr');
    expect(within(region).getByText('Execution')).toBeInTheDocument();
    expect(within(region).getByText('1000.00 USD')).toBeInTheDocument();
    expect(within(region).queryByRole('alert')).not.toBeInTheDocument();
  });

  it('offers no way to change or take data out', async () => {
    const utils = await openBridge();
    for (const name of [/export/i, /print/i, /save/i, /delete/i, /edit/i, /download/i]) {
      expect(utils.queryByRole('button', { name })).not.toBeInTheDocument();
    }
  });

  it('warns about a broken audit trail', async () => {
    app.AdminViewUserProject = vi.fn(async () => ({
      project: projectMeta('Bridge'),
      audit_warning: "This project's audit trail fails its tamper check, so it may have been changed outside GoPMgr: sequence 1",
    }));
    const utils = await openBridge();
    expect(utils.getByRole('alert').textContent).toContain('audit trail fails its tamper check');
  });

  it('loads each view once, when its tab is first shown', async () => {
    const utils = await openBridge();
    await fireEvent.click(utils.getByRole('tab', { name: 'Schedule' }));
    const panel = await utils.findByRole('tabpanel', { name: 'Schedule' });
    await waitFor(() => expect(within(panel).getByText('Design')).toBeInTheDocument());
    expect(within(panel).getByText('milestone')).toBeInTheDocument();
    expect(within(panel).getByText("This schedule's data could not be read.")).toBeInTheDocument();

    await fireEvent.click(utils.getByRole('tab', { name: 'Costs' }));
    await waitFor(() => expect(utils.getByText('Rebar')).toBeInTheDocument());
    expect(utils.getByText('Acme Steel')).toBeInTheDocument();

    await fireEvent.click(utils.getByRole('tab', { name: 'Documents' }));
    await waitFor(() => expect(utils.getByText('Charter')).toBeInTheDocument());
    expect(utils.getByText('Titles only; document contents are not shown.')).toBeInTheDocument();

    await fireEvent.click(utils.getByRole('tab', { name: 'Schedule' }));
    await waitFor(() => expect(utils.getByText('Design')).toBeInTheDocument());
    expect(app.AdminViewSchedule).toHaveBeenCalledTimes(1);
    expect(app.AdminViewCosts).toHaveBeenCalledTimes(1);
    expect(app.AdminViewDocuments).toHaveBeenCalledTimes(1);
  });

  it('stops access and tells the panel', async () => {
    const onstop = vi.fn();
    const utils = await openBridge(onstop);
    await fireEvent.click(utils.getByRole('button', { name: 'Stop viewing' }));
    await waitFor(() => expect(onstop).toHaveBeenCalledTimes(1));
    expect(app.AdminStopUserData).toHaveBeenCalledTimes(1);
  });

  it('reports a view it cannot load', async () => {
    app.AdminViewCosts = vi.fn(async () => {
      throw new Error('administrator privileges required');
    });
    const utils = await openBridge();
    await fireEvent.click(utils.getByRole('tab', { name: 'Costs' }));
    expect((await utils.findByRole('alert')).textContent).toContain('administrator privileges required');
  });
});

describe('AdminUserDataViewer cleanup', () => {
  it('stops access when the view is left without pressing Stop', async () => {
    const utils = await openBridge();
    utils.unmount();
    expect(app.AdminStopUserData).toHaveBeenCalledTimes(1);
  });

  it('does not stop again when left after Stop', async () => {
    const onstop = vi.fn();
    const utils = await openBridge(onstop);
    await fireEvent.click(utils.getByRole('button', { name: 'Stop viewing' }));
    await waitFor(() => expect(onstop).toHaveBeenCalled());
    utils.unmount();
    expect(app.AdminStopUserData).toHaveBeenCalledTimes(1);
  });
});
