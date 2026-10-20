// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { cleanup, render, fireEvent, waitFor, within } from '@testing-library/svelte';

import WorkItemEditor from './WorkItemEditor.svelte';
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
  it.each(['manual', 'guarded'])('encodes empty Go timestamps on %s save without mutating the draft prop', async (mode) => {
    const item = makeItem({ id: '', closed_at: '' });
    const before = { ...item };
    const onSaved = vi.fn();
    const onClose = vi.fn();
    const utils = render(WorkItemEditor, { item, onSaved, onClose, onDeleted: vi.fn() });
    await fireEvent.input(utils.container.querySelector('input')!, { target: { value: 'Native create' } });

    if (mode === 'guarded') {
      requestNavigation('portfolio');
      await saveAndContinueNavigation();
    } else {
      await fireEvent.click(utils.getByRole('button', { name: 'Save', exact: true }));
    }

    expect(app.SaveWorkItem).toHaveBeenCalledExactlyOnceWith({
      ...item,
      title: 'Native create',
      created_at: '0001-01-01T00:00:00Z',
      updated_at: '0001-01-01T00:00:00Z',
      closed_at: '0001-01-01T00:00:00Z',
    });
    expect(item).toEqual(before);
    expect(onSaved).toHaveBeenCalledOnce();
    expect(onClose).toHaveBeenCalledOnce();
  });

  it.each([
    {
      name: 'mixed empty and supplied timestamps with no closed_at',
      timestamps: { created_at: '', updated_at: '2026-10-16T15:30:01.123456789-05:00' },
      expected: { created_at: '0001-01-01T00:00:00Z', updated_at: '2026-10-16T15:30:01.123456789-05:00' },
    },
    {
      name: 'all supplied timestamps',
      timestamps: {
        created_at: '2026-10-16T15:30:01.123456789-05:00',
        updated_at: '2026-10-16T20:31:00Z',
        closed_at: '2026-10-16T15:32:00.123-05:00',
      },
      expected: {
        created_at: '2026-10-16T15:30:01.123456789-05:00',
        updated_at: '2026-10-16T20:31:00Z',
        closed_at: '2026-10-16T15:32:00.123-05:00',
      },
    },
  ])('preserves $name byte-for-byte', async ({ timestamps, expected }) => {
    const item = makeItem(timestamps);
    const utils = render(WorkItemEditor, { item, onSaved: vi.fn(), onClose: vi.fn(), onDeleted: vi.fn() });
    await fireEvent.click(utils.getByRole('button', { name: 'Save', exact: true }));

    expect(app.SaveWorkItem).toHaveBeenCalledExactlyOnceWith({ ...item, ...expected });
    if (!('closed_at' in timestamps)) {
      expect(app.SaveWorkItem.mock.calls[0][0]).not.toHaveProperty('closed_at');
    }
  });

  it('does not hide malformed timestamps or lose the draft after bridge rejection', async () => {
    app.SaveWorkItem.mockRejectedValue(new Error('Invalid timestamp'));
    const onSaved = vi.fn();
    const onClose = vi.fn();
    const utils = render(WorkItemEditor, {
      item: makeItem({ updated_at: 'not-a-timestamp' }),
      onSaved, onClose, onDeleted: vi.fn(),
    });
    const titleInput = utils.container.querySelector('input')!;
    await fireEvent.input(titleInput, { target: { value: 'Retained draft' } });
    requestNavigation('portfolio');
    await saveAndContinueNavigation();

    expect(app.SaveWorkItem).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({ updated_at: 'not-a-timestamp' }));
    expect(utils.getByRole('alert').textContent).toContain('Invalid timestamp');
    expect(navigation.error).toBe('Invalid timestamp');
    expect(titleInput.value).toBe('Retained draft');
    expect(autosave.hasDirty()).toBe(true);
    expect(navigation.pending).not.toBeNull();
    expect(onSaved).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

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

  it('opens the shared unsaved-changes guard on Cancel when the title was edited, and keeps the modal open when declined', async () => {
    // Wails v2.16.0's darwin WKUIDelegate implements none of the JS
    // confirm/alert/prompt panel methods (verified: no `runJavaScript*Panel`
    // implementation anywhere in the vendored module). The observed result
    // in a packaged build is that `window.confirm()` produces no dialog and
    // the close silently no-ops (2026-08-16 GUI evidence) — the exact value
    // WebKit's `confirm()` returns was not independently confirmed, only
    // that no dialog appears. Mocking it to return `false` reproduces the
    // observed symptom; the close guard must not depend on this return
    // value at all.
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    const onClose = vi.fn();
    session.view = 'kanban';
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

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(navigation.pending?.view).toBe('kanban');
    expect(onClose).not.toHaveBeenCalled();

    cancelNavigation();
    expect(onClose).not.toHaveBeenCalled();
    // The draft survives the declined discard.
    expect((utils.container.querySelector('input') as HTMLInputElement).value).toBe('Edited title');
  });

  it('discards and closes on Cancel when the shared guard is told to discard', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    const onClose = vi.fn();
    session.view = 'kanban';
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
    expect(navigation.pending?.view).toBe('kanban');

    await discardAndContinueNavigation();

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(app.SaveWorkItem).not.toHaveBeenCalled();
  });

  it('prompts via the shared guard on backdrop click and on the header close button when dirty', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    const onClose = vi.fn();
    session.view = 'kanban';
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
    expect(navigation.pending?.view).toBe('kanban');
    expect(onClose).not.toHaveBeenCalled();

    cancelNavigation();

    await fireEvent.click(utils.getByRole('button', { name: 'Close' }));
    expect(navigation.pending?.view).toBe('kanban');
    await discardAndContinueNavigation();

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('prompts via the shared guard on Escape when dirty and does not close on decline', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    const onClose = vi.fn();
    session.view = 'kanban';
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

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(navigation.pending?.view).toBe('kanban');
    expect(onClose).not.toHaveBeenCalled();

    cancelNavigation();
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
    expect(navigation.error).toBe('disk full');
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

  it('retains an empty-ID draft after a failed native-close save and retries the same payload', async () => {
    const item = makeItem({ id: '', title: '', state: 'backlog', closed_at: '' });
    const zeroTime = '0001-01-01T00:00:00Z';
    const payload = {
      ...item,
      title: 'New item retry',
      description: 'Retain every edited field',
      type: 'bug',
      priority: 'urgent',
      points: 13,
      assignee: 'fixture-owner',
      created_at: zeroTime,
      updated_at: zeroTime,
      closed_at: zeroTime,
    };
    const saved = { ...payload, id: 'wi-created', created_at: '2026-10-18T12:00:00Z' };
    app.SaveWorkItem = vi.fn()
      .mockRejectedValueOnce(new Error('database is locked'))
      .mockResolvedValueOnce(saved);
    const onClose = vi.fn();
    const onSaved = vi.fn();
    const utils = render(WorkItemEditor, {
      item, onClose, onSaved, onDeleted: vi.fn(),
    });
    await fireEvent.input(utils.getByLabelText('Title'), { target: { value: payload.title } });
    await fireEvent.input(utils.getByLabelText('Description'), { target: { value: payload.description } });
    await fireEvent.change(utils.getByLabelText('Type'), { target: { value: payload.type } });
    await fireEvent.change(utils.getByLabelText('Priority'), { target: { value: payload.priority } });
    await fireEvent.input(utils.getByLabelText('Points'), { target: { value: String(payload.points) } });
    await fireEvent.input(utils.getByLabelText('Assignee'), { target: { value: payload.assignee } });
    const nativeApp = {
      EnableNativeCloseGuard: vi.fn().mockResolvedValue(undefined),
      CompleteNativeClose: vi.fn().mockResolvedValue(undefined),
    };
    const controller = new NativeCloseController({
      app: nativeApp, reportError: vi.fn(), setInteractionLocked: vi.fn(),
    });
    session.view = 'backlog';
    controller.request();
    await saveAndContinueNavigation();

    expect(app.SaveWorkItem).toHaveBeenCalledTimes(1);
    expect(app.SaveWorkItem).toHaveBeenNthCalledWith(1, payload);
    expect(navigation.pending?.view).toBe('backlog');
    expect(navigation.error).toBe('database is locked');
    expect(utils.getByRole('alert')).toHaveTextContent('database is locked');
    expect(utils.getByLabelText('Title')).toHaveValue(payload.title);
    expect(utils.getByLabelText('Description')).toHaveValue(payload.description);
    expect(utils.getByLabelText('Type')).toHaveValue(payload.type);
    expect(utils.getByLabelText('Priority')).toHaveValue(payload.priority);
    expect(utils.getByLabelText('Points')).toHaveValue(payload.points);
    expect(utils.getByLabelText('Assignee')).toHaveValue(payload.assignee);
    expect(autosave.hasDirty()).toBe(true);
    expect(onSaved).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
    expect(nativeApp.CompleteNativeClose).not.toHaveBeenCalled();
    expect(item).toEqual(makeItem({ id: '', title: '', state: 'backlog', closed_at: '' }));

    await saveAndContinueNavigation();

    expect(app.SaveWorkItem).toHaveBeenCalledTimes(2);
    expect(app.SaveWorkItem).toHaveBeenNthCalledWith(2, payload);
    expect(onSaved).toHaveBeenCalledExactlyOnceWith(saved);
    expect(onClose).toHaveBeenCalledOnce();
    expect(autosave.hasDirty()).toBe(false);
    expect(navigation.pending).toBeNull();
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
    expect(navigation.error).toBe('Title is required.');
  });

  it.each(['', 'wi-existing'])('replaces a backend error with current validation and recovers draft %s', async (id) => {
    const item = makeItem({ id });
    const onSaved = vi.fn();
    const onClose = vi.fn();
    app.SaveWorkItem.mockRejectedValueOnce('storage unavailable');
    const utils = render(WorkItemEditor, { item, onSaved, onClose, onDeleted: vi.fn() });
    const title = utils.getByLabelText('Title');
    await fireEvent.input(title, { target: { value: 'Retained title' } });
    session.view = 'backlog';
    requestNavigation('portfolio');
    await saveAndContinueNavigation();

    expect(navigation.error).toBe('storage unavailable');
    expect(utils.getByRole('alert')).toHaveTextContent('storage unavailable');
    expect(navigation.pending?.view).toBe('portfolio');
    expect(session.view).toBe('backlog');
    expect(onSaved).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
    cancelNavigation();
    expect(title).toHaveValue('Retained title');

    await fireEvent.input(title, { target: { value: '   ' } });
    requestNavigation('portfolio');
    expect(navigation.error).toBe('');
    await saveAndContinueNavigation();
    expect(navigation.error).toBe('Title is required.');
    expect(app.SaveWorkItem).toHaveBeenCalledTimes(1);
    expect(navigation.pending?.view).toBe('portfolio');
    expect(autosave.hasDirty()).toBe(true);
    expect(onSaved).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
    cancelNavigation();

    await fireEvent.input(title, { target: { value: 'Recovered title' } });
    const payload = { ...item, title: 'Recovered title', created_at: '0001-01-01T00:00:00Z', updated_at: '0001-01-01T00:00:00Z' };
    const saved = { ...payload, id: id || 'wi-created', created_at: '2026-10-20T12:00:00Z' };
    app.SaveWorkItem.mockResolvedValueOnce(saved);
    requestNavigation('portfolio');
    await saveAndContinueNavigation();
    expect(app.SaveWorkItem).toHaveBeenCalledTimes(2);
    expect(app.SaveWorkItem).toHaveBeenLastCalledWith(payload);
    expect(onSaved).toHaveBeenCalledExactlyOnceWith(saved);
    expect(onClose).toHaveBeenCalledOnce();
    expect(session.view).toBe('portfolio');
    expect(navigation.pending).toBeNull();
    expect(navigation.error).toBe('');
    expect(autosave.hasDirty()).toBe(false);
    expect(item).toEqual(makeItem({ id }));
  });

  it('keeps the generic guard fallback when the backend supplies no error message', async () => {
    app.SaveWorkItem.mockRejectedValueOnce(new Error(''));
    const onSaved = vi.fn();
    const onClose = vi.fn();
    const utils = render(WorkItemEditor, { item: makeItem(), onSaved, onClose, onDeleted: vi.fn() });
    await fireEvent.input(utils.getByLabelText('Title'), { target: { value: 'Retained without details' } });
    requestNavigation('portfolio');
    await saveAndContinueNavigation();
    expect(navigation.error).toBe('The editor reported that the save failed.');
    expect(navigation.pending?.view).toBe('portfolio');
    expect(utils.getByLabelText('Title')).toHaveValue('Retained without details');
    expect(autosave.hasDirty()).toBe(true);
    expect(app.SaveWorkItem).toHaveBeenCalledOnce();
    expect(onSaved).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it('keeps the generic guard fallback during a pending manual save without another write', async () => {
    let rejectSave!: (reason: Error) => void;
    app.SaveWorkItem.mockImplementationOnce(() => new Promise<AgileWorkItem>((_, reject) => { rejectSave = reject; }));
    const onClose = vi.fn();
    const onSaved = vi.fn();
    const utils = render(WorkItemEditor, { item: makeItem(), onClose, onSaved, onDeleted: vi.fn() });
    await fireEvent.input(utils.getByLabelText('Title'), { target: { value: 'Pending title' } });
    await fireEvent.click(utils.getByRole('button', { name: 'Save', exact: true }));
    await waitFor(() => expect(app.SaveWorkItem).toHaveBeenCalledOnce());
    try {
      requestNavigation('portfolio');
      await saveAndContinueNavigation();
      expect(navigation.error).toBe('The editor reported that the save failed.');
      expect(navigation.pending?.view).toBe('portfolio');
      expect(app.SaveWorkItem).toHaveBeenCalledOnce();
      expect(onSaved).not.toHaveBeenCalled();
      expect(onClose).not.toHaveBeenCalled();
      expect(autosave.hasDirty()).toBe(true);
    } finally {
      rejectSave(new Error('manual failure'));
      await waitFor(() => expect(utils.getByRole('alert')).toHaveTextContent('manual failure'));
    }
    expect(app.SaveWorkItem).toHaveBeenCalledOnce();
  });

  it('does not forward an earlier save error when native-close Save is refused during deletion', async () => {
    app.SaveWorkItem.mockRejectedValueOnce(new Error('earlier save failure'));
    let rejectDelete!: (reason: Error) => void;
    app.DeleteWorkItem.mockImplementationOnce(() => new Promise<void>((_, reject) => { rejectDelete = reject; }));
    const onSaved = vi.fn();
    const onClose = vi.fn();
    const onDeleted = vi.fn();
    const utils = render(WorkItemEditor, { item: makeItem(), onSaved, onClose, onDeleted });
    await fireEvent.input(utils.getByLabelText('Title'), { target: { value: 'Retained during deletion' } });
    requestNavigation('portfolio');
    await saveAndContinueNavigation();
    expect(navigation.error).toBe('earlier save failure');
    cancelNavigation();
    await fireEvent.click(utils.getByRole('button', { name: 'Delete', exact: true }));
    await fireEvent.click(within(utils.getByRole('alertdialog')).getByRole('button', { name: 'Delete', exact: true }));
    await waitFor(() => expect(app.DeleteWorkItem).toHaveBeenCalledExactlyOnceWith('wi-1'));
    const nativeApp = {
      EnableNativeCloseGuard: vi.fn().mockResolvedValue(undefined),
      CompleteNativeClose: vi.fn().mockResolvedValue(undefined),
    };
    const controller = new NativeCloseController({ app: nativeApp, reportError: vi.fn(), setInteractionLocked: vi.fn() });
    session.view = 'kanban';
    try {
      controller.request();
      await saveAndContinueNavigation();
      expect(navigation.error).toBe('The editor reported that the save failed.');
      expect(navigation.pending?.view).toBe('kanban');
      expect(app.SaveWorkItem).toHaveBeenCalledOnce();
      expect(app.DeleteWorkItem).toHaveBeenCalledOnce();
      expect(nativeApp.CompleteNativeClose).not.toHaveBeenCalled();
      expect(onSaved).not.toHaveBeenCalled();
      expect(onClose).not.toHaveBeenCalled();
      expect(onDeleted).not.toHaveBeenCalled();
      expect(utils.getByLabelText('Title')).toHaveValue('Retained during deletion');
      expect(autosave.hasDirty()).toBe(true);
    } finally {
      rejectDelete(new Error('deletion failed'));
      await waitFor(() => expect(utils.getByRole('alert')).toHaveTextContent('deletion failed'));
    }
    expect(nativeApp.CompleteNativeClose).not.toHaveBeenCalled();
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

  it('regression: never calls window.confirm to guard a dirty close, and dismissal does not depend on its return value', async () => {
    // Wails v2.16.0's darwin WKUIDelegate implements none of the JS
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
    const onClose = vi.fn();
    session.view = 'kanban';
    const utils = render(WorkItemEditor, {
      item: makeItem(),
      onClose,
      onSaved: vi.fn(),
      onDeleted: vi.fn(),
    });
    await fireEvent.input(utils.container.querySelector('input') as HTMLInputElement, {
      target: { value: 'Regression check' },
    });

    const cancelBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Cancel',
    )!;
    await fireEvent.click(cancelBtn);

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(navigation.pending?.view).toBe('kanban');

    await discardAndContinueNavigation();

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});

describe('WorkItemEditor delete', () => {
  // Delete used `confirm()` — silently a no-op in the packaged macOS build
  // (Wails v2.16.0's darwin WKUIDelegate implements none of the JS
  // confirm/alert/prompt panel methods). Now routed through the shared
  // ConfirmDialog instead; these tests cover that path directly, since
  // nothing above exercised the Delete button at all.
  it('opens the shared confirm dialog on Delete and does not call window.confirm', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm');
    const utils = render(WorkItemEditor, {
      item: makeItem({ title: 'Doomed item' }),
      onClose: vi.fn(),
      onSaved: vi.fn(),
      onDeleted: vi.fn(),
    });
    const deleteBtn = utils.getByText('Delete');
    await fireEvent.click(deleteBtn);

    const dialog = document.querySelector('[role="alertdialog"]');
    expect(dialog).not.toBeNull();
    expect(dialog?.textContent).toContain('Doomed item');
    expect(confirmSpy).not.toHaveBeenCalled();
    expect(app.DeleteWorkItem).not.toHaveBeenCalled();
  });

  it('deletes the work item, notifies the parent, and closes when the dialog is confirmed', async () => {
    const onDeleted = vi.fn();
    const onClose = vi.fn();
    const utils = render(WorkItemEditor, {
      item: makeItem({ id: 'wi-42' }),
      onClose,
      onSaved: vi.fn(),
      onDeleted,
    });
    await fireEvent.click(utils.getByText('Delete'));
    const confirmBtn = Array.from(document.querySelectorAll('[role="alertdialog"] button')).find(
      (b) => b.textContent?.trim() === 'Delete',
    )!;
    await fireEvent.click(confirmBtn);

    await waitFor(() => expect(app.DeleteWorkItem).toHaveBeenCalledWith('wi-42'));
    expect(onDeleted).toHaveBeenCalledWith('wi-42');
    expect(onClose).toHaveBeenCalledOnce();
    expect(document.querySelector('[role="alertdialog"]')).toBeNull();
  });

  it('keeps the work item and the editor open when the dialog is cancelled', async () => {
    const utils = render(WorkItemEditor, {
      item: makeItem(),
      onClose: vi.fn(),
      onSaved: vi.fn(),
      onDeleted: vi.fn(),
    });
    await fireEvent.click(utils.getByText('Delete'));
    const dialogCancelBtn = Array.from(document.querySelectorAll('[role="alertdialog"] button')).find(
      (b) => b.textContent?.trim() === 'Cancel',
    )!;
    await fireEvent.click(dialogCancelBtn);

    expect(document.querySelector('[role="alertdialog"]')).toBeNull();
    expect(app.DeleteWorkItem).not.toHaveBeenCalled();
    // Editor itself is still open.
    expect(document.querySelector('input')).not.toBeNull();
  });
});
