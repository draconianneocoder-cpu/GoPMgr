// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { Call } from '@wailsio/runtime';

import { App } from '../../bindings/gopmgr/internal/desktop/index';

// BackendError is an error a Go method returned. Its text is the method's
// own message: Wails delivers these as RuntimeError, whose default text
// ("RuntimeError: no project open") put the class name into every message
// the interface builds with `${e}` or String(e). The original error stays
// in `cause`.
export class BackendError extends Error {
  constructor(message: string, options?: ErrorOptions) {
    super(message, options);
    this.name = 'BackendError';
  }

  override toString(): string {
    return this.message;
  }
}

// asBackendError converts a Wails RuntimeError and leaves every other
// rejection (unknown method, wrong arguments, transport failure) as it is.
export function asBackendError(err: unknown): unknown {
  return err instanceof Call.RuntimeError ? new BackendError(err.message, { cause: err }) : err;
}

type DesktopApi = typeof App;

// desktopApi is the generated App namespace with each method's rejection
// passed through asBackendError. Results, arguments and the returned
// promise (a Wails CancellablePromise, whose catch keeps it cancellable)
// are otherwise unchanged.
export const desktopApi: DesktopApi = new Proxy(App, {
  get(target, key, receiver) {
    const member: unknown = Reflect.get(target, key, receiver);
    if (typeof member !== 'function') return member;
    return (...args: unknown[]) =>
      (member as (...a: unknown[]) => Promise<unknown>)(...args).catch((err: unknown) => {
        throw asBackendError(err);
      });
  },
});
