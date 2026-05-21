<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import DocumentFieldEditor from './DocumentFieldEditor.svelte';

  let doc = $state<DocumentRecord | null>(null);
  let definition = $state<DocumentDefinition | null>(null);
  let content = $state<Record<string, unknown>>({});
  let status = $state('');
  let saving = $state(false);

  onMount(async () => {
    if (!session.editingId) return;
    doc = await window.go.main.App.GetDocument(session.editingId);
    try {
      content = JSON.parse(doc.content);
    } catch {
      content = {};
    }
    // Look up the charter definition (works for both Word and Excel variants).
    const all = await window.go.main.App.ListDocumentKinds();
    definition = all.find((d) => d.kind === doc!.kind) ?? null;
    // If the kind is the Excel alias, fall back to the Word kind's schema.
    if (definition && definition.fields.length === 0) {
      definition = all.find((d) => d.kind === 'charter_word') ?? definition;
    }
  });

  async function save() {
    if (!doc) return;
    saving = true;
    status = '';
    try {
      const updated = await window.go.main.App.SaveDocument({
        ...doc,
        content: JSON.stringify(content),
      });
      doc = updated;
      status = `Saved. Version ${updated.version} at ${new Date().toLocaleTimeString()}.`;
    } catch (err: any) {
      status = `Save failed: ${err}`;
    } finally {
      saving = false;
    }
  }

  async function exportPDF() {
    if (!doc) return;
    status = '';
    try {
      // Save first so the export reflects the latest edits.
      await save();
      const path = await window.go.main.App.ExportDocumentPDF(doc.id);
      status = `Exported to ${path}`;
    } catch (err: any) {
      status = `Export failed: ${err}`;
    }
  }
</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <button onclick={() => goto('dashboard')} class="text-xs text-slate-400 hover:text-cyan-400">
        &larr; Dashboard
      </button>
      <h1 class="text-sm font-bold tracking-widest uppercase text-white">
        {definition?.name ?? 'Document'}
      </h1>
      {#if doc}
        <span class="text-xs text-slate-500">v{doc.version} · {doc.status}</span>
      {/if}
    </div>
    <div class="flex items-center gap-2">
      <button onclick={exportPDF} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded">
        Export PDF
      </button>
      <button
        onclick={save}
        disabled={saving}
        class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-3 py-1 rounded"
      >
        {saving ? 'Saving...' : 'Save'}
      </button>
    </div>
  </header>

  <main class="max-w-3xl mx-auto p-8 space-y-6">
    {#if status}
      <p class="text-xs text-cyan-400" role="status">{status}</p>
    {/if}

    {#if definition}
      <p class="text-xs text-slate-500">{definition.description}</p>

      <label class="block">
        <span class="text-xs font-semibold text-slate-500 uppercase">Title</span>
        <input
          bind:value={doc!.title}
          class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        />
      </label>

      {#each definition.fields as field (field.key)}
        <DocumentFieldEditor {field} bind:value={content[field.key]} />
      {/each}
    {:else}
      <p class="text-sm text-slate-500">Loading...</p>
    {/if}
  </main>
</div>
