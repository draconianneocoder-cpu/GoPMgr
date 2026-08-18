// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import { rebaseEditableChanges } from './rebase-editable-changes';

interface Fixture {
  id: string;
  updated_at: string;
  name: string;
  note: string;
  count: number;
}

const BACKEND_OWNED: readonly (keyof Fixture)[] = ['id', 'updated_at'];

describe('rebaseEditableChanges', () => {
  it('takes backend-owned fields from `saved` even when they differ across all three inputs', () => {
    const saved: Fixture = { id: 'server-id', updated_at: '2026-01-01T00:00:05Z', name: 'A', note: 'x', count: 1 };
    const savingDraft: Fixture = { id: 'client-id', updated_at: '2026-01-01T00:00:00Z', name: 'A', note: 'x', count: 1 };
    const latestDraft: Fixture = { id: 'other-id', updated_at: '2026-01-01T00:00:01Z', name: 'A', note: 'x', count: 1 };

    const result = rebaseEditableChanges(saved, savingDraft, latestDraft, BACKEND_OWNED);

    expect(result.id).toBe('server-id');
    expect(result.updated_at).toBe('2026-01-01T00:00:05Z');
  });

  it('prefers latestDraft over saved for a non-backend-owned field edited mid-save', () => {
    const saved: Fixture = { id: '1', updated_at: 't1', name: 'from-server', note: 'x', count: 1 };
    const savingDraft: Fixture = { id: '1', updated_at: 't0', name: 'first-edit', note: 'x', count: 1 };
    const latestDraft: Fixture = { id: '1', updated_at: 't0', name: 'second-edit-mid-save', note: 'x', count: 1 };

    const result = rebaseEditableChanges(saved, savingDraft, latestDraft, BACKEND_OWNED);

    expect(result.name).toBe('second-edit-mid-save');
  });

  it('takes the server value for a non-backend-owned field the user did not touch mid-save', () => {
    const saved: Fixture = { id: '1', updated_at: 't1', name: 'from-server', note: 'canonicalized-by-server', count: 1 };
    const savingDraft: Fixture = { id: '1', updated_at: 't0', name: 'edited', note: 'x', count: 1 };
    const latestDraft: Fixture = { id: '1', updated_at: 't0', name: 'edited', note: 'x', count: 1 };

    const result = rebaseEditableChanges(saved, savingDraft, latestDraft, BACKEND_OWNED);

    expect(result.note).toBe('canonicalized-by-server');
  });

  it('produces exactly `saved` when savingDraft and latestDraft are identical', () => {
    const saved: Fixture = { id: '1', updated_at: 't1', name: 'from-server', note: 'y', count: 9 };
    const savingDraft: Fixture = { id: '1', updated_at: 't0', name: 'edited', note: 'x', count: 1 };
    const latestDraft: Fixture = { ...savingDraft };

    const result = rebaseEditableChanges(saved, savingDraft, latestDraft, BACKEND_OWNED);

    expect(result).toEqual(saved);
  });

  it('handles a numeric field edited mid-save (not just strings)', () => {
    const saved: Fixture = { id: '1', updated_at: 't1', name: 'x', note: 'x', count: 100 };
    const savingDraft: Fixture = { id: '1', updated_at: 't0', name: 'x', note: 'x', count: 5 };
    const latestDraft: Fixture = { id: '1', updated_at: 't0', name: 'x', note: 'x', count: 7 };

    const result = rebaseEditableChanges(saved, savingDraft, latestDraft, BACKEND_OWNED);

    expect(result.count).toBe(7);
  });
});
