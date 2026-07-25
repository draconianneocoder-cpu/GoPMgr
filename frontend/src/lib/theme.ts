// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

export type AppTheme = 'dark' | 'light';

export const THEME_STORAGE_KEY = 'pmforge.app-theme';

function normaliseTheme(theme: string | null | undefined): AppTheme {
  return theme === 'light' ? 'light' : 'dark';
}

// App settings are per user, so the flash-prevention cache must be scoped the
// same way. The base key remains the fallback for tests and legacy callers.
function storageKey(username?: string | null): string {
  const account = username?.trim();
  return account ? `${THEME_STORAGE_KEY}:${encodeURIComponent(account)}` : THEME_STORAGE_KEY;
}

// applyTheme updates both the CSS theme and the browser/native color-scheme
// hint. Keeping these together prevents form controls and window chrome from
// disagreeing with the application surface.
export function applyTheme(theme: string | null | undefined): AppTheme {
  const resolved = normaliseTheme(theme);
  if (typeof document === 'undefined') return resolved;

  document.documentElement.dataset.theme = resolved;
  document.documentElement.style.colorScheme = resolved;

  let meta = document.querySelector<HTMLMetaElement>('meta[name="color-scheme"]');
  if (!meta) {
    meta = document.createElement('meta');
    meta.name = 'color-scheme';
    document.head.append(meta);
  }
  meta.content = resolved;
  return resolved;
}

// rememberTheme caches only the visual preference. The backend remains the
// source of truth; this local value simply avoids a dark frame while the
// signed-in user's settings are loading.
export function rememberTheme(theme: string | null | undefined, username?: string | null): void {
  if (typeof localStorage === 'undefined') return;
  try {
    localStorage.setItem(storageKey(username), normaliseTheme(theme));
  } catch {
    // A restricted webview can deny storage. Theme application still works.
  }
}

export function readCachedTheme(username?: string | null): AppTheme | null {
  if (typeof localStorage === 'undefined') return null;
  try {
    const cached = localStorage.getItem(storageKey(username));
    return cached === 'dark' || cached === 'light' ? cached : null;
  } catch {
    return null;
  }
}
