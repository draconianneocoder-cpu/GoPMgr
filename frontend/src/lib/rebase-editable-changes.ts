// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Shared by StakeholderManager, ProjectSettings, WorkItemEditor, and
// SprintList's save() functions -- not every autosave.register()-backed
// editor; the shared chart-editor shells that back most other editors
// use a different rebase mechanism of their own.
//
// The problem this solves: save() sends `savingDraft` to the backend and
// gets `saved` back. If the user edited a field again while that request
// was still in flight, `latestDraft` (the live state at response time) can
// differ from `savingDraft`. A save() that just does `draft = saved`
// silently discards that mid-flight edit. Backend-owned fields (id,
// timestamps, and any field the backend computes or that's mutated through
// a different code path than this editor's own form) must NOT be
// diffed the same way -- they should always come from `saved`, because the
// draft's copy of them is stale by construction, not a real edit.
//
// `backendOwnedKeys` is deliberately a blocklist, not an allowlist: a new
// field added to the underlying type is editable-by-default and covered
// here with zero code change at the call site. Only fields the backend
// mutates independently of this editor's own form need to be named.
export function rebaseEditableChanges<T extends object>(
  saved: T,
  savingDraft: T,
  latestDraft: T,
  backendOwnedKeys: readonly (keyof T)[],
): T {
  const result = { ...saved };
  for (const key of Object.keys(saved) as (keyof T)[]) {
    if (backendOwnedKeys.includes(key)) continue;
    if (latestDraft[key] !== savingDraft[key]) {
      result[key] = latestDraft[key];
    }
  }
  return result;
}
