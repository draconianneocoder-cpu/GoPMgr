<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Shared button, replacing the app's ~217 hand-copied Tailwind class
  // strings (6+ divergent families, including 4 files with an entirely
  // separate hardcoded-hex CSS system) with one component backed by the
  // real distribution of what's already in use. `size` mirrors the real
  // padding buckets found app-wide (px-3 py-1 / px-3 py-2 / px-4 py-2 /
  // px-3 py-1.5), not an invented scale. `link` is exempt from both `size`
  // and the shared base classes: it's an inline underlined cross-reference
  // inside prose (no padding, no rounded, no fixed text size in the
  // original pattern), not a padded action button, so forcing it through
  // the same base would expand its box and disturb surrounding text.
  //
  // `disabled:opacity-50` lives per-variant, not in the shared base: the
  // `canvas`/`canvas-danger` variants (a distinct, consistently-reused
  // idiom for DAG-editor canvas-toolbar actions -- WorkflowEditor,
  // ActivityEditor, and, as of 2026-08-19, unmigrated but confirmed in
  // WBSEditor/CauseEffectEditor/_layered_editor_shell.svelte too) use
  // `disabled:opacity-30` instead, app-wide, with no exceptions found by
  // grep -- this is real, deliberate second idiom, not an inconsistency to
  // silently normalize away. `canvas` is visually identical to `secondary`
  // except in the disabled state; that's a real discoverability cost
  // (nothing stops a future call site from reaching for the wrong one) and
  // is accepted, not solved, here -- the alternative (a `disabledOpacity`
  // prop) would make invalid combinations like `variant="primary"
  // disabledOpacity="30"` representable when no design intent in this
  // codebase supports them.
  import type { Snippet } from 'svelte';
  import type { HTMLButtonAttributes } from 'svelte/elements';

  let {
    variant = 'secondary',
    size = 'md',
    type = 'button',
    disabled = false,
    class: klass = '',
    onclick,
    children,
    ...rest
  }: Omit<HTMLButtonAttributes, 'type' | 'class' | 'disabled' | 'onclick'> & {
    variant?:
      | 'primary'
      | 'secondary'
      | 'danger'
      | 'caution'
      | 'ghost'
      | 'canvas'
      | 'canvas-danger'
      | 'link'
      | 'remove'
      | 'nav';
    size?: 'sm' | 'compact' | 'md' | 'lg';
    type?: 'button' | 'submit';
    disabled?: boolean;
    class?: string;
    onclick?: (e: MouseEvent) => void;
    children?: Snippet;
  } = $props();

  const sizeClasses: Record<'sm' | 'compact' | 'md' | 'lg', string> = {
    sm: 'px-3 py-1',
    compact: 'px-3 py-1.5',
    md: 'px-3 py-2',
    lg: 'px-4 py-2',
  };

  // danger/caution intentionally omit `uppercase` — ConfirmDialog's original
  // hand-written buttons (the dogfooding call site) never had it, unlike
  // the primary/cyan family which does app-wide; adding it here would
  // silently change ConfirmDialog's already-live-verified appearance.
  const variantClasses: Record<Exclude<typeof variant, 'link' | 'remove' | 'nav'>, string> = {
    primary: 'bg-cyan-600 hover:bg-cyan-500 text-white font-bold uppercase disabled:opacity-50',
    secondary: 'bg-slate-800 hover:bg-slate-700 disabled:opacity-50',
    danger: 'bg-red-700 hover:bg-red-600 text-white font-bold disabled:opacity-50',
    caution: 'bg-amber-700 hover:bg-amber-600 text-white font-bold disabled:opacity-50',
    ghost: 'text-slate-300 hover:bg-slate-800 disabled:opacity-50',
    // The canvas-toolbar idiom: same idle background as `secondary`, but
    // disabled state dims to 30% opacity, not 50 -- confirmed by grep as a
    // consistent, app-wide-reused pattern (WorkflowEditor, ActivityEditor,
    // WBSEditor, CauseEffectEditor, _layered_editor_shell.svelte all use
    // this exact opacity for canvas selection-dependent toolbar actions),
    // not a one-off. `canvas-danger` is the same idiom for a
    // destructive-in-context action (e.g. "Delete node") -- distinct from
    // `danger`, which is a solid-red, always-visible CTA (ConfirmDialog's
    // Confirm button), not a toolbar action gated on canvas selection.
    canvas: 'bg-slate-800 hover:bg-slate-700 disabled:opacity-30',
    'canvas-danger': 'bg-slate-800 hover:bg-red-900 disabled:opacity-30',
  };
</script>

{#if variant === 'link'}
  <button {type} {disabled} {onclick} class="text-cyan-400 underline hover:text-cyan-300 disabled:opacity-50 {klass}" {...rest}>
    {@render children?.()}
  </button>
{:else if variant === 'remove'}
  <!-- Inline icon-glyph buttons (the app's 21 aria-label="Remove …" ×
       buttons across 14 chart editors) — exempt from the shared base like
       `link`, since none of those call sites have `rounded`/`text-xs`/
       padding in their original hand-written classes; folding this into
       `variantClasses` instead would silently add padding to all 21. -->
  <button {type} {disabled} {onclick} class="text-slate-500 hover:text-red-400 disabled:opacity-50 {klass}" {...rest}>
    {@render children?.()}
  </button>
{:else if variant === 'nav'}
  <!-- The "&larr; Dashboard" header-nav idiom: 28 grep-confirmed call sites
       app-wide, all `text-xs text-slate-400 hover:text-cyan-400` with no
       padding/rounded — exempt from the shared base for the same reason as
       `link`/`remove`. None of the 28 pass `disabled`, so
       `disabled:opacity-50` here is untested by any live call site, but
       included for parity with `link`/`remove` rather than left silently
       broken if a future caller does pass it. Distinct from `link`
       (cyan-400 + underline, a different color contract) — a further 3
       call sites (AppHeader "Sign out", Dashboard "Settings"/"Close
       project") add `underline` to this same slate/cyan pair; that's a
       third, rarer idiom, deliberately left unmigrated rather than folded
       in here. -->
  <button {type} {disabled} {onclick} class="text-xs text-slate-400 hover:text-cyan-400 disabled:opacity-50 {klass}" {...rest}>
    {@render children?.()}
  </button>
{:else}
  <button
    {type}
    {disabled}
    {onclick}
    class="rounded text-xs {sizeClasses[size]} {variantClasses[variant]} {klass}"
    {...rest}
  >
    {@render children?.()}
  </button>
{/if}
