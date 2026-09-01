// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { cleanup, render, fireEvent, waitFor } from '@testing-library/svelte';

import SprintList from './SprintList.svelte';
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
  vi.useRealTimers();
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

  it('opens the shared unsaved-changes guard on Cancel when the name was edited, and keeps the modal open when declined', async () => {
    // Wails v2.15.0's darwin WKUIDelegate implements none of the JS
    // confirm/alert/prompt panel methods (verified: no `runJavaScript*Panel`
    // implementation anywhere in the vendored module). The observed result
    // in a packaged build is that `window.confirm()` produces no dialog and
    // the close silently no-ops (2026-08-16 GUI evidence) — the exact value
    // WebKit's `confirm()` returns was not independently confirmed, only
    // that no dialog appears. Mocking it to return `false` reproduces the
    // observed symptom; the close guard must not depend on this return
    // value at all.
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    const utils = render(SprintList);
    await openEditor(utils);
    session.view = 'sprints';

    const nameInput = document.querySelector('input') as HTMLInputElement;
    await fireEvent.input(nameInput, { target: { value: 'Edited name' } });

    const cancelBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Cancel',
    )!;
    await fireEvent.click(cancelBtn);

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(navigation.pending?.view).toBe('sprints');
    expect((document.querySelector('input') as HTMLInputElement).value).toBe('Edited name');

    cancelNavigation();
    expect((document.querySelector('input') as HTMLInputElement).value).toBe('Edited name');
  });

  it('discards and closes on Cancel when the shared guard is told to discard', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    const utils = render(SprintList);
    await openEditor(utils);
    session.view = 'sprints';

    const nameInput = document.querySelector('input') as HTMLInputElement;
    await fireEvent.input(nameInput, { target: { value: 'Edited name' } });

    const cancelBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Cancel',
    )!;
    await fireEvent.click(cancelBtn);
    expect(navigation.pending?.view).toBe('sprints');

    await discardAndContinueNavigation();

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(document.querySelector('input')).toBeNull();
    expect(app.SaveSprint).not.toHaveBeenCalled();
  });

  it('prompts via the shared guard on backdrop click and on the header close button when dirty', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    const utils = render(SprintList);
    await openEditor(utils);
    session.view = 'sprints';

    const nameInput = document.querySelector('input') as HTMLInputElement;
    await fireEvent.input(nameInput, { target: { value: 'Edited via backdrop test' } });

    // The click-blocking element and the outer `fixed inset-0` handler are
    // different nodes here (unlike WorkItemEditor): the onclick listener
    // lives on the outer dialog div, but only fires `requestClose` when the
    // event target is the inner `[data-role="backdrop"]` sibling.
    const backdrop = document.querySelector('[data-role="backdrop"]') as HTMLElement;
    expect(backdrop).not.toBeNull();
    await fireEvent.click(backdrop);

    expect(navigation.pending?.view).toBe('sprints');
    expect(document.querySelector('input')).not.toBeNull();

    cancelNavigation();

    await fireEvent.click(closeButton());
    expect(navigation.pending?.view).toBe('sprints');
    await discardAndContinueNavigation();

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(document.querySelector('input')).toBeNull();
  });

  it('prompts via the shared guard on Escape when dirty and does not close on decline', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    const utils = render(SprintList);
    await openEditor(utils);
    session.view = 'sprints';

    const nameInput = document.querySelector('input') as HTMLInputElement;
    await fireEvent.input(nameInput, { target: { value: 'Edited via escape test' } });

    const dialog = document.querySelector('[role="dialog"]') as HTMLElement;
    await fireEvent.keyDown(dialog, { key: 'Escape' });

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(navigation.pending?.view).toBe('sprints');
    expect(document.querySelector('input')).not.toBeNull();

    cancelNavigation();
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
    // openNew() below stamps its default start/end dates from the real
    // wall clock. This test also types a fixed '2026-09-01' start-date
    // literal into the mid-flight edit further down -- pin the clock so
    // that default can never collide with the literal (it did on the real
    // 2026-09-01, which made rebaseEditableChanges's untouched-field
    // fallback silently mask a real edit; see the fixture note below).
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-08-15T00:00:00Z'));
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
    const confirmSpy = vi.spyOn(window, 'confirm');
    const utils = render(SprintList);
    await openEditor(utils);
    session.view = 'sprints';
    await fireEvent.input(document.querySelector('input') as HTMLInputElement, {
      target: { value: 'Dirty existing sprint' },
    });
    expect(autosave.hasDirty()).toBe(true);

    await fireEvent.click(editorButtons().find((button) => button.textContent?.trim() === 'Cancel')!);
    expect(navigation.pending?.view).toBe('sprints');
    await discardAndContinueNavigation();
    expect(confirmSpy).not.toHaveBeenCalled();
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

  it('regression: never calls window.confirm to guard a dirty close, and dismissal does not depend on its return value', async () => {
    // Wails v2.15.0's darwin WKUIDelegate implements none of the JS
    // confirm/alert/prompt panel methods (verified against the vendored
    // module source). The observed result in a packaged build is that
    // `window.confirm()` produces no dialog and the close silently no-ops
    // — this is the same defect class confirmed in StakeholderManager's
    // 2026-08-16 GUI evidence; the exact value WebKit's `confirm()` returns
    // was not independently confirmed. The assertion below (`not
    // .toHaveBeenCalled()`) is the actual regression guard; throwing from
    // the mock is belt-and-braces so a `confirm()` call surfaces loudly at
    // the call site too.
    const confirmSpy = vi.spyOn(window, 'confirm').mockImplementation(() => {
      throw new Error('requestClose() must not call window.confirm() - Wails silently no-ops it on macOS');
    });
    const utils = render(SprintList);
    await openEditor(utils);
    session.view = 'sprints';
    await fireEvent.input(document.querySelector('input') as HTMLInputElement, {
      target: { value: 'Regression check' },
    });

    const cancelBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Cancel',
    )!;
    await fireEvent.click(cancelBtn);

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(navigation.pending?.view).toBe('sprints');

    await discardAndContinueNavigation();

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(document.querySelector('input')).toBeNull();
  });
});

function findButton(root: ParentNode, text: string): HTMLButtonElement {
  const btn = Array.from(root.querySelectorAll('button')).find((b) => b.textContent?.trim() === text);
  if (!btn) throw new Error(`no button with text "${text}" found`);
  return btn as HTMLButtonElement;
}

describe('SprintList complete/delete', () => {
  // Complete/Delete both used `confirm()` — silently a no-op in the
  // packaged macOS build (Wails v2.15.0's darwin WKUIDelegate implements
  // none of the JS confirm/alert/prompt panel methods). Now routed through
  // the shared ConfirmDialog instead.
  it('opens the shared confirm dialog on Delete and does not call window.confirm', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm');
    const utils = render(SprintList);
    await waitFor(() => expect(app.ListSprints).toHaveBeenCalled());
    await waitFor(() => expect(() => findButton(utils.container, 'Delete')).not.toThrow());
    // Migrated from a raw <button> to `Button variant="remove"` — pins the
    // exact rendered class string so the row's appearance is provably
    // unchanged by the migration, not just assumed safe.
    expect(findButton(utils.container, 'Delete').className.trim()).toBe(
      'text-slate-500 hover:text-red-400 disabled:opacity-50 text-xs',
    );
    await fireEvent.click(findButton(utils.container, 'Delete'));

    const dialog = document.querySelector('[role="alertdialog"]');
    expect(dialog).not.toBeNull();
    expect(dialog?.textContent).toContain('Sprint 1');
    expect(confirmSpy).not.toHaveBeenCalled();
    expect(app.DeleteSprint).not.toHaveBeenCalled();
  });

  it('deletes the sprint when the delete dialog is confirmed', async () => {
    const utils = render(SprintList);
    await waitFor(() => expect(() => findButton(utils.container, 'Delete')).not.toThrow());
    await fireEvent.click(findButton(utils.container, 'Delete'));
    await waitFor(() => expect(document.querySelector('[role="alertdialog"]')).not.toBeNull());
    await fireEvent.click(findButton(document.querySelector('[role="alertdialog"]')!, 'Delete'));

    await waitFor(() => expect(app.DeleteSprint).toHaveBeenCalledWith('sp-1'));
    await waitFor(() => expect(document.querySelector('[role="alertdialog"]')).toBeNull());
  });

  it('keeps the sprint when the delete dialog is cancelled', async () => {
    const utils = render(SprintList);
    await waitFor(() => expect(() => findButton(utils.container, 'Delete')).not.toThrow());
    await fireEvent.click(findButton(utils.container, 'Delete'));
    await waitFor(() => expect(document.querySelector('[role="alertdialog"]')).not.toBeNull());
    await fireEvent.click(findButton(document.querySelector('[role="alertdialog"]')!, 'Cancel'));

    expect(document.querySelector('[role="alertdialog"]')).toBeNull();
    expect(app.DeleteSprint).not.toHaveBeenCalled();
  });

  it('opens a caution-toned confirm dialog on Complete for an active sprint and does not call window.confirm', async () => {
    const activeSprint: AgileSprint = { ...sprint, status: 'active' };
    app.ListSprints = vi.fn(async () => [activeSprint]);
    const confirmSpy = vi.spyOn(window, 'confirm');
    const utils = render(SprintList);
    await waitFor(() => expect(() => findButton(utils.container, 'Complete')).not.toThrow());
    await fireEvent.click(findButton(utils.container, 'Complete'));

    await waitFor(() => expect(document.querySelector('[role="alertdialog"]')).not.toBeNull());
    const dialog = document.querySelector('[role="alertdialog"]')!;
    expect(dialog.textContent).toContain('Sprint 1');
    const confirmBtn = findButton(dialog, 'Complete');
    expect(confirmBtn.className).toContain('bg-amber-700');
    expect(confirmSpy).not.toHaveBeenCalled();
    expect(app.SaveSprint).not.toHaveBeenCalled();
  });

  it('completes the sprint via SaveSprint when the complete dialog is confirmed', async () => {
    const activeSprint: AgileSprint = { ...sprint, status: 'active' };
    app.ListSprints = vi.fn(async () => [activeSprint]);
    const utils = render(SprintList);
    await waitFor(() => expect(() => findButton(utils.container, 'Complete')).not.toThrow());
    await fireEvent.click(findButton(utils.container, 'Complete'));
    await waitFor(() => expect(document.querySelector('[role="alertdialog"]')).not.toBeNull());
    await fireEvent.click(findButton(document.querySelector('[role="alertdialog"]')!, 'Complete'));

    await waitFor(() =>
      expect(app.SaveSprint).toHaveBeenCalledWith(expect.objectContaining({ id: 'sp-1', status: 'complete' })),
    );
    await waitFor(() => expect(document.querySelector('[role="alertdialog"]')).toBeNull());
  });
});

// Added 2026-08-19 alongside migrating the header "&larr; Dashboard" nav
// link (previously a raw <button>) to the new `nav` Button variant — see
// Button.svelte's `nav` branch comment.
describe('SprintList migrated header "&larr; Dashboard" button', () => {
  it('Button variant="nav"', async () => {
    const utils = render(SprintList);
    await waitFor(() => expect(app.ListSprints).toHaveBeenCalled());

    const btn = Array.from(utils.container.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === '← Dashboard',
    ) as HTMLButtonElement;
    expect(btn).toBeDefined();
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'text-xs text-slate-400 hover:text-cyan-400 disabled:opacity-50'.split(/\s+/).sort(),
    );
  });
});
