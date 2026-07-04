// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from 'vitest';
import { rowEnd, barPieces, baselineBar, depPath, type GanttRow } from './gantt_geometry';

function row(over: Partial<GanttRow>): GanttRow {
  return { id: 'x', label: 'x', es: 0, ef: 1, percent_complete: 0, ...over };
}

describe('rowEnd', () => {
  it('returns EF for a contiguous task', () => {
    expect(rowEnd(row({ es: 2, ef: 5 }))).toBe(5);
  });
  it('returns the last split-segment end when it runs past EF', () => {
    // duration 3 but split across days 0,2,4 -> real finish is 5, EF is 3.
    const r = row({ es: 0, ef: 3, work_segments: [{ start: 0, end: 1 }, { start: 2, end: 3 }, { start: 4, end: 5 }] });
    expect(rowEnd(r)).toBe(5);
  });
});

describe('barPieces', () => {
  it('a contiguous task is one bar at ES..EF', () => {
    expect(barPieces(row({ es: 1, ef: 4 }), 10)).toEqual([{ x: 10, w: 30 }]);
  });
  it('a split task is one bar per absolute segment', () => {
    const r = row({ work_segments: [{ start: 0, end: 1 }, { start: 2, end: 3 }, { start: 4, end: 5 }] });
    expect(barPieces(r, 10)).toEqual([
      { x: 0, w: 10 },
      { x: 20, w: 10 },
      { x: 40, w: 10 },
    ]);
  });
  it('floors tiny/zero widths so the bar stays visible', () => {
    expect(barPieces(row({ es: 2, ef: 2 }), 10)).toEqual([{ x: 20, w: 2 }]);
  });
});

describe('baselineBar', () => {
  it('is null without a variance', () => {
    expect(baselineBar(row({}), undefined, 10)).toBeNull();
  });
  it('offsets the ghost bar by the recorded start/finish variance', () => {
    const b = baselineBar(row({ es: 3, ef: 6 }), { start_var_days: 1, finish_var_days: 1 }, 10);
    expect(b).toEqual({ x: 20, w: 30 }); // baseline started a day earlier, same span
  });
  it('is null when the baseline span is empty', () => {
    expect(baselineBar(row({ es: 0, ef: 1 }), { start_var_days: 0, finish_var_days: 5 }, 10)).toBeNull();
  });
});

describe('depPath', () => {
  const rows: GanttRow[] = [
    row({ id: 'A', es: 0, ef: 2 }),
    row({ id: 'B', es: 2, ef: 4 }),
  ];
  it('leaves the predecessor at EF for a contiguous task', () => {
    const p = depPath({ from: 'A', to: 'B' }, rows, 10, 30)!;
    expect(p.startsWith('M 20 ')).toBe(true); // A.ef=2 * 10 = 20
  });
  it('leaves a split predecessor at its last-segment finish, not EF', () => {
    const split: GanttRow[] = [
      row({ id: 'A', es: 0, ef: 3, work_segments: [{ start: 0, end: 1 }, { start: 2, end: 3 }, { start: 4, end: 5 }] }),
      row({ id: 'B', es: 6, ef: 8 }),
    ];
    const p = depPath({ from: 'A', to: 'B' }, split, 10, 30)!;
    expect(p.startsWith('M 50 ')).toBe(true); // rowEnd(A)=5 * 10 = 50, not EF(3)*10=30
  });
  it('returns null when an endpoint is missing', () => {
    expect(depPath({ from: 'A', to: 'ZZ' }, rows, 10)).toBeNull();
  });
});
