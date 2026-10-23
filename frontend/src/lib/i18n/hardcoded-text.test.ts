// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { readdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join, relative } from 'node:path';
import { describe, expect, it } from 'vitest';

// A ratchet on interface text written straight into components. New text
// goes in the catalog (messages/en.ts) and reaches the screen through t(), so
// each file's count of literal text may only fall (ADR-008). After moving
// text into the catalog, record the new counts with:
//
//   UPDATE_HARDCODED_TEXT_BASELINE=1 npx vitest run src/lib/i18n/hardcoded-text.test.ts

const SRC = join(__dirname, '..', '..');
const BASELINE = join(__dirname, 'hardcoded-text-baseline.json');

function svelteFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) return svelteFiles(path);
    return entry.name.endsWith('.svelte') ? [path] : [];
  });
}

// literalText returns the text a person would read that is written into the
// markup: text between tags, and the placeholder, aria-label, title, label,
// and alt attributes. Anything inside {braces}, such as {t('key')}, is code.
export function literalText(source: string): string[] {
  let markup = source
    .replace(/<script[\s\S]*?<\/script>/g, '')
    .replace(/<style[\s\S]*?<\/style>/g, '')
    .replace(/<!--[\s\S]*?-->/g, '');
  for (let previous = ''; previous !== markup; ) {
    previous = markup;
    markup = markup.replace(/\{[^{}]*\}/g, '');
  }
  const hasWords = (text: string) => /\p{L}{2}/u.test(text);
  const between = Array.from(markup.matchAll(/>([^<>]+)</g), (m) => m[1].trim()).filter(hasWords);
  const attributes = Array.from(
    markup.matchAll(/\b(?:placeholder|aria-label|title|label|alt)="([^"]*)"/g),
    (m) => m[1].trim(),
  ).filter(hasWords);
  return [...between, ...attributes];
}

function counts(): Record<string, number> {
  const out: Record<string, number> = {};
  for (const file of svelteFiles(SRC).sort()) {
    const n = literalText(readFileSync(file, 'utf8')).length;
    if (n > 0) out[relative(SRC, file).split('\\').join('/')] = n;
  }
  return out;
}

describe('literalText', () => {
  it('counts text and labelled attributes, not code or comments', () => {
    const source = `<script>const x = '<b>Not markup</b>';</script>
<!-- SPDX note -->
<p>Plain words</p>
<p>{t('key')}</p>
<p>Hello {name}!</p>
<input placeholder="Search projects" aria-label={t('search')} />
<span>✓</span>`;
    expect(literalText(source)).toEqual(['Plain words', 'Hello !', 'Search projects']);
  });
});

describe('hard-coded interface text', () => {
  it('does not grow in any component, and the baseline records every fall', () => {
    const now = counts();
    if (process.env.UPDATE_HARDCODED_TEXT_BASELINE) {
      writeFileSync(BASELINE, JSON.stringify(now, null, 2) + '\n');
      return;
    }
    const baseline: Record<string, number> = JSON.parse(readFileSync(BASELINE, 'utf8'));
    const grew: string[] = [];
    const fell: string[] = [];
    for (const file of new Set([...Object.keys(now), ...Object.keys(baseline)])) {
      const was = baseline[file] ?? 0;
      const is = now[file] ?? 0;
      if (is > was) grew.push(`${file}: ${was} → ${is}; move the new text into messages/en.ts and use t()`);
      if (is < was) fell.push(`${file}: ${was} → ${is}`);
    }
    expect(grew, 'literal interface text was added').toEqual([]);
    expect(fell, 'text moved to the catalog; update the baseline (see this file’s header)').toEqual([]);
  });
});
