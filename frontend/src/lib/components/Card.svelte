<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Shared static panel wrapper for the app's ~40 hand-copied
  // "rounded border bg-slate-900/900" container variants. Deliberately
  // static only (a <div>, no clickable/polymorphic mode) — the app's
  // card-shaped *buttons* (Dashboard's stat/catalog tiles) are structurally
  // full custom rows with their own layout, not a generic clickable
  // surface, and forcing them through a polymorphic <svelte:element>
  // risks losing real behavior (text-left, focus handling) on the app's
  // most-visited screen for a small dedup win. This covers the confirmed
  // byte-identical wrapper duplicated across ChartCatalog.svelte and
  // DocumentCatalog.svelte.
  import type { Snippet } from 'svelte';

  let {
    class: klass = '',
    children,
  }: {
    class?: string;
    children?: Snippet;
  } = $props();
</script>

<div class="overflow-hidden rounded-lg border border-slate-800 bg-slate-900/60 {klass}">
  {@render children?.()}
</div>
