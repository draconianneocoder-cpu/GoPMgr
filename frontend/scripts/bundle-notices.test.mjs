// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Unit tests for bundle-notices-lib.mjs. Run with:
//   node --test scripts/bundle-notices.test.mjs

import assert from 'node:assert/strict';
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { after, describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import {
  MIT_TERMS,
  NO_LICENSE_FILE,
  collectPackages,
  firstDifference,
  isNoticeName,
  moduleFile,
  normalizeText,
  packageRoot,
  parseBanners,
  renderNotices,
} from './bundle-notices-lib.mjs';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
const work = mkdtempSync(path.join(tmpdir(), 'gopmgr-bundle-notices-'));
after(() => rmSync(work, { recursive: true, force: true }));

const frontendDir = path.join(work, 'frontend');
const nodeModules = path.join(frontendDir, 'node_modules');

function put(rel, text) {
  const file = path.join(frontendDir, rel);
  mkdirSync(path.dirname(file), { recursive: true });
  writeFileSync(file, text);
  return file;
}

function pkg(dir, manifest, files = {}) {
  put(path.join('node_modules', dir, 'package.json'), JSON.stringify(manifest));
  for (const [name, text] of Object.entries(files)) {
    put(path.join('node_modules', dir, name), text);
  }
}

pkg('alpha', { name: 'alpha', version: '1.0.0', license: 'MIT' }, { LICENSE: 'Alpha  \r\nterms\r\n\r\n', 'license.js': 'code' });
pkg('alpha/sub', { name: 'alpha/sub', version: '9.9.9' });
put('node_modules/alpha/sub/index.js', 'export {}');
put('node_modules/alpha/index.js', 'export {}');
pkg('@scope/beta', { name: '@scope/beta', version: '2.0.0', license: 'ISC' }, { 'LICENSE.md': 'Beta', NOTICE: 'Beta notice' });
put('node_modules/@scope/beta/dist/beta.js', 'export {}');
pkg('alpha/node_modules/gamma', { name: 'gamma', version: '0.1.0', license: 'BSD-3-Clause' }, { COPYING: 'Gamma' });
put('node_modules/alpha/node_modules/gamma/g.js', 'export {}');
pkg('vite', { name: 'vite', version: '8.3.4', license: 'MIT' }, { 'LICENSE.md': '# Vite core license\nMIT\n\n# Licenses of bundled dependencies\nmany more\n' });
pkg('tailwindcss', { name: 'tailwindcss', version: '4.3.3', license: 'MIT' }, { LICENSE: 'Tailwind' });
pkg('bare', { name: 'bare', version: '1.0.0', license: 'MIT', author: 'Someone' });
put('node_modules/bare/i.js', 'export {}');
const exempt = NO_LICENSE_FILE['worker-f'];
pkg('worker-f', { name: 'worker-f', version: exempt.version, license: 'MIT', author: 'catamphetamine <x@example.invalid>' });
put('node_modules/worker-f/i.js', 'export {}');
const src = put('src/main.ts', 'export {}');

const collect = (moduleIds, assetTexts = []) => collectPackages({ moduleIds, assetTexts, frontendDir, nodeModules });

describe('isNoticeName', () => {
  const cases = [
    ['LICENSE', true], ['license', true], ['LICENSE.md', true], ['LICENCE.txt', true], ['LICENSE-MIT', true],
    ['NOTICE', true], ['COPYING', true], ['UNLICENSE', true],
    ['license.js', false], ['LICENSE.json', false], ['licensed.txt', false], ['README.md', false], ['AUTHORS', false],
  ];
  for (const [name, want] of cases) {
    it(`${name} -> ${want}`, () => assert.equal(isNoticeName(name), want));
  }
});

describe('normalizeText', () => {
  it('normalizes only whitespace', () => {
    assert.equal(normalizeText(Buffer.from('\uFEFF\n a  \r\nb\rc \n\n\n'), 'x'), ' a\nb\nc\n');
  });
  it('rejects invalid UTF-8', () => {
    assert.throws(() => normalizeText(Buffer.from([0x63, 0x61, 0x66, 0xe9]), 'x'), /UTF-8/);
  });
  it('rejects whitespace only', () => {
    assert.throws(() => normalizeText(Buffer.from(' \r\n\t'), 'x'), /empty/);
  });
});

describe('packageRoot', () => {
  it('skips stub package.json files inside a package', () => {
    assert.equal(packageRoot(path.join(nodeModules, 'alpha/sub/index.js'), nodeModules).name, 'alpha');
  });
  it('handles scoped and nested packages', () => {
    assert.equal(packageRoot(path.join(nodeModules, '@scope/beta/dist/beta.js'), nodeModules).name, '@scope/beta');
    assert.equal(packageRoot(path.join(nodeModules, 'alpha/node_modules/gamma/g.js'), nodeModules).name, 'gamma');
  });
  it('rejects a path outside node_modules', () => {
    assert.throws(() => packageRoot(src, nodeModules), /not inside/);
  });
});

describe('moduleFile', () => {
  // Vite writes module IDs with forward slashes on every platform. path.win32
  // stands in for a Windows host so the CI on Linux covers that form too.
  const cases = [
    ['Windows ID with query', path.win32, 'C:/work/GoPMgr/frontend/src/main.ts?v=1', 'C:\\work\\GoPMgr\\frontend\\src\\main.ts'],
    ['Windows node_modules ID', path.win32, 'C:/work/GoPMgr/frontend/node_modules/@scope/beta/dist/beta.js', 'C:\\work\\GoPMgr\\frontend\\node_modules\\@scope\\beta\\dist\\beta.js'],
    ['POSIX ID with query', path.posix, '/work/GoPMgr/frontend/src/main.ts?inline', '/work/GoPMgr/frontend/src/main.ts'],
    ['POSIX ID with redundant segments', path.posix, '/work/GoPMgr/frontend//src/./main.ts', '/work/GoPMgr/frontend/src/main.ts'],
  ];
  for (const [name, api, id, want] of cases) {
    it(name, () => assert.equal(moduleFile(id, api), want));
  }
  it('makes a Windows ID match the native frontend prefix', () => {
    const dir = 'C:\\work\\GoPMgr\\frontend';
    assert.ok(moduleFile('C:/work/GoPMgr/frontend/src/main.ts', path.win32).startsWith(dir + path.win32.sep));
  });
});

describe('parseBanners', () => {
  it('reads tool license banners', () => {
    assert.deepEqual(parseBanners('a/*! tailwindcss v4.3.3 | MIT License | https://tailwindcss.com */b/*! @x/y v1.2.3-rc.1 | ISC */'), [
      { name: 'tailwindcss', version: '4.3.3' },
      { name: '@x/y', version: '1.2.3-rc.1' },
    ]);
    assert.deepEqual(parseBanners('/* plain comment */ /*! no version here */'), []);
  });
});

describe('collectPackages', () => {
  it('maps modules, virtual helpers and banners to packages, sorted', () => {
    const got = collect(
      [
        path.join(nodeModules, 'alpha/index.js'),
        path.join(nodeModules, 'alpha/sub/index.js') + '?commonjs-proxy',
        path.join(nodeModules, '@scope/beta/dist/beta.js'),
        path.join(nodeModules, 'alpha/node_modules/gamma/g.js'),
        '\0vite/preload-helper.js',
        src,
      ],
      ['.x{}/*! tailwindcss v4.3.3 | MIT License | https://tailwindcss.com */'],
    );
    assert.deepEqual(got.map((p) => `${p.name} ${p.version}`), ['@scope/beta 2.0.0', 'alpha 1.0.0', 'gamma 0.1.0', 'tailwindcss 4.3.3', 'vite 8.3.4']);
    const alpha = got.find((p) => p.name === 'alpha');
    assert.deepEqual(alpha.files, [{ name: 'LICENSE', text: 'Alpha\nterms\n' }]);
    assert.deepEqual(got.find((p) => p.name === '@scope/beta').files.map((f) => f.name), ['LICENSE.md', 'NOTICE']);
    const vite = got.find((p) => p.name === 'vite');
    assert.equal(vite.files[0].text, '# Vite core license\nMIT\n');
    assert.match(vite.note, /Only the Vite core license applies/);
    assert.deepEqual(vite.roles, ['build helpers that Vite injects into the bundle']);
  });
  it('rejects an unknown virtual module', () => {
    assert.throws(() => collect(['\0rolldown/runtime.js']), /unknown virtual module/);
  });
  it('rejects a module outside the frontend', () => {
    assert.throws(() => collect([path.join(work, 'elsewhere.js')]), /outside the frontend/);
  });
  it('rejects a banner whose version differs from node_modules', () => {
    assert.throws(() => collect([], ['/*! tailwindcss v4.0.0 | MIT */']), /node_modules has tailwindcss 4\.3\.3/);
  });
  it('rejects an unreviewed package without a license file', () => {
    assert.throws(() => collect([path.join(nodeModules, 'bare/i.js')]), /bare 1\.0\.0 ships no license file/);
  });
  it('accepts a reviewed package without a license file, at its reviewed version', () => {
    const [w] = collect([path.join(nodeModules, 'worker-f/i.js')]);
    assert.match(w.note, /has a license file/);
    assert.equal(w.files[0].text, `Copyright (c) catamphetamine <x@example.invalid>\n\n${MIT_TERMS}`);
  });
});

describe('MIT_TERMS', () => {
  it("matches the repository's MIT text after its copyright line", () => {
    const repoMIT = readFileSync(path.join(repoRoot, 'LICENSES', 'MIT.txt'), 'utf8').split('\n').slice(4).join('\n');
    assert.equal(MIT_TERMS, repoMIT);
  });
});

describe('renderNotices', () => {
  it('is deterministic and has no trailing whitespace', () => {
    const packages = collect([path.join(nodeModules, 'alpha/index.js'), path.join(nodeModules, '@scope/beta/dist/beta.js')]);
    const text = renderNotices(packages);
    assert.equal(text, renderNotices(packages));
    assert.match(text, /\n {2}@scope\/beta 2\.0\.0\n {2}alpha 1\.0\.0\n/);
    assert.match(text, /\nPackage: alpha 1\.0\.0\nLicense \(package\.json\): MIT\nIncluded as: JavaScript modules in the bundle\n\n----- LICENSE -----\n\nAlpha\nterms\n/);
    for (const line of text.split('\n')) {
      assert.equal(line, line.trimEnd());
    }
    assert.ok(text.endsWith('\n') && !text.endsWith('\n\n'));
  });
});

describe('firstDifference', () => {
  it('names the first differing line', () => {
    assert.equal(firstDifference('a\nb\n', 'a\nc\n'), 'line 2: have "b", want "c"');
    assert.equal(firstDifference('a\n', 'a\nb\n'), 'line 2: have "", want "b"');
  });
});
