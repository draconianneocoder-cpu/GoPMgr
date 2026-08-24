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
    ExportFinancialReportPDF: vi.fn(async () => '/tmp/financial-report.pdf'),
    SaveCostEntry: vi.fn(async (entry) => entry),
    SaveCostReserve: vi.fn(async (reserve) => reserve),
    ApproveCostBaseline: vi.fn(async () => ({})),
    AggregateCostEntryQuantities: vi.fn(async () => []),
    SearchCostEntries: vi.fn(async () => entries),
    ListCostEntryAttachments: vi.fn(async () => []),
    AttachCostEntryFile: vi.fn(async () => ({ id: 'att-1', cost_entry_id: '', filename: 'file.pdf', content_type: 'application/pdf', size_bytes: 1024, sha256: '', created_at: '' })),
    ExportCostEntryAttachmentsZip: vi.fn(async () => '/tmp/attachments.zip'),
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

  it('uses the existing free-text description to identify a cost item without changing the save payload', async () => {
    const app = installApp({
      entries: [{ id: 'entry-1', cost_type_id: 'labor', kind: 'actual', cost_date: '2026-08-21', description: 'Historic supplier reference', amount: '25.00' }],
    });
    const { findByRole, findByText, getAllByLabelText, getByLabelText, getByRole } = render(CostControlPanel);
    expect(await findByRole('columnheader', { name: 'Cost item or reference' })).toBeInTheDocument();
    expect(getByLabelText('Cost item or reference')).toHaveAttribute('placeholder', 'Material, invoice reference, supplier, or overhead note');
    await fireEvent.input(getByLabelText('Cost item or reference'), { target: { value: 'Concrete delivery, supplier PO-1042' } });
    await fireEvent.input(getAllByLabelText('Amount')[0], { target: { value: '25.00' } });
    await fireEvent.input(getByLabelText('Date'), { target: { value: '2026-08-21' } });
    await fireEvent.click(getByRole('button', { name: 'Add ledger entry' }));
    const saved = app.SaveCostEntry.mock.calls[0][0];
    expect(Object.keys(saved).sort()).toEqual(['amount', 'cost_date', 'cost_type_id', 'description', 'id', 'invoice_reference', 'item_name', 'kind', 'quantity', 'sku', 'supplier_name', 'unit']);
    expect(saved).toEqual({
      id: '',
      cost_type_id: 'labor',
      kind: 'planned',
      amount: '25.00',
      description: 'Concrete delivery, supplier PO-1042',
      cost_date: '2026-08-21',
      quantity: '',
      unit: '',
      item_name: '',
      sku: '',
      supplier_name: '',
      invoice_reference: '',
    });
    expect(await findByText('Historic supplier reference')).toBeInTheDocument();
  });

  it('saves structured procurement detail (item, SKU, supplier, invoice ref, quantity) alongside the free-text description', async () => {
    const app = installApp();
    const { findByText, getAllByLabelText, getByLabelText, getByRole } = render(CostControlPanel);
    await findByText('Legacy budget rollup');
    await fireEvent.input(getByLabelText('Cost item or reference'), { target: { value: 'Structural steel delivery' } });
    await fireEvent.input(getAllByLabelText('Amount')[0], { target: { value: '1200.00' } });
    await fireEvent.input(getByLabelText('Date'), { target: { value: '2026-08-21' } });
    await fireEvent.input(getByLabelText('Item name'), { target: { value: 'Structural steel beam' } });
    await fireEvent.input(getByLabelText('SKU'), { target: { value: 'SKU-BEAM-1' } });
    await fireEvent.input(getByLabelText('Supplier'), { target: { value: 'Acme Structural Supply' } });
    await fireEvent.input(getByLabelText('Invoice reference'), { target: { value: 'INV-1001' } });
    await fireEvent.input(getByLabelText('Quantity'), { target: { value: '10.000' } });
    await fireEvent.input(getByLabelText('Unit'), { target: { value: 'each' } });
    await fireEvent.click(getByRole('button', { name: 'Add ledger entry' }));
    expect(app.SaveCostEntry).toHaveBeenCalledWith(expect.objectContaining({
      item_name: 'Structural steel beam', sku: 'SKU-BEAM-1', supplier_name: 'Acme Structural Supply', invoice_reference: 'INV-1001', quantity: '10.000', unit: 'each',
    }));
  });

  it('searches the ledger and lets the user export a ledger attachments archive', async () => {
    const app = installApp({
      entries: [{ id: 'entry-1', cost_type_id: 'labor', kind: 'actual', cost_date: '2026-08-21', description: 'Steel beams', amount: '25.00' }],
    });
    const { findByText, getByLabelText, getByRole } = render(CostControlPanel);
    await findByText('Legacy budget rollup');
    await fireEvent.input(getByLabelText('Search ledger'), { target: { value: 'steel' } });
    await fireEvent.click(getByRole('button', { name: 'Search' }));
    expect(app.SearchCostEntries).toHaveBeenCalledWith('steel');

    await fireEvent.click(getByRole('button', { name: 'Export ledger attachments (.zip)' }));
    expect(app.ExportCostEntryAttachmentsZip).toHaveBeenCalledTimes(1);
  });

  it('keeps an active ledger search applied after a mutation reloads the panel', async () => {
    const app = installApp({
      entries: [{ id: 'entry-1', cost_type_id: 'labor', kind: 'actual', cost_date: '2026-08-21', description: 'Steel beams', amount: '25.00' }],
    });
    const { findByText, getAllByLabelText, getByLabelText, getByRole } = render(CostControlPanel);
    await findByText('Legacy budget rollup');
    await fireEvent.input(getByLabelText('Search ledger'), { target: { value: 'steel' } });
    await fireEvent.click(getByRole('button', { name: 'Search' }));
    expect(app.SearchCostEntries).toHaveBeenCalledWith('steel');
    app.SearchCostEntries.mockClear();
    app.ListCostEntries.mockClear();

    await fireEvent.input(getByLabelText('Cost item or reference'), { target: { value: 'More steel' } });
    await fireEvent.input(getAllByLabelText('Amount')[0], { target: { value: '10.00' } });
    await fireEvent.input(getByLabelText('Date'), { target: { value: '2026-08-21' } });
    await fireEvent.click(getByRole('button', { name: 'Add ledger entry' }));
    await waitFor(() => expect(app.SaveCostEntry).toHaveBeenCalledTimes(1));

    expect(app.SearchCostEntries).toHaveBeenCalledWith('steel');
    expect(app.ListCostEntries).not.toHaveBeenCalled();
  });

  it('shows the quantity aggregation table and lets a user attach a file to a ledger entry', async () => {
    const app = installApp({
      entries: [{ id: 'entry-1', cost_type_id: 'labor', kind: 'actual', cost_date: '2026-08-21', description: 'Steel beams', amount: '25.00' }],
    });
    app.AggregateCostEntryQuantities.mockResolvedValue([{ item_name: 'Structural steel beam', unit: 'each', total_quantity: '15.000', entry_count: 2 }]);
    const { findByText, getByRole } = render(CostControlPanel);
    await findByText('Legacy budget rollup');
    expect(await findByText('Quantity by item & unit')).toBeInTheDocument();
    expect(await findByText('15.000')).toBeInTheDocument();

    await fireEvent.click(getByRole('button', { name: 'Attachments' }));
    expect(app.ListCostEntryAttachments).toHaveBeenCalledWith('entry-1');
    await fireEvent.click(getByRole('button', { name: 'Attach file…' }));
    expect(app.AttachCostEntryFile).toHaveBeenCalledWith('entry-1');
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

  it('exports a printable project financial report without inventing a forecast', async () => {
    const app = installApp();
    const { findByRole, findByText } = render(CostControlPanel);
    await findByText('Legacy budget rollup');
    await fireEvent.click(await findByRole('button', { name: 'Export printable financial report' }));
    expect(app.ExportFinancialReportPDF).toHaveBeenCalledTimes(1);
    expect(await findByText(/does not calculate a forecast or remaining funding/i)).toBeInTheDocument();
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
