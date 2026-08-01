<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Global, per-user application settings (distinct from per-project
  // settings). Edits the default font/theme applied to newly created
  // projects, and shows read-only environment info.
  import { onDestroy, onMount } from 'svelte';
  import AppHeader from './AppHeader.svelte';
  import { applyTheme, rememberTheme, type AppTheme } from '../theme';
  import { autosave } from '../autosave.svelte';
  import { session } from '../session.svelte';
  import { showToast } from '../toast.svelte';

  let info = $state<AppInfo | null>(null);
  let font = $state('');
  let theme = $state('');
  let appTheme = $state<AppTheme>('dark');
  let committedAppTheme = $state<AppTheme>('dark');
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
  let updateStatus = $state<UpdateStatus | null>(null);
  let checkingUpdate = $state(false);

  const themes = [
    { value: '', label: 'Modern (default)' },
    { value: 'classic', label: 'Classic' },
    { value: 'archival', label: 'Archival' },
  ];

  const appearanceChoices: Array<{
    value: AppTheme;
    label: string;
    description: string;
  }> = [
    {
      value: 'dark',
      label: 'Dark workshop',
      description: 'Graphite surfaces with a bright blueprint accent.',
    },
    {
      value: 'light',
      label: 'Light fieldbook',
      description: 'Paper-light surfaces with an ink-blue accent.',
    },
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

  onDestroy(() => {
    applyTheme(committedAppTheme);
  });

  async function load() {
    loading = true;
    error = '';
    try {
      const i = await window.go.main.App.GetAppInfo();
      info = i;
      font = i.settings.default_font ?? '';
      theme = i.settings.default_theme ?? '';
      appTheme = i.settings.app_theme === 'light' ? 'light' : 'dark';
      committedAppTheme = appTheme;
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

  async function checkForUpdates() {
    checkingUpdate = true;
    try {
      updateStatus = await window.go.main.App.CheckLatestVersion();
    } catch (err: any) {
      updateStatus = {
        configured: true,
        current: info?.version ?? '',
        channel: 'unknown',
        update_available: false,
        error: String(err?.message ?? err),
      };
    } finally {
      checkingUpdate = false;
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
      rememberTheme(appTheme, session.user?.username);
      committedAppTheme = appTheme;
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
    appTheme = settings.app_theme === 'light' ? 'light' : 'dark';
    committedAppTheme = appTheme;
    const secs = settings.auto_save_seconds ?? 0;
    autoSaveOn = secs > 0;
    autoSaveSeconds = secs > 0 ? secs : 60;
    applyTheme(appTheme);
    rememberTheme(appTheme, session.user?.username);
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

  <main class="max-w-3xl mx-auto p-8">
    <h1 class="text-xl font-bold mb-1">Application settings</h1>
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
          <h2 class="text-xs font-bold uppercase tracking-widest text-cyan-400">Appearance</h2>
          <fieldset>
            <legend class="text-sm font-semibold text-slate-200">Application appearance</legend>
            <div class="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
              {#each appearanceChoices as choice (choice.value)}
                <label
                  class={`cursor-pointer rounded-lg border p-3 transition-colors ${
                    appTheme === choice.value
                      ? 'border-cyan-500 bg-cyan-950/20'
                      : 'border-slate-800 bg-slate-950 hover:border-slate-700'
                  }`}
                >
                  <span
                    class={`mb-3 block h-16 overflow-hidden rounded-md border ${
                      choice.value === 'dark'
                        ? 'border-[#334155] bg-[#020617]'
                        : 'border-[#cbd5e1] bg-[#f8fafc]'
                    }`}
                    aria-hidden="true"
                  >
                    <span
                      class={`flex h-5 items-center gap-1 border-b px-2 ${
                        choice.value === 'dark'
                          ? 'border-[#1e293b] bg-[#0f172a]'
                          : 'border-[#e2e8f0] bg-[#f1f5f9]'
                      }`}
                    >
                      <span class={`h-1.5 w-7 rounded-full ${choice.value === 'dark' ? 'bg-[#00d4ff]' : 'bg-[#0e7490]'}`}></span>
                      <span class={`h-1.5 w-3 rounded-full ${choice.value === 'dark' ? 'bg-[#334155]' : 'bg-[#cbd5e1]'}`}></span>
                    </span>
                    <span class="grid grid-cols-3 gap-1.5 p-2">
                      {#each [1, 2, 3] as item}
                        <span
                          class={`h-7 rounded border ${
                            choice.value === 'dark'
                              ? 'border-[#1e293b] bg-[#0f172a]'
                              : 'border-[#e2e8f0] bg-white'
                          }`}
                        ></span>
                      {/each}
                    </span>
                  </span>
                  <span class="flex items-start gap-2.5">
                    <input
                      type="radio"
                      name="app-theme"
                      value={choice.value}
                      bind:group={appTheme}
                      onchange={previewTheme}
                      class="mt-0.5 shrink-0"
                    />
                    <span>
                      <span class="block text-sm font-semibold text-slate-100">{choice.label}</span>
                      <span class="mt-0.5 block text-xs leading-relaxed text-slate-500">
                        {choice.description}
                      </span>
                    </span>
                  </span>
                </label>
              {/each}
            </div>
            <span class="mt-2 block text-xs text-slate-500">
              Applies immediately as a preview; click Save to keep it.
            </span>
          </fieldset>
        </section>

        <section class="p-4 bg-slate-900 border border-slate-800 rounded-lg space-y-4">
          <h2 class="text-xs font-bold uppercase tracking-widest text-cyan-400">Saving</h2>
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
          <h2 class="text-xs font-bold uppercase tracking-widest text-cyan-400">
            Defaults for new projects
          </h2>

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
            <span class="text-xs font-semibold text-slate-500 uppercase">Default report/PDF theme</span>
            <select
              bind:value={theme}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            >
              {#each themes as t (t.value)}
                <option value={t.value}>{t.label}</option>
              {/each}
            </select>
            <span class="mt-1 block text-[11px] text-slate-500">
              Controls exported documents only; it does not change the application appearance.
            </span>
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
            <h2 class="text-xs font-bold uppercase tracking-widest text-amber-400">No administrator configured</h2>
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
          <h2 class="text-xs font-bold uppercase tracking-widest text-cyan-400">Beta Center</h2>
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
          <p class="pt-2 text-[11px] leading-relaxed text-amber-300/80">
            Beta packages may be unsigned. Windows and Fedora lifecycle validation and publicly trusted PAdES evidence remain release limitations until their native evidence is recorded.
          </p>
          <button onclick={checkForUpdates} disabled={checkingUpdate} class="mt-2 bg-slate-800 hover:bg-slate-700 disabled:opacity-50 text-slate-200 text-xs font-semibold px-3 py-1.5 rounded">
            {checkingUpdate ? 'Checking…' : 'Check for updates'}
          </button>
          {#if updateStatus}
            <div class="mt-2 rounded border border-slate-800 bg-slate-950 p-3 space-y-1">
              <p>Channel: <span class="font-mono">{updateStatus.channel || 'unconfigured'}</span></p>
              {#if !updateStatus.configured}<p class="text-slate-400">This build has no automatic update channel configured.</p>
              {:else if updateStatus.error}<p class="text-red-400" role="alert">Verification failed: {updateStatus.error}</p>
              {:else if updateStatus.update_available}<p class="text-cyan-300">Update available: {updateStatus.latest}</p>
              {:else}<p class="text-emerald-400">This build is current.</p>{/if}
              {#if updateStatus.sha256}<p class="break-all font-mono text-[10px] text-slate-500">SHA-256: {updateStatus.sha256}</p>{/if}
            </div>
          {/if}
        </section>

        <section class="p-4 bg-slate-900 border border-slate-800 rounded-lg space-y-3 text-xs">
          <h2 class="text-xs font-bold uppercase tracking-widest text-slate-500">Diagnostics</h2>
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
