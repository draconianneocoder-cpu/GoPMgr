// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';

import ScheduleAuditPanel from './ScheduleAuditPanel.svelte';
import { en } from '../../i18n/messages/en';

const desktopApiMock = vi.hoisted(() => ({ current: {} as Record<string, ReturnType<typeof vi.fn>> }));
vi.mock('../../desktop-api', () => ({ get desktopApi() { return desktopApiMock.current; } }));

// A result as Go sends it: an empty list arrives as null.
const result = {
  profile: 'GoPMgr default',
  source: { chart_id: 'chart-1', revision: '2026-06-15T10:00:00Z' },
  engine_version: 'schedaudit/1',
  reporting_date: '2026-06-15',
  findings: [
    { rule: 'SQ-09', profile: 'GoPMgr default', severity: 'error', task_id: 'B', task_title: 'Build', observed: 'MFO 2026-06-03 cannot be met', explanation: 'The constraint cannot be met.', advice: 'Move the date.', source: { chart_id: 'chart-1', revision: 'r' }, engine_version: 'schedaudit/1' },
    { rule: 'SQ-17', profile: 'GoPMgr default', severity: 'info', task_id: 'GONE', task_title: 'Removed', observed: 'no assignment', explanation: '', advice: '', source: { chart_id: 'chart-1', revision: 'r' }, engine_version: 'schedaudit/1' },
  ],
  metrics: null,
  rules: [
    { rule: 'SQ-09', evaluated: true },
    { rule: 'SQ-15', evaluated: false, reason: 'no status date' },
  ],
};

function mount(saveIfDirty: () => Promise<boolean>) {
  const AuditSchedule = vi.fn(async () => result);
  desktopApiMock.current = { AuditSchedule };
  const selectTask = vi.fn((id: string) => id === 'B');
  const view = render(ScheduleAuditPanel, {
    currentChartId: () => 'chart-1',
    saveIfDirty,
    selectTask,
    taskExists: (id: string) => id === 'B',
  });
  return { ...view, AuditSchedule, selectTask };
}

describe('ScheduleAuditPanel', () => {
  it('does not audit when unrecorded edits could not be saved', async () => {
    const { getByRole, AuditSchedule } = mount(async () => false);
    await fireEvent.click(getByRole('button', { name: 'Run audit' }));
    await waitFor(() => expect(getByRole('alert').textContent).toBe(en['audit.saveFailed']));
    expect(AuditSchedule).not.toHaveBeenCalled();
  });

  it('saves first, then audits the named chart with the chosen profile and status date', async () => {
    const order: string[] = [];
    const { getByRole, getByLabelText, AuditSchedule } = mount(async () => {
      order.push('save');
      return true;
    });
    AuditSchedule.mockImplementation(async () => {
      order.push('audit');
      return result;
    });
    await fireEvent.change(getByLabelText('Thresholds'), { target: { value: 'dcma-style' } });
    await fireEvent.input(getByLabelText('Status date'), { target: { value: '2026-06-15' } });
    await fireEvent.click(getByRole('button', { name: 'Run audit' }));
    await waitFor(() => expect(AuditSchedule).toHaveBeenCalledWith('chart-1', '2026-06-15', 'dcma-style'));
    expect(order).toEqual(['save', 'audit']);
  });

  it('writes severity as text, announces the summary and lists rules that did not run', async () => {
    const { getByRole, getByText, findByText } = mount(async () => true);
    await fireEvent.click(getByRole('button', { name: 'Run audit' }));
    await findByText('Error');
    expect(getByText('Note')).toBeTruthy();
    expect(getByRole('status').textContent).toBe('Audit finished: 1 error, 0 warnings, 1 note.');
    expect(getByText('SQ-15 Missing progress evidence: no status date')).toBeTruthy();
    expect(getByText('Audited revision 2026-06-15T10:00:00Z. Run the audit again after editing.')).toBeTruthy();
  });

  it('selects a finding’s task, announces it, and says when the task is gone', async () => {
    const { getByRole, getByText, findByRole, selectTask } = mount(async () => true);
    await fireEvent.click(getByRole('button', { name: 'Run audit' }));
    const show = await findByRole('button', { name: 'Show Build on the chart' });
    await fireEvent.click(show);
    expect(selectTask).toHaveBeenCalledWith('B');
    await waitFor(() => expect(getByRole('status').textContent).toBe('Selected Build (B).'));
    expect(getByText(en['audit.taskMissing'])).toBeTruthy();
  });

  it('replaces an earlier result, so a failed re-run shows no stale findings', async () => {
    const { getByRole, findByText, queryByText, AuditSchedule } = mount(async () => true);
    await fireEvent.click(getByRole('button', { name: 'Run audit' }));
    await findByText('SQ-15 Missing progress evidence: no status date');
    AuditSchedule.mockRejectedValueOnce(new Error('chart has no tasks'));
    await fireEvent.click(getByRole('button', { name: 'Run audit' }));
    await waitFor(() => expect(getByRole('alert').textContent).toBe('The audit could not run: chart has no tasks'));
    expect(queryByText('SQ-15 Missing progress evidence: no status date')).toBeNull();
  });

  it('never presents skipped rules as a clean schedule', async () => {
    const { getByRole, findByText, AuditSchedule } = mount(async () => true);
    AuditSchedule.mockResolvedValue({ ...result, findings: null });
    await fireEvent.click(getByRole('button', { name: 'Run audit' }));
    await findByText('No findings from the rules that ran, but 1 rule could not run. See below.');
  });

  it('names every rule of the specification in the catalog', () => {
    // The 18 rules of docs/testing/PS-02-schedule-quality-audit-spec.md §3.
    for (let n = 1; n <= 18; n++) {
      const key = `audit.rule.SQ-${String(n).padStart(2, '0')}`;
      expect(en, key).toHaveProperty([key]);
    }
  });
});
