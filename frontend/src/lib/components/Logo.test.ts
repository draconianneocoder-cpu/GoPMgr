// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { render, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it } from 'vitest';
import Logo from './Logo.svelte';

afterEach(() => {
  document.documentElement.dataset.theme = 'dark';
});

describe('Logo', () => {
  it('uses the matching full lockup when the application theme changes', async () => {
    document.documentElement.dataset.theme = 'dark';
    const { getByRole } = render(Logo);
    const logo = getByRole('img', { name: 'GoPMgr, featuring Bobby Beaver' });

    expect(logo).toHaveAttribute('src', '/branding/gopmgr-logo-lockup-dark.png');

    document.documentElement.dataset.theme = 'light';
    await waitFor(() => expect(logo).toHaveAttribute('src', '/branding/gopmgr-logo-lockup-light.png'));
  });

  it('uses a square themed mark in compact contexts', async () => {
    document.documentElement.dataset.theme = 'light';
    const { getByRole } = render(Logo, { props: { variant: 'compact', class: 'h-8 w-8' } });
    const logo = getByRole('img', { name: 'GoPMgr, featuring Bobby Beaver' });

    await waitFor(() => expect(logo).toHaveAttribute('src', '/branding/gopmgr-app-icon-light.png'));
    expect(logo).toHaveAttribute(
      'srcset',
      '/branding/gopmgr-app-icon-light-128.png 128w, /branding/gopmgr-app-icon-light.png 512w',
    );
    expect(logo).toHaveAttribute('sizes', '32px');
    expect(logo).toHaveClass('h-8', 'w-8');
  });
});
