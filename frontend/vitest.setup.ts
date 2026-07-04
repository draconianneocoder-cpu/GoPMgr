// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Test setup: register jest-dom matchers (toBeInTheDocument, etc.) on
// Vitest's expect. @testing-library/svelte's svelteTesting plugin handles
// per-test DOM cleanup automatically.
import '@testing-library/jest-dom/vitest';
