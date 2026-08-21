// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { cleanup, render } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';

import BudgetPanel from './BudgetPanel.svelte';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

function installBudget(summary: BudgetSummary) {
  const app = { ComputeBudget: vi.fn(async () => summary) };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
  return app;
}

describe('BudgetPanel', () => {
  it('renders large and negative monetary values without number coercion', async () => {
    installBudget({
      budget: '92233720368547758.07',
      contract_value: '92233720368547758.07',
      labour_estimate: '0.00',
      committed: '92233720368547758.07',
      remaining: '-0.01',
      by_category: { vendor: '92233720368547758.07' },
    });
    const { findAllByText, findByText } = render(BudgetPanel);

    expect((await findAllByText('92233720368547758.07')).length).toBeGreaterThanOrEqual(4);
    expect(await findByText('Remaining: -0.01')).toBeInTheDocument();
  });

  it('uses an approximate value only for display progress', async () => {
    installBudget({
      budget: '100.00',
      contract_value: '125.00',
      labour_estimate: '0.00',
      committed: '125.00',
      remaining: '-25.00',
      by_category: {},
    });
    const { findByText, container } = render(BudgetPanel);

    expect(await findByText('125% committed')).toBeInTheDocument();
    expect(container.querySelector('[style*="width: 100"]')).toBeInTheDocument();
  });
});
