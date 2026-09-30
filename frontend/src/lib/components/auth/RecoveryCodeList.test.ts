// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';

import RecoveryCodeList from './RecoveryCodeList.svelte';

const codes = ['AAAAAAAA-BBBBBBBB', 'CCCCCCCC-DDDDDDDD'];
let app: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  app = { SaveRecoveryCodesFile: vi.fn(async () => '/Users/alice/codes.txt') };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

function renderList(props: Record<string, unknown> = {}) {
  const utils = render(RecoveryCodeList, { props: { username: 'alice', codes, ...props } });
  return { utils, save: utils.getByRole('button', { name: 'Save as .txt…' }) };
}

describe('RecoveryCodeList', () => {
  it('shows every code and copies them one per line', async () => {
    const writeText = vi.fn(async () => undefined);
    Object.assign(navigator, { clipboard: { writeText } });
    const { utils } = renderList();

    for (const code of codes) expect(utils.getByText(code)).toBeInTheDocument();
    await fireEvent.click(utils.getByRole('button', { name: 'Copy' }));
    expect(writeText).toHaveBeenCalledWith(codes.join('\n'));
    expect(await utils.findByText('Recovery codes copied to the clipboard.')).toBeInTheDocument();
  });

  it('saves for the named account and says where the file went', async () => {
    const onsaved = vi.fn();
    const { utils, save } = renderList({ onsaved });
    await fireEvent.click(save);

    expect(app.SaveRecoveryCodesFile).toHaveBeenCalledWith('alice', codes);
    expect(await utils.findByText(
      'Saved to /Users/alice/codes.txt. Keep a copy somewhere other than this computer.',
    )).toBeInTheDocument();
    expect(onsaved).toHaveBeenCalledWith('/Users/alice/codes.txt');
    expect(utils.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('uses the caller\'s hint after the saved path', async () => {
    const { utils, save } = renderList({ savedHint: 'These start working later.' });
    await fireEvent.click(save);
    expect(await utils.findByText('Saved to /Users/alice/codes.txt. These start working later.')).toBeInTheDocument();
  });

  it('says nothing when the save dialog is cancelled', async () => {
    const onsaved = vi.fn();
    app.SaveRecoveryCodesFile.mockRejectedValueOnce('export cancelled');
    const { utils, save } = renderList({ onsaved });
    await fireEvent.click(save);

    await waitFor(() => expect(save).not.toBeDisabled());
    expect(app.SaveRecoveryCodesFile).toHaveBeenCalledOnce();
    expect(utils.queryByRole('alert')).not.toBeInTheDocument();
    expect(utils.queryByText(/Saved to/)).not.toBeInTheDocument();
    expect(onsaved).not.toHaveBeenCalled();
  });

  it('explains that an existing file is never replaced', async () => {
    app.SaveRecoveryCodesFile.mockRejectedValueOnce('export destination already exists');
    const { utils, save } = renderList();
    await fireEvent.click(save);

    expect(await utils.findByRole('alert')).toHaveTextContent(
      'A file with that name already exists, and GoPMgr never replaces files. Save again with a new name.',
    );
  });

  it('reports any other failure and clears it when a retry succeeds', async () => {
    app.SaveRecoveryCodesFile.mockRejectedValueOnce('recovery codes are saved as a .txt file');
    const { utils, save } = renderList();
    await fireEvent.click(save);

    expect(await utils.findByRole('alert')).toHaveTextContent(
      'Could not save the codes: recovery codes are saved as a .txt file',
    );
    await fireEvent.click(save);
    await waitFor(() => expect(utils.queryByRole('alert')).not.toBeInTheDocument());
    expect(await utils.findByText(/Saved to \/Users\/alice\/codes\.txt\./)).toBeInTheDocument();
  });

  it('opens one save dialog at a time', async () => {
    let finishSave: (path: string) => void = () => {};
    app.SaveRecoveryCodesFile.mockImplementationOnce(() => new Promise<string>((resolve) => { finishSave = resolve; }));
    const { utils, save } = renderList();
    await fireEvent.click(save);

    expect(save).toBeDisabled();
    expect(save).toHaveTextContent('Saving…');
    await fireEvent.click(save);
    expect(app.SaveRecoveryCodesFile).toHaveBeenCalledOnce();

    finishSave('/Users/alice/codes.txt');
    await waitFor(() => expect(save).not.toBeDisabled());
    expect(utils.getByText(/Saved to \/Users\/alice\/codes\.txt\./)).toBeInTheDocument();
  });
});
