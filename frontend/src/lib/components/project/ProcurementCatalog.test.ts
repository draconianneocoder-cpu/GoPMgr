// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProcurementCatalog from './ProcurementCatalog.svelte';
import { autosave } from '../../autosave.svelte';
import {
  cancelNavigation,
  navigation,
  requestNavigation,
  saveAndContinueNavigation,
  session as navSession,
} from '../../session.svelte';

const desktopApiMock = vi.hoisted(() => {
  const state: { current: Record<string, ReturnType<typeof vi.fn>> } = { current: {} };
  return state;
});
vi.mock('../../desktop-api', () => ({ get desktopApi() { return desktopApiMock.current; } }));

function installApp() {
  const app = {
    ListCatalogVendors: vi.fn(async () => [{ id: 'vendor-1', version: 1, name: 'Acme Supply', address: '10 Main St', phone: '555', fax: '556', email: 'orders@example.test', primary_contact: 'Dana', notes: '', archived: false, created_at: '', updated_at: '' }]),
    ListCatalogItems: vi.fn(async () => [{ id: 'item-1', version: 1, name: 'Concrete mix', sku: 'CM-42', kind: 'material', default_unit: 'bag', description: '', archived: false, created_at: '', updated_at: '' }]),
    SaveCatalogVendor: vi.fn(async (vendor) => ({ ...vendor, id: vendor.id || 'vendor-2', version: 1 })),
    SaveCatalogItem: vi.fn(async (item) => ({ ...item, id: item.id || 'item-2', version: 1 })),
  };
  desktopApiMock.current = app;
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


// Owner decision 2026-11-24 ("All five forms"): an open supplier or item
// dialog asks before it is discarded by leaving the view.
describe('ProcurementCatalog unsaved dialog', () => {
  afterEach(() => {
    autosave.discardAll();
    cancelNavigation();
    navigation.saving = false;
    navigation.error = '';
  });

  it('asks before leaving with a typed supplier, then saves it', async () => {
    const app = installApp();
    const ui = render(ProcurementCatalog);
    await fireEvent.click(await ui.findByRole('button', { name: 'New supplier' }));
    requestNavigation('project_settings');
    expect(navigation.pending).toBeNull(); // an empty dialog holds nothing
    await waitFor(() => expect(navSession.view).toBe('project_settings'));

    navSession.view = 'procurement_catalog' as typeof navSession.view;
    await fireEvent.input(ui.getByLabelText('Name'), { target: { value: 'Bolt Co' } });
    requestNavigation('project_settings');
    expect(navigation.pending?.view).toBe('project_settings');

    await saveAndContinueNavigation();
    expect(app.SaveCatalogVendor).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({ id: '', name: 'Bolt Co' }));
    expect(navSession.view).toBe('project_settings');
    expect(ui.queryByRole('dialog')).toBeNull();
  });
});

// Keyboard focus moves into the dialog when it opens and returns to the
// control that opened it when it closes (modalFocus, owned by
// modal-focus.test.ts; 2026-11-24 dialog focus audit).
describe('ProcurementCatalog dialog focus', () => {
  it('names the dialog, takes focus, closes on Escape and returns focus to New supplier', async () => {
    installApp();
    const ui = render(ProcurementCatalog);
    const opener = await ui.findByRole('button', { name: 'New supplier' });
    opener.focus();
    await fireEvent.click(opener);
    const dialog = ui.getByRole('dialog', { name: 'New supplier' });
    await waitFor(() => expect(dialog.contains(document.activeElement)).toBe(true));

    await fireEvent.keyDown(dialog, { key: 'Escape' });
    expect(ui.queryByRole('dialog')).toBeNull();
    expect(document.activeElement).toBe(opener);
  });
});
