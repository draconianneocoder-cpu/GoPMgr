// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Font stacks for code that sets fonts outside CSS, such as canvas charts.
// They must match the --font-sans and --font-mono tokens in src/app.css;
// typography.test.ts checks that. See docs/design/typography-ibm-plex.md.

// The script families follow IBM Plex Sans; their faces load only for their
// own scripts (unicode-range in app.css).
export const FONT_SANS =
  '"IBM Plex Sans", "IBM Plex Sans Arabic", "IBM Plex Sans Hebrew", "IBM Plex Sans Devanagari", "IBM Plex Sans Thai", ui-sans-serif, system-ui, sans-serif, "Apple Color Emoji", "Segoe UI Emoji", "Noto Color Emoji"';

export const FONT_MONO = '"IBM Plex Mono", ui-monospace, monospace';
