// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { cleanup, render, fireEvent, waitFor } from '@testing-library/svelte';

import WorkItemEditor from './WorkItemEditor.svelte';
import { autosave } from '../../autosave.svelte';
import {
  cancelNavigation,
  navigation,
  requestNavigation,
  saveAndContinueNavigation,
  session,
} from '../../session.svelte';
import { NativeCloseController } from '../../native-close';

function makeItem(overrides: Partial<AgileWorkItem> = {}): AgileWorkItem {
  return {
    id: 'wi-1',
    project_id: 'p1',
    type: 'story',
    title: 'Original title',
    description: '',
    state: 'col-1',
    points: 0,
    assignee: '',
    sprint_id: '',
    priority: 'medium',
    order_idx: 0,
    created_at: '',
    updated_at: '',
    ...overrides,
  };
}

let app: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  app = {
    SaveWorkItem: vi.fn(async (wi: AgileWorkItem) => wi),
    DeleteWorkItem: vi.fn(async () => undefined),
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
});

describe('WorkItemEditor close guard', () => {
  it('closes without prompting when there are no unsaved edits', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    const onClose = vi.fn();
    render(WorkItemEditor, {
      item: makeItem(),
      onClose,
      onSaved: vi.fn(),
      onDeleted: vi.fn(),
    });

    const cancelBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Cancel',
    )!;
    await fireEvent.click(cancelBtn);

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('prompts to discard on Cancel when the title was edited, and keeps the modal open on decline', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    const onClose = vi.fn();
    const utils = render(WorkItemEditor, {
      item: makeItem(),
      onClose,
      onSaved: vi.fn(),
      onDeleted: vi.fn(),
    });

    const titleInput = utils.container.querySelector('input') as HTMLInputElement;
    await fireEvent.input(titleInput, { target: { value: 'Edited title' } });

    const cancelBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Cancel',
    )!;
    await fireEvent.click(cancelBtn);

    expect(confirmSpy).toHaveBeenCalledTimes(1);
    expect(onClose).not.toHaveBeenCalled();
    // The draft survives the declined discard.
    expect((utils.container.querySelector('input') as HTMLInputElement).value).toBe('Edited title');
  });

  it('discards and closes on Cancel when the user confirms', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    const onClose = vi.fn();
    const utils = render(WorkItemEditor, {
      item: makeItem(),
      onClose,
      onSaved: vi.fn(),
      onDeleted: vi.fn(),
    });

    const titleInput = utils.container.querySelector('input') as HTMLInputElement;
    await fireEvent.input(titleInput, { target: { value: 'Edited title' } });

    const cancelBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Cancel',
    )!;
    await fireEvent.click(cancelBtn);

    expect(confirmSpy).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(app.SaveWorkItem).not.toHaveBeenCalled();
  });

  it('prompts on backdrop click and on the header close button when dirty', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm')
      .mockReturnValueOnce(false)
      .mockReturnValueOnce(true);
    const onClose = vi.fn();
    const utils = render(WorkItemEditor, {
      item: makeItem(),
      onClose,
      onSaved: vi.fn(),
      onDeleted: vi.fn(),
    });

    const titleInput = utils.container.querySelector('input') as HTMLInputElement;
    await fireEvent.input(titleInput, { target: { value: 'Edited via backdrop test' } });

    const backdrop = utils.container.querySelector('[data-role="backdrop"]') as HTMLElement;
    await fireEvent.click(backdrop);
    expect(onClose).not.toHaveBeenCalled();

    await fireEvent.click(utils.getByRole('button', { name: 'Close' }));

    expect(confirmSpy).toHaveBeenCalledTimes(2);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('prompts on Escape when dirty and does not close on decline', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    const onClose = vi.fn();
    const utils = render(WorkItemEditor, {
      item: makeItem(),
      onClose,
      onSaved: vi.fn(),
      onDeleted: vi.fn(),
    });

    const titleInput = utils.container.querySelector('input') as HTMLInputElement;
    titleInput.focus();
    await fireEvent.input(titleInput, { target: { value: 'Edited via escape test' } });

    const backdrop = utils.container.querySelector('[data-role="backdrop"]') as HTMLElement;
    await fireEvent.keyDown(backdrop, { key: 'Escape' });

    expect(confirmSpy).toHaveBeenCalledTimes(1);
    expect(onClose).not.toHaveBeenCalled();
  });

  it('closes without prompting after a successful save even though the draft differs from the original item', async () => {
    vi.spyOn(window, 'confirm');
    const onClose = vi.fn();
    const onSaved = vi.fn();
    const utils = render(WorkItemEditor, {
      item: makeItem(),
      onClose,
      onSaved,
      onDeleted: vi.fn(),
    });

    const titleInput = utils.container.querySelector('input') as HTMLInputElement;
    await fireEvent.input(titleInput, { target: { value: 'Saved title' } });

    const saveBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Save',
    )!;
    await fireEvent.click(saveBtn);

    await waitFor(() => expect(app.SaveWorkItem).toHaveBeenCalledTimes(1));
    expect(onSaved).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(window.confirm).not.toHaveBeenCalled();
  });

  it('ignores a close request made while a save is still in flight', async () => {
    let resolveSave!: (wi: AgileWorkItem) => void;
    app.SaveWorkItem = vi.fn(
      () => new Promise<AgileWorkItem>((resolve) => { resolveSave = resolve; }),
    );
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    const onClose = vi.fn();
    const utils = render(WorkItemEditor, {
      item: makeItem(),
      onClose,
      onSaved: vi.fn(),
      onDeleted: vi.fn(),
    });

    const titleInput = utils.container.querySelector('input') as HTMLInputElement;
    await fireEvent.input(titleInput, { target: { value: 'Edited title' } });

    const saveBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Save',
    )!;
    await fireEvent.click(saveBtn);
    await waitFor(() => expect(app.SaveWorkItem).toHaveBeenCalledTimes(1));
    expect(titleInput.disabled).toBe(true);
    expect(saveBtn.disabled).toBe(true);

    // The keyboard shortcut is another path to save(). While the first
    // request is pending it must not issue a duplicate persistence request.
    const backdrop = utils.container.querySelector('[data-role="backdrop"]') as HTMLElement;
    await fireEvent.keyDown(backdrop, { key: 'Enter', ctrlKey: true });
    expect(app.SaveWorkItem).toHaveBeenCalledTimes(1);
    expect(app.SaveWorkItem).toHaveBeenCalledWith(expect.objectContaining({ title: 'Edited title' }));

    // A close requested while SaveWorkItem is still pending must be a
    // no-op, not a discard-confirm dialog that could fire before the save
    // resolves and persists the very edits being "discarded".
    const cancelBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Cancel',
    )!;
    await fireEvent.click(cancelBtn);

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();

    resolveSave({ ...makeItem(), title: 'Edited title' });
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
  });

  it('keeps a dirty draft pending during guarded navigation until its snapshot saves', async () => {
    const onClose = vi.fn();
    const onSaved = vi.fn();
    const utils = render(WorkItemEditor, {
      item: makeItem(),
      onClose,
      onSaved,
      onDeleted: vi.fn(),
    });
    const titleInput = utils.container.querySelector('input') as HTMLInputElement;
    await fireEvent.input(titleInput, { target: { value: 'Saved by guarded navigation' } });

    session.view = 'backlog';
    requestNavigation('portfolio');
    expect(navigation.pending?.view).toBe('portfolio');
    expect(session.view).toBe('backlog');

    await saveAndContinueNavigation();

    expect(app.SaveWorkItem).toHaveBeenCalledWith(expect.objectContaining({
      title: 'Saved by guarded navigation',
    }));
    expect(onSaved).toHaveBeenCalledOnce();
    expect(onClose).toHaveBeenCalledOnce();
    expect(session.view).toBe('portfolio');
    expect(navigation.pending).toBeNull();
  });

  it('keeps a failed native-close save dirty and permits close only after retry', async () => {
    app.SaveWorkItem = vi.fn()
      .mockRejectedValueOnce(new Error('disk full'))
      .mockResolvedValueOnce(makeItem({ title: 'Retried work item' }));
    const onClose = vi.fn();
    const onSaved = vi.fn();
    const utils = render(WorkItemEditor, {
      item: makeItem(),
      onClose,
      onSaved,
      onDeleted: vi.fn(),
    });
    const titleInput = utils.container.querySelector('input') as HTMLInputElement;
    await fireEvent.input(titleInput, { target: { value: 'Retried work item' } });

    const nativeApp = {
      EnableNativeCloseGuard: vi.fn().mockResolvedValue(undefined),
      CompleteNativeClose: vi.fn().mockResolvedValue(undefined),
    };
    const controller = new NativeCloseController({
      app: nativeApp,
      reportError: vi.fn(),
      setInteractionLocked: vi.fn(),
    });
    session.view = 'kanban';
    controller.request();
    expect(navigation.pending?.view).toBe('kanban');
    expect(nativeApp.CompleteNativeClose).not.toHaveBeenCalled();

    await saveAndContinueNavigation();

    expect(navigation.pending?.view).toBe('kanban');
    expect(navigation.error).toContain('reported that the save failed');
    expect(utils.getByRole('alert').textContent).toContain('disk full');
    expect(titleInput.value).toBe('Retried work item');
    expect(onSaved).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
    expect(nativeApp.CompleteNativeClose).not.toHaveBeenCalled();
    expect(autosave.hasDirty()).toBe(true);

    await saveAndContinueNavigation();

    expect(onSaved).toHaveBeenCalledOnce();
    expect(onClose).toHaveBeenCalledOnce();
    await waitFor(() => expect(nativeApp.CompleteNativeClose).toHaveBeenCalledOnce());
  });

  it('keeps a late edit open and dirty after guarded save rebases a newly-created item', async () => {
    let finishFirstSave!: (saved: AgileWorkItem) => void;
    const columns: AgileColumn[] = [{
      id: 'col-late', board_id: 'board-1', name: 'Late column', order_idx: 0, wip_limit: 0,
    }];
    const sprints: AgileSprint[] = [{
      id: 'sprint-late', project_id: 'p1', name: 'Late sprint', goal: '', status: 'planning',
      start_date: '', end_date: '', capacity: 0, created_at: '',
    }];
    const firstSaved = makeItem({
      id: 'wi-new',
      project_id: 'server-project',
      title: 'First saved draft',
      description: 'server description',
      state: 'server-column',
      points: 1,
      assignee: 'server assignee',
      sprint_id: 'server-sprint',
      priority: 'medium',
      order_idx: 77,
      created_at: '2026-08-15T00:00:00Z',
      updated_at: '2026-08-15T00:01:00Z',
      closed_at: '2026-08-15T00:02:00Z',
    });
    app.SaveWorkItem = vi.fn()
      .mockImplementationOnce(() => new Promise<AgileWorkItem>((resolve) => { finishFirstSave = resolve; }))
      .mockResolvedValueOnce(firstSaved);
    const onClose = vi.fn();
    const onSaved = vi.fn();
    const utils = render(WorkItemEditor, {
      item: makeItem({ id: '', title: 'First draft', state: 'backlog' }),
      columns,
      sprints,
      onClose,
      onSaved,
      onDeleted: vi.fn(),
    });
    const titleInput = utils.container.querySelector('input') as HTMLInputElement;
    await fireEvent.input(titleInput, { target: { value: 'First saved draft' } });
    session.view = 'backlog';
    requestNavigation('portfolio');
    const continuing = saveAndContinueNavigation();
    await waitFor(() => expect(app.SaveWorkItem).toHaveBeenCalledOnce());
    expect(titleInput.disabled).toBe(true);
    expect(app.SaveWorkItem).toHaveBeenCalledWith(expect.objectContaining({
      id: '',
      title: 'First saved draft',
    }));

    // A real user cannot type into the disabled control, but this simulates a
    // future programmatic state update while the bridge request is pending.
    await fireEvent.input(titleInput, { target: { value: 'Edited during save' } });
    const inputs = utils.container.querySelectorAll('input');
    const selects = utils.container.querySelectorAll('select');
    await fireEvent.change(selects[0], { target: { value: 'bug' } });
    await fireEvent.change(selects[1], { target: { value: 'urgent' } });
    await fireEvent.input(inputs[1], { target: { value: '13' } });
    await fireEvent.input(inputs[2], { target: { value: 'late assignee' } });
    await fireEvent.change(selects[2], { target: { value: 'col-late' } });
    await fireEvent.change(selects[3], { target: { value: 'sprint-late' } });
    await fireEvent.input(utils.container.querySelector('textarea')!, {
      target: { value: 'late description' },
    });
    finishFirstSave(firstSaved);
    await continuing;

    expect(onSaved).toHaveBeenCalledWith(firstSaved);
    expect(onClose).not.toHaveBeenCalled();
    expect(titleInput.value).toBe('Edited during save');
    expect(navigation.pending?.view).toBe('portfolio');
    expect(navigation.error).toContain('Changes were made while saving');
    expect(autosave.hasDirty()).toBe(true);

    await saveAndContinueNavigation();

    expect(app.SaveWorkItem).toHaveBeenCalledTimes(2);
    expect(app.SaveWorkItem).toHaveBeenLastCalledWith(expect.objectContaining({
      id: 'wi-new',
      project_id: 'server-project',
      type: 'bug',
      priority: 'urgent',
      title: 'Edited during save',
      description: 'late description',
      state: 'col-late',
      points: 13,
      assignee: 'late assignee',
      sprint_id: 'sprint-late',
      order_idx: 77,
      created_at: '2026-08-15T00:00:00Z',
      updated_at: '2026-08-15T00:01:00Z',
      closed_at: '2026-08-15T00:02:00Z',
    }));
    expect(onClose).toHaveBeenCalledOnce();
    expect(session.view).toBe('portfolio');
  });

  it('rejects whitespace-only titles through guarded save as well as the disabled button', async () => {
    const utils = render(WorkItemEditor, {
      item: makeItem({ title: 'Valid title' }),
      onClose: vi.fn(),
      onSaved: vi.fn(),
      onDeleted: vi.fn(),
    });
    const saveBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Save',
    )!;
    const titleInput = utils.container.querySelector('input') as HTMLInputElement;
    await fireEvent.input(titleInput, { target: { value: '   ' } });
    expect(saveBtn.disabled).toBe(true);

    session.view = 'backlog';
    requestNavigation('portfolio');
    expect(navigation.pending?.view).toBe('portfolio');
    await saveAndContinueNavigation();

    expect(app.SaveWorkItem).not.toHaveBeenCalled();
    expect(navigation.pending?.view).toBe('portfolio');
    expect(utils.getByRole('alert').textContent).toContain('Title is required');
  });

  it('does not retain a dirty guard when the reused modal changes item, closes, or unmounts', async () => {
    const props = {
      item: makeItem({ id: 'wi-a' }),
      onClose: vi.fn(),
      onSaved: vi.fn(),
      onDeleted: vi.fn(),
    };
    const utils = render(WorkItemEditor, props);
    const titleInput = utils.container.querySelector('input') as HTMLInputElement;
    await fireEvent.input(titleInput, { target: { value: 'Dirty first item' } });
    expect(autosave.hasDirty()).toBe(true);

    await utils.rerender({ ...props, item: makeItem({ id: 'wi-b', title: 'Second item' }) });
    expect(autosave.hasDirty()).toBe(false);
    session.view = 'backlog';
    requestNavigation('portfolio');
    await waitFor(() => expect(session.view).toBe('portfolio'));
    expect(navigation.pending).toBeNull();

    await utils.rerender({ ...props, item: null });
    expect(autosave.hasDirty()).toBe(false);
    await utils.rerender({ ...props, item: makeItem({ id: '', title: '' }) });
    const newTitleInput = utils.container.querySelector('input') as HTMLInputElement;
    await fireEvent.input(newTitleInput, { target: { value: 'New draft' } });
    expect(autosave.hasDirty()).toBe(true);

    utils.unmount();
    expect(autosave.hasDirty()).toBe(false);
  });
});
