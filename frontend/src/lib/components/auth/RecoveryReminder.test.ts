// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';

import RecoveryReminder from './RecoveryReminder.svelte';
import { session } from '../../session.svelte';

const status = (unused: number, legacy = false) => ({ unused, total: 8, legacy, encryption_ready: !legacy && unused > 0 });
const account = (username: string) => ({ username }) as unknown as Account;

let app: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  session.user = account('alice');
  session.recoveryReminderDismissedFor = null;
  app = {
    RecoveryCodeStatus: vi.fn(async () => status(1)),
    PrepareRecoveryCodes: vi.fn(async () => ['AAAAAAAA-BBBBBBBB']),
    ConfirmRecoveryCodes: vi.fn(async () => undefined),
    DiscardRecoveryCodes: vi.fn(async () => undefined),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  session.user = null;
  session.recoveryReminderDismissedFor = null;
});

const heading = { name: 'Keep a way back into your account' };

describe('RecoveryReminder', () => {
  it.each([
    ['no unused codes', status(0)],
    ['one unused code', status(1)],
    ['codes from an older version, even with 8 unused', status(8, true)],
  ])('shows for %s', async (_, s) => {
    app.RecoveryCodeStatus.mockResolvedValue(s);
    const utils = render(RecoveryReminder);
    expect(await utils.findByRole('heading', heading)).toBeInTheDocument();
  });

  it('stays hidden with two unused codes', async () => {
    app.RecoveryCodeStatus.mockResolvedValue(status(2));
    const utils = render(RecoveryReminder);
    await waitFor(() => expect(app.RecoveryCodeStatus).toHaveBeenCalled());
    expect(utils.queryByRole('heading', heading)).not.toBeInTheDocument();
  });

  it('stays hidden when the status cannot be read', async () => {
    app.RecoveryCodeStatus.mockRejectedValue('no session');
    const utils = render(RecoveryReminder);
    await waitFor(() => expect(app.RecoveryCodeStatus).toHaveBeenCalled());
    expect(utils.queryByRole('heading', heading)).not.toBeInTheDocument();
    expect(utils.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('"Not now" lasts only until the next sign-in, for this user or another', async () => {
    const first = render(RecoveryReminder);
    await fireEvent.click(await first.findByRole('button', { name: 'Not now' }));
    expect(first.queryByRole('heading', heading)).not.toBeInTheDocument();
    cleanup();

    // Same account, back on the home screen: still dismissed.
    const before = app.RecoveryCodeStatus.mock.calls.length;
    const again = render(RecoveryReminder);
    await waitFor(() => expect(app.RecoveryCodeStatus.mock.calls.length).toBeGreaterThan(before));
    expect(again.queryByRole('heading', heading)).not.toBeInTheDocument();
    cleanup();

    // Another account signs in without alice's dismissal carrying over.
    session.user = account('bob');
    const bob = render(RecoveryReminder);
    expect(await bob.findByRole('heading', heading)).toBeInTheDocument();
    cleanup();

    // Alice signs in again: each sign-in assigns a new Account object.
    session.user = account('alice');
    const aliceAgain = render(RecoveryReminder);
    expect(await aliceAgain.findByRole('heading', heading)).toBeInTheDocument();
  });

  it('reads the status on every visit, so codes renewed elsewhere clear it', async () => {
    const first = render(RecoveryReminder);
    expect(await first.findByRole('heading', heading)).toBeInTheDocument();
    cleanup();
    app.RecoveryCodeStatus.mockResolvedValue(status(8));
    const before = app.RecoveryCodeStatus.mock.calls.length;
    const second = render(RecoveryReminder);
    await waitFor(() => expect(app.RecoveryCodeStatus.mock.calls.length).toBeGreaterThan(before));
    expect(second.queryByRole('heading', heading)).not.toBeInTheDocument();
  });

  it('goes away once new codes are saved from it', async () => {
    const utils = render(RecoveryReminder);
    await fireEvent.click(await utils.findByRole('button', { name: 'Create new recovery codes' }));
    await fireEvent.input(utils.getByLabelText('Current password'), { target: { value: 'current-password' } });
    await fireEvent.click(utils.getByRole('button', { name: 'Create codes' }));
    await fireEvent.click(await utils.findByLabelText('I have saved these codes somewhere safe.'));
    app.RecoveryCodeStatus.mockResolvedValue(status(8));
    await fireEvent.click(utils.getByRole('button', { name: 'Use the new codes' }));
    await waitFor(() => expect(utils.queryByRole('heading', heading)).not.toBeInTheDocument());
    expect(app.ConfirmRecoveryCodes).toHaveBeenCalledOnce();
  });
});
