// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{svelte,ts}'],
  // The app is themeable (light/dark) via a `data-theme` attribute on
  // <html>. The structural palette (slate) and primary accent (cyan) are
  // mapped to CSS variables defined in src/app.css, so every existing
  // `slate-*` / `cyan-*` utility flips with the theme - no per-component
  // edits. Values are RGB channel triplets via `rgb(var(--x) / <alpha-value>)`
  // so Tailwind opacity modifiers (e.g. `bg-slate-800/40`) keep working.
  theme: {
    extend: {
      colors: {
        slate: {
          50: 'rgb(var(--slate-50) / <alpha-value>)',
          100: 'rgb(var(--slate-100) / <alpha-value>)',
          200: 'rgb(var(--slate-200) / <alpha-value>)',
          300: 'rgb(var(--slate-300) / <alpha-value>)',
          400: 'rgb(var(--slate-400) / <alpha-value>)',
          500: 'rgb(var(--slate-500) / <alpha-value>)',
          600: 'rgb(var(--slate-600) / <alpha-value>)',
          700: 'rgb(var(--slate-700) / <alpha-value>)',
          800: 'rgb(var(--slate-800) / <alpha-value>)',
          900: 'rgb(var(--slate-900) / <alpha-value>)',
          950: 'rgb(var(--slate-950) / <alpha-value>)',
        },
        cyan: {
          200: 'rgb(var(--cyan-200) / <alpha-value>)',
          300: 'rgb(var(--cyan-300) / <alpha-value>)',
          400: 'rgb(var(--cyan-400) / <alpha-value>)',
          500: 'rgb(var(--cyan-500) / <alpha-value>)',
          600: 'rgb(var(--cyan-600) / <alpha-value>)',
          700: 'rgb(var(--cyan-700) / <alpha-value>)',
          900: 'rgb(var(--cyan-900) / <alpha-value>)',
          950: 'rgb(var(--cyan-950) / <alpha-value>)',
        },
        // Semantic indicator colors — only the shades actually used by the
        // app are wired to CSS variables so the light/dark theme can remap
        // them without touching every component. Shades 500–800 (action
        // buttons, solid fills) are left as standard Tailwind values because
        // they pair with `text-white` and need no remapping.
        red: {
          100: 'rgb(var(--red-100) / <alpha-value>)',
          200: 'rgb(var(--red-200) / <alpha-value>)',
          300: 'rgb(var(--red-300) / <alpha-value>)',
          400: 'rgb(var(--red-400) / <alpha-value>)',
          900: 'rgb(var(--red-900) / <alpha-value>)',
          950: 'rgb(var(--red-950) / <alpha-value>)',
        },
        emerald: {
          100: 'rgb(var(--emerald-100) / <alpha-value>)',
          200: 'rgb(var(--emerald-200) / <alpha-value>)',
          300: 'rgb(var(--emerald-300) / <alpha-value>)',
          400: 'rgb(var(--emerald-400) / <alpha-value>)',
          900: 'rgb(var(--emerald-900) / <alpha-value>)',
          950: 'rgb(var(--emerald-950) / <alpha-value>)',
        },
        amber: {
          200: 'rgb(var(--amber-200) / <alpha-value>)',
          300: 'rgb(var(--amber-300) / <alpha-value>)',
          400: 'rgb(var(--amber-400) / <alpha-value>)',
          900: 'rgb(var(--amber-900) / <alpha-value>)',
          950: 'rgb(var(--amber-950) / <alpha-value>)',
        },
        orange: {
          300: 'rgb(var(--orange-300) / <alpha-value>)',
          950: 'rgb(var(--orange-950) / <alpha-value>)',
        },
        rose: {
          300: 'rgb(var(--rose-300) / <alpha-value>)',
          950: 'rgb(var(--rose-950) / <alpha-value>)',
        },
        sky: {
          300: 'rgb(var(--sky-300) / <alpha-value>)',
          950: 'rgb(var(--sky-950) / <alpha-value>)',
        },
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
