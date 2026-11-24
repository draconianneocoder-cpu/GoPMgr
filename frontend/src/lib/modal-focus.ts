// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { tick } from 'svelte';

// modalFocus gives a modal dialog the keyboard behaviour ConfirmDialog
// established: focus moves into the dialog when it opens, Tab and Shift+Tab
// stay inside it, and focus returns to the control that opened it when it
// closes. `aria-modal` alone does none of this, so without it keyboard
// users keep typing into the page the dialog hides.
//
// Use it on the dialog element of a dialog rendered with {#if}: the action's
// mount and destroy are the dialog's open and close. The element should
// carry tabindex="-1" so it can hold focus when it has no enabled control.
// onEscape, when given, is called for Escape; dialogs that already handle
// Escape themselves leave it out.
export interface ModalFocusOptions {
  onEscape?: () => void;
}

const focusableSelector =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

export function modalFocus(node: HTMLElement, options: ModalFocusOptions = {}) {
  let current = options;
  // Diagram nodes are focusable SVG elements, so either kind can open one.
  const active = document.activeElement;
  const previouslyFocused = active instanceof HTMLElement || active instanceof SVGElement ? active : null;

  const focusable = () => Array.from(node.querySelectorAll<HTMLElement>(focusableSelector));

  let open = true;
  void tick().then(() => {
    if (open && !node.contains(document.activeElement)) (focusable()[0] ?? node).focus();
  });

  function onKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape' && current.onEscape) {
      event.preventDefault();
      current.onEscape();
      return;
    }
    if (event.key !== 'Tab') return;
    const controls = focusable();
    if (controls.length === 0) {
      event.preventDefault();
      node.focus();
      return;
    }
    const first = controls[0];
    const last = controls[controls.length - 1];
    const active = document.activeElement;
    if (event.shiftKey && (active === first || active === node)) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && active === last) {
      event.preventDefault();
      first.focus();
    }
  }
  node.addEventListener('keydown', onKeydown);

  return {
    update(next: ModalFocusOptions = {}) {
      current = next;
    },
    destroy() {
      open = false;
      node.removeEventListener('keydown', onKeydown);
      if (previouslyFocused?.isConnected && !previouslyFocused.matches(':disabled, [inert]')) {
        previouslyFocused.focus();
      }
    },
  };
}
