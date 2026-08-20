// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, fireEvent, render } from '@testing-library/svelte';

import Tabs from './Tabs.svelte';

afterEach(() => cleanup());

const tabs = [
  { id: 'a', label: 'Alpha' },
  { id: 'b', label: 'Beta' },
  { id: 'c', label: 'Gamma' },
];

describe('Tabs', () => {
  it('renders one tab per entry with the active one selected', () => {
    const { getByRole } = render(Tabs, { tabs, activeTab: 'b', idPrefix: 'x', label: 'Sections' });

    expect(getByRole('tablist', { name: 'Sections' })).toBeInTheDocument();
    expect(getByRole('tab', { selected: true })).toHaveTextContent('Beta');
    expect(getByRole('tab', { name: 'Alpha' })).toHaveAttribute('aria-selected', 'false');
  });

  it('clicking an inactive tab selects it', async () => {
    const { getByRole } = render(Tabs, { tabs, activeTab: 'a', idPrefix: 'x', label: 'Sections' });

    await fireEvent.click(getByRole('tab', { name: 'Gamma' }));

    expect(getByRole('tab', { selected: true })).toHaveTextContent('Gamma');
  });

  it('only the active tab is in the roving tab-stop order', () => {
    const { getByRole } = render(Tabs, { tabs, activeTab: 'a', idPrefix: 'x', label: 'Sections' });

    expect(getByRole('tab', { name: 'Alpha' })).toHaveAttribute('tabindex', '0');
    expect(getByRole('tab', { name: 'Beta' })).toHaveAttribute('tabindex', '-1');
    expect(getByRole('tab', { name: 'Gamma' })).toHaveAttribute('tabindex', '-1');
  });

  it('ArrowRight/ArrowLeft move selection and wrap at the ends', async () => {
    const { getByRole } = render(Tabs, { tabs, activeTab: 'a', idPrefix: 'x', label: 'Sections' });

    await fireEvent.keyDown(getByRole('tab', { name: 'Alpha' }), { key: 'ArrowRight' });
    expect(getByRole('tab', { selected: true })).toHaveTextContent('Beta');

    await fireEvent.keyDown(getByRole('tab', { name: 'Beta' }), { key: 'ArrowLeft' });
    expect(getByRole('tab', { selected: true })).toHaveTextContent('Alpha');

    // Wraps: ArrowLeft from the first tab goes to the last, ArrowRight from
    // the last wraps back to the first.
    await fireEvent.keyDown(getByRole('tab', { name: 'Alpha' }), { key: 'ArrowLeft' });
    expect(getByRole('tab', { selected: true })).toHaveTextContent('Gamma');

    await fireEvent.keyDown(getByRole('tab', { name: 'Gamma' }), { key: 'ArrowRight' });
    expect(getByRole('tab', { selected: true })).toHaveTextContent('Alpha');
  });

  it('Home/End jump to the first/last tab', async () => {
    const { getByRole } = render(Tabs, { tabs, activeTab: 'b', idPrefix: 'x', label: 'Sections' });

    await fireEvent.keyDown(getByRole('tab', { name: 'Beta' }), { key: 'End' });
    expect(getByRole('tab', { selected: true })).toHaveTextContent('Gamma');

    await fireEvent.keyDown(getByRole('tab', { name: 'Gamma' }), { key: 'Home' });
    expect(getByRole('tab', { selected: true })).toHaveTextContent('Alpha');
  });

  it('moves DOM focus to the newly selected tab on arrow-key navigation', async () => {
    const { getByRole } = render(Tabs, { tabs, activeTab: 'a', idPrefix: 'x', label: 'Sections' });

    await fireEvent.keyDown(getByRole('tab', { name: 'Alpha' }), { key: 'ArrowRight' });

    expect(document.activeElement).toBe(getByRole('tab', { name: 'Beta' }));
  });

  it('an unrelated key is ignored', async () => {
    const { getByRole } = render(Tabs, { tabs, activeTab: 'a', idPrefix: 'x', label: 'Sections' });

    await fireEvent.keyDown(getByRole('tab', { name: 'Alpha' }), { key: 'Enter' });

    expect(getByRole('tab', { selected: true })).toHaveTextContent('Alpha');
  });

  it('wires aria-controls to the id scheme consumers must match for their panels', () => {
    const { getByRole } = render(Tabs, { tabs, activeTab: 'a', idPrefix: 'dashboard', label: 'Sections' });

    expect(getByRole('tab', { name: 'Alpha' })).toHaveAttribute('aria-controls', 'dashboard-panel-a');
    expect(getByRole('tab', { name: 'Alpha' })).toHaveAttribute('id', 'dashboard-tab-a');
  });
});
