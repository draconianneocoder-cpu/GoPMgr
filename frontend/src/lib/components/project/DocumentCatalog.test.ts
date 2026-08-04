// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render } from '@testing-library/svelte';

import DocumentCatalog from './DocumentCatalog.svelte';

const definitions: DocumentDefinition[] = [
  {
    kind: 'charter_word',
    name: 'Project Charter (Word)',
    phase: 'initiation',
    description: 'Authorise the project and record its initial boundaries.',
    fields: [],
  },
  {
    kind: 'project_plan_word',
    name: 'Project Plan (Word)',
    phase: 'planning',
    description: 'Define how the project will be delivered and controlled.',
    fields: [],
  },
  {
    kind: 'status_report',
    name: 'Status Report',
    phase: 'monitoring',
    description: 'Communicate progress, decisions, risks, and next actions.',
    fields: [],
  },
];

describe('document catalog', () => {
  it('stays compact until the user opens it', async () => {
    const { getByRole, queryByRole } = render(DocumentCatalog, {
      props: { definitions, onCreate: vi.fn() },
    });

    expect(queryByRole('searchbox', { name: /search document templates/i })).not.toBeInTheDocument();

    await fireEvent.click(getByRole('button', { name: /browse 3 document templates/i }));

    expect(getByRole('searchbox', { name: /search document templates/i })).toBeInTheDocument();
    expect(getByRole('button', { name: /hide document templates/i })).toHaveAttribute('aria-expanded', 'true');
  });

  it('filters by lifecycle phase and creates the selected registry definition', async () => {
    const onCreate = vi.fn();
    const { getByRole, queryByRole } = render(DocumentCatalog, {
      props: { definitions, initiallyExpanded: true, onCreate },
    });

    await fireEvent.click(getByRole('button', { name: /^monitoring$/i }));

    expect(getByRole('button', { name: /status report/i })).toBeInTheDocument();
    expect(queryByRole('button', { name: /project charter/i })).not.toBeInTheDocument();

    await fireEvent.click(getByRole('button', { name: /status report/i }));

    expect(onCreate).toHaveBeenCalledOnce();
    expect(onCreate).toHaveBeenCalledWith(definitions[2]);
  });

  it('searches document purpose and reports an empty result', async () => {
    const { getByRole, queryByRole } = render(DocumentCatalog, {
      props: { definitions, initiallyExpanded: true, onCreate: vi.fn() },
    });
    const search = getByRole('searchbox', { name: /search document templates/i });

    await fireEvent.input(search, { target: { value: 'boundaries' } });
    expect(getByRole('button', { name: /project charter/i })).toBeInTheDocument();
    expect(queryByRole('button', { name: /status report/i })).not.toBeInTheDocument();

    await fireEvent.input(search, { target: { value: 'earned value' } });
    expect(getByRole('status')).toHaveTextContent('No document templates match');
  });
});
