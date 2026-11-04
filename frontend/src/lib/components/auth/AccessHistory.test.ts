// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render } from '@testing-library/svelte';

import AccessHistory from './AccessHistory.svelte';

const desktopApiMock = vi.hoisted(() => {
  const state: { current: Record<string, ReturnType<typeof vi.fn>> } = { current: {} };
  return state;
});
vi.mock('../../desktop-api', () => ({ get desktopApi() { return desktopApiMock.current; } }));

function setApp(history: () => Promise<AccountEvent[]>) {
  desktopApiMock.current = { MyDataAccessHistory: vi.fn(history) };
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('AccessHistory', () => {
  it('lists every recorded access with its reason', async () => {
    setApp(async () => [
      { id: 7, occurred_at: '2026-10-02T10:00:00Z', actor: 'alice', username: 'bob', action: 'admin_access', detail: 'Ticket 7' },
      { id: 4, occurred_at: '2026-10-01T10:00:00Z', actor: 'carol', username: 'bob', action: 'admin_access', detail: 'Ticket 4' },
    ]);
    const utils = render(AccessHistory);
    expect(await utils.findByText('Reason: Ticket 7')).toBeInTheDocument();
    expect(utils.getByText('Reason: Ticket 4')).toBeInTheDocument();
    expect(utils.getAllByRole('listitem')).toHaveLength(2);
  });

  it('says when no one has opened your data, and when it cannot load', async () => {
    setApp(async () => []);
    const empty = render(AccessHistory);
    expect(await empty.findByText('No administrator has opened your data.')).toBeInTheDocument();
    cleanup();

    setApp(async () => {
      throw new Error('system database unavailable');
    });
    const failed = render(AccessHistory);
    expect((await failed.findByRole('alert')).textContent).toContain('system database unavailable');
  });
});
