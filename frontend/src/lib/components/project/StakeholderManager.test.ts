// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { cleanup, render, fireEvent, waitFor } from '@testing-library/svelte';

import StakeholderManager from './StakeholderManager.svelte';
import { autosave } from '../../autosave.svelte';
import {
  cancelNavigation,
  discardAndContinueNavigation,
  navigation,
  requestNavigation,
  saveAndContinueNavigation,
  session,
} from '../../session.svelte';
import { NativeCloseController } from '../../native-close';

let app: Record<string, ReturnType<typeof vi.fn>>;

const stakeholder: Stakeholder = {
  id: 'sh-1',
  project_id: 'p1',
  name: 'Ada Lovelace',
  role: 'Tech lead',
  organisation: 'Analytical Engines Ltd',
  email: '',
  phone: '',
  category: 'team',
  availability: 1,
  hourly_rate: 88.25,
  hourly_rate_minor_units: 8825,
  contract_value: 1200.99,
  contract_value_minor_units: 120099,
  notes: '',
  created_at: '',
  updated_at: '',
};

beforeEach(() => {
  session.project = { id: 'p1' } as unknown as typeof session.project;
  app = {
    ListStakeholders: vi.fn(async () => [stakeholder]),
    SaveStakeholder: vi.fn(async (s: Stakeholder) => s),
    DeleteStakeholder: vi.fn(async () => undefined),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  autosave.discardAll();
  cancelNavigation();
  navigation.saving = false;
  navigation.error = '';
  session.view = 'portfolio';
  session.editingId = null;
  session.project = null;
});

async function openEditor(utils: ReturnType<typeof render>) {
  await waitFor(() => expect(app.ListStakeholders).toHaveBeenCalled());
  const cardButtons = () =>
    Array.from(utils.container.querySelectorAll('button')).filter((b) =>
      b.textContent?.includes('Ada Lovelace'),
    );
  await waitFor(() => expect(cardButtons().length).toBe(1));
  await fireEvent.click(cardButtons()[0]);
  await waitFor(() => document.querySelector('input') as HTMLInputElement);
}

async function openNewEditor(): Promise<void> {
  const button = Array.from(document.querySelectorAll('button')).find(
    (candidate) => candidate.textContent?.trim() === '+ Stakeholder',
  )!;
  await fireEvent.click(button);
  await waitFor(() => expect(document.querySelector('[role="dialog"] input')).not.toBeNull());
}

function editorButtons() {
  return Array.from(document.querySelectorAll('[role="dialog"] button'));
}

function closeButton() {
  return document.querySelector('[role="dialog"] button[aria-label="Close"]') as HTMLButtonElement;
}

function nativeCloseController() {
  const app = {
    EnableNativeCloseGuard: vi.fn().mockResolvedValue(undefined),
    CompleteNativeClose: vi.fn().mockResolvedValue(undefined),
  };
  return {
    app,
    controller: new NativeCloseController({
      app,
      reportError: vi.fn(),
      setInteractionLocked: vi.fn(),
    }),
  };
}

describe('StakeholderManager close guard', () => {
  it('closes without prompting when there are no unsaved edits', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    const utils = render(StakeholderManager);
    await openEditor(utils);

    const cancelBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Cancel',
    )!;
    await fireEvent.click(cancelBtn);

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(document.querySelector('input')).toBeNull();
  });

  it('opens the shared unsaved-changes guard on Cancel when the name was edited, and keeps the modal open when declined', async () => {
    // Wails v2.13.0's darwin WKUIDelegate implements none of the JS
    // confirm/alert/prompt panel methods (verified: no `runJavaScript*Panel`
    // implementation anywhere in the vendored module). The observed result
    // in a packaged build is that `window.confirm()` produces no dialog and
    // the close silently no-ops (2026-08-16 GUI evidence) — the exact value
    // WebKit's `confirm()` returns was not independently confirmed, only
    // that no dialog appears. Mocking it to return `false` reproduces the
    // observed symptom; the close guard must not depend on this return
    // value at all.
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    const utils = render(StakeholderManager);
    await openEditor(utils);
    session.view = 'stakeholders';

    const nameInput = document.querySelector('input') as HTMLInputElement;
    await fireEvent.input(nameInput, { target: { value: 'Edited name' } });

    const cancelBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Cancel',
    )!;
    await fireEvent.click(cancelBtn);

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(navigation.pending?.view).toBe('stakeholders');
    expect((document.querySelector('input') as HTMLInputElement).value).toBe('Edited name');

    cancelNavigation();
    expect(navigation.pending).toBeNull();
    expect((document.querySelector('input') as HTMLInputElement).value).toBe('Edited name');
  });

  it('discards and closes on Cancel when the shared guard is told to discard', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    const utils = render(StakeholderManager);
    await openEditor(utils);
    session.view = 'stakeholders';

    const nameInput = document.querySelector('input') as HTMLInputElement;
    await fireEvent.input(nameInput, { target: { value: 'Edited name' } });

    const cancelBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Cancel',
    )!;
    await fireEvent.click(cancelBtn);
    expect(navigation.pending?.view).toBe('stakeholders');

    await discardAndContinueNavigation();

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(document.querySelector('input')).toBeNull();
    expect(app.SaveStakeholder).not.toHaveBeenCalled();
    expect(navigation.pending).toBeNull();
  });

  it('opens the shared unsaved-changes guard on the header close button when dirty', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    const utils = render(StakeholderManager);
    await openEditor(utils);
    session.view = 'stakeholders';

    const nameInput = document.querySelector('input') as HTMLInputElement;
    await fireEvent.input(nameInput, { target: { value: 'Edited via header close' } });

    // Scoped to the dialog: the stakeholder list also has a per-row "×"
    // delete button with the same text content.
    const dialog = document.querySelector('[role="dialog"]') as HTMLElement;
    const closeBtn = Array.from(dialog.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === '×',
    )!;
    await fireEvent.click(closeBtn);

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(navigation.pending?.view).toBe('stakeholders');
    expect(document.querySelector('input')).not.toBeNull();

    await discardAndContinueNavigation();
    expect(document.querySelector('input')).toBeNull();
  });

  it('opens the shared unsaved-changes guard on Escape when dirty and does not close when declined', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    const utils = render(StakeholderManager);
    await openEditor(utils);
    session.view = 'stakeholders';

    const nameInput = document.querySelector('input') as HTMLInputElement;
    await fireEvent.input(nameInput, { target: { value: 'Edited via escape test' } });

    const dialog = document.querySelector('[role="dialog"]') as HTMLElement;
    await fireEvent.keyDown(dialog, { key: 'Escape' });

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(navigation.pending?.view).toBe('stakeholders');
    expect(document.querySelector('input')).not.toBeNull();

    cancelNavigation();
    expect(document.querySelector('input')).not.toBeNull();
  });

  it('closes without prompting after a successful save even though the draft differs from the original', async () => {
    vi.spyOn(window, 'confirm');
    const utils = render(StakeholderManager);
    await openEditor(utils);

    const nameInput = document.querySelector('input') as HTMLInputElement;
    await fireEvent.input(nameInput, { target: { value: 'Saved name' } });

    const saveBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Save',
    )!;
    await fireEvent.click(saveBtn);

    await waitFor(() => expect(app.SaveStakeholder).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(document.querySelector('input')).toBeNull());
    expect(window.confirm).not.toHaveBeenCalled();
  });

  it('ignores a close request made while a save is still in flight', async () => {
    let resolveSave!: (s: Stakeholder) => void;
    app.SaveStakeholder = vi.fn(
      () => new Promise<Stakeholder>((resolve) => { resolveSave = resolve; }),
    );
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    const utils = render(StakeholderManager);
    await openEditor(utils);

    const nameInput = document.querySelector('input') as HTMLInputElement;
    await fireEvent.input(nameInput, { target: { value: 'Edited name' } });

    const saveBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Save',
    )!;
    await fireEvent.click(saveBtn);
    await waitFor(() => expect(app.SaveStakeholder).toHaveBeenCalledTimes(1));

    expect(closeButton().disabled).toBe(true);
    expect(editorButtons().find((button) => button.textContent?.trim() === 'Cancel')?.disabled).toBe(true);
    for (const control of document.querySelectorAll('[role="dialog"] input, [role="dialog"] select, [role="dialog"] textarea')) {
      expect((control as HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement).disabled).toBe(true);
    }
    await fireEvent.click(saveBtn);
    expect(app.SaveStakeholder).toHaveBeenCalledTimes(1);

    const cancelBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Cancel',
    )!;
    await fireEvent.click(cancelBtn);

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(document.querySelector('input')).not.toBeNull();

    resolveSave({ ...stakeholder, name: 'Edited name' });
    await waitFor(() => expect(document.querySelector('input')).toBeNull());
  });

  it('saves a dirty stakeholder through guarded navigation before continuing', async () => {
    const utils = render(StakeholderManager);
    await openEditor(utils);
    await fireEvent.input(document.querySelector('input') as HTMLInputElement, {
      target: { value: 'Saved by navigation' },
    });

    session.view = 'stakeholders';
    requestNavigation('dashboard');
    expect(navigation.pending?.view).toBe('dashboard');
    await saveAndContinueNavigation();

    expect(app.SaveStakeholder).toHaveBeenCalledWith(expect.objectContaining({
      id: 'sh-1',
      name: 'Saved by navigation',
      hourly_rate_minor_units: 8825,
      contract_value_minor_units: 120099,
    }));
    expect(document.querySelector('input')).toBeNull();
    expect(navigation.pending).toBeNull();
    expect(session.view).toBe('dashboard');
  });

  it('keeps native close pending after a failed save and completes it after retry', async () => {
    app.SaveStakeholder = vi.fn()
      .mockRejectedValueOnce(new Error('bridge unavailable'))
      .mockResolvedValueOnce({ ...stakeholder, name: 'Retry saved' });
    const utils = render(StakeholderManager);
    await openEditor(utils);
    await fireEvent.input(document.querySelector('input') as HTMLInputElement, {
      target: { value: 'Retry saved' },
    });
    const { app: nativeApp, controller } = nativeCloseController();

    controller.request();
    expect(navigation.pending).not.toBeNull();
    await saveAndContinueNavigation();

    expect(app.SaveStakeholder).toHaveBeenCalledTimes(1);
    expect(document.querySelector('input')).not.toBeNull();
    expect(autosave.hasDirty()).toBe(true);
    expect(navigation.pending).not.toBeNull();
    expect(nativeApp.CompleteNativeClose).not.toHaveBeenCalled();

    await saveAndContinueNavigation();
    await waitFor(() => expect(nativeApp.CompleteNativeClose).toHaveBeenCalledOnce());
    expect(app.SaveStakeholder).toHaveBeenCalledTimes(2);
  });

  it('preserves late modal edits and canonical backend money fields for a second guarded save', async () => {
    let finishFirstSave!: (saved: Stakeholder) => void;
    const firstSaved: Stakeholder = {
      ...stakeholder,
      id: 'sh-server',
      project_id: 'server-project',
      category: 'sponsor',
      created_at: '2026-08-15T00:00:00Z',
      updated_at: '2026-08-15T00:01:00Z',
    };
    app.SaveStakeholder = vi.fn()
      .mockImplementationOnce(() => new Promise<Stakeholder>((resolve) => { finishFirstSave = resolve; }))
      .mockResolvedValueOnce(firstSaved);
    const utils = render(StakeholderManager);
    await openEditor(utils);
    const inputs = document.querySelectorAll('[role="dialog"] input');
    const selects = document.querySelectorAll('[role="dialog"] select');
    const notes = document.querySelector('[role="dialog"] textarea') as HTMLTextAreaElement;

    await fireEvent.input(inputs[0] as HTMLInputElement, { target: { value: 'First name' } });
    session.view = 'stakeholders';
    requestNavigation('dashboard');
    const continuing = saveAndContinueNavigation();
    await waitFor(() => expect(app.SaveStakeholder).toHaveBeenCalledOnce());
    const firstPayload = app.SaveStakeholder.mock.calls[0][0] as Stakeholder;
    const expectedFirstPayload = { ...firstPayload };
    expect(firstPayload).toMatchObject({
      id: 'sh-1',
      project_id: 'p1',
      name: 'First name',
      hourly_rate: 88.25,
      hourly_rate_minor_units: 8825,
      contract_value: 1200.99,
      contract_value_minor_units: 120099,
    });
    expect((inputs[0] as HTMLInputElement).disabled).toBe(true);

    await fireEvent.input(inputs[0] as HTMLInputElement, { target: { value: 'Late name' } });
    await fireEvent.change(selects[0] as HTMLSelectElement, { target: { value: 'vendor' } });
    await fireEvent.input(inputs[1] as HTMLInputElement, { target: { value: 'Late role' } });
    await fireEvent.input(inputs[2] as HTMLInputElement, { target: { value: 'Late organisation' } });
    await fireEvent.input(inputs[3] as HTMLInputElement, { target: { value: 'late@example.test' } });
    await fireEvent.input(inputs[4] as HTMLInputElement, { target: { value: '+15551234567' } });
    await fireEvent.input(inputs[5] as HTMLInputElement, { target: { value: '101.5' } });
    await fireEvent.input(inputs[6] as HTMLInputElement, { target: { value: '2000.25' } });
    await fireEvent.input(inputs[7] as HTMLInputElement, { target: { value: '2.5' } });
    await fireEvent.input(notes, { target: { value: 'Late notes' } });
    expect(firstPayload).toEqual(expectedFirstPayload);
    finishFirstSave(firstSaved);
    await continuing;

    expect((document.querySelector('[role="dialog"] input') as HTMLInputElement).value).toBe('Late name');
    expect(navigation.pending?.view).toBe('dashboard');
    expect(navigation.error).toContain('Changes were made while saving');
    expect(autosave.hasDirty()).toBe(true);

    await saveAndContinueNavigation();

    expect(app.SaveStakeholder).toHaveBeenCalledTimes(2);
    expect(app.SaveStakeholder).toHaveBeenLastCalledWith({
      id: 'sh-server',
      project_id: 'server-project',
      name: 'Late name',
      role: 'Late role',
      organisation: 'Late organisation',
      email: 'late@example.test',
      phone: '+15551234567',
      category: 'vendor',
      availability: 2.5,
      hourly_rate: 101.5,
      hourly_rate_minor_units: 0,
      contract_value: 2000.25,
      contract_value_minor_units: 0,
      notes: 'Late notes',
      created_at: '2026-08-15T00:00:00Z',
      updated_at: '2026-08-15T00:01:00Z',
    });
    expect(document.querySelector('input')).toBeNull();
    expect(session.view).toBe('dashboard');
  });

  it('rejects a whitespace-only name through guarded save as well as the disabled button', async () => {
    render(StakeholderManager);
    await openNewEditor();
    const nameInput = document.querySelector('[role="dialog"] input') as HTMLInputElement;
    await fireEvent.input(nameInput, { target: { value: '   ' } });
    const saveButton = editorButtons().find((button) => button.textContent?.trim() === 'Save')!;
    expect(saveButton.disabled).toBe(true);

    session.view = 'stakeholders';
    requestNavigation('dashboard');
    await saveAndContinueNavigation();

    expect(app.SaveStakeholder).not.toHaveBeenCalled();
    expect(navigation.pending?.view).toBe('dashboard');
    expect(document.querySelector('[role="alert"]')?.textContent).toContain('Stakeholder name is required');
  });

  it('normalizes only an edited hourly rate and preserves the exact contract minor units', async () => {
    const exactContractMinorUnits = 9007199253740993;
    app.ListStakeholders = vi.fn(async () => [{
      ...stakeholder,
      contract_value: exactContractMinorUnits / 100,
      contract_value_minor_units: exactContractMinorUnits,
    }]);
    const utils = render(StakeholderManager);
    await openEditor(utils);
    const inputs = document.querySelectorAll('[role="dialog"] input');
    await fireEvent.input(inputs[5] as HTMLInputElement, { target: { value: '101.5' } });
    await fireEvent.click(editorButtons().find((button) => button.textContent?.trim() === 'Save')!);

    await waitFor(() => expect(app.SaveStakeholder).toHaveBeenCalledOnce());
    expect(app.SaveStakeholder).toHaveBeenCalledWith(expect.objectContaining({
      hourly_rate: 101.5,
      hourly_rate_minor_units: 0,
      contract_value_minor_units: exactContractMinorUnits,
    }));
  });

  it('preserves exact minor units when saving only a non-money field', async () => {
    const exactHourlyMinorUnits = 9007199253740993;
    const exactContractMinorUnits = 9007199253740991;
    app.ListStakeholders = vi.fn(async () => [{
      ...stakeholder,
      hourly_rate: exactHourlyMinorUnits / 100,
      hourly_rate_minor_units: exactHourlyMinorUnits,
      contract_value: exactContractMinorUnits / 100,
      contract_value_minor_units: exactContractMinorUnits,
    }]);
    const utils = render(StakeholderManager);
    await openEditor(utils);
    await fireEvent.input(document.querySelector('[role="dialog"] input') as HTMLInputElement, {
      target: { value: 'Non-money edit' },
    });
    await fireEvent.click(editorButtons().find((button) => button.textContent?.trim() === 'Save')!);

    await waitFor(() => expect(app.SaveStakeholder).toHaveBeenCalledOnce());
    expect(app.SaveStakeholder).toHaveBeenCalledWith(expect.objectContaining({
      name: 'Non-money edit',
      hourly_rate_minor_units: exactHourlyMinorUnits,
      contract_value_minor_units: exactContractMinorUnits,
    }));
  });

  it('refuses a guarded save when Wails supplied an unsafe minor-unit number', async () => {
    // An int64 above this boundary is rounded before Wails exposes it as a
    // JavaScript number, so returning it unchanged would corrupt money.
    const unsafeHourlyMinorUnits = Number.MAX_SAFE_INTEGER + 1;
    app.ListStakeholders = vi.fn(async () => [{
      ...stakeholder,
      hourly_rate: unsafeHourlyMinorUnits / 100,
      hourly_rate_minor_units: unsafeHourlyMinorUnits,
    }]);
    const utils = render(StakeholderManager);
    await openEditor(utils);
    await fireEvent.input(document.querySelector('[role="dialog"] input') as HTMLInputElement, {
      target: { value: 'Cannot save safely' },
    });

    session.view = 'stakeholders';
    requestNavigation('dashboard');
    await saveAndContinueNavigation();

    expect(app.SaveStakeholder).not.toHaveBeenCalled();
    expect(navigation.pending?.view).toBe('dashboard');
    expect(autosave.hasDirty()).toBe(true);
    expect(document.querySelector('[role="alert"]')?.textContent).toContain('too large to save safely');
  });

  it('normalizes only an edited contract value and preserves exact hourly-rate minor units', async () => {
    const exactHourlyMinorUnits = 9007199253740993;
    app.ListStakeholders = vi.fn(async () => [{
      ...stakeholder,
      hourly_rate: exactHourlyMinorUnits / 100,
      hourly_rate_minor_units: exactHourlyMinorUnits,
    }]);
    const utils = render(StakeholderManager);
    await openEditor(utils);
    const inputs = document.querySelectorAll('[role="dialog"] input');
    await fireEvent.input(inputs[6] as HTMLInputElement, { target: { value: '2000.25' } });
    await fireEvent.click(editorButtons().find((button) => button.textContent?.trim() === 'Save')!);

    await waitFor(() => expect(app.SaveStakeholder).toHaveBeenCalledOnce());
    expect(app.SaveStakeholder).toHaveBeenCalledWith(expect.objectContaining({
      hourly_rate_minor_units: exactHourlyMinorUnits,
      contract_value: 2000.25,
      contract_value_minor_units: 0,
    }));
  });

  it('does not replace or delete an open draft through background actions', async () => {
    let resolveSave!: (saved: Stakeholder) => void;
    app.SaveStakeholder = vi.fn(
      () => new Promise<Stakeholder>((resolve) => { resolveSave = resolve; }),
    );
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    const utils = render(StakeholderManager);
    await openEditor(utils);
    const nameInput = document.querySelector('[role="dialog"] input') as HTMLInputElement;
    await fireEvent.input(nameInput, { target: { value: 'Dirty draft' } });
    const backgroundOpen = Array.from(utils.container.querySelectorAll('button')).find(
      (button) => button.textContent?.includes('Ada Lovelace'),
    )!;
    const backgroundDelete = utils.container.querySelector('[aria-label="Delete stakeholder"]') as HTMLButtonElement;

    await fireEvent.click(backgroundOpen);
    await fireEvent.click(backgroundDelete);
    expect(nameInput.value).toBe('Dirty draft');
    expect(confirmSpy).not.toHaveBeenCalled();
    expect(app.DeleteStakeholder).not.toHaveBeenCalled();

    await fireEvent.click(editorButtons().find((button) => button.textContent?.trim() === 'Save')!);
    await waitFor(() => expect(app.SaveStakeholder).toHaveBeenCalledOnce());
    await fireEvent.click(backgroundOpen);
    await fireEvent.click(backgroundDelete);
    expect(app.DeleteStakeholder).not.toHaveBeenCalled();

    resolveSave({ ...stakeholder, name: 'Dirty draft' });
    await waitFor(() => expect(document.querySelector('[role="dialog"]')).toBeNull());
  });

  it('does not retain a dirty shared registration after confirmed close, reopen, or unmount', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm');
    const utils = render(StakeholderManager);
    await openEditor(utils);
    session.view = 'stakeholders';
    await fireEvent.input(document.querySelector('input') as HTMLInputElement, {
      target: { value: 'Dirty existing stakeholder' },
    });
    expect(autosave.hasDirty()).toBe(true);

    await fireEvent.click(editorButtons().find((button) => button.textContent?.trim() === 'Cancel')!);
    expect(navigation.pending?.view).toBe('stakeholders');
    await discardAndContinueNavigation();
    expect(confirmSpy).not.toHaveBeenCalled();
    expect(autosave.hasDirty()).toBe(false);

    await openNewEditor();
    await fireEvent.input(document.querySelector('[role="dialog"] input') as HTMLInputElement, {
      target: { value: 'Dirty new stakeholder' },
    });
    expect(autosave.hasDirty()).toBe(true);

    session.view = 'stakeholders';
    requestNavigation('dashboard');
    await saveAndContinueNavigation();
    expect(app.SaveStakeholder).toHaveBeenCalledOnce();
    expect(session.view).toBe('dashboard');
    expect(autosave.hasDirty()).toBe(false);

    await openNewEditor();
    await fireEvent.input(document.querySelector('[role="dialog"] input') as HTMLInputElement, {
      target: { value: 'Dirty unmounted stakeholder' },
    });
    expect(autosave.hasDirty()).toBe(true);

    utils.unmount();
    expect(autosave.hasDirty()).toBe(false);
  });

  it('regression: never calls window.confirm to guard a dirty close, and dismissal does not depend on its return value', async () => {
    // Wails v2.13.0's darwin WKUIDelegate implements none of the JS
    // confirm/alert/prompt panel methods (verified against the vendored
    // module source). The observed result in a packaged build is that
    // `window.confirm()` produces no dialog and the close silently no-ops
    // (2026-08-16 GUI evidence) — this is exactly what left Cancel/×/Escape
    // stuck on a dirty draft; the exact value WebKit's `confirm()` returns
    // was not independently confirmed. The assertion below (`not
    // .toHaveBeenCalled()`) is the actual regression guard; throwing from
    // the mock is belt-and-braces so a `confirm()` call surfaces loudly at
    // the call site too, unless a future refactor swallows it in a
    // try/catch.
    const confirmSpy = vi.spyOn(window, 'confirm').mockImplementation(() => {
      throw new Error('requestClose() must not call window.confirm() - Wails silently no-ops it on macOS');
    });
    const utils = render(StakeholderManager);
    await openEditor(utils);
    session.view = 'stakeholders';
    await fireEvent.input(document.querySelector('input') as HTMLInputElement, {
      target: { value: 'Regression check' },
    });

    const cancelBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Cancel',
    )!;
    await fireEvent.click(cancelBtn);

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(navigation.pending?.view).toBe('stakeholders');

    await discardAndContinueNavigation();

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(document.querySelector('input')).toBeNull();
  });
});

describe('StakeholderManager delete', () => {
  // Delete used `confirm()` — silently a no-op in the packaged macOS build
  // (Wails v2.13.0's darwin WKUIDelegate implements none of the JS
  // confirm/alert/prompt panel methods). Now routed through the shared
  // ConfirmDialog instead. The "does not replace or delete an open draft
  // through background actions" test above already covers the
  // `editing || busy` guard that runs before this dialog ever opens; these
  // cover the dialog path itself.
  it('opens the shared confirm dialog on the row delete button and does not call window.confirm', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm');
    const utils = render(StakeholderManager);
    await waitFor(() => expect(app.ListStakeholders).toHaveBeenCalled());
    const deleteBtn = await waitFor(
      () => utils.container.querySelector('[aria-label="Delete stakeholder"]') as HTMLButtonElement,
    );
    await fireEvent.click(deleteBtn);

    const dialog = document.querySelector('[role="alertdialog"]');
    expect(dialog).not.toBeNull();
    expect(dialog?.textContent).toContain('Ada Lovelace');
    expect(confirmSpy).not.toHaveBeenCalled();
    expect(app.DeleteStakeholder).not.toHaveBeenCalled();
  });

  it('deletes the stakeholder and refreshes the list when the dialog is confirmed', async () => {
    const utils = render(StakeholderManager);
    await waitFor(() => expect(app.ListStakeholders).toHaveBeenCalled());
    const deleteBtn = await waitFor(
      () => utils.container.querySelector('[aria-label="Delete stakeholder"]') as HTMLButtonElement,
    );
    await fireEvent.click(deleteBtn);
    const confirmBtn = Array.from(document.querySelectorAll('[role="alertdialog"] button')).find(
      (b) => b.textContent?.trim() === 'Delete',
    )!;
    await fireEvent.click(confirmBtn);

    await waitFor(() => expect(app.DeleteStakeholder).toHaveBeenCalledWith('sh-1'));
    await waitFor(() => expect(document.querySelector('[role="alertdialog"]')).toBeNull());
  });

  it('keeps the stakeholder when the dialog is cancelled', async () => {
    const utils = render(StakeholderManager);
    await waitFor(() => expect(app.ListStakeholders).toHaveBeenCalled());
    const deleteBtn = await waitFor(
      () => utils.container.querySelector('[aria-label="Delete stakeholder"]') as HTMLButtonElement,
    );
    await fireEvent.click(deleteBtn);
    const cancelBtn = Array.from(document.querySelectorAll('[role="alertdialog"] button')).find(
      (b) => b.textContent?.trim() === 'Cancel',
    )!;
    await fireEvent.click(cancelBtn);

    expect(document.querySelector('[role="alertdialog"]')).toBeNull();
    expect(app.DeleteStakeholder).not.toHaveBeenCalled();
  });
});
