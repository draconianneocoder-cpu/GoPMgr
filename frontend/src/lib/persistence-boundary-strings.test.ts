// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import helpGuideSource from './components/HelpGuide.svelte?raw';
import dashboardSource from './components/project/Dashboard.svelte?raw';
import projectLaunchpadTestSource from './components/project/ProjectLaunchpad.test.ts?raw';

// These strings describe real on-disk paths users read to find their own data
// (help text, error copy, test fixtures) — not branding. The Go side pins the
// same four literals in internal/users/root_dir_test.go,
// project_path_confinement_test.go, and internal/db/backup_test.go (see
// DEVELOPER_HANDBOOK.md §9, 2026-08-04 entry). The frontend has no Wails
// binding that exposes users.DefaultRootDir()'s resolved value (checked:
// main.go's App type binds no root/log-path getter), so these copies are
// hand-typed and can drift silently. Asserted as literals, not derived from a
// shared constant, for the same reason the Go tests do: there is no single
// source of truth to import from a .svelte file, so the next-best thing is a
// test that fails loudly the moment a rename or find/replace touches them.
const MACOS_DATA_ROOT = '~/Library/Application Support/PMForge/';
const OTHER_DATA_ROOT = '~/Documents/PMForge/';
const PROJECT_EXTENSION = '.pmforge';

describe('frontend copies of the persistence-boundary literals', () => {
  it('HelpGuide names the real macOS and Linux/Windows data-root paths', () => {
    expect(helpGuideSource).toContain(MACOS_DATA_ROOT);
    expect(helpGuideSource).toContain(OTHER_DATA_ROOT);
  });

  it('HelpGuide names the real .pmforge project-file extension', () => {
    expect(helpGuideSource).toContain(PROJECT_EXTENSION);
  });

  it('Dashboard names the real .pmforge project-file extension', () => {
    expect(dashboardSource).toContain(PROJECT_EXTENSION);
  });

  it('ProjectLaunchpad test fixtures use the real .pmforge project-file extension', () => {
    expect(projectLaunchpadTestSource).toContain(PROJECT_EXTENSION);
  });
});
