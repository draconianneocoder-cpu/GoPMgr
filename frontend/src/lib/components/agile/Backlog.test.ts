// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor } from '@testing-library/svelte';

import Backlog from './Backlog.svelte';
import { session } from '../../session.svelte';

let app: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  session.project = { id: 'p1' } as unknown as typeof session.project;
  app = {
    EnsureDefaultBoard: vi.fn(async () => ({
      board: { id: 'b1', project_id: 'p1', name: 'Default', is_default: true, created_at: '', updated_at: '' },
      columns: [{ id: 'col-1', board_id: 'b1', name: 'To Do', order_idx: 0, wip_limit: 0 }],
    })),
    ListWorkItems: vi.fn(async () => []),
    ListSprints: vi.fn(async () => []),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
});

afterEach(() => {
  vi.restoreAllMocks();
  session.project = null;
});

// Added 2026-08-19 alongside migrating the header "&larr; Dashboard" nav
// link (previously a raw <button>) to the `nav` Button variant — see
// Button.svelte's `nav` branch comment (tier-3 deferred file, closed
// this cycle).
describe('Backlog migrated header "&larr; Dashboard" button', () => {
  it('Button variant="nav"', async () => {
    const { getByText } = render(Backlog);
    await waitFor(() => expect(app.EnsureDefaultBoard).toHaveBeenCalled());

    const btn = getByText('← Dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'text-xs text-slate-400 hover:text-cyan-400 disabled:opacity-50'.split(/\s+/).sort(),
    );
  });
});
