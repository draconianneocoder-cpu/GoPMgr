// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';

import InputBindHarness from './__test_harness__/InputBindHarness.svelte';

describe('Input', () => {
  it('renders the current value and round-trips typed text up through bind:value to the parent', async () => {
    const { getByRole, getByTestId } = render(InputBindHarness, { props: { value: 'start' } });
    const input = getByRole('textbox') as HTMLInputElement;
    expect(input.value).toBe('start');

    await fireEvent.input(input, { target: { value: 'typed' } });

    await waitFor(() => expect(getByTestId('readout').textContent).toBe('typed'));
  });

  // Post-implementation review concern (mid-implementation review, this pass):
  // bind:value on a native <input type="number"> coerces to a JS number.
  // Routed through this component's own $bindable() hop, that coercion is
  // the plausible silent-failure point — prove it survives the wrapper.
  it('coerces to a number through the wrapper for type="number", not a string', async () => {
    const { getByRole, getByTestId } = render(InputBindHarness, {
      props: { value: 1, type: 'number', 'aria-label': 'Amount' },
    });
    const input = getByRole('spinbutton') as HTMLInputElement;

    await fireEvent.input(input, { target: { value: '42' } });

    await waitFor(() => expect(getByTestId('readout').textContent).toBe('42'));
    expect(getByTestId('readout-type').textContent).toBe('number');
  });

  it('reaches type="password" onto the real DOM element', () => {
    const { getByLabelText } = render(InputBindHarness, {
      props: { value: '', type: 'password', 'aria-label': 'Password' },
    });
    expect((getByLabelText('Password') as HTMLInputElement).type).toBe('password');
  });

  it('forwards disabled and aria-label to the real input', () => {
    const { getByLabelText } = render(InputBindHarness, {
      props: { value: '', disabled: true, 'aria-label': 'Notes' },
    });
    const input = getByLabelText('Notes') as HTMLInputElement;
    expect(input.disabled).toBe(true);
  });

  it('appends a passed-through class without dropping the base classes', () => {
    const { getByRole } = render(InputBindHarness, { props: { value: '', class: 'w-24', 'aria-label': 'F' } });
    const input = getByRole('textbox');
    expect(input.className).toContain('w-24');
    expect(input.className).toContain('focus:border-cyan-500');
  });
});
