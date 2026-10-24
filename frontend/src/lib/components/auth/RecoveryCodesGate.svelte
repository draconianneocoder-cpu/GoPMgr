<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import RecoveryCodesPanel from './RecoveryCodesPanel.svelte';
  import type { RecoveryGate } from '../../recovery-gate';
  import { t } from '../../i18n/i18n.svelte';

  // Shown before creating or encrypting a project when the user has no
  // working recovery codes. Saving new codes, or (with no codes at all)
  // accepting to go without them, calls onready so the caller can carry on.
  // Legacy codes cannot be skipped: a reset with them would lose the key.
  let {
    variant,
    onready,
    oncancel,
  }: { variant: RecoveryGate; onready: () => void; oncancel: () => void } = $props();

  let understood = $state(false);
  // The gate appears below the button that opened it, so move focus to its
  // heading for keyboard and screen-reader users.
  let heading = $state<HTMLHeadingElement>();
  onMount(() => heading?.focus());
  let busy = $state(false);
  let error = $state('');

  async function continueWithout() {
    if (busy || !understood || variant !== 'none') return;
    busy = true;
    error = '';
    try {
      await window.go.main.App.AcceptEncryptionWithoutRecoveryCodes();
      onready();
    } catch (err: any) {
      error = t('recoveryGate.error', { error: String(err?.message ?? err) });
    } finally {
      busy = false;
    }
  }
</script>

<section
  class="border border-amber-700/50 bg-amber-950/20 rounded-lg p-4 space-y-3"
  aria-labelledby="recovery-gate-heading"
>
  <h3 id="recovery-gate-heading" bind:this={heading} tabindex="-1" class="text-sm font-semibold text-amber-200 outline-none">{t('recoveryGate.title')}</h3>
  {#if variant === 'legacy'}
    <p class="text-xs text-slate-300">{t('recoveryGate.legacy')}</p>
  {:else}
    <p class="text-xs text-slate-300">{t('recoveryGate.none')}</p>
  {/if}

  <RecoveryCodesPanel onrenewed={onready} />

  {#if variant === 'none'}
    <div class="border-t border-slate-800 pt-3 space-y-2">
      <label class="flex items-start gap-2 text-xs text-slate-300 select-none">
        <input type="checkbox" bind:checked={understood} class="mt-0.5 accent-amber-500" />
        {t('recoveryGate.understood')}
      </label>
      <button
        type="button"
        onclick={continueWithout}
        disabled={busy || !understood}
        class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 disabled:cursor-not-allowed border border-slate-700 px-3 py-1.5 rounded"
      >{t('recoveryGate.continueWithout')}</button>
    </div>
  {/if}

  {#if error}
    <p class="text-xs text-red-400" role="alert">{error}</p>
  {/if}
  <button type="button" onclick={oncancel} class="text-xs text-slate-400 hover:text-slate-200 underline">{t('common.cancel')}</button>
</section>
