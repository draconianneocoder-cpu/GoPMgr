// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';

import BaselineList from './BaselineList.svelte';

const newer = { id: 'b2', project_id: 'p', chart_id: 'c1', name: 'Replan', data: '{}', created_at: '2026-09-02T10:00:00Z' };
const older = { id: 'b1', project_id: 'p', chart_id: 'c1', name: '', data: '{}', created_at: '2026-08-01T10:00:00Z' };

let app: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  app = {
    ListScheduleBaselines: vi.fn(async () => [newer, older]),
    DeleteScheduleBaseline: vi.fn(async () => undefined),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

async function openList(onchange = vi.fn()) {
  const utils = render(BaselineList, { props: { chartId: 'c1', onchange } });
  await fireEvent.click(utils.getByRole('button', { name: 'Baselines' }));
  await utils.findByRole('button', { name: /Delete baseline Replan/ });
  return { utils, onchange };
}

describe('BaselineList', () => {
  it('lists the chart baselines newest first and marks the compared one', async () => {
    const { utils } = await openList();
    expect(app.ListScheduleBaselines).toHaveBeenCalledWith('c1');
    const items = utils.getAllByRole('listitem');
    expect(items[0]).toHaveTextContent(/Replan.*\(compared\)/);
    expect(items[1]).not.toHaveTextContent('(compared)');
  });

  it('reloads the list each time it opens', async () => {
    const { utils } = await openList();
    await fireEvent.click(utils.getByRole('button', { name: 'Baselines' }));
    await fireEvent.click(utils.getByRole('button', { name: 'Baselines' }));
    await waitFor(() => expect(app.ListScheduleBaselines).toHaveBeenCalledTimes(2));
  });

  it('deletes only after confirmation, then reloads and tells the editor', async () => {
    const { utils, onchange } = await openList();
    await fireEvent.click(utils.getByRole('button', { name: /Delete baseline Replan/ }));
    expect(app.DeleteScheduleBaseline).not.toHaveBeenCalled();
    app.ListScheduleBaselines.mockResolvedValueOnce([older]);
    await fireEvent.click(utils.getByRole('button', { name: 'Delete baseline' }));
    await waitFor(() => expect(onchange).toHaveBeenCalledOnce());
    expect(app.DeleteScheduleBaseline).toHaveBeenCalledWith('b2');
    expect(utils.queryByRole('button', { name: /Delete baseline Replan/ })).not.toBeInTheDocument();
  });

  it('keeps the baseline when the user cancels', async () => {
    const { utils, onchange } = await openList();
    await fireEvent.click(utils.getByRole('button', { name: /Delete baseline Replan/ }));
    await fireEvent.click(utils.getByRole('button', { name: 'Keep baseline' }));
    expect(app.DeleteScheduleBaseline).not.toHaveBeenCalled();
    expect(onchange).not.toHaveBeenCalled();
  });

  it('shows why a baseline a scenario depends on was not deleted', async () => {
    app.DeleteScheduleBaseline.mockRejectedValueOnce(
      'The scenario "Late vendor" is based on this baseline. Change its source baseline in Project Settings › Scenarios before deleting it.',
    );
    const { utils, onchange } = await openList();
    await fireEvent.click(utils.getByRole('button', { name: /Delete baseline Replan/ }));
    await fireEvent.click(utils.getByRole('button', { name: 'Delete baseline' }));
    expect(await utils.findByRole('alert')).toHaveTextContent('The scenario "Late vendor" is based on this baseline.');
    expect(onchange).not.toHaveBeenCalled();
  });
});
