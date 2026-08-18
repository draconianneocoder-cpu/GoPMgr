// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';

import TextareaBindHarness from './__test_harness__/TextareaBindHarness.svelte';

describe('Textarea', () => {
  it('renders the current value and round-trips typed text up through bind:value', async () => {
    const { getByRole, getByTestId } = render(TextareaBindHarness, {
      props: { value: 'start', 'aria-label': 'Notes' },
    });
    const textarea = getByRole('textbox') as HTMLTextAreaElement;
    expect(textarea.value).toBe('start');

    await fireEvent.input(textarea, { target: { value: 'multi\nline' } });

    await waitFor(() => expect(getByTestId('readout').textContent).toBe('multi\nline'));
  });

  it('forwards disabled to the real textarea', () => {
    const { getByLabelText } = render(TextareaBindHarness, {
      props: { value: '', disabled: true, 'aria-label': 'Notes' },
    });
    expect((getByLabelText('Notes') as HTMLTextAreaElement).disabled).toBe(true);
  });
});
