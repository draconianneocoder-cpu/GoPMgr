// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, describe, expect, it, vi } from 'vitest';

import { recoveryGateNeeded } from './recovery-gate';

const desktopApiMock = vi.hoisted(() => {
  const state: { current: Record<string, ReturnType<typeof vi.fn>> } = { current: {} };
  return state;
});
vi.mock('./desktop-api', () => ({ get desktopApi() { return desktopApiMock.current; } }));

function install(status: () => Promise<unknown>) {
  desktopApiMock.current = { RecoveryCodeStatus: vi.fn(status) };
}

afterEach(() => vi.restoreAllMocks());

describe('recoveryGateNeeded', () => {
  it.each([
    [{ unused: 8, total: 8, legacy: false, encryption_ready: true }, null],
    [{ unused: 0, total: 8, legacy: false, encryption_ready: true }, null],
    [{ unused: 0, total: 8, legacy: false, encryption_ready: false }, 'none'],
    [{ unused: 8, total: 8, legacy: true, encryption_ready: false }, 'legacy'],
  ])('status %o needs gate %s', async (status, gate) => {
    install(async () => status);
    expect(await recoveryGateNeeded()).toBe(gate);
  });

  it('lets the backend decide when the status cannot be read', async () => {
    install(async () => {
      throw new Error('system database unavailable');
    });
    expect(await recoveryGateNeeded()).toBeNull();
  });
});
