// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor, within } from '@testing-library/svelte';

import RecoveryCodesPanel from './RecoveryCodesPanel.svelte';
import { session } from '../../session.svelte';
import * as toast from '../../toast.svelte';

const newCodes = ['AAAAAAAA-BBBBBBBB', 'CCCCCCCC-DDDDDDDD'];
let app: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  app = {
    RecoveryCodeStatus: vi.fn(async () => ({ unused: 8, total: 8, legacy: false })),
    PrepareRecoveryCodes: vi.fn(async () => newCodes),
    ConfirmRecoveryCodes: vi.fn(async () => undefined),
    DiscardRecoveryCodes: vi.fn(async () => undefined),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

async function openCodes(utils: ReturnType<typeof render>) {
  await fireEvent.click(await utils.findByRole('button', { name: 'Create new recovery codes' }));
  await fireEvent.input(utils.getByLabelText('Current password'), { target: { value: 'current-password' } });
  await fireEvent.click(utils.getByRole('button', { name: 'Create codes' }));
  await utils.findByText(newCodes[0]);
}

describe('RecoveryCodesPanel status', () => {
  it.each([
    [{ unused: 8, total: 8, legacy: false }, null],
    [{ unused: 2, total: 8, legacy: false }, null],
    [{ unused: 1, total: 8, legacy: false }, /Only one recovery code is left/],
    [{ unused: 0, total: 8, legacy: false }, /You have no unused recovery codes/],
    [{ unused: 8, total: 8, legacy: true }, /from an older version of GoPMgr and can't recover encrypted projects/],
  ])('status %o shows the right warning', async (status, warning) => {
    app.RecoveryCodeStatus.mockResolvedValue(status);
    const utils = render(RecoveryCodesPanel);
    expect(await utils.findByText(`${status.unused} of 8 recovery codes unused.`)).toBeInTheDocument();
    if (warning) {
      expect(utils.getByRole('status')).toHaveTextContent(warning);
    } else {
      expect(utils.queryByRole('status')).not.toBeInTheDocument();
    }
  });

  it('says when the status cannot be read', async () => {
    app.RecoveryCodeStatus.mockRejectedValue(new Error('system database unavailable'));
    const utils = render(RecoveryCodesPanel);
    expect(await utils.findByRole('alert')).toHaveTextContent('Could not check your recovery codes');
  });
});

describe('RecoveryCodesPanel rotation', () => {
  it('asks for the current password before making codes', async () => {
    const utils = render(RecoveryCodesPanel);
    await fireEvent.click(await utils.findByRole('button', { name: 'Create new recovery codes' }));
    expect(app.PrepareRecoveryCodes).not.toHaveBeenCalled();
    expect(utils.getByRole('button', { name: 'Create codes' })).toBeDisabled();

    await fireEvent.input(utils.getByLabelText('Current password'), { target: { value: 'current-password' } });
    await fireEvent.click(utils.getByRole('button', { name: 'Create codes' }));
    await waitFor(() => expect(app.PrepareRecoveryCodes).toHaveBeenCalledWith('current-password'));
  });

  it('saves the new codes only after the user says they saved them', async () => {
    const utils = render(RecoveryCodesPanel);
    await openCodes(utils);
    const use = utils.getByRole('button', { name: 'Use the new codes' });
    expect(use).toBeDisabled();
    await fireEvent.click(use);
    expect(app.ConfirmRecoveryCodes).not.toHaveBeenCalled();

    await fireEvent.click(utils.getByLabelText('I have saved these codes somewhere safe.'));
    await fireEvent.click(use);
    await waitFor(() => expect(app.ConfirmRecoveryCodes).toHaveBeenCalledOnce());
    await waitFor(() => expect(utils.queryByText(newCodes[0])).not.toBeInTheDocument());
    expect(app.RecoveryCodeStatus).toHaveBeenCalledTimes(2);
  });

  it('keeps the current codes when the user backs out', async () => {
    const utils = render(RecoveryCodesPanel);
    await openCodes(utils);
    await fireEvent.click(utils.getByRole('button', { name: 'Keep my current codes' }));

    await waitFor(() => expect(app.DiscardRecoveryCodes).toHaveBeenCalledOnce());
    expect(app.ConfirmRecoveryCodes).not.toHaveBeenCalled();
    expect(utils.queryByText(newCodes[0])).not.toBeInTheDocument();
  });

  it('discards unsaved codes when the page closes', async () => {
    const utils = render(RecoveryCodesPanel);
    await openCodes(utils);
    utils.unmount();
    expect(app.DiscardRecoveryCodes).toHaveBeenCalledOnce();
    expect(app.ConfirmRecoveryCodes).not.toHaveBeenCalled();
  });

  it('shows why codes were not made or not saved', async () => {
    app.PrepareRecoveryCodes.mockRejectedValueOnce(new Error('current password is incorrect'));
    const utils = render(RecoveryCodesPanel);
    await fireEvent.click(await utils.findByRole('button', { name: 'Create new recovery codes' }));
    await fireEvent.input(utils.getByLabelText('Current password'), { target: { value: 'wrong-password' } });
    await fireEvent.click(utils.getByRole('button', { name: 'Create codes' }));
    expect(await utils.findByRole('alert')).toHaveTextContent('Current password is incorrect.');

    app.ConfirmRecoveryCodes.mockRejectedValueOnce(new Error('your recovery codes changed while these were on screen, so they were not saved; create new ones again'));
    await fireEvent.input(utils.getByLabelText('Current password'), { target: { value: 'current-password' } });
    await fireEvent.click(utils.getByRole('button', { name: 'Create codes' }));
    await utils.findByText(newCodes[0]);
    await fireEvent.click(utils.getByLabelText('I have saved these codes somewhere safe.'));
    await fireEvent.click(utils.getByRole('button', { name: 'Use the new codes' }));
    expect(await utils.findByRole('alert')).toHaveTextContent('Your recovery codes changed while these were on screen');
    expect(within(utils.container).queryByText(newCodes[0])).not.toBeInTheDocument();
  });
});

describe('RecoveryCodesPanel onrenewed', () => {
  it('runs only after new codes are saved', async () => {
    const onrenewed = vi.fn();
    const utils = render(RecoveryCodesPanel, { props: { onrenewed } });

    // Backing out does not count.
    await openCodes(utils);
    await fireEvent.click(utils.getByRole('button', { name: 'Keep my current codes' }));
    await waitFor(() => expect(app.DiscardRecoveryCodes).toHaveBeenCalled());
    expect(onrenewed).not.toHaveBeenCalled();

    // A failed prepare does not count.
    app.PrepareRecoveryCodes.mockRejectedValueOnce(new Error('current password is incorrect'));
    await fireEvent.click(utils.getByRole('button', { name: 'Create new recovery codes' }));
    await fireEvent.input(utils.getByLabelText('Current password'), { target: { value: 'wrong-password' } });
    await fireEvent.click(utils.getByRole('button', { name: 'Create codes' }));
    await utils.findByRole('alert');
    expect(onrenewed).not.toHaveBeenCalled();

    // A failed confirm does not count.
    app.ConfirmRecoveryCodes.mockRejectedValueOnce(new Error('your recovery codes changed while these were on screen'));
    await fireEvent.input(utils.getByLabelText('Current password'), { target: { value: 'current-password' } });
    await fireEvent.click(utils.getByRole('button', { name: 'Create codes' }));
    await utils.findByText(newCodes[0]);
    await fireEvent.click(utils.getByLabelText('I have saved these codes somewhere safe.'));
    await fireEvent.click(utils.getByRole('button', { name: 'Use the new codes' }));
    await waitFor(() => expect(utils.getByRole('alert')).toHaveTextContent('Your recovery codes changed'));
    expect(onrenewed).not.toHaveBeenCalled();

    // A saved set does.
    await openCodes(utils);
    await fireEvent.click(utils.getByLabelText('I have saved these codes somewhere safe.'));
    await fireEvent.click(utils.getByRole('button', { name: 'Use the new codes' }));
    await waitFor(() => expect(onrenewed).toHaveBeenCalledOnce());
  });
});

describe('RecoveryCodesPanel saving new codes', () => {
  const file = '/Users/alice/gopmgr-recovery-codes-alice-2026-09-30.txt';
  const unusedWarning = `The codes saved to ${file} were never put in use and won't work. Delete that file and any copy you printed.`;

  beforeEach(() => {
    session.user = {
      username: 'alice', display_name: 'Alice', data_dir: '', created_at: '', last_login: '', is_admin: false,
    } as Account;
    app.SaveRecoveryCodesFile = vi.fn(async () => file);
  });

  afterEach(() => {
    session.user = null;
  });

  async function saveNewCodes(utils: ReturnType<typeof render>) {
    await openCodes(utils);
    await fireEvent.click(utils.getByRole('button', { name: 'Save as .txt…' }));
    await utils.findByText(/Saved to/);
  }

  it('saves the new codes for the signed-in user and says they are not in use yet', async () => {
    const utils = render(RecoveryCodesPanel);
    await saveNewCodes(utils);

    expect(app.SaveRecoveryCodesFile).toHaveBeenCalledWith('alice', newCodes);
    expect(utils.getByText(
      `Saved to ${file}. Keep a copy somewhere other than this computer. These codes start working only when you choose “Use the new codes”.`,
    )).toBeInTheDocument();
  });

  it('names a saved file whose codes were kept out of use', async () => {
    const utils = render(RecoveryCodesPanel);
    await saveNewCodes(utils);
    await fireEvent.click(utils.getByRole('button', { name: 'Keep my current codes' }));

    expect(await utils.findByRole('alert')).toHaveTextContent(unusedWarning);
  });

  it('names every file saved from one set of new codes', async () => {
    const usb = '/Volumes/USB/gopmgr-recovery-codes-alice-2026-09-30.txt';
    app.SaveRecoveryCodesFile.mockResolvedValueOnce(file).mockResolvedValueOnce(usb);
    const toastSpy = vi.spyOn(toast, 'showToast');
    const utils = render(RecoveryCodesPanel);
    await saveNewCodes(utils);
    await fireEvent.click(utils.getByRole('button', { name: 'Save as .txt…' }));
    await utils.findByText(new RegExp(`Saved to ${usb}`));
    await fireEvent.click(utils.getByRole('button', { name: 'Keep my current codes' }));

    expect(await utils.findByRole('alert')).toHaveTextContent(
      `The codes saved to ${file}, ${usb} were never put in use and won't work. Delete those files and any copies you printed.`,
    );

    // Both are named in the toast too when the panel closes instead.
    await openCodes(utils);
    app.SaveRecoveryCodesFile.mockResolvedValueOnce('/a.txt').mockResolvedValueOnce('/b.txt');
    await fireEvent.click(utils.getByRole('button', { name: 'Save as .txt…' }));
    await utils.findByText(/Saved to \/a\.txt/);
    await fireEvent.click(utils.getByRole('button', { name: 'Save as .txt…' }));
    await utils.findByText(/Saved to \/b\.txt/);
    utils.unmount();
    expect(toastSpy).toHaveBeenCalledWith(
      "The codes saved to /a.txt, /b.txt were never put in use and won't work. Delete those files and any copies you printed.",
      'error',
    );
  });

  it('names a saved file when putting its codes in use fails', async () => {
    app.ConfirmRecoveryCodes.mockRejectedValueOnce(new Error('your recovery codes changed while these were on screen'));
    const utils = render(RecoveryCodesPanel);
    await saveNewCodes(utils);
    await fireEvent.click(utils.getByLabelText('I have saved these codes somewhere safe.'));
    await fireEvent.click(utils.getByRole('button', { name: 'Use the new codes' }));

    await waitFor(() => expect(utils.getAllByRole('alert').map((a) => a.textContent?.trim())).toContain(unusedWarning));
  });

  it('warns about nothing when the saved codes are put in use, or none were saved', async () => {
    const utils = render(RecoveryCodesPanel);
    await saveNewCodes(utils);
    await fireEvent.click(utils.getByLabelText('I have saved these codes somewhere safe.'));
    await fireEvent.click(utils.getByRole('button', { name: 'Use the new codes' }));
    await waitFor(() => expect(app.ConfirmRecoveryCodes).toHaveBeenCalledOnce());

    await openCodes(utils);
    await fireEvent.click(utils.getByRole('button', { name: 'Keep my current codes' }));
    await waitFor(() => expect(app.DiscardRecoveryCodes).toHaveBeenCalledOnce());
    expect(utils.queryByText(/never put in use/)).not.toBeInTheDocument();
  });

  it('still names the file when the panel closes with its codes unused', async () => {
    const toastSpy = vi.spyOn(toast, 'showToast');
    const utils = render(RecoveryCodesPanel);
    await saveNewCodes(utils);
    utils.unmount();

    expect(app.DiscardRecoveryCodes).toHaveBeenCalledOnce();
    expect(toastSpy).toHaveBeenCalledWith(unusedWarning, 'error');
  });
});
