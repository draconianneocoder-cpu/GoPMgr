// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';

import ChartFallback from './ChartFallback.svelte';
import { session } from '../../session.svelte';

afterEach(() => {
  cleanup();
  session.editingId = null;
  session.view = 'portfolio';
});

function setApp(overrides: Record<string, ReturnType<typeof vi.fn>> = {}) {
  const app = {
    GetChart: vi.fn(async () => ({
      id: 'chart-1', project_id: 'p1', kind: 'future_kind', title: 'Untitled Future Chart',
      data: '{}', config: '{}', template_id: '', created_at: '', updated_at: '',
    })),
    ...overrides,
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
  return app;
}

// Regression coverage for the 'charts' registry-drift fallback (design
// doc §3.2): Dashboard.svelte's own tested fallback path
// (Dashboard.test.ts: "falls back to the generic charts view for a chart
// kind with no registered route") only proves session.view becomes
// 'charts'. This component is what a user actually sees once they land
// there -- it must say the chart was saved (true: both call sites that can
// reach this view only navigate after their SaveChart succeeds), not read
// as an error.
describe('ChartFallback', () => {
  it('names the saved chart by title once the lookup succeeds', async () => {
    session.editingId = 'chart-1';
    const app = setApp();
    const { getByText } = render(ChartFallback);
    await waitFor(() => expect(app.GetChart).toHaveBeenCalledWith('chart-1'));
    await waitFor(() => expect(getByText(/Untitled Future Chart/)).toBeTruthy());
    expect(getByText(/was saved/)).toBeTruthy();
  });

  it('falls back to generic copy, not an error, if the title lookup itself fails', async () => {
    session.editingId = 'chart-1';
    setApp({ GetChart: vi.fn(async () => { throw new Error('boom'); }) });
    const { getByText } = render(ChartFallback);
    await waitFor(() => expect(getByText(/Your chart was saved/)).toBeTruthy());
    expect(getByText(/Could not look up the chart's title, but the save itself succeeded/)).toBeTruthy();
  });

  it('navigates back to the dashboard', async () => {
    session.editingId = 'chart-1';
    const app = setApp();
    const { getByRole } = render(ChartFallback);
    await waitFor(() => expect(app.GetChart).toHaveBeenCalled());
    await fireEvent.click(getByRole('button', { name: 'Back to dashboard' }));
    expect(session.view).toBe('dashboard');
  });
});
