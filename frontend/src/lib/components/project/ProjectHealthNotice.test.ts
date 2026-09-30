// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';

const { goto, session } = vi.hoisted(() => ({
  goto: vi.fn(),
  session: { settingsTab: null as 'protection' | null },
}));
vi.mock('../../session.svelte', () => ({ goto: (...args: unknown[]) => goto(...args), session }));

import ProjectHealthNotice from './ProjectHealthNotice.svelte';

let app: Record<string, ReturnType<typeof vi.fn>>;
const check = (over: Partial<OpenProjectCheck> = {}) => ({ checked: true, damaged: true, dismissed: false, ...over });
const heading = { name: 'This project may be damaged' };

beforeEach(() => {
  goto.mockReset();
  session.settingsTab = null;
  app = {
    CheckOpenProject: vi.fn(async () => check()),
    DismissOpenProjectCheck: vi.fn(async () => undefined),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('ProjectHealthNotice', () => {
  it('shows when the check on open found damage, without an alert role', async () => {
    const utils = render(ProjectHealthNotice);
    expect(await utils.findByRole('heading', heading)).toBeInTheDocument();
    expect(utils.queryByRole('alert')).not.toBeInTheDocument();
  });

  it.each([
    ['the setting is off', check({ checked: false, damaged: false })],
    ['the project is healthy', check({ damaged: false })],
    ['the user dismissed it this open', check({ dismissed: true })],
  ])('stays hidden when %s', async (_, result) => {
    app.CheckOpenProject.mockResolvedValue(result);
    const utils = render(ProjectHealthNotice);
    await waitFor(() => expect(app.CheckOpenProject).toHaveBeenCalled());
    expect(utils.queryByRole('heading', heading)).not.toBeInTheDocument();
  });

  it('stays hidden when the check cannot be read', async () => {
    app.CheckOpenProject.mockRejectedValue('no project open');
    const utils = render(ProjectHealthNotice);
    await waitFor(() => expect(app.CheckOpenProject).toHaveBeenCalled());
    expect(utils.queryByRole('heading', heading)).not.toBeInTheDocument();
  });

  it('opens Project Settings on Data Protection', async () => {
    const utils = render(ProjectHealthNotice);
    await fireEvent.click(await utils.findByRole('button', { name: 'Check and repair' }));
    expect(session.settingsTab).toBe('protection');
    expect(goto).toHaveBeenCalledWith('project_settings');
  });

  it('dismisses for the rest of this open', async () => {
    const utils = render(ProjectHealthNotice);
    await fireEvent.click(await utils.findByRole('button', { name: 'Dismiss' }));
    expect(utils.queryByRole('heading', heading)).not.toBeInTheDocument();
    expect(app.DismissOpenProjectCheck).toHaveBeenCalledOnce();
  });
});
