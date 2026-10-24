// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';

import { checkCatalog, render, renderParts, type Catalog, type Message } from './catalog';
import { registeredCatalogs } from './i18n.svelte';
import { en } from './messages/en';

const source: Catalog = {
  title: 'Create your account',
  failed: 'Codes could not be generated ({error})',
  files: { one: '{count} file', other: '{count} files' },
};

describe('render', () => {
  it('chooses the plural form for the language and formats the count', () => {
    expect(render(source.files, { count: 1 }, 'en', 'en')).toBe('1 file');
    expect(render(source.files, { count: 1284 }, 'en', 'de-DE')).toBe('1.284 files');
    expect(render({ other: '{count} 件' }, { count: 1 }, 'ja', 'ja')).toBe('1 件');
  });
});

describe('renderParts', () => {
  it('marks each filled placeholder, so a component can style it in place', () => {
    expect(renderParts('{actor} opened your data on {time}.', { actor: 'alice', time: '23/10/2026' }, 'en', 'en')).toEqual([
      { text: 'alice', param: 'actor' },
      { text: ' opened your data on ' },
      { text: '23/10/2026', param: 'time' },
      { text: '.' },
    ]);
  });

  it('follows the order a translation puts the placeholders in', () => {
    const ja = '{time}に{actor}があなたのデータを開きました。';
    expect(renderParts(ja, { actor: 'alice', time: '2026/10/23' }, 'ja', 'ja').map((p) => p.param ?? '')).toEqual([
      'time',
      '',
      'actor',
      '',
    ]);
  });

  it('keeps a placeholder with no value as literal text and formats numbers', () => {
    expect(renderParts('{unused} of {count} left ({error})', { unused: 1284, count: 2 }, 'en', 'de-DE')).toEqual([
      { text: '1.284', param: 'unused' },
      { text: ' of ' },
      { text: '2', param: 'count' },
      { text: ' left ({error})' },
    ]);
  });

  it('joins to exactly what render returns, plural forms included', () => {
    const cases: [Message, Record<string, string | number>][] = [
      [source.files, { count: 1 }],
      [source.files, { count: 7 }],
      ['{a}{b}', { a: 'x', b: 'y' }],
      ['no placeholders', {}],
      ['{missing} stays', {}],
    ];
    for (const [message, params] of cases) {
      const joined = renderParts(message, params, 'en', 'en').map((p) => p.text).join('');
      expect(joined).toBe(render(message, params, 'en', 'en'));
    }
  });
});

describe('checkCatalog', () => {
  it('accepts a complete translation', () => {
    const fr: Catalog = {
      title: 'Créez votre compte',
      failed: 'Impossible de générer les codes ({error})',
      files: { one: '{count} fichier', many: '{count} de fichiers', other: '{count} fichiers' },
    };
    expect(checkCatalog(source, fr, 'fr')).toEqual([]);
  });

  it('needs only "other" for Japanese', () => {
    const ja: Catalog = { title: 'アカウントを作成', failed: 'コードを生成できませんでした（{error}）', files: { other: '{count} 件のファイル' } };
    expect(checkCatalog(source, ja, 'ja')).toEqual([]);
  });

  it('reports missing, extra, lost, unknown, empty, and wrongly shaped messages', () => {
    const broken: Catalog = {
      failed: 'Impossible de générer les codes ({erreur})',
      files: { one: '{count} fichier', other: '' },
      extra: 'Hors catalogue',
    };
    expect(checkCatalog(source, broken, 'fr')).toEqual([
      { key: 'title', problem: 'missing' },
      { key: 'failed', problem: 'unknown placeholder {erreur} in text' },
      { key: 'failed', problem: 'lost placeholder {error} in text' },
      { key: 'files', problem: 'missing plural form "many"' },
      { key: 'files', problem: 'missing plural form "other"' },
      { key: 'files', problem: 'empty other' },
      { key: 'files', problem: 'lost placeholder {count} in other' },
      { key: 'extra', problem: 'not in the English catalog' },
    ]);
    expect(checkCatalog(source, { ...source, title: { other: 'x' } }, 'en')).toEqual([
      { key: 'title', problem: 'should be plain text' },
    ]);
  });

  it('passes for every registered catalog', () => {
    for (const [code, catalog] of Object.entries(registeredCatalogs)) {
      expect(checkCatalog(en, catalog ?? {}, code), code).toEqual([]);
    }
  });
});
