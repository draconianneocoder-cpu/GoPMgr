// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, describe, expect, it, vi } from 'vitest';
import { autosave } from './autosave.svelte';
import {
  cancelNavigation,
  discardAndContinueNavigation,
  navigation,
  requestNavigation,
  saveAndContinueNavigation,
  session,
} from './session.svelte';

afterEach(() => {
  cancelNavigation();
  autosave.discardAll();
  session.view = 'portfolio';
});

describe('guarded navigation', () => {
  it('stays on the editor when save fails and continues after retry', async () => {
    let value = 'saved';
    const save = vi.fn().mockResolvedValueOnce(false).mockResolvedValueOnce(true);
    const unregister = autosave.register(() => value, save, false);
    value = 'edited';

    session.view = 'cpm';
    requestNavigation('portfolio');
    expect(navigation.pending?.view).toBe('portfolio');

    await saveAndContinueNavigation();
    expect(session.view).toBe('cpm');
    expect(navigation.pending?.view).toBe('portfolio');

    await saveAndContinueNavigation();
    expect(session.view).toBe('portfolio');
    expect(navigation.pending).toBeNull();
    unregister();
  });

  it('requires explicit discard before leaving', async () => {
    let value = 'saved';
    const unregister = autosave.register(() => value, () => true, false);
    value = 'edited';
    session.view = 'gantt';
    requestNavigation('dashboard');
    expect(session.view).toBe('gantt');
    await discardAndContinueNavigation();
    expect(session.view).toBe('dashboard');
    unregister();
  });
});
