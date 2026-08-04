// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, waitFor } from '@testing-library/svelte';

import AppSettings from './AppSettings.svelte';
import { THEME_STORAGE_KEY } from '../theme';

const appInfo = {
  version: '1.1.0',
  username: 'tester',
  data_location: '/tmp/gopmgr',
  logs_dir: '/tmp/gopmgr/logs',
  fonts: [],
  settings: {
    default_font: '',
    default_theme: '',
    app_theme: 'dark',
    auto_save_seconds: 60,
  },
};

let app: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  app = {
    GetAppInfo: vi.fn(async () => appInfo),
    HasAnyAdmin: vi.fn(async () => true),
    SaveAppSettings: vi.fn(async () => undefined),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
  window.localStorage.clear();
  document.documentElement.dataset.theme = 'dark';
});

afterEach(() => {
  window.localStorage.clear();
  document.documentElement.dataset.theme = 'dark';
});

describe('application appearance settings', () => {
  it('previews a selected appearance immediately', async () => {
    const { findByRole } = render(AppSettings);
    const lightChoice = await findByRole('radio', { name: /light fieldbook/i });

    await fireEvent.click(lightChoice);

    expect(document.documentElement.dataset.theme).toBe('light');
  });

  it('remembers the appearance only after save succeeds', async () => {
    const { findByRole, getByRole } = render(AppSettings);
    await fireEvent.click(await findByRole('radio', { name: /light fieldbook/i }));
    expect(window.localStorage.getItem(THEME_STORAGE_KEY)).toBeNull();

    await fireEvent.click(getByRole('button', { name: /save settings/i }));

    await waitFor(() => expect(app.SaveAppSettings).toHaveBeenCalled());
    expect(window.localStorage.getItem(THEME_STORAGE_KEY)).toBe('light');
  });

  it('restores the committed appearance when an unsaved preview is closed', async () => {
    const { findByRole, unmount } = render(AppSettings);
    await fireEvent.click(await findByRole('radio', { name: /light fieldbook/i }));

    unmount();

    expect(document.documentElement.dataset.theme).toBe('dark');
  });
});
