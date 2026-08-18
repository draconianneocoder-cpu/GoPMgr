<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Shared multi-line field — same rationale and rest-spread approach as
  // Input.svelte, for the app's ~28 <textarea> call sites that share the
  // same field family (see Input.svelte for the full note).
  import type { HTMLTextareaAttributes } from 'svelte/elements';

  let {
    value = $bindable<string | undefined>(),
    class: klass = '',
    ...rest
  }: HTMLTextareaAttributes & {
    value?: string;
  } = $props();
</script>

<textarea
  bind:value
  class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none disabled:opacity-50 {klass}"
  {...rest}
></textarea>
