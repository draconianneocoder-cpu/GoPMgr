<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Global, per-user application settings (distinct from per-project
  // settings). Edits the default font/theme applied to newly created
  // projects, and shows read-only environment info.
  import { onMount } from 'svelte';
  import AppHeader from './AppHeader.svelte';
  import { applyTheme } from '../theme';
  import { autosave } from '../autosave.svelte';
  import { session } from '../session.svelte';
  import { showToast } from '../toast.svelte';

  let info = $state<AppInfo | null>(null);
  let font = $state('');
  let theme = $state('');
  let appTheme = $state('dark');
  let autoSaveOn = $state(true);
  let autoSaveSeconds = $state(60);
  let loading = $state(true);
  let saving = $state(false);
  let resetting = $state(false);
  let status = $state('');
  let error = $state('');
  let openingLogs = $state(false);
  let generatingReport = $state(false);
  let reportPath = $state('');
  let diagError = $state('');
  let hasAdmin = $state(true);
  let claimingAdmin = $state(false);

  const themes = [
    { value: '', label: 'Modern (default)' },
    { value: 'classic', label: 'Classic' },
    { value: 'archival', label: 'Archival' },
  ];

  const autoSaveChoices = [
    { value: 15, label: 'Every 15 seconds' },
    { value: 30, label: 'Every 30 seconds' },
    { value: 60, label: 'Every minute' },
    { value: 120, label: 'Every 2 minutes' },
    { value: 300, label: 'Every 5 minutes' },
  ];

  onMount(async () => {
    await load();
    try { hasAdmin = await window.go.main.App.HasAnyAdmin(); } catch { hasAdmin = true; }
  });

  async function load() {
    loading = true;
    error = '';
    try {
      const i = await window.go.main.App.GetAppInfo();
      info = i;
      font = i.settings.default_font ?? '';
      theme = i.settings.default_theme ?? '';
      appTheme = i.settings.app_theme || 'dark';
      const secs = i.settings.auto_save_seconds ?? 0;
      autoSaveOn = secs > 0;
      autoSaveSeconds = secs > 0 ? secs : 60;
    } catch (err: any) {
      error = `Could not load settings: ${err}`;
    } finally {
      loading = false;
    }
  }

  // Preview the UI theme immediately as the user changes it.
  function previewTheme() {
    applyTheme(appTheme);
  }

  async function openLogsFolder() {
    openingLogs = true;
    diagError = '';
    try {
      await window.go.main.App.OpenLogsFolder();
    } catch (err: any) {
      diagError = `Could not open logs folder: ${err}`;
    } finally {
      openingLogs = false;
    }
  }

  async function claimAdmin() {
    claimingAdmin = true;
    try {
      await window.go.main.App.BecomeAdmin();
      hasAdmin = true;
      if (session.user) session.user = { ...session.user, is_admin: true };
      showToast('You are now the administrator.', 'success');
    } catch (err: any) {
      showToast(`Could not claim administrator: ${err}`, 'error');
    } finally {
      claimingAdmin = false;
    }
  }

  async function generateBugReport() {
    generatingReport = true;
    diagError = '';
    reportPath = '';
    try {
      reportPath = await window.go.main.App.GenerateBugReport();
    } catch (err: any) {
      diagError = `Could not generate report: ${err}`;
    } finally {
      generatingReport = false;
    }
  }

  async function save() {
    saving = true;
    status = '';
    error = '';
    const autoVal = autoSaveOn ? autoSaveSeconds : 0;
    try {
      await window.go.main.App.SaveAppSettings({
        default_font: font,
        default_theme: theme,
        app_theme: appTheme,
        auto_save_seconds: autoVal,
      });
      applyTheme(appTheme);
      autosave.setInterval(autoVal);
      status = 'Saved.';
    } catch (err: any) {
      error = `Save failed: ${err}`;
    } finally {
      saving = false;
    }
  }

  function applySettings(settings: AppSettings) {
    font = settings.default_font ?? '';
    theme = settings.default_theme ?? '';
    appTheme = settings.app_theme || 'dark';
    const secs = settings.auto_save_seconds ?? 0;
    autoSaveOn = secs > 0;
    autoSaveSeconds = secs > 0 ? secs : 60;
    applyTheme(appTheme);
    autosave.setInterval(secs);
  }

  async function resetDefaults() {
    resetting = true;
    status = '';
    error = '';
    try {
      const defaults = await window.go.main.App.ResetAppSettings();
      if (info) info = { ...info, settings: defaults };
      applySettings(defaults);
      status = 'Defaults restored.';
    } catch (err: any) {
      error = `Reset failed: ${err}`;
    } finally {
      resetting = false;
    }
  }
</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <AppHeader active="settings" />

  <main class="max-w-2xl mx-auto p-8">
    <h2 class="text-xl font-bold mb-1">Application settings</h2>
    <p class="text-xs text-slate-500 mb-6">
      App-level preferences for your account. New projects inherit these defaults.
    </p>

    {#if error}
      <p class="text-sm text-red-400 mb-4" role="alert">{error}</p>
    {/if}

    {#if loading}
      <p class="text-sm text-slate-500 text-center py-12" role="status" aria-live="polite">Loading…</p>
    {:else if info}
      <div class="space-y-6">
        <section class="p-4 bg-slate-900 border border-slate-800 rounded-lg space-y-4">
          <h3 class="text-xs font-bold uppercase tracking-widest text-cyan-400">Appearance</h3>
          <label class="block">
            <span class="text-xs font-semibold text-slate-500 uppercase">Application theme</span>
            <select
              bind:value={appTheme}
              onchange={previewTheme}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            >
              <option value="dark">Dark</option>
              <option value="light">Light</option>
            </select>
            <span class="mt-1 block text-[11px] text-slate-500">
              Applies immediately as a preview; click Save to keep it.
            </span>
          </label>
        </section>

        <section class="p-4 bg-slate-900 border border-slate-800 rounded-lg space-y-4">
          <h3 class="text-xs font-bold uppercase tracking-widest text-cyan-400">Saving</h3>
          <label class="flex items-center gap-2">
            <input type="checkbox" bind:checked={autoSaveOn} />
            <span class="text-xs font-semibold text-slate-300">Auto-save open editors</span>
          </label>
          <label class="block">
            <span class="text-xs font-semibold text-slate-500 uppercase">Auto-save interval</span>
            <select
              bind:value={autoSaveSeconds}
              disabled={!autoSaveOn}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none disabled:opacity-50"
            >
              {#each autoSaveChoices as c (c.value)}
                <option value={c.value}>{c.label}</option>
              {/each}
            </select>
            <span class="mt-1 block text-[11px] text-slate-500">
              Editors also save manually anytime with {'⌘'}S / Ctrl+S or the Save button.
              Auto-save only writes when there are unsaved changes.
            </span>
          </label>
        </section>

        <section class="p-4 bg-slate-900 border border-slate-800 rounded-lg space-y-4">
          <h3 class="text-xs font-bold uppercase tracking-widest text-cyan-400">
            Defaults for new projects
          </h3>

          <label class="block">
            <span class="text-xs font-semibold text-slate-500 uppercase">Default document font</span>
            <select
              bind:value={font}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            >
              <option value="">Catalog default</option>
              {#each info.fonts as f (f.name)}
                <option value={f.name}>{f.name}</option>
              {/each}
            </select>
          </label>

          <label class="block">
            <span class="text-xs font-semibold text-slate-500 uppercase">Default export theme</span>
            <select
              bind:value={theme}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            >
              {#each themes as t (t.value)}
                <option value={t.value}>{t.label}</option>
              {/each}
            </select>
          </label>
        </section>

        <div class="flex flex-wrap items-center gap-3 pt-2">
          <button
            onclick={save}
            disabled={saving || resetting}
            class="bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white text-xs font-bold uppercase tracking-wider px-4 py-2 rounded"
          >
            {saving ? 'Saving…' : 'Save settings'}
          </button>
          <button
            onclick={resetDefaults}
            disabled={saving || resetting}
            class="bg-slate-800 hover:bg-slate-700 disabled:opacity-50 text-slate-200 text-xs font-bold uppercase tracking-wider px-4 py-2 rounded"
          >
            {resetting ? 'Resetting…' : 'Reset defaults'}
          </button>
          {#if status}<span class="text-xs text-emerald-400">{status}</span>{/if}
        </div>

        {#if !hasAdmin && !session.user?.is_admin}
          <section class="p-4 bg-amber-950/30 border border-amber-700/50 rounded-lg space-y-3 text-xs">
            <h3 class="text-xs font-bold uppercase tracking-widest text-amber-400">No administrator configured</h3>
            <p class="text-amber-300/80">
              This machine has no PMForge administrator. An administrator can create and delete accounts
              and manage roles. Claim this role to take responsibility for managing users on this machine.
            </p>
            <button
              onclick={claimAdmin}
              disabled={claimingAdmin}
              class="bg-amber-700 hover:bg-amber-600 disabled:opacity-50 text-white text-xs font-bold uppercase tracking-wider px-4 py-2 rounded"
            >
              {claimingAdmin ? 'Claiming…' : 'Become administrator'}
            </button>
          </section>
        {/if}

        <section class="p-4 bg-slate-900 border border-slate-800 rounded-lg space-y-2 text-xs">
          <h3 class="text-xs font-bold uppercase tracking-widest text-slate-500">About</h3>
          <div class="flex justify-between gap-4">
            <span class="text-slate-500">Version</span><span class="font-mono">{info.version}</span>
          </div>
          <div class="flex justify-between gap-4">
            <span class="text-slate-500">Signed in as</span>
            <span class="font-mono truncate">{info.username}</span>
          </div>
          <div class="flex justify-between gap-4">
            <span class="text-slate-500">Data location</span>
            <span class="font-mono break-all text-right">{info.data_location}</span>
          </div>
        </section>

        <section class="p-4 bg-slate-900 border border-slate-800 rounded-lg space-y-3 text-xs">
          <h3 class="text-xs font-bold uppercase tracking-widest text-slate-500">Diagnostics</h3>
          {#if info.logs_dir}
            <div class="flex justify-between gap-4">
              <span class="text-slate-500 shrink-0">Log files</span>
              <span class="font-mono break-all text-right text-slate-400">{info.logs_dir}</span>
            </div>
          {/if}
          <div class="flex flex-wrap gap-2">
            <button
              onclick={openLogsFolder}
              disabled={openingLogs}
              class="bg-slate-800 hover:bg-slate-700 disabled:opacity-50 text-slate-200 text-xs font-semibold px-3 py-1.5 rounded"
            >
              {openingLogs ? 'Opening…' : 'Open logs folder'}
            </button>
            <button
              onclick={generateBugReport}
              disabled={generatingReport}
              class="bg-slate-800 hover:bg-slate-700 disabled:opacity-50 text-slate-200 text-xs font-semibold px-3 py-1.5 rounded"
            >
              {generatingReport ? 'Generating…' : 'Generate bug report'}
            </button>
          </div>
          {#if reportPath}
            <p class="text-[11px] text-emerald-400 break-all">Report saved: {reportPath}</p>
          {/if}
          {#if diagError}
            <p class="text-[11px] text-red-400" role="alert">{diagError}</p>
          {/if}
        </section>
      </div>
    {/if}
  </main>
</div>
