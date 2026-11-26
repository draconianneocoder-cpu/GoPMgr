// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { readdirSync, readFileSync } from 'node:fs';
import { join, relative } from 'node:path';
import { describe, expect, it } from 'vitest';

// Every form control needs an accessible name (WCAG 4.1.2). A grid cell's
// visible label is its column header, which does not name the input inside
// it, so a screen reader announced the chart editors' data grids as a run of
// unnamed "edit text" fields. This is a static policy check: each <input>,
// <select> and <textarea> in a component must carry a name source itself,
// or sit inside a <label> in the same file. axe is not a dependency, and
// jsdom computes no accessible names, so the markup is the strongest
// boundary available without adding one.

const SRC = join(__dirname, '..');

// Components whose control is named by its caller's <label>, or which pass
// the caller's attributes through ({...rest}).
const NAMED_BY_CALLER = new Set([
  'lib/components/Input.svelte',
  'lib/components/Textarea.svelte',
  // Rendered only inside DocumentFieldEditor's <label>.
  'lib/components/documents/ChartPicker.svelte',
  // Its wrapper is <svelte:element this={grouped ? 'fieldset' : 'label'}>,
  // which this check cannot evaluate. Single controls sit in the label;
  // controls in a grouped (fieldset) row name themselves.
  'lib/components/documents/DocumentFieldEditor.svelte',
]);

function svelteFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) return entry.name === '__test_harness__' ? [] : svelteFiles(path);
    return entry.name.endsWith('.svelte') ? [path] : [];
  });
}

// controls yields each control's start offset and attribute text. Attribute
// values may contain `>` inside {expressions} (arrow functions), so the tag
// ends at the first `>` outside braces and quotes.
export function* controls(source: string): Generator<{ start: number; attrs: string }> {
  for (const m of source.matchAll(/<(input|select|textarea)\b/g)) {
    let i = (m.index ?? 0) + m[0].length;
    let depth = 0;
    let quote = '';
    for (; i < source.length; i++) {
      const c = source[i];
      if (quote) {
        if (c === quote) quote = '';
      } else if (c === '{') depth++;
      else if (c === '}') depth--;
      else if (depth === 0 && (c === '"' || c === "'")) quote = c;
      else if (depth === 0 && c === '>') break;
    }
    yield { start: m.index ?? 0, attrs: source.slice((m.index ?? 0) + m[0].length, i) };
  }
}

// unnamed returns the line numbers of controls with no name source. Hidden,
// file and button-like inputs are exempt; placeholder and title count, as
// browsers fall back to them, though a visible label is better.
export function unnamed(source: string): number[] {
  // Blank out comments and scripts, keeping offsets and line numbers.
  const blank = (c: string) => c.replace(/[^\n]/g, ' ');
  const text = source.replace(/<!--[\s\S]*?-->/g, blank).replace(/<script\b[\s\S]*?<\/script>/g, blank);
  const lines: number[] = [];
  for (const { start, attrs } of controls(text)) {
    if (/type="(hidden|file|submit|button)"/.test(attrs)) continue;
    if (/\baria-label(ledby)?=|\bid=|\bplaceholder=|\btitle=|\{\.\.\./.test(attrs)) continue;
    const before = text.slice(0, start);
    if (before.lastIndexOf('<label') > before.lastIndexOf('</label>')) continue;
    lines.push(before.split('\n').length);
  }
  return lines;
}

describe('form control names', () => {
  it('flags only controls with no name source', () => {
    const source = `<label>Name <input bind:value={a} /></label>
<input aria-label={t('x')} oninput={(e) => go(e)} />
<!-- <input /> in a comment -->
<script>// a <select> in a script comment</script>
<input type="checkbox" onchange={(e) => (a = e.currentTarget.checked)} />
<select value={v}>`;
    expect(unnamed(source)).toEqual([5, 6]);
  });

  it('every input, select and textarea has an accessible name', () => {
    const missing = svelteFiles(SRC)
      .map((file) => relative(SRC, file).split('\\').join('/'))
      .filter((file) => !NAMED_BY_CALLER.has(file))
      .flatMap((file) => unnamed(readFileSync(join(SRC, file), 'utf-8')).map((line) => `${file}:${line}`));

    expect(missing).toEqual([]);
  });
});
