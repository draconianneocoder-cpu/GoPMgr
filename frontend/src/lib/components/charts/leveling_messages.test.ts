// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from 'vitest';
import {
  summarizeLabels,
  levelResourcesMessages,
  splitPreviewMessage,
  splitLevelStatus,
  clearWorkSegments,
} from './leveling_messages';

describe('summarizeLabels', () => {
  it('joins up to the cap with no suffix', () => {
    expect(summarizeLabels(['a', 'b', 'c'])).toBe('a, b, c');
  });
  it('adds "+N more" beyond the cap', () => {
    expect(summarizeLabels(['a', 'b', 'c', 'd', 'e'])).toBe('a, b, c +2 more');
  });
  it('is empty for no labels', () => {
    expect(summarizeLabels([])).toBe('');
  });
});

describe('levelResourcesMessages', () => {
  it('reports pins with no warning when everything fit', () => {
    const m = levelResourcesMessages({ pinned: 2 });
    expect(m.flash).toBe('Levelled: 2 task(s) pinned (SNET)');
    expect(m.warn).toBe('');
    expect(m.warnTitle).toBe('');
  });
  it('reports "already level" when nothing moved and nothing is stuck', () => {
    expect(levelResourcesMessages({ pinned: 0 }).flash).toBe('Already level: nothing moved');
  });
  it('does not claim "already level" when tasks stay overallocated', () => {
    const m = levelResourcesMessages({ pinned: 0, unplaced_labels: ['A', 'B'] });
    expect(m.flash).toBe('No tasks could be shifted into free capacity');
    expect(m.warn).toBe('2 task(s) still overallocated: A, B');
    expect(m.warnTitle).toContain('Demand exceeds available capacity for: A, B');
  });
  it('pins can coexist with a residual overallocation warning', () => {
    const m = levelResourcesMessages({ pinned: 1, unplaced_labels: ['X'] });
    expect(m.flash).toBe('Levelled: 1 task(s) pinned (SNET)');
    expect(m.warn).toBe('1 task(s) still overallocated: X');
  });
});

describe('splitPreviewMessage', () => {
  it('says nothing needs splitting when there are no split tasks', () => {
    const m = splitPreviewMessage({ split_task_labels: [], resolves_overallocation: true });
    expect(m.msg).toBe('No tasks need splitting at current capacity');
    expect(m.title).toBe('');
  });
  it('reports a clean resolution with a truncated task list', () => {
    const m = splitPreviewMessage({
      split_task_labels: ['A', 'B', 'C', 'D'],
      resolves_overallocation: true,
    });
    expect(m.msg).toBe('Splitting 4 task(s) would clear overallocation: A, B, C +1 more');
    expect(m.title).toContain('analysis-only and is not saved');
  });
  it('reports resources still over capacity when splitting is insufficient', () => {
    const m = splitPreviewMessage({
      split_task_labels: ['A'],
      resolves_overallocation: false,
      remaining_overallocated_resources: ['alice', 'bob'],
    });
    expect(m.msg).toBe('Even with splitting, 2 resource(s) stay over capacity');
    expect(m.title).toContain('single-day demand above supply');
  });
});

describe('splitLevelStatus', () => {
  it('reports how many tasks were split', () => {
    expect(splitLevelStatus({ pinned: 0, split_labels: ['A', 'B'] })).toBe(
      'Levelled; split 2 task(s): A, B',
    );
  });
  it('reports no splitting was needed', () => {
    expect(splitLevelStatus({ pinned: 0 })).toBe('Levelled; no tasks needed splitting');
  });
});

describe('clearWorkSegments', () => {
  it('strips work_segments from every node in place', () => {
    const nodes = [
      { id: 'A', work_segments: [{ start: 0, end: 1 }] },
      { id: 'B' },
    ];
    clearWorkSegments(nodes);
    expect(nodes[0].work_segments).toBeUndefined();
    expect('work_segments' in nodes[1] ? nodes[1].work_segments : undefined).toBeUndefined();
    // Other fields are untouched.
    expect(nodes[0].id).toBe('A');
  });
});
