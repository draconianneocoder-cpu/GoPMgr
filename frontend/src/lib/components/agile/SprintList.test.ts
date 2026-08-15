// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { cleanup, render, fireEvent, waitFor } from '@testing-library/svelte';

import SprintList from './SprintList.svelte';
import { autosave } from '../../autosave.svelte';
import {
  cancelNavigation,
  navigation,
  requestNavigation,
  saveAndContinueNavigation,
  session,
} from '../../session.svelte';
import { NativeCloseController } from '../../native-close';

let app: Record<string, ReturnType<typeof vi.fn>>;

const sprint: AgileSprint = {
  id: 'sp-1',
  project_id: 'p1',
  name: 'Sprint 1',
  goal: 'Ship the thing',
  status: 'planning',
  start_date: '2026-08-01',
  end_date: '2026-08-14',
  capacity: 10,
  created_at: '',
};

beforeEach(() => {
  session.project = { id: 'p1' } as unknown as typeof session.project;
  app = {
    ListSprints: vi.fn(async () => [sprint]),
    ListWorkItems: vi.fn(async () => []),
    SaveSprint: vi.fn(async (s: AgileSprint) => s),
    DeleteSprint: vi.fn(async () => undefined),
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
  await waitFor(() => expect(app.ListSprints).toHaveBeenCalled());
  const cardButtons = () =>
    Array.from(utils.container.querySelectorAll('button')).filter((b) =>
      b.textContent?.includes('Sprint 1'),
    );
  await waitFor(() => expect(cardButtons().length).toBe(1));
  await fireEvent.click(cardButtons()[0]);
  await waitFor(() => document.querySelector('input') as HTMLInputElement);
}

async function openNewEditor(): Promise<void> {
  const button = Array.from(document.querySelectorAll('button')).find(
    (candidate) => candidate.textContent?.trim() === '+ Sprint',
  )!;
  await fireEvent.click(button);
  await waitFor(() => expect(document.querySelector('input')).not.toBeNull());
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

describe('SprintList close guard', () => {
  it('closes without prompting when there are no unsaved edits', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    const utils = render(SprintList);
    await openEditor(utils);

    const cancelBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Cancel',
    )!;
    await fireEvent.click(cancelBtn);

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(document.querySelector('input')).toBeNull();
  });

  it('prompts to discard on Cancel when the name was edited, and keeps the modal open on decline', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    const utils = render(SprintList);
    await openEditor(utils);

    const nameInput = document.querySelector('input') as HTMLInputElement;
    await fireEvent.input(nameInput, { target: { value: 'Edited name' } });

    const cancelBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Cancel',
    )!;
    await fireEvent.click(cancelBtn);

    expect(confirmSpy).toHaveBeenCalledTimes(1);
    expect((document.querySelector('input') as HTMLInputElement).value).toBe('Edited name');
  });

  it('discards and closes on Cancel when the user confirms', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    const utils = render(SprintList);
    await openEditor(utils);

    const nameInput = document.querySelector('input') as HTMLInputElement;
    await fireEvent.input(nameInput, { target: { value: 'Edited name' } });

    const cancelBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Cancel',
    )!;
    await fireEvent.click(cancelBtn);

    expect(confirmSpy).toHaveBeenCalledTimes(1);
    expect(document.querySelector('input')).toBeNull();
    expect(app.SaveSprint).not.toHaveBeenCalled();
  });

  it('prompts on backdrop click and on the header close button when dirty', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm')
      .mockReturnValueOnce(false)
      .mockReturnValueOnce(true);
    const utils = render(SprintList);
    await openEditor(utils);

    const nameInput = document.querySelector('input') as HTMLInputElement;
    await fireEvent.input(nameInput, { target: { value: 'Edited via backdrop test' } });

    // The click-blocking element and the outer `fixed inset-0` handler are
    // different nodes here (unlike WorkItemEditor): the onclick listener
    // lives on the outer dialog div, but only fires `requestClose` when the
    // event target is the inner `[data-role="backdrop"]` sibling.
    const backdrop = document.querySelector('[data-role="backdrop"]') as HTMLElement;
    expect(backdrop).not.toBeNull();
    await fireEvent.click(backdrop);

    expect(confirmSpy).toHaveBeenCalledTimes(1);
    expect(document.querySelector('input')).not.toBeNull();

    await fireEvent.click(closeButton());

    expect(confirmSpy).toHaveBeenCalledTimes(2);
    expect(document.querySelector('input')).toBeNull();
  });

  it('prompts on Escape when dirty and does not close on decline', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    const utils = render(SprintList);
    await openEditor(utils);

    const nameInput = document.querySelector('input') as HTMLInputElement;
    await fireEvent.input(nameInput, { target: { value: 'Edited via escape test' } });

    const dialog = document.querySelector('[role="dialog"]') as HTMLElement;
    await fireEvent.keyDown(dialog, { key: 'Escape' });

    expect(confirmSpy).toHaveBeenCalledTimes(1);
    expect(document.querySelector('input')).not.toBeNull();
  });

  it('closes without prompting after a successful save even though the draft differs from the original', async () => {
    vi.spyOn(window, 'confirm');
    const utils = render(SprintList);
    await openEditor(utils);

    const nameInput = document.querySelector('input') as HTMLInputElement;
    await fireEvent.input(nameInput, { target: { value: 'Saved name' } });

    const saveBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Save',
    )!;
    await fireEvent.click(saveBtn);

    await waitFor(() => expect(app.SaveSprint).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(document.querySelector('input')).toBeNull());
    expect(window.confirm).not.toHaveBeenCalled();
  });

  it('ignores a close request made while a save is still in flight', async () => {
    let resolveSave!: (s: AgileSprint) => void;
    app.SaveSprint = vi.fn(
      () => new Promise<AgileSprint>((resolve) => { resolveSave = resolve; }),
    );
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    const utils = render(SprintList);
    await openEditor(utils);

    const nameInput = document.querySelector('input') as HTMLInputElement;
    await fireEvent.input(nameInput, { target: { value: 'Edited name' } });

    const saveBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Save',
    )!;
    await fireEvent.click(saveBtn);
    await waitFor(() => expect(app.SaveSprint).toHaveBeenCalledTimes(1));

    expect(closeButton().disabled).toBe(true);
    expect(editorButtons().find((button) => button.textContent?.trim() === 'Cancel')?.disabled).toBe(true);
    for (const control of document.querySelectorAll('[role="dialog"] input, [role="dialog"] textarea')) {
      expect((control as HTMLInputElement | HTMLTextAreaElement).disabled).toBe(true);
    }
    await fireEvent.click(saveBtn);
    expect(app.SaveSprint).toHaveBeenCalledTimes(1);

    const cancelBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Cancel',
    )!;
    await fireEvent.click(cancelBtn);

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(document.querySelector('input')).not.toBeNull();

    resolveSave({ ...sprint, name: 'Edited name' });
    await waitFor(() => expect(document.querySelector('input')).toBeNull());
  });

  it('saves a dirty sprint through guarded navigation before continuing', async () => {
    const utils = render(SprintList);
    await openEditor(utils);
    const nameInput = document.querySelector('input') as HTMLInputElement;
    await fireEvent.input(nameInput, { target: { value: 'Saved by navigation' } });

    session.view = 'sprints';
    requestNavigation('dashboard');
    expect(navigation.pending?.view).toBe('dashboard');

    await saveAndContinueNavigation();

    expect(app.SaveSprint).toHaveBeenCalledWith(expect.objectContaining({
      id: 'sp-1',
      name: 'Saved by navigation',
    }));
    expect(document.querySelector('input')).toBeNull();
    expect(navigation.pending).toBeNull();
    expect(session.view).toBe('dashboard');
  });

  it('keeps native close pending after a failed save and completes it after retry', async () => {
    app.SaveSprint = vi.fn()
      .mockRejectedValueOnce(new Error('bridge unavailable'))
      .mockResolvedValueOnce({ ...sprint, name: 'Retry saved' });
    const utils = render(SprintList);
    await openEditor(utils);
    await fireEvent.input(document.querySelector('input') as HTMLInputElement, {
      target: { value: 'Retry saved' },
    });
    const { app: nativeApp, controller } = nativeCloseController();

    controller.request();
    expect(navigation.pending).not.toBeNull();
    await saveAndContinueNavigation();

    expect(app.SaveSprint).toHaveBeenCalledTimes(1);
    expect(document.querySelector('input')).not.toBeNull();
    expect(autosave.hasDirty()).toBe(true);
    expect(navigation.pending).not.toBeNull();
    expect(nativeApp.CompleteNativeClose).not.toHaveBeenCalled();

    await saveAndContinueNavigation();
    await waitFor(() => expect(nativeApp.CompleteNativeClose).toHaveBeenCalledOnce());
    expect(app.SaveSprint).toHaveBeenCalledTimes(2);
  });

  it('preserves programmatic edits made during a save for a second guarded save', async () => {
    let finishFirstSave!: (saved: AgileSprint) => void;
    const firstSaved: AgileSprint = {
      id: 'sp-server',
      project_id: 'server-project',
      name: 'Initial sprint',
      goal: 'Initial goal',
      status: 'active',
      start_date: '2026-08-01',
      end_date: '2026-08-14',
      capacity: 5,
      created_at: '2026-08-15T00:00:00Z',
    };
    app.SaveSprint = vi.fn()
      .mockImplementationOnce(() => new Promise<AgileSprint>((resolve) => { finishFirstSave = resolve; }))
      .mockResolvedValueOnce(firstSaved);
    render(SprintList);
    await openNewEditor();
    const inputs = document.querySelectorAll('[role="dialog"] input');
    const nameInput = inputs[0] as HTMLInputElement;
    const goalInput = document.querySelector('[role="dialog"] textarea') as HTMLTextAreaElement;
    await fireEvent.input(nameInput, { target: { value: 'Initial sprint' } });
    await fireEvent.input(goalInput, { target: { value: 'Initial goal' } });

    session.view = 'sprints';
    requestNavigation('dashboard');
    const continuing = saveAndContinueNavigation();
    await waitFor(() => expect(app.SaveSprint).toHaveBeenCalledOnce());
    const firstPayload = app.SaveSprint.mock.calls[0][0] as AgileSprint;
    const expectedFirstPayload = { ...firstPayload };
    expect(firstPayload).toMatchObject({
      id: '',
      project_id: 'p1',
      name: 'Initial sprint',
      goal: 'Initial goal',
    });
    expect(nameInput.disabled).toBe(true);

    await fireEvent.input(nameInput, { target: { value: 'Late sprint' } });
    await fireEvent.input(goalInput, { target: { value: 'Late goal' } });
    await fireEvent.input(inputs[1] as HTMLInputElement, { target: { value: '2026-09-01' } });
    await fireEvent.input(inputs[2] as HTMLInputElement, { target: { value: '2026-09-21' } });
    await fireEvent.input(inputs[3] as HTMLInputElement, { target: { value: '13.5' } });
    expect(firstPayload).toEqual(expectedFirstPayload);
    finishFirstSave(firstSaved);
    await continuing;

    expect((document.querySelector('[role="dialog"] input') as HTMLInputElement).value).toBe('Late sprint');
    expect(navigation.pending?.view).toBe('dashboard');
    expect(navigation.error).toContain('Changes were made while saving');
    expect(autosave.hasDirty()).toBe(true);

    await saveAndContinueNavigation();

    expect(app.SaveSprint).toHaveBeenCalledTimes(2);
    expect(app.SaveSprint).toHaveBeenLastCalledWith({
      id: 'sp-server',
      project_id: 'server-project',
      name: 'Late sprint',
      goal: 'Late goal',
      status: 'active',
      start_date: '2026-09-01',
      end_date: '2026-09-21',
      capacity: 13.5,
      created_at: '2026-08-15T00:00:00Z',
    });
    expect(document.querySelector('input')).toBeNull();
    expect(session.view).toBe('dashboard');
  });

  it('rejects a whitespace-only name through guarded save as well as the disabled button', async () => {
    render(SprintList);
    await openNewEditor();
    const nameInput = document.querySelector('[role="dialog"] input') as HTMLInputElement;
    await fireEvent.input(nameInput, { target: { value: '   ' } });
    const saveButton = editorButtons().find((button) => button.textContent?.trim() === 'Save')!;
    expect(saveButton.disabled).toBe(true);

    session.view = 'sprints';
    requestNavigation('dashboard');
    await saveAndContinueNavigation();

    expect(app.SaveSprint).not.toHaveBeenCalled();
    expect(navigation.pending?.view).toBe('dashboard');
    expect(document.querySelector('[role="alert"]')?.textContent).toContain('Sprint name is required');
  });

  it('does not retain a dirty shared registration after confirmed close, reopen, or unmount', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    const utils = render(SprintList);
    await openEditor(utils);
    await fireEvent.input(document.querySelector('input') as HTMLInputElement, {
      target: { value: 'Dirty existing sprint' },
    });
    expect(autosave.hasDirty()).toBe(true);

    await fireEvent.click(editorButtons().find((button) => button.textContent?.trim() === 'Cancel')!);
    expect(confirmSpy).toHaveBeenCalledOnce();
    expect(autosave.hasDirty()).toBe(false);

    await openNewEditor();
    expect(autosave.hasDirty()).toBe(false);
    await fireEvent.input(document.querySelector('[role="dialog"] input') as HTMLInputElement, {
      target: { value: 'Dirty new sprint' },
    });
    expect(autosave.hasDirty()).toBe(true);

    session.view = 'sprints';
    requestNavigation('dashboard');
    await saveAndContinueNavigation();
    expect(app.SaveSprint).toHaveBeenCalledOnce();
    expect(session.view).toBe('dashboard');
    expect(autosave.hasDirty()).toBe(false);

    await openNewEditor();
    await fireEvent.input(document.querySelector('[role="dialog"] input') as HTMLInputElement, {
      target: { value: 'Dirty unmounted sprint' },
    });
    expect(autosave.hasDirty()).toBe(true);

    utils.unmount();
    expect(autosave.hasDirty()).toBe(false);
  });
});
