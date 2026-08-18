// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render } from '@testing-library/svelte';

import AppHeader from './AppHeader.svelte';
import { session } from '../session.svelte';

afterEach(() => {
  cleanup();
  session.user = null;
});

// Regression test for the portfolio/dashboard naming collision (design
// critique, 2026-08-17): the nav tab for the `portfolio` view used to read
// "Dashboard" while that view's own heading read "Portfolio dashboard", and
// a SEPARATE per-project view was also informally called "dashboard" —
// two different screens, both calling themselves some form of "dashboard".
// The nav label is the one half of that collision this component owns.
describe('AppHeader', () => {
  it('labels the portfolio nav tab "Portfolio", not "Dashboard"', () => {
    session.user = { username: 'alice', display_name: 'Alice', data_dir: '', created_at: '', last_login: '', is_admin: false };
    const { getByRole, queryByRole } = render(AppHeader, { props: { active: 'portfolio' } });
    expect(getByRole('button', { name: 'Portfolio' })).toBeTruthy();
    expect(queryByRole('button', { name: 'Dashboard' })).toBeNull();
  });

  it('marks the active nav item with aria-current="page"', () => {
    session.user = { username: 'alice', display_name: 'Alice', data_dir: '', created_at: '', last_login: '', is_admin: false };
    const { getByRole } = render(AppHeader, { props: { active: 'portfolio' } });
    const active = getByRole('button', { name: 'Portfolio' });
    expect(active.getAttribute('aria-current')).toBe('page');
    const inactive = getByRole('button', { name: 'Projects' });
    expect(inactive.getAttribute('aria-current')).toBeNull();
  });

  it('shows the Admin nav item only for admin users', () => {
    session.user = { username: 'alice', display_name: 'Alice', data_dir: '', created_at: '', last_login: '', is_admin: false };
    const nonAdmin = render(AppHeader, { props: { active: 'portfolio' } });
    expect(nonAdmin.queryByRole('button', { name: 'Admin' })).toBeNull();
    nonAdmin.unmount();

    session.user = { username: 'bob', display_name: 'Bob', data_dir: '', created_at: '', last_login: '', is_admin: true };
    const admin = render(AppHeader, { props: { active: 'portfolio' } });
    expect(admin.getByRole('button', { name: 'Admin' })).toBeTruthy();
  });
});
