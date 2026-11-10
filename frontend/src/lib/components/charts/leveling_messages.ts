// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Pure builders for the resource-leveling / split-preview user messages and
// the manual-edit segment reset. Kept free of Svelte and the Wails bridge so
// the CPM and Gantt editors' action handlers are behaviour-verified by unit
// tests rather than only type-checked.

import { formatList, t } from '../../i18n/i18n.svelte';

// Structural subsets of the App return shapes (see app_charts.go
// LevelResult / SplitLevelingPreview).
export interface LevelResult {
  pinned: number;
  unplaced_labels?: string[] | null;
  split_labels?: string[] | null;
}

export interface SplitLevelingPreview {
  split_task_labels?: string[] | null;
  resolves_overallocation: boolean;
  remaining_overallocated_resources?: string[] | null;
}

// summarizeLabels renders a capped, comma-joined label list with a "+N more"
// suffix, e.g. "a, b, c +2 more".
export function summarizeLabels(labels: string[], max = 3): string {
  const shown = formatList(labels.slice(0, max));
  return labels.length > max ? t('leveling.moreLabels', { labels: shown, count: labels.length - max }) : shown;
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
    flash = t('leveling.pinned', { count: res.pinned });
  } else if (unplaced.length > 0) {
    flash = t('leveling.noneShifted');
  } else {
    flash = t('leveling.alreadyLevel');
  }
  let warn = '';
  let warnTitle = '';
  if (unplaced.length > 0) {
    warn = t('leveling.stillOverallocated', { count: unplaced.length, labels: summarizeLabels(unplaced) });
    warnTitle = t('leveling.stillOverallocated.hint', { labels: formatList(unplaced) });
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
    return { msg: t('leveling.noSplitNeeded'), title: '' };
  }
  if (p.resolves_overallocation) {
    return {
      msg: t('leveling.splitClears', { count: labels.length, labels: summarizeLabels(labels) }),
      title: t('leveling.splitClears.hint', { labels: formatList(labels) }),
    };
  }
  const stuck = p.remaining_overallocated_resources ?? [];
  return {
    msg: t('leveling.splitStuck', { count: stuck.length }),
    title: t('leveling.splitStuck.hint', { labels: formatList(stuck) }),
  };
}

// Gantt "Level (split)" apply-result status line.
export function splitLevelStatus(res: LevelResult): string {
  const split = res.split_labels ?? [];
  return split.length
    ? t('leveling.splitApplied', { count: split.length, labels: summarizeLabels(split) })
    : t('leveling.splitNone');
}

// clearWorkSegments strips persisted split segments from chart nodes. A
// manual schedule edit invalidates the leveled snapshot, so the Gantt editor
// calls this before re-laying-out. Mutates in place.
export function clearWorkSegments(nodes: Array<Record<string, unknown>>): void {
  for (const n of nodes) n.work_segments = undefined;
}
