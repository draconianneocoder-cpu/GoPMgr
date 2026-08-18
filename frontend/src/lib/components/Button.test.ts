// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/svelte';

import Button from './Button.svelte';
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

  it('sizes: sm is px-3 py-1, md is px-3 py-2, lg is px-4 py-2', () => {
    const { getByText, rerender } = render(ButtonChildrenHarness, { props: { text: 'X', size: 'sm' } });
    expect(getByText('X').className).toContain('px-3 py-1');
    rerender({ text: 'X', size: 'lg' });
    expect(getByText('X').className).toContain('px-4 py-2');
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
});
