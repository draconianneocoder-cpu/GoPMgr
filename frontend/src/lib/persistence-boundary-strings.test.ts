// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import helpGuideSource from './components/HelpGuide.svelte?raw';
import dashboardSource from './components/project/Dashboard.svelte?raw';
import projectLaunchpadTestSource from './components/project/ProjectLaunchpad.test.ts?raw';

// File extensions are a persistence boundary, not branding. New projects use
// .gopmgr while existing .pmforge projects remain readable. Help text avoids
// hard-coded per-user filesystem paths because App Settings exposes the
// resolved data location.
const PROJECT_EXTENSION = '.gopmgr';
const LEGACY_PROJECT_EXTENSION = '.pmforge';

describe('frontend copies of the persistence-boundary literals', () => {
  it('HelpGuide names both the current .gopmgr and the still-openable legacy .pmforge project-file extension', () => {
    expect(helpGuideSource).toContain(PROJECT_EXTENSION);
    expect(helpGuideSource).toContain(LEGACY_PROJECT_EXTENSION);
  });

  it('Dashboard names the real .gopmgr project-file extension', () => {
    expect(dashboardSource).toContain(PROJECT_EXTENSION);
  });

  it('ProjectLaunchpad test fixtures use the real .gopmgr project-file extension', () => {
    expect(projectLaunchpadTestSource).toContain(PROJECT_EXTENSION);
  });
});
