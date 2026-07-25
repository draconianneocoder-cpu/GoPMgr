// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render } from '@testing-library/svelte';

import ChartCatalog from './ChartCatalog.svelte';

const definitions: ChartDefinition[] = [
  {
    kind: 'wbs',
    name: 'Work Breakdown Structure',
    engine: 'dag',
    description: 'Decompose scope into deliverables and work packages.',
    data_example: '{}',
  },
  {
    kind: 'control',
    name: 'Control Chart',
    engine: 'stats',
    description: 'Track process stability against control limits.',
    data_example: '{}',
  },
  {
    kind: 'raci',
    name: 'RACI Matrix',
    engine: 'matrix',
    description: 'Assign responsibility for project work.',
    data_example: '{}',
  },
];

describe('chart catalog', () => {
  it('stays compact until the user opens it', async () => {
    const { getByRole, queryByRole } = render(ChartCatalog, {
      props: { definitions, onCreate: vi.fn() },
    });

    expect(queryByRole('searchbox', { name: /search chart tools/i })).not.toBeInTheDocument();

    await fireEvent.click(getByRole('button', { name: /browse 3 chart tools/i }));

    expect(getByRole('searchbox', { name: /search chart tools/i })).toBeInTheDocument();
    expect(getByRole('button', { name: /hide chart tools/i })).toHaveAttribute('aria-expanded', 'true');
  });

  it('filters by engine and creates the selected backend definition', async () => {
    const onCreate = vi.fn();
    const { getByRole, queryByRole } = render(ChartCatalog, {
      props: { definitions, initiallyExpanded: true, onCreate },
    });

    await fireEvent.click(getByRole('button', { name: /metrics and trends/i }));

    expect(getByRole('button', { name: /control chart/i })).toBeInTheDocument();
    expect(queryByRole('button', { name: /work breakdown structure/i })).not.toBeInTheDocument();

    await fireEvent.click(getByRole('button', { name: /control chart/i }));

    expect(onCreate).toHaveBeenCalledOnce();
    expect(onCreate).toHaveBeenCalledWith(definitions[1]);
  });

  it('reports an empty search without hiding the active filters', async () => {
    const { getByRole } = render(ChartCatalog, {
      props: { definitions, initiallyExpanded: true, onCreate: vi.fn() },
    });

    await fireEvent.input(getByRole('searchbox', { name: /search chart tools/i }), {
      target: { value: 'earned value' },
    });

    expect(getByRole('status')).toHaveTextContent('No chart tools match');
    expect(getByRole('button', { name: /all tools/i })).toHaveAttribute('aria-pressed', 'true');
  });
});
