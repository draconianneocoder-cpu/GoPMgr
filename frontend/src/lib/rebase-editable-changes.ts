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
  // Equality against savingDraft is a proxy for "was this field edited
  // during the flight," not a real dirty flag. It's only wrong if the
  // user's mid-flight edit round-trips back to the exact value that was
  // sent AND the backend independently returns something different for
  // that same field -- today no editor's backend does the latter for a
  // non-blocklisted field (it just echoes what was sent), so the proxy
  // and a real dirty flag agree in every reachable case. If a future
  // backend change starts normalizing a non-blocklisted field, this
  // stops being equivalent to a real dirty flag.
  const result = { ...saved };
  for (const key of Object.keys(saved) as (keyof T)[]) {
    if (backendOwnedKeys.includes(key)) continue;
    if (latestDraft[key] !== savingDraft[key]) {
      result[key] = latestDraft[key];
    }
  }
  return result;
}
