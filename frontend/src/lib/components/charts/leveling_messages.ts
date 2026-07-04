// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Pure builders for the resource-leveling / split-preview user messages and
// the manual-edit segment reset. Kept free of Svelte and the Wails bridge so
// the CPM and Gantt editors' action handlers are behaviour-verified by unit
// tests rather than only type-checked.

// Structural subsets of the App return shapes (see main.go LevelResult /
// SplitLevelingPreview).
export interface LevelResult {
  pinned: number;
  unplaced_labels?: string[];
  split_labels?: string[];
}

export interface SplitLevelingPreview {
  split_task_labels?: string[];
  resolves_overallocation: boolean;
  remaining_overallocated_resources?: string[];
}

// summarizeLabels renders a capped, comma-joined label list with a "+N more"
// suffix, e.g. "a, b, c +2 more".
export function summarizeLabels(labels: string[], max = 3): string {
  const shown = labels.slice(0, max).join(', ');
  const more = labels.length > max ? ` +${labels.length - max} more` : '';
  return `${shown}${more}`;
}

// CPM "Level resources": a transient success flash plus a persistent
// overallocation warning (with tooltip) for tasks that could not be placed.
export interface LevelResourcesMessages {
  flash: string;
  warn: string;
  warnTitle: string;
}

export function levelResourcesMessages(res: LevelResult): LevelResourcesMessages {
  const unplaced = res.unplaced_labels ?? [];
  let flash: string;
  if (res.pinned > 0) {
    flash = `Levelled: ${res.pinned} task(s) pinned (SNET)`;
  } else if (unplaced.length > 0) {
    flash = 'No tasks could be shifted into free capacity';
  } else {
    flash = 'Already level: nothing moved';
  }
  let warn = '';
  let warnTitle = '';
  if (unplaced.length > 0) {
    warn = `${unplaced.length} task(s) still overallocated: ${summarizeLabels(unplaced)}`;
    warnTitle =
      'Demand exceeds available capacity for: ' +
      unplaced.join(', ') +
      '. Reduce assigned units, add resource capacity, or split the work across days.';
  }
  return { flash, warn, warnTitle };
}

// Read-only split-leveling preview: message plus a tooltip title. Shared by
// both editors.
export interface SplitPreviewMessage {
  msg: string;
  title: string;
}

export function splitPreviewMessage(p: SplitLevelingPreview): SplitPreviewMessage {
  const labels = p.split_task_labels ?? [];
  if (labels.length === 0) {
    return { msg: 'No tasks need splitting at current capacity', title: '' };
  }
  if (p.resolves_overallocation) {
    return {
      msg: `Splitting ${labels.length} task(s) would clear overallocation: ${summarizeLabels(labels)}`,
      title:
        'Interrupting these tasks across non-contiguous days resolves all resource conflicts: ' +
        labels.join(', ') +
        '. Splitting is analysis-only and is not saved.',
    };
  }
  const stuck = p.remaining_overallocated_resources ?? [];
  return {
    msg: `Even with splitting, ${stuck.length} resource(s) stay over capacity`,
    title:
      'These resources have single-day demand above supply, which splitting cannot fix: ' +
      stuck.join(', ') +
      '. Reduce assigned units or add capacity.',
  };
}

// Gantt "Level (split)" apply-result status line.
export function splitLevelStatus(res: LevelResult): string {
  const split = res.split_labels ?? [];
  return split.length
    ? `Levelled; split ${split.length} task(s): ${summarizeLabels(split)}`
    : 'Levelled; no tasks needed splitting';
}

// clearWorkSegments strips persisted split segments from chart nodes. A
// manual schedule edit invalidates the leveled snapshot, so the Gantt editor
// calls this before re-laying-out. Mutates in place.
export function clearWorkSegments(nodes: Array<Record<string, unknown>>): void {
  for (const n of nodes) n.work_segments = undefined;
}
