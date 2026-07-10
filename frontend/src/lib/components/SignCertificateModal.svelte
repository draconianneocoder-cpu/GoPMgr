<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { tick } from 'svelte';

  // Reusable modal for choosing document export signing behavior.
  // Usage:
  // <SignCertificateModal
  //   bind:open={showModal}
  //   certPath={currentCertPath}
  //   onConfirm={(options) => doSignedExport(options)}
  // />

  let {
    open = $bindable(false),
    certPath = '',
    method = 'pades',
    gpgKeyID = '',
    onConfirm,
  }: {
    open: boolean;
    certPath?: string;
    method?: SignatureMethod;
    gpgKeyID?: string;
    onConfirm: (options: SignatureExportOptions) => void;
  } = $props();

  let password = $state('');
  let selectedMethod = $state<SignatureMethod>('pades');
  // A certificate the user picked in this dialog overrides the prop.
  let chosenPath = $state('');
  let selectedGPGKeyID = $state('');
  let chooseError = $state('');
  // Focus management: the dialog element and the control that was focused
  // before the modal opened (so we can restore focus on close).
  let dialogEl = $state<HTMLElement>();
  let previouslyFocused: HTMLElement | null = null;
  const effectivePath = $derived(chosenPath || certPath);
  const canConfirm = $derived(
    selectedMethod === 'none' ||
      selectedMethod === 'gpg' ||
      (selectedMethod === 'pades' && Boolean(password && effectivePath)),
  );

  $effect(() => {
    if (open) {
      selectedMethod = method;
      selectedGPGKeyID = gpgKeyID;
      chosenPath = '';
      chooseError = '';
    }
  });

  // Move focus into the dialog when it opens and restore it to the trigger
  // when it closes, so keyboard and screen-reader users are not stranded.
  $effect(() => {
    if (open) {
      previouslyFocused = document.activeElement as HTMLElement | null;
      tick().then(focusFirst);
    } else if (previouslyFocused) {
      previouslyFocused.focus?.();
      previouslyFocused = null;
    }
  });

  const FOCUSABLE =
    'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

  function getFocusable(): HTMLElement[] {
    if (!dialogEl) return [];
    return Array.from(dialogEl.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
      (el) => el.offsetParent !== null || el === document.activeElement,
    );
  }

  function focusFirst() {
    const focusable = getFocusable();
    (focusable[0] ?? dialogEl)?.focus();
  }

  // Dialog-wide keyboard handling: Escape closes from anywhere in the dialog,
  // and Tab is trapped so focus cycles within the modal instead of escaping
  // to the (inert) page behind it.
  function onDialogKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.preventDefault();
      cancel();
      return;
    }
    if (e.key !== 'Tab') return;
    const focusable = getFocusable();
    if (focusable.length === 0) {
      e.preventDefault();
      return;
    }
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    const active = document.activeElement;
    if (e.shiftKey) {
      if (active === first || active === dialogEl) {
        e.preventDefault();
        last.focus();
      }
    } else if (active === last) {
      e.preventDefault();
      first.focus();
    }
  }

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
    if (!canConfirm) return;
    onConfirm({
      method: selectedMethod,
      cert_path: effectivePath,
      cert_password: selectedMethod === 'pades' ? password : '',
      gpg_key_id: selectedMethod === 'gpg' ? selectedGPGKeyID.trim() : '',
    });
    // Parent is responsible for closing the modal via bind:open.
    password = '';
  }

  function cancel() {
    password = '';
    chosenPath = '';
    selectedGPGKeyID = gpgKeyID;
    chooseError = '';
    open = false;
  }

  // Allow Enter key to confirm from a text input. Escape is handled at the
  // dialog level (onDialogKeydown) so it works regardless of focus.
  function handleKey(e: KeyboardEvent) {
    if (e.key === 'Enter' && canConfirm) {
      confirm();
    }
  }
</script>

{#if open}
  <div class="fixed inset-0 flex items-center justify-center z-50">
    <button
      type="button"
      tabindex="-1"
      class="absolute inset-0 bg-black/60"
      aria-label="Cancel digital signature"
      onclick={cancel}
    ></button>
    <div
      bind:this={dialogEl}
      class="relative bg-slate-900 border border-slate-700 rounded-lg p-6 w-full max-w-md mx-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="signature-modal-title"
      tabindex="-1"
      onkeydown={onDialogKeydown}
    >
      <h3 id="signature-modal-title" class="text-sm font-bold uppercase tracking-widest text-cyan-400 mb-4">
        Signature Options
      </h3>

      <div class="space-y-4">
        <div>
          <label for="signature-method" class="text-xs text-slate-400">Signing method</label>
          <select
            id="signature-method"
            bind:value={selectedMethod}
            class="w-full mt-1 bg-slate-800 border border-slate-700 p-2 rounded text-sm focus:border-cyan-500 outline-none"
          >
            <option value="pades">PAdES digital signature</option>
            <option value="gpg">GnuPG detached signature</option>
            <option value="none">No digital signature</option>
          </select>
        </div>

        {#if selectedMethod === 'pades'}
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
        {:else if selectedMethod === 'gpg'}
        <div>
          <label for="gpg-key-id" class="text-xs text-slate-400">GnuPG key ID</label>
          <input
            id="gpg-key-id"
            type="text"
            bind:value={selectedGPGKeyID}
            onkeydown={handleKey}
            class="w-full mt-1 bg-slate-800 border border-slate-700 p-2 rounded text-sm focus:border-cyan-500 outline-none"
            placeholder="Optional; blank uses your default GnuPG key"
          />
          <p class="mt-2 text-[11px] text-slate-500">
            PMForge writes an ASCII-armored detached .asc signature next to the PDF.
          </p>
        </div>
        {:else}
        <p class="text-xs text-slate-400">
          Exports a plain PDF with no digital signature for print and physical sign-off.
        </p>
        {/if}
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
          disabled={!canConfirm}
          title={selectedMethod === 'pades' && !effectivePath ? 'Choose a certificate first' : undefined}
          class="text-xs px-4 py-1.5 rounded bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white font-bold"
        >
          Export
        </button>
      </div>

      <p class="mt-4 text-[10px] text-slate-500">
        Certificate passwords are used only for this operation and never stored.
      </p>
    </div>
  </div>
{/if}
