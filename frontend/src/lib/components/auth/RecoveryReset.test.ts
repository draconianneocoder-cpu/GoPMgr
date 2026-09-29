// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render } from '@testing-library/svelte';

import RecoveryReset from './RecoveryReset.svelte';

// The exact text of users.ErrLegacyRecoveryCode as Wails delivers it.
const LEGACY_ERROR =
  "This recovery code is from an older version of GoPMgr and can't unlock your encryption key, so nothing was changed.";

let app: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  app = { ResetWithRecoveryCode: vi.fn(async () => undefined) };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

async function submitReset() {
  const utils = render(RecoveryReset);
  await fireEvent.input(utils.getByLabelText('Username'), { target: { value: 'alice' } });
  await fireEvent.input(utils.getByLabelText('Recovery code'), { target: { value: 'AAAAAAAA-BBBBBBBB' } });
  await fireEvent.input(utils.getByLabelText('New password'), { target: { value: 'new-password' } });
  await fireEvent.input(utils.getByLabelText(/confirm/i), { target: { value: 'new-password' } });
  await fireEvent.submit(utils.getByRole('button', { name: /reset password/i }).closest('form')!);
  return utils;
}

describe('RecoveryReset', () => {
  it('says up front that only the password changes', () => {
    const utils = render(RecoveryReset);
    expect(utils.getByText('This changes only your password. Your encrypted projects stay readable.')).toBeInTheDocument();
  });

  it('confirms the projects are unchanged after a reset', async () => {
    const utils = await submitReset();
    expect(app.ResetWithRecoveryCode).toHaveBeenCalledWith('alice', 'AAAAAAAA-BBBBBBBB', 'new-password');
    expect(await utils.findByRole('status')).toHaveTextContent(
      'Password reset. Your projects are unchanged. Sign in with your new password.',
    );
  });

  it('explains an old-format code without implying the projects were touched', async () => {
    app.ResetWithRecoveryCode.mockRejectedValueOnce(LEGACY_ERROR);
    const utils = await submitReset();
    const alert = await utils.findByRole('alert');
    expect(alert).toHaveTextContent(
      "This recovery code is from an older version of GoPMgr and can't unlock your encryption key, so nothing was changed and your projects are safe. Sign in with your password if you remember it.",
    );
    expect(utils.queryByRole('status')).not.toBeInTheDocument();
  });

  it('keeps every other failure generic, so it does not reveal which accounts exist', async () => {
    app.ResetWithRecoveryCode.mockRejectedValueOnce('users: invalid or used recovery code');
    const utils = await submitReset();
    expect(await utils.findByRole('alert')).toHaveTextContent(/^Invalid username or recovery code\.$/);
  });
});
