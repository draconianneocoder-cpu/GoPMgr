// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { Call } from '@wailsio/runtime';
import { describe, expect, it, vi } from 'vitest';

// Only the generated bindings are replaced; the wrapper and the Wails
// RuntimeError class are real.
vi.mock('../../bindings/gopmgr/internal/desktop/index', () => ({
  App: {
    ProjectsOverview: vi.fn(async () => {
      throw new Call.RuntimeError('no project open');
    }),
    GetChart: vi.fn(async (id: string) => ({ id })),
    Broken: vi.fn(async () => {
      throw new TypeError('wrong argument count');
    }),
  },
}));

import { BackendError, desktopApi } from './desktop-api';

type Fake = Record<string, (...args: unknown[]) => Promise<unknown>>;
const api = desktopApi as unknown as Fake;

describe('desktopApi', () => {
  it('reports a Go method error as its own message, keeping the original as cause', async () => {
    const err = await api.ProjectsOverview().catch((e: unknown) => e);

    // What the interface shows for `Could not load projects: ${e}`.
    expect(`${err}`).toBe('no project open');
    expect(String(err)).toBe('no project open');
    expect(err).toBeInstanceOf(BackendError);
    expect((err as Error).cause).toBeInstanceOf(Call.RuntimeError);
  });

  it('passes results and arguments through, and leaves other rejections unchanged', async () => {
    await expect(api.GetChart('c1')).resolves.toEqual({ id: 'c1' });

    const err = await api.Broken().catch((e: unknown) => e);
    expect(err).toBeInstanceOf(TypeError);
    expect(String(err)).toBe('TypeError: wrong argument count');
  });
});
