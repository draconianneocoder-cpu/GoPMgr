// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{svelte,ts}'],
  theme: {
    extend: {
      colors: {
        // PMForge palette — slate base + cyan accent (matches GanttChart).
        accent: {
          DEFAULT: '#00D4FF',
          dim: '#0891b2',
        },
        critical: '#ef4444',
      },
    },
  },
  plugins: [],
};
