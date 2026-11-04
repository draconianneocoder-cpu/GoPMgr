// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
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
const generatedApp = readFileSync(new URL('../bindings/gopmgr/internal/desktop/app.ts', import.meta.url), 'utf8');
const generatedModels = readFileSync(new URL('../bindings/gopmgr/internal/db/models.ts', import.meta.url), 'utf8');
const scenarioChartEditor = readFileSync(
  new URL('../src/lib/components/project/ScenarioChartEditor.svelte', import.meta.url),
  'utf8',
);

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
    // Match the call shape, not exact identifier names: onCreated(...) whose
    // second argument references a "path". This previously hard-coded
    // `onCreated(project, projectPath)` and silently broke when the source was
    // refactored to `onCreated(res.project, res.path)` — the wiring was intact
    // but the literal drifted. Assert intent instead of a verbatim string.
    /onCreated\(\s*[\w.]+\s*,\s*[\w.]*path[\w.]*\s*\)/i.test(launchpad),
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
    'settings renews legacy recovery codes through the two-step panel',
    /<RecoveryCodesPanel\s[^>]*onrenewed=/.test(component) &&
      !component.includes('IssueRecoveryCodes()'),
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
    'settings exposes scenario metadata panel actions',
    component.includes('loadScenarios') &&
      component.includes('saveScenario') &&
      component.includes('deleteScenario') &&
      component.includes('What-if Scenarios') &&
      component.includes('Add scenario'),
  ],
  [
    'settings uses Wails scenario metadata methods',
    component.includes('desktopApi.ListScenarios()') &&
      component.includes('desktopApi.SaveScenario') &&
      component.includes('desktopApi.DeleteScenario'),
  ],
  [
    'settings exposes scenario chart copy controls',
    component.includes('loadScenarioSources') &&
      component.includes('branchScenarioChart') &&
      component.includes('Copy chart into scenario') &&
      component.includes('Copied scenario charts'),
  ],
  [
    'settings uses Wails scenario chart copy methods',
    component.includes('desktopApi.ListCharts') &&
      component.includes('desktopApi.ListScheduleBaselines') &&
      component.includes('desktopApi.BranchScenarioChart') &&
      component.includes('desktopApi.ListScenarioCharts'),
  ],
  [
    'settings exposes scenario promotion controls',
    component.includes('promoteScenarioChart') &&
      component.includes('Promote to baseline') &&
      component.includes('scenarioPromotionName'),
  ],
  [
    'settings uses Wails scenario promotion method',
    component.includes('desktopApi.PromoteScenarioChartToBaseline') &&
      generatedApp.includes('export function PromoteScenarioChartToBaseline('),
  ],
  [
    'settings exposes scenario comparison controls',
    component.includes('compareScenarioChart') &&
      component.includes('Compare to baseline') &&
      component.includes('scenarioComparisons'),
  ],
  [
    'settings uses Wails scenario comparison method',
    component.includes('desktopApi.CompareScenarioChart') &&
      generatedApp.includes('export function CompareScenarioChart(') &&
      generatedApp.includes('scenarioChartID: string') &&
      generatedApp.includes('$CancellablePromise<{ [_ in string]?: kernel$0.ScheduleVariance } | null>'),
  ],
  [
    'settings exposes scenario chart edit controls',
    component.includes('editScenarioChart') &&
      component.includes('saveScenarioChart') &&
      component.includes('Save scenario edits') &&
      component.includes('scenarioChartDraft'),
  ],
  [
    'settings uses Wails scenario chart save method',
    component.includes('desktopApi.SaveScenarioChart') &&
      generatedApp.includes('export function SaveScenarioChart(c: db$0.ScenarioChart): $CancellablePromise<db$0.ScenarioChart>'),
  ],
  [
    'session includes scenario chart editor route',
    sessionStore.includes("| 'scenario_chart'"),
  ],
  [
    'app lazy-loads scenario chart editor route',
    app.includes("scenario_chart: () => import('./lib/components/project/ScenarioChartEditor.svelte')"),
  ],
  [
    'settings opens copied scenario charts in the scenario editor',
    component.includes("goto('scenario_chart', chart.id)") &&
      component.includes('Open editor'),
  ],
  [
    'scenario chart editor uses scoped scenario APIs',
    scenarioChartEditor.includes('desktopApi.GetScenarioChart') &&
      scenarioChartEditor.includes('desktopApi.SaveScenarioChart') &&
      scenarioChartEditor.includes('desktopApi.CompareScenarioChart') &&
      scenarioChartEditor.includes('desktopApi.PromoteScenarioChartToBaseline') &&
      !scenarioChartEditor.includes('desktopApi.SaveChart') &&
      scenarioChartEditor.includes('Scenario chart editor'),
  ],
  [
    'scenario chart editor exposes review actions',
    scenarioChartEditor.includes('Compare to baseline') &&
      scenarioChartEditor.includes('Promote to baseline') &&
      scenarioChartEditor.includes('varianceRows'),
  ],
  [
    'generated Wails App service includes scenario chart get method',
    generatedApp.includes('export function GetScenarioChart(id: string): $CancellablePromise<db$0.ScenarioChart>'),
  ],
  [
    'settings loads and saves compliance_mode through Wails settings',
    component.includes('complianceMode = s.compliance_mode ?? false') &&
      component.includes('compliance_mode: complianceMode') &&
      component.includes('complianceMode = defaults.compliance_mode ?? false'),
  ],
  [
    'generated Wails UserSettings model includes compliance_mode',
    generatedModels.includes('"compliance_mode": boolean'),
  ],
  [
    'generated Wails App service includes audit verification report export',
    generatedApp.includes('export function ExportAuditVerificationReport(): $CancellablePromise<string>'),
  ],
  [
    'generated Wails App service includes audit repair evidence export',
    generatedApp.includes('export function ExportAuditRepairEvidence(): $CancellablePromise<string>'),
  ],
  [
    'generated Wails App service includes scenario metadata methods',
    generatedApp.includes('export function ListScenarios(): $CancellablePromise<db$0.Scenario[] | null>') &&
      generatedApp.includes('export function GetScenario(id: string): $CancellablePromise<db$0.Scenario>') &&
      generatedApp.includes('export function SaveScenario(s: db$0.Scenario): $CancellablePromise<db$0.Scenario>') &&
      generatedApp.includes('export function DeleteScenario(id: string): $CancellablePromise<void>'),
  ],
  [
    'generated Wails App service includes scenario chart partition methods',
    generatedApp.includes('export function BranchScenarioChart(') &&
      generatedApp.includes('scenarioID: string') &&
      generatedApp.includes('chartID: string') &&
      generatedApp.includes('baselineID: string') &&
      generatedApp.includes('$CancellablePromise<db$0.ScenarioChart>') &&
      generatedApp.includes('export function ListScenarioCharts(scenarioID: string): $CancellablePromise<db$0.ScenarioChart[] | null>'),
  ],
  [
    'generated Wails models includes Scenario type',
    generatedModels.includes('export interface Scenario') &&
      generatedModels.includes('"source_baseline_id": string') &&
      generatedModels.includes('"is_active": boolean'),
  ],
  [
    'generated Wails models includes ScenarioChart type',
    generatedModels.includes('export interface ScenarioChart') &&
      generatedModels.includes('"source_chart_id": string') &&
      generatedModels.includes('"baseline_data": string'),
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
