// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import { capitalised, term } from './terminology';

describe('term', () => {
  it('falls back to the generic English word when methodology is blank', () => {
    expect(term(undefined, 'task')).toBe('task');
    expect(term('', 'milestone')).toBe('milestone');
  });

  it('resolves methodology-specific overrides', () => {
    expect(term('scrum', 'task')).toBe('user story');
    expect(term('kanban', 'task')).toBe('card');
    expect(term('cpm', 'deadline')).toBe('late finish');
  });

  it('falls back to the generic word for a term a methodology does not override', () => {
    // scrumban overrides task/deadline/iteration but not milestone.
    expect(term('scrumban', 'milestone')).toBe('milestone');
  });

  it('is case-insensitive on the methodology name', () => {
    expect(term('SCRUM', 'task')).toBe('user story');
    expect(term('Kanban', 'task')).toBe('card');
  });

  it('falls back to the generic word for an unknown methodology', () => {
    expect(term('not-a-real-methodology', 'task')).toBe('task');
  });
});

describe('capitalised', () => {
  it('uppercases only the first character of the resolved term', () => {
    expect(capitalised('scrum', 'tasks')).toBe('User stories');
  });

  it('uses the generic default when methodology is blank', () => {
    expect(capitalised(undefined, 'retrospective')).toBe('Retrospective');
  });
});
