// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import tsPlugin from '@typescript-eslint/eslint-plugin';
import tsParser from '@typescript-eslint/parser';
import sveltePlugin from 'eslint-plugin-svelte';

export default [
  {
    ignores: ['dist/**', 'wailsjs/**']
  },
  ...sveltePlugin.configs['flat/recommended'],
  {
    files: ['**/*.svelte'],
    languageOptions: {
      parserOptions: {
        parser: tsParser
      }
    },
    rules: {
      // These rules became recommended in eslint-plugin-svelte 3. Keep the
      // established PMForge policy during the toolchain migration; enable and
      // remediate them in focused follow-up changes instead of rewriting many
      // unrelated components as part of a dependency bump.
      'svelte/no-useless-mustaches': 'off',
      'svelte/prefer-svelte-reactivity': 'off',
      'svelte/require-each-key': 'off'
    }
  },
  {
    // Keep this after the Svelte preset because v3 also matches .svelte.ts
    // modules; PMForge's rune helper modules are ordinary TypeScript and need
    // the TypeScript parser rather than Espree.
    files: ['**/*.ts'],
    languageOptions: {
      parser: tsParser,
      ecmaVersion: 2022,
      sourceType: 'module'
    },
    plugins: {
      '@typescript-eslint': tsPlugin
    },
    rules: {
      ...tsPlugin.configs.recommended.rules,
      'svelte/prefer-svelte-reactivity': 'off'
    }
  }
];
