// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/svelte';

import ButtonChildrenHarness from './__test_harness__/ButtonChildrenHarness.svelte';

describe('Button', () => {
  it('renders children and defaults to a secondary, medium, type="button" button', () => {
    const { getByText } = render(ButtonChildrenHarness, { props: { text: 'Save' } });
    const btn = getByText('Save') as HTMLButtonElement;
    expect(btn.tagName).toBe('BUTTON');
    expect(btn.type).toBe('button');
    expect(btn.className).toContain('bg-slate-800');
    expect(btn.className).toContain('px-3 py-2');
  });

  it('fires onclick', async () => {
    const onclick = vi.fn();
    const { getByText } = render(ButtonChildrenHarness, { props: { text: 'Go', onclick } });
    await fireEvent.click(getByText('Go'));
    expect(onclick).toHaveBeenCalledOnce();
  });

  it('forwards disabled to the real button element', () => {
    const { getByText } = render(ButtonChildrenHarness, { props: { text: 'Go', disabled: true } });
    expect((getByText('Go') as HTMLButtonElement).disabled).toBe(true);
  });

  it('applies the primary variant classes (solid cyan, white bold uppercase)', () => {
    const { getByText } = render(ButtonChildrenHarness, { props: { text: 'Save', variant: 'primary' } });
    const btn = getByText('Save');
    expect(btn.className).toContain('bg-cyan-600');
    expect(btn.className).toContain('text-white');
    expect(btn.className).toContain('uppercase');
  });

  it('applies the danger variant classes', () => {
    const { getByText } = render(ButtonChildrenHarness, { props: { text: 'Delete', variant: 'danger' } });
    expect(getByText('Delete').className).toContain('bg-red-700');
  });

  it('applies the caution variant classes', () => {
    const { getByText } = render(ButtonChildrenHarness, { props: { text: 'Complete', variant: 'caution' } });
    expect(getByText('Complete').className).toContain('bg-amber-700');
  });

  it('applies the ghost variant classes (transparent at rest, hover background)', () => {
    const { getByText } = render(ButtonChildrenHarness, { props: { text: 'Cancel', variant: 'ghost' } });
    const btn = getByText('Cancel');
    expect(btn.className).toContain('text-slate-300');
    expect(btn.className).toContain('hover:bg-slate-800');
    // Confirms there's no *solid* background utility (e.g. secondary's
    // `bg-slate-800`) sitting alongside the hover one — ghost is
    // transparent at rest. The exact-reconstruction test below pins the
    // full class set precisely.
    expect(btn.className.split(/\s+/)).not.toContain('bg-slate-800');
  });

  it('reconstructs ConfirmDialog\'s original Cancel button string exactly via ghost + md', () => {
    const { getByText } = render(ButtonChildrenHarness, { props: { text: 'Cancel', variant: 'ghost' } });
    const classes = getByText('Cancel').className.split(/\s+/).filter(Boolean).sort();
    const expected = 'rounded px-3 py-2 text-xs text-slate-300 hover:bg-slate-800 disabled:opacity-50'
      .split(/\s+/)
      .sort();
    expect(classes).toEqual(expected);
  });

  it('reconstructs ConfirmDialog\'s original danger Confirm button string exactly (no uppercase)', () => {
    const { getByText } = render(ButtonChildrenHarness, { props: { text: 'Delete', variant: 'danger' } });
    const classes = getByText('Delete').className.split(/\s+/).filter(Boolean).sort();
    const expected =
      'rounded px-3 py-2 text-xs font-bold text-white disabled:opacity-50 bg-red-700 hover:bg-red-600'
        .split(/\s+/)
        .sort();
    expect(classes).toEqual(expected);
  });

  it('the link variant ignores size and renders no padding/rounded/base text-size classes', () => {
    const { getByText } = render(ButtonChildrenHarness, {
      props: { text: 'See Backlog', variant: 'link', size: 'lg' },
    });
    const btn = getByText('See Backlog');
    expect(btn.className.trim()).toBe('text-cyan-400 underline hover:text-cyan-300 disabled:opacity-50');
  });

  it('sizes: sm is px-3 py-1, compact is px-3 py-1.5, md is px-3 py-2, lg is px-4 py-2', () => {
    const { getByText, rerender } = render(ButtonChildrenHarness, { props: { text: 'X', size: 'sm' } });
    expect(getByText('X').className).toContain('px-3 py-1');
    rerender({ text: 'X', size: 'compact' });
    // 'px-3 py-1' is a prefix of 'px-3 py-1.5', so assert py-1.5
    // specifically rather than the ambiguous 'py-1' substring.
    expect(getByText('X').className).toContain('px-3 py-1.5');
    rerender({ text: 'X', size: 'lg' });
    expect(getByText('X').className).toContain('px-4 py-2');
  });

  // Added 2026-08-19 for the canvas-toolbar idiom (WorkflowEditor/
  // ActivityEditor's "Connect…"/"Delete node"): a real, grep-confirmed,
  // app-wide-reused second disabled-opacity idiom (30%, not 50%) distinct
  // from every other variant. disabled:opacity-50 was relocated out of the
  // shared base template into each of the five pre-existing variantClasses
  // entries to make room for this — verified as a pure relocation (no
  // rendered-output change) by running the full pre-existing suite above
  // unmodified against the relocated code before adding these two cases.
  describe('canvas / canvas-danger variants', () => {
    it('canvas: bg-slate-800, hover:bg-slate-700, and — the actual differentiator from secondary — disabled:opacity-30 while disabled', () => {
      const { getByText } = render(ButtonChildrenHarness, {
        props: { text: 'Connect…', variant: 'canvas', size: 'sm', disabled: true },
      });
      const classes = getByText('Connect…').className.split(/\s+/).filter(Boolean).sort();
      const expected = 'rounded text-xs px-3 py-1 bg-slate-800 hover:bg-slate-700 disabled:opacity-30'
        .split(/\s+/)
        .sort();
      expect(classes).toEqual(expected);
    });

    it('canvas-danger: bg-slate-800, hover:bg-red-900, disabled:opacity-30 while disabled — distinct from the solid-red always-visible `danger` variant', () => {
      const { getByText } = render(ButtonChildrenHarness, {
        props: { text: 'Delete node', variant: 'canvas-danger', size: 'sm', disabled: true },
      });
      const classes = getByText('Delete node').className.split(/\s+/).filter(Boolean).sort();
      const expected = 'rounded text-xs px-3 py-1 bg-slate-800 hover:bg-red-900 disabled:opacity-30'
        .split(/\s+/)
        .sort();
      expect(classes).toEqual(expected);
    });
  });

  it('appends a passed-through class without dropping the base classes', () => {
    const { getByText } = render(ButtonChildrenHarness, { props: { text: 'X', class: 'mt-6' } });
    expect(getByText('X').className).toContain('mt-6');
    expect(getByText('X').className).toContain('rounded');
  });

  it('type="submit" is forwarded', () => {
    const { getByText } = render(ButtonChildrenHarness, { props: { text: 'Submit', type: 'submit' } });
    expect((getByText('Submit') as HTMLButtonElement).type).toBe('submit');
  });

  // Added 2026-08-18 for the app's 21 aria-label="Remove …" inline ×
  // buttons across 14 chart editors (docs/beta-release-backlog.md's
  // Priority #2 row): those buttons could not migrate to Button as built
  // because Button had no `...rest` prop spread, so aria-label/title
  // (their only accessible name -- the visible content is a bare "×") could
  // not reach the rendered element. Fixed by widening Button's props type
  // to intersect with svelte/elements' HTMLButtonAttributes.
  describe('remove variant', () => {
    it('reconstructs the original hand-written class string exactly, with no rounded/text-xs/padding', () => {
      const { getByText } = render(ButtonChildrenHarness, {
        props: { text: '×', variant: 'remove', size: 'lg' },
      });
      // size is deliberately passed and must be ignored, same as `link` --
      // this is the whole reason `remove` is its own branch rather than a
      // variantClasses entry: folding it into the shared base would
      // silently add padding to all 21 inline glyph buttons it replaces.
      expect(getByText('×').className.trim()).toBe('text-slate-500 hover:text-red-400 disabled:opacity-50');
    });

    it('forwards aria-label and title through the ...rest spread', () => {
      const { getByRole } = render(ButtonChildrenHarness, {
        props: { text: '×', variant: 'remove', 'aria-label': 'Remove series', title: 'Remove series' },
      });
      const btn = getByRole('button', { name: 'Remove series' });
      expect(btn).toHaveAttribute('title', 'Remove series');
    });

    it('fires onclick and appends a passed-through class', async () => {
      const onclick = vi.fn();
      const { getByText } = render(ButtonChildrenHarness, {
        props: { text: '×', variant: 'remove', class: 'ml-1', onclick },
      });
      const btn = getByText('×');
      expect(btn.className).toContain('ml-1');
      await fireEvent.click(btn);
      expect(onclick).toHaveBeenCalledOnce();
    });

    it('forwards disabled', () => {
      const { getByText } = render(ButtonChildrenHarness, {
        props: { text: '×', variant: 'remove', disabled: true },
      });
      expect((getByText('×') as HTMLButtonElement).disabled).toBe(true);
    });
  });
});
