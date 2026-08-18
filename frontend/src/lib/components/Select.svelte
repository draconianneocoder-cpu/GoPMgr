<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Shared select — same rest-spread approach and `w-full mt-1` layout
  // opinion as Input.svelte (see that file for the full note on both).
  // Deliberately normalizes to one background (bg-slate-950) and always
  // includes the cyan focus-border ring: the inventory found selects
  // app-wide split between bg-slate-900/950 with no evident reason, and
  // usually omit the focus ring inputs/textareas almost always have — this
  // closes that inconsistency rather than preserving it, so migrated call
  // sites need a live look (not just a passing test) to confirm the visual
  // delta is the intended one.
  import type { Snippet } from 'svelte';
  import type { HTMLSelectAttributes } from 'svelte/elements';

  let {
    value = $bindable<string | undefined>(),
    class: klass = '',
    children,
    ...rest
  }: HTMLSelectAttributes & {
    value?: string;
    children?: Snippet;
  } = $props();
</script>

<select
  bind:value
  class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none disabled:opacity-50 {klass}"
  {...rest}
>
  {@render children?.()}
</select>
