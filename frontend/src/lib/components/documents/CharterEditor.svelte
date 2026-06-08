<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import { showToast } from '../../toast';
  import SignCertificateModal from '../SignCertificateModal.svelte';
  import DocumentFieldEditor from './DocumentFieldEditor.svelte';

  let doc = $state<DocumentRecord | null>(null);
  let definition = $state<DocumentDefinition | null>(null);
  let content = $state<Record<string, unknown>>({});
  let status = $state('');
  let saving = $state(false);

  let lastSavedContent = $state<string | null>(null);
  let lastSavedTitle = $state<string | null>(null);
  let dirty = $derived(
    lastSavedContent !== null &&
      (JSON.stringify(content) !== lastSavedContent || doc?.title !== lastSavedTitle),
  );

  const docStatuses = ['draft', 'review', 'approved', 'archived'] as const;

  function handleKeyDown(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key === 's') {
      e.preventDefault();
      save();
    }
  }

  onMount(async () => {
    window.addEventListener('keydown', handleKeyDown);
    if (!session.editingId) return;
    doc = await window.go.main.App.GetDocument(session.editingId);
    try {
      content = JSON.parse(doc.content);
    } catch {
      content = {};
    }
    lastSavedContent = JSON.stringify(content);
    lastSavedTitle = doc.title;
    const all = await window.go.main.App.ListDocumentKinds();
    definition = all.find((d) => d.kind === doc!.kind) ?? null;
    // charter_excel and plan_excel carry empty fields — borrow the paired _word schema.
    if (definition && definition.fields.length === 0 && doc!.kind.endsWith('_excel')) {
      const wordKind = doc!.kind.replace('_excel', '_word');
      definition = all.find((d) => d.kind === wordKind) ?? definition;
    }
  });

  onDestroy(() => {
    window.removeEventListener('keydown', handleKeyDown);
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
      lastSavedContent = JSON.stringify(content);
      lastSavedTitle = updated.title;
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
      await save();
      const path = await window.go.main.App.ExportDocumentPDF(doc.id);
      status = `Exported to ${path}`;
    } catch (err: any) {
      status = `Export failed: ${err}`;
    }
  }

  async function exportDOCX() {
    if (!doc) return;
    status = '';
    try {
      await save();
      const path = await window.go.main.App.ExportDocumentDOCX(doc.id);
      status = `Exported to ${path}`;
    } catch (err: any) {
      status = `Export failed: ${err}`;
    }
  }

  async function exportODT() {
    if (!doc) return;
    status = '';
    try {
      await save();
      const path = await window.go.main.App.ExportDocumentODT(doc.id);
      status = `Exported to ${path}`;
    } catch (err: any) {
      status = `Export failed: ${err}`;
    }
  }

  let certPathForSign = $state('');
  let signing = $state(false);
  let showSignModal = $state(false);
  let pendingCertPath = $state('');

  async function exportSignedPDF() {
    if (!doc) return;
    status = '';
    signing = true;
    try {
      await save();

      if (!certPathForSign) {
        try {
          const s = await window.go.main.App.GetSettings();
          if (s?.cert_path) certPathForSign = s.cert_path;
        } catch {
          /* ignore */
        }
      }

      pendingCertPath = certPathForSign;
      showSignModal = true;
      signing = false; // modal will handle the actual signing
    } catch (err: any) {
      status = `Signed export failed: ${err}`;
      signing = false;
    }
  }

  function handleSignedConfirm(pwd: string) {
    showSignModal = false;
    if (!pendingCertPath || !pwd || !doc) return;

    signing = true;
    (async () => {
      try {
        const path = await window.go.main.App.ExportDocumentPDFSigned(
          doc.id,
          pendingCertPath,
          pwd,
        );
        status = `Signed PDF exported to ${path}`;
        showToast(`Signed PDF exported successfully`, 'success');
      } catch (err: any) {
        status = `Signed export failed: ${err}`;
      } finally {
        signing = false;
        pendingCertPath = '';
      }
    })();
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
        <span class="text-xs text-slate-500">v{doc.version}</span>
        <select
          bind:value={doc.status}
          onchange={() => save()}
          class="text-xs bg-slate-800 border border-slate-700 text-slate-300 px-2 py-0.5 rounded"
        >
          {#each docStatuses as s (s)}
            <option value={s}>{s}</option>
          {/each}
        </select>
        {#if dirty}
          <span class="text-xs text-amber-400 font-semibold">Unsaved changes</span>
        {/if}
      {/if}
    </div>
    <div class="flex items-center gap-2">
      <button onclick={exportDOCX} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded">
        Export DOCX
      </button>
      <button onclick={exportODT} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded">
        Export ODT
      </button>
      <button onclick={exportPDF} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded">
        Export PDF
      </button>
      <button
        onclick={exportSignedPDF}
        disabled={signing}
        class="text-xs bg-emerald-800 hover:bg-emerald-700 disabled:opacity-50 px-3 py-1 rounded"
        title="Export with an embedded PAdES B-B digital signature"
      >
        {signing ? 'Signing…' : 'Export Signed PDF'}
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

      <!-- Digital signature action next to human sign-off fields -->
      <div class="mt-6 p-4 border border-emerald-800 bg-emerald-950/30 rounded">
        <div class="text-xs uppercase tracking-widest text-emerald-400 mb-2">Digital Signature</div>
        <p class="text-xs text-slate-400 mb-3">
          Apply an embedded PAdES B-B digital signature with a tamper-evident ByteRange.
        </p>
        <button
          onclick={exportSignedPDF}
          disabled={signing}
          class="text-xs bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white font-bold uppercase px-4 py-1.5 rounded"
        >
          {signing ? 'Signing…' : 'Sign & Export PDF'}
        </button>
      </div>
    {:else}
      <p class="text-sm text-slate-500">Loading...</p>
    {/if}
  </main>

  <SignCertificateModal
    bind:open={showSignModal}
    certPath={pendingCertPath}
    onConfirm={handleSignedConfirm}
  />
</div>
