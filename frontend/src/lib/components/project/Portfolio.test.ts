// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, waitFor } from '@testing-library/svelte';

import Portfolio from './Portfolio.svelte';
import { session } from '../../session.svelte';

afterEach(() => {
  cleanup();
  session.user = null;
});

function setApp(overrides: Record<string, ReturnType<typeof vi.fn>> = {}) {
  const app = {
    ProjectsOverview: vi.fn(async () => []),
    ...overrides,
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
  return app;
}

// Regression test for the portfolio/dashboard naming collision (design
// critique, 2026-08-17): this screen's own heading used to read "Portfolio
// dashboard" -- the same word ("dashboard") the nav tab pointing to it
// used, while a completely separate per-project view was also informally
// called "dashboard". This is the heading half of that collision;
// AppHeader.test.ts covers the nav label half.
describe('Portfolio', () => {
  it('headings the screen "Portfolio", not "Portfolio dashboard"', async () => {
    session.user = { username: 'alice', display_name: 'Alice', data_dir: '', created_at: '', last_login: '', is_admin: false };
    const app = setApp();
    const { getByRole, queryByText } = render(Portfolio);
    await waitFor(() => expect(app.ProjectsOverview).toHaveBeenCalled());
    expect(getByRole('heading', { name: 'Portfolio', level: 1 })).toBeTruthy();
    expect(queryByText('Portfolio dashboard')).toBeNull();
  });
});
