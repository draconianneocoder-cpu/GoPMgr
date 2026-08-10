// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';

import WorkItemEditor from './WorkItemEditor.svelte';

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
  vi.restoreAllMocks();
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
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
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

    expect(confirmSpy).toHaveBeenCalledTimes(1);
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
});
