// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';

import CPMEditor from './CPMEditor.svelte';
import { session } from '../../session.svelte';

// The real auto-save module, unlike CPMEditor.test.ts: this file checks
// editor actions that save through the shell's registered snapshot: the
// schedule audit saves exactly the edits auto-save has not recorded, and
// clearing a levelled start saves and reloads.
const desktopApiMock = vi.hoisted(() => ({ current: {} as Record<string, ReturnType<typeof vi.fn>> }));
vi.mock('../../desktop-api', () => ({ get desktopApi() { return desktopApiMock.current; } }));

const emptyResult = {
  profile: 'GoPMgr default',
  source: { chart_id: 'chart-1', revision: 'r' },
  engine_version: 'schedaudit/1',
  findings: null,
  metrics: null,
  rules: null,
};

function installApp() {
  const chart = { id: 'chart-1', project_id: 'p1', kind: 'cpm', title: 'C', data: JSON.stringify({ nodes: [], edges: [] }) };
  const app = {
    GetChart: vi.fn(async () => chart),
    SaveChart: vi.fn(async (c: unknown) => c),
    LayoutChart: vi.fn(async () => ({ body: { layout: { nodes: [], edges: [], width: 0, height: 0 } } })),
    ListScheduleBaselines: vi.fn(async () => []),
    CompareScheduleBaseline: vi.fn(async () => ({})),
    ListStakeholders: vi.fn(async () => []),
    AuditSchedule: vi.fn(async () => emptyResult),
  };
  desktopApiMock.current = app;
  return app;
}

function settle(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

beforeEach(() => {
  session.editingId = 'chart-1';
});

describe('CPMEditor schedule audit saving', () => {
  it('audits without writing an unchanged chart, and saves an unrecorded edit first', async () => {
    const app = installApp();
    const { getByRole, getByLabelText } = render(CPMEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());
    // The shell registers with auto-save once its first layout refresh has
    // finished; let that settle so the click sees the registered editor.
    await settle(50);

    // Nothing edited: the audit reads the stored chart and writes nothing.
    const savesAtLoad = app.SaveChart.mock.calls.length;
    await fireEvent.click(getByRole('button', { name: 'Run audit' }));
    await waitFor(() => expect(app.AuditSchedule).toHaveBeenCalledTimes(1));
    expect(app.SaveChart).toHaveBeenCalledTimes(savesAtLoad);

    // Add a task, then rename it. Adding refreshes the layout at once and,
    // because it selects the new node, again 300 ms later; wait for both.
    // The rename is saved only by a re-layout 300 ms after it, so the audit,
    // run at once, must save it itself.
    const layoutsBeforeAdd = app.LayoutChart.mock.calls.length;
    await fireEvent.click(getByRole('button', { name: '+ Node' }));
    await waitFor(() => expect(app.LayoutChart.mock.calls.length).toBe(layoutsBeforeAdd + 2), { timeout: 2000 });
    const layoutsBeforeRename = app.LayoutChart.mock.calls.length;
    await fireEvent.input(getByLabelText('Label'), { target: { value: 'Renamed task' } });
    await fireEvent.click(getByRole('button', { name: 'Run audit' }));
    await waitFor(() => expect(app.AuditSchedule).toHaveBeenCalledTimes(2));

    // No re-layout has run since the add, so the save carrying the rename
    // came from the audit, and it happened before the audit.
    expect(app.LayoutChart).toHaveBeenCalledTimes(layoutsBeforeRename);
    const renamedSave = app.SaveChart.mock.calls.findIndex(([c]) => String((c as { data: string }).data).includes('Renamed task'));
    expect(renamedSave).toBeGreaterThanOrEqual(0);
    expect(app.SaveChart.mock.invocationCallOrder[renamedSave]).toBeLessThan(app.AuditSchedule.mock.invocationCallOrder[1]);
  });

  it('does not rewrite a chart the user has just saved by hand', async () => {
    const app = installApp();
    const { getByRole, getByLabelText } = render(CPMEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());
    await settle(50);
    const layoutsBeforeAdd = app.LayoutChart.mock.calls.length;
    await fireEvent.click(getByRole('button', { name: '+ Node' }));
    await waitFor(() => expect(app.LayoutChart.mock.calls.length).toBe(layoutsBeforeAdd + 2), { timeout: 2000 });
    await fireEvent.input(getByLabelText('Label'), { target: { value: 'Saved by hand' } });
    await fireEvent.click(getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(app.SaveChart.mock.calls.some(([c]) => String((c as { data: string }).data).includes('Saved by hand'))).toBe(true));
    const savesAfterManualSave = app.SaveChart.mock.calls.length;

    await fireEvent.click(getByRole('button', { name: 'Run audit' }));
    await waitFor(() => expect(app.AuditSchedule).toHaveBeenCalledTimes(1));
    expect(app.SaveChart).toHaveBeenCalledTimes(savesAfterManualSave);
  });

  it('does not save a chart again when layout has only added derived fields', async () => {
    const app = installApp();
    app.LayoutChart.mockImplementation(async () => {
      const saved = app.SaveChart.mock.calls.at(-1)?.[0] as { data?: string } | undefined;
      const parsed = JSON.parse(saved?.data || '{"nodes":[],"edges":[]}');
      for (const node of parsed.nodes ?? []) node.es = 1;
      return { body: { layout: { nodes: [], edges: [], width: 0, height: 0 }, doc: parsed } };
    });
    const { getByRole } = render(CPMEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());
    await settle(50);

    const layoutsBeforeAdd = app.LayoutChart.mock.calls.length;
    await fireEvent.click(getByRole('button', { name: '+ Node' }));
    await waitFor(() => expect(app.LayoutChart.mock.calls.length).toBe(layoutsBeforeAdd + 2), { timeout: 2000 });
    const savesAfterLayout = app.SaveChart.mock.calls.length;

    await fireEvent.click(getByRole('button', { name: 'Run audit' }));
    await waitFor(() => expect(app.AuditSchedule).toHaveBeenCalledTimes(1));
    expect(app.SaveChart).toHaveBeenCalledTimes(savesAfterLayout);
  });

  it('drops one chart’s findings when the editor switches to another chart', async () => {
    const app = installApp();
    app.AuditSchedule.mockResolvedValue({ ...emptyResult, rules: [{ rule: 'SQ-15', evaluated: false, reason: 'no status date' }] });
    const { getByRole, findByText, queryByText } = render(CPMEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());
    await settle(50);
    await fireEvent.click(getByRole('button', { name: 'Run audit' }));
    await findByText('SQ-15 Missing progress evidence: no status date');

    session.editingId = 'chart-2';
    await waitFor(() => expect(queryByText('SQ-15 Missing progress evidence: no status date')).toBeNull());
  });
});

// Levelling keeps a delay as the task's levelled start (owner decision
// 2026-11-18). The panel shows it, and Clear removes it from the stored
// chart without touching the constraint, then reloads.
describe('CPMEditor levelled start', () => {
  it('shows a levelled start and clears it', async () => {
    const node = { id: 'n1', label: 'Pour', duration: 2, constraint: 'SNLT', constraint_date: '2026-06-04', level_start: '2026-06-03' };
    let stored = JSON.stringify({ nodes: [node], edges: [] });
    const app = installApp();
    app.GetChart = vi.fn(async () => ({ id: 'chart-1', project_id: 'p1', kind: 'cpm', title: 'C', data: stored }));
    app.SaveChart = vi.fn(async (c: unknown) => {
      stored = (c as { data: string }).data;
      return c;
    });
    app.LayoutChart = vi.fn(async () => ({ body: { layout: { nodes: [{ id: 'n1', title: 'Pour', depth: 0, x: 0, y: 0, width: 120, height: 40 }], edges: [], width: 200, height: 80 } } }));
    const { getByRole, findByText, queryByText } = render(CPMEditor);
    await waitFor(() => expect(app.LayoutChart).toHaveBeenCalled());
    await settle(50);
    await fireEvent.click(getByRole('button', { name: 'Pour' }));
    expect(await findByText('2026-06-03')).toBeTruthy();
    const loads = app.GetChart.mock.calls.length;

    await fireEvent.click(getByRole('button', { name: 'Clear the levelled start of Pour' }));
    await waitFor(() => expect(app.GetChart.mock.calls.length).toBeGreaterThan(loads));
    const saved = JSON.parse(stored).nodes[0];
    expect(saved.level_start).toBeUndefined();
    expect(saved.constraint).toBe('SNLT');
    expect(saved.constraint_date).toBe('2026-06-04');
    await waitFor(() => expect(queryByText('Levelled start')).toBeNull());
  });
});
