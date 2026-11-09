// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { desktopRuntime } from './desktop-runtime';
import { installNativeCloseGuard, NativeCloseController } from './native-close';

const transport = vi.hoisted(() => {
  const state: {
    listeners: Map<string, (event: { data: unknown; name: string; sender?: string }) => void>;
    registrationError: Error | null;
  } = { listeners: new Map(), registrationError: null };
  return state;
});
vi.mock('@wailsio/runtime', () => ({
  Events: {
    On(name: string, callback: (event: { data: unknown; name: string; sender?: string }) => void) {
      if (transport.registrationError) throw transport.registrationError;
      transport.listeners.set(name, callback);
      return () => { transport.listeners.delete(name); };
    },
  },
}));

beforeEach(() => {
  transport.listeners.clear();
  transport.registrationError = null;
});

describe('desktop runtime events', () => {
  it('delivers data rather than the runtime envelope, including falsy payloads', () => {
    const received: unknown[] = [];
    desktopRuntime.EventsOn('menu:help', (data) => received.push(data));
    for (const data of [{ reason: 'menu' }, null, false, 0, '']) {
      transport.listeners.get('menu:help')?.({ name: 'menu:help', data, sender: 'main' });
    }
    expect(received).toEqual([{ reason: 'menu' }, null, false, 0, '']);
  });

  it('stops delivery when the subscription is disposed', () => {
    let deliveries = 0;
    const stop = desktopRuntime.EventsOn('menu:help', () => { deliveries++; });
    transport.listeners.get('menu:help')?.({ name: 'menu:help', data: null });
    stop();
    transport.listeners.get('menu:help')?.({ name: 'menu:help', data: null });
    expect(deliveries).toBe(1);
  });

  it('does not swallow listener errors', () => {
    desktopRuntime.EventsOn('menu:help', () => { throw new Error('navigation failed'); });
    expect(() => transport.listeners.get('menu:help')?.({ name: 'menu:help', data: null }))
      .toThrow('navigation failed');
  });

  it('does not enable the native guard when listener registration fails', () => {
    transport.registrationError = new Error('events unavailable');
    let enabled = false;
    const app = {
      async EnableNativeCloseGuard() { enabled = true; },
      async CompleteNativeClose() {},
    };
    const controller = new NativeCloseController({ app, reportError: () => {}, setInteractionLocked: () => {} });
    expect(() => installNativeCloseGuard(desktopRuntime, app, controller, () => {})).toThrow('events unavailable');
    expect(enabled).toBe(false);
  });

  it('registers before enabling the guard and reports readiness errors', async () => {
    let listeningAtEnable = false;
    let error = '';
    const app = {
      async EnableNativeCloseGuard() {
        listeningAtEnable = transport.listeners.has('app:before-close');
        throw new Error('guard unavailable');
      },
      async CompleteNativeClose() {},
    };
    const controller = new NativeCloseController({ app, reportError: () => {}, setInteractionLocked: () => {} });
    const stop = installNativeCloseGuard(desktopRuntime, app, controller, (message) => { error = message; });
    await vi.waitFor(() => expect(error).toBe('Could not enable the native close guard: guard unavailable'));
    expect(listeningAtEnable).toBe(true);
    stop();
    expect(transport.listeners.has('app:before-close')).toBe(false);
  });
});
