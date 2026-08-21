// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render, waitFor } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import CostControlPanel from './CostControlPanel.svelte';

type CostControlFixture = {
  baselines?: Array<Record<string, string | number>>;
  types?: Array<Record<string, string | boolean>>;
  entries?: Array<Record<string, string>>;
  failFirstLoad?: boolean;
  mutationDisabledReason?: string;
};

function installApp({ baselines = [], types = [{ id: 'labor', project_id: 'p', code: 'labor', name: 'Labor', attribution: 'direct', behavior: 'variable', treatment: 'opex', active: true }], entries = [], failFirstLoad = false, mutationDisabledReason = '' }: CostControlFixture = {}) {
  const listCostTypes = failFirstLoad
    ? vi.fn().mockRejectedValueOnce(new Error('temporary load failure')).mockResolvedValue(types)
    : vi.fn(async () => types);
  const app = {
    ListCostTypes: listCostTypes,
    ListCostEntries: vi.fn(async () => entries),
    ListCostReserves: vi.fn(async () => []),
    ListCostBaselines: vi.fn(async () => baselines),
    ComputeCostSummary: vi.fn(async () => ({ currency_code: 'USD', mutation_disabled_reason: mutationDisabledReason, legacy_budget: '1000.00', planned: '800.00', contingency: '100.00', cost_baseline: '900.00', management_reserve: '50.00', authorised_funding: '950.00', commitment: '0.00', actual: '0.00' })),
    ComputeCostClassificationSummary: vi.fn(async () => ({ attribution: [{ value: 'direct', planned: '800.00', commitment: '0.00', actual: '0.00' }, { value: 'indirect', planned: '0.00', commitment: '0.00', actual: '0.00' }], behavior: [{ value: 'fixed', planned: '0.00', commitment: '0.00', actual: '0.00' }, { value: 'variable', planned: '800.00', commitment: '0.00', actual: '0.00' }], treatment: [{ value: 'capex', planned: '0.00', commitment: '0.00', actual: '0.00' }, { value: 'opex', planned: '800.00', commitment: '0.00', actual: '0.00' }, { value: 'not_applicable', planned: '0.00', commitment: '0.00', actual: '0.00' }] })),
    SaveCostEntry: vi.fn(async (entry) => entry),
    SaveCostReserve: vi.fn(async (reserve) => reserve),
    ApproveCostBaseline: vi.fn(async () => ({})),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
  return app;
}

describe('CostControlPanel', () => {
  it('separates the legacy Budget rollup and omits unsupported funding labels', async () => {
    installApp();
    const { findByText, queryByText } = render(CostControlPanel);
    expect(await findByText('Legacy budget rollup')).toBeInTheDocument();
    expect(await findByText('Shown for context only; it is not included in Cost Control baseline or authorised funding.')).toBeInTheDocument();
    expect(queryByText(/^Funding$/)).not.toBeInTheDocument();
    expect(queryByText('Unallocated')).not.toBeInTheDocument();
  });

  it('shows independent classification lenses and keeps historical archived types visible', async () => {
    installApp({
      types: [
        { id: 'labor', project_id: 'p', code: 'labor', name: 'Labor', attribution: 'direct', behavior: 'variable', treatment: 'opex', active: true },
        { id: 'retired', project_id: 'p', code: 'retired', name: 'Retired capital', attribution: 'direct', behavior: 'fixed', treatment: 'capex', active: false },
      ],
      entries: [{ id: 'entry-1', cost_type_id: 'retired', kind: 'actual', cost_date: '2026-08-21', description: 'Historical asset', amount: '25.00' }],
    });
    const { findByText, getByLabelText, queryByRole } = render(CostControlPanel);
    expect(await findByText('Classification and reconciliation')).toBeInTheDocument();
    expect(await findByText('Each lens independently reconciles to the ledger totals. Do not add values across lenses.')).toBeInTheDocument();
    expect(await findByText('Retired capital')).toBeInTheDocument();
    expect(queryByRole('option', { name: 'Retired capital' })).not.toBeInTheDocument();
    expect(getByLabelText('Cost type')).toHaveValue('labor');
  });

  it('offers a retry after initial Cost Control loading fails', async () => {
    const app = installApp({ failFirstLoad: true });
    const { findByRole, findByText } = render(CostControlPanel);
    expect(await findByRole('alert')).toHaveTextContent('temporary load failure');
    await fireEvent.click(await findByRole('button', { name: 'Retry Cost Control' }));
    expect(await findByText('Legacy budget rollup')).toBeInTheDocument();
    expect(app.ListCostTypes).toHaveBeenCalledTimes(2);
  });

  it('makes a legacy JPY project read-only while retaining its ledger view', async () => {
    const app = installApp({
      mutationDisabledReason: 'Cost Control is read-only for this legacy JPY project: existing amounts retain their original fixed two-decimal convention and are not being converted.',
      entries: [{ id: 'entry-1', cost_type_id: 'labor', kind: 'actual', cost_date: '2026-08-21', description: 'Existing JPY amount', amount: '123.45' }],
    });
    const { findByRole, findByText, getAllByLabelText, getByLabelText, getByRole } = render(CostControlPanel);
    expect(await findByRole('alert')).toHaveTextContent('legacy JPY project');
    expect(await findByText('Existing JPY amount')).toBeInTheDocument();
    expect(getAllByLabelText('Amount')[0]).toBeDisabled();
    expect(getByLabelText('Reserve amount')).toBeDisabled();
    expect(getByLabelText('Approval rationale')).toBeDisabled();
    expect(getByRole('button', { name: 'Add ledger entry' })).toBeDisabled();
    expect(getByRole('button', { name: 'Set reserve balance' })).toBeDisabled();
    expect(getByRole('button', { name: 'Approve immutable baseline' })).toBeDisabled();
    expect(app.SaveCostEntry).not.toHaveBeenCalled();
  });

  it('finishes taxonomy seeding before it starts classification reads', async () => {
    let releaseTypes: (types: Array<Record<string, string | boolean>>) => void;
    const seededTypes = [{ id: 'labor', project_id: 'p', code: 'labor', name: 'Labor', attribution: 'direct', behavior: 'variable', treatment: 'opex', active: true }];
    const typesReady = new Promise<Array<Record<string, string | boolean>>>((resolve) => { releaseTypes = resolve; });
    const app = installApp();
    app.ListCostTypes.mockImplementationOnce(() => typesReady);

    render(CostControlPanel);
    await Promise.resolve();
    expect(app.ComputeCostClassificationSummary).not.toHaveBeenCalled();

    releaseTypes!(seededTypes);
    await waitFor(() => expect(app.ComputeCostClassificationSummary).toHaveBeenCalledTimes(1));
  });

  it('keeps contingency and management reserves distinct and saves a reserve', async () => {
    const app = installApp();
    const { findByText, getByLabelText, getByRole } = render(CostControlPanel);
    await findByText('Contingency reserve');
    await findByText('Management reserve');
    await fireEvent.input(getByLabelText('Reserve amount'), { target: { value: '25.00' } });
    await fireEvent.input(getByLabelText('Basis / owner note'), { target: { value: 'Known supplier risk' } });
    await fireEvent.click(getByRole('button', { name: 'Set reserve balance' }));
    expect(app.SaveCostReserve).toHaveBeenCalledWith({ kind: 'contingency', amount: '25.00', description: 'Known supplier risk' });
  });

  it('requires confirmation before recording an immutable baseline', async () => {
    const app = installApp();
    const { findByLabelText, findByRole } = render(CostControlPanel);
    const note = await findByLabelText('Approval rationale');
    await fireEvent.input(note, { target: { value: 'Approved plan' } });
    await fireEvent.click(await findByRole('button', { name: 'Approve immutable baseline' }));
    expect(app.ApproveCostBaseline).not.toHaveBeenCalled();
    await fireEvent.click(await findByRole('button', { name: 'Approve baseline' }));
    expect(app.ApproveCostBaseline).toHaveBeenCalledWith('Approved plan');
  });

  it('shows an immutable baseline rationale and approval timestamp in history', async () => {
    const approvedAt = '2026-08-20T22:15:00.000000000Z';
    installApp({ baselines: [{ version: 1, currency_code: 'USD', planned: '100.00', contingency: '10.00', cost_baseline: '110.00', management_reserve: '5.00', authorised_funding: '115.00', approved_by: 'alice', approval_note: 'Approved after risk review', approved_at: approvedAt }] });
    const { findByText } = render(CostControlPanel);
    expect(await findByText('Approved after risk review')).toBeInTheDocument();
    expect(await findByText(approvedAt)).toHaveAttribute('datetime', approvedAt);
  });
});
