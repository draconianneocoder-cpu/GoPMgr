// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { readFileSync } from 'node:fs';

const adminPanel = readFileSync(
  new URL('../src/lib/components/admin/AdminPanel.svelte', import.meta.url),
  'utf8',
);
const budgetPanel = readFileSync(
  new URL('../src/lib/components/project/BudgetPanel.svelte', import.meta.url),
  'utf8',
);
const wbsEditor = readFileSync(
  new URL('../src/lib/components/charts/WBSEditor.svelte', import.meta.url),
  'utf8',
);

const checks = [
  [
    'admin panel treats zero-value last_login as Never',
    adminPanel.includes('formatLastLogin') &&
      adminPanel.includes('getFullYear() <= 1') &&
      adminPanel.includes("formatLastLogin(user.last_login)"),
  ],
  [
    'budget panel uses compact numbers to prevent overflow',
    budgetPanel.includes('formatCompactCurrency') &&
      budgetPanel.includes('compact: true') &&
      budgetPanel.includes('truncate') &&
      budgetPanel.includes('title={fmt('),
  ],
  [
    'WBS editor rejects negative effort units',
    wbsEditor.includes('min="0"') &&
      wbsEditor.includes('normalizeEffort') &&
      wbsEditor.includes('Math.max(0'),
  ],
];

const failures = checks.filter(([, ok]) => !ok).map(([name]) => name);
if (failures.length > 0) {
  console.error('bug-report-regression-check failed:');
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exit(1);
}

console.log('bug-report-regression-check: all assertions passed.');
