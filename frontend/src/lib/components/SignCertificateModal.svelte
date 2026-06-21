<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Simple reusable modal for entering certificate password for signing.
  // Usage:
  // <SignCertificateModal
  //   bind:open={showModal}
  //   certPath={currentCertPath}
  //   onConfirm={(pwd, certPath) => doSignedExport(pwd, certPath)}
  // />

  let {
    open = $bindable(false),
    certPath = '',
    onConfirm,
  }: {
    open: boolean;
    certPath?: string;
    // Receives the entered password and the effective certificate path
    // (the one passed in, or one the user picked via "Choose certificate").
    onConfirm: (password: string, certPath: string) => void;
  } = $props();

  let password = $state('');
  // A certificate the user picked in this dialog overrides the prop.
  let chosenPath = $state('');
  let chooseError = $state('');
  const effectivePath = $derived(chosenPath || certPath);

  async function chooseCert() {
    chooseError = '';
    try {
      const picked = await window.go.main.App.ChooseCertFile();
      if (picked) chosenPath = picked;
    } catch (err: any) {
      chooseError = `Could not choose certificate: ${err}`;
    }
  }

  function confirm() {
    if (!password || !effectivePath) return;
    onConfirm(password, effectivePath);
    // Parent is responsible for closing the modal via bind:open.
    password = '';
  }

  function cancel() {
    password = '';
    chosenPath = '';
    chooseError = '';
    open = false;
  }

  // Allow Enter key to confirm
  function handleKey(e: KeyboardEvent) {
    if (e.key === 'Enter' && password) {
      confirm();
    } else if (e.key === 'Escape') {
      cancel();
    }
  }
</script>

{#if open}
  <div class="fixed inset-0 flex items-center justify-center z-50">
    <button
      type="button"
      class="absolute inset-0 bg-black/60"
      aria-label="Cancel digital signature"
      onclick={cancel}
    ></button>
    <div
      class="relative bg-slate-900 border border-slate-700 rounded-lg p-6 w-full max-w-md mx-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="signature-modal-title"
    >
      <h3 id="signature-modal-title" class="text-sm font-bold uppercase tracking-widest text-cyan-400 mb-4">
        Digital Signature
      </h3>

      <div class="space-y-4">
        <div>
          <div class="flex items-center justify-between">
            <div class="text-xs text-slate-400">Certificate</div>
            <button
              type="button"
              onclick={chooseCert}
              class="text-[11px] uppercase tracking-wider px-2 py-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-300"
            >
              Choose certificate…
            </button>
          </div>
          <div
            class="mt-1 text-xs bg-slate-800 border border-slate-700 p-2 rounded font-mono break-all"
            class:text-slate-500={!effectivePath}
          >
            {effectivePath || '(no certificate configured)'}
          </div>
          {#if chooseError}
            <p class="mt-1 text-[11px] text-red-400" role="alert">{chooseError}</p>
          {/if}
        </div>

        <div>
          <label for="sign-certificate-password" class="text-xs text-slate-400">Password</label>
          <input
            id="sign-certificate-password"
            type="password"
            bind:value={password}
            onkeydown={handleKey}
            class="w-full mt-1 bg-slate-800 border border-slate-700 p-2 rounded text-sm focus:border-cyan-500 outline-none"
            placeholder="Enter certificate password"
          />
        </div>
      </div>

      <div class="mt-6 flex gap-3 justify-end">
        <button
          onclick={cancel}
          class="text-xs px-4 py-1.5 rounded bg-slate-800 hover:bg-slate-700"
        >
          Cancel
        </button>
        <button
          onclick={confirm}
          disabled={!password || !effectivePath}
          title={!effectivePath ? 'Choose a certificate first' : undefined}
          class="text-xs px-4 py-1.5 rounded bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white font-bold"
        >
          Sign & Export
        </button>
      </div>

      <p class="mt-4 text-[10px] text-slate-500">
        Password is used only for this operation and never stored.
      </p>
    </div>
  </div>
{/if}
