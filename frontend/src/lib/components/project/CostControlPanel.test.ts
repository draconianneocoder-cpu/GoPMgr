// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import CostControlPanel from './CostControlPanel.svelte';

function installApp(baselines: Array<Record<string, string | number>> = []) {
  const app = {
    ListCostTypes: vi.fn(async () => [{ id: 'labor', project_id: 'p', code: 'labor', name: 'Labor', attribution: 'direct', behavior: 'variable', treatment: 'opex', active: true }]),
    ListCostEntries: vi.fn(async () => []),
    ListCostReserves: vi.fn(async () => []),
    ListCostBaselines: vi.fn(async () => baselines),
    ComputeCostSummary: vi.fn(async () => ({ currency_code: 'USD', funding: '1000.00', planned: '800.00', contingency: '100.00', cost_baseline: '900.00', management_reserve: '50.00', authorised_funding: '950.00', commitment: '0.00', actual: '0.00', remaining_funding: '150.00' })),
    SaveCostEntry: vi.fn(async (entry) => entry),
    SaveCostReserve: vi.fn(async (reserve) => reserve),
    ApproveCostBaseline: vi.fn(async () => ({})),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
  return app;
}

describe('CostControlPanel', () => {
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
    installApp([{ version: 1, currency_code: 'USD', planned: '100.00', contingency: '10.00', cost_baseline: '110.00', management_reserve: '5.00', authorised_funding: '115.00', approved_by: 'alice', approval_note: 'Approved after risk review', approved_at: approvedAt }]);
    const { findByText } = render(CostControlPanel);
    expect(await findByText('Approved after risk review')).toBeInTheDocument();
    expect(await findByText(approvedAt)).toHaveAttribute('datetime', approvedAt);
  });
});
