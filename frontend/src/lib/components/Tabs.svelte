<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Shared tab row (design doc dashboard-ia-restructuring-proposal.md
  // §4.1/C5): roving-tabindex, automatic activation, arrow-key + Home/End
  // navigation, per the W3C ARIA APG tabs pattern
  // (https://www.w3.org/WAI/ARIA/apg/patterns/tabs/). This component only
  // renders the tablist -- the consumer renders its own tabpanels and is
  // responsible for matching this component's id scheme
  // (`${idPrefix}-tab-${id}` / `${idPrefix}-panel-${id}`) in its
  // `aria-labelledby`/`id` pairing, since panel content varies too much per
  // consumer to generalize here.

  let {
    tabs,
    activeTab = $bindable(),
    idPrefix,
    label,
  }: {
    tabs: { id: string; label: string }[];
    activeTab: string;
    idPrefix: string;
    label: string;
  } = $props();

  function select(id: string) {
    activeTab = id;
  }

  function focusTab(id: string) {
    document.getElementById(`${idPrefix}-tab-${id}`)?.focus();
  }

  function handleKeydown(e: KeyboardEvent, index: number) {
    let nextIndex: number;
    switch (e.key) {
      case 'ArrowRight':
        nextIndex = (index + 1) % tabs.length;
        break;
      case 'ArrowLeft':
        nextIndex = (index - 1 + tabs.length) % tabs.length;
        break;
      case 'Home':
        nextIndex = 0;
        break;
      case 'End':
        nextIndex = tabs.length - 1;
        break;
      default:
        return;
    }
    e.preventDefault();
    const next = tabs[nextIndex];
    select(next.id);
    focusTab(next.id);
  }
</script>

<div role="tablist" aria-label={label} class="flex gap-1 border-b border-slate-800">
  {#each tabs as tab, i (tab.id)}
    <button
      type="button"
      role="tab"
      id="{idPrefix}-tab-{tab.id}"
      aria-selected={activeTab === tab.id}
      aria-controls="{idPrefix}-panel-{tab.id}"
      tabindex={activeTab === tab.id ? 0 : -1}
      onclick={() => select(tab.id)}
      onkeydown={(e) => handleKeydown(e, i)}
      class="px-4 py-2 text-xs font-bold uppercase tracking-widest border-b-2 -mb-px transition-colors {activeTab === tab.id ? 'border-cyan-400 text-cyan-300' : 'border-transparent text-slate-500 hover:text-slate-300'}"
    >
      {tab.label}
    </button>
  {/each}
</div>
