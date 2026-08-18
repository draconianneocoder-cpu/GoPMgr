// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import { fireEvent, render } from '@testing-library/svelte';

import DocumentFieldEditor from './DocumentFieldEditor.svelte';

// Narrow, targeted coverage added alongside the 2026-08-18 Button
// migration (see TEST_COVERAGE_LEDGER.md): this file previously had zero
// test coverage. Rather than backfill every FieldKind branch, this covers
// the two branches actually touched by the migration (string_array,
// object_array) plus their row add/remove behavior, since both branches'
// remove buttons moved from raw <button> elements to
// `Button variant="remove"` and needed a regression guard before that
// change could be trusted. Other FieldKind branches (string/text/number/
// date/bool/chart_ref) are unchanged by this cycle and remain an
// untested, disclosed gap.

describe('DocumentFieldEditor string_array field', () => {
  const field: DocumentField = { key: 'tags', label: 'Tags', type: 'string_array' };

  it('renders one row per existing value and adds a blank row on "+ Add"', async () => {
    const { getByText, container } = render(DocumentFieldEditor, {
      props: { field, value: ['alpha', 'beta'] },
    });

    const inputs = () => Array.from(container.querySelectorAll('input[type="text"]')) as HTMLInputElement[];
    expect(inputs().map((i) => i.value)).toEqual(['alpha', 'beta']);

    await fireEvent.click(getByText('+ Add'));
    expect(inputs().map((i) => i.value)).toEqual(['alpha', 'beta', '']);
  });

  it('removes the row at the clicked index via the migrated remove button, preserving its exact class string', async () => {
    const { container } = render(DocumentFieldEditor, {
      props: { field, value: ['alpha', 'beta', 'gamma'] },
    });

    const removeButtons = () => Array.from(container.querySelectorAll('button')).filter(
      (b) => b.textContent?.trim() === '×',
    ) as HTMLButtonElement[];
    expect(removeButtons()).toHaveLength(3);
    // Migrated from a raw <button class="text-xs text-slate-500
    // hover:text-red-400 px-2"> to `Button variant="remove"` — pins the
    // exact rendered class string (base + the passed-through `text-xs
    // px-2`) so the appearance is provably unchanged, not just assumed.
    expect(removeButtons()[0].className.trim()).toBe(
      'text-slate-500 hover:text-red-400 disabled:opacity-50 text-xs px-2',
    );

    await fireEvent.click(removeButtons()[1]);

    const inputs = Array.from(container.querySelectorAll('input[type="text"]')) as HTMLInputElement[];
    expect(inputs.map((i) => i.value)).toEqual(['alpha', 'gamma']);
  });
});

describe('DocumentFieldEditor object_array field', () => {
  const field: DocumentField = {
    key: 'contacts',
    label: 'Contacts',
    type: 'object_array',
    object_shape: [{ key: 'name', label: 'Name', type: 'string' }],
  };

  it('adds a blank row on "+ Add row" and removes it via the migrated "Remove row" button, preserving its exact class string', async () => {
    const { getByText, container } = render(DocumentFieldEditor, {
      props: { field, value: [] },
    });

    await fireEvent.click(getByText('+ Add row'));
    expect(container.querySelectorAll('input[type="text"]')).toHaveLength(1);

    const removeRow = getByText('Remove row') as HTMLButtonElement;
    // Migrated from a raw <button class="text-xs text-slate-500
    // hover:text-red-400"> to `Button variant="remove"` — pins the exact
    // rendered class string so the appearance is provably unchanged.
    expect(removeRow.className.trim()).toBe(
      'text-slate-500 hover:text-red-400 disabled:opacity-50 text-xs',
    );

    await fireEvent.click(removeRow);
    expect(container.querySelectorAll('input[type="text"]')).toHaveLength(0);
  });
});
