// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';

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

  it('renders exact portfolio money strings without JavaScript arithmetic', async () => {
    session.user = { username: 'alice', display_name: 'Alice', data_dir: '', created_at: '', last_login: '', is_admin: false };
    const app = setApp({
      RunPortfolioAnalytics: vi.fn(async () => ({
        project_count: 2,
        evm_project_count: 1,
        evm_unavailable_project_count: 1,
        as_of_date: '2026-08-21',
		currency_code: 'USD',
        total_budgeted_cost: '92233720368547758.07',
        total_committed_cost: '92233720368547758.08',
        total_actual_cost: '-92233720368547758.08',
        total_earned_value: '0.01',
        total_planned_value: '0.01',
        remaining: '-0.01',
        schedule_performance_index: 1,
        cost_performance_index: -0.5,
      })),
    });
    const { getByRole, getByText } = render(Portfolio);
    await waitFor(() => expect(app.ProjectsOverview).toHaveBeenCalled());
    await fireEvent.click(getByRole('button', { name: 'Run rollup' }));

    await waitFor(() => expect(app.RunPortfolioAnalytics).toHaveBeenCalledOnce());
    expect(getByText('92,233,720,368,547,758.07')).toBeTruthy();
    expect(getByText('92,233,720,368,547,758.08')).toBeTruthy();
    expect(getByText('-0.01')).toBeTruthy();
    expect(getByText('-92,233,720,368,547,758.08')).toBeTruthy();
		expect(getByText('Reporting currency: USD')).toBeTruthy();
  });

	it('shows a mixed-currency refusal without retaining a prior rollup', async () => {
		session.user = { username: 'alice', display_name: 'Alice', data_dir: '', created_at: '', last_login: '', is_admin: false };
		const app = setApp({
			RunPortfolioAnalytics: vi.fn(async () => {
				throw new Error('portfolio analytics cannot combine EUR and USD reporting currencies because foreign exchange conversion is not implemented');
			}),
		});
		const { getByRole, getByText, queryByText } = render(Portfolio);
		await waitFor(() => expect(app.ProjectsOverview).toHaveBeenCalled());
		await fireEvent.click(getByRole('button', { name: 'Run rollup' }));

		await waitFor(() => expect(getByText(/cannot combine EUR and USD reporting currencies/)).toBeTruthy());
		expect(queryByText('Reporting currency: USD')).toBeNull();
	});
});
