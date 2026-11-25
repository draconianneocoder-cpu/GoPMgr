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

// Removing one comment can join the halves of another ("<!<!---->--"), so
// the removals repeat until nothing changes, as in hardcoded-text.test.ts.
function withoutComments(text: string): string {
  let previous: string;
  do {
    previous = text;
    text = text
      .replace(/\/\*[\s\S]*?\*\//g, '')
      .replace(/<!--[\s\S]*?--!?>/g, '')
      .replace(/^\s*\/\/.*$/gm, '');
  } while (text !== previous);
  return text;
}

const appCss = withoutComments(readFileSync(join(SRC, 'app.css'), 'utf-8'));
const defined = new Set(Array.from(appCss.matchAll(/(--[\w-]+)\s*:/g), (m) => m[1]));

describe('theme custom properties', () => {
  it('references only properties that app.css defines', () => {
    const files = [...sourceFiles(SRC), join(FRONTEND, 'tailwind.config.js'), join(FRONTEND, 'index.html')];
    const undefinedRefs = files.flatMap((file) =>
      Array.from(withoutComments(readFileSync(file, 'utf-8')).matchAll(/var\(\s*(--[\w-]+)/g), (m) => m[1])
        .filter((name) => !defined.has(name))
        .map((name) => `${file.slice(FRONTEND.length + 1)}: ${name}`),
    );

    expect(undefinedRefs).toEqual([]);
  });
});
