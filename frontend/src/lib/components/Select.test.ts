// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';

import SelectBindHarness from './__test_harness__/SelectBindHarness.svelte';

describe('Select', () => {
  it('renders <option>s passed as snippet children and resolves the initial bound value against them', () => {
    const { getByRole } = render(SelectBindHarness, { props: { value: 'kanban', 'aria-label': 'Methodology' } });
    const select = getByRole('combobox') as HTMLSelectElement;
    expect(select.value).toBe('kanban');
    expect(Array.from(select.options).map((o) => o.value)).toEqual(['scrum', 'kanban', 'scrumban']);
  });

  it('round-trips a user selection up through bind:value to the parent', async () => {
    const { getByRole, getByTestId } = render(SelectBindHarness, {
      props: { value: 'scrum', 'aria-label': 'Methodology' },
    });
    const select = getByRole('combobox') as HTMLSelectElement;

    await fireEvent.change(select, { target: { value: 'scrumban' } });

    await waitFor(() => expect(getByTestId('readout').textContent).toBe('scrumban'));
  });

  it('forwards disabled to the real select', () => {
    const { getByLabelText } = render(SelectBindHarness, {
      props: { value: 'scrum', disabled: true, 'aria-label': 'Methodology' },
    });
    expect((getByLabelText('Methodology') as HTMLSelectElement).disabled).toBe(true);
  });

  it('always includes the focus ring', () => {
    const { getByRole } = render(SelectBindHarness, {
      props: { value: 'scrum', 'aria-label': 'Methodology' },
    });
    expect(getByRole('combobox').className).toContain('focus:border-cyan-500');
  });
});
