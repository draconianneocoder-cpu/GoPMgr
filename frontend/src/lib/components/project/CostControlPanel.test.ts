// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render, waitFor } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import CostControlPanel from './CostControlPanel.svelte';

type CostControlFixture = {
  baselines?: Array<Record<string, string | number>>;
  types?: Array<Record<string, string | boolean>>;
  entries?: Array<Record<string, string>>;
  catalogItems?: Array<Record<string, string | boolean | number>>;
  catalogVendors?: Array<Record<string, string | boolean | number>>;
  failFirstLoad?: boolean;
  mutationDisabledReason?: string;
};

function installApp({ baselines = [], types = [{ id: 'labor', project_id: 'p', code: 'labor', name: 'Labor', attribution: 'direct', behavior: 'variable', treatment: 'opex', active: true }], entries = [], catalogItems = [], catalogVendors = [], failFirstLoad = false, mutationDisabledReason = '' }: CostControlFixture = {}) {
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
    ListCatalogItems: vi.fn(async () => catalogItems),
    ListCatalogVendors: vi.fn(async () => catalogVendors),
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

  it('gives the empty ledger and the searched-empty ledger distinct guidance', async () => {
    const app = installApp();
    const { findByText, getByLabelText, getByRole, queryByText } = render(CostControlPanel);
    expect(await findByText('No ledger entries yet.')).toBeInTheDocument();
    expect(queryByText('No ledger entries match this search.')).not.toBeInTheDocument();
    expect(getByLabelText('Search ledger')).toHaveAttribute('aria-describedby', 'ledger-entry-help');
    await fireEvent.input(getByLabelText('Search ledger'), { target: { value: 'missing' } });
    expect(queryByText('No ledger entries match this search.')).not.toBeInTheDocument();
    await fireEvent.click(getByRole('button', { name: 'Search' }));
    await waitFor(() => expect(app.SearchCostEntries).toHaveBeenCalledWith('missing'));
    expect(await findByText('No ledger entries match this search.')).toBeInTheDocument();
    expect(queryByText('No ledger entries yet.')).not.toBeInTheDocument();
  });

  it('shows catalog no-match guidance only after a completed lookup', async () => {
    const app = installApp();
    const { findByText, getByLabelText, getByRole, queryByText } = render(CostControlPanel);
    await findByText('Legacy budget rollup');
    await fireEvent.input(getByLabelText('Find catalog item'), { target: { value: 'missing item' } });
    expect(queryByText('No active catalog items match this search.')).not.toBeInTheDocument();
    await fireEvent.click(getByRole('button', { name: 'Find catalog item' }));
    expect(await findByText('No active catalog items match this search.')).toBeInTheDocument();
    expect(app.ListCatalogItems).toHaveBeenCalledWith('missing item', false);
  });

  it('shows request-scoped catalog busy feedback for item and supplier searches', async () => {
    let resolveItems: (rows: Array<Record<string, string | boolean | number>>) => void;
    let resolveSuppliers: (rows: Array<Record<string, string | boolean | number>>) => void;
    const pendingItems = new Promise<Array<Record<string, string | boolean | number>>>((resolve) => { resolveItems = resolve; });
    const pendingSuppliers = new Promise<Array<Record<string, string | boolean | number>>>((resolve) => { resolveSuppliers = resolve; });
    const app = installApp();
    app.ListCatalogItems.mockImplementationOnce(() => pendingItems);
    app.ListCatalogVendors.mockImplementationOnce(() => pendingSuppliers);
    const { findByText, getByLabelText, getByRole } = render(CostControlPanel);
    await findByText('Legacy budget rollup');

    await fireEvent.input(getByLabelText('Find catalog item'), { target: { value: 'steel' } });
    await fireEvent.click(getByRole('button', { name: 'Find catalog item' }));
    expect(getByRole('button', { name: 'Searching…' })).toBeDisabled();
    resolveItems!([]);
    await waitFor(() => expect(getByRole('button', { name: 'Find catalog item' })).toBeEnabled());

    await fireEvent.input(getByLabelText('Find catalog supplier'), { target: { value: 'acme' } });
    await fireEvent.click(getByRole('button', { name: 'Find catalog supplier' }));
    expect(getByRole('button', { name: 'Searching…' })).toBeDisabled();
    resolveSuppliers!([]);
    await waitFor(() => expect(getByRole('button', { name: 'Find catalog supplier' })).toBeEnabled());
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

  it('keeps direct procurement entry available without catalog assistance', async () => {
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

  it('copies catalog defaults into editable project snapshots without supplier contact data or a catalog relationship', async () => {
    const privateAddress = 'PRIVATE-ADDRESS-ONLY';
    const privatePhone = 'PRIVATE-PHONE-ONLY';
    const privateEmail = 'PRIVATE-EMAIL-ONLY';
    const app = installApp({
      catalogItems: [{ id: 'item-1', version: 4, name: 'Concrete mix', sku: 'CM-42', default_unit: 'bag', kind: 'material', description: 'Private item note', archived: false }],
      catalogVendors: [{ id: 'supplier-1', version: 7, name: 'Acme Supply', address: privateAddress, phone: privatePhone, fax: 'PRIVATE-FAX-ONLY', email: privateEmail, primary_contact: 'PRIVATE-CONTACT-ONLY', notes: 'PRIVATE-NOTES-ONLY', archived: false }],
    });
    const { findByText, getAllByLabelText, getByLabelText, getByRole, queryByText } = render(CostControlPanel);
    await findByText('Legacy budget rollup');

    await fireEvent.input(getByLabelText('Cost item or reference'), { target: { value: 'Keep this description' } });
    await fireEvent.input(getAllByLabelText('Amount')[0], { target: { value: '1200.00' } });
    await fireEvent.input(getByLabelText('Date'), { target: { value: '2026-08-28' } });
    await fireEvent.input(getByLabelText('Quantity'), { target: { value: '4.000' } });
    await fireEvent.input(getByLabelText('Invoice reference'), { target: { value: 'INV-KEEP' } });

    await fireEvent.input(getByLabelText('Find catalog item'), { target: { value: 'concrete' } });
    await fireEvent.click(getByRole('button', { name: 'Find catalog item' }));
    await waitFor(() => expect(app.ListCatalogItems).toHaveBeenCalledWith('concrete', false));
    await fireEvent.change(getByLabelText('Copy catalog item defaults'), { target: { value: 'item-1' } });
    expect(getByLabelText('Item name')).toHaveValue('Concrete mix');
    expect(getByLabelText('SKU')).toHaveValue('CM-42');
    expect(getByLabelText('Unit')).toHaveValue('bag');
    expect(getByLabelText('Cost item or reference')).toHaveValue('Keep this description');
    expect(getAllByLabelText('Amount')[0]).toHaveValue('1200.00');
    expect(getByLabelText('Date')).toHaveValue('2026-08-28');
    expect(getByLabelText('Quantity')).toHaveValue('4.000');
    expect(getByLabelText('Invoice reference')).toHaveValue('INV-KEEP');

    await fireEvent.input(getByLabelText('Find catalog supplier'), { target: { value: 'acme' } });
    await fireEvent.click(getByRole('button', { name: 'Find catalog supplier' }));
    await waitFor(() => expect(app.ListCatalogVendors).toHaveBeenCalledWith('acme', false));
    await fireEvent.change(getByLabelText('Copy catalog supplier name'), { target: { value: 'supplier-1' } });
    expect(getByLabelText('Supplier')).toHaveValue('Acme Supply');
    expect(queryByText(privateAddress)).not.toBeInTheDocument();
    expect(queryByText(privatePhone)).not.toBeInTheDocument();
    expect(queryByText(privateEmail)).not.toBeInTheDocument();

    await fireEvent.input(getByLabelText('Item name'), { target: { value: 'Concrete mix, project grade' } });
    await fireEvent.input(getByLabelText('SKU'), { target: { value: 'CM-42-PROJ' } });
    await fireEvent.input(getByLabelText('Supplier'), { target: { value: 'Acme Supply, project desk' } });
    await fireEvent.input(getByLabelText('Unit'), { target: { value: 'pallet' } });
    await fireEvent.input(getByLabelText('Cost item or reference'), { target: { value: 'Concrete delivery' } });
    await fireEvent.click(getByRole('button', { name: 'Add ledger entry' }));
    await waitFor(() => expect(app.SaveCostEntry).toHaveBeenCalledTimes(1));

    const saved = app.SaveCostEntry.mock.calls[0][0];
    expect(Object.keys(saved).sort()).toEqual(['amount', 'cost_date', 'cost_type_id', 'description', 'id', 'invoice_reference', 'item_name', 'kind', 'quantity', 'sku', 'supplier_name', 'unit']);
    expect(saved).toEqual({ id: '', cost_type_id: 'labor', kind: 'planned', amount: '1200.00', description: 'Concrete delivery', cost_date: '2026-08-28', quantity: '4.000', unit: 'pallet', item_name: 'Concrete mix, project grade', sku: 'CM-42-PROJ', supplier_name: 'Acme Supply, project desk', invoice_reference: 'INV-KEEP' });
    expect(JSON.stringify(saved)).not.toContain(privateAddress);
    expect(JSON.stringify(saved)).not.toContain(privatePhone);
    expect(JSON.stringify(saved)).not.toContain(privateEmail);
    expect(JSON.stringify(saved)).not.toContain('PRIVATE-CONTACT-ONLY');
    expect(JSON.stringify(saved)).not.toContain('supplier-1');
    expect(JSON.stringify(saved)).not.toContain('item-1');
    expect(JSON.stringify(saved)).not.toContain('version');

    await waitFor(() => expect(getByLabelText('Find catalog item')).toHaveValue(''));
    expect(getByLabelText('Find catalog supplier')).toHaveValue('');
    expect(getByLabelText('Copy catalog item defaults')).toHaveValue('');
    expect(getByLabelText('Copy catalog supplier name')).toHaveValue('');
    expect(getByLabelText('Item name')).toHaveValue('');
    expect(getByLabelText('SKU')).toHaveValue('');
    expect(getByLabelText('Supplier')).toHaveValue('');
    expect(getByLabelText('Unit')).toHaveValue('');
  });

  it('filters malformed archived catalog results before they can be copied', async () => {
    const app = installApp({
      catalogItems: [{ id: 'item-active', name: 'Active item', sku: 'A-1', default_unit: 'each', archived: false }, { id: 'item-archived', name: 'Archived item', sku: 'Z-9', default_unit: 'each', archived: true }],
      catalogVendors: [{ id: 'supplier-active', name: 'Active supplier', archived: false }, { id: 'supplier-archived', name: 'Archived supplier', archived: true }],
    });
    const { findByRole, findByText, getByLabelText, getByRole, queryByRole } = render(CostControlPanel);
    await findByText('Legacy budget rollup');
    await fireEvent.input(getByLabelText('Find catalog item'), { target: { value: 'item' } });
    await fireEvent.click(getByRole('button', { name: 'Find catalog item' }));
    expect(await findByRole('option', { name: /Active item/ })).toBeInTheDocument();
    expect(queryByRole('option', { name: /Archived item/ })).not.toBeInTheDocument();
    expect(app.ListCatalogItems).toHaveBeenCalledWith('item', false);

    await fireEvent.input(getByLabelText('Find catalog supplier'), { target: { value: 'supplier' } });
    await fireEvent.click(getByRole('button', { name: 'Find catalog supplier' }));
    expect(await findByRole('option', { name: 'Active supplier' })).toBeInTheDocument();
    expect(queryByRole('option', { name: 'Archived supplier' })).not.toBeInTheDocument();
    expect(app.ListCatalogVendors).toHaveBeenCalledWith('supplier', false);
  });

  it('treats empty catalog results as no matches rather than a lookup failure', async () => {
    const app = installApp();
    const { findByText, getByLabelText, getByRole, queryByText } = render(CostControlPanel);
    await findByText('Legacy budget rollup');
    await fireEvent.input(getByLabelText('Find catalog item'), { target: { value: 'no such item' } });
    await fireEvent.click(getByRole('button', { name: 'Find catalog item' }));
    await waitFor(() => expect(app.ListCatalogItems).toHaveBeenCalledWith('no such item', false));
    expect(getByLabelText('Copy catalog item defaults')).toBeDisabled();
    expect(queryByText(/Catalog item lookup is unavailable/)).not.toBeInTheDocument();

    await fireEvent.input(getByLabelText('Find catalog supplier'), { target: { value: 'no such supplier' } });
    await fireEvent.click(getByRole('button', { name: 'Find catalog supplier' }));
    await waitFor(() => expect(app.ListCatalogVendors).toHaveBeenCalledWith('no such supplier', false));
    expect(getByLabelText('Copy catalog supplier name')).toBeDisabled();
    expect(queryByText(/Catalog supplier lookup is unavailable/)).not.toBeInTheDocument();
  });

  it('ignores a stale catalog lookup result after a newer query completes', async () => {
    let resolveFirst: (rows: Array<Record<string, string | boolean | number>>) => void;
    const firstLookup = new Promise<Array<Record<string, string | boolean | number>>>((resolve) => { resolveFirst = resolve; });
    const app = installApp();
    app.ListCatalogItems
      .mockImplementationOnce(() => firstLookup)
      .mockResolvedValueOnce([{ id: 'new-item', name: 'New result', sku: 'NEW', default_unit: 'each', archived: false }]);
    const { findByRole, findByText, getByLabelText, getByRole, queryByRole } = render(CostControlPanel);
    await findByText('Legacy budget rollup');

    await fireEvent.input(getByLabelText('Find catalog item'), { target: { value: 'old' } });
    await fireEvent.click(getByRole('button', { name: 'Find catalog item' }));
    await fireEvent.input(getByLabelText('Find catalog item'), { target: { value: 'new' } });
    await fireEvent.click(getByRole('button', { name: 'Find catalog item' }));
    expect(await findByRole('option', { name: /New result/ })).toBeInTheDocument();

    resolveFirst!([{ id: 'old-item', name: 'Old result', sku: 'OLD', default_unit: 'each', archived: false }]);
    await Promise.resolve();
    expect(queryByRole('option', { name: /Old result/ })).not.toBeInTheDocument();
    expect(app.ListCatalogItems).toHaveBeenNthCalledWith(1, 'old', false);
    expect(app.ListCatalogItems).toHaveBeenNthCalledWith(2, 'new', false);
  });

  it('does not repopulate a reset form when a pending catalog lookup resolves after save', async () => {
    let resolveItems: (rows: Array<Record<string, string | boolean | number>>) => void;
    let resolveSuppliers: (rows: Array<Record<string, string | boolean | number>>) => void;
    const pendingItems = new Promise<Array<Record<string, string | boolean | number>>>((resolve) => { resolveItems = resolve; });
    const pendingSuppliers = new Promise<Array<Record<string, string | boolean | number>>>((resolve) => { resolveSuppliers = resolve; });
    const app = installApp();
    app.ListCatalogItems.mockImplementationOnce(() => pendingItems);
    app.ListCatalogVendors.mockImplementationOnce(() => pendingSuppliers);
    const { findByText, getAllByLabelText, getByLabelText, getByRole, queryByRole } = render(CostControlPanel);
    await findByText('Legacy budget rollup');

    await fireEvent.input(getByLabelText('Find catalog item'), { target: { value: 'pending item' } });
    await fireEvent.click(getByRole('button', { name: 'Find catalog item' }));
    await fireEvent.input(getByLabelText('Find catalog supplier'), { target: { value: 'pending supplier' } });
    await fireEvent.click(getByRole('button', { name: 'Find catalog supplier' }));
    await fireEvent.input(getByLabelText('Cost item or reference'), { target: { value: 'Manual entry while lookups are pending' } });
    await fireEvent.input(getAllByLabelText('Amount')[0], { target: { value: '20.00' } });
    await fireEvent.input(getByLabelText('Date'), { target: { value: '2026-08-28' } });
    await fireEvent.click(getByRole('button', { name: 'Add ledger entry' }));
    await waitFor(() => expect(app.SaveCostEntry).toHaveBeenCalledTimes(1));

    resolveItems!([{ id: 'stale-item', name: 'Stale item', sku: 'STALE', default_unit: 'each', archived: false }]);
    resolveSuppliers!([{ id: 'stale-supplier', name: 'Stale supplier', archived: false }]);
    await Promise.resolve();
    await Promise.resolve();
    expect(queryByRole('option', { name: /Stale item/ })).not.toBeInTheDocument();
    expect(queryByRole('option', { name: 'Stale supplier' })).not.toBeInTheDocument();
    expect(getByLabelText('Find catalog item')).toHaveValue('');
    expect(getByLabelText('Find catalog supplier')).toHaveValue('');
  });

  it('keeps manual Cost Control entry available after a catalog lookup fails', async () => {
    const app = installApp();
    app.ListCatalogItems.mockRejectedValueOnce(new Error('catalog is unavailable'));
    const { findByText, getAllByLabelText, getByLabelText, getByRole, queryByRole } = render(CostControlPanel);
    await findByText('Legacy budget rollup');
    await fireEvent.input(getByLabelText('Find catalog item'), { target: { value: 'concrete' } });
    await fireEvent.click(getByRole('button', { name: 'Find catalog item' }));
    expect(await findByText('Catalog item lookup is unavailable. You can enter procurement detail manually.')).toBeInTheDocument();
    expect(queryByRole('button', { name: 'Retry Cost Control' })).not.toBeInTheDocument();

    await fireEvent.input(getByLabelText('Cost item or reference'), { target: { value: 'Manual concrete delivery' } });
    await fireEvent.input(getAllByLabelText('Amount')[0], { target: { value: '20.00' } });
    await fireEvent.input(getByLabelText('Date'), { target: { value: '2026-08-28' } });
    await fireEvent.input(getByLabelText('Item name'), { target: { value: 'Manual concrete' } });
    await fireEvent.click(getByRole('button', { name: 'Add ledger entry' }));
    await waitFor(() => expect(app.SaveCostEntry).toHaveBeenCalledWith(expect.objectContaining({ description: 'Manual concrete delivery', item_name: 'Manual concrete' })));
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
    expect(getByLabelText('Find catalog item')).toBeDisabled();
    expect(getByLabelText('Copy catalog item defaults')).toBeDisabled();
    expect(getByLabelText('Find catalog supplier')).toBeDisabled();
    expect(getByLabelText('Copy catalog supplier name')).toBeDisabled();
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
