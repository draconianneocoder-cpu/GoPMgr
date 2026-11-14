// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor, within } from '@testing-library/svelte';
import HelpGuide from './HelpGuide.svelte';
import { HELP_GLOSSARY } from './help-glossary';
import { helpIndex, sidebar } from './help-sections';

vi.mock('../desktop-api', () => ({ desktopApi: {} }));
afterEach(cleanup);

describe('Help Guide reference navigation', () => {
  it('resolves every index and comparison to a unique reference destination', () => {
    const ids = new Set(HELP_GLOSSARY.map((entry) => entry.id));
    const sections = new Set(sidebar.flatMap((group) => group.items.map((item) => item.id)));
    expect(ids.size).toBe(HELP_GLOSSARY.length);
    expect(new Set(helpIndex.map((entry) => `${entry.kind}:${entry.label}`)).size).toBe(helpIndex.length);
    for (const entry of HELP_GLOSSARY) {
      for (const target of entry.see ?? []) expect(ids.has(target), `${entry.term}: ${target}`).toBe(true);
    }
    for (const entry of helpIndex) {
      expect(sections.has(entry.section), entry.label).toBe(true);
      if (entry.kind === 'definition') expect(ids.has(entry.termId!), entry.label).toBe(true);
    }
  });
  it('opens the contents and follows an existing topic', async () => {
    const screen = render(HelpGuide);
    await fireEvent.click(screen.getByRole('button', { name: 'Table of contents' }));
    const main = within(screen.getByRole('main'));
    expect(main.getByRole('heading', { name: 'Table of contents' })).toBeInTheDocument();
    await fireEvent.click(main.getByRole('button', { name: 'Database Encryption' }));
    expect(main.getByRole('heading', { name: 'Database Encryption' })).toBeInTheDocument();
  });

  it('finds a term through filtered search and focuses its definition repeatedly', async () => {
    const screen = render(HelpGuide);
    const search = screen.getByRole('searchbox', { name: 'Search help sections' });
    await fireEvent.input(search, { target: { value: 'DEK' } });
    expect(screen.getByRole('button', { name: 'Glossary' })).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'Subject index' }));
    expect(search).toHaveValue('');
    const main = within(screen.getByRole('main'));
    for (let attempt = 0; attempt < 2; attempt += 1) {
      if (attempt) await fireEvent.click(screen.getByRole('button', { name: 'Subject index' }));
      await fireEvent.click(main.getByRole('button', { name: 'DEK: Definition' }));
      await waitFor(() => expect(document.getElementById('help-term-dek')).toHaveFocus());
      const term = document.getElementById('help-term-dek')!;
      expect(term.parentElement).toHaveTextContent('Data Encryption Key');
      expect(within(term.parentElement!).getByRole('button', { name: 'Passphrase: Definition' })).toBeInTheDocument();
    }
  });

  it('distinguishes archival formatting, signing and encryption', async () => {
    const screen = render(HelpGuide);
    await fireEvent.click(screen.getByRole('button', { name: 'Subject index' }));
    const main = within(screen.getByRole('main'));
    await fireEvent.click(main.getByRole('button', { name: 'PDF/A-2u: Definition' }));
    await waitFor(() => expect(document.getElementById('help-term-pdf-a-2u')).toHaveFocus());
    const definition = document.getElementById('help-term-pdf-a-2u')!.parentElement!;
    expect(definition).toHaveTextContent('does not prove accessibility');
    await fireEvent.click(within(definition).getByRole('button', { name: 'PAdES: Definition' }));
    await waitFor(() => expect(document.getElementById('help-term-pades')).toHaveFocus());
    expect(document.getElementById('help-term-pades')!.parentElement).toHaveTextContent('not encryption');
  });
});
