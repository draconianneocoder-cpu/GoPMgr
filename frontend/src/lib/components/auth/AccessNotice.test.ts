// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';

import AccessNotice from './AccessNotice.svelte';

let app: Record<string, ReturnType<typeof vi.fn>>;

function access(id: number, reason: string): AccountEvent {
  return { id, occurred_at: '2026-10-02T10:00:00Z', actor: 'alice', username: 'bob', action: 'admin_access', detail: reason };
}

beforeEach(() => {
  app = {
    MyDataAccessNotices: vi.fn(async () => [access(7, 'Ticket 7'), access(4, 'Ticket 4')]),
    AcknowledgeDataAccessNotices: vi.fn(async () => undefined),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('AccessNotice', () => {
  it('lists who opened your data, when, and why', async () => {
    const utils = render(AccessNotice);
    const region = await utils.findByRole('region', { name: 'Administrators opened your data' });
    expect(region.textContent).toContain('Reason: Ticket 7');
    expect(region.textContent).toContain('Reason: Ticket 4');
    expect(region.textContent).toContain('alice opened your data on');
    expect(region.textContent).toContain('read only');
  });

  it('acknowledges only what it showed, then disappears', async () => {
    app.MyDataAccessNotices.mockResolvedValueOnce([access(7, 'Ticket 7'), access(4, 'Ticket 4')]).mockResolvedValueOnce([]);
    const utils = render(AccessNotice);
    await fireEvent.click(await utils.findByRole('button', { name: "I've read this" }));
    expect(app.AcknowledgeDataAccessNotices).toHaveBeenCalledWith(7);
    await waitFor(() => expect(utils.queryByRole('region')).not.toBeInTheDocument());
  });

  it('keeps showing an access recorded while the notice was open', async () => {
    app.MyDataAccessNotices.mockResolvedValueOnce([access(4, 'Ticket 4')]).mockResolvedValueOnce([access(9, 'Ticket 9')]);
    const utils = render(AccessNotice);
    await fireEvent.click(await utils.findByRole('button', { name: "I've read this" }));
    expect(app.AcknowledgeDataAccessNotices).toHaveBeenCalledWith(4);
    expect((await utils.findByRole('region', { name: 'An administrator opened your data' })).textContent).toContain('Ticket 9');
  });

  it('says so, with a retry, when it cannot check', async () => {
    app.MyDataAccessNotices.mockRejectedValueOnce(new Error('system database unavailable')).mockResolvedValueOnce([access(4, 'Ticket 4')]);
    const utils = render(AccessNotice);
    expect((await utils.findByRole('alert')).textContent).toContain("couldn't check whether an administrator opened your data");
    await fireEvent.click(utils.getByRole('button', { name: 'Try again' }));
    expect(await utils.findByText('Reason: Ticket 4')).toBeInTheDocument();
  });

  it('stays when saving the acknowledgement fails', async () => {
    app.AcknowledgeDataAccessNotices.mockRejectedValueOnce(new Error('disk full'));
    const utils = render(AccessNotice);
    await fireEvent.click(await utils.findByRole('button', { name: "I've read this" }));
    expect((await utils.findByRole('alert')).textContent).toContain('disk full');
    expect(utils.getByRole('region', { name: 'Administrators opened your data' })).toBeInTheDocument();
  });

  it('shows nothing when no one opened your data', async () => {
    app.MyDataAccessNotices.mockResolvedValue([]);
    const utils = render(AccessNotice);
    await waitFor(() => expect(app.MyDataAccessNotices).toHaveBeenCalled());
    expect(utils.queryByRole('region')).not.toBeInTheDocument();
  });
});
