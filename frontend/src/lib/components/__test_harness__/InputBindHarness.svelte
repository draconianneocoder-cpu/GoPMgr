<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Exposes a readout of the bound value outside Input's own <input> DOM
  // node, so tests can prove the $bindable() round-trip actually reaches
  // the parent — not just that the underlying <input>'s own DOM value
  // changed, which bind:value inside Input would do even if the binding up
  // to the consumer were broken.
  import Input from '../Input.svelte';

  let {
    value = $bindable<string | number | undefined>(),
    type = undefined,
    disabled = undefined,
    class: klass = undefined,
    'aria-label': ariaLabel = undefined,
  }: {
    value?: string | number;
    type?: string;
    disabled?: boolean;
    class?: string;
    'aria-label'?: string;
  } = $props();
</script>

<Input bind:value {type} {disabled} class={klass} aria-label={ariaLabel} />
<p data-testid="readout">{value}</p>
<p data-testid="readout-type">{typeof value}</p>
