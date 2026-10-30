// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Pure helpers for bundle-notices.mjs: find the npm packages whose code is in
// the production bundle, read their license and notice files, and render the
// notices file. Kept apart from the Vite build so node:test can cover them.

import { readdirSync, readFileSync, statSync } from 'node:fs';
import path from 'node:path';

// The standard MIT License terms, as in LICENSES/MIT.txt after its copyright
// line. Used only for a package listed in NO_LICENSE_FILE.
export const MIT_TERMS = `Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
`;

// Reviewed packages that publish no license file. Each entry is pinned to the
// version reviewed, so an update is reviewed again (it may add a file).
export const NO_LICENSE_FILE = {
  'worker-f': {
    version: '0.1.16',
    license: 'MIT',
    checked:
      'Neither the npm package nor its source at https://gitlab.com/catamphetamine/worker-f ' +
      '(tag v0.1.16, commit cc1f5f33ac93d418dbb108ec890913ecf573a95f) has a license file.',
  },
};

// Reviewed cuts of a license file that also covers code not in this bundle.
// Vite's LICENSE.md adds the licenses of the dependencies bundled into Vite's
// own Node.js build; only Vite's core license covers the helpers it injects.
// A missing marker fails, so a changed layout is reviewed again.
export const LICENSE_EXCERPTS = {
  vite: {
    file: 'LICENSE.md',
    until: '# Licenses of bundled dependencies',
    note: "Only the Vite core license applies: the rest of Vite's LICENSE.md covers dependencies bundled into Vite's own Node.js build, not into this bundle.",
  },
};

// Virtual modules a bundler injects, by the package whose code they are.
const VIRTUAL_MODULES = [{ pattern: /^\0vite\//, pkg: 'vite', role: 'build helpers that Vite injects into the bundle' }];

const NOTICE_STEMS = ['copying', 'copyleft', 'copyright', 'legal', 'licence', 'license', 'notice', 'patents', 'third_party_notices', 'third-party-notices', 'unlicense'];
const NOTICE_EXTENSIONS = new Set(['', '.txt', '.md', '.markdown', '.rst']);

// A license banner that a tool writes into generated output, such as
// "/*! tailwindcss v4.3.3 | MIT License | https://tailwindcss.com */".
const BANNER = /\/\*!\s*((?:@[a-z0-9._-]+\/)?[a-z0-9._-]+) v(\d[^\s|*]*)\s*\|[^*]*\*\//gi;

// isNoticeName matches tools/module-notices: a license-like stem (exact, or
// followed by "-" or "_") and no extension or a text one, so license.js is out.
export function isNoticeName(name) {
  const lower = name.toLowerCase();
  const ext = path.extname(lower);
  if (!NOTICE_EXTENSIONS.has(ext)) {
    return false;
  }
  const stem = lower.slice(0, lower.length - ext.length);
  return NOTICE_STEMS.some((p) => stem === p || stem.startsWith(p + '-') || stem.startsWith(p + '_'));
}

// normalizeText matches tools/module-notices: LF line endings, no trailing
// spaces, no byte-order mark, one final newline.
export function normalizeText(bytes, label) {
  let text;
  try {
    text = new TextDecoder('utf-8', { fatal: true }).decode(bytes);
  } catch {
    throw new Error(`${label}: not valid UTF-8 text`);
  }
  const lines = text
    .replace(/^\uFEFF/, '')
    .replace(/\r\n?/g, '\n')
    .split('\n')
    .map((line) => line.replace(/[ \t\f\v]+$/, ''));
  const joined = lines.join('\n').replace(/^\n+|\n+$/g, '');
  if (joined === '') {
    throw new Error(`${label}: empty license text`);
  }
  return joined + '\n';
}

function readJSON(file) {
  return JSON.parse(readFileSync(file, 'utf8'));
}

// packageRoot finds the package a bundled file belongs to: the directory
// right after the last node_modules in its path (two segments for a scoped
// name). Stub package.json files deeper inside a package, such as
// read-excel-file/browser/package.json, do not count.
export function packageRoot(file, nodeModules) {
  const rel = path.relative(nodeModules, file);
  if (rel.startsWith('..') || path.isAbsolute(rel)) {
    throw new Error(`${file} is not inside ${nodeModules}`);
  }
  const parts = rel.split(path.sep);
  let at = parts.lastIndexOf('node_modules') + 1;
  const width = parts[at]?.startsWith('@') ? 2 : 1;
  if (parts.length < at + width + 1) {
    throw new Error(`${file} is not inside a package directory`);
  }
  const dir = path.join(nodeModules, ...parts.slice(0, at + width));
  const manifest = path.join(dir, 'package.json');
  let pkg;
  try {
    pkg = readJSON(manifest);
  } catch (err) {
    throw new Error(`${manifest}: ${err.message}`);
  }
  if (!pkg.name || !pkg.version) {
    throw new Error(`${manifest} has no name or version`);
  }
  return { dir, name: pkg.name, version: pkg.version };
}

export function parseBanners(text) {
  return [...text.matchAll(BANNER)].map((m) => ({ name: m[1], version: m[2] }));
}

// moduleFile turns a Vite module ID into a native file path. Vite writes IDs
// with forward slashes on every platform (C:/work/... on Windows), so the
// containment checks below would never match backslash prefixes without
// this. The query string (?v=..., ?inline) is dropped.
export function moduleFile(id, pathApi = path) {
  return pathApi.normalize(id.split('?')[0]);
}

// collectPackages maps the bundle's module IDs and generated asset texts to
// the npm packages they come from. Anything it cannot place is an error, so
// unknown code never ships without a review.
export function collectPackages({ moduleIds, assetTexts, frontendDir, nodeModules }) {
  const found = new Map(); // dir -> package
  const add = (root, role) => {
    const prev = found.get(root.dir);
    if (prev) {
      prev.roles.add(role);
      return;
    }
    found.set(root.dir, { ...root, roles: new Set([role]) });
  };
  for (const raw of moduleIds) {
    if (raw.startsWith('\0')) {
      const v = VIRTUAL_MODULES.find((m) => m.pattern.test(raw));
      if (!v) {
        throw new Error(`unknown virtual module ${JSON.stringify(raw)} in the bundle; add it to VIRTUAL_MODULES after review`);
      }
      add(packageRoot(path.join(nodeModules, v.pkg, 'package.json'), nodeModules), v.role);
      continue;
    }
    const file = moduleFile(raw);
    if (file.startsWith(nodeModules + path.sep)) {
      add(packageRoot(file, nodeModules), 'JavaScript modules in the bundle');
      continue;
    }
    if (!file.startsWith(frontendDir + path.sep)) {
      throw new Error(`bundle module ${file} is outside the frontend and its node_modules`);
    }
  }
  for (const text of assetTexts) {
    for (const banner of parseBanners(text)) {
      const root = packageRoot(path.join(nodeModules, banner.name, 'package.json'), nodeModules);
      if (root.name !== banner.name || root.version !== banner.version) {
        throw new Error(`generated output names ${banner.name} ${banner.version}, but node_modules has ${root.name} ${root.version}`);
      }
      add(root, 'generated CSS in the bundle');
    }
  }
  return [...found.values()].map(withNotices).sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0));
}

function noticeFiles(dir) {
  return readdirSync(dir)
    .filter((name) => isNoticeName(name) && statSync(path.join(dir, name)).isFile())
    .sort();
}

function withNotices(pkg) {
  const manifest = readJSON(path.join(pkg.dir, 'package.json'));
  const license = typeof manifest.license === 'string' ? manifest.license : JSON.stringify(manifest.license ?? null);
  const excerpt = LICENSE_EXCERPTS[pkg.name];
  const files = noticeFiles(pkg.dir).map((name) => {
    let bytes = readFileSync(path.join(pkg.dir, name));
    if (excerpt && excerpt.file === name) {
      const text = bytes.toString('utf8');
      const at = text.indexOf(excerpt.until);
      if (at < 0) {
        throw new Error(`${pkg.name}/${name} no longer contains ${JSON.stringify(excerpt.until)}; review LICENSE_EXCERPTS`);
      }
      bytes = Buffer.from(text.slice(0, at), 'utf8');
    }
    return { name, text: normalizeText(bytes, `${pkg.name}/${name}`) };
  });
  const out = { name: pkg.name, version: pkg.version, license, roles: [...pkg.roles].sort(), files, note: excerpt ? excerpt.note : '' };
  if (files.length > 0) {
    return out;
  }
  const exception = NO_LICENSE_FILE[pkg.name];
  if (!exception || exception.version !== pkg.version || exception.license !== license) {
    throw new Error(
      `${pkg.name} ${pkg.version} ships no license file; review its license and, if it is ` +
        'acceptable, record it in NO_LICENSE_FILE in frontend/scripts/bundle-notices-lib.mjs',
    );
  }
  const author = typeof manifest.author === 'string' ? manifest.author : manifest.author?.name;
  if (license !== 'MIT' || !author) {
    throw new Error(`${pkg.name}: only an MIT package with a named author can be listed without a license file`);
  }
  out.note = `${exception.checked} Its package.json declares the MIT License and the author below.`;
  out.files = [{ name: 'MIT License terms, with the package.json author as copyright holder', text: `Copyright (c) ${author}\n\n${MIT_TERMS}` }];
  return out;
}

const RULE = '='.repeat(80);

export function renderNotices(packages) {
  const out = [
    'Frontend package notices for GoPMgr',
    '===================================',
    '',
    "GoPMgr's interface is a web bundle embedded in its binary. Besides",
    "GoPMgr's own code, the bundle contains code from the npm packages listed",
    "below. Each section reproduces that package's license and notice files as",
    'published; only line endings and trailing spaces are normalized.',
    '',
    'Generated by frontend/scripts/bundle-notices.mjs from the production build.',
    'Do not edit this file by hand; run `make frontend-notices`.',
    '',
    'Contents',
    '--------',
    '',
    ...packages.map((p) => `  ${p.name} ${p.version}`),
  ];
  for (const p of packages) {
    out.push('', RULE, `Package: ${p.name} ${p.version}`, `License (package.json): ${p.license}`, `Included as: ${p.roles.join('; ')}`);
    if (p.note) {
      out.push(`Note: ${p.note}`);
    }
    for (const f of p.files) {
      out.push('', `----- ${f.name} -----`, '', f.text.replace(/\n$/, ''));
    }
  }
  return out.join('\n') + '\n';
}

export function firstDifference(have, want) {
  const a = have.split('\n');
  const b = want.split('\n');
  for (let i = 0; i < Math.max(a.length, b.length); i++) {
    if (a[i] !== b[i]) {
      return `line ${i + 1}: have ${JSON.stringify(a[i] ?? '')}, want ${JSON.stringify(b[i] ?? '')}`;
    }
  }
  return 'files differ';
}
