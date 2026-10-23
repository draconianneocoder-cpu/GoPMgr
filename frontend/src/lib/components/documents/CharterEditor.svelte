<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import { autosave } from '../../autosave.svelte';
  import { rebaseEditableChanges } from '../../rebase-editable-changes';
  import { showToast } from '../../toast.svelte';
  import SignCertificateModal from '../SignCertificateModal.svelte';
  import DocumentFieldEditor from './DocumentFieldEditor.svelte';
  import Button from '../Button.svelte';

  let doc = $state<DocumentRecord | null>(null);
  let definition = $state<DocumentDefinition | null>(null);
  let content = $state<Record<string, unknown>>({});
  // The project's reporting currency, shown after amount labels.
  let currency = $state('');
  let status = $state('');
  let saving = $state(false);
  let syncingRiskMatrix = $state(false);

  let lastSavedSnapshot = $state<string | null>(null);
  let dirty = $derived(lastSavedSnapshot !== null && snapshot() !== lastSavedSnapshot);

  function snapshot(): string {
    return JSON.stringify({ c: content, t: doc?.title, s: doc?.status });
  }

  const docStatuses = ['draft', 'review', 'approved', 'archived'] as const;
  const linkedRiskMatrixId = $derived(
    doc?.kind === 'risk_register' && typeof content.risk_matrix_ref === 'string'
      ? content.risk_matrix_ref.trim()
      : '',
  );

  function handleKeyDown(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key === 's') {
      e.preventDefault();
      save();
    }
  }

  let stopAutosave: (() => void) | null = null;
  let active = true;

  onMount(async () => {
    window.addEventListener('keydown', handleKeyDown);
    if (!session.editingId) return;
    try {
      const loaded = await window.go.main.App.GetDocument(session.editingId);
      if (!active) return;
      doc = loaded;
      try {
        content = JSON.parse(loaded.content);
      } catch {
        content = {};
      }
      lastSavedSnapshot = snapshot();
      const all = await window.go.main.App.ListDocumentKinds();
      if (!active) return;
      definition = all.find((d) => d.kind === loaded.kind) ?? null;
      // charter_excel and plan_excel carry empty fields; borrow the paired _word schema.
      if (definition && definition.fields.length === 0 && loaded.kind.endsWith('_excel')) {
        const wordKind = loaded.kind.replace('_excel', '_word');
        definition = all.find((d) => d.kind === wordKind) ?? definition;
      }
      stopAutosave = autosave.register(snapshot, persist);
      try {
        const meta = await window.go.main.App.GetProjectMeta();
        if (active) currency = meta.currency_code ?? '';
      } catch {
        // Amount labels then name no currency; editing is unaffected.
      }
    } catch (err: unknown) {
      if (active) status = `Load failed: ${err}`;
    }
  });

  onDestroy(() => {
    active = false;
    window.removeEventListener('keydown', handleKeyDown);
    stopAutosave?.();
  });

  // Every write shares the coordinator's gate/baseline. Export and sync also
  // require the live draft to match the acknowledged write, not a late edit.
  async function save(): Promise<boolean> {
    return (await autosave.saveRegistered(snapshot)) && active && !dirty;
  }

  async function persist(): Promise<boolean> {
    if (!active || !doc || saving) return false;
    const submission = { ...doc, content: JSON.stringify(content) };
    saving = true;
    status = '';
    try {
      const updated = await window.go.main.App.SaveDocument(submission);
      if (!active) return false;
      const acknowledgedContent = JSON.parse(updated.content);
      doc = rebaseEditableChanges(updated, submission, { ...doc, content: JSON.stringify(content) },
        ['id', 'project_id', 'kind', 'template_id', 'version', 'created_at', 'updated_at']);
      content = JSON.parse(doc.content);
      lastSavedSnapshot = JSON.stringify({ c: acknowledgedContent, t: updated.title, s: updated.status });
      status = dirty ? 'Saved earlier changes. Newer edits remain unsaved.'
        : `Saved. Version ${updated.version} at ${new Date().toLocaleTimeString()}.`;
      return true;
    } catch (err: any) {
      if (active) status = `Save failed: ${err}`;
      return false;
    } finally {
      if (active) saving = false;
    }
  }

  async function syncRiskMatrix() {
    if (!doc || !linkedRiskMatrixId) return;
    if (!(await save())) return;
    syncingRiskMatrix = true;
    try {
      const chart = await window.go.main.App.SyncRiskRegisterToMatrix(doc.id);
      status = `Risk Matrix “${chart.title}” refreshed from this register.`;
      showToast('Linked Risk Matrix refreshed', 'success');
    } catch (err: any) {
      status = `Risk Matrix sync failed: ${err?.message ?? err}`;
    } finally {
      syncingRiskMatrix = false;
    }
  }

  async function exportPDF() {
    if (!doc) return;
    status = '';
    try {
      if (!(await save())) return;
      const path = await window.go.main.App.ExportDocumentPDF(doc.id);
      status = `Exported to ${path}`;
    } catch (err: any) {
      if (String(err).includes('export cancelled')) return;
      status = `Export failed: ${err}`;
    }
  }

  async function exportDOCX() {
    if (!doc) return;
    status = '';
    try {
      if (!(await save())) return;
      const path = await window.go.main.App.ExportDocumentDOCX(doc.id);
      status = `Exported to ${path}`;
    } catch (err: any) {
      if (String(err).includes('export cancelled')) return;
      status = `Export failed: ${err}`;
    }
  }

  async function exportODT() {
    if (!doc) return;
    status = '';
    try {
      if (!(await save())) return;
      const path = await window.go.main.App.ExportDocumentODT(doc.id);
      status = `Exported to ${path}`;
    } catch (err: any) {
      if (String(err).includes('export cancelled')) return;
      status = `Export failed: ${err}`;
    }
  }

  let certPathForSign = $state('');
  let gpgKeyIDForSign = $state('');
  let preferredSignatureMethod = $state<SignatureMethod>('pades');
  let signing = $state(false);
  let showSignModal = $state(false);
  let pendingCertPath = $state('');
  let pendingGPGKeyID = $state('');
  let pendingSignatureMethod = $state<SignatureMethod>('pades');
  let signatureSettingsLoaded = $state(false);

  async function exportSignedPDF() {
    if (!doc) return;
    status = '';
    signing = true;
    try {
      if (!(await save())) {
        signing = false;
        return;
      }

      if (!signatureSettingsLoaded) {
        try {
          const s = await window.go.main.App.GetSettings();
          if (s?.cert_path) certPathForSign = s.cert_path;
          if (s?.signature_method) preferredSignatureMethod = s.signature_method;
          if (s?.gpg_key_id) gpgKeyIDForSign = s.gpg_key_id;
          signatureSettingsLoaded = true;
        } catch {
          /* ignore */
        }
      }

      pendingCertPath = certPathForSign;
      pendingGPGKeyID = gpgKeyIDForSign;
      pendingSignatureMethod = preferredSignatureMethod;
      showSignModal = true;
      signing = false; // modal will handle the actual signing
    } catch (err: any) {
      status = `Signed export failed: ${err}`;
      signing = false;
    }
  }

  function handleSignedConfirm(options: SignatureExportOptions) {
    showSignModal = false;
    if (!doc) return;

    signing = true;
    (async () => {
      try {
        if (options.method === 'pades') {
          const path = await window.go.main.App.ExportDocumentPDFSigned(
            doc.id,
            options.cert_path,
            options.cert_password,
          );
          status = `PAdES signed PDF exported to ${path}`;
          showToast(`PAdES signed PDF exported successfully`, 'success');
        } else if (options.method === 'gpg') {
          const result = await window.go.main.App.ExportDocumentPDFGnuPG(
            doc.id,
            options.gpg_key_id,
          );
          status = `PDF exported to ${result.pdf_path}; GnuPG signature exported to ${result.signature_path}`;
          showToast(`GnuPG detached signature exported successfully`, 'success');
        } else {
          const path = await window.go.main.App.ExportDocumentPDF(doc.id);
          status = `PDF exported without digital signature to ${path}`;
          showToast(`PDF exported without digital signature`, 'success');
        }
      } catch (err: any) {
        if (String(err).includes('export cancelled')) return;
        status = `Export failed: ${err}`;
      } finally {
        signing = false;
        pendingCertPath = '';
        pendingGPGKeyID = '';
      }
    })();
  }
</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <Button variant="nav" onclick={() => goto('dashboard')}>
        &larr; Dashboard
      </Button>
      <h1 class="text-sm font-bold tracking-widest uppercase text-slate-50">
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
      {#if doc?.kind === 'risk_register'}
        <button
          onclick={syncRiskMatrix}
          disabled={!linkedRiskMatrixId || syncingRiskMatrix || saving}
          title={linkedRiskMatrixId ? 'Replace the linked matrix items with this saved register' : 'Select a Linked Risk Matrix in the document first'}
          class="text-xs bg-amber-800 hover:bg-amber-700 disabled:opacity-40 text-white px-3 py-1 rounded"
        >
          {syncingRiskMatrix ? 'Syncing...' : 'Refresh Risk Matrix'}
        </button>
      {/if}
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
        class="text-xs bg-emerald-800 hover:bg-emerald-700 disabled:opacity-50 text-white px-3 py-1 rounded"
        title="Choose PAdES, GnuPG, or no digital signature"
      >
        {signing ? 'Exporting…' : 'Signature Options'}
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
        <DocumentFieldEditor {field} {currency} bind:value={content[field.key]} />
      {/each}

      <!-- Digital signature action next to human sign-off fields -->
      <div class="mt-6 p-4 border border-emerald-800 bg-emerald-950/30 rounded">
        <div class="text-xs uppercase tracking-widest text-emerald-400 mb-2">Digital Signature</div>
        <p class="text-xs text-slate-400 mb-3">
          Export with an embedded PAdES signature, a detached GnuPG signature, or no digital signature for physical sign-off.
        </p>
        <button
          onclick={exportSignedPDF}
          disabled={signing}
          class="text-xs bg-emerald-800 hover:bg-emerald-700 disabled:opacity-50 text-white font-bold uppercase px-4 py-1.5 rounded"
        >
          {signing ? 'Exporting…' : 'Choose Signature & Export'}
        </button>
      </div>
    {:else}
      <p class="text-sm text-slate-500">Loading...</p>
    {/if}
  </main>

  <SignCertificateModal
    bind:open={showSignModal}
    certPath={pendingCertPath}
    method={pendingSignatureMethod}
    gpgKeyID={pendingGPGKeyID}
    onConfirm={handleSignedConfirm}
  />
</div>
