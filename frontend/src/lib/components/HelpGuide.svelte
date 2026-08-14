<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import AppHeader from './AppHeader.svelte';
  import HelpOverview from './help/HelpOverview.svelte';
  import HelpTutorials from './help/HelpTutorials.svelte';
  import HelpMethodologies from './help/HelpMethodologies.svelte';
  import HelpFeatures from './help/HelpFeatures.svelte';
  import HelpReference from './help/HelpReference.svelte';
  import HelpTroubleshooting from './help/HelpTroubleshooting.svelte';
  import { sidebar, KEYWORDS, type SectionId } from './help-sections';

  let active = $state<SectionId>('getting-started');

  function nav(id: SectionId) {
    active = id;
  }

  // ── Search ────────────────────────────────────────────────────────
  // sidebar and KEYWORDS live in help-sections.ts, shared with the six
  // Help*.svelte content components below -- see that file's own comment
  // for why the search index stays centralized here rather than splitting
  // per component.
  let query = $state('');

  const filteredSidebar = $derived.by(() => {
    const q = query.trim().toLowerCase();
    if (!q) return sidebar;
    return sidebar
      .map((group) => ({
        group: group.group,
        items: group.items.filter(
          (item) =>
            item.label.toLowerCase().includes(q) ||
            group.group.toLowerCase().includes(q) ||
            KEYWORDS[item.id].includes(q),
        ),
      }))
      .filter((g) => g.items.length > 0);
  });

  const matchCount = $derived(filteredSidebar.reduce((n, g) => n + g.items.length, 0));
</script>

<div class="min-h-screen bg-slate-950 text-slate-100 flex flex-col">
  <AppHeader active="help" />

  <div class="flex flex-1 overflow-hidden">
    <!-- Sidebar -->
    <nav
      class="w-52 shrink-0 border-r border-slate-800 overflow-y-auto py-4 px-2"
      aria-label="Help sections"
    >
      <div class="px-2 mb-4">
        <input
          type="search"
          bind:value={query}
          placeholder="Search help…"
          aria-label="Search help sections"
          class="w-full bg-slate-900 border border-slate-800 rounded px-2 py-1.5 text-xs text-slate-200 placeholder:text-slate-600 focus:border-cyan-500 outline-none"
        />
        {#if query.trim()}
          <p class="mt-1.5 text-[10px] text-slate-500" role="status">
            {matchCount === 0
              ? 'No sections match.'
              : `${matchCount} section${matchCount === 1 ? '' : 's'} match.`}
          </p>
        {/if}
      </div>
      {#each filteredSidebar as group}
        <div class="mb-5">
          <p class="px-2 mb-1 text-[10px] font-bold uppercase tracking-widest text-slate-500">
            {group.group}
          </p>
          {#each group.items as item}
            <button
              onclick={() => nav(item.id)}
              class={`w-full text-left px-3 py-1.5 rounded text-xs font-medium transition-colors ${
                active === item.id
                  ? 'bg-slate-800 text-cyan-400'
                  : 'text-slate-400 hover:text-slate-100 hover:bg-slate-800/50'
              }`}
            >
              {item.label}
            </button>
          {/each}
        </div>
      {/each}
    </nav>

    <!-- Content -->
    <main class="flex-1 overflow-y-auto">
      <div class="max-w-3xl mx-auto px-8 py-6">
        <h1 class="sr-only">Help</h1>

        <HelpOverview {active} {nav} />
        <HelpTutorials {active} {nav} />
        <HelpMethodologies {active} {nav} />
        <HelpFeatures {active} {nav} />
        <HelpReference {active} {nav} />
        <HelpTroubleshooting {active} {nav} />
      </div>
    </main>
  </div>
</div>
