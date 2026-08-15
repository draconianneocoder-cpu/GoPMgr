// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
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
  save: () => unknown | Promise<unknown>;
  last: string;
  automatic: boolean;
  // True while a save is in flight, so a slow save can never overlap the
  // next interval's save for the same editor (overlapping whole-doc writes
  // could land out of order and persist the older snapshot last).
  saving?: boolean;
}

let intervalSeconds = $state(0); // 0 = auto-save off
let elapsed = 0;
const entries = new Set<Entry>();
let heartbeat: ReturnType<typeof setInterval> | null = null;
let lastError = $state('');

function safeSnapshot(fn: () => string): string {
  try {
    return fn();
  } catch {
    return '';
  }
}

async function saveEntry(e: Entry): Promise<boolean> {
  if (e.saving) return false;
  e.saving = true;
  const savedSnapshot = safeSnapshot(e.snapshot);
  try {
    const result = await e.save();
    if (result === false) throw new Error('The editor reported that the save failed.');
    e.last = savedSnapshot;
    lastError = '';
    return true;
  } catch (err: unknown) {
    lastError = err instanceof Error ? err.message : String(err ?? 'Save failed.');
    return false;
  } finally {
    e.saving = false;
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
    if (!e.automatic) continue;
    if (e.saving) continue; // previous save still in flight
    const snap = safeSnapshot(e.snapshot);
    if (snap === e.last) continue; // no changes since last save
    void saveEntry(e);
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
  get lastError(): string {
    return lastError;
  },
  hasDirty(): boolean {
    return [...entries].some((e) => safeSnapshot(e.snapshot) !== e.last);
  },
  async saveAll(): Promise<boolean> {
    const dirty = [...entries].filter((e) => safeSnapshot(e.snapshot) !== e.last);
    const results = await Promise.all(dirty.map(saveEntry));
    if (!results.every(Boolean)) return false;
    if (this.hasDirty()) {
      lastError = 'Changes were made while saving. Save again before continuing.';
      return false;
    }
    return true;
  },
  discardAll(): void {
    for (const e of entries) e.last = safeSnapshot(e.snapshot);
    lastError = '';
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
  register(
    snapshot: () => string,
    save: () => unknown | Promise<unknown>,
    automatic = true,
  ): () => void {
    const entry: Entry = { snapshot, save, last: safeSnapshot(snapshot), automatic };
    entries.add(entry);
    ensureHeartbeat();
    return () => {
      entries.delete(entry);
    };
  },
};
