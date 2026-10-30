// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Writes third_party/FRONTEND-NOTICES.txt: the license and notice files of
// every npm package whose code is in the production bundle that the GoPMgr
// binary embeds.
//
// Usage, from frontend/ after `npm ci`:
//
//   node scripts/bundle-notices.mjs          # rewrite the file
//   node scripts/bundle-notices.mjs --check  # fail if it is out of date
//
// It runs the real production build (vite.config.ts) in memory, without
// writing dist/, and takes the package set from the bundle's own module
// graph and from license banners in generated assets, so dev-only tooling
// that leaves no code in the bundle is not listed.

import { readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { build } from 'vite';

import { collectPackages, firstDifference, renderNotices } from './bundle-notices-lib.mjs';

const frontendDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const nodeModules = path.join(frontendDir, 'node_modules');
const output = path.resolve(frontendDir, '..', 'third_party', 'FRONTEND-NOTICES.txt');
const check = process.argv.includes('--check');

async function bundleContents() {
  const moduleIds = new Set();
  const assetTexts = [];
  await build({
    root: frontendDir,
    configFile: path.join(frontendDir, 'vite.config.ts'),
    logLevel: 'error',
    build: { write: false, emptyOutDir: false },
    plugins: [
      {
        name: 'gopmgr-bundle-notices',
        generateBundle(_options, bundle) {
          for (const item of Object.values(bundle)) {
            if (item.type === 'chunk') {
              Object.keys(item.modules).forEach((id) => moduleIds.add(id));
              assetTexts.push(item.code);
            } else if (typeof item.source === 'string') {
              assetTexts.push(item.source);
            }
          }
        },
      },
    ],
  });
  return { moduleIds: [...moduleIds], assetTexts };
}

async function main() {
  const { moduleIds, assetTexts } = await bundleContents();
  const packages = collectPackages({ moduleIds, assetTexts, frontendDir, nodeModules });
  const content = renderNotices(packages);
  const shown = path.relative(path.resolve(frontendDir, '..'), output);
  if (!check) {
    await writeFile(output, content);
    console.log(`bundle-notices: wrote ${shown} (${packages.length} packages)`);
    return;
  }
  let have = '';
  try {
    have = await readFile(output, 'utf8');
  } catch (err) {
    throw new Error(`read ${shown}: ${err.message}; run \`make frontend-notices\``);
  }
  if (have !== content) {
    throw new Error(`${shown} is out of date (${firstDifference(have, content)}); run \`make frontend-notices\` and commit the result`);
  }
  console.log(`bundle-notices: ${shown} is current (${packages.length} packages)`);
}

main().catch((err) => {
  console.error(`bundle-notices: ${err.message}`);
  process.exit(1);
});
