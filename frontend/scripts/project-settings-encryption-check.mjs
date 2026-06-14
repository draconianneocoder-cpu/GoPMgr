// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { readFileSync } from 'node:fs';

const component = readFileSync(
  new URL('../src/lib/components/project/ProjectSettings.svelte', import.meta.url),
  'utf8',
);
const sessionStore = readFileSync(
  new URL('../src/lib/session.svelte.ts', import.meta.url),
  'utf8',
);
const picker = readFileSync(
  new URL('../src/lib/components/project/ProjectPicker.svelte', import.meta.url),
  'utf8',
);
const launchpad = readFileSync(
  new URL('../src/lib/components/project/ProjectLaunchpad.svelte', import.meta.url),
  'utf8',
);
const app = readFileSync(new URL('../src/App.svelte', import.meta.url), 'utf8');

const checks = [
  [
    'session tracks the current project file path',
    sessionStore.includes('projectPath: string | null'),
  ],
  [
    'project picker stores the opened project path',
    picker.includes('session.projectPath = p.path'),
  ],
  [
    'launchpad forwards the created project path',
    launchpad.includes('onCreated(project, projectPath)'),
  ],
  [
    'app stores launchpad-created project path',
    app.includes('onCreated: (p: ProjectMeta, projectPath?: string)'),
  ],
  [
    'settings loads encryption state from Wails',
    component.includes('IsProjectEncrypted(session.projectPath)'),
  ],
  [
    'settings migrates plaintext projects through Wails',
    component.includes('EncryptProjectAtRest(session.projectPath)'),
  ],
  [
    'settings exposes the required action label',
    component.includes('Encrypt database'),
  ],
  [
    'settings displays the migration backup path',
    component.includes('encryptionBackupPath'),
  ],
  [
    'settings handles legacy recovery-code reissue',
    component.includes('IssueRecoveryCodes()') &&
      component.includes('Reissue recovery codes'),
  ],
];

const failures = checks.filter(([, ok]) => !ok).map(([name]) => name);
if (failures.length > 0) {
  console.error('project-settings-encryption-check failed:');
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exit(1);
}

console.log('project-settings-encryption-check: all assertions passed.');
