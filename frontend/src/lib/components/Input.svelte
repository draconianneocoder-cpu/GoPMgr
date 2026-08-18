<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Shared text field, replacing the app's single 90%-shared class string
  // (duplicated ~55 times verbatim, plus ~30 near-identical variants) that
  // underlies nearly every <input> in the app. Every native attribute
  // (type, min/max/step, list, autocomplete, aria-*, etc.) reaches the real
  // <input> via the ...rest spread below, so this wraps styling only — it
  // does not narrow what an <input> here can do. Focus-visible outline
  // already comes free from app.css's global rule; the focus:border-cyan-500
  // here is this field family's own border-color-on-focus, a separate,
  // pre-existing visual cue this wrapper preserves rather than reimplements.
  //
  // `w-full mt-1` is baked in and NOT overridable via `class` (Tailwind
  // resolves conflicting utilities by stylesheet position, not by class
  // order, so `class` can only append, never undo these two) — this
  // component is for the app's modal/form-field layout, not inline fields.
  // A dense-field size variant was considered (mid-implementation review)
  // but dropped: no proof-of-concept call site in this pass actually needs
  // one, so it isn't built speculatively — add it, matched to a real class
  // string, when a migration actually needs it.
  import type { HTMLInputAttributes } from 'svelte/elements';

  let {
    value = $bindable<string | number | undefined>(),
    class: klass = '',
    ...rest
  }: HTMLInputAttributes & {
    value?: string | number;
  } = $props();
</script>

<input
  bind:value
  class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none disabled:opacity-50 {klass}"
  {...rest}
/>
