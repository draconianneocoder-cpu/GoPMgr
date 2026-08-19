// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor } from '@testing-library/svelte';

import ReportComposer from './ReportComposer.svelte';

let app: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  app = {
    ListDocuments: vi.fn(async () => []),
    ListReportProfiles: vi.fn(async () => []),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
});

afterEach(() => {
  vi.restoreAllMocks();
});

// Added 2026-08-19 alongside migrating the header "&larr; Dashboard" nav
// link (previously a raw <button>) to the `nav` Button variant — see
// Button.svelte's `nav` branch comment (tier-3 deferred file, closed this
// cycle). This file previously had zero test coverage entirely, not just
// for this button. Not independently fault-seeded on this file — relies
// on Button.test.ts's single shared seed; this file's own query resolving
// is proven by the assertion passing rather than throwing.
describe('ReportComposer migrated header "&larr; Dashboard" button', () => {
  it('Button variant="nav"', async () => {
    const { getByText } = render(ReportComposer);
    await waitFor(() => expect(app.ListDocuments).toHaveBeenCalled());

    const btn = getByText('← Dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'text-xs text-slate-400 hover:text-cyan-400 disabled:opacity-50'.split(/\s+/).sort(),
    );
  });
});
