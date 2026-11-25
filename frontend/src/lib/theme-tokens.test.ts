// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';

// Theme colours reach components only through the custom properties that
// app.css defines (and redefines for the light theme). A reference to a
// property nobody defines is not an error in CSS: the fallback in
// var(--name, fallback) is used silently, in every theme. Four Six Sigma
// editors drew light-grey panels and white cards that way under the dark
// theme's light text, leaving their fields unreadable (about 1.1:1).
// This is a static policy check: every var(--name) in the frontend must
// name a property app.css defines.

const SRC = join(__dirname, '..');
const FRONTEND = join(SRC, '..');

function sourceFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) return sourceFiles(path);
    return /\.(svelte|css)$/.test(entry.name) ? [path] : [];
  });
}

// matchesOutsideComments returns the matches of pattern that do not start
// inside a CSS (/* */), HTML (<!-- -->) or line (//) comment; an
// unterminated comment runs to the end of the text. Comments are
// located in the original text rather than removed, so no removal can join
// the halves of another comment.
function matchesOutsideComments(text: string, pattern: RegExp): RegExpMatchArray[] {
  const comments = Array.from(
    text.matchAll(/\/\*[\s\S]*?(?:\*\/|(?![\s\S]))|<!--[\s\S]*?(?:--!?>|(?![\s\S]))|^\s*\/\/.*$/gm),
    (m) => [m.index ?? 0, (m.index ?? 0) + m[0].length] as const,
  );
  return Array.from(text.matchAll(pattern)).filter(
    (m) => !comments.some(([start, end]) => (m.index ?? 0) >= start && (m.index ?? 0) < end),
  );
}

const defined = new Set(
  matchesOutsideComments(readFileSync(join(SRC, 'app.css'), 'utf-8'), /(--[\w-]+)\s*:/g).map((m) => m[1]),
);

describe('theme custom properties', () => {
  it('references only properties that app.css defines', () => {
    const files = [...sourceFiles(SRC), join(FRONTEND, 'tailwind.config.js'), join(FRONTEND, 'index.html')];
    const undefinedRefs = files.flatMap((file) =>
      matchesOutsideComments(readFileSync(file, 'utf-8'), /var\(\s*(--[\w-]+)/g)
        .map((m) => m[1])
        .filter((name) => !defined.has(name))
        .map((name) => `${file.slice(FRONTEND.length + 1)}: ${name}`),
    );

    expect(undefinedRefs).toEqual([]);
  });
});
