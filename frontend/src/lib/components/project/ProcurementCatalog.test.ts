// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render, waitFor } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import ProcurementCatalog from './ProcurementCatalog.svelte';

function installApp() {
  const app = {
    ListCatalogVendors: vi.fn(async () => [{ id: 'vendor-1', version: 1, name: 'Acme Supply', address: '10 Main St', phone: '555', fax: '556', email: 'orders@example.test', primary_contact: 'Dana', notes: '', archived: false, created_at: '', updated_at: '' }]),
    ListCatalogItems: vi.fn(async () => [{ id: 'item-1', version: 1, name: 'Concrete mix', sku: 'CM-42', kind: 'material', default_unit: 'bag', description: '', archived: false, created_at: '', updated_at: '' }]),
    SaveCatalogVendor: vi.fn(async (vendor) => ({ ...vendor, id: vendor.id || 'vendor-2', version: 1 })),
    SaveCatalogItem: vi.fn(async (item) => ({ ...item, id: item.id || 'item-2', version: 1 })),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
  return app;
}

describe('ProcurementCatalog', () => {
  it('searches the reusable supplier and item library and saves all supplier contact fields', async () => {
    const app = installApp();
    const ui = render(ProcurementCatalog);
    expect(await ui.findByText('Acme Supply')).toBeInTheDocument();
    expect(await ui.findByText('Concrete mix')).toBeInTheDocument();
    await fireEvent.click(ui.getByRole('button', { name: 'New supplier' }));
    const dialog = ui.getByRole('dialog');
    const fields = dialog.querySelectorAll('input');
    await fireEvent.input(fields[0], { target: { value: 'Northwind' } });
    await fireEvent.input(fields[1], { target: { value: 'Morgan' } });
    await fireEvent.input(fields[2], { target: { value: '555-0100' } });
    await fireEvent.input(fields[3], { target: { value: '555-0101' } });
    await fireEvent.input(fields[4], { target: { value: 'orders@northwind.test' } });
    await fireEvent.click(ui.getByRole('button', { name: 'Save supplier' }));
    await waitFor(() => expect(app.SaveCatalogVendor).toHaveBeenCalledTimes(1));
    expect(app.SaveCatalogVendor.mock.calls[0][0]).toMatchObject({ name: 'Northwind', primary_contact: 'Morgan', phone: '555-0100', fax: '555-0101', email: 'orders@northwind.test' });
  });
});
