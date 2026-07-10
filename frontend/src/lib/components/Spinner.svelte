<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Shared loading indicator so every screen shows the same accessible
  // spinner (matching App.svelte's route-loading indicator). The spin
  // animation is disabled app-wide under prefers-reduced-motion via app.css.
  let { label = 'Loading', class: klass = 'py-12' }: { label?: string; class?: string } = $props();
</script>

<div class="flex items-center justify-center gap-3 {klass}" role="status" aria-live="polite">
  <span
    class="h-4 w-4 rounded-full border-2 border-slate-700 border-t-cyan-400 animate-spin"
    aria-hidden="true"
  ></span>
  <p class="text-xs uppercase tracking-widest text-slate-400">{label}</p>
</div>
