// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render } from '@testing-library/svelte';
import SigmaFishbone from './SigmaFishbone.svelte';

afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

describe('SigmaFishbone', () => {
  it('assigns distinct cause IDs when additions share a clock tick', async () => {
    const data: FishboneData = {
      problem_statement: '',
      branches: [{ category: 'Man', causes: [] }],
    };
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-07-21T00:00:00Z'));
    vi.stubGlobal('prompt', vi.fn()
      .mockReturnValueOnce('First cause')
      .mockReturnValueOnce('Second cause'));
    const { getByRole } = render(SigmaFishbone, { props: { data } });

    const addCause = getByRole('button', { name: '+ Add Cause' });
    await fireEvent.click(addCause);
    await fireEvent.click(addCause);

    expect(data.branches[0].causes.map((cause) => cause.id)).toEqual(['cause-1', 'cause-2']);
  });
});
