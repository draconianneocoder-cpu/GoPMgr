// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Pure geometry helpers for the Gantt bar canvas. Kept free of Svelte and
// the Wails bridge so they can be unit-tested directly; GanttBars.svelte
// renders from these so the tests cover the production code path.

export const GANTT_ROW_H = 30;
export const GANTT_BAR_H = 14;

export interface GanttSegment {
  start: number;
  end: number;
}

export interface GanttRow {
  id: string;
  label: string;
  es: number;
  ef: number;
  float?: number;
  is_critical?: boolean;
  milestone?: boolean;
  percent_complete: number;
  start_date?: string;
  finish_date?: string;
  overallocated?: boolean;
  constraint_violated?: boolean;
  work_segments?: GanttSegment[];
}

export interface GanttDep {
  from: string;
  to: string;
  label?: string;
}

export interface GanttLayout {
  rows: GanttRow[];
  deps: GanttDep[];
  horizon: number;
  anchored?: boolean;
}

export interface Bar {
  x: number;
  w: number;
}

// Minimal shape of a schedule-baseline variance (structural subset of the
// global ScheduleVariance) so this module has no ambient-type dependency.
export interface Variance {
  start_var_days: number;
  finish_var_days: number;
}

// rowEnd is a row's rightmost occupied offset: EF, or the last split segment
// end when the task is interrupted past its contiguous finish.
export function rowEnd(r: GanttRow): number {
  let end = r.ef;
  const segs = r.work_segments;
  if (segs && segs.length) end = Math.max(end, segs[segs.length - 1].end);
  return end;
}

// barPieces returns the filled bar rectangles for a task: one per split
// working-day run (absolute offsets), or a single ES..EF bar when the task
// is contiguous. Widths are floored so a zero/tiny bar stays visible.
export function barPieces(r: GanttRow, pxPerDay: number): Bar[] {
  const segs = r.work_segments;
  if (segs && segs.length) {
    return segs.map((s) => ({ x: s.start * pxPerDay, w: Math.max(2, (s.end - s.start) * pxPerDay) }));
  }
  return [{ x: r.es * pxPerDay, w: Math.max(2, (r.ef - r.es) * pxPerDay) }];
}

// baselineBar returns the ghost baseline bar for a row, or null when there
// is no variance or the baseline span is empty.
export function baselineBar(r: GanttRow, v: Variance | undefined, pxPerDay: number): Bar | null {
  if (!v) return null;
  const bes = r.es - v.start_var_days;
  const bef = r.ef - v.finish_var_days;
  if (bef <= bes) return null;
  return { x: bes * pxPerDay, w: (bef - bes) * pxPerDay };
}

// depPath builds the elbow SVG path for a dependency arrow. It leaves the
// predecessor at its real finish (rowEnd — the last split segment end for an
// interrupted task, else EF) and enters the successor at its ES. Returns
// null when either endpoint isn't in the layout.
export function depPath(
  dep: { from: string; to: string },
  rows: GanttRow[],
  pxPerDay: number,
  rowH: number = GANTT_ROW_H,
): string | null {
  const fi = rows.findIndex((r) => r.id === dep.from);
  const ti = rows.findIndex((r) => r.id === dep.to);
  if (fi < 0 || ti < 0) return null;
  const x1 = rowEnd(rows[fi]) * pxPerDay;
  const y1 = fi * rowH + rowH / 2;
  const x2 = rows[ti].es * pxPerDay;
  const y2 = ti * rowH + rowH / 2;
  const elbow = Math.max(x1 + 8, x2 - 8);
  return `M ${x1} ${y1} L ${elbow} ${y1} L ${elbow} ${y2} L ${x2} ${y2}`;
}
