<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import Textarea from '../Textarea.svelte';

  let {
    value = $bindable<string | undefined>(),
    disabled = undefined,
    'aria-label': ariaLabel = undefined,
  }: {
    value?: string;
    disabled?: boolean;
    'aria-label'?: string;
  } = $props();
</script>

<Textarea bind:value {disabled} aria-label={ariaLabel} />
<p data-testid="readout">{value}</p>
