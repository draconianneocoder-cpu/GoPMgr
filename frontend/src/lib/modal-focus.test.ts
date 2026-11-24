// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, describe, expect, it, vi } from 'vitest';
import { tick } from 'svelte';

import { modalFocus } from './modal-focus';

// The keyboard contract of every custom modal that uses modalFocus; each
// dialog's own test checks only that it uses it.
describe('modalFocus', () => {
  afterEach(() => {
    document.body.innerHTML = '';
  });

  function setup(dialogHTML: string) {
    document.body.innerHTML = `<button id="opener">Open</button><div id="dialog" tabindex="-1">${dialogHTML}</div><button id="behind">Behind</button>`;
    const opener = document.getElementById('opener') as HTMLButtonElement;
    const dialog = document.getElementById('dialog') as HTMLElement;
    opener.focus();
    return { opener, dialog };
  }

  function tab(node: HTMLElement, shiftKey = false) {
    node.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', shiftKey, bubbles: true, cancelable: true }));
  }

  it('moves focus to the first enabled control, keeps Tab inside, and restores focus on close', async () => {
    const { opener, dialog } = setup('<button disabled>Off</button><input id="first"><button id="last">Save</button>');
    const action = modalFocus(dialog);
    await tick();
    expect(document.activeElement?.id).toBe('first');

    (document.getElementById('last') as HTMLElement).focus();
    tab(dialog);
    expect(document.activeElement?.id).toBe('first');
    tab(dialog, true);
    expect(document.activeElement?.id).toBe('last');

    action.destroy();
    expect(document.activeElement).toBe(opener);
  });

  it('holds focus on the dialog when it has no enabled control', async () => {
    const { dialog } = setup('<button disabled>Saving…</button>');
    modalFocus(dialog);
    await tick();
    expect(document.activeElement).toBe(dialog);
    tab(dialog);
    expect(document.activeElement).toBe(dialog);
  });

  it('calls onEscape, and the latest one after an update', async () => {
    const { dialog } = setup('<button>Cancel</button>');
    const first = vi.fn();
    const second = vi.fn();
    const action = modalFocus(dialog, { onEscape: first });
    action.update({ onEscape: second });
    dialog.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }));
    expect(first).not.toHaveBeenCalled();
    expect(second).toHaveBeenCalledOnce();
  });

  it('restores focus to an SVG opener, such as a diagram node', async () => {
    document.body.innerHTML = '<svg><text id="node" tabindex="0">Cause</text></svg><div id="dialog" tabindex="-1"><button>OK</button></div>';
    const node = document.getElementById('node') as unknown as SVGElement;
    node.focus();
    const action = modalFocus(document.getElementById('dialog') as HTMLElement);
    await tick();
    action.destroy();
    expect(document.activeElement).toBe(node);
  });

  it('does not move focus to an opener that is gone or disabled', async () => {
    const { opener, dialog } = setup('<button id="inside">OK</button>');
    const action = modalFocus(dialog);
    await tick();
    opener.disabled = true;
    action.destroy();
    expect(document.activeElement).not.toBe(opener);
  });
});
