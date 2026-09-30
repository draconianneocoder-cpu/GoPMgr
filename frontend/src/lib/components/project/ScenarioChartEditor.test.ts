// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { cleanup, render, waitFor } from '@testing-library/svelte';

import ScenarioChartEditor from './ScenarioChartEditor.svelte';
import { session } from '../../session.svelte';

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
  (window as unknown as { go: unknown }).go = { main: { App: app } };
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
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
