<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import SignatureSettings from './admin/SignatureSettings.svelte';
  import { onMount } from 'svelte';

  let password = $state('');
  let theme = $state<'modern' | 'classic' | 'archival'>('modern');
  let autoRepair = $state(true);
  let certPath = $state('');
  let signatureEnabled = $state(false);
  let statusMessage = $state('');

  onMount(async () => {
    if (window.go?.main?.App?.GetSettings) {
      try {
        const s = await window.go.main.App.GetSettings();
        password = s.default_password;
        theme = s.export_theme;
        autoRepair = s.auto_repair;
        certPath = s.cert_path;
        signatureEnabled = s.signature_enabled;
      } catch (e) {
        statusMessage = `Could not load settings: ${e}`;
      }
    }
  });

  async function save() {
    if (!window.go?.main?.App?.SaveSettings) {
      statusMessage = 'Wails bridge not available (run `wails dev`).';
      return;
    }
    try {
      await window.go.main.App.SaveSettings({
        default_password: password,
        export_theme: theme,
        auto_repair: autoRepair,
        cert_path: certPath,
        signature_enabled: signatureEnabled,
      });
      statusMessage = 'Settings saved successfully.';
    } catch (e) {
      statusMessage = `Error saving settings: ${e}`;
    }
  }
</script>

<div class="p-8 bg-slate-900 text-slate-200 rounded-xl border border-slate-700 shadow-xl max-w-md">
  <h2 class="text-xl font-bold mb-6 text-white uppercase tracking-widest">Global Settings</h2>

  <div class="space-y-4">
    <label class="block">
      <span class="text-xs font-semibold text-slate-500 uppercase">Default Encryption Password</span>
      <input
        type="password"
        bind:value={password}
        class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
      />
    </label>

    <label class="block">
      <span class="text-xs font-semibold text-slate-500 uppercase">Export Theme</span>
      <select
        bind:value={theme}
        class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded"
      >
        <option value="modern">Modern (Dark)</option>
        <option value="classic">Classic (Light)</option>
        <option value="archival">Archival (B&amp;W)</option>
      </select>
    </label>

    <label class="flex items-center gap-3 cursor-pointer">
      <input type="checkbox" bind:checked={autoRepair} class="accent-cyan-500" />
      <span class="text-sm">Enable Background Self-Healing</span>
    </label>

    <SignatureSettings bind:certPath bind:isSignatureEnabled={signatureEnabled} />

    <button
      onclick={save}
      class="w-full bg-cyan-600 hover:bg-cyan-500 text-white font-bold py-2 rounded transition-all mt-4"
    >
      APPLY CHANGES
    </button>

    {#if statusMessage}
      <p class="text-center text-xs text-cyan-400 mt-2">{statusMessage}</p>
    {/if}
  </div>
</div>
