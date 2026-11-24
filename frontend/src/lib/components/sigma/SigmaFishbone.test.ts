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
    const { getByRole, getByLabelText } = render(SigmaFishbone, { props: { data } });

    const addCause = getByRole('button', { name: '+ Add Cause' });
    for (const description of ['First cause', 'Second cause']) {
      await fireEvent.click(addCause);
      await fireEvent.input(getByLabelText('Cause description'), { target: { value: description } });
      await fireEvent.click(getByRole('button', { name: 'Add' }));
    }

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

// window.prompt() returns null at once in the macOS app: Wails v3.1.0's
// WKUIDelegate does not implement runJavaScriptTextInputPanel, so the old
// prompt-based "+ Add Cause" silently added nothing. The cause is now
// entered in an in-app dialog.
describe('SigmaFishbone add cause dialog', () => {
  it('adds the typed cause from a named, focused dialog without window.prompt', async () => {
    const promptSpy = vi.fn(() => null);
    vi.stubGlobal('prompt', promptSpy);
    const data: FishboneData = { problem_statement: '', branches: [{ category: 'Machine', causes: [] }] };
    const ui = render(SigmaFishbone, { props: { data } });

    await fireEvent.click(ui.getByRole('button', { name: '+ Add Cause' }));
    const dialog = ui.getByRole('dialog', { name: 'Add cause' });
    const field = ui.getByLabelText('Cause description');
    await waitFor(() => expect(document.activeElement).toBe(field));
    expect(ui.getByRole('button', { name: 'Add' })).toBeDisabled();

    await fireEvent.input(field, { target: { value: '  Worn bearing  ' } });
    await fireEvent.submit(dialog.querySelector('form')!);
    expect(data.branches[0].causes.map((c) => c.description)).toEqual(['Worn bearing']);
    expect(ui.queryByRole('dialog')).toBeNull();
    expect(promptSpy).not.toHaveBeenCalled();
  });

  it('adds nothing when cancelled with Escape', async () => {
    const data: FishboneData = { problem_statement: '', branches: [{ category: 'Machine', causes: [] }] };
    const ui = render(SigmaFishbone, { props: { data } });
    await fireEvent.click(ui.getByRole('button', { name: '+ Add Cause' }));
    await fireEvent.input(ui.getByLabelText('Cause description'), { target: { value: 'Draft' } });
    await fireEvent.keyDown(ui.getByRole('dialog', { name: 'Add cause' }), { key: 'Escape' });
    expect(ui.queryByRole('dialog')).toBeNull();
    expect(data.branches[0].causes).toEqual([]);
  });
});
