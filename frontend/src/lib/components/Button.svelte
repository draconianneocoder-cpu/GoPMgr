<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Shared button, replacing the app's ~217 hand-copied Tailwind class
  // strings (6+ divergent families, including 4 files with an entirely
  // separate hardcoded-hex CSS system) with one component backed by the
  // real distribution of what's already in use. `size` mirrors the three
  // largest padding buckets found app-wide (px-3 py-1 / px-3 py-2 /
  // px-4 py-2), not an invented scale. `link` is exempt from both `size`
  // and the shared base classes: it's an inline underlined cross-reference
  // inside prose (no padding, no rounded, no fixed text size in the
  // original pattern), not a padded action button, so forcing it through
  // the same base would expand its box and disturb surrounding text.
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
    variant?: 'primary' | 'secondary' | 'danger' | 'caution' | 'ghost' | 'link' | 'remove';
    size?: 'sm' | 'md' | 'lg';
    type?: 'button' | 'submit';
    disabled?: boolean;
    class?: string;
    onclick?: (e: MouseEvent) => void;
    children?: Snippet;
  } = $props();

  const sizeClasses: Record<'sm' | 'md' | 'lg', string> = {
    sm: 'px-3 py-1',
    md: 'px-3 py-2',
    lg: 'px-4 py-2',
  };

  // danger/caution intentionally omit `uppercase` — ConfirmDialog's original
  // hand-written buttons (the dogfooding call site) never had it, unlike
  // the primary/cyan family which does app-wide; adding it here would
  // silently change ConfirmDialog's already-live-verified appearance.
  const variantClasses: Record<Exclude<typeof variant, 'link' | 'remove'>, string> = {
    primary: 'bg-cyan-600 hover:bg-cyan-500 text-white font-bold uppercase',
    secondary: 'bg-slate-800 hover:bg-slate-700',
    danger: 'bg-red-700 hover:bg-red-600 text-white font-bold',
    caution: 'bg-amber-700 hover:bg-amber-600 text-white font-bold',
    ghost: 'text-slate-300 hover:bg-slate-800',
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
{:else}
  <button
    {type}
    {disabled}
    {onclick}
    class="rounded text-xs disabled:opacity-50 {sizeClasses[size]} {variantClasses[variant]} {klass}"
    {...rest}
  >
    {@render children?.()}
  </button>
{/if}
