// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Methodology wording. A project's methodology picks between a few fixed
// catalog messages, as Azure DevOps's process templates name the estimate
// field Story Points, Effort or Size. Schedule editors (Gantt, CPM, RACI)
// say "task" for every methodology, as scheduling tools do. See the
// 2026-11-09 decision in docs/design/ADR-008-localization.md.

import type { MessageKey } from './messages/en';

// Methodologies whose teams estimate work items in story points.
const STORY_POINT_METHODOLOGIES = new Set(['scrum', 'scrumban']);

// estimateLabelKey returns the catalog key naming a work item's estimate
// field. methodology is the project's stored value, compared without case.
export function estimateLabelKey(methodology: string | null | undefined): MessageKey {
  return STORY_POINT_METHODOLOGIES.has((methodology ?? '').toLowerCase())
    ? 'workItem.field.storyPoints'
    : 'workItem.field.points';
}
