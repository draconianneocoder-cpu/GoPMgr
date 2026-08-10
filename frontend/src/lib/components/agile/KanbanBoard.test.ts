// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';

import KanbanBoard from './KanbanBoard.svelte';
import { session } from '../../session.svelte';

let app: Record<string, ReturnType<typeof vi.fn>>;

const board = { id: 'b1', project_id: 'p1', name: 'Default', is_default: true, created_at: '', updated_at: '' };
const columns = [
  { id: 'col-1', board_id: 'b1', name: 'To Do', order_idx: 0, wip_limit: 0 },
  { id: 'col-2', board_id: 'b1', name: 'Done', order_idx: 1, wip_limit: 0 },
];
const items: AgileWorkItem[] = [
  {
    id: 'wi-1',
    project_id: 'p1',
    type: 'story',
    title: 'First card',
    description: '',
    state: 'col-1',
    points: 0,
    assignee: '',
    sprint_id: '',
    priority: 'medium',
    order_idx: 0,
    created_at: '',
    updated_at: '',
  },
  {
    id: 'wi-2',
    project_id: 'p1',
    type: 'story',
    title: 'Second card',
    description: '',
    state: 'col-1',
    points: 0,
    assignee: '',
    sprint_id: '',
    priority: 'medium',
    order_idx: 1,
    created_at: '',
    updated_at: '',
  },
];

beforeEach(() => {
  session.project = { id: 'p1' } as unknown as typeof session.project;
  app = {
    EnsureDefaultBoard: vi.fn(async () => ({ board, columns })),
    ListWorkItems: vi.fn(async () => items),
    ListSprints: vi.fn(async () => []),
    WIPCounts: vi.fn(async () => ({})),
    SaveWorkItem: vi.fn(async (wi: AgileWorkItem) => wi),
    DeleteWorkItem: vi.fn(async () => undefined),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
});

afterEach(() => {
  vi.restoreAllMocks();
  session.project = null;
});

describe('KanbanBoard modal backdrop', () => {
  // jsdom does not implement paint-order hit-testing, so `fireEvent.click`
  // on an obscured element still dispatches directly to it — it cannot
  // prove a real click is blocked. That claim (a real click on a card
  // behind the open editor's backdrop does not reach the card, so
  // switching cards mid-edit cannot silently discard a draft) was instead
  // verified manually in a real browser: a `position:fixed; inset:0;
  // z-index:40` backdrop identical to this component's over a plain
  // unstyled button beneath it, clicked at the button's coordinates via
  // the OS input path, left the button's own click listener unfired. This
  // test guards the CSS contract that verification depends on: the
  // backdrop must keep covering the full viewport above the board.
  it('opens the editor behind a full-viewport backdrop over the board', async () => {
    const utils = render(KanbanBoard);
    await waitFor(() => expect(app.ListWorkItems).toHaveBeenCalled());

    const cardButtons = () =>
      Array.from(utils.container.querySelectorAll('button')).filter((b) =>
        b.textContent?.includes('First card'),
      );
    await waitFor(() => expect(cardButtons().length).toBe(1));
    await fireEvent.click(cardButtons()[0]);

    const backdrop = await waitFor(() => {
      const el = document.querySelector('[data-role="backdrop"]');
      expect(el).not.toBeNull();
      return el as HTMLElement;
    });
    expect(backdrop.className).toMatch(/\bfixed\b/);
    expect(backdrop.className).toMatch(/\binset-0\b/);
    expect(backdrop.className).toMatch(/\bz-40\b/);
  });
});
