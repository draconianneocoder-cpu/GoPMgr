// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';

import RecoveryCodesGate from './RecoveryCodesGate.svelte';

let app: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  app = {
    RecoveryCodeStatus: vi.fn(async () => ({ unused: 0, total: 8, legacy: false, encryption_ready: false })),
    PrepareRecoveryCodes: vi.fn(async () => ['AAAAAAAA-BBBBBBBB']),
    ConfirmRecoveryCodes: vi.fn(async () => undefined),
    DiscardRecoveryCodes: vi.fn(async () => undefined),
    AcceptEncryptionWithoutRecoveryCodes: vi.fn(async () => undefined),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('RecoveryCodesGate', () => {
  it('lets a user with no codes continue only after ticking that they understand', async () => {
    const onready = vi.fn();
    const utils = render(RecoveryCodesGate, { props: { variant: 'none', onready, oncancel: vi.fn() } });
    const skip = utils.getByRole('button', { name: 'Continue without recovery codes' });

    expect(skip).toBeDisabled();
    await fireEvent.click(skip);
    expect(app.AcceptEncryptionWithoutRecoveryCodes).not.toHaveBeenCalled();
    expect(onready).not.toHaveBeenCalled();

    await fireEvent.click(utils.getByLabelText(/projects I create or encrypt until I sign out/));
    await fireEvent.click(skip);
    await waitFor(() => expect(onready).toHaveBeenCalledOnce());
    expect(app.AcceptEncryptionWithoutRecoveryCodes).toHaveBeenCalledOnce();
  });

  it('offers no way to skip legacy codes', () => {
    const utils = render(RecoveryCodesGate, { props: { variant: 'legacy', onready: vi.fn(), oncancel: vi.fn() } });
    expect(utils.getByRole('region', { name: /save a way back in/i })).toHaveTextContent(/from an older version of GoPMgr/);
    expect(utils.queryByRole('button', { name: 'Continue without recovery codes' })).not.toBeInTheDocument();
    expect(utils.queryByRole('checkbox', { name: /until I sign out/ })).not.toBeInTheDocument();
  });

  it('carries on once new codes are saved', async () => {
    const onready = vi.fn();
    const utils = render(RecoveryCodesGate, { props: { variant: 'legacy', onready, oncancel: vi.fn() } });
    await fireEvent.click(await utils.findByRole('button', { name: 'Create new recovery codes' }));
    await fireEvent.input(utils.getByLabelText('Current password'), { target: { value: 'current-password' } });
    await fireEvent.click(utils.getByRole('button', { name: 'Create codes' }));
    await utils.findByText('AAAAAAAA-BBBBBBBB');
    expect(onready).not.toHaveBeenCalled();
    await fireEvent.click(utils.getByLabelText('I have saved these codes somewhere safe.'));
    await fireEvent.click(utils.getByRole('button', { name: 'Use the new codes' }));
    await waitFor(() => expect(onready).toHaveBeenCalledOnce());
  });

  it('reports a failed acceptance without carrying on', async () => {
    app.AcceptEncryptionWithoutRecoveryCodes.mockRejectedValue(new Error('not signed in'));
    const onready = vi.fn();
    const utils = render(RecoveryCodesGate, { props: { variant: 'none', onready, oncancel: vi.fn() } });
    await fireEvent.click(utils.getByLabelText(/projects I create or encrypt until I sign out/));
    await fireEvent.click(utils.getByRole('button', { name: 'Continue without recovery codes' }));

    expect(await utils.findByRole('alert')).toHaveTextContent('Could not continue: not signed in');
    expect(onready).not.toHaveBeenCalled();
  });
});
