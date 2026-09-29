// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render } from '@testing-library/svelte';

const goto = vi.fn();
vi.mock('../../session.svelte', () => ({ goto: (...args: unknown[]) => goto(...args) }));

import ProjectRepairPanel from './ProjectRepairPanel.svelte';

let app: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  goto.mockReset();
  app = { RepairAndSwap: vi.fn() };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

async function run() {
  const utils = render(ProjectRepairPanel);
  await fireEvent.click(utils.getByRole('button', { name: 'Check and repair' }));
  return utils;
}

describe('ProjectRepairPanel', () => {
  it('says a healthy project has no problems and offers no reload', async () => {
    app.RepairAndSwap.mockResolvedValue({ success: true, swapped: false, log: ['Starting diagnostic check...', 'No corruption found.'] });
    const utils = await run();
    expect(await utils.findByRole('status')).toHaveTextContent('No problems found.');
    expect(utils.queryByRole('button', { name: 'Reload the project' })).not.toBeInTheDocument();
  });

  it('names where the damaged copy was kept and sends the user back to reload the project', async () => {
    app.RepairAndSwap.mockResolvedValue({
      success: true,
      swapped: true,
      damaged_copy: '/data/alice/projects/Plan/Plan.gopmgr.corrupt',
      log: ['Corruption found.'],
    });
    const utils = await run();
    expect(await utils.findByRole('status')).toHaveTextContent(
      'Repaired. The damaged copy was kept at /data/alice/projects/Plan/Plan.gopmgr.corrupt.',
    );
    await fireEvent.click(utils.getByRole('button', { name: 'Reload the project' }));
    expect(goto).toHaveBeenCalledWith('dashboard');
  });

  it('points to a backup when the repair fails', async () => {
    app.RepairAndSwap.mockRejectedValue('file is not a database');
    const utils = await run();
    expect(await utils.findByRole('alert')).toHaveTextContent(
      'This project could not be repaired: file is not a database. Restore it from a backup.',
    );
  });

  it('ignores a second click while the check runs', async () => {
    let finish!: (v: unknown) => void;
    app.RepairAndSwap.mockReturnValue(new Promise((r) => (finish = r)));
    const utils = render(ProjectRepairPanel);
    const button = utils.getByRole('button', { name: 'Check and repair' });
    await fireEvent.click(button);
    await fireEvent.click(utils.getByRole('button', { name: 'Checking…' }));
    expect(app.RepairAndSwap).toHaveBeenCalledOnce();
    finish({ success: true, swapped: false, log: [] });
  });
});
