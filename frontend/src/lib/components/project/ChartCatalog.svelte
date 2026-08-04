<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // ChartCatalog is a compact "tool crib" for the Dashboard. Definitions come
  // from Go's chart registry, so names, descriptions, and engine membership
  // cannot drift from the renderers that actually support them.
  let {
    definitions,
    initiallyExpanded = false,
    onCreate,
  }: {
    definitions: ChartDefinition[];
    initiallyExpanded?: boolean;
    onCreate: (definition: ChartDefinition) => void;
  } = $props();

  // Read the prop through a closure to make the one-time initialization
  // explicit to Svelte: parent changes must not unexpectedly reopen a catalog
  // that the user deliberately collapsed.
  const initialExpansion = () => initiallyExpanded;
  let expanded = $state(initialExpansion());
  let query = $state('');
  let engine = $state('all');

  const engineLabels: Record<string, string> = {
    dag: 'Schedules and networks',
    stats: 'Metrics and trends',
    matrix: 'Matrices and responsibility',
    flow: 'Process flows',
  };

  function engineLabel(id: string): string {
    const fallback = id.replaceAll('_', ' ');
    return engineLabels[id] ?? fallback.charAt(0).toUpperCase() + fallback.slice(1);
  }

  // Registry order is domain-oriented, while this short priority list puts
  // common project-control tools first within filtered results. Unknown future
  // kinds remain available and sort alphabetically after the known set.
  const preferredKinds = ['wbs', 'cpm', 'gantt', 'pert', 'network'];
  const preferredRank = new Map(preferredKinds.map((kind, index) => [kind, index]));

  let filters = $derived([
    { id: 'all', label: 'All tools' },
    ...Array.from(new Set(definitions.map((definition) => definition.engine))).map((id) => ({
      id,
      label: engineLabel(id),
    })),
  ]);

  let visibleDefinitions = $derived.by(() => {
    const needle = query.trim().toLowerCase();
    return definitions
      .filter((definition) => engine === 'all' || definition.engine === engine)
      .filter((definition) => {
        if (!needle) return true;
        return `${definition.name} ${definition.description} ${definition.kind}`
          .toLowerCase()
          .includes(needle);
      })
      .sort((a, b) => {
        const aRank = preferredRank.get(a.kind) ?? Number.MAX_SAFE_INTEGER;
        const bRank = preferredRank.get(b.kind) ?? Number.MAX_SAFE_INTEGER;
        return aRank - bRank || a.name.localeCompare(b.name);
      });
  });

  function toggle() {
    expanded = !expanded;
    if (!expanded) {
      // A collapsed catalog should reopen in a predictable state instead of
      // appearing empty because an old search or engine filter was retained.
      query = '';
      engine = 'all';
    }
  }
</script>

<div class="overflow-hidden rounded-lg border border-slate-800 bg-slate-900/60">
  <button
    type="button"
    onclick={toggle}
    aria-expanded={expanded}
    aria-controls="chart-tool-catalog"
    class="flex w-full items-center justify-between gap-4 px-4 py-3 text-left hover:bg-slate-800/70"
  >
    <span>
      <span class="block text-[10px] font-bold uppercase tracking-[0.18em] text-cyan-300">
        Precision tool catalog
      </span>
      <span class="mt-0.5 block text-sm font-semibold text-slate-100">
        {expanded ? 'Hide chart tools' : `Browse ${definitions.length} chart tools`}
      </span>
    </span>
    <span class="font-mono text-lg text-slate-500" aria-hidden="true">{expanded ? '−' : '+'}</span>
  </button>

  {#if expanded}
    <div id="chart-tool-catalog" class="border-t border-slate-800 p-4">
      <div class="flex flex-col gap-3 lg:flex-row lg:items-center">
        <label class="relative min-w-0 flex-1">
          <span class="sr-only">Search chart tools</span>
          <input
            type="search"
            bind:value={query}
            placeholder="Search by name, use, or chart kind…"
            class="w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 text-sm outline-none focus:border-cyan-500"
          />
        </label>
        <div class="flex flex-wrap gap-1.5" aria-label="Filter chart tools by engine">
          {#each filters as filter (filter.id)}
            <button
              type="button"
              onclick={() => (engine = filter.id)}
              aria-pressed={engine === filter.id}
              class="rounded border px-2.5 py-1.5 text-[11px] font-semibold
                {engine === filter.id
                  ? 'border-cyan-600 bg-slate-900 text-cyan-300'
                  : 'border-slate-700 bg-slate-900 text-slate-400 hover:border-slate-600 hover:text-slate-200'}"
            >
              {filter.label}
            </button>
          {/each}
        </div>
      </div>

      {#if visibleDefinitions.length === 0}
        <p class="py-8 text-center text-sm text-slate-500" role="status">
          No chart tools match this search and engine filter.
        </p>
      {:else}
        <ul class="mt-4 grid grid-cols-1 gap-2 md:grid-cols-2 xl:grid-cols-3">
          {#each visibleDefinitions as definition (definition.kind)}
            <li>
              <button
                type="button"
                onclick={() => onCreate(definition)}
                class="group h-full w-full rounded border border-slate-800 bg-slate-950/70 p-3 text-left
                  hover:border-cyan-800 hover:bg-slate-900"
              >
                <span class="flex items-start justify-between gap-3">
                  <span class="font-semibold text-slate-100 group-hover:text-cyan-300">
                    {definition.name}
                  </span>
                  <span class="shrink-0 font-mono text-[10px] uppercase tracking-wider text-slate-600">
                    {definition.kind.replace('_', '-')}
                  </span>
                </span>
                <span class="mt-1.5 block text-xs leading-relaxed text-slate-500">
                  {definition.description}
                </span>
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  {/if}
</div>
