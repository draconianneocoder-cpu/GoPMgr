// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';

import ChangePasswordForm from './ChangePasswordForm.svelte';

let app: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  app = { ChangePassword: vi.fn(async () => undefined) };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

async function fill(utils: ReturnType<typeof render>, current: string, next: string, confirm: string) {
  await fireEvent.input(utils.getByLabelText('Current password'), { target: { value: current } });
  await fireEvent.input(utils.getByLabelText('New password'), { target: { value: next } });
  await fireEvent.input(utils.getByLabelText('Confirm new password'), { target: { value: confirm } });
}

describe('ChangePasswordForm', () => {
  it('uses password-manager hints and hides the passwords until asked', async () => {
    const utils = render(ChangePasswordForm);
    const current = utils.getByLabelText('Current password');
    const next = utils.getByLabelText('New password');
    expect(current).toHaveAttribute('autocomplete', 'current-password');
    expect(next).toHaveAttribute('autocomplete', 'new-password');
    expect(utils.getByLabelText('Confirm new password')).toHaveAttribute('autocomplete', 'new-password');
    expect(current).toHaveAttribute('type', 'password');

    await fireEvent.click(utils.getByLabelText('Show passwords'));
    expect(current).toHaveAttribute('type', 'text');
    expect(next).toHaveAttribute('type', 'text');
  });

  it('stays disabled until the new password is long enough and confirmed', async () => {
    const utils = render(ChangePasswordForm);
    const button = utils.getByRole('button', { name: 'Change password' });

    await fill(utils, 'original-password', 'short', 'short');
    expect(button).toBeDisabled();
    await fill(utils, 'original-password', 'replacement-password', 'replacement-passwrd');
    expect(button).toBeDisabled();
    await fill(utils, '', 'replacement-password', 'replacement-password');
    expect(button).toBeDisabled();

    await fireEvent.submit(utils.container.querySelector('form')!);
    expect(app.ChangePassword).not.toHaveBeenCalled();
  });

  it('changes the password once and clears every field', async () => {
    const toasts = await import('../../toast.svelte');
    const toastSpy = vi.spyOn(toasts, 'showToast');
    const utils = render(ChangePasswordForm);
    await fill(utils, 'original-password', 'replacement-password', 'replacement-password');
    await fireEvent.click(utils.getByRole('button', { name: 'Change password' }));

    await waitFor(() => expect(app.ChangePassword).toHaveBeenCalledOnce());
    expect(app.ChangePassword).toHaveBeenCalledWith('original-password', 'replacement-password');
    await waitFor(() => expect(utils.getByLabelText('Current password')).toHaveValue(''));
    expect(utils.getByLabelText('New password')).toHaveValue('');
    expect(utils.getByLabelText('Confirm new password')).toHaveValue('');
    expect(toastSpy).toHaveBeenCalledWith('Password changed. Your projects and recovery codes are unaffected.', 'success');
  });

  it('shows why a change was refused and keeps what was typed', async () => {
    app.ChangePassword.mockRejectedValue(new Error('current password is incorrect'));
    const utils = render(ChangePasswordForm);
    await fill(utils, 'wrong-password', 'replacement-password', 'replacement-password');
    await fireEvent.click(utils.getByRole('button', { name: 'Change password' }));

    expect(await utils.findByRole('alert')).toHaveTextContent('Current password is incorrect.');
    expect(utils.getByLabelText('New password')).toHaveValue('replacement-password');
  });
});
