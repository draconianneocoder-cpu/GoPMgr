// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, describe, expect, it } from 'vitest';

import {
  applyTheme,
  readCachedTheme,
  rememberTheme,
  THEME_STORAGE_KEY,
} from './theme';

afterEach(() => {
  document.documentElement.dataset.theme = 'dark';
  document.querySelector('meta[name="color-scheme"]')?.setAttribute('content', 'dark');
  window.localStorage.clear();
});

describe('application theme state', () => {
  it('applies the light theme to the document and browser chrome', () => {
    applyTheme('light');

    expect(document.documentElement.dataset.theme).toBe('light');
    expect(document.querySelector('meta[name="color-scheme"]')).toHaveAttribute('content', 'light');
  });

  it('falls back to dark for an unsupported theme', () => {
    applyTheme('sepia');

    expect(document.documentElement.dataset.theme).toBe('dark');
  });

  it('round-trips a remembered theme', () => {
    rememberTheme('light');

    expect(readCachedTheme()).toBe('light');
  });

  it('ignores an invalid cached theme', () => {
    window.localStorage.setItem(THEME_STORAGE_KEY, 'sepia');

    expect(readCachedTheme()).toBeNull();
  });

  it('keeps remembered themes isolated by account', () => {
    rememberTheme('light', 'alice');
    rememberTheme('dark', 'bob');

    expect([readCachedTheme('alice'), readCachedTheme('bob')]).toEqual(['light', 'dark']);
  });

  it('degrades safely when a restricted WebView denies storage access', () => {
    const storageDescriptor = Object.getOwnPropertyDescriptor(window, 'localStorage');
    if (!storageDescriptor) throw new Error('jsdom localStorage descriptor is missing');

    Object.defineProperty(window, 'localStorage', {
      configurable: true,
      get: () => {
        throw new DOMException('Storage is unavailable', 'SecurityError');
      },
    });
    try {
      expect(() => rememberTheme('light')).not.toThrow();
      expect(readCachedTheme()).toBeNull();
    } finally {
      // Restore jsdom before the shared afterEach hook clears storage.
      Object.defineProperty(window, 'localStorage', storageDescriptor);
    }
  });
});
