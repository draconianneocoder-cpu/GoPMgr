<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // LayeredDiagram is the shared SVG renderer for Network/PERT/CPM
  // charts. It draws the layered DAG geometry that the backend computed
  // and, via a `nodeContent` snippet, lets the caller decorate each
  // node with kind-specific text (PERT durations, CPM floats, etc.).
  //
  // Props
  // -----
  //   layout       — Layout returned by charts.LayoutLayered (Go side).
  //   nodes        — The LayeredNode array (so the snippet can read
  //                  the kind-specific fields like ES/EF/Float).
  //   selectedId   — Currently-focused node ID.
  //   nodeContent  — Snippet that renders inside each node rectangle.
  //                  Receives (node, nodeLayout) as arguments.

  interface LayoutNode {
    id: string;
    title: string;
    note?: string;
    owner?: string;
    depth: number;
    x: number;
    y: number;
    width: number;
    height: number;
  }
  interface LayoutEdge {
    from: string;
    to: string;
  }
  interface Layout {
    nodes: LayoutNode[];
    edges: LayoutEdge[];
    width: number;
    height: number;
  }
  interface LayeredNode {
    id: string;
    label: string;
    [k: string]: unknown;
  }

  import type { Snippet } from 'svelte';

  let {
    layout,
    nodes,
    selectedId = $bindable<string | null>(null),
    nodeContent,
  }: {
    layout: Layout;
    nodes: LayeredNode[];
    selectedId?: string | null;
    nodeContent?: Snippet<[LayeredNode, LayoutNode]>;
  } = $props();

  function nodeData(id: string): LayeredNode | undefined {
    return nodes.find((n) => n.id === id);
  }

  // Edge routing: orthogonal path with a midpoint elbow. Simple but
  // legible and avoids overlap with adjacent nodes.
  function edgePath(from: LayoutNode, to: LayoutNode): string {
    const x1 = from.x + from.width;
    const y1 = from.y + from.height / 2;
    const x2 = to.x;
    const y2 = to.y + to.height / 2;
    const midX = (x1 + x2) / 2;
    return `M ${x1} ${y1} L ${midX} ${y1} L ${midX} ${y2} L ${x2} ${y2}`;
  }
</script>

<svg
  role="application"
  aria-label="Layered DAG diagram"
  width={Math.max(layout.width + 40, 600)}
  height={Math.max(layout.height + 60, 400)}
  class="bg-slate-900 border border-slate-800 rounded"
>
  <!-- Arrowhead marker -->
  <defs>
    <marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
      <path d="M 0 0 L 10 5 L 0 10 z" fill="#64748b" />
    </marker>
  </defs>

  <!-- Edges -->
  <g transform="translate(20,30)">
    {#each layout.edges as e (e.from + '-' + e.to)}
      {@const from = layout.nodes.find((n) => n.id === e.from)}
      {@const to = layout.nodes.find((n) => n.id === e.to)}
      {#if from && to}
        <path
          d={edgePath(from, to)}
          stroke="#64748b"
          stroke-width="1.5"
          fill="none"
          marker-end="url(#arrow)"
        />
      {/if}
    {/each}

    <!-- Nodes -->
    {#each layout.nodes as n (n.id)}
      {@const data = nodeData(n.id)}
      <g
        transform={`translate(${n.x},${n.y})`}
        onclick={() => (selectedId = n.id)}
        onkeydown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            selectedId = n.id;
          }
        }}
        role="button"
        tabindex="0"
        aria-label={n.title}
        aria-pressed={selectedId === n.id}
        class="cursor-pointer"
      >
        <rect
          width={n.width}
          height={n.height}
          rx="6"
          fill={selectedId === n.id ? '#0e7490' : '#1e293b'}
          stroke={selectedId === n.id ? '#22d3ee' : '#334155'}
          stroke-width="1.5"
        />
        {#if nodeContent && data}
          {@render nodeContent(data, n)}
        {:else}
          <text x="8" y="20" font-size="12" fill="#f1f5f9" font-weight="bold">
            {n.title.length > 22 ? n.title.slice(0, 21) + '…' : n.title}
          </text>
        {/if}
      </g>
    {/each}
  </g>
</svg>
