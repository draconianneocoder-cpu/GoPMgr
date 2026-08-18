// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Single source of truth for industry-scoped methodology recommendations,
// shared by ProjectLaunchpad's creation wizard and ProjectSettings'
// reclassification panel so the two surfaces can never independently
// drift on what a given industry's methodology options are. IDs match
// the JDM's `methodology` column (lowercase).
export const METHODOLOGIES: Record<string, { id: string; label: string; blurb: string }[]> = {
  business: [
    { id: 'lean', label: 'Lean', blurb: 'Eliminate waste; flow-based.' },
    { id: 'six_sigma', label: 'Six Sigma', blurb: 'Process improvement via DMAIC.' },
    { id: 'okrs', label: 'OKRs', blurb: 'Objectives & key results.' },
  ],
  administration: [
    { id: 'waterfall', label: 'Waterfall', blurb: 'Linear, sequential phases.' },
    { id: 'prince2', label: 'PRINCE2', blurb: 'Stage-gated governance.' },
    { id: 'pmbok', label: 'PMBOK', blurb: 'PMI process groups.' },
  ],
  engineering: [
    { id: 'cpm', label: 'Critical Path', blurb: 'Network-based scheduling.' },
    { id: 'waterfall', label: 'Waterfall', blurb: 'Sequential design / build / test.' },
    { id: 'six_sigma', label: 'Six Sigma', blurb: 'Quality control loops.' },
  ],
  software: [
    { id: 'scrum', label: 'Scrum', blurb: 'Time-boxed sprints, backlog.' },
    { id: 'kanban', label: 'Kanban', blurb: 'Continuous flow, WIP limits.' },
    { id: 'scrumban', label: 'Scrumban', blurb: 'Hybrid: backlog + flow.' },
  ],
  construction: [
    { id: 'waterfall', label: 'Waterfall', blurb: 'Phase-gated build.' },
    { id: 'lean', label: 'Lean Construction', blurb: 'Pull planning; minimise waste.' },
    { id: 'cpm', label: 'CPM', blurb: 'Critical-path scheduling.' },
  ],
  custom: [{ id: 'custom', label: 'Build it yourself', blurb: 'No starter artifacts.' }],
};
