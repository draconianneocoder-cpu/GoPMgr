<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
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
  let exportFormat = $state<'pdf' | 'docx' | 'odt' | 'csv' | 'html' | 'mspdi' | null>(null);
  let exportStatus = $state('');
  let exportError = $state(false);

  // Export / signature settings
  let exportTheme = $state<'modern' | 'classic' | 'archival'>('modern');
  let autoRepair = $state(true);
  let certPath = $state('');
  let signatureEnabled = $state(false);
  let complianceMode = $state(false);
  let settingsBusy = $state(false);
  let settingsResetting = $state(false);
  let settingsStatus = $state('');
  let settingsError = $state('');
  let auditReportBusy = $state(false);
  let auditReportStatus = $state('');
  let auditReportError = $state('');
  let auditRepairBusy = $state(false);
  let auditRepairStatus = $state('');
  let auditRepairError = $state('');

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

  type ResourceCalendarDraft = {
    id: string;
    name: string;
    resource: string;
    default_capacity: number;
    weekly_capacity: string;
    overrides: string;
    skill_tags: string;
    notes: string;
  };

  const emptyResourceCalendarDraft = (): ResourceCalendarDraft => ({
    id: '',
    name: '',
    resource: '',
    default_capacity: 1,
    weekly_capacity: '',
    overrides: '',
    skill_tags: '',
    notes: '',
  });

  let resourceCalendars = $state<ResourceCalendar[]>([]);
  let resourceCalendarDraft = $state<ResourceCalendarDraft>(emptyResourceCalendarDraft());
  let resourceCalendarBusy = $state(false);
  let resourceCalendarStatus = $state('');
  let resourceCalendarError = $state('');

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
      complianceMode = s.compliance_mode ?? false;
    } catch {
      // non-fatal; leave defaults
    }
    try {
      fonts = (await window.go.main.App.ListFonts()) ?? [];
      defaultFont = (await window.go.main.App.GetDefaultFont()) ?? '';
    } catch {
      // non-fatal
    }
    await loadResourceCalendars();
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
        compliance_mode: complianceMode,
      });
      settingsStatus = 'Saved.';
    } catch (err: any) {
      settingsError = `Save failed: ${err}`;
    } finally {
      settingsBusy = false;
    }
  }

  async function resetProjectSettings() {
    settingsResetting = true;
    settingsStatus = '';
    settingsError = '';
    try {
      const defaults = await window.go.main.App.ResetProjectSettings();
      exportTheme = (defaults.export_theme || 'modern') as 'modern' | 'classic' | 'archival';
      autoRepair = defaults.auto_repair;
      certPath = defaults.cert_path ?? '';
      signatureEnabled = defaults.signature_enabled;
      complianceMode = defaults.compliance_mode ?? false;
      defaultFont = defaults.default_font ?? '';
      fontStatus = '';
      settingsStatus = 'Defaults restored.';
    } catch (err: any) {
      settingsError = `Reset failed: ${err}`;
    } finally {
      settingsResetting = false;
    }
  }

  async function exportAuditVerificationReport() {
    auditReportBusy = true;
    auditReportStatus = '';
    auditReportError = '';
    try {
      const path = await window.go.main.App.ExportAuditVerificationReport();
      auditReportStatus = `Audit verification report exported to: ${path}`;
    } catch (err: any) {
      auditReportError = `Audit verification report failed: ${err}`;
    } finally {
      auditReportBusy = false;
    }
  }

  async function exportAuditRepairEvidence() {
    auditRepairBusy = true;
    auditRepairStatus = '';
    auditRepairError = '';
    try {
      const path = await window.go.main.App.ExportAuditRepairEvidence();
      auditRepairStatus = `Audit repair evidence exported to: ${path}`;
    } catch (err: any) {
      auditRepairError = `Audit repair evidence failed: ${err}`;
    } finally {
      auditRepairBusy = false;
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

  type ScheduleExportFormat = 'pdf' | 'docx' | 'odt' | 'csv' | 'html' | 'mspdi';

  async function exportScheduleReport(format: ScheduleExportFormat) {
    exporting = true;
    exportFormat = format;
    exportStatus = '';
    exportError = false;

    const exporters: Record<ScheduleExportFormat, () => Promise<string>> = {
      pdf: () => window.go.main.App.ExportScheduleReportPDF(),
      docx: () => window.go.main.App.ExportScheduleReportDOCX(),
      odt: () => window.go.main.App.ExportScheduleReportODT(),
      csv: () => window.go.main.App.ExportScheduleReportCSV(),
      html: () => window.go.main.App.ExportScheduleReportHTML(),
      mspdi: () => window.go.main.App.ExportScheduleReportMSPDI(),
    };

    try {
      const path = await exporters[format]();
      exportStatus = `Exported to: ${path}`;
    } catch (err: any) {
      exportError = true;
      exportStatus = `Export failed: ${err}`;
    } finally {
      exporting = false;
      exportFormat = null;
    }
  }

  function parseCapacityPairs(raw: string, field: string, weekly = false): Record<number, number> {
    const out: Record<number, number> = {};
    const trimmed = raw.trim();
    if (!trimmed) return out;
    for (const token of trimmed.split(/[,\n]+/)) {
      const part = token.trim();
      if (!part) continue;
      const match = part.match(/^(-?\d+)\s*[:=]\s*(-?\d+(?:\.\d+)?)$/);
      if (!match) throw new Error(`${field} entry "${part}" must be day:capacity`);
      const day = Number(match[1]);
      const capacity = Number(match[2]);
      if (!Number.isInteger(day) || day < 0 || (weekly && day > 6)) {
        throw new Error(`${field} day ${day} is out of range`);
      }
      if (!Number.isFinite(capacity) || capacity < 0) {
        throw new Error(`${field} capacity for day ${day} must be zero or greater`);
      }
      out[day] = capacity;
    }
    return out;
  }

  function parseNotePairs(raw: string): Record<number, string> {
    const out: Record<number, string> = {};
    const trimmed = raw.trim();
    if (!trimmed) return out;
    for (const token of trimmed.split(/\n+/)) {
      const part = token.trim();
      if (!part) continue;
      const match = part.match(/^(-?\d+)\s*[:=]\s*(.+)$/);
      if (!match) throw new Error(`Note entry "${part}" must be day:note`);
      const day = Number(match[1]);
      if (!Number.isInteger(day) || day < 0) throw new Error(`Note day ${day} is out of range`);
      out[day] = match[2].trim();
    }
    return out;
  }

  function formatCapacityPairs(values: Record<number, number> | undefined): string {
    if (!values) return '';
    return Object.entries(values)
      .sort(([a], [b]) => Number(a) - Number(b))
      .map(([day, capacity]) => `${day}:${capacity}`)
      .join(', ');
  }

  function formatNotePairs(values: Record<number, string> | undefined): string {
    if (!values) return '';
    return Object.entries(values)
      .sort(([a], [b]) => Number(a) - Number(b))
      .map(([day, note]) => `${day}: ${note}`)
      .join('\n');
  }

  async function loadResourceCalendars() {
    resourceCalendarError = '';
    try {
      resourceCalendars = (await window.go.main.App.ListResourceCalendars()) ?? [];
    } catch (err: any) {
      resourceCalendarError = `Could not load resource calendars: ${err}`;
    }
  }

  function editResourceCalendar(c: ResourceCalendar) {
    resourceCalendarDraft = {
      id: c.id,
      name: c.name,
      resource: c.resource,
      default_capacity: c.default_capacity || 1,
      weekly_capacity: formatCapacityPairs(c.weekly_capacity),
      overrides: formatCapacityPairs(c.overrides),
      skill_tags: (c.skill_tags ?? []).join(', '),
      notes: formatNotePairs(c.notes),
    };
    resourceCalendarStatus = '';
    resourceCalendarError = '';
  }

  function resetResourceCalendarDraft() {
    resourceCalendarDraft = emptyResourceCalendarDraft();
    resourceCalendarStatus = '';
    resourceCalendarError = '';
  }

  async function saveResourceCalendar() {
    resourceCalendarBusy = true;
    resourceCalendarStatus = '';
    resourceCalendarError = '';
    try {
      const defaultCapacity = Number(resourceCalendarDraft.default_capacity);
      if (!Number.isFinite(defaultCapacity) || defaultCapacity <= 0) {
        throw new Error('Default capacity must be greater than zero');
      }
      const saved = await window.go.main.App.SaveResourceCalendar({
        id: resourceCalendarDraft.id,
        project_id: '',
        name: resourceCalendarDraft.name.trim(),
        resource: resourceCalendarDraft.resource.trim(),
        default_capacity: defaultCapacity,
        weekly_capacity: parseCapacityPairs(
          resourceCalendarDraft.weekly_capacity,
          'Weekly capacity',
          true,
        ),
        overrides: parseCapacityPairs(resourceCalendarDraft.overrides, 'Override capacity'),
        skill_tags: resourceCalendarDraft.skill_tags
          .split(',')
          .map((tag) => tag.trim())
          .filter(Boolean),
        notes: parseNotePairs(resourceCalendarDraft.notes),
        created_at: '',
        updated_at: '',
      });
      resourceCalendars = [
        ...resourceCalendars.filter((c) => c.id !== saved.id),
        saved,
      ].sort((a, b) => `${a.resource}${a.name}`.localeCompare(`${b.resource}${b.name}`));
      resourceCalendarDraft = emptyResourceCalendarDraft();
      resourceCalendarStatus = 'Saved.';
    } catch (err: any) {
      resourceCalendarError = `Save failed: ${err?.message ?? err}`;
    } finally {
      resourceCalendarBusy = false;
    }
  }

  async function deleteResourceCalendar(id: string) {
    resourceCalendarBusy = true;
    resourceCalendarStatus = '';
    resourceCalendarError = '';
    try {
      await window.go.main.App.DeleteResourceCalendar(id);
      resourceCalendars = resourceCalendars.filter((c) => c.id !== id);
      if (resourceCalendarDraft.id === id) resourceCalendarDraft = emptyResourceCalendarDraft();
      resourceCalendarStatus = 'Deleted.';
    } catch (err: any) {
      resourceCalendarError = `Delete failed: ${err}`;
    } finally {
      resourceCalendarBusy = false;
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
      <h1 class="text-sm font-bold tracking-widest uppercase text-slate-50">Project Settings</h1>
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

       <!-- Resource Capacity -->
       <section>
         <h2 class="text-xs font-bold uppercase tracking-widest text-slate-500 mb-2">
           Resource Capacity
         </h2>
         <div class="space-y-3">
           {#if resourceCalendars.length > 0}
             <div class="divide-y divide-slate-800 border border-slate-800 rounded overflow-hidden">
               {#each resourceCalendars as calendar (calendar.id)}
                 <div class="grid grid-cols-1 md:grid-cols-[1fr_auto] gap-2 p-3 bg-slate-900/40">
                   <div>
                     <p class="text-sm font-semibold text-slate-100">{calendar.name || calendar.resource}</p>
                     <p class="text-xs text-slate-400">
                       {calendar.resource || 'Unassigned resource'} · default {calendar.default_capacity || 1}
                     </p>
                     {#if calendar.skill_tags?.length}
                       <p class="text-[10px] text-slate-500 uppercase mt-1">
                         {calendar.skill_tags.join(', ')}
                       </p>
                     {/if}
                   </div>
                   <div class="flex gap-2 md:justify-end">
                     <button
                       onclick={() => editResourceCalendar(calendar)}
                       disabled={resourceCalendarBusy}
                       class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-3 py-1 rounded"
                     >
                       Edit
                     </button>
                     <button
                       onclick={() => deleteResourceCalendar(calendar.id)}
                       disabled={resourceCalendarBusy}
                       class="text-xs bg-red-950/60 hover:bg-red-900/70 disabled:opacity-50 text-red-100 px-3 py-1 rounded border border-red-900/70"
                     >
                       Delete
                     </button>
                   </div>
                 </div>
               {/each}
             </div>
           {/if}

           <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
             <label class="block">
               <span class="text-xs text-slate-500 uppercase">Calendar name</span>
               <input
                 bind:value={resourceCalendarDraft.name}
                 class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
               />
             </label>
             <label class="block">
               <span class="text-xs text-slate-500 uppercase">Resource</span>
               <input
                 bind:value={resourceCalendarDraft.resource}
                 class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
               />
             </label>
             <label class="block">
               <span class="text-xs text-slate-500 uppercase">Default capacity</span>
               <input
                 type="number"
                 min="0.01"
                 step="0.25"
                 bind:value={resourceCalendarDraft.default_capacity}
                 class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
               />
             </label>
             <label class="block">
               <span class="text-xs text-slate-500 uppercase">Skill tags</span>
               <input
                 bind:value={resourceCalendarDraft.skill_tags}
                 placeholder="piping, qa"
                 class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
               />
             </label>
             <label class="block">
               <span class="text-xs text-slate-500 uppercase">Weekly capacity</span>
               <input
                 bind:value={resourceCalendarDraft.weekly_capacity}
                 placeholder="0:1, 4:0.5"
                 class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
               />
             </label>
             <label class="block">
               <span class="text-xs text-slate-500 uppercase">Day overrides</span>
               <input
                 bind:value={resourceCalendarDraft.overrides}
                 placeholder="12:0, 18:0.5"
                 class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
               />
             </label>
             <label class="block md:col-span-2">
               <span class="text-xs text-slate-500 uppercase">Notes</span>
               <textarea
                 bind:value={resourceCalendarDraft.notes}
                 rows="2"
                 placeholder="12: Medical leave"
                 class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
               ></textarea>
             </label>
           </div>

           <div class="flex flex-wrap gap-2">
             <button
               onclick={saveResourceCalendar}
               disabled={resourceCalendarBusy}
               class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-4 py-2 rounded"
             >
               {resourceCalendarBusy
                 ? 'Saving…'
                 : resourceCalendarDraft.id
                   ? 'Update calendar'
                   : 'Add calendar'}
             </button>
             <button
               onclick={resetResourceCalendarDraft}
               disabled={resourceCalendarBusy}
               class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-4 py-2 rounded"
             >
               Clear
             </button>
           </div>

           {#if resourceCalendarStatus}
             <p class="text-xs text-cyan-400">{resourceCalendarStatus}</p>
           {/if}
           {#if resourceCalendarError}
             <p class="text-xs text-red-400" role="alert">{resourceCalendarError}</p>
           {/if}
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

           <button
             onclick={() => exportScheduleReport('csv')}
             disabled={exporting}
             class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-4 py-2 rounded border border-slate-700"
           >
             {exporting && exportFormat === 'csv' ? 'Exporting…' : 'Export CSV'}
           </button>

           <button
             onclick={() => exportScheduleReport('html')}
             disabled={exporting}
             class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-4 py-2 rounded border border-slate-700"
           >
             {exporting && exportFormat === 'html' ? 'Exporting…' : 'Export HTML'}
           </button>

           <button
             onclick={() => exportScheduleReport('mspdi')}
             disabled={exporting}
             class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-4 py-2 rounded border border-slate-700"
           >
             {exporting && exportFormat === 'mspdi' ? 'Exporting…' : 'Export MS Project XML'}
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
               <p class="text-sm font-semibold text-slate-50">
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

           <label class="flex items-center gap-3 cursor-pointer">
             <input type="checkbox" bind:checked={complianceMode} class="accent-cyan-500" />
             <span class="text-sm text-slate-300">Verify tamper-evident audit trail on open</span>
           </label>

           <div class="space-y-2">
             <div class="flex flex-wrap gap-2">
               <button
                 onclick={exportAuditVerificationReport}
                 disabled={auditReportBusy}
                 class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 text-slate-200 font-bold uppercase px-4 py-1.5 rounded"
               >
                 {auditReportBusy ? 'Exporting…' : 'Export audit verification report'}
               </button>
               <button
                 onclick={exportAuditRepairEvidence}
                 disabled={auditRepairBusy}
                 class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 text-slate-200 font-bold uppercase px-4 py-1.5 rounded"
               >
                 {auditRepairBusy ? 'Exporting…' : 'Export audit repair evidence'}
               </button>
             </div>
             {#if auditReportStatus}
               <p class="text-xs text-cyan-400">{auditReportStatus}</p>
             {/if}
             {#if auditReportError}
               <p class="text-xs text-red-400" role="alert">{auditReportError}</p>
             {/if}
             {#if auditRepairStatus}
               <p class="text-xs text-cyan-400">{auditRepairStatus}</p>
             {/if}
             {#if auditRepairError}
               <p class="text-xs text-red-400" role="alert">{auditRepairError}</p>
             {/if}
           </div>

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
             disabled={settingsBusy || settingsResetting}
             class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-4 py-1.5 rounded"
           >
             {settingsBusy ? 'Saving…' : 'Save export settings'}
           </button>
           <button
             onclick={resetProjectSettings}
             disabled={settingsBusy || settingsResetting}
             class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 text-slate-200 font-bold uppercase px-4 py-1.5 rounded"
           >
             {settingsResetting ? 'Resetting…' : 'Reset defaults'}
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
