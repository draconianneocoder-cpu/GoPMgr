// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import { shouldAutoCheckForUpdates } from './update-gate';

describe('shouldAutoCheckForUpdates', () => {
  it('does not check when the user has not opted in', () => {
    expect(shouldAutoCheckForUpdates({ auto_check_updates: false }, false)).toBe(false);
  });

  it('does not check when settings are missing entirely', () => {
    expect(shouldAutoCheckForUpdates(undefined, false)).toBe(false);
    expect(shouldAutoCheckForUpdates(null, false)).toBe(false);
  });

  it('checks once when opted in and not yet checked this session', () => {
    expect(shouldAutoCheckForUpdates({ auto_check_updates: true }, false)).toBe(true);
  });

  it('does not check again once already checked this session', () => {
    expect(shouldAutoCheckForUpdates({ auto_check_updates: true }, true)).toBe(false);
  });
});
