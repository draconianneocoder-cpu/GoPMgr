// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { createHash } from 'node:crypto';
import { mkdirSync, mkdtempSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { resolve } from 'node:path';
import { tmpdir } from 'node:os';
import { inflateSync } from 'node:zlib';

const root = resolve(import.meta.dirname, '..');
const requiredPNGs = new Map([
  ['assets/branding/source/gopmgr-logo-lockup-dark.png', [832, 1248]],
  ['assets/branding/source/gopmgr-logo-lockup-light.png', [784, 1168]],
  ['assets/branding/source/gopmgr-app-icon-light.png', [784, 1168]],
  ['assets/branding/platform/gopmgr-app-icon-dark.png', [1024, 1024]],
  ['assets/branding/platform/gopmgr-app-icon-light.png', [1024, 1024]],
  ['build/appicon.png', [1024, 1024]],
  ['frontend/public/branding/gopmgr-logo-lockup-dark.png', [341, 512]],
  ['frontend/public/branding/gopmgr-logo-lockup-light.png', [343, 512]],
  ['frontend/public/branding/gopmgr-app-icon-dark.png', [512, 512]],
  ['frontend/public/branding/gopmgr-app-icon-light.png', [512, 512]],
  ['frontend/public/branding/gopmgr-app-icon-dark-128.png', [128, 128]],
  ['frontend/public/branding/gopmgr-app-icon-light-128.png', [128, 128]],
]);
const nativeIcon = 'assets/branding/platform/gopmgr-app-icon-dark.png';

function fail(message) {
  console.error(`brand-assets: ${message}`);
  process.exitCode = 1;
}

function pngDimensions(bytes, relativePath) {
  const signature = '89504e470d0a1a0a';
  if (bytes.subarray(0, 8).toString('hex') !== signature || bytes.toString('ascii', 12, 16) !== 'IHDR') {
    throw new Error(`${relativePath} is not a PNG with an IHDR header`);
  }
  let offset = 8;
  const idat = [];
  let foundIEND = false;
  while (offset < bytes.length) {
    if (offset + 12 > bytes.length) throw new Error(`${relativePath} has a truncated PNG chunk`);
    const length = bytes.readUInt32BE(offset);
    const type = bytes.toString('ascii', offset + 4, offset + 8);
    const chunkEnd = offset + 12 + length;
    if (chunkEnd > bytes.length) throw new Error(`${relativePath} has an invalid ${type} chunk length`);
    if (type === 'IDAT') idat.push(bytes.subarray(offset + 8, offset + 8 + length));
    if (type === 'IEND') {
      if (length !== 0 || chunkEnd !== bytes.length) throw new Error(`${relativePath} has an invalid IEND chunk`);
      foundIEND = true;
      break;
    }
    offset = chunkEnd;
  }
  if (!foundIEND || idat.length === 0) throw new Error(`${relativePath} has no complete PNG image data`);
  inflateSync(Buffer.concat(idat));
  return [bytes.readUInt32BE(16), bytes.readUInt32BE(20)];
}

for (const [relativePath, expected] of requiredPNGs) {
  try {
    const actual = pngDimensions(readFileSync(resolve(root, relativePath)), relativePath);
    if (actual[0] !== expected[0] || actual[1] !== expected[1]) {
      fail(`${relativePath} dimensions are ${actual.join('x')}, expected ${expected.join('x')}`);
    }
  } catch (error) {
    fail(error instanceof Error ? error.message : String(error));
  }
}

for (const relativePath of [
  'assets/branding/macos/gopmgr-app-icon-dark.icns',
  'assets/branding/macos/gopmgr-app-icon-light.icns',
  'assets/branding/macos/gopmgr-logo-lockup-dark.icns',
  'assets/branding/macos/gopmgr-logo-lockup-light.icns',
]) {
  try {
    const bytes = readFileSync(resolve(root, relativePath));
    if (bytes.subarray(0, 4).toString('ascii') !== 'icns' || bytes.length < 16 || bytes.readUInt32BE(4) !== bytes.length) {
      fail(`${relativePath} is not an ICNS file`);
      continue;
    }
    let offset = 8;
    let decodedImage = false;
    while (offset < bytes.length) {
      if (offset + 8 > bytes.length) throw new Error(`${relativePath} has a truncated ICNS element`);
      const length = bytes.readUInt32BE(offset + 4);
      if (length < 8 || offset + length > bytes.length) throw new Error(`${relativePath} has an invalid ICNS element length`);
      const payload = bytes.subarray(offset + 8, offset + length);
      if (payload.subarray(0, 8).toString('hex') === '89504e470d0a1a0a') {
        pngDimensions(payload, `${relativePath} embedded image`);
        decodedImage = true;
      }
      offset += length;
    }
    if (!decodedImage) fail(`${relativePath} has no decodable embedded PNG rendition`);
  } catch (error) {
    fail(error instanceof Error ? error.message : String(error));
  }
}

try {
  const canonical = readFileSync(resolve(root, nativeIcon));
  const buildSource = readFileSync(resolve(root, 'build/appicon.png'));
  if (!canonical.equals(buildSource)) {
    fail(`build/appicon.png must exactly match ${nativeIcon}`);
  }
} catch (error) {
  fail(error instanceof Error ? error.message : String(error));
}

try {
  const fixture = mkdtempSync(resolve(tmpdir(), 'gopmgr-brand-clean-'));
  try {
    writeFileSync(resolve(fixture, 'Makefile'), readFileSync(resolve(root, 'Makefile')));
    const fixtureIcon = resolve(fixture, 'build/appicon.png');
    const fixtureBuildDir = resolve(fixture, 'build');
    mkdirSync(fixtureBuildDir, { recursive: true });
    writeFileSync(fixtureIcon, 'branding source fixture');
    const command = spawnSync('make', ['clean'], { cwd: fixture, encoding: 'utf8' });
    if (command.status !== 0) throw new Error(command.stderr || 'make clean failed in the fixture');
    if (!statSync(fixtureIcon).isFile()) fail('make clean must preserve build/appicon.png');
  } finally {
    rmSync(fixture, { recursive: true, force: true });
  }
} catch (error) {
  fail(error instanceof Error ? error.message : String(error));
}

if (process.exitCode) process.exit(process.exitCode);
const digest = createHash('sha256').update(readFileSync(resolve(root, nativeIcon))).digest('hex');
const size = statSync(resolve(root, nativeIcon)).size;
console.log(`brand-assets: ${requiredPNGs.size} PNGs and 4 ICNS exports fully decoded; native icon ${digest.slice(0, 12)} (${size} bytes).`);
