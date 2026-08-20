// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import CostControlPanel from './CostControlPanel.svelte';

function installApp() {
  const app = {
    ListCostTypes: vi.fn(async () => [{ id: 'labor', project_id: 'p', code: 'labor', name: 'Labor', attribution: 'direct', behavior: 'variable', treatment: 'opex', active: true }]),
    ListCostEntries: vi.fn(async () => []),
    ListCostReserves: vi.fn(async () => []),
    ComputeCostSummary: vi.fn(async () => ({ currency_code: 'USD', funding: '1000.00', planned: '800.00', contingency: '100.00', cost_baseline: '900.00', management_reserve: '50.00', authorised_funding: '950.00', commitment: '0.00', actual: '0.00', remaining_funding: '150.00' })),
    SaveCostEntry: vi.fn(async (entry) => entry),
    SaveCostReserve: vi.fn(async (reserve) => reserve),
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
});
