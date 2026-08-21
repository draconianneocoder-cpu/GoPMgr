<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onDestroy, tick } from 'svelte';

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

  let dialogEl = $state<HTMLElement>();
  let previouslyFocused: HTMLElement | null = null;

  const focusableSelector =
    'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

  function getFocusable(): HTMLElement[] {
    if (!dialogEl) return [];
    return Array.from(dialogEl.querySelectorAll<HTMLElement>(focusableSelector));
  }

  function focusFirst() {
    (getFocusable()[0] ?? dialogEl)?.focus();
  }

  function restoreFocus() {
    if (previouslyFocused?.isConnected && !previouslyFocused.matches(':disabled, [inert]')) {
      previouslyFocused.focus();
    }
    previouslyFocused = null;
  }

  // A native confirm dialog owns focus while open. Mirror that behavior so
  // keyboard users cannot reach the concealed page and can resume at the
  // control that requested confirmation.
  $effect(() => {
    if (!open) {
      restoreFocus();
      return;
    }

    previouslyFocused = document.activeElement as HTMLElement | null;
    let active = true;
    tick().then(() => {
      if (active && open) focusFirst();
    });
    return () => {
      active = false;
    };
  });

  onDestroy(restoreFocus);

  function onDialogKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      if (!busy) onCancel();
      return;
    }
    if (event.key !== 'Tab') return;

    const focusable = getFocusable();
    if (focusable.length === 0) {
      event.preventDefault();
      dialogEl?.focus();
      return;
    }
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    const active = document.activeElement;
    if (event.shiftKey) {
      if (active === first || active === dialogEl) {
        event.preventDefault();
        last.focus();
      }
    } else if (active === last) {
      event.preventDefault();
      first.focus();
    }
  }
</script>

{#if open}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4"
    role="presentation"
  >
    <div
      bind:this={dialogEl}
      class="w-full max-w-md rounded-lg border border-slate-700 bg-slate-900 p-5 shadow-2xl"
      role="alertdialog"
      aria-modal="true"
      aria-labelledby="confirm-dialog-title"
      aria-describedby="confirm-dialog-description"
      tabindex="-1"
      onkeydown={onDialogKeydown}
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
