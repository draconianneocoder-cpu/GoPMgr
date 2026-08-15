// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it, vi } from 'vitest';
import { autosave } from './autosave.svelte';

describe('autosave persistence contract', () => {
  it('keeps a failed save dirty and allows a successful retry', async () => {
    let value = 'saved';
    const save = vi.fn().mockResolvedValueOnce(false).mockResolvedValueOnce(true);
    const unregister = autosave.register(() => value, save, false);

    value = 'edited';
    expect(autosave.hasDirty()).toBe(true);
    expect(await autosave.saveAll()).toBe(false);
    expect(autosave.hasDirty()).toBe(true);
    expect(autosave.lastError).toContain('reported that the save failed');

    expect(await autosave.saveAll()).toBe(true);
    expect(autosave.hasDirty()).toBe(false);
    unregister();
  });

  it('requires an explicit discard before advancing the baseline', () => {
    let value = 'saved';
    const unregister = autosave.register(() => value, () => true, false);
    value = 'edited';
    autosave.discardAll();
    expect(autosave.hasDirty()).toBe(false);
    unregister();
  });

  it('keeps an edit made during a save dirty for a second save', async () => {
    let value = 'saved';
    let finishSave!: (result: boolean) => void;
    const save = vi.fn(() => new Promise<boolean>((resolve) => { finishSave = resolve; }));
    const unregister = autosave.register(() => value, save, false);
    value = 'first edit';

    const saving = autosave.saveAll();
    value = 'second edit';
    finishSave(true);

    expect(await saving).toBe(false);
    expect(autosave.hasDirty()).toBe(true);
    expect(autosave.lastError).toContain('Changes were made while saving');
    unregister();
  });
});
