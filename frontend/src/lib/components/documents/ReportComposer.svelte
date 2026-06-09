<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // ReportComposer lets a user assemble multiple documents into one
  // PDF "Project Plan" / "Status Pack" / etc. The user names the
  // report, picks which documents to include from the project, drags
  // them into the desired order, and clicks Export.
  //
  // Drag-and-drop is implemented with native HTML drag events rather
  // than a library — the list is small (typically < 25 entries) and
  // we don't need fancy animation.

  import { onMount } from 'svelte';
  import { goto } from '../../session.svelte';
  import { showToast } from '../../toast.svelte';
  import SignCertificateModal from '../SignCertificateModal.svelte';

  let reportTitle = $state('Project Report');
  let subtitle = $state('');
  let available = $state<DocumentRecord[]>([]);
  let included = $state<DocumentRecord[]>([]);
  let descriptions = $state<Record<string, string>>({});
  let status = $state('');
  let exporting = $state(false);
  let dragIndex = $state<number | null>(null);
  let showSignModal = $state(false);
  let pendingSignCertPath = $state('');

  onMount(async () => {
    try {
      const all = (await window.go.main.App.ListDocuments('')) ?? [];
      available = all;
    } catch (err: any) {
      status = `Could not load documents: ${err}`;
    }
  });

  function include(d: DocumentRecord) {
    if (included.find((x) => x.id === d.id)) return;
    included = [...included, d];
  }

  function exclude(id: string) {
    included = included.filter((d) => d.id !== id);
    delete descriptions[id];
  }

  function move(id: string, delta: -1 | 1) {
    const i = included.findIndex((d) => d.id === id);
    const j = i + delta;
    if (i < 0 || j < 0 || j >= included.length) return;
    const next = [...included];
    [next[i], next[j]] = [next[j], next[i]];
    included = next;
  }

  function onDragStart(e: DragEvent, i: number) {
    dragIndex = i;
    e.dataTransfer?.setData('text/plain', String(i));
  }

  function onDragOver(e: DragEvent) {
    e.preventDefault();
  }

  function onDrop(e: DragEvent, target: number) {
    e.preventDefault();
    const src = dragIndex;
    dragIndex = null;
    if (src === null || src === target) return;
    const next = [...included];
    const [moved] = next.splice(src, 1);
    next.splice(target, 0, moved);
    included = next;
  }

  function buildSections(): ReportSection[] {
    return included.map((d) => ({
      document_id: d.id,
      title: d.title,
      description: descriptions[d.id] ?? '',
    }));
  }

  async function exportReport() {
    if (included.length === 0) {
      status = 'Add at least one section before exporting.';
      return;
    }
    exporting = true;
    status = '';
    try {
      const path = await window.go.main.App.ExportCombinedReport(
        reportTitle,
        subtitle,
        buildSections(),
      );
      status = `Report exported to ${path}`;
    } catch (err: any) {
      status = `Export failed: ${err}`;
    } finally {
      exporting = false;
    }
  }

  async function exportSignedReport() {
    if (included.length === 0) {
      status = 'Add at least one section before exporting.';
      return;
    }
    let certPath = '';
    try {
      const s = await window.go.main.App.GetSettings();
      if (s?.cert_path) certPath = s.cert_path;
    } catch {}
    pendingSignCertPath = certPath;
    showSignModal = true;
  }

  async function handleSignedConfirm(pwd: string) {
    showSignModal = false;
    if (!pendingSignCertPath || !pwd) {
      status = 'Certificate path and password are required for signed export.';
      return;
    }

    exporting = true;
    status = '';
    try {
      const path = await window.go.main.App.ExportCombinedReportSigned(
        reportTitle,
        subtitle,
        buildSections(),
        pendingSignCertPath,
        pwd,
      );
      status = `Signed report exported to ${path}`;
      showToast('Signed combined report exported successfully', 'success');
    } catch (err: any) {
      status = `Signed export failed: ${err}`;
      showToast(`Signed export failed: ${err}`, 'error');
    } finally {
      exporting = false;
      pendingSignCertPath = '';
    }
  }

  // Documents not yet in the report (so the picker only shows new ones).
  let remaining = $derived(
    available.filter((d) => !included.find((x) => x.id === d.id)),
  );
</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <button onclick={() => goto('dashboard')} class="text-xs text-slate-400 hover:text-cyan-400">
        &larr; Dashboard
      </button>
      <h1 class="text-sm font-bold tracking-widest uppercase text-white">Combined Report</h1>
    </div>
    <button
      onclick={exportReport}
      disabled={exporting || included.length === 0}
      class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-3 py-1 rounded"
    >
      {exporting ? 'Exporting...' : 'Export PDF'}
    </button>
    <button
      onclick={exportSignedReport}
      disabled={exporting || included.length === 0}
      class="text-xs bg-emerald-700 hover:bg-emerald-600 disabled:opacity-50 text-white font-bold uppercase px-3 py-1 rounded"
      title="Export with an embedded PAdES B-B digital signature"
    >
      {exporting ? 'Signing...' : 'Sign & Export'}
    </button>
  </header>

  <main class="p-6 space-y-6">
    {#if status}
      <p class="text-xs text-cyan-400">{status}</p>
    {/if}

    <section class="grid grid-cols-1 md:grid-cols-2 gap-4 max-w-3xl">
      <label class="block">
        <span class="text-xs font-semibold text-slate-500 uppercase">Report Title</span>
        <input
          bind:value={reportTitle}
          class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        />
      </label>
      <label class="block">
        <span class="text-xs font-semibold text-slate-500 uppercase">Subtitle</span>
        <input
          bind:value={subtitle}
          placeholder="e.g. Q3 2026 Status Pack"
          class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        />
      </label>
    </section>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <!-- Picker -->
      <section>
        <h2 class="text-xs font-bold tracking-widest uppercase text-slate-500 mb-3">
          Available documents ({remaining.length})
        </h2>
        {#if remaining.length === 0}
          <p class="text-xs text-slate-500">
            Every document in this project is already in the report, or the
            project has no documents yet.
          </p>
        {:else}
          <ul class="space-y-2">
            {#each remaining as d (d.id)}
              <li class="flex items-center justify-between p-3 bg-slate-900 border border-slate-800 rounded">
                <div>
                  <div class="font-bold text-white text-sm">{d.title}</div>
                  <div class="text-xs text-slate-500">{d.kind} · v{d.version} · {d.status}</div>
                </div>
                <button
                  onclick={() => include(d)}
                  class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded"
                >
                  Add →
                </button>
              </li>
            {/each}
          </ul>
        {/if}
      </section>

      <!-- Included with drag-and-drop ordering -->
      <section>
        <h2 class="text-xs font-bold tracking-widest uppercase text-slate-500 mb-3">
          Sections in report ({included.length})
        </h2>
        {#if included.length === 0}
          <p class="text-xs text-slate-500">
            Add documents from the left. Drag handles to reorder.
          </p>
        {:else}
          <ol class="space-y-2">
            {#each included as d, i (d.id)}
              <li
                draggable="true"
                ondragstart={(e) => onDragStart(e, i)}
                ondragover={onDragOver}
                ondrop={(e) => onDrop(e, i)}
                class="p-3 bg-slate-900 border border-slate-800 rounded cursor-move"
              >
                <div class="flex items-start justify-between gap-2">
                  <div class="flex-1 min-w-0">
                    <div class="font-bold text-white text-sm">
                      <span class="text-cyan-400 mr-2">{i + 1}.</span>
                      {d.title}
                    </div>
                    <div class="text-xs text-slate-500 mb-2">{d.kind}</div>
                    <input
                      placeholder="Section intro (optional, one line)"
                      value={descriptions[d.id] ?? ''}
                      oninput={(e) => (descriptions[d.id] = (e.target as HTMLInputElement).value)}
                      class="w-full bg-slate-950 border border-slate-800 p-1 text-xs rounded focus:border-cyan-500 outline-none"
                    />
                  </div>
                  <div class="flex flex-col gap-1 items-end">
                    <button onclick={() => move(d.id, -1)} class="text-slate-500 hover:text-cyan-400 text-xs px-2" aria-label="Move up">▲</button>
                    <button onclick={() => move(d.id, 1)} class="text-slate-500 hover:text-cyan-400 text-xs px-2" aria-label="Move down">▼</button>
                    <button onclick={() => exclude(d.id)} class="text-slate-500 hover:text-red-400 text-xs px-2" aria-label="Remove">×</button>
                  </div>
                </div>
              </li>
            {/each}
          </ol>
        {/if}
      </section>
    </div>
  </main>

  <SignCertificateModal
    bind:open={showSignModal}
    certPath={pendingSignCertPath}
    onConfirm={handleSignedConfirm}
  />
</div>
