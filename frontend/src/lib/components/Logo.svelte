<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Theme-aware PMForge wordmark + Gantt/anvil mark, rendered inline so it
  // adapts to the light/dark theme with no file swapping: the cyan accent
  // (#00D4FF) is constant, while the timeline gridlines and the "PMForge"
  // wordmark use `currentColor` and therefore follow the surrounding text
  // colour (which flips with the theme). Background is transparent.
  //
  // Size with the `class` prop (height + a flipping text colour), e.g.
  //   <Logo class="h-6 text-slate-100" />
  // The viewBox is 4:1, so width tracks the height automatically.
  let { class: klass = 'h-7 text-slate-100' }: { class?: string } = $props();
</script>

<svg
  class={klass}
  viewBox="0 0 800 200"
  role="img"
  aria-label="PMForge"
  xmlns="http://www.w3.org/2000/svg"
>
  <g transform="translate(45,38)">
    <line x1="0" y1="0" x2="0" y2="125" stroke="currentColor" stroke-width="3" opacity="0.55" />
    <line x1="32" y1="0" x2="32" y2="125" stroke="currentColor" stroke-width="3" opacity="0.55" />
    <line x1="64" y1="0" x2="64" y2="125" stroke="currentColor" stroke-width="3" opacity="0.55" />
    <line x1="96" y1="0" x2="96" y2="125" stroke="currentColor" stroke-width="3" opacity="0.55" />
    <rect x="8" y="8" width="38" height="19" rx="4" fill="#00D4FF" />
    <rect x="8" y="34" width="58" height="19" rx="4" fill="#00D4FF" />
    <rect x="8" y="60" width="88" height="19" rx="4" fill="#00D4FF" />
    <rect x="8" y="86" width="118" height="19" rx="4" fill="#00D4FF" />
    <polygon points="8,115 28,115 38,135 88,135 98,115" fill="#00D4FF" />
  </g>
  <text
    x="225"
    y="142"
    font-family="'Arial Black', system-ui, sans-serif"
    font-size="98"
    font-weight="900"
    letter-spacing="-3.5"
    fill="currentColor">PMForge</text
  >
</svg>
