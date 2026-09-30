// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';

import ColumnManager from './ColumnManager.svelte';

const col = (id: string, name: string, order_idx: number, wip_limit = 0): AgileColumn => ({
  id,
  board_id: 'board-1',
  name,
  order_idx,
  wip_limit,
});
const initial = [col('todo', 'To Do', 0), col('doing', 'In Progress', 1, 3), col('blocked', 'Blocked', 2), col('done', 'Done', 3)];

let app: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  app = {
    SaveColumn: vi.fn(async () => undefined),
    DeleteColumn: vi.fn(async () => undefined),
    EnsureDefaultBoard: vi.fn(async () => ({ board: { id: 'board-1' }, columns: initial })),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

function setup(itemCounts: Record<string, number> = {}) {
  const onchange = vi.fn();
  const utils = render(ColumnManager, {
    props: { boardId: 'board-1', columns: initial, itemCounts, onchange, onclose: vi.fn() },
  });
  return { utils, onchange };
}

describe('ColumnManager', () => {
  it('saves every column with its new position after a move, then reloads', async () => {
    const { utils, onchange } = setup();
    await fireEvent.click(utils.getByRole('button', { name: 'Move column 3 left' }));
    await fireEvent.input(utils.getByLabelText('Column 2 WIP limit'), { target: { value: '5' } });
    await fireEvent.click(utils.getByRole('button', { name: 'Save columns' }));
    await waitFor(() => expect(onchange).toHaveBeenCalledOnce());
    expect(app.SaveColumn.mock.calls.map(([c]) => [c.id, c.order_idx, c.wip_limit])).toEqual([
      ['todo', 0, 0],
      ['blocked', 1, 5],
      ['doing', 2, 3],
      ['done', 3, 0],
    ]);
    expect(app.SaveColumn.mock.calls.every(([c]) => c.board_id === 'board-1')).toBe(true);
  });

  it('adds a column with no ID so the backend assigns one', async () => {
    const { utils } = setup();
    await fireEvent.click(utils.getByRole('button', { name: '+ Column' }));
    await fireEvent.input(utils.getByLabelText('Column 5 name'), { target: { value: '  QA  ' } });
    await fireEvent.click(utils.getByRole('button', { name: 'Save columns' }));
    await waitFor(() => expect(app.SaveColumn).toHaveBeenCalledTimes(5));
    expect(app.SaveColumn.mock.calls[4][0]).toMatchObject({ id: '', name: 'QA', order_idx: 4 });
  });

  it('refuses a blank name before saving anything', async () => {
    const { utils } = setup();
    await fireEvent.input(utils.getByLabelText('Column 1 name'), { target: { value: '   ' } });
    await fireEvent.click(utils.getByRole('button', { name: 'Save columns' }));
    expect(await utils.findByRole('alert')).toHaveTextContent('Give every column a name.');
    expect(app.SaveColumn).not.toHaveBeenCalled();
  });

  it('offers no delete for built-in columns and blocks deleting a column with work items', () => {
    const { utils } = setup({ blocked: 2 });
    expect(utils.queryByRole('button', { name: 'Delete column To Do' })).not.toBeInTheDocument();
    expect(utils.queryByRole('button', { name: 'Delete column Done' })).not.toBeInTheDocument();
    const del = utils.getByRole('button', { name: 'Delete column Blocked' });
    expect(del).toBeDisabled();
    expect(del).toHaveAttribute('title', 'Move its 2 work items out first.');
  });

  it('deletes an empty added column only after confirmation, then reloads', async () => {
    const { utils, onchange } = setup();
    await fireEvent.click(utils.getByRole('button', { name: 'Delete column Blocked' }));
    expect(app.DeleteColumn).not.toHaveBeenCalled();
    await fireEvent.click(utils.getByRole('button', { name: 'Delete column' }));
    await waitFor(() => expect(onchange).toHaveBeenCalledOnce());
    expect(app.DeleteColumn).toHaveBeenCalledWith('blocked');
  });

  it('holds deletes while there are unsaved changes', async () => {
    const { utils } = setup();
    await fireEvent.input(utils.getByLabelText('Column 1 name'), { target: { value: 'Ready' } });
    expect(utils.getByRole('button', { name: 'Delete column Blocked' })).toBeDisabled();
  });

  it('shows the backend refusal when a card arrived in the column meanwhile', async () => {
    app.DeleteColumn.mockRejectedValueOnce('Move the 1 work item out of this column before deleting it.');
    const { utils, onchange } = setup();
    await fireEvent.click(utils.getByRole('button', { name: 'Delete column Blocked' }));
    await fireEvent.click(utils.getByRole('button', { name: 'Delete column' }));
    expect(await utils.findByRole('alert')).toHaveTextContent('Move the 1 work item out of this column before deleting it.');
    expect(onchange).not.toHaveBeenCalled();
  });
});
