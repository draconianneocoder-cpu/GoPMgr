<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // DocumentCatalog is the Dashboard's controlled-document index. It consumes
  // Go registry definitions directly, keeping lifecycle phase, purpose, and
  // user-facing names aligned with the schemas used by the editor.
  import Card from '../Card.svelte';

  // expanded/query/phase are optionally bindable: a parent that keeps this
  // component's tab/section conditionally mounted (Dashboard.svelte's
  // {#if}-gated tab panels) can bind them to persist search/filter/expansion
  // state across an unmount+remount instead of it silently resetting on
  // every tab switch. A caller that doesn't bind them (this component's own
  // test file) gets the original one-time-init-from-prop behavior, since
  // $bindable()'s default is only used when the parent doesn't supply the
  // prop at all -- initiallyExpanded is already in scope by the time
  // `expanded`'s default is evaluated, since destructuring defaults run
  // left-to-right and can reference earlier bindings in the same pattern.
  let {
    definitions,
    initiallyExpanded = false,
    onCreate,
    expanded = $bindable(initiallyExpanded),
    query = $bindable(''),
    phase = $bindable('all'),
  }: {
    definitions: DocumentDefinition[];
    initiallyExpanded?: boolean;
    onCreate: (definition: DocumentDefinition) => void;
    expanded?: boolean;
    query?: string;
    phase?: string;
  } = $props();

  const lifecyclePhases = ['initiation', 'planning', 'execution', 'monitoring', 'closing'];
  const phaseRank = new Map(lifecyclePhases.map((id, index) => [id, index]));

  // Common governance documents lead broad result sets. Unrecognized future
  // kinds remain searchable and sort alphabetically after this short list.
  const preferredKinds = [
    'charter_word',
    'business_case',
    'project_plan_word',
    'status_report',
    'project_closure',
  ];
  const preferredRank = new Map(preferredKinds.map((kind, index) => [kind, index]));

  function phaseLabel(id: string): string {
    const words = id.replaceAll('_', ' ');
    return words.charAt(0).toUpperCase() + words.slice(1);
  }

  let filters = $derived([
    { id: 'all', label: 'All phases' },
    ...Array.from(new Set(definitions.map((definition) => definition.phase)))
      .sort((a, b) => {
        const aRank = phaseRank.get(a) ?? Number.MAX_SAFE_INTEGER;
        const bRank = phaseRank.get(b) ?? Number.MAX_SAFE_INTEGER;
        return aRank - bRank || a.localeCompare(b);
      })
      .map((id) => ({ id, label: phaseLabel(id) })),
  ]);

  let visibleDefinitions = $derived.by(() => {
    const needle = query.trim().toLowerCase();
    return definitions
      .filter((definition) => phase === 'all' || definition.phase === phase)
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
      // Reopening from a neutral state prevents a forgotten phase filter or
      // search from making the controlled-document index appear incomplete.
      query = '';
      phase = 'all';
    }
  }
</script>

<Card>
  <button
    type="button"
    onclick={toggle}
    aria-expanded={expanded}
    aria-controls="document-template-catalog"
    class="flex w-full items-center justify-between gap-4 px-4 py-3 text-left hover:bg-slate-800/70"
  >
    <span>
      <span class="block text-[10px] font-bold uppercase tracking-[0.18em] text-cyan-300">
        Controlled document index
      </span>
      <span class="mt-0.5 block text-sm font-semibold text-slate-100">
        {expanded ? 'Hide document templates' : `Browse ${definitions.length} document templates`}
      </span>
    </span>
    <span class="font-mono text-lg text-slate-500" aria-hidden="true">{expanded ? '−' : '+'}</span>
  </button>

  {#if expanded}
    <div id="document-template-catalog" class="border-t border-slate-800 p-4">
      <div class="flex flex-col gap-3 lg:flex-row lg:items-center">
        <label class="relative min-w-0 flex-1">
          <span class="sr-only">Search document templates</span>
          <input
            type="search"
            bind:value={query}
            placeholder="Search by name, purpose, or document kind…"
            class="w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 text-sm outline-none focus:border-cyan-500"
          />
        </label>
        <div class="flex flex-wrap gap-1.5" aria-label="Filter document templates by lifecycle phase">
          {#each filters as filter (filter.id)}
            <button
              type="button"
              onclick={() => (phase = filter.id)}
              aria-pressed={phase === filter.id}
              class="rounded border px-2.5 py-1.5 text-[11px] font-semibold
                {phase === filter.id
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
          No document templates match this search and lifecycle phase.
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
                  <span class="shrink-0 rounded border border-slate-800 px-1.5 py-0.5 text-[9px]
                    font-bold uppercase tracking-wider text-slate-500">
                    {phaseLabel(definition.phase)}
                  </span>
                </span>
                <span class="mt-1.5 block text-xs leading-relaxed text-slate-500">
                  {definition.description}
                </span>
                <span class="mt-2 block font-mono text-[10px] text-slate-500">
                  {definition.kind.replaceAll('_', '-')}
                </span>
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  {/if}
</Card>
