// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Session is the in-memory state shared across components: who is
// logged in, which project is open. Uses Svelte 5 runes for
// reactivity. Import once and read directly.

export const session = $state<{
  user: Account | null;
  project: ProjectMeta | null;
  projectPath: string | null;
  // High-level view state: drives App.svelte's routing. The union
  // grows as more chart/document editors are implemented.
  view:
    | 'login'
    | 'create_account'
    | 'recovery_reset'
    | 'project_picker'
    | 'portfolio'
    | 'app_settings'
    | 'dashboard'
    | 'wbs'
    | 'network'
    | 'pert'
    | 'cpm'
    | 'gantt'
    | 'fishbone'
    | 'cause_effect'
    | 'workflow'
    | 'activity'
    | 'raci'
    | 'swot'
    | 'stakeholder'
    | 'matrix'
    | 'line'
    | 'bar'
    | 'pareto'
    | 'pie'
    | 'burnup'
    | 'burndown'
    | 'cumulative_flow'
    | 'control'
    | 'charter'
    | 'report_composer'
    | 'kanban'
    | 'backlog'
    | 'sprints'
    | 'dora'
    | 'sigma_dashboard'
    | 'sigma_project'
    | 'launchpad'
    | 'stakeholders'
    | 'timeline'
    | 'project_settings'
    | 'documents'
    | 'charts'
    | 'admin_panel'
    | 'help';
  // When `view` is a chart/doc editor, the currently-edited record ID.
  editingId: string | null;
}>({
  user: null,
  project: null,
  projectPath: null,
  view: 'login',
  editingId: null,
});

export function goto(view: typeof session.view, editingId: string | null = null) {
  session.view = view;
  session.editingId = editingId;
}
