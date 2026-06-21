<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // SignatureSettings is a sub-panel of the global Settings dialog.
  // It binds the active signing certificate path and a master enable
  // toggle. The file picker is wired through the Wails runtime; until
  // that binding lands, the input accepts a typed-in path.
  let {
    certPath = $bindable(''),
    isSignatureEnabled = $bindable(false),
  } = $props();

  let certPassword = $state('');

  async function selectFile() {
    try {
      const picked = await window.go.main.App.ChooseCertFile();
      if (picked) certPath = picked;
    } catch {
      // User cancelled or the dialog isn't available — keep the
      // existing path so the user can still type one in.
    }
  }
</script>

<div class="mt-6 p-4 bg-slate-950 border border-slate-800 rounded-lg">
  <div class="flex items-center justify-between mb-4">
    <h3 class="text-xs font-bold text-cyan-500 uppercase tracking-widest">
      Digital Signatures (PDF/A)
    </h3>
    <input type="checkbox" bind:checked={isSignatureEnabled} class="accent-cyan-500" />
  </div>

  {#if isSignatureEnabled}
    <div class="space-y-3">
      <div class="flex gap-2">
        <input
          placeholder="Path to .p12 / .pfx"
          bind:value={certPath}
          class="flex-1 bg-slate-900 border border-slate-800 p-2 text-xs rounded"
        />
        <button
          onclick={selectFile}
          class="bg-slate-700 px-3 py-1 text-xs rounded hover:bg-slate-600"
        >
          BROWSE
        </button>
      </div>
      <input
        type="password"
        placeholder="Certificate Password"
        bind:value={certPassword}
        class="w-full bg-slate-900 border border-slate-800 p-2 text-xs rounded"
      />
      <p class="text-[10px] text-slate-500">
        The certificate password is not persisted; it is requested per signing operation.
      </p>
    </div>
  {/if}
</div>
