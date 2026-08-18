<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // The plausible failure point named in the pre-implementation review:
  // Select's <option>s arrive through the {@render children?.()} snippet
  // rather than as literal children of a real <select>, so this proves
  // bind:value still resolves against options rendered that way.
  import Select from '../Select.svelte';

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

<Select bind:value {disabled} aria-label={ariaLabel}>
  <option value="scrum">Scrum</option>
  <option value="kanban">Kanban</option>
  <option value="scrumban">Scrumban</option>
</Select>
<p data-testid="readout">{value}</p>
