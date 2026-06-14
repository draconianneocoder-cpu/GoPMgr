<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // ProjectSettings lets the user edit project-level metadata after
  // creation: name, description, industry, sub-category, methodology,
  // country, budget, owner, dates, status, phase.
  //
  // The Launchpad sets these at creation time; this panel is the
  // canonical "go back and reclassify" entry point. Reuses existing
  // App.UpdateProjectMeta and App.UpdateProjectIndustry — no new
  // backend code.

  import { onMount, onDestroy } from 'svelte';
  import { session, goto } from '../../session.svelte';

  let draft = $state<ProjectMeta | null>(null);
  let original = $state<ProjectMeta | null>(null);
  let busy = $state(false);
  let status = $state('');
  let error = $state('');

  // Schedule report export state
  let exporting = $state(false);
  let exportFormat = $state<'pdf' | 'docx' | 'odt' | null>(null);
  let exportStatus = $state('');
  let exportError = $state(false);

  // Export / signature settings
  let exportTheme = $state<'modern' | 'classic' | 'archival'>('modern');
  let autoRepair = $state(true);
  let certPath = $state('');
  let signatureEnabled = $state(false);
  let settingsBusy = $state(false);
  let settingsStatus = $state('');
  let settingsError = $state('');

  // Database encryption state
  let encryptionState = $state<'unknown' | 'plaintext' | 'encrypted'>('unknown');
  let encryptionBusy = $state(false);
  let encryptionStatus = $state('');
  let encryptionError = $state('');
  let encryptionBackupPath = $state('');
  let recoveryCodes = $state<string[]>([]);

  // Font settings
  let fonts = $state<FontFamilyInfo[]>([]);
  let defaultFont = $state('');
  let fontBusy = $state(false);
  let fontStatus = $state('');

  onMount(async () => {
    try {
      const p = await window.go.main.App.GetProjectMeta();
      draft = { ...p };
      original = p;
    } catch (err: any) {
      error = `Could not load project: ${err}`;
    }
    try {
      const s = await window.go.main.App.GetSettings();
      exportTheme = s.export_theme;
      autoRepair = s.auto_repair;
      certPath = s.cert_path ?? '';
      signatureEnabled = s.signature_enabled;
    } catch {
      // non-fatal; leave defaults
    }
    try {
      fonts = (await window.go.main.App.ListFonts()) ?? [];
      defaultFont = (await window.go.main.App.GetDefaultFont()) ?? '';
    } catch {
      // non-fatal
    }
    await loadEncryptionState();
  });

  let dirty = $derived(
    draft !== null && original !== null && JSON.stringify(draft) !== JSON.stringify(original),
  );

  async function save() {
    if (!draft) return;
    busy = true;
    error = '';
    status = '';
    try {
      // Two calls because UpdateProjectIndustry covers the four
      // Launchpad columns explicitly; UpdateProjectMeta handles
      // everything else.
      const meta = await window.go.main.App.UpdateProjectMeta(draft);
      const merged = await window.go.main.App.UpdateProjectIndustry(
        draft.industry,
        draft.sub_category,
        draft.methodology,
        draft.country_code,
      );
      original = merged;
      draft = { ...merged };
      session.project = merged;
      status = 'Saved.';
      // Suppress unused-variable warning while keeping the explicit
      // call so the metadata path is always exercised.
      void meta;
    } catch (err: any) {
      error = `Save failed: ${err}`;
    } finally {
      busy = false;
    }
  }

  function revert() {
    if (original) draft = { ...original };
  }

  async function saveExportSettings() {
    settingsBusy = true;
    settingsStatus = '';
    settingsError = '';
    try {
      const current = await window.go.main.App.GetSettings();
      await window.go.main.App.SaveSettings({
        ...current,
        export_theme: exportTheme,
        auto_repair: autoRepair,
        cert_path: certPath,
        signature_enabled: signatureEnabled,
      });
      settingsStatus = 'Saved.';
    } catch (err: any) {
      settingsError = `Save failed: ${err}`;
    } finally {
      settingsBusy = false;
    }
  }

  function recoveryReissueRequired(message: string) {
    return message.includes('Reissue recovery codes before enabling database encryption');
  }

  async function loadEncryptionState() {
    encryptionStatus = '';
    encryptionError = '';
    encryptionBackupPath = '';
    recoveryCodes = [];
    if (!session.projectPath) {
      encryptionState = 'unknown';
      encryptionError = 'Open this project from the project list before checking database encryption.';
      return;
    }
    try {
      const encrypted = await window.go.main.App.IsProjectEncrypted(session.projectPath);
      encryptionState = encrypted ? 'encrypted' : 'plaintext';
    } catch (err: any) {
      encryptionState = 'unknown';
      encryptionError = `Could not check encryption: ${err}`;
    }
  }

  async function encryptDatabase() {
    if (!session.projectPath) {
      encryptionError = 'Open this project from the project list before encrypting the database.';
      return;
    }
    encryptionBusy = true;
    encryptionStatus = '';
    encryptionError = '';
    encryptionBackupPath = '';
    recoveryCodes = [];
    try {
      const backupPath = await window.go.main.App.EncryptProjectAtRest(session.projectPath);
      encryptionBackupPath = backupPath;
      encryptionState = 'encrypted';
      encryptionStatus = 'Database encrypted.';
    } catch (err: any) {
      const message = String(err?.message ?? err);
      encryptionError = message;
    } finally {
      encryptionBusy = false;
    }
  }

  async function reissueRecoveryCodes() {
    encryptionBusy = true;
    encryptionStatus = '';
    encryptionError = '';
    recoveryCodes = [];
    try {
      recoveryCodes = (await window.go.main.App.IssueRecoveryCodes()) ?? [];
      encryptionStatus = 'Recovery codes reissued. Save these codes, then encrypt the database.';
    } catch (err: any) {
      encryptionError = `Recovery-code reissue failed: ${err}`;
    } finally {
      encryptionBusy = false;
    }
  }

  async function chooseCert() {
    try {
      const p = await window.go.main.App.ChooseCertFile();
      if (p) certPath = p;
    } catch {
      // user cancelled
    }
  }

  async function applyFont() {
    if (!defaultFont) return;
    fontBusy = true;
    fontStatus = '';
    try {
      await window.go.main.App.SetDefaultFont(defaultFont);
      fontStatus = 'Default font updated.';
    } catch (err: any) {
      fontStatus = `Failed: ${err}`;
    } finally {
      fontBusy = false;
    }
  }

  async function importFont() {
    fontBusy = true;
    fontStatus = '';
    try {
      const fi = await window.go.main.App.ImportFont();
      fonts = [...fonts.filter((f) => f.name !== fi.name), fi];
      defaultFont = fi.name;
      fontStatus = `Imported "${fi.name}".`;
    } catch (err: any) {
      fontStatus = `Import failed: ${err}`;
    } finally {
      fontBusy = false;
    }
  }

  async function exportScheduleReport(format: 'pdf' | 'docx' | 'odt') {
    exporting = true;
    exportFormat = format;
    exportStatus = '';
    exportError = false;

    try {
      let path: string;
      if (format === 'pdf') {
        path = await window.go.main.App.ExportScheduleReportPDF();
      } else if (format === 'docx') {
        path = await window.go.main.App.ExportScheduleReportDOCX();
      } else {
        path = await window.go.main.App.ExportScheduleReportODT();
      }
      exportStatus = `Exported to: ${path}`;
    } catch (err: any) {
      exportError = true;
      exportStatus = `Export failed: ${err}`;
    } finally {
      exporting = false;
      exportFormat = null;
    }
  }

  onDestroy(() => {});
</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <button onclick={() => goto('dashboard')} class="text-xs text-slate-400 hover:text-cyan-400">
        &larr; Dashboard
      </button>
      <h1 class="text-sm font-bold tracking-widest uppercase text-white">Project Settings</h1>
    </div>
    <div class="flex gap-2">
      <button
        onclick={revert}
        disabled={!dirty || busy}
        class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-30 px-3 py-1 rounded"
      >
        Revert
      </button>
      <button
        onclick={save}
        disabled={!dirty || busy}
        class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-3 py-1 rounded"
      >
        {busy ? 'Saving…' : 'Save changes'}
      </button>
    </div>
  </header>

  <main class="p-6 max-w-3xl mx-auto space-y-6">
    {#if error}
      <p class="text-xs text-red-400" role="alert">{error}</p>
    {/if}
    {#if status}
      <p class="text-xs text-cyan-400">{status}</p>
    {/if}

    {#if !draft}
      <p class="text-sm text-slate-500">Loading…</p>
    {:else}
      <!-- Identity -->
      <section class="grid grid-cols-1 md:grid-cols-2 gap-3">
        <label class="block">
          <span class="text-xs text-slate-500 uppercase">Project name</span>
          <input
            bind:value={draft.name}
            class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          />
        </label>
        <label class="block">
          <span class="text-xs text-slate-500 uppercase">Owner</span>
          <input
            bind:value={draft.owner}
            class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          />
        </label>
        <label class="block md:col-span-2">
          <span class="text-xs text-slate-500 uppercase">Description</span>
          <textarea
            bind:value={draft.description}
            rows="3"
            class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          ></textarea>
        </label>
      </section>

      <!-- Classification (Launchpad fields) -->
      <section>
        <h2 class="text-xs font-bold uppercase tracking-widest text-slate-500 mb-2">
          Classification
        </h2>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Industry</span>
            <select
              bind:value={draft.industry}
              class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded"
            >
              <option value="">(none)</option>
              <option value="business">Business</option>
              <option value="administration">Administration</option>
              <option value="engineering">Engineering</option>
              <option value="software">Software</option>
              <option value="construction">Construction</option>
              <option value="custom">Custom</option>
            </select>
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Sub-category</span>
            <input
              bind:value={draft.sub_category}
              class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Methodology</span>
            <input
              bind:value={draft.methodology}
              placeholder="e.g. scrum / cpm / waterfall"
              class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Country (for holidays)</span>
            <select
              bind:value={draft.country_code}
              class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded"
            >
              <option value="US">United States</option>
              <option value="GB">United Kingdom</option>
              <option value="CA">Canada</option>
              <option value="DE">Germany</option>
              <option value="FR">France</option>
              <option value="AU">Australia</option>
              <option value="">Other / generic</option>
            </select>
          </label>
        </div>
      </section>

      <!-- Lifecycle -->
      <section>
        <h2 class="text-xs font-bold uppercase tracking-widest text-slate-500 mb-2">
          Lifecycle
        </h2>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Status</span>
            <select
              bind:value={draft.status}
              class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded"
            >
              <option value="planning">Planning</option>
              <option value="active">Active</option>
              <option value="on_hold">On hold</option>
              <option value="complete">Complete</option>
              <option value="cancelled">Cancelled</option>
            </select>
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Phase</span>
            <select
              bind:value={draft.phase}
              class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded"
            >
              <option value="initiation">Initiation</option>
              <option value="planning">Planning</option>
              <option value="execution">Execution</option>
              <option value="monitoring">Monitoring</option>
              <option value="closing">Closing</option>
            </select>
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Start date</span>
            <input
              type="date"
              bind:value={draft.start_date}
              class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">End date</span>
            <input
              type="date"
              bind:value={draft.end_date}
              class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block md:col-span-2">
            <span class="text-xs text-slate-500 uppercase">Budget</span>
            <input
              type="number"
              step="100"
              bind:value={draft.budget}
              class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
            <span class="block text-[10px] text-slate-500 mt-1">
              Feeds the Dashboard Budget panel via stakeholder rates × work-item points.
            </span>
           </label>
         </div>
       </section>

       <!-- Schedule Reports (CPM) -->
       <section>
         <h2 class="text-xs font-bold uppercase tracking-widest text-slate-500 mb-2">
           Schedule Reports (CPM)
         </h2>
         <p class="text-xs text-slate-400 mb-3">
           Export the current project schedule with full Critical Path Method (ES/EF/LS/LF/Float/Critical) calculations.
         </p>

         <div class="flex flex-wrap gap-2">
           <button
             onclick={() => exportScheduleReport('pdf')}
             disabled={exporting}
             class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-4 py-2 rounded border border-slate-700"
           >
             {exporting && exportFormat === 'pdf' ? 'Exporting…' : 'Export PDF'}
           </button>

           <button
             onclick={() => exportScheduleReport('docx')}
             disabled={exporting}
             class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-4 py-2 rounded border border-slate-700"
           >
             {exporting && exportFormat === 'docx' ? 'Exporting…' : 'Export DOCX'}
           </button>

           <button
             onclick={() => exportScheduleReport('odt')}
             disabled={exporting}
             class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-4 py-2 rounded border border-slate-700"
           >
             {exporting && exportFormat === 'odt' ? 'Exporting…' : 'Export ODT'}
           </button>
         </div>

         {#if exportStatus}
           <p class="text-xs mt-2 {exportError ? 'text-red-400' : 'text-cyan-400'}">
             {exportStatus}
           </p>
         {/if}
       </section>

       <!-- Database Encryption -->
       <section>
         <h2 class="text-xs font-bold uppercase tracking-widest text-slate-500 mb-2">
           Database Encryption
         </h2>
         <div class="border border-slate-800 bg-slate-900/60 rounded p-4 space-y-3">
           <div class="flex flex-wrap items-center justify-between gap-3">
             <div>
               <span class="text-xs text-slate-500 uppercase">State</span>
               <p class="text-sm font-semibold text-white">
                 {encryptionState === 'encrypted'
                   ? 'Encrypted'
                   : encryptionState === 'plaintext'
                     ? 'Plaintext'
                     : 'Unknown'}
               </p>
             </div>
             {#if encryptionState === 'plaintext'}
               <button
                 onclick={encryptDatabase}
                 disabled={encryptionBusy}
                 class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-4 py-2 rounded"
               >
                 {encryptionBusy ? 'Encrypting…' : 'Encrypt database'}
               </button>
             {:else}
               <button
                 onclick={loadEncryptionState}
                 disabled={encryptionBusy}
                 class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-4 py-2 rounded"
               >
                 Refresh
               </button>
             {/if}
           </div>

           {#if encryptionState === 'plaintext'}
             <p class="text-xs text-slate-400">
               Encryption keeps project rows in a SQLCipher database and retains a plaintext backup
               beside the project file.
             </p>
           {/if}

           {#if encryptionBackupPath}
             <p class="text-xs text-cyan-400">
               Backup retained at: {encryptionBackupPath}
             </p>
           {/if}
           {#if encryptionStatus}
             <p class="text-xs text-cyan-400">{encryptionStatus}</p>
           {/if}
           {#if encryptionError}
             <div class="space-y-2">
               <p class="text-xs text-red-400" role="alert">{encryptionError}</p>
               {#if recoveryReissueRequired(encryptionError)}
                 <button
                   onclick={reissueRecoveryCodes}
                   disabled={encryptionBusy}
                   class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-4 py-2 rounded border border-slate-700"
                 >
                   {encryptionBusy ? 'Reissuing…' : 'Reissue recovery codes'}
                 </button>
               {/if}
             </div>
           {/if}
           {#if recoveryCodes.length > 0}
             <div class="border border-cyan-900/60 bg-cyan-950/20 rounded p-3">
               <p class="text-xs text-cyan-300 mb-2">Save these new recovery codes now.</p>
               <ul class="grid grid-cols-1 sm:grid-cols-2 gap-1 font-mono text-xs text-slate-200">
                 {#each recoveryCodes as code}
                   <li>{code}</li>
                 {/each}
               </ul>
             </div>
           {/if}
         </div>
       </section>

       <!-- Export & Signature Settings -->
       <section>
         <h2 class="text-xs font-bold uppercase tracking-widest text-slate-500 mb-2">
           Export &amp; Signature Settings
         </h2>
         <div class="space-y-3">
           <label class="block">
             <span class="text-xs text-slate-500 uppercase">Export Theme</span>
             <select
               bind:value={exportTheme}
               class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded"
             >
               <option value="modern">Modern (Dark)</option>
               <option value="classic">Classic (Light)</option>
               <option value="archival">Archival (B&amp;W)</option>
             </select>
           </label>

           <label class="flex items-center gap-3 cursor-pointer">
             <input type="checkbox" bind:checked={autoRepair} class="accent-cyan-500" />
             <span class="text-sm text-slate-300">Enable background self-healing</span>
           </label>

           <label class="flex items-center gap-3 cursor-pointer">
             <input type="checkbox" bind:checked={signatureEnabled} class="accent-cyan-500" />
             <span class="text-sm text-slate-300">Enable PDF digital signatures</span>
           </label>

           <div>
             <span class="text-xs text-slate-500 uppercase">Certificate path</span>
             <div class="flex gap-2 mt-1">
               <input
                 bind:value={certPath}
                 placeholder="Path to .p12 / .pfx certificate"
                 class="flex-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none text-sm"
               />
               <button
                 onclick={chooseCert}
                 class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded"
               >
                 Browse…
               </button>
             </div>
           </div>

           <button
             onclick={saveExportSettings}
             disabled={settingsBusy}
             class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-4 py-1.5 rounded"
           >
             {settingsBusy ? 'Saving…' : 'Save export settings'}
           </button>

           {#if settingsStatus}
             <p class="text-xs text-cyan-400">{settingsStatus}</p>
           {/if}
           {#if settingsError}
             <p class="text-xs text-red-400">{settingsError}</p>
           {/if}
         </div>
       </section>

       <!-- Document Font -->
       <section>
         <h2 class="text-xs font-bold uppercase tracking-widest text-slate-500 mb-2">
           Document Font
         </h2>
         <p class="text-xs text-slate-400 mb-3">
           Applies to all PDF, DOCX, and ODT exports for this project.
         </p>
         <div class="flex flex-wrap gap-2 items-end">
           <div class="flex-1 min-w-40">
             <label class="block">
               <span class="text-xs text-slate-500 uppercase">Default family</span>
               <select
                 bind:value={defaultFont}
                 class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded"
               >
                 {#each fonts as f (f.name)}
                   <option value={f.name}>{f.name} ({f.category})</option>
                 {/each}
               </select>
             </label>
           </div>
           <button
             onclick={applyFont}
             disabled={fontBusy || !defaultFont}
             class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-4 py-2 rounded"
           >
             Apply
           </button>
           <button
             onclick={importFont}
             disabled={fontBusy}
             class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-4 py-2 rounded"
           >
             Import font…
           </button>
         </div>
         {#if fontStatus}
           <p class="text-xs mt-2 text-cyan-400">{fontStatus}</p>
         {/if}
       </section>
     {/if}
   </main>
 </div>
