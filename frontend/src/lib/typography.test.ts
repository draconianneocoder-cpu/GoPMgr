// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { existsSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

import { FONT_MONO, FONT_SANS } from './typography';

// Read from disk, as persistence-boundary-strings.test.ts explains, so these
// files stay out of Vite's module graph.
const path = (relative: string) => fileURLToPath(new URL(relative, import.meta.url));
const appCss = readFileSync(path('../app.css'), 'utf-8');
const indexHtml = readFileSync(path('../../index.html'), 'utf-8');

interface Face {
  family: string;
  style: string;
  weight: string;
  url: string;
}

function fontFaces(css: string): Face[] {
  return [...css.matchAll(/@font-face\s*\{([^}]*)\}/g)].map(([, body]) => {
    const prop = (name: string) => body.match(new RegExp(`${name}:\\s*([^;]+);`))?.[1].trim() ?? '';
    return {
      family: prop('font-family').replace(/['"]/g, ''),
      style: prop('font-style'),
      weight: prop('font-weight'),
      url: body.match(/url\('([^']+)'\)/)?.[1] ?? '',
    };
  });
}

function themeToken(name: string): string {
  return appCss.match(new RegExp(`--${name}:\\s*([^;]+);`))?.[1].trim() ?? '';
}

// The weights each family serves, per docs/design/typography-ibm-plex.md.
const EXPECTED = [
  'IBM Plex Sans normal 400',
  'IBM Plex Sans italic 400',
  'IBM Plex Sans normal 500',
  'IBM Plex Sans normal 600',
  'IBM Plex Sans normal 700',
  'IBM Plex Mono normal 400',
  'IBM Plex Mono normal 500',
  'IBM Plex Mono normal 600',
  'IBM Plex Mono normal 700',
];

describe('IBM Plex typography', () => {
  const faces = fontFaces(appCss);

  it('declares a designed face for every weight and style the app uses', () => {
    expect(faces.map((f) => `${f.family} ${f.style} ${f.weight}`).sort()).toEqual([...EXPECTED].sort());
  });

  it('loads every face from a bundled WOFF2 file named for its weight', () => {
    const fileWeight: Record<string, string> = { Regular: '400', Italic: '400', Medium: '500', SemiBold: '600', Bold: '700' };
    for (const face of faces) {
      const match = face.url.match(/^\/fonts\/ibm-plex\/(IBMPlex(?:Sans|Mono))-(\w+)\.woff2$/);
      expect(match, face.url).not.toBeNull();
      const [, file, style] = match!;
      expect(file).toBe(face.family.replace(/ /g, ''));
      expect(fileWeight[style]).toBe(face.weight);
      expect(face.style).toBe(style === 'Italic' ? 'italic' : 'normal');
      expect(existsSync(path(`../../public${face.url}`)), face.url).toBe(true);
    }
  });

  it('keeps the Tailwind font tokens and the canvas font stacks identical', () => {
    expect(themeToken('font-sans')).toBe(FONT_SANS);
    expect(themeToken('font-mono')).toBe(FONT_MONO);
    expect(FONT_SANS.startsWith('"IBM Plex Sans",')).toBe(true);
    expect(FONT_MONO.startsWith('"IBM Plex Mono",')).toBe(true);
  });

  it('preloads only declared Sans faces, as fonts, with CORS so the preload is reused', () => {
    const preloads = [...indexHtml.matchAll(/<link rel="preload" href="([^"]+)" as="font" type="font\/woff2" crossorigin \/>/g)].map(
      (m) => m[1],
    );
    expect(preloads.sort()).toEqual(
      faces
        .filter((f) => f.family === 'IBM Plex Sans' && f.style === 'normal')
        .map((f) => f.url)
        .sort(),
    );
  });
});
