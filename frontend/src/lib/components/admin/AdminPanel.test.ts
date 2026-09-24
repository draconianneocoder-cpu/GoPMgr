// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor, within } from '@testing-library/svelte';

import AdminPanel from './AdminPanel.svelte';
import { session } from '../../session.svelte';

function account(username: string, extra: Partial<Account> = {}): Account {
  return {
    username,
    display_name: username,
    data_dir: `/tmp/gopmgr/${username}`,
    created_at: '',
    last_login: '',
    is_admin: false,
    disabled: false,
    ...extra,
  };
}

let app: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  app = {
    AdminListUsers: vi.fn(async () => [account('alice', { is_admin: true }), account('bob')]),
    AdminListAccountEvents: vi.fn(async () => []),
    AdminSetUserDisabled: vi.fn(async () => undefined),
    AdminPurgeUser: vi.fn(async () => undefined),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
  session.user = account('alice', { is_admin: true });
});

afterEach(() => {
  cleanup();
  session.user = null;
  vi.restoreAllMocks();
});

// The row holding bob's actions: the username and display-name cells both
// read "bob", so find it from the cell with the font-mono username.
function bobRow(utils: ReturnType<typeof render>) {
  return utils.getAllByText('bob', { selector: 'td.font-mono' })[0].closest('tr') as HTMLElement;
}

describe('AdminPanel account removal', () => {
  it('offers Disable and Delete permanently for other accounts, and nothing for your own', async () => {
    const utils = render(AdminPanel);
    await waitFor(() => expect(utils.getByRole('button', { name: 'Disable account bob' })).toBeInTheDocument());

    expect(utils.getByRole('button', { name: 'Delete account bob permanently' })).toBeInTheDocument();
    expect(utils.queryByRole('button', { name: 'Disable account alice' })).not.toBeInTheDocument();
    expect(utils.queryByRole('button', { name: 'Delete account alice permanently' })).not.toBeInTheDocument();
  });

  it('disables an account only after confirmation and says its projects are kept', async () => {
    const utils = render(AdminPanel);
    await fireEvent.click(await utils.findByRole('button', { name: 'Disable account bob' }));

    expect(app.AdminSetUserDisabled).not.toHaveBeenCalled();
    expect(within(bobRow(utils)).getByText('Disable? Projects are kept.')).toBeInTheDocument();

    await fireEvent.click(within(bobRow(utils)).getByRole('button', { name: 'Confirm' }));
    await waitFor(() => expect(app.AdminSetUserDisabled).toHaveBeenCalledWith('bob', true));
    await waitFor(() => expect(app.AdminListUsers).toHaveBeenCalledTimes(2));
  });

  it('shows a disabled account and lets an administrator enable it', async () => {
    app.AdminListUsers.mockResolvedValue([account('alice', { is_admin: true }), account('bob', { disabled: true })]);
    const utils = render(AdminPanel);

    await fireEvent.click(await utils.findByRole('button', { name: 'Enable account bob' }));
    expect(within(bobRow(utils)).getByText('Disabled')).toBeInTheDocument();
    await fireEvent.click(within(bobRow(utils)).getByRole('button', { name: 'Confirm' }));
    await waitFor(() => expect(app.AdminSetUserDisabled).toHaveBeenCalledWith('bob', false));
  });

  it('deletes permanently only after the exact username is typed, and lists what is lost', async () => {
    const utils = render(AdminPanel);
    await fireEvent.click(await utils.findByRole('button', { name: 'Delete account bob permanently' }));

    const panel = utils.getByRole('group', { name: 'Permanently delete bob' });
    expect(panel).toHaveTextContent(/projects \(including encrypted ones\), certificates, exports, and recovery codes/);
    expect(panel).toHaveTextContent(/can't be\s+undone/);
    const input = within(panel).getByLabelText(/Type\s+bob\s+to confirm/);
    const confirm = within(panel).getByRole('button', { name: 'Delete permanently' });
    expect(confirm).toBeDisabled();

    for (const typed of ['Bob', 'bob ', 'bo']) {
      await fireEvent.input(input, { target: { value: typed } });
      expect(confirm).toBeDisabled();
    }
    await fireEvent.click(confirm);
    expect(app.AdminPurgeUser).not.toHaveBeenCalled();

    await fireEvent.input(input, { target: { value: 'bob' } });
    expect(confirm).toBeEnabled();
    await fireEvent.click(confirm);
    await waitFor(() => expect(app.AdminPurgeUser).toHaveBeenCalledWith('bob', 'bob'));
    expect(app.AdminPurgeUser).toHaveBeenCalledOnce();
  });

  it('cancels a permanent deletion without calling the backend', async () => {
    const utils = render(AdminPanel);
    await fireEvent.click(await utils.findByRole('button', { name: 'Delete account bob permanently' }));
    const panel = utils.getByRole('group', { name: 'Permanently delete bob' });
    await fireEvent.input(within(panel).getByLabelText(/Type\s+bob\s+to confirm/), { target: { value: 'bob' } });
    await fireEvent.click(within(panel).getByRole('button', { name: 'Cancel' }));

    expect(utils.queryByRole('group', { name: 'Permanently delete bob' })).not.toBeInTheDocument();
    expect(app.AdminPurgeUser).not.toHaveBeenCalled();
  });
});

describe('AdminPanel account history', () => {
  it('lists disable, enable, delete, and incomplete-deletion events', async () => {
    app.AdminListAccountEvents.mockResolvedValue([
      { id: 4, occurred_at: '2026-09-24T10:03:00Z', actor: 'alice', username: 'carol', action: 'folder_not_removed', detail: '/tmp/gopmgr/carol: permission denied' },
      { id: 3, occurred_at: '2026-09-24T10:02:00Z', actor: 'alice', username: 'carol', action: 'purged', detail: '' },
      { id: 2, occurred_at: '2026-09-24T10:01:00Z', actor: 'alice', username: 'bob', action: 'enabled', detail: '' },
      { id: 1, occurred_at: '2026-09-24T10:00:00Z', actor: 'alice', username: 'bob', action: 'disabled', detail: '' },
    ]);
    const utils = render(AdminPanel);
    const history = await utils.findByRole('region', { name: 'Account history' });

    await waitFor(() => expect(within(history).getAllByRole('listitem')).toHaveLength(4));
    const items = within(history).getAllByRole('listitem').map((li) => li.textContent?.replace(/\s+/g, ' '));
    expect(items[0]).toContain('alice could not fully remove the folder of carol');
    expect(items[0]).toContain('permission denied');
    expect(items[1]).toContain('alice permanently deleted carol');
    expect(items[2]).toContain('alice enabled bob');
    expect(items[3]).toContain('alice disabled bob');
  });

  it('says when there is no history, and reports a failed load', async () => {
    const utils = render(AdminPanel);
    expect(await utils.findByText('No accounts have been disabled, enabled, or deleted.')).toBeInTheDocument();
    cleanup();

    app.AdminListAccountEvents.mockRejectedValue(new Error('system database unavailable'));
    const failed = render(AdminPanel);
    expect(await failed.findByText(/Could not load account history/)).toBeInTheDocument();
  });
});
