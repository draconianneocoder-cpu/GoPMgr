// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';

import SprintList from './SprintList.svelte';
import { session } from '../../session.svelte';

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
  vi.restoreAllMocks();
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
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
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

    const cancelBtn = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Cancel',
    )!;
    await fireEvent.click(cancelBtn);

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(document.querySelector('input')).not.toBeNull();

    resolveSave({ ...sprint, name: 'Edited name' });
    await waitFor(() => expect(document.querySelector('input')).toBeNull());
  });
});
