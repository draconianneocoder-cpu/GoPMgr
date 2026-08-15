// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, describe, expect, it, vi } from 'vitest';
import { autosave } from './autosave.svelte';
import {
  discardAndContinueNavigation,
  navigation,
  requestNavigation,
  session,
  saveAndContinueNavigation,
} from './session.svelte';
import { NativeCloseController, installNativeCloseGuard, type NativeCloseApp } from './native-close';

afterEach(() => {
  autosave.discardAll();
  navigation.pending = null;
  navigation.saving = false;
  navigation.error = '';
  session.view = 'portfolio';
  session.editingId = null;
});

function controllerWith(overrides: Partial<NativeCloseApp> = {}) {
  const app: NativeCloseApp = {
    EnableNativeCloseGuard: vi.fn().mockResolvedValue(undefined),
    CompleteNativeClose: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  };
  const reportError = vi.fn();
  const setInteractionLocked = vi.fn();
  return {
    app,
    reportError,
    setInteractionLocked,
    controller: new NativeCloseController({ app, reportError, setInteractionLocked }),
  };
}

describe('native close guard', () => {
  it('installs the listener before enabling the Go guard and unsubscribes cleanly', async () => {
    const calls: string[] = [];
    const stop = vi.fn();
    const runtime = {
      EventsOn: vi.fn(() => {
        calls.push('listen');
        return stop;
      }),
    };
    const app = {
      EnableNativeCloseGuard: vi.fn().mockImplementation(async () => {
        calls.push('enable');
      }),
    };
    const { controller, reportError } = controllerWith();

    installNativeCloseGuard(runtime, app, controller, reportError)();
    await vi.waitFor(() => expect(calls).toEqual(['listen', 'enable']));
    expect(stop).toHaveBeenCalledOnce();
    expect(reportError).not.toHaveBeenCalled();
  });

  it('permits a clean native close exactly once', async () => {
    const { app, controller, reportError, setInteractionLocked } = controllerWith();

    controller.request();
    await vi.waitFor(() => expect(app.CompleteNativeClose).toHaveBeenCalledOnce());
    expect(reportError).not.toHaveBeenCalled();
    expect(setInteractionLocked).toHaveBeenCalledWith(true);

    controller.request();
    expect(app.CompleteNativeClose).toHaveBeenCalledOnce();
  });

  it('keeps a dirty editor open until an explicit successful save', async () => {
    let value = 'saved';
    const save = vi.fn().mockResolvedValue(true);
    const unregister = autosave.register(() => value, save, false);
    value = 'edited';
    session.view = 'gantt';
    const { app, controller } = controllerWith();

    controller.request();
    controller.request();
    expect(navigation.pending?.view).toBe('gantt');
    expect(app.CompleteNativeClose).not.toHaveBeenCalled();

    await saveAndContinueNavigation();
    await vi.waitFor(() => expect(app.CompleteNativeClose).toHaveBeenCalledOnce());
    expect(save).toHaveBeenCalledOnce();
    expect(app.CompleteNativeClose).toHaveBeenCalledOnce();
    unregister();
  });

  it('does not replace a pending native close with another navigation request', async () => {
    let value = 'saved';
    const unregister = autosave.register(() => value, () => true, false);
    value = 'edited';
    session.view = 'gantt';
    const { app, controller } = controllerWith();

    controller.request();
    requestNavigation('help');
    expect(navigation.pending?.view).toBe('gantt');

    await discardAndContinueNavigation();
    await vi.waitFor(() => expect(app.CompleteNativeClose).toHaveBeenCalledOnce());
    unregister();
  });

  it('retains a failed save and does not permit or quit', async () => {
    let value = 'saved';
    const unregister = autosave.register(() => value, vi.fn().mockResolvedValue(false), false);
    value = 'edited';
    const { app, controller } = controllerWith();

    controller.request();
    await saveAndContinueNavigation();

    expect(navigation.pending).not.toBeNull();
    expect(navigation.error).toContain('reported that the save failed');
    expect(app.CompleteNativeClose).not.toHaveBeenCalled();
    unregister();
  });

  it('does not permit or quit when edits arrive during a save', async () => {
    let value = 'saved';
    let finishSave!: (result: boolean) => void;
    const save = vi.fn(() => new Promise<boolean>((resolve) => { finishSave = resolve; }));
    const unregister = autosave.register(() => value, save, false);
    value = 'first edit';
    const { app, controller } = controllerWith();

    controller.request();
    const continuing = saveAndContinueNavigation();
    value = 'second edit';
    finishSave(true);
    await continuing;

    expect(navigation.pending).not.toBeNull();
    expect(navigation.error).toContain('Changes were made while saving');
    expect(app.CompleteNativeClose).not.toHaveBeenCalled();
    unregister();
  });

  it('permits a dirty native close after explicit discard', async () => {
    let value = 'saved';
    const unregister = autosave.register(() => value, () => true, false);
    value = 'edited';
    const { app, controller } = controllerWith();

    controller.request();
    await discardAndContinueNavigation();

    await vi.waitFor(() => expect(app.CompleteNativeClose).toHaveBeenCalledOnce());
    unregister();
  });

  it('does not quit when the one-shot permit fails and allows a retry', async () => {
    const app = {
      CompleteNativeClose: vi.fn().mockRejectedValueOnce(new Error('bridge unavailable')).mockResolvedValueOnce(undefined),
    };
    const { controller, reportError, setInteractionLocked } = controllerWith(app);

    controller.request();
    await vi.waitFor(() => expect(reportError).toHaveBeenCalledWith('Could not close application: bridge unavailable'));
    expect(setInteractionLocked).toHaveBeenLastCalledWith(false);

    controller.request();
    await vi.waitFor(() => expect(app.CompleteNativeClose).toHaveBeenCalledTimes(2));
  });

  it('locks interaction before the backend completes native close and keeps it locked', async () => {
    const { app, controller, setInteractionLocked } = controllerWith();

    controller.request();
    await vi.waitFor(() => expect(app.CompleteNativeClose).toHaveBeenCalledOnce());

    expect(setInteractionLocked).toHaveBeenCalledWith(true);
    expect(setInteractionLocked.mock.invocationCallOrder[0]).toBeLessThan(app.CompleteNativeClose.mock.invocationCallOrder[0]);
    expect(setInteractionLocked).not.toHaveBeenCalledWith(false);
  });

  it('lets a canceled native close prompt again', () => {
    let value = 'saved';
    const unregister = autosave.register(() => value, () => true, false);
    value = 'edited';
    const { controller } = controllerWith();

    controller.request();
    expect(navigation.pending).not.toBeNull();
    controller.cancel();
    expect(navigation.pending).toBeNull();
    controller.request();
    expect(navigation.pending).not.toBeNull();
    unregister();
  });
});
