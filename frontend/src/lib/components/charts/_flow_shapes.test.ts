// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import {
  edgeLabelPosition,
  edgePath,
  shapeFill,
  shapePath,
  shapeTextFill,
  type FlowNode,
} from './_flow_shapes';

function node(overrides: Partial<FlowNode> = {}): FlowNode {
  return {
    id: 'n1',
    label: 'Node',
    shape: 'action',
    rank: 0,
    x: 0,
    y: 0,
    width: 120,
    height: 40,
    ...overrides,
  };
}

describe('shapePath', () => {
  it('draws start/end as an oval — a rounded rect with rx = h/2', () => {
    const n = node({ shape: 'start', width: 100, height: 40 });
    // roundedRect(0,0,100,40, rr=min(20,50,20)=20) starts its path at
    // (x+rr, y) = (20, 0). A radius equal to half the height is what
    // makes the rect read as an oval instead of a rounded rectangle.
    expect(shapePath(n)).toMatch(/^M 20 0 /);
    expect(shapePath(node({ shape: 'end', width: 100, height: 40 }))).toBe(
      shapePath(node({ shape: 'start', width: 100, height: 40 })),
    );
  });

  it('draws decision as a diamond through the four edge midpoints', () => {
    const n = node({ shape: 'decision', width: 100, height: 40 });
    expect(shapePath(n)).toBe('M 50 0 L 100 20 L 50 40 L 0 20 Z');
  });

  it('draws io as a parallelogram slanted by h/3', () => {
    const n = node({ shape: 'io', width: 90, height: 30 });
    // slant s = h/3 = 10
    expect(shapePath(n)).toBe('M 10 0 L 90 0 L 80 30 L 0 30 Z');
  });

  it('falls back to a rounded rect for an unrecognised shape', () => {
    expect(shapePath(node({ shape: 'not-a-real-shape' }))).toBe(
      shapePath(node({ shape: 'action' })),
    );
  });

  it('a_decision matches decision (both render as a diamond)', () => {
    const dims = { width: 80, height: 50 };
    expect(shapePath(node({ shape: 'a_decision', ...dims }))).toBe(
      shapePath(node({ shape: 'decision', ...dims })),
    );
  });
});

describe('shapeFill', () => {
  it('selected always wins regardless of shape', () => {
    expect(shapeFill('start', true)).toBe('#0e7490');
    expect(shapeFill('decision', true)).toBe('#0e7490');
  });

  it('gives workflow terminal shapes distinct colours', () => {
    expect(shapeFill('start', false)).toBe('#16a34a');
    expect(shapeFill('end', false)).toBe('#7f1d1d');
    expect(shapeFill('start', false)).not.toBe(shapeFill('end', false));
  });

  it('shares a colour between decision and a_decision', () => {
    expect(shapeFill('decision', false)).toBe(shapeFill('a_decision', false));
  });
});

describe('shapeTextFill', () => {
  it('gives decision shapes dark-on-light label text', () => {
    expect(shapeTextFill('decision')).toBe('#fef3c7');
    expect(shapeTextFill('a_decision')).toBe('#fef3c7');
  });

  it('defaults to light label text for other shapes', () => {
    expect(shapeTextFill('action')).toBe('#f1f5f9');
    expect(shapeTextFill('not-a-real-shape')).toBe('#f1f5f9');
  });
});

describe('edgePath', () => {
  it('routes straight down when nodes share a centre x', () => {
    const from = node({ x: 0, y: 0, width: 100, height: 40 });
    const to = node({ x: 0, y: 100, width: 100, height: 40 });
    expect(edgePath(from, to)).toBe('M 50 40 L 50 100');
  });

  it('routes through the vertical midpoint when nodes are offset horizontally', () => {
    const from = node({ x: 0, y: 0, width: 100, height: 40 });
    const to = node({ x: 200, y: 100, width: 100, height: 40 });
    // x1=50, y1=40; x2=250, y2=100; midY=(40+100)/2=70
    expect(edgePath(from, to)).toBe('M 50 40 L 50 70 L 250 70 L 250 100');
  });
});

describe('edgeLabelPosition', () => {
  it('places the label at the midpoint, offset for legibility', () => {
    const from = node({ x: 0, y: 0, width: 100, height: 40 });
    const to = node({ x: 200, y: 100, width: 100, height: 40 });
    // x=(50+250)/2+6=156; y=(40+100)/2-4=66
    expect(edgeLabelPosition(from, to)).toEqual({ x: 156, y: 66 });
  });
});
