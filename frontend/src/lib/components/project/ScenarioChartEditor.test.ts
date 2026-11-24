// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';

import ScenarioChartEditor from './ScenarioChartEditor.svelte';
import { autosave } from '../../autosave.svelte';
import {
  cancelNavigation,
  navigation,
  requestNavigation,
  saveAndContinueNavigation,
  session,
} from '../../session.svelte';

const desktopApiMock = vi.hoisted(() => {
  const state: { current: Record<string, ReturnType<typeof vi.fn>> } = { current: {} };
  return state;
});
vi.mock('../../desktop-api', () => ({ get desktopApi() { return desktopApiMock.current; } }));

let app: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  session.editingId = 'sc-1';
  app = {
    GetScenarioChart: vi.fn(async () => ({
      id: 'sc-1',
      scenario_id: 'scn-1',
      project_id: 'p1',
      kind: 'what_if',
      source_chart_id: 'chart-1',
      source_baseline_id: '',
      title: 'Scenario A',
      data: '{}',
      config: '{}',
      baseline_data: '',
      created_at: '',
      updated_at: '',
    })),
  };
  desktopApiMock.current = app;
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  autosave.discardAll();
  cancelNavigation();
  navigation.saving = false;
  navigation.error = '';
  session.editingId = null;
});

// Added 2026-08-19 alongside migrating the header "&larr; Project Settings"
// nav link (previously a raw <button>) to the `nav` Button variant — see
// Button.svelte's `nav` branch comment (tier-3 deferred file, closed this
// cycle). Not independently fault-seeded on this file — relies on
// Button.test.ts's single shared seed; this file's own query resolving is
// proven by the assertion passing rather than throwing.
describe('ScenarioChartEditor migrated header "&larr; Project Settings" button', () => {
  it('Button variant="nav"', async () => {
    const { getByText } = render(ScenarioChartEditor);
    await waitFor(() => expect(app.GetScenarioChart).toHaveBeenCalled());

    const btn = getByText('← Project Settings');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'text-xs text-slate-400 hover:text-cyan-400 disabled:opacity-50'.split(/\s+/).sort(),
    );
  });
});

describe('ScenarioChartEditor scenario context', () => {
  it('shows the scenario the chart belongs to', async () => {
    app.GetScenario = vi.fn(async () => ({
      id: 'scn-1', project_id: 'p1', name: 'Late vendor', source_baseline_id: '',
      description: 'Supplier slips two weeks', is_active: false, created_at: '', updated_at: '',
    }));
    const utils = render(ScenarioChartEditor);
    expect(await utils.findByText('Late vendor')).toBeInTheDocument();
    expect(utils.getByText('Supplier slips two weeks')).toBeInTheDocument();
    expect(app.GetScenario).toHaveBeenCalledWith('scn-1');
  });

  it('still opens the chart when the scenario cannot be read', async () => {
    app.GetScenario = vi.fn(async () => {
      throw 'db: scenario not found';
    });
    const utils = render(ScenarioChartEditor);
    await waitFor(() => expect(app.GetScenario).toHaveBeenCalled());
    expect(await utils.findByText('Unavailable')).toBeInTheDocument();
    for (const save of utils.getAllByRole('button', { name: /Save scenario edits/i })) expect(save).toBeEnabled();
  });
});

// The editor's edits are protected by the shared unsaved-changes guard
// (P0 "Prevent silent editor data loss"): leaving asks first, and a save
// clears it even though the editor pretty-prints what was saved.
describe('ScenarioChartEditor unsaved changes', () => {
  async function openEditor() {
    app.SaveScenarioChart = vi.fn(async (chart: ScenarioChart) => chart);
    const utils = render(ScenarioChartEditor);
    const dataBox = (await utils.findByLabelText(/Scenario chart data JSON/i)) as HTMLTextAreaElement;
    session.view = 'scenario_chart';
    return { utils, dataBox };
  }

  it('asks before leaving with an unsaved edit, then saves it', async () => {
    const { dataBox } = await openEditor();
    await fireEvent.input(dataBox, { target: { value: '{"tasks":[1]}' } });

    requestNavigation('project_settings');
    expect(navigation.pending?.view).toBe('project_settings');
    expect(session.view).toBe('scenario_chart');

    await saveAndContinueNavigation();
    expect(app.SaveScenarioChart).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({ id: 'sc-1', data: '{"tasks":[1]}' }),
    );
    expect(session.view).toBe('project_settings');
  });

  it('has nothing unsaved after a manual save that reformats the JSON and trims the title', async () => {
    const { utils, dataBox } = await openEditor();
    await fireEvent.input(dataBox, { target: { value: '{"tasks":[1]}' } });
    await fireEvent.input(utils.getByLabelText(/Scenario chart title/i), { target: { value: '  Scenario B  ' } });
    expect(autosave.hasDirty()).toBe(true);

    await fireEvent.click(utils.getAllByRole('button', { name: /Save scenario edits/i })[0]);
    await utils.findByText('Saved.');
    expect(app.SaveScenarioChart).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({ title: 'Scenario B', data: '{"tasks":[1]}' }),
    );
    expect(dataBox.value).toBe('{\n  "tasks": [\n    1\n  ]\n}');
    expect(autosave.hasDirty()).toBe(false);

    requestNavigation('project_settings');
    expect(navigation.pending).toBeNull();
    await waitFor(() => expect(session.view).toBe('project_settings'));
  });

  it('keeps the draft and says why when leaving with invalid JSON', async () => {
    const { dataBox } = await openEditor();
    await fireEvent.input(dataBox, { target: { value: '{"tasks":' } });

    requestNavigation('project_settings');
    await saveAndContinueNavigation();
    expect(navigation.error).toMatch(/^Scenario chart data is not valid JSON/);
    expect(navigation.pending?.view).toBe('project_settings');
    expect(session.view).toBe('scenario_chart');
    expect(app.SaveScenarioChart).not.toHaveBeenCalled();
    expect(dataBox.value).toBe('{"tasks":');
    expect(autosave.hasDirty()).toBe(true);
  });
});
