<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';

  // Bobby's supplied lockups are portrait compositions, while the header
  // needs a compact square mark. The two variants deliberately use distinct
  // theme assets instead of squeezing a full lockup into the toolbar.
  let {
    class: klass = 'h-32 w-auto',
    variant = 'full',
  }: { class?: string; variant?: 'compact' | 'full' } = $props();

  function currentTheme(): 'dark' | 'light' {
    if (typeof document === 'undefined') return 'dark';
    return document.documentElement.dataset.theme === 'light' ? 'light' : 'dark';
  }

  let theme = $state<'dark' | 'light'>(currentTheme());

  onMount(() => {
    const root = document.documentElement;
    const syncTheme = () => {
      theme = root.dataset.theme === 'light' ? 'light' : 'dark';
    };
    const observer = new MutationObserver(syncTheme);
    syncTheme();
    observer.observe(root, { attributes: true, attributeFilter: ['data-theme'] });
    return () => observer.disconnect();
  });

  const asset = $derived(
    variant === 'compact'
      ? `/branding/gopmgr-app-icon-${theme}.png`
      : `/branding/gopmgr-logo-lockup-${theme}.png`,
  );
  const compactSrcset = $derived(
    `/branding/gopmgr-app-icon-${theme}-128.png 128w, /branding/gopmgr-app-icon-${theme}.png 512w`,
  );
</script>

<img
  class={klass}
  src={asset}
  srcset={variant === 'compact' ? compactSrcset : undefined}
  sizes={variant === 'compact' ? '32px' : undefined}
  alt="GoPMgr, featuring Bobby Beaver"
/>
