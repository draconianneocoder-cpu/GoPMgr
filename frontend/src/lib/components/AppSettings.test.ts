// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, waitFor } from '@testing-library/svelte';

import AppSettings from './AppSettings.svelte';
import { THEME_STORAGE_KEY } from '../theme';
import { session } from '../session.svelte';

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
    auto_check_updates: false,
  },
};

const upToDateStatus = {
  configured: true,
  current: '1.1.0',
  update_available: false,
  channel: 'stable',
  platform: 'darwin',
};

let app: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  app = {
    GetAppInfo: vi.fn(async () => appInfo),
    HasAnyAdmin: vi.fn(async () => true),
    SaveAppSettings: vi.fn(async () => undefined),
    CheckLatestVersion: vi.fn(async () => upToDateStatus),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
  window.localStorage.clear();
  document.documentElement.dataset.theme = 'dark';
  session.updateStatus = null;
});

afterEach(() => {
  window.localStorage.clear();
  document.documentElement.dataset.theme = 'dark';
  session.updateStatus = null;
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

describe('automatic update checking and installation', () => {
  it('saves the auto-check-updates preference', async () => {
    const { findByRole, getByRole } = render(AppSettings);
    const toggle = await findByRole('checkbox', { name: /automatically check for updates on launch/i });
    expect(toggle).not.toBeChecked();

    await fireEvent.click(toggle);
    await fireEvent.click(getByRole('button', { name: /save settings/i }));

    await waitFor(() =>
      expect(app.SaveAppSettings).toHaveBeenCalledWith(
        expect.objectContaining({ auto_check_updates: true }),
      ),
    );
  });

  it('shows no download button when the build is already current', async () => {
    const { findByRole, queryByRole } = render(AppSettings);
    await fireEvent.click(await findByRole('button', { name: 'Check for updates' }));

    await waitFor(() => expect(app.CheckLatestVersion).toHaveBeenCalled());
    expect(queryByRole('button', { name: /download & open installer/i })).not.toBeInTheDocument();
  });

  it('downloads, verifies, and opens the installer on macOS with no quit step', async () => {
    app.CheckLatestVersion = vi.fn(async () => ({
      configured: true,
      current: '1.1.0',
      latest: '1.2.0',
      update_available: true,
      channel: 'stable',
      sha256: 'a'.repeat(64),
      platform: 'darwin',
    }));
    app.DownloadAndInstallUpdate = vi.fn(async () => '/Users/tester/Library/GoPMgr/updates/GoPMgr-1.2.0.dmg');

    const { findByRole, findByText, queryByRole } = render(AppSettings);
    await fireEvent.click(await findByRole('button', { name: 'Check for updates' }));
    const installButton = await findByRole('button', { name: /download & open installer/i });

    await fireEvent.click(installButton);

    await waitFor(() => expect(app.DownloadAndInstallUpdate).toHaveBeenCalledTimes(1));
    expect(await findByText(/drag gopmgr to applications/i)).toBeInTheDocument();
    expect(queryByRole('button', { name: /close & install/i })).not.toBeInTheDocument();
  });

  it('asks for confirmation before quitting to finish a Windows install', async () => {
    app.CheckLatestVersion = vi.fn(async () => ({
      configured: true,
      current: '1.1.0',
      latest: '1.2.0',
      update_available: true,
      channel: 'stable',
      sha256: 'a'.repeat(64),
      platform: 'windows',
    }));
    app.DownloadAndInstallUpdate = vi.fn(async () => 'C:\\Users\\tester\\AppData\\GoPMgr\\updates\\GoPMgr-1.2.0.exe');
    app.QuitToInstallUpdate = vi.fn(async () => undefined);

    const { findByRole } = render(AppSettings);
    await fireEvent.click(await findByRole('button', { name: 'Check for updates' }));
    await fireEvent.click(await findByRole('button', { name: /download & open installer/i }));

    const quitButton = await findByRole('button', { name: /close & install/i });
    expect(app.QuitToInstallUpdate).not.toHaveBeenCalled();

    await fireEvent.click(quitButton);
    await waitFor(() => expect(app.QuitToInstallUpdate).toHaveBeenCalledTimes(1));
  });

  it('shows a clear error and no quit prompt when the download fails verification', async () => {
    app.CheckLatestVersion = vi.fn(async () => ({
      configured: true,
      current: '1.1.0',
      latest: '1.2.0',
      update_available: true,
      channel: 'stable',
      sha256: 'a'.repeat(64),
      platform: 'windows',
    }));
    app.DownloadAndInstallUpdate = vi.fn(async () => {
      throw new Error('update: downloaded artifact checksum mismatch: got b, want a');
    });

    const { findByRole, findByText, queryByRole } = render(AppSettings);
    await fireEvent.click(await findByRole('button', { name: 'Check for updates' }));
    await fireEvent.click(await findByRole('button', { name: /download & open installer/i }));

    expect(await findByText(/checksum mismatch/i)).toBeInTheDocument();
    expect(queryByRole('button', { name: /close & install/i })).not.toBeInTheDocument();
  });

  it('pre-populates from a prior automatic check without re-checking', async () => {
    session.updateStatus = {
      configured: true,
      current: '1.1.0',
      latest: '1.2.0',
      update_available: true,
      channel: 'stable',
      sha256: 'a'.repeat(64),
      platform: 'darwin',
    };

    const { findByText } = render(AppSettings);

    expect(await findByText('Update available: 1.2.0')).toBeInTheDocument();
    expect(app.CheckLatestVersion).not.toHaveBeenCalled();
  });
});
