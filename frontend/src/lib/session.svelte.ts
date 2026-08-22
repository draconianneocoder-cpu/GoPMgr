// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Session is the in-memory state shared across components: who is
// logged in, which project is open. Uses Svelte 5 runes for
// reactivity. Import once and read directly.

import { autosave } from './autosave.svelte';

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
    | 'risk_matrix'
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
    | 'catalog'
    | 'timeline'
    | 'project_settings'
    | 'scenario_chart'
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

type PendingNavigation = {
  view: typeof session.view;
  editingId: string | null;
  beforeCommit?: () => Promise<boolean>;
};

export const navigation = $state<{
  pending: PendingNavigation | null;
  saving: boolean;
  error: string;
}>({ pending: null, saving: false, error: '' });

function commit(target: PendingNavigation): void {
  session.view = target.view;
  session.editingId = target.editingId;
}

async function finish(target: PendingNavigation): Promise<void> {
  if (target.beforeCommit && !(await target.beforeCommit())) return;
  commit(target);
}

export function requestNavigation(
  view: typeof session.view,
  editingId: string | null = null,
  beforeCommit?: () => Promise<boolean>,
): void {
  if (navigation.pending || navigation.saving) return;
  const target = { view, editingId, beforeCommit };
  if (autosave.hasDirty()) {
    navigation.pending = target;
    navigation.error = '';
    return;
  }
  void finish(target);
}

export function goto(view: typeof session.view, editingId: string | null = null): void {
  requestNavigation(view, editingId);
}

export async function saveAndContinueNavigation(): Promise<void> {
  const target = navigation.pending;
  if (!target) return;
  navigation.saving = true;
  navigation.error = '';
  if (!(await autosave.saveAll())) {
    navigation.error = autosave.lastError || 'Could not save the current editor.';
    navigation.saving = false;
    return;
  }
  navigation.pending = null;
  navigation.saving = false;
  await finish(target);
}

export async function discardAndContinueNavigation(): Promise<void> {
  const target = navigation.pending;
  if (!target) return;
  autosave.discardAll();
  navigation.pending = null;
  navigation.error = '';
  await finish(target);
}

export function cancelNavigation(): void {
  navigation.pending = null;
  navigation.error = '';
}
