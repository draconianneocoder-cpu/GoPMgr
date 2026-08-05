// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import helpGuideSource from './components/HelpGuide.svelte?raw';
import dashboardSource from './components/project/Dashboard.svelte?raw';
import projectLaunchpadTestSource from './components/project/ProjectLaunchpad.test.ts?raw';

// These strings describe real on-disk paths users read to find their own data
// (help text, error copy, test fixtures) — not branding. The Go side pins the
// same literals in internal/users/root_dir_test.go,
// project_path_confinement_test.go, and internal/db/backup_test.go (see
// DEVELOPER_HANDBOOK.md §9). The frontend has no Wails binding that exposes
// users.DefaultRootDir()'s resolved value (checked: main.go's App type binds
// no root/log-path getter), so these copies are hand-typed and can drift
// silently. Asserted as literals, not derived from a shared constant, for
// the same reason the Go tests do: there is no single source of truth to
// import from a .svelte file, so the next-best thing is a test that fails
// loudly the moment a rename or find/replace touches them.
//
// Both the data-root directory name and the project-file extension were
// renamed PMForge -> GoPMgr / .pmforge -> .gopmgr on 2026-08-04; updated
// here in lockstep with that rename. HelpGuide additionally documents the
// OLD extension (as still-openable, not as the current one) since app
// code keeps reading .pmforge files — see the "or an older .pmforge
// project" copy near the CLI maintenance section.
const MACOS_DATA_ROOT = '~/Library/Application Support/GoPMgr/';
const OTHER_DATA_ROOT = '~/Documents/GoPMgr/';
const PROJECT_EXTENSION = '.gopmgr';
const LEGACY_PROJECT_EXTENSION = '.pmforge';

describe('frontend copies of the persistence-boundary literals', () => {
  it('HelpGuide names the real macOS and Linux/Windows data-root paths', () => {
    expect(helpGuideSource).toContain(MACOS_DATA_ROOT);
    expect(helpGuideSource).toContain(OTHER_DATA_ROOT);
  });

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
