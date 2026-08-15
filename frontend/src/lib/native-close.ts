// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { cancelNavigation, navigation, requestNavigation, session } from './session.svelte';

export interface NativeCloseRuntime {
  EventsOn(event: string, callback: () => void): () => void;
}

export interface NativeCloseApp {
  EnableNativeCloseGuard(): Promise<void>;
  CompleteNativeClose(): Promise<void>;
}

interface NativeCloseOptions {
  app: NativeCloseApp;
  reportError: (message: string) => void;
  setInteractionLocked: (locked: boolean) => void;
}

// NativeCloseController bridges Wails's synchronous close veto to the
// asynchronous save/discard workflow. The Go guard blocks every native close
// until this controller grants its one-shot permit immediately before Quit.
export class NativeCloseController {
  private resolving = false;
  private awaitingChoice = false;

  constructor(private readonly options: NativeCloseOptions) {}

  request(): void {
    if (this.resolving || navigation.pending) return;
    this.resolving = true;
    requestNavigation(session.view, session.editingId, async () => {
      this.awaitingChoice = false;
      this.options.setInteractionLocked(true);
      try {
        await this.options.app.CompleteNativeClose();
      } catch (err: unknown) {
        try {
          await this.options.app.EnableNativeCloseGuard();
          this.options.setInteractionLocked(false);
          this.resolving = false;
          const detail = err instanceof Error ? err.message : String(err);
          this.options.reportError(`Could not close application: ${detail}`);
        } catch (resetErr: unknown) {
          const detail = resetErr instanceof Error ? resetErr.message : String(resetErr);
          this.options.reportError(`Could not safely cancel application close: ${detail}`);
        }
      }
      return false;
    });
    this.awaitingChoice = navigation.pending !== null;
  }

  cancel(): void {
    cancelNavigation();
    if (this.awaitingChoice) {
      this.awaitingChoice = false;
      this.resolving = false;
    }
  }
}

// installNativeCloseGuard registers the event listener before telling Go to
// veto close requests. This ordering leaves only the documented startup window
// unguarded, rather than risking a veto with no frontend listener.
export function installNativeCloseGuard(
  runtime: Pick<NativeCloseRuntime, 'EventsOn'>,
  app: Pick<NativeCloseApp, 'EnableNativeCloseGuard'>,
  controller: NativeCloseController,
  reportError: (message: string) => void,
): () => void {
  const stop = runtime.EventsOn('app:before-close', () => controller.request());
  void app.EnableNativeCloseGuard().catch((err: unknown) => {
    const detail = err instanceof Error ? err.message : String(err);
    reportError(`Could not enable the native close guard: ${detail}`);
  });
  return stop;
}
