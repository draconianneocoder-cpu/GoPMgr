// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it, vi } from 'vitest';
import { autosave } from './autosave.svelte';

describe('autosave persistence contract', () => {
  for (const outcome of ['resolve', 'reject'] as const) {
    it(`preserves a current error when an unregistered save ${outcome}s`, async () => {
      let finish!: () => void;
      let fail!: (error: Error) => void;
      const pending = new Promise<void>((resolve, reject) => { finish = resolve; fail = reject; });
      const oldSnapshot = () => 'old';
      const old = autosave.register(oldSnapshot, () => pending, false);
      const saving = autosave.saveRegistered(oldSnapshot);
      old();
      const currentSnapshot = () => 'current';
      const current = autosave.register(currentSnapshot, () => { throw new Error('current failure'); }, false);
      try {
        expect(await autosave.saveRegistered(currentSnapshot)).toBe(false);
        expect(autosave.lastError).toBe('current failure');
        if (outcome === 'resolve') finish();
        else fail(new Error('stale failure'));
        // Modal editors may unregister themselves after a committed write.
        expect(await saving).toBe(outcome === 'resolve');
        expect(autosave.lastError).toBe('current failure');
        expect(autosave.hasDirty()).toBe(false);
      } finally {
        current();
        old();
      }
    });
  }
  it('serializes heartbeat writes and keeps a round-trip late edit dirty', async () => {
    vi.resetModules();
    vi.useFakeTimers();
    const { autosave: timed } = await import('./autosave.svelte');
    let value = 'original';
    let persisted = 'original';
    let finish!: () => void;
    const save = vi.fn(async () => { persisted = value; });
    save.mockImplementationOnce(() => {
      const submitted = value;
      return new Promise<void>((resolve) => {
        finish = () => { persisted = submitted; resolve(); };
      });
    });
    const unregister = timed.register(() => value, save);
    try {
      timed.setInterval(1);
      value = 'submitted';
      await vi.advanceTimersByTimeAsync(1000);
      value = 'original';
      expect(timed.hasDirty()).toBe(true);
      await vi.advanceTimersByTimeAsync(2000);
      expect(save).toHaveBeenCalledTimes(1);
      finish();
      await Promise.resolve();
      expect(persisted).toBe('submitted');
      expect(timed.hasDirty()).toBe(true);
      await vi.advanceTimersByTimeAsync(1000);
      expect(save).toHaveBeenCalledTimes(2);
      expect(persisted).toBe('original');
      expect(timed.hasDirty()).toBe(false);
    } finally {
      unregister();
      timed.setInterval(0);
      vi.clearAllTimers();
      vi.useRealTimers();
      vi.resetModules();
    }
  });
  it('routes a registered save only to that editor and blocks navigation while in flight', async () => {
    let value = 'original';
    const snapshot = () => value;
    let finish!: () => void;
    const save = vi.fn(() => new Promise<void>((resolve) => { finish = resolve; }));
    const otherSave = vi.fn();
    const unregister = autosave.register(snapshot, save, false);
    const unregisterOther = autosave.register(() => 'other', otherSave, false);
    try {
      value = 'submitted';
      const pending = autosave.saveRegistered(snapshot);
      value = 'original';
      expect(autosave.hasDirty()).toBe(true);
      expect(await autosave.saveRegistered(snapshot)).toBe(false);
      expect(save).toHaveBeenCalledTimes(1);
      finish();
      expect(await pending).toBe(true);
      expect(autosave.hasDirty()).toBe(true);
      expect(otherSave).not.toHaveBeenCalled();
    } finally {
      unregister();
      unregisterOther();
    }
    expect(await autosave.saveRegistered(snapshot)).toBe(false);
  });

  it('refuses ambiguous snapshot registrations without writing either editor', async () => {
    const snapshot = () => 'same';
    const save = vi.fn();
    const first = autosave.register(snapshot, save, false);
    const second = autosave.register(snapshot, save, false);
    try {
      expect(await autosave.saveRegistered(snapshot)).toBe(false);
      expect(save).not.toHaveBeenCalled();
    } finally {
      first();
      second();
    }
  });
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
