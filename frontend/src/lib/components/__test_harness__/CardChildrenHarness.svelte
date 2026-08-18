<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import Card from '../Card.svelte';

  let { text, class: klass = undefined }: { text: string; class?: string } = $props();
</script>

<Card class={klass}>
  <p>{text}</p>
</Card>
