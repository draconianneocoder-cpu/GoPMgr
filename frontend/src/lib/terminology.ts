// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// terminology.ts maps generic PMForge nouns to methodology-specific
// vocabulary so the GUI can speak the user's own language. Lookup
// table; no runtime cleverness.
//
// Usage in a Svelte component:
//
//   import { term } from '$lib/terminology';
//   $: workWord = term(session.project?.methodology, 'task');
//   // "task" / "user story" / "activity" / "work package"
//
// Keep the map small. If it grows past ~50 entries, promote to a
// JDM decision driven by zen-go (see internal/templates).

type Term =
  | 'task'
  | 'tasks'
  | 'deadline'
  | 'milestone'
  | 'estimate'
  | 'iteration'
  | 'planning_meeting'
  | 'retrospective';

const TABLE: Record<string, Partial<Record<Term, string>>> = {
  scrum: {
    task: 'user story',
    tasks: 'user stories',
    deadline: 'sprint end date',
    milestone: 'sprint goal',
    estimate: 'story points',
    iteration: 'sprint',
    planning_meeting: 'sprint planning',
    retrospective: 'sprint retrospective',
  },
  scrumban: {
    task: 'user story',
    tasks: 'user stories',
    deadline: 'service-level expectation',
    iteration: 'cadence',
  },
  kanban: {
    task: 'card',
    tasks: 'cards',
    deadline: 'service-level expectation',
    iteration: 'flow cycle',
  },
  cpm: {
    task: 'activity',
    tasks: 'activities',
    deadline: 'late finish',
    milestone: 'milestone',
    estimate: 'duration',
  },
  waterfall: {
    task: 'activity',
    tasks: 'activities',
    deadline: 'phase gate',
  },
  prince2: {
    task: 'work package',
    tasks: 'work packages',
    iteration: 'stage',
    milestone: 'end-stage assessment',
  },
  lean: {
    task: 'value step',
    tasks: 'value steps',
  },
  six_sigma: {
    task: 'measurement',
    tasks: 'measurements',
  },
  okrs: {
    task: 'key result action',
    tasks: 'key result actions',
    deadline: 'check-in',
    milestone: 'key result',
  },
};

const DEFAULTS: Record<Term, string> = {
  task: 'task',
  tasks: 'tasks',
  deadline: 'deadline',
  milestone: 'milestone',
  estimate: 'estimate',
  iteration: 'iteration',
  planning_meeting: 'planning meeting',
  retrospective: 'retrospective',
};

/**
 * term resolves a generic PM noun to its methodology-specific name.
 *
 * Falls back to the generic English term when the methodology is
 * blank or doesn't override the given term. Methodology comparison
 * is case-insensitive so callers can pass project.methodology
 * verbatim.
 */
export function term(methodology: string | undefined, key: Term): string {
  if (!methodology) return DEFAULTS[key];
  const tbl = TABLE[methodology.toLowerCase()];
  if (!tbl) return DEFAULTS[key];
  return tbl[key] ?? DEFAULTS[key];
}

/**
 * capitalised is a thin convenience around term() that uppercases
 * the first character — handy when the noun starts a sentence:
 *
 *   capitalised(methodology, 'tasks')  →  "User Stories"
 */
export function capitalised(methodology: string | undefined, key: Term): string {
  const s = term(methodology, key);
  return s.charAt(0).toUpperCase() + s.slice(1);
}
