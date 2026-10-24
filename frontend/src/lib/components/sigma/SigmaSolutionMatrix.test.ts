// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from '@testing-library/svelte';
import SigmaSolutionMatrix from './SigmaSolutionMatrix.svelte';

const solution: SigmaSolution = {
  id: 'sol-1',
  title: 'New fixture',
  description: '',
  impact: 8,
  effort: 3,
  risk: 2,
  cost: 1284500,
  selected: false,
  status: 'pilot',
};

let app: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  app = {
    SigmaGetSolutions: vi.fn(async () => [{ ...solution }]),
    GetProjectMeta: vi.fn(async () => ({ currency_code: 'EUR' })),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
});

// Costs are in the project's reporting currency; the column used to say
// "Cost ($)" for every project.
describe('SigmaSolutionMatrix cost column', () => {
  it.each(['EUR', 'JPY', 'USD'])('names the %s reporting currency', async (code) => {
    app.GetProjectMeta = vi.fn(async () => ({ currency_code: code }));
    const { findByRole, queryByRole } = render(SigmaSolutionMatrix, { props: { projectID: 'sigma-1' } });

    expect(await findByRole('columnheader', { name: `Cost (${code})` })).toBeInTheDocument();
    expect(queryByRole('columnheader', { name: /\$/ })).toBeNull();
  });

  it('names no currency when the project cannot be read, and still lists the solutions', async () => {
    app.GetProjectMeta = vi.fn(async () => {
      throw new Error('no project open');
    });
    const { findByRole, findByDisplayValue, queryByRole } = render(SigmaSolutionMatrix, { props: { projectID: 'sigma-1' } });

    expect(await findByDisplayValue('New fixture')).toBeInTheDocument();
    expect(await findByRole('columnheader', { name: 'Cost' })).toBeInTheDocument();
    expect(queryByRole('columnheader', { name: /\$/ })).toBeNull();
  });
});
