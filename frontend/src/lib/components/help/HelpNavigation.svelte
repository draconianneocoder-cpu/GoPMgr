<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { sidebar, helpIndex, type SectionId } from '../help-sections';
  import { t } from '../../i18n/i18n.svelte';

  let { active, nav }: {
    active: SectionId;
    nav: (id: SectionId, termId?: string) => void;
  } = $props();
</script>

{#if active === 'contents'}
  <h2 class="text-xl font-bold text-slate-100 mb-2">{t('help.contents.title')}</h2>
  <p class="text-sm text-slate-400 mb-5">{t('help.contents.description')}</p>
  <nav aria-label={t('help.contents.title')} class="space-y-5">
    {#each sidebar as group (group.group)}
      <section>
        <h3 class="font-semibold text-cyan-300 mb-2">{group.group}</h3>
        <ul class="space-y-2">
          {#each group.items.filter((item) => item.id !== 'contents') as item (item.id)}
            <li><button type="button" class="text-left text-cyan-400 underline hover:text-cyan-200 focus-visible:outline focus-visible:outline-2 focus-visible:outline-cyan-300" onclick={() => nav(item.id)}>{item.label}</button></li>
          {/each}
        </ul>
      </section>
    {/each}
  </nav>
{:else if active === 'subject-index'}
  <h2 class="text-xl font-bold text-slate-100 mb-2">{t('help.index.title')}</h2>
  <p class="text-sm text-slate-400 mb-5">{t('help.index.description')}</p>
  <ul class="grid gap-x-6 gap-y-2 sm:grid-cols-2">
    {#each helpIndex as entry (`${entry.kind}:${entry.label}`)}
      <li>
        <button
          type="button"
          class="w-full text-left text-cyan-400 underline hover:text-cyan-200 focus-visible:outline focus-visible:outline-2 focus-visible:outline-cyan-300"
          aria-label={entry.kind === 'definition' ? t('help.lookup.openDefinition', { term: entry.label }) : t('help.lookup.openTopic', { topic: entry.label })}
          onclick={() => nav(entry.section, entry.termId)}
        >
          {entry.label}
          <span class="text-xs text-slate-400">{entry.kind === 'definition' ? t('help.lookup.definition') : t('help.lookup.topic')}</span>
        </button>
      </li>
    {/each}
  </ul>
{/if}
