// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/svelte';

import CardChildrenHarness from './__test_harness__/CardChildrenHarness.svelte';

describe('Card', () => {
  it('renders its children inside the panel wrapper', () => {
    const { getByText } = render(CardChildrenHarness, { props: { text: 'Panel content' } });
    expect(getByText('Panel content')).toBeTruthy();
  });

  it('reconstructs the confirmed ChartCatalog/DocumentCatalog wrapper class string exactly', () => {
    const { getByText, container } = render(CardChildrenHarness, { props: { text: 'X' } });
    const card = getByText('X').parentElement as HTMLElement;
    expect(card).toBe(container.firstElementChild);
    const classes = card.className.split(/\s+/).filter(Boolean).sort();
    const expected = 'overflow-hidden rounded-lg border border-slate-800 bg-slate-900/60'.split(/\s+/).sort();
    expect(classes).toEqual(expected);
  });

  it('appends a passed-through class', () => {
    const { getByText } = render(CardChildrenHarness, { props: { text: 'X', class: 'p-4' } });
    expect((getByText('X').parentElement as HTMLElement).className).toContain('p-4');
  });

  it('renders a plain div, not a button', () => {
    const { getByText } = render(CardChildrenHarness, { props: { text: 'X' } });
    expect((getByText('X').parentElement as HTMLElement).tagName).toBe('DIV');
  });
});
