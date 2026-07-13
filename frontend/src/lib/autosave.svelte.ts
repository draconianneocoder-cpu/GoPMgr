// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Timed auto-save coordinator. Editors register a (snapshot, save) pair;
// a single 1-second heartbeat saves each registered editor whose snapshot
// has changed since its last save, once per configured interval. The
// interval comes from the user's app settings (auto_save_seconds): 0 turns
// auto-save off. Snapshot-based change detection means an idle editor is
// never re-saved, so auto-save never churns updated_at without real edits.
//
// Manual save (Ctrl/Cmd+S and the toolbar Save button) is unchanged and
// always available; this only adds the timed safety net.

interface Entry {
  snapshot: () => string;
  save: () => unknown;
  last: string;
  // True while a save is in flight, so a slow save can never overlap the
  // next interval's save for the same editor (overlapping whole-doc writes
  // could land out of order and persist the older snapshot last).
  saving?: boolean;
}

let intervalSeconds = $state(0); // 0 = auto-save off
let elapsed = 0;
const entries = new Set<Entry>();
let heartbeat: ReturnType<typeof setInterval> | null = null;

function safeSnapshot(fn: () => string): string {
  try {
    return fn();
  } catch {
    return '';
  }
}

function tick(): void {
  if (intervalSeconds <= 0 || entries.size === 0) {
    elapsed = 0;
    return;
  }
  elapsed += 1;
  if (elapsed < intervalSeconds) return;
  elapsed = 0;
  for (const e of entries) {
    if (e.saving) continue; // previous save still in flight
    const snap = safeSnapshot(e.snapshot);
    if (snap === e.last) continue; // no changes since last save
    try {
      e.saving = true;
      Promise.resolve(e.save())
        .then(() => {
          e.last = safeSnapshot(e.snapshot);
        })
        .catch(() => {})
        .finally(() => {
          e.saving = false;
        });
    } catch {
      e.saving = false;
    }
  }
}

function ensureHeartbeat(): void {
  if (heartbeat === null) heartbeat = setInterval(tick, 1000);
}

export const autosave = {
  /** Current interval in seconds (0 = off). Reactive in components. */
  get intervalSeconds(): number {
    return intervalSeconds;
  },
  /** Set the interval (seconds); 0 disables auto-save. */
  setInterval(seconds: number): void {
    intervalSeconds = Math.max(0, Math.floor(seconds || 0));
    elapsed = 0;
  },
  /**
   * Register an editor for auto-save. `snapshot` returns a string that
   * changes when the editor's working content changes; `save` persists it.
   * Returns an unregister function — call it in onDestroy.
   */
  register(snapshot: () => string, save: () => unknown): () => void {
    const entry: Entry = { snapshot, save, last: safeSnapshot(snapshot) };
    entries.add(entry);
    ensureHeartbeat();
    return () => {
      entries.delete(entry);
    };
  },
};
