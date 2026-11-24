// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';

import UnsavedChangesDialog from './UnsavedChangesDialog.svelte';
import { autosave } from '../autosave.svelte';
import { cancelNavigation, navigation, requestNavigation, session } from '../session.svelte';

// The prompt a guarded navigation opens: it takes keyboard focus (Cancel,
// the least destructive choice), keeps Tab inside, and Escape cancels
// unless a save is running (modalFocus, owned by modal-focus.test.ts).
describe('UnsavedChangesDialog', () => {
  let stop: (() => void) | null = null;
  afterEach(() => {
    cleanup();
    stop?.();
    autosave.discardAll();
    cancelNavigation();
    navigation.saving = false;
    document.body.innerHTML = '';
  });

  async function openPrompt() {
    let draft = 'clean';
    stop = autosave.register(() => draft, async () => true, false);
    draft = 'edited';
    const onCancel = vi.fn();
    const opener = document.createElement('button');
    document.body.append(opener);
    opener.focus();
    const utils = render(UnsavedChangesDialog, { props: { onCancel } });
    session.view = 'dashboard';
    requestNavigation('project_settings');
    const dialog = await utils.findByRole('alertdialog', { name: 'Unsaved changes' });
    return { utils, dialog, onCancel };
  }

  it('focuses Cancel when it opens and keeps Tab inside', async () => {
    const { utils, dialog } = await openPrompt();
    const cancel = utils.getByRole('button', { name: 'Cancel' });
    await waitFor(() => expect(document.activeElement).toBe(cancel));

    const save = utils.getByRole('button', { name: 'Save and continue' });
    save.focus();
    await fireEvent.keyDown(dialog, { key: 'Tab' });
    expect(document.activeElement).toBe(cancel);
  });

  it('cancels on Escape, but not while saving', async () => {
    const { dialog, onCancel } = await openPrompt();
    navigation.saving = true;
    await fireEvent.keyDown(dialog, { key: 'Escape' });
    expect(onCancel).not.toHaveBeenCalled();
    navigation.saving = false;
    await fireEvent.keyDown(dialog, { key: 'Escape' });
    expect(onCancel).toHaveBeenCalledOnce();
  });
});
