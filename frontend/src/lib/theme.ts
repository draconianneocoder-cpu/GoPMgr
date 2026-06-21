// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// applyTheme sets the application UI theme by toggling the `data-theme`
// attribute on <html>. The CSS variables in app.css key off this attribute,
// so the whole app (slate structure + cyan accent) flips between light and
// dark. Anything other than 'light' is treated as the default dark theme.
export function applyTheme(theme: string | null | undefined): void {
  document.documentElement.dataset.theme = theme === 'light' ? 'light' : 'dark';
}
