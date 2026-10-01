<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onDestroy } from 'svelte';
  import Button from '../Button.svelte';

  // Shows a set of recovery codes with Copy, Save as .txt, and a printable
  // PDF sheet, for account creation, the Admin panel, and renewal. username
  // names the account the codes belong to (in the Admin panel, not the
  // signed-in user). savedHint, when given, is added after the saved
  // message; onsaved lets a parent track every file, since renewed codes are
  // not in use until the parent confirms them. A parent that swaps in a new
  // set while this stays mounted should wrap it in {#key} so the last save's
  // message does not carry over.
  let {
    username,
    codes,
    savedHint = '',
    onsaved,
  }: {
    username: string;
    codes: string[];
    savedHint?: string;
    onsaved?: (path: string) => void;
  } = $props();

  let copied = $state(false);
  // One save dialog at a time, whichever button opened it.
  let saving = $state<'' | 'text' | 'sheet'>('');
  let savedPath = $state('');
  let savedKind = $state<'text' | 'sheet'>('text');
  let saveError = $state('');
  // DEVELOPER_HANDBOOK.md §10.5: every timer must be cleared on destroy.
  let copiedTimer: ReturnType<typeof setTimeout> | null = null;
  onDestroy(() => {
    if (copiedTimer) clearTimeout(copiedTimer);
  });

  async function copyCodes() {
    try {
      await navigator.clipboard.writeText(codes.join('\n'));
      copied = true;
      if (copiedTimer) clearTimeout(copiedTimer);
      copiedTimer = setTimeout(() => (copied = false), 2000);
    } catch {
      // Clipboard may be unavailable; the codes stay visible to copy by hand.
    }
  }

  // Saved through the desktop save dialog the other exports use; a browser
  // blob download was never shown to produce a file in the app window.
  async function saveCodes(kind: 'text' | 'sheet') {
    if (saving) return;
    saving = kind;
    saveError = '';
    try {
      savedPath = await (kind === 'sheet'
        ? window.go.main.App.SaveRecoverySheetPDF(username, codes)
        : window.go.main.App.SaveRecoveryCodesFile(username, codes));
      savedKind = kind;
      onsaved?.(savedPath);
    } catch (err: any) {
      const message = String(err?.message ?? err);
      if (message.includes('export cancelled')) return;
      saveError = message.includes('export destination already exists')
        ? 'A file with that name already exists, and GoPMgr never replaces files. Save again with a new name.'
        : `Could not save the codes: ${message}`;
    } finally {
      saving = '';
    }
  }

  // A printed sheet is the copy away from this computer, so its file should
  // not stay behind once printed.
  const savedMessage = $derived(
    `Saved to ${savedPath}. ${savedKind === 'sheet' ? 'Print it, then delete the file.' : 'Keep a copy somewhere other than this computer.'}${savedHint ? ` ${savedHint}` : ''}`,
  );
</script>

<div class="space-y-2">
  <ul class="grid grid-cols-1 sm:grid-cols-2 gap-1.5 font-mono text-sm text-slate-100 bg-slate-950 border border-slate-800 rounded-lg p-3">
    {#each codes as code (code)}
      <li>{code}</li>
    {/each}
  </ul>
  <div class="flex flex-wrap gap-2">
    <Button size="compact" class="font-semibold uppercase tracking-wide text-slate-100" onclick={copyCodes}>
      {copied ? 'Copied ✓' : 'Copy'}
    </Button>
    <Button size="compact" class="font-semibold uppercase tracking-wide text-slate-100" onclick={() => saveCodes('text')} disabled={saving !== ''}>
      {saving === 'text' ? 'Saving…' : 'Save as .txt…'}
    </Button>
    <Button size="compact" class="font-semibold uppercase tracking-wide text-slate-100" onclick={() => saveCodes('sheet')} disabled={saving !== ''}>
      {saving === 'sheet' ? 'Saving…' : 'Save printable PDF…'}
    </Button>
  </div>
  <p class="text-[11px] text-slate-400 min-h-[1rem] break-all" aria-live="polite">
    {copied ? 'Recovery codes copied to the clipboard.' : savedPath ? savedMessage : ''}
  </p>
  {#if saveError}
    <p class="text-[11px] text-red-400" role="alert">{saveError}</p>
  {/if}
</div>
