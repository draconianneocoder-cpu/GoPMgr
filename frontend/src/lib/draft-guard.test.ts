// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, describe, expect, it } from 'vitest';

import { autosave } from './autosave.svelte';
import { createDraftGuard, type DraftGuard } from './draft-guard';

// The shared contract of form drafts under the unsaved-changes guard; the
// forms' own tests check that each one wires its draft to it.
describe('createDraftGuard', () => {
  let guard: DraftGuard | null = null;
  afterEach(() => {
    guard?.disarm();
    autosave.discardAll();
  });

  it('is clean when armed, dirty after an edit, and clean after a save that normalises the value', async () => {
    let name = 'Supplier';
    guard = createDraftGuard(
      () => name,
      async () => {
        name = name.trim(); // what the backend stores
        return true;
      },
      () => '',
    );
    guard.arm();
    expect(autosave.hasDirty()).toBe(false);

    name = '  Supplier B  ';
    expect(autosave.hasDirty()).toBe(true);
    expect(await autosave.saveAll()).toBe(true);
    expect(name).toBe('Supplier B');
    expect(autosave.hasDirty()).toBe(false);
  });

  it('keeps the draft and gives the reason when the save fails', async () => {
    let amount = '';
    guard = createDraftGuard(() => amount, async () => false, () => 'Amount is required.');
    guard.arm();
    amount = 'abc';

    expect(await autosave.saveAll()).toBe(false);
    expect(autosave.lastError).toBe('Amount is required.');
    expect(autosave.hasDirty()).toBe(true);
  });

  it('protects nothing once disarmed', () => {
    let note = '';
    guard = createDraftGuard(() => note, async () => true, () => '');
    guard.arm();
    note = 'typed';
    guard.disarm();
    expect(autosave.hasDirty()).toBe(false);
  });
});
