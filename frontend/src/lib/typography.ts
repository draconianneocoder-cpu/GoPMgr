// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Font stacks for code that sets fonts outside CSS, such as canvas charts.
// They must match the --font-sans and --font-mono tokens in src/app.css;
// typography.test.ts checks that. See docs/design/typography-ibm-plex.md.

export const FONT_SANS =
  '"IBM Plex Sans", ui-sans-serif, system-ui, sans-serif, "Apple Color Emoji", "Segoe UI Emoji", "Noto Color Emoji"';

export const FONT_MONO = '"IBM Plex Mono", ui-monospace, monospace';
