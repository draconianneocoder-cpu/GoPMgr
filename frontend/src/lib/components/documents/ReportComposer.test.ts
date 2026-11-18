// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor, fireEvent } from '@testing-library/svelte';

import ReportComposer from './ReportComposer.svelte';

const desktopApiMock = vi.hoisted(() => {
  const state: { current: Record<string, ReturnType<typeof vi.fn>> } = { current: {} };
  return state;
});
vi.mock('../../desktop-api', () => ({ get desktopApi() { return desktopApiMock.current; } }));

let app: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  app = {
    ListDocuments: vi.fn(async () => []),
    ListReportProfiles: vi.fn(async () => []),
  };
  desktopApiMock.current = app;
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

describe('ReportComposer schedule audits', () => {
  it('exports an audit section with no document ID and keeps it out of signed exports', async () => {
    app.ListCharts = vi.fn(async () => [{ id: 'chart-1', title: 'Master schedule', kind: 'cpm' }]);
    app.ListDocuments = vi.fn(async () => [{ id: 'doc-1', title: 'Charter', kind: 'charter_word', version: 1, status: 'draft' }]);
    app.ExportCombinedReportWithOptions = vi.fn(async () => '/tmp/report.pdf');
    const { getByRole, findByRole, getByLabelText, getByText } = render(ReportComposer);

    await fireEvent.click(await findByRole('button', { name: 'Add a schedule audit of Master schedule' }));
    await fireEvent.change(getByLabelText('Thresholds'), { target: { value: 'dcma-style' } });
    await fireEvent.input(getByLabelText('Status date'), { target: { value: '2026-06-15' } });
    expect((getByRole('button', { name: 'Signature Options' }) as HTMLButtonElement).disabled).toBe(true);
    expect(getByText('Signed reports cannot include a schedule audit yet. Export this report as an unsigned PDF, or remove the audit to sign it.')).toBeTruthy();

    await fireEvent.click(getByRole('button', { name: 'Export PDF' }));
    await waitFor(() => expect(app.ExportCombinedReportWithOptions).toHaveBeenCalled());
    const sections = app.ExportCombinedReportWithOptions.mock.calls[0][2];
    expect(sections).toEqual([{
      document_id: '',
      title: 'Schedule audit: Master schedule',
      description: '',
      schedule_audit: { chart_id: 'chart-1', reporting_date: '2026-06-15', profile: 'dcma-style' },
    }]);
  });
});
