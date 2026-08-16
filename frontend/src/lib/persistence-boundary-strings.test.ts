// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

// Read source files directly from disk rather than via Vite's `?raw` import.
// A `?raw` import pulls the target file into this test's own module graph
// under a raw-text loader, which suppresses Vitest's `--coverage.all`
// instrumentation for that file -- it stops counting as "never imported,
// needs synthetic instrumentation" but was never executed as real
// Svelte/JS either, so it silently reports as 0/0 covered statements
// instead of its true (typically much larger) uncovered count. Confirmed
// 2026-08-15: this shadowed HelpGuide.svelte, HelpFeatures.svelte,
// HelpTroubleshooting.svelte, and Dashboard.svelte to 0/0 in the coverage
// ratchet. `readFileSync` never enters Vite's module graph, so none of
// these files are shadowed.
const read = (relativePath: string) => readFileSync(fileURLToPath(new URL(relativePath, import.meta.url)), 'utf-8');

const helpGuideSource = read('./components/HelpGuide.svelte');
const helpFeaturesSource = read('./components/help/HelpFeatures.svelte');
const helpTroubleshootingSource = read('./components/help/HelpTroubleshooting.svelte');
const dashboardSource = read('./components/project/Dashboard.svelte');
const projectLaunchpadTestSource = read('./components/project/ProjectLaunchpad.test.ts');

// HelpGuide.svelte was split into HelpGuide.svelte (shell) plus six
// Help*.svelte content components under ./components/help/ (2026-08-13).
// The .gopmgr/.pmforge text this suite checks for lives in HelpFeatures
// (the "Backups & Data Safety" section) and HelpTroubleshooting (the "cli"
// section) -- checked across the concatenation of all Help*.svelte sources
// rather than any one file, so a future reorganization of section content
// between these files doesn't silently break this check.
const allHelpSource = [helpGuideSource, helpFeaturesSource, helpTroubleshootingSource].join('\n');

// File extensions are a persistence boundary, not branding. New projects use
// .gopmgr while existing .pmforge projects remain readable. Help text avoids
// hard-coded per-user filesystem paths because App Settings exposes the
// resolved data location.
const PROJECT_EXTENSION = '.gopmgr';
const LEGACY_PROJECT_EXTENSION = '.pmforge';

describe('frontend copies of the persistence-boundary literals', () => {
  it('HelpGuide names both the current .gopmgr and the still-openable legacy .pmforge project-file extension', () => {
    expect(allHelpSource).toContain(PROJECT_EXTENSION);
    expect(allHelpSource).toContain(LEGACY_PROJECT_EXTENSION);
  });

  it('Dashboard names the real .gopmgr project-file extension', () => {
    expect(dashboardSource).toContain(PROJECT_EXTENSION);
  });

  it('ProjectLaunchpad test fixtures use the real .gopmgr project-file extension', () => {
    expect(projectLaunchpadTestSource).toContain(PROJECT_EXTENSION);
  });
});
