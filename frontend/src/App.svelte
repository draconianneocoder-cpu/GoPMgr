<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { session, goto } from './lib/session.svelte';
  import { applyTheme } from './lib/theme';
  import { autosave } from './lib/autosave.svelte';

  import ToastContainer from './lib/components/ToastContainer.svelte';

  // On sign-in, load the user's app settings to apply the UI theme and the
  // auto-save interval; on sign-out, revert to the dark default and stop
  // auto-save (the login screen is always dark with no editors open).
  $effect(() => {
    if (session.user) {
      const p = window.go?.main?.App?.GetAppInfo?.();
      if (p) {
        p.then((info) => {
          applyTheme(info?.settings?.app_theme);
          autosave.setInterval(info?.settings?.auto_save_seconds ?? 0);
        }).catch(() => {});
      }
    } else {
      applyTheme('dark');
      autosave.setInterval(0);
    }
  });

  type RouteComponentModule = { default: any };
  type RouteLoader = () => Promise<RouteComponentModule>;

  const routeLoaders: Record<string, RouteLoader> = {
    login: () => import('./lib/components/auth/Login.svelte'),
    create_account: () => import('./lib/components/auth/CreateAccount.svelte'),
    recovery_reset: () => import('./lib/components/auth/RecoveryReset.svelte'),
    project_picker: () => import('./lib/components/project/ProjectPicker.svelte'),
    portfolio: () => import('./lib/components/project/Portfolio.svelte'),
    app_settings: () => import('./lib/components/AppSettings.svelte'),
    admin_panel: () => import('./lib/components/admin/AdminPanel.svelte'),
    help: () => import('./lib/components/HelpGuide.svelte'),
    dashboard: () => import('./lib/components/project/Dashboard.svelte'),
    wbs: () => import('./lib/components/charts/WBSEditor.svelte'),
    network: () => import('./lib/components/charts/NetworkEditor.svelte'),
    pert: () => import('./lib/components/charts/PERTEditor.svelte'),
    cpm: () => import('./lib/components/charts/CPMEditor.svelte'),
    gantt: () => import('./lib/components/charts/GanttEditor.svelte'),
    fishbone: () => import('./lib/components/charts/FishboneEditor.svelte'),
    cause_effect: () => import('./lib/components/charts/CauseEffectEditor.svelte'),
    workflow: () => import('./lib/components/charts/WorkflowEditor.svelte'),
    activity: () => import('./lib/components/charts/ActivityEditor.svelte'),
    raci: () => import('./lib/components/charts/RACIEditor.svelte'),
    swot: () => import('./lib/components/charts/SWOTEditor.svelte'),
    stakeholder: () => import('./lib/components/charts/StakeholderEditor.svelte'),
    matrix: () => import('./lib/components/charts/MatrixEditor.svelte'),
    line: () => import('./lib/components/charts/LineEditor.svelte'),
    bar: () => import('./lib/components/charts/BarEditor.svelte'),
    pie: () => import('./lib/components/charts/PieEditor.svelte'),
    pareto: () => import('./lib/components/charts/ParetoEditor.svelte'),
    burnup: () => import('./lib/components/charts/BurnUpEditor.svelte'),
    burndown: () => import('./lib/components/charts/BurnDownEditor.svelte'),
    cumulative_flow: () => import('./lib/components/charts/CumulativeFlowEditor.svelte'),
    control: () => import('./lib/components/charts/ControlChartEditor.svelte'),
    charter: () => import('./lib/components/documents/CharterEditor.svelte'),
    documents: () => import('./lib/components/documents/CharterEditor.svelte'),
    report_composer: () => import('./lib/components/documents/ReportComposer.svelte'),
    kanban: () => import('./lib/components/agile/KanbanBoard.svelte'),
    backlog: () => import('./lib/components/agile/Backlog.svelte'),
    sprints: () => import('./lib/components/agile/SprintList.svelte'),
    dora: () => import('./lib/components/agile/DORADashboard.svelte'),
    sigma_dashboard: () => import('./lib/components/sigma/SigmaWorkspace.svelte'),
    sigma_project: () => import('./lib/components/sigma/SigmaProjectView.svelte'),
    launchpad: () => import('./lib/components/project/ProjectLaunchpad.svelte'),
    stakeholders: () => import('./lib/components/project/StakeholderManager.svelte'),
    timeline: () => import('./lib/components/project/TimelineView.svelte'),
    project_settings: () => import('./lib/components/project/ProjectSettings.svelte'),
    scenario_chart: () => import('./lib/components/project/ScenarioChartEditor.svelte'),
  };

  let RouteComponent = $state<any>(null);
  let routeProps = $state<Record<string, unknown>>({});
  let routeError = $state('');
  let routeToken = 0;

  function propsForView(view: string): Record<string, unknown> {
    if (view === 'launchpad') {
      return {
        onCreated: (p: ProjectMeta, projectPath?: string) => {
          session.project = p;
          session.projectPath = projectPath ?? null;
          goto('dashboard');
        },
        onCancel: () => goto('project_picker'),
      };
    }
    return {};
  }

  $effect(() => {
    const view = session.view;
    const loader = routeLoaders[view];
    routeProps = propsForView(view);
    routeError = '';

    if (!loader) {
      RouteComponent = null;
      return;
    }

    const token = ++routeToken;
    RouteComponent = null;
    loader()
      .then((mod) => {
        if (token === routeToken) {
          RouteComponent = mod.default;
        }
      })
      .catch((err) => {
        if (token === routeToken) {
          routeError = String(err?.message ?? err);
          RouteComponent = null;
        }
      });
  });

  // On first mount, wire the native menu and check whether a user is already
  // signed in (the Go side keeps state across `wails dev` HMR cycles).
  onMount(async () => {
    // Native menu items (built in Go) emit these events; turn them into
    // navigation. `window.runtime` is injected by the Wails runtime.
    const rt = (window as any).runtime;
    if (rt?.EventsOn) {
      rt.EventsOn('menu:new-project', () => {
        if (session.user) goto('launchpad');
      });
      rt.EventsOn('menu:open-project', () => {
        if (session.user) goto('project_picker');
      });
      rt.EventsOn('menu:settings', () => {
        if (session.user && session.project) goto('project_settings');
      });
      rt.EventsOn('menu:close-project', async () => {
        if (!session.project) return;
        try {
          await window.go.main.App.CloseProject();
        } catch {
          /* ignore */
        }
        session.project = null;
        session.projectPath = null;
        goto('portfolio');
      });
      rt.EventsOn('menu:dashboard', () => {
        if (session.user) goto('portfolio');
      });
      rt.EventsOn('menu:app-settings', () => {
        if (session.user) goto('app_settings');
      });
      rt.EventsOn('menu:help', () => {
        if (session.user) goto('help');
      });
    }

    if (!window.go?.main?.App?.CurrentUser) return;
    try {
      const u = await window.go.main.App.CurrentUser();
      if (u) {
        session.user = u;
        goto('portfolio');
      }
    } catch {
      // No active session — stay on login.
    }
  });
</script>

{#if routeError}
  <div class="min-h-screen bg-slate-950 text-slate-200 flex items-center justify-center">
    <div class="text-center space-y-4">
      <p class="text-sm text-red-400 break-words" role="alert">Failed to load view: {routeError}</p>
      <button
        onclick={() => goto('dashboard')}
        class="text-xs bg-cyan-600 hover:bg-cyan-500 text-white font-bold uppercase px-3 py-2 rounded"
      >
        Back to dashboard
      </button>
    </div>
  </div>
{:else if RouteComponent}
  <RouteComponent {...routeProps} />
{:else if routeLoaders[session.view]}
  <div class="min-h-screen bg-slate-950 text-slate-400 flex items-center justify-center">
    <div class="flex items-center gap-3" role="status" aria-live="polite">
      <span
        class="h-4 w-4 rounded-full border-2 border-slate-700 border-t-cyan-400 animate-spin"
        aria-hidden="true"
      ></span>
      <p class="text-xs uppercase tracking-widest">Loading</p>
    </div>
  </div>
{:else}
  <!-- Safety fallback: all known views are in routeLoaders above. This
       branch is only reached if session.view is set to an unrecognised
       string, which should not happen in normal operation. -->
  <div class="min-h-screen bg-slate-950 text-slate-200 flex items-center justify-center">
    <div class="text-center space-y-4">
      <p class="text-sm text-slate-500">
        Unknown view. Please navigate back to the dashboard.
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

  <ToastContainer />
