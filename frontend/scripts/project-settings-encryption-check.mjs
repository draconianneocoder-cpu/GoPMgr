// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
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
const wailsWindowTypes = readFileSync(new URL('../src/wails-window.d.ts', import.meta.url), 'utf8');

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
    launchpad.includes('onCreated(res.project, res.path)'),
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
  [
    'settings exposes compliance-mode audit verification toggle',
    component.includes('let complianceMode = $state(false)') &&
      component.includes('Verify tamper-evident audit trail on open'),
  ],
  [
    'settings exposes audit verification report export action',
    component.includes('exportAuditVerificationReport') &&
      component.includes('Export audit verification report'),
  ],
  [
    'settings exposes audit repair evidence export action',
    component.includes('exportAuditRepairEvidence') &&
      component.includes('Export audit repair evidence'),
  ],
  [
    'settings loads and saves compliance_mode through Wails settings',
    component.includes('complianceMode = s.compliance_mode ?? false') &&
      component.includes('compliance_mode: complianceMode') &&
      component.includes('complianceMode = defaults.compliance_mode ?? false'),
  ],
  [
    'tracked Wails UserSettings shim includes compliance_mode',
    wailsWindowTypes.includes('compliance_mode?: boolean'),
  ],
  [
    'tracked Wails App shim includes audit verification report export',
    wailsWindowTypes.includes('ExportAuditVerificationReport: () => Promise<string>'),
  ],
  [
    'tracked Wails App shim includes audit repair evidence export',
    wailsWindowTypes.includes('ExportAuditRepairEvidence: () => Promise<string>'),
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
