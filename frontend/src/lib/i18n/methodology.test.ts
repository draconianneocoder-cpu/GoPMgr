// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import { estimateLabelKey } from './methodology';
import { en } from './messages/en';

describe('estimateLabelKey', () => {
  it('names the estimate "Story points" for Scrum and Scrumban, ignoring case', () => {
    for (const methodology of ['scrum', 'scrumban', 'Scrum', 'SCRUMBAN']) {
      expect(en[estimateLabelKey(methodology)]).toBe('Story points');
    }
  });

  it('keeps "Points" for every other, blank or unknown methodology', () => {
    for (const methodology of ['kanban', 'cpm', 'waterfall', 'six_sigma', 'custom', '', null, undefined, 'scrum-ish']) {
      expect(en[estimateLabelKey(methodology)]).toBe('Points');
    }
  });
});
