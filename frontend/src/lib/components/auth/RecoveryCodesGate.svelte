<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import RecoveryCodesPanel from './RecoveryCodesPanel.svelte';
  import type { RecoveryGate } from '../../recovery-gate';

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
      error = `Could not continue: ${err?.message ?? err}`;
    } finally {
      busy = false;
    }
  }
</script>

<section
  class="border border-amber-700/50 bg-amber-950/20 rounded-lg p-4 space-y-3"
  aria-labelledby="recovery-gate-heading"
>
  <h3 id="recovery-gate-heading" bind:this={heading} tabindex="-1" class="text-sm font-semibold text-amber-200 outline-none">Before you continue: save a way back in</h3>
  {#if variant === 'legacy'}
    <p class="text-xs text-slate-300">
      Your recovery codes are from an older version of GoPMgr and can't recover encrypted projects.
      Create new ones below; GoPMgr carries on as soon as they're saved.
    </p>
  {:else}
    <p class="text-xs text-slate-300">
      Projects are encrypted, and only your password or a recovery code opens them. You have no unused
      recovery codes, so if you forget your password this project can't be recovered. Create codes
      below; GoPMgr carries on as soon as they're saved.
    </p>
  {/if}

  <RecoveryCodesPanel onrenewed={onready} />

  {#if variant === 'none'}
    <div class="border-t border-slate-800 pt-3 space-y-2">
      <label class="flex items-start gap-2 text-xs text-slate-300 select-none">
        <input type="checkbox" bind:checked={understood} class="mt-0.5 accent-amber-500" />
        I understand that projects I create or encrypt until I sign out can't be recovered if I forget my password.
      </label>
      <button
        type="button"
        onclick={continueWithout}
        disabled={busy || !understood}
        class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 disabled:cursor-not-allowed border border-slate-700 px-3 py-1.5 rounded"
      >Continue without recovery codes</button>
    </div>
  {/if}

  {#if error}
    <p class="text-xs text-red-400" role="alert">{error}</p>
  {/if}
  <button type="button" onclick={oncancel} class="text-xs text-slate-400 hover:text-slate-200 underline">Cancel</button>
</section>
