// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { svelteTesting } from '@testing-library/svelte/vite';

// Vitest config for component + unit tests. Reuses the Svelte plugin so
// .svelte components compile the same way the app build does; svelteTesting
// wires up auto-cleanup and the browser resolve condition. jsdom gives the
// DOM that @testing-library/svelte mounts into.
export default defineConfig({
  plugins: [svelte(), svelteTesting()],
  test: {
    environment: 'jsdom',
    environmentOptions: {
      jsdom: {
        url: 'http://localhost/',
      },
    },
    globals: true,
    // Node 26 enables its process-wide Web Storage global by default. Disable
    // it in Vitest workers so jsdom remains the sole browser storage provider;
    // otherwise Node's unconfigured localStorage shadows jsdom and throws.
    execArgv: ['--no-experimental-webstorage'],
    setupFiles: ['./vitest.setup.ts'],
    include: ['src/**/*.{test,spec}.{ts,js}'],
  },
});
