// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { autosave } from './autosave.svelte';

// A form draft protected by the shared unsaved-changes guard: leaving the
// view, closing the project or closing the window asks to save or discard
// it first. There is no timed auto-save, so a half-filled form is never
// written on its own.
//
// arm() takes the current snapshot as the saved state. Call it when the
// form shows stored or blank values: after loading, and after each
// successful save (the backend may store normalised values, so the state
// after a save, not before it, is what counts as clean). disarm() removes
// the draft, for example when its dialog closes or the form unmounts.
export interface DraftGuard {
  arm(): void;
  disarm(): void;
}

export function createDraftGuard(
  snapshot: () => string,
  // Saves the draft and reports whether it was saved.
  save: () => Promise<boolean>,
  // Why the draft was not saved, shown by the leave prompt.
  reason: () => string,
): DraftGuard {
  let stop: (() => void) | null = null;
  const guard: DraftGuard = {
    arm() {
      stop?.();
      stop = autosave.register(snapshot, saveFromPrompt, false);
    },
    disarm() {
      stop?.();
      stop = null;
    },
  };
  async function saveFromPrompt(): Promise<boolean> {
    if (!(await save())) throw new Error(reason() || 'The form was not saved.');
    // A save that cleared or closed the form leaves nothing to protect.
    if (stop) guard.arm();
    return true;
  }
  return guard;
}
