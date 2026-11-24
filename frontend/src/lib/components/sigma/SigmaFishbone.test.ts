// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, waitFor } from '@testing-library/svelte';
import SigmaFishbone from './SigmaFishbone.svelte';

// These cases do not call the backend. Unexpected calls must fail.
vi.mock('../../desktop-api', () => ({ desktopApi: {} }));

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

// The 5 Whys drill-down is a modal dialog: named by its heading, it takes
// keyboard focus, closes on Escape and returns focus to the cause that
// opened it (modalFocus; 2026-11-24 dialog focus audit).
describe('SigmaFishbone 5 Whys dialog', () => {
  it('takes focus, closes on Escape and returns focus to the cause', async () => {
    const data: FishboneData = {
      problem_statement: '',
      branches: [{ category: 'Man', causes: [{ id: 'c1', description: 'Late delivery', is_root_cause: false, five_whys: [], evidence: '' }] }],
    };
    const ui = render(SigmaFishbone, { props: { data } });
    const cause = ui.getByRole('button', { name: 'Late delivery' });
    cause.focus();
    await fireEvent.keyDown(cause, { key: 'Enter' });
    const dialog = ui.getByRole('dialog', { name: '5 Whys Drill-Down' });
    await waitFor(() => expect(dialog.contains(document.activeElement)).toBe(true));

    await fireEvent.keyDown(dialog, { key: 'Escape' });
    expect(ui.queryByRole('dialog')).toBeNull();
    expect(document.activeElement).toBe(cause);
  });
});
