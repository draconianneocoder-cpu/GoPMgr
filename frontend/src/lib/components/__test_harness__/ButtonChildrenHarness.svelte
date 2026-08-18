<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Test-only harness: @testing-library/svelte's render() cannot pass
  // Svelte 5 snippet children as a prop directly, so tests that need
  // Button's default-slot content mount it through this thin wrapper
  // instead.
  import Button from '../Button.svelte';

  let {
    text,
    variant = undefined,
    size = undefined,
    type = undefined,
    disabled = undefined,
    class: klass = undefined,
    onclick = undefined,
    ...rest
  }: {
    text: string;
    variant?: 'primary' | 'secondary' | 'danger' | 'caution' | 'ghost' | 'link' | 'remove';
    size?: 'sm' | 'md' | 'lg';
    type?: 'button' | 'submit';
    disabled?: boolean;
    class?: string;
    onclick?: (e: MouseEvent) => void;
    [key: string]: unknown;
  } = $props();
</script>

<Button {variant} {size} {type} {disabled} class={klass} {onclick} {...rest}>{text}</Button>
