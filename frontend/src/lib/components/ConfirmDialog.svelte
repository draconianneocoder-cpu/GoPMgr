<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Shared yes/no confirmation modal, replacing `window.confirm()` at call
  // sites where a destructive or hard-to-reverse action needs a real prompt.
  // Wails v2.13.0's darwin WKUIDelegate implements none of the JS
  // confirm/alert/prompt panel methods (verified: no `runJavaScript*Panel`
  // implementation anywhere in the vendored module), so `confirm()` silently
  // no-ops in the packaged macOS build — the calling code proceeds as if the
  // user declined, with no dialog ever appearing. Mirrors the structural/ARIA
  // pattern of App.svelte's "Unsaved changes" guard (the first proven,
  // packaged-build-verified replacement for this same defect class) rather
  // than inventing a new one.
  //
  // Dogfoods the shared Button component (design-critique Priority #2):
  // Cancel is variant="ghost" and Confirm is variant={tone === 'danger' ?
  // 'danger' : 'caution'}, both size="md" — reconstructs this file's own
  // original hand-written button classes exactly (proven by
  // Button.test.ts's two reconstruction tests), so this migration needed
  // zero `class` passthrough to preserve the appearance already
  // live-verified in a running packaged-adjacent app in an earlier pass.
  import Button from './Button.svelte';

  let {
    open,
    title,
    message,
    confirmLabel = 'Delete',
    cancelLabel = 'Cancel',
    tone = 'danger',
    busy = false,
    onConfirm,
    onCancel,
  }: {
    open: boolean;
    title: string;
    message: string;
    confirmLabel?: string;
    cancelLabel?: string;
    tone?: 'danger' | 'caution';
    busy?: boolean;
    onConfirm: () => void;
    onCancel: () => void;
  } = $props();
</script>

{#if open}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4"
    role="presentation"
    onkeydown={(e) => e.key === 'Escape' && !busy && onCancel()}
  >
    <div
      class="w-full max-w-md rounded-lg border border-slate-700 bg-slate-900 p-5 shadow-2xl"
      role="alertdialog"
      aria-modal="true"
      aria-labelledby="confirm-dialog-title"
      aria-describedby="confirm-dialog-description"
      tabindex="-1"
    >
      <h2 id="confirm-dialog-title" class="text-base font-bold text-slate-100">{title}</h2>
      <p id="confirm-dialog-description" class="mt-2 text-sm text-slate-400">{message}</p>
      <div class="mt-5 flex flex-wrap justify-end gap-2">
        <Button variant="ghost" size="md" onclick={onCancel} disabled={busy}>{cancelLabel}</Button>
        <Button variant={tone === 'danger' ? 'danger' : 'caution'} size="md" onclick={onConfirm} disabled={busy}>
          {busy ? 'Working…' : confirmLabel}
        </Button>
      </div>
    </div>
  </div>
{/if}
