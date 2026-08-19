// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor } from '@testing-library/svelte';

import SigmaProjectView from './SigmaProjectView.svelte';
import { session } from '../../session.svelte';

let app: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  session.editingId = 'sig-1';
  app = {
    SigmaGetProject: vi.fn(async () => ({
      id: 'sig-1',
      gopmgr_project_id: 'p1',
      title: 'Reduce Defect Rate',
      description: '',
      belt_level: 'green',
      phase: 'define',
      status: 'active',
      sponsor: '',
      process_owner: '',
      belt_lead: '',
      created_at: '',
      updated_at: '',
    })),
    SigmaGetCharter: vi.fn(async () => ({
      id: 'ch-1',
      project_id: 'sig-1',
      problem_statement: '',
      business_case: '',
      goal_statement: '',
      scope_in: [],
      scope_out: [],
      ctqs: [],
      sponsor: '',
      updated_at: '',
    })),
    SigmaGetFishbone: vi.fn(async () => ({ problem_statement: '', branches: [] })),
    SigmaGetToolStatus: vi.fn(async () => ({ phase: 'define', tools: [] })),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
});

afterEach(() => {
  vi.restoreAllMocks();
  session.editingId = null;
});

// Added 2026-08-19 alongside migrating the header "&larr; Sigma Home" nav
// link (previously a raw <button>) to the `nav` Button variant — see
// Button.svelte's `nav` branch comment (tier-3 deferred file, closed this
// cycle). Independently fault-seeded (see TEST_COVERAGE_LEDGER.md) — this
// is the heaviest mount of this cycle's 7 files (4 sequential Wails calls
// in loadProject/loadToolStatus). The 4 load mocks are all asserted called
// so a silently-swallowed load failure (caught internally and routed to
// showToast, not thrown) would surface as a timeout here rather than a
// false pass. Note: this file also has an unrelated "Cancel" button
// (`bg-slate-800 hover:bg-slate-700 px-4 py-2 rounded`, no
// text-slate-400/hover:cyan) inside the charter form — confirmed by grep
// not to match the nav pattern, correctly out of scope for this migration.
describe('SigmaProjectView migrated header "&larr; Sigma Home" button', () => {
  it('Button variant="nav"', async () => {
    const { getByText } = render(SigmaProjectView);
    await waitFor(() => expect(app.SigmaGetProject).toHaveBeenCalled());
    await waitFor(() => expect(app.SigmaGetCharter).toHaveBeenCalled());
    await waitFor(() => expect(app.SigmaGetFishbone).toHaveBeenCalled());
    await waitFor(() => expect(app.SigmaGetToolStatus).toHaveBeenCalled());

    const btn = getByText('← Sigma Home');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'text-xs text-slate-400 hover:text-cyan-400 disabled:opacity-50'.split(/\s+/).sort(),
    );
  });
});
