// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor } from '@testing-library/svelte';

// autosave.register sets up a timer; stub it so the mounted editor doesn't
// leave a live interval running through the test.
vi.mock('../../autosave.svelte', () => ({
  autosave: { register: () => () => {} },
}));

import CharterEditor from './CharterEditor.svelte';
import { session } from '../../session.svelte';

let app: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  session.editingId = 'doc-1';
  app = {
    GetDocument: vi.fn(async () => ({
      id: 'doc-1',
      project_id: 'p1',
      kind: 'charter',
      title: 'Project Charter',
      content: '{}',
      template_id: '',
      version: 1,
      status: 'draft',
      created_at: '',
      updated_at: '',
    })),
    ListDocumentKinds: vi.fn(async () => [
      { kind: 'charter', name: 'Charter', phase: 'initiate', description: '', fields: [] },
    ]),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
});

afterEach(() => {
  vi.restoreAllMocks();
  session.editingId = null;
});

// Added 2026-08-19 alongside migrating the header "&larr; Dashboard" nav
// link (previously a raw <button>) to the `nav` Button variant — see
// Button.svelte's `nav` branch comment (tier-3 deferred file, closed this
// cycle). Independently fault-seeded (see TEST_COVERAGE_LEDGER.md) —
// covers the autosave-registering mount path, the most complex of this
// cycle's 7 files alongside SigmaProjectView.
describe('CharterEditor migrated header "&larr; Dashboard" button', () => {
  it('Button variant="nav"', async () => {
    const { getByText } = render(CharterEditor);
    await waitFor(() => expect(app.GetDocument).toHaveBeenCalled());

    const btn = getByText('← Dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'text-xs text-slate-400 hover:text-cyan-400 disabled:opacity-50'.split(/\s+/).sort(),
    );
  });
});
