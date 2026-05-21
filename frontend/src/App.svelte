<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { session, goto } from './lib/session.svelte';

  import Login from './lib/components/auth/Login.svelte';
  import CreateAccount from './lib/components/auth/CreateAccount.svelte';
  import RecoveryReset from './lib/components/auth/RecoveryReset.svelte';
  import ProjectPicker from './lib/components/project/ProjectPicker.svelte';
  import Dashboard from './lib/components/project/Dashboard.svelte';
  import WBSEditor from './lib/components/charts/WBSEditor.svelte';
  import NetworkEditor from './lib/components/charts/NetworkEditor.svelte';
  import PERTEditor from './lib/components/charts/PERTEditor.svelte';
  import CPMEditor from './lib/components/charts/CPMEditor.svelte';
  import FishboneEditor from './lib/components/charts/FishboneEditor.svelte';
  import CauseEffectEditor from './lib/components/charts/CauseEffectEditor.svelte';
  import WorkflowEditor from './lib/components/charts/WorkflowEditor.svelte';
  import ActivityEditor from './lib/components/charts/ActivityEditor.svelte';
  import RACIEditor from './lib/components/charts/RACIEditor.svelte';
  import SWOTEditor from './lib/components/charts/SWOTEditor.svelte';
  import StakeholderEditor from './lib/components/charts/StakeholderEditor.svelte';
  import MatrixEditor from './lib/components/charts/MatrixEditor.svelte';
  import LineEditor from './lib/components/charts/LineEditor.svelte';
  import BarEditor from './lib/components/charts/BarEditor.svelte';
  import PieEditor from './lib/components/charts/PieEditor.svelte';
  import ParetoEditor from './lib/components/charts/ParetoEditor.svelte';
  import BurnUpEditor from './lib/components/charts/BurnUpEditor.svelte';
  import BurnDownEditor from './lib/components/charts/BurnDownEditor.svelte';
  import CumulativeFlowEditor from './lib/components/charts/CumulativeFlowEditor.svelte';
  import ControlChartEditor from './lib/components/charts/ControlChartEditor.svelte';
  import CharterEditor from './lib/components/documents/CharterEditor.svelte';
  import ReportComposer from './lib/components/documents/ReportComposer.svelte';
  import KanbanBoard from './lib/components/agile/KanbanBoard.svelte';
  import Backlog from './lib/components/agile/Backlog.svelte';
  import SprintList from './lib/components/agile/SprintList.svelte';
  import DORADashboard from './lib/components/agile/DORADashboard.svelte';
  import ProjectLaunchpad from './lib/components/project/ProjectLaunchpad.svelte';
  import StakeholderManager from './lib/components/project/StakeholderManager.svelte';
  import TimelineView from './lib/components/project/TimelineView.svelte';
  import ProjectSettings from './lib/components/project/ProjectSettings.svelte';

  // On first mount, check whether a user is already signed in
  // (the Go side keeps state across `wails dev` HMR cycles).
  onMount(async () => {
    if (!window.go?.main?.App?.CurrentUser) return;
    try {
      const u = await window.go.main.App.CurrentUser();
      if (u) {
        session.user = u;
        goto('project_picker');
      }
    } catch {
      // No active session — stay on login.
    }
  });
</script>

{#if session.view === 'login'}
  <Login />
{:else if session.view === 'create_account'}
  <CreateAccount />
{:else if session.view === 'recovery_reset'}
  <RecoveryReset />
{:else if session.view === 'project_picker'}
  <ProjectPicker />
{:else if session.view === 'dashboard'}
  <Dashboard />
{:else if session.view === 'wbs'}
  <WBSEditor />
{:else if session.view === 'network'}
  <NetworkEditor />
{:else if session.view === 'pert'}
  <PERTEditor />
{:else if session.view === 'cpm'}
  <CPMEditor />
{:else if session.view === 'fishbone'}
  <FishboneEditor />
{:else if session.view === 'cause_effect'}
  <CauseEffectEditor />
{:else if session.view === 'workflow'}
  <WorkflowEditor />
{:else if session.view === 'activity'}
  <ActivityEditor />
{:else if session.view === 'raci'}
  <RACIEditor />
{:else if session.view === 'swot'}
  <SWOTEditor />
{:else if session.view === 'stakeholder'}
  <StakeholderEditor />
{:else if session.view === 'matrix'}
  <MatrixEditor />
{:else if session.view === 'line'}
  <LineEditor />
{:else if session.view === 'bar'}
  <BarEditor />
{:else if session.view === 'pie'}
  <PieEditor />
{:else if session.view === 'pareto'}
  <ParetoEditor />
{:else if session.view === 'burnup'}
  <BurnUpEditor />
{:else if session.view === 'burndown'}
  <BurnDownEditor />
{:else if session.view === 'cumulative_flow'}
  <CumulativeFlowEditor />
{:else if session.view === 'control'}
  <ControlChartEditor />
{:else if session.view === 'charter'}
  <CharterEditor />
{:else if session.view === 'report_composer'}
  <ReportComposer />
{:else if session.view === 'kanban'}
  <KanbanBoard />
{:else if session.view === 'backlog'}
  <Backlog />
{:else if session.view === 'sprints'}
  <SprintList />
{:else if session.view === 'dora'}
  <DORADashboard />
{:else if session.view === 'launchpad'}
  <ProjectLaunchpad
    onCreated={(p) => { session.project = p; goto('dashboard'); }}
    onCancel={() => goto('project_picker')}
  />
{:else if session.view === 'stakeholders'}
  <StakeholderManager />
{:else if session.view === 'timeline'}
  <TimelineView />
{:else if session.view === 'project_settings'}
  <ProjectSettings />
{:else}
  <!-- Placeholder for the other 17 chart types and 24 document types.
       Future sessions wire each one to a dedicated editor; until then
       this fallback panel lets the user navigate back to the dashboard. -->
  <div class="min-h-screen bg-slate-950 text-slate-200 flex items-center justify-center">
    <div class="text-center space-y-4">
      <p class="text-sm text-slate-500">
        This chart/document type does not yet have a dedicated editor.
      </p>
      <button
        onclick={() => goto('dashboard')}
        class="text-xs bg-cyan-600 hover:bg-cyan-500 text-white font-bold uppercase px-3 py-2 rounded"
      >
        Back to dashboard
      </button>
    </div>
  </div>
{/if}
