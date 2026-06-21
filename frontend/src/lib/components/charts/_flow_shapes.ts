// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Geometry helpers shared by WorkflowEditor and ActivityEditor.
//
// Each shape function returns the SVG path/element data needed to
// render that node. Returning strings (rather than imperatively
// drawing) keeps the editors declarative: the Svelte template can
// just say <path d={shapePath(n)} ... />.

export interface FlowNode {
  id: string;
  label: string;
  shape: string;
  swimlane_id?: string;
  rank: number;
  x: number;
  y: number;
  width: number;
  height: number;
}

// shapePath returns an SVG path string for a node's outline. The path
// is positioned in the local coordinate frame (0,0 at the node's
// top-left corner) so the caller can wrap it in
//
//   <g transform="translate(n.x, n.y)">
//     <path d={shapePath(n)} ... />
//
// without doing further offset math.
export function shapePath(n: FlowNode): string {
  const w = n.width;
  const h = n.height;
  switch (n.shape) {
    // ---- Workflow ----
    case 'start':
    case 'end':
      // Oval (rounded rect with rx = h/2).
      return roundedRect(0, 0, w, h, h / 2);
    case 'action':
      return roundedRect(0, 0, w, h, 6);
    case 'decision':
      // Diamond from corner points.
      return [
        `M ${w / 2} 0`,
        `L ${w} ${h / 2}`,
        `L ${w / 2} ${h}`,
        `L 0 ${h / 2}`,
        'Z',
      ].join(' ');
    case 'io': {
      // Parallelogram (slanted edges). Slant = h/3.
      const s = h / 3;
      return [
        `M ${s} 0`,
        `L ${w} 0`,
        `L ${w - s} ${h}`,
        `L 0 ${h}`,
        'Z',
      ].join(' ');
    }
    case 'subprocess':
      // Rect; the double vertical bars are drawn separately.
      return roundedRect(0, 0, w, h, 6);

    // ---- Activity ----
    case 'initial':
      // Filled circle, drawn as a small rect's worth of pixel cells.
      return circlePath(w / 2, h / 2, Math.min(w, h) / 2);
    case 'final':
      // Outer circle; the inner filled dot is drawn separately.
      return circlePath(w / 2, h / 2, Math.min(w, h) / 2);
    case 'activity':
      return roundedRect(0, 0, w, h, 10);
    case 'a_decision':
      return [
        `M ${w / 2} 0`,
        `L ${w} ${h / 2}`,
        `L ${w / 2} ${h}`,
        `L 0 ${h / 2}`,
        'Z',
      ].join(' ');
    case 'fork':
    case 'join':
      return roundedRect(0, 0, w, h, 2);
    default:
      return roundedRect(0, 0, w, h, 6);
  }
}

// shapeFill returns the fill colour for a given shape. The frontend
// keeps the per-shape palette here so both editors stay visually
// consistent and a theme change is one file.
export function shapeFill(shape: string, selected: boolean): string {
  if (selected) return '#0e7490';
  switch (shape) {
    case 'start':
      return '#16a34a';
    case 'end':
      return '#7f1d1d';
    case 'decision':
    case 'a_decision':
      return '#a16207';
    case 'io':
      return '#1e40af';
    case 'subprocess':
      return '#312e81';
    case 'initial':
    case 'fork':
    case 'join':
      return '#f1f5f9';
    case 'final':
      return '#1e293b';
    default:
      return '#1e293b';
  }
}

// shapeTextFill returns the label colour for a given shape.
export function shapeTextFill(shape: string): string {
  switch (shape) {
    case 'start':
    case 'end':
    case 'io':
    case 'subprocess':
      return '#f1f5f9';
    case 'decision':
    case 'a_decision':
      return '#fef3c7';
    default:
      return '#f1f5f9';
  }
}

// ---- Helpers ----

function roundedRect(x: number, y: number, w: number, h: number, r: number): string {
  const rr = Math.min(r, w / 2, h / 2);
  return [
    `M ${x + rr} ${y}`,
    `L ${x + w - rr} ${y}`,
    `Q ${x + w} ${y} ${x + w} ${y + rr}`,
    `L ${x + w} ${y + h - rr}`,
    `Q ${x + w} ${y + h} ${x + w - rr} ${y + h}`,
    `L ${x + rr} ${y + h}`,
    `Q ${x} ${y + h} ${x} ${y + h - rr}`,
    `L ${x} ${y + rr}`,
    `Q ${x} ${y} ${x + rr} ${y}`,
    'Z',
  ].join(' ');
}

function circlePath(cx: number, cy: number, r: number): string {
  return [
    `M ${cx - r} ${cy}`,
    `a ${r} ${r} 0 1 0 ${2 * r} 0`,
    `a ${r} ${r} 0 1 0 ${-2 * r} 0`,
    'Z',
  ].join(' ');
}

// ---- Edge routing ----

// edgePath returns the SVG path for an orthogonal connector from one
// node to another, going down through the midpoint between rows.
export function edgePath(from: FlowNode, to: FlowNode): string {
  const x1 = from.x + from.width / 2;
  const y1 = from.y + from.height;
  const x2 = to.x + to.width / 2;
  const y2 = to.y;
  const midY = (y1 + y2) / 2;
  if (Math.abs(x1 - x2) < 1) {
    return `M ${x1} ${y1} L ${x2} ${y2}`;
  }
  return `M ${x1} ${y1} L ${x1} ${midY} L ${x2} ${midY} L ${x2} ${y2}`;
}

// edgeLabelPosition returns (x, y) for placing an edge's label at the
// vertical midpoint.
export function edgeLabelPosition(from: FlowNode, to: FlowNode): { x: number; y: number } {
  const x1 = from.x + from.width / 2;
  const y1 = from.y + from.height;
  const x2 = to.x + to.width / 2;
  const y2 = to.y;
  return { x: (x1 + x2) / 2 + 6, y: (y1 + y2) / 2 - 4 };
}
