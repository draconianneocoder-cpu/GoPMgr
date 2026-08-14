// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import helpGuideSource from './components/HelpGuide.svelte?raw';
import helpFeaturesSource from './components/help/HelpFeatures.svelte?raw';
import helpTroubleshootingSource from './components/help/HelpTroubleshooting.svelte?raw';
import dashboardSource from './components/project/Dashboard.svelte?raw';
import projectLaunchpadTestSource from './components/project/ProjectLaunchpad.test.ts?raw';

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
