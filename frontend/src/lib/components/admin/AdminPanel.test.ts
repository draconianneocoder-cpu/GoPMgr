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

  it('reports an incomplete deletion as a deletion, not as a failure', async () => {
    const toasts = await import('../../toast.svelte');
    const toastSpy = vi.spyOn(toasts, 'showToast');
    app.AdminPurgeUser.mockRejectedValue(new Error(
      'the account was deleted, but part of its folder could not be removed; remove it by hand (/tmp/gopmgr/bob: permission denied)',
    ));
    const utils = render(AdminPanel);
    await fireEvent.click(await utils.findByRole('button', { name: 'Delete account bob permanently' }));
    const panel = utils.getByRole('group', { name: 'Permanently delete bob' });
    await fireEvent.input(within(panel).getByLabelText(/Type\s+bob\s+to confirm/), { target: { value: 'bob' } });
    await fireEvent.click(within(panel).getByRole('button', { name: 'Delete permanently' }));

    await waitFor(() => expect(toastSpy).toHaveBeenCalled());
    const [message] = toastSpy.mock.calls.at(-1)!;
    expect(message).toMatch(/^The account was deleted, but part of its folder could not be removed/);
    expect(message).toContain('/tmp/gopmgr/bob');
    expect(message).not.toMatch(/Delete failed/);
    expect(utils.queryByRole('group', { name: 'Permanently delete bob' })).not.toBeInTheDocument();
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
  it('describes creations and role changes, and falls back for an unknown action', async () => {
    app.AdminListAccountEvents.mockResolvedValue([
      { id: 7, occurred_at: '2026-09-28T10:06:00Z', actor: 'alice', username: 'bob', action: 'renamed', detail: '' },
      { id: 6, occurred_at: '2026-09-28T10:05:00Z', actor: 'alice', username: 'bob', action: 'demoted', detail: '' },
      { id: 5, occurred_at: '2026-09-28T10:04:00Z', actor: 'alice', username: 'bob', action: 'promoted', detail: '' },
      { id: 4, occurred_at: '2026-09-28T10:03:00Z', actor: 'dave', username: 'dave', action: 'promoted', detail: 'claimed with no administrator on the machine' },
      { id: 3, occurred_at: '2026-09-28T10:02:00Z', actor: 'alice', username: 'bob', action: 'created', detail: 'standard' },
      { id: 1, occurred_at: '2026-09-28T10:00:00Z', actor: 'alice', username: 'alice', action: 'created', detail: 'administrator' },
    ]);
    const utils = render(AdminPanel);
    const history = await utils.findByRole('region', { name: 'Account history' });
    await waitFor(() => expect(within(history).getAllByRole('listitem')).toHaveLength(6));
    const items = within(history).getAllByRole('listitem').map((li) => li.textContent?.replace(/\s+/g, ' ').trim());
    expect(items[0]).toContain('alice renamed bob');
    expect(items[1]).toContain("alice removed bob's administrator role");
    expect(items[2]).toContain('alice made bob an administrator');
    expect(items[3]).toContain('dave claimed the administrator role');
    expect(items[3]).not.toContain('claimed with no administrator on the machine');
    expect(items[4]).toContain('alice created bob (standard)');
    expect(items[5]).toContain('alice created the first account (administrator)');
  });

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

  it('lists administrator key events in plain words', async () => {
    app.AdminListAccountEvents.mockResolvedValue([
      { id: 6, occurred_at: '2026-10-02T10:05:00Z', actor: 'alice', username: 'bob', action: 'escrow_rotated', detail: '' },
      { id: 5, occurred_at: '2026-10-02T10:04:00Z', actor: 'alice', username: 'bob', action: 'admin_access', detail: '' },
      { id: 4, occurred_at: '2026-10-02T10:03:00Z', actor: 'alice', username: 'dave', action: 'personal_key_trusted', detail: '' },
      { id: 3, occurred_at: '2026-10-02T10:02:00Z', actor: 'carol', username: 'carol', action: 'personal_key_repaired', detail: 'the stored key could not be opened, so a new one was made' },
      { id: 2, occurred_at: '2026-10-02T10:01:00Z', actor: 'bob', username: 'bob', action: 'escrow_reenrolled', detail: '' },
      { id: 1, occurred_at: '2026-10-02T10:00:00Z', actor: 'bob', username: 'bob', action: 'escrow_key_mismatch', detail: 'the administrator key in system.db is not the one this account trusts, so nothing was sealed' },
    ]);
    const utils = render(AdminPanel);
    const history = await utils.findByRole('region', { name: 'Account history' });

    await waitFor(() => expect(within(history).getAllByRole('listitem')).toHaveLength(6));
    const items = within(history).getAllByRole('listitem').map((li) => li.textContent?.replace(/\s+/g, ' '));
    expect(items[0]).toContain('alice replaced the administrator key');
    expect(items[1]).toContain("alice opened bob's data as an administrator");
    expect(items[2]).toContain("alice accepted dave's account key without an earlier check");
    expect(items[3]).toContain("carol's account key was changed outside GoPMgr and was repaired at sign-in");
    expect(items[3]).toContain('a new one was made');
    expect(items[4]).toContain("bob's key records were missing and were made again at sign-in");
    expect(items[5]).toContain('A key check failed for bob');
    expect(items[5]).toContain('nothing was sealed');
    for (const item of items) expect(item).not.toMatch(/escrow|_/);
  });

  it('says when there is no history, and reports a failed load', async () => {
    const utils = render(AdminPanel);
    expect(await utils.findByText('No account changes yet.')).toBeInTheDocument();
    cleanup();

    app.AdminListAccountEvents.mockRejectedValue(new Error('system database unavailable'));
    const failed = render(AdminPanel);
    expect(await failed.findByText(/Could not load account history/)).toBeInTheDocument();
  });
});

describe('AdminPanel new-account recovery codes', () => {
  async function createCarol(utils: ReturnType<typeof render>) {
    await fireEvent.click(await utils.findByRole('button', { name: 'Create user' }));
    await fireEvent.input(utils.getByPlaceholderText('username'), { target: { value: 'carol' } });
    await fireEvent.input(utils.getByLabelText(/Initial password/), { target: { value: 'correct horse battery' } });
    await fireEvent.submit(utils.container.querySelector('form')!);
    await utils.findByText('Recovery codes for carol');
  }

  beforeEach(() => {
    app.CreateAccount = vi.fn(async () => account('carol'));
    app.AdminIssueRecoveryCodes = vi.fn(async () => ['AAAAAAAA-BBBBBBBB', 'CCCCCCCC-DDDDDDDD']);
    app.SaveRecoveryCodesFile = vi.fn(async () => '/Users/alice/gopmgr-recovery-codes-carol.txt');
  });

  it("saves the new account's codes through the desktop dialog", async () => {
    const utils = render(AdminPanel);
    await createCarol(utils);
    await fireEvent.click(utils.getByRole('button', { name: 'Save as .txt…' }));

    expect(app.SaveRecoveryCodesFile).toHaveBeenCalledWith('carol', ['AAAAAAAA-BBBBBBBB', 'CCCCCCCC-DDDDDDDD']);
    expect(await utils.findByText(
      'Saved to /Users/alice/gopmgr-recovery-codes-carol.txt. Keep a copy somewhere other than this computer.',
    )).toBeInTheDocument();
  });

  it("starts a second account's codes without the first account's save message", async () => {
    const utils = render(AdminPanel);
    await createCarol(utils);
    await fireEvent.click(utils.getByRole('button', { name: 'Save as .txt…' }));
    await utils.findByText(/Saved to/);

    // Without pressing Done, so the codes block stays on screen.
    app.CreateAccount = vi.fn(async () => account('dave'));
    app.AdminIssueRecoveryCodes = vi.fn(async () => ['EEEEEEEE-FFFFFFFF']);
    await fireEvent.click(await utils.findByRole('button', { name: 'Create user' }));
    await fireEvent.input(utils.getByPlaceholderText('username'), { target: { value: 'dave' } });
    await fireEvent.input(utils.getByLabelText(/Initial password/), { target: { value: 'correct horse battery' } });
    await fireEvent.submit(utils.container.querySelector('form')!);
    await utils.findByText('Recovery codes for dave');
    expect(utils.getByText('EEEEEEEE-FFFFFFFF')).toBeInTheDocument();
    expect(utils.queryByText(/Saved to/)).not.toBeInTheDocument();

    await fireEvent.click(utils.getByRole('button', { name: 'Save as .txt…' }));
    expect(app.SaveRecoveryCodesFile).toHaveBeenLastCalledWith('dave', ['EEEEEEEE-FFFFFFFF']);
  });
});

describe('AdminPanel own row', () => {
  const sole = /You're the only administrator, so no one can change your account\. To step down, make someone else an administrator who can sign in first\./;
  const other = 'Another administrator can change your account.';

  // The note sits in your own row only; other rows keep their actions.
  function ownRowNote(utils: ReturnType<typeof render>) {
    const row = utils.getByText('(you)').closest('tr') as HTMLElement;
    expect(within(bobRow(utils)).queryByText(sole)).not.toBeInTheDocument();
    expect(within(bobRow(utils)).queryByText(other)).not.toBeInTheDocument();
    return row;
  }

  it('tells the only administrator how to step down', async () => {
    const utils = render(AdminPanel);
    await utils.findByRole('button', { name: 'Disable account bob' });
    expect(within(ownRowNote(utils)).getByText(sole)).toBeInTheDocument();
  });

  it('points to the other administrators when there are any', async () => {
    app.AdminListUsers.mockResolvedValue([account('alice', { is_admin: true }), account('bob', { is_admin: true })]);
    const utils = render(AdminPanel);
    await utils.findByRole('button', { name: 'Disable account bob' });
    const row = ownRowNote(utils);
    expect(within(row).getByText(other)).toBeInTheDocument();
    expect(within(row).queryByText(sole)).not.toBeInTheDocument();
  });

  it('does not count a disabled administrator, who cannot sign in', async () => {
    app.AdminListUsers.mockResolvedValue([account('alice', { is_admin: true }), account('bob', { is_admin: true, disabled: true })]);
    const utils = render(AdminPanel);
    await utils.findByRole('button', { name: 'Enable account bob' });
    expect(within(ownRowNote(utils)).getByText(sole)).toBeInTheDocument();
  });
});

describe('AdminPanel recorded access to user data', () => {
  beforeEach(() => {
    app.AdminOpenUserData = vi.fn(async () => undefined);
    app.AdminListUserProjects = vi.fn(async () => []);
    app.AdminStopUserData = vi.fn(async () => undefined);
    app.AdminListAdminsWithoutKey = vi.fn(async () => []);
  });

  it('opens another account only with a reason, then shows the read-only viewer', async () => {
    const utils = render(AdminPanel);
    await fireEvent.click(await utils.findByRole('button', { name: "Open bob's data" }));
    expect(utils.queryByRole('button', { name: "Open alice's data" })).not.toBeInTheDocument();

    const form = utils.getByRole('group', { name: "Open bob's data" });
    const open = within(form).getByRole('button', { name: 'Open data' });
    expect(open).toBeDisabled();
    await fireEvent.input(within(form).getByLabelText('Reason (required)'), { target: { value: '   ' } });
    expect(open).toBeDisabled();
    expect(within(form).getByText(/recorded in the account history with your reason/)).toBeInTheDocument();

    await fireEvent.input(within(form).getByLabelText('Reason (required)'), { target: { value: 'Support ticket 42' } });
    await fireEvent.click(open);
    expect(app.AdminOpenUserData).toHaveBeenCalledWith('bob', 'Support ticket 42');
    expect(await utils.findByRole('region', { name: "Viewing bob's data" })).toBeInTheDocument();
    // One account at a time: no other account can be opened while viewing.
    expect(utils.queryByRole('button', { name: /^Open .*'s data$/ })).not.toBeInTheDocument();

    await fireEvent.click(utils.getByRole('button', { name: 'Stop viewing' }));
    expect(app.AdminStopUserData).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(utils.queryByRole('region', { name: "Viewing bob's data" })).not.toBeInTheDocument());
  });

  it("refuses in the handler too, if the disabled button is forced", async () => {
    const utils = render(AdminPanel);
    await fireEvent.click(await utils.findByRole('button', { name: "Open bob's data" }));
    const form = utils.getByRole('group', { name: "Open bob's data" });
    await fireEvent.input(within(form).getByLabelText('Reason (required)'), { target: { value: '  ' } });
    const open = within(form).getByRole('button', { name: 'Open data' });
    open.removeAttribute('disabled');
    await fireEvent.click(open);
    expect(app.AdminOpenUserData).not.toHaveBeenCalled();
  });

  it('shows no viewer when opening is refused', async () => {
    app.AdminOpenUserData = vi.fn(async () => {
      throw new Error("bob has not signed in since administrator access began, so their data can't be opened yet");
    });
    const utils = render(AdminPanel);
    await fireEvent.click(await utils.findByRole('button', { name: "Open bob's data" }));
    const form = utils.getByRole('group', { name: "Open bob's data" });
    await fireEvent.input(within(form).getByLabelText('Reason (required)'), { target: { value: 'Checking' } });
    await fireEvent.click(within(form).getByRole('button', { name: 'Open data' }));
    await waitFor(() => expect(app.AdminOpenUserData).toHaveBeenCalled());
    expect(utils.queryByRole('region', { name: "Viewing bob's data" })).not.toBeInTheDocument();
  });

  it('shows the reason for each recorded access in the account history', async () => {
    app.AdminListAccountEvents.mockResolvedValue([
      { id: 1, occurred_at: '2026-10-02T10:00:00Z', actor: 'alice', username: 'bob', action: 'admin_access', detail: 'Support ticket 42' },
    ]);
    const utils = render(AdminPanel);
    const history = await utils.findByRole('region', { name: 'Account history' });
    await waitFor(() => expect(within(history).getAllByRole('listitem')).toHaveLength(1));
    const item = within(history).getAllByRole('listitem')[0].textContent?.replace(/\s+/g, ' ');
    expect(item).toContain("alice opened bob's data as an administrator");
    expect(item).toContain('Reason: Support ticket 42');
  });

  it('offers the administrator key to administrators without it', async () => {
    app.AdminListUsers.mockResolvedValue([
      account('alice', { is_admin: true }),
      account('carol', { is_admin: true }),
      account('bob'),
    ]);
    app.AdminListAdminsWithoutKey = vi.fn(async () => ['carol']);
    app.AdminGrantKey = vi.fn(async () => undefined);
    const utils = render(AdminPanel);
    const give = await utils.findByRole('button', { name: 'Give administrator key' });
    expect(utils.getAllByRole('button', { name: 'Give administrator key' })).toHaveLength(1);
    expect(utils.getByText('No administrator key')).toBeInTheDocument();
    await fireEvent.click(give);
    expect(app.AdminGrantKey).toHaveBeenCalledWith('carol');
  });
});
