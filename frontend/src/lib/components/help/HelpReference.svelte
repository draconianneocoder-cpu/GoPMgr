<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import type { SectionId } from '../help-sections';

  let { active, nav }: { active: SectionId; nav: (id: SectionId) => void } = $props();
</script>

        <!-- ── Charts ──────────────────────────────── -->
        {#if active === 'charts'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Charts Reference</h2>
          <p class="text-sm text-slate-400 mb-5">GoPMgr includes 22 chart types organized across four rendering engines. Create them from the Project Dashboard's searchable chart-tool catalog; filter by engine when you already know the visual family you need.</p>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-3">DAG Engine — Schedule &amp; Hierarchy (7 charts)</h3>
            <p class="text-xs text-slate-500 mb-3">Activity-on-node directed graphs. Tasks are nodes; arrows are dependency relationships.</p>
            <div class="space-y-3">
              {#each [
                { name: 'Work Breakdown Structure (WBS)', desc: 'Hierarchical decomposition of project scope into deliverables and work packages. Root = project; leaf nodes = work packages.' },
                { name: 'Network Diagram', desc: 'Activity-on-node diagram showing precedence relationships. Displays ES/EF/LS/LF values and highlights the critical path.' },
                { name: 'PERT Chart', desc: 'Network diagram with three-point duration estimates per activity: Optimistic, Most Likely, Pessimistic. Use when durations are uncertain.' },
                { name: 'CPM Chart', desc: 'Activity nodes annotated with ES/EF/LS/LF and critical-path highlighting. Deterministic (single-point) durations.' },
                { name: 'Gantt Chart', desc: 'Schedule bars on a time axis with dependency arrows, critical-path highlighting, progress bars, and baseline overlay. Shares data with CPM.' },
                { name: 'Fishbone (Ishikawa) Diagram', desc: 'Root-cause analysis. Default cause categories (People, Process, Equipment, Materials, Environment, Measurement) branch from a central effect statement. Categories are editable.' },
                { name: 'Cause-and-Effect Diagram', desc: 'Generic cause/effect tree. More flexible than Fishbone; supports arbitrary nesting. Use when Fishbone categories do not fit the domain.' },
              ] as chart}
                <div>
                  <p class="text-sm font-medium text-slate-100">{chart.name}</p>
                  <p class="text-xs text-slate-400 mt-0.5">{chart.desc}</p>
                </div>
              {/each}
            </div>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-3">Statistical Engine — Data Visualization (8 charts)</h3>
            <p class="text-xs text-slate-500 mb-3">Data series charts. Enter data series; the chart renders the visualization.</p>
            <div class="space-y-3">
              {#each [
                { name: 'Line Chart', desc: 'One or more series against a continuous x-axis. Use for trends over time (KPIs, cost curves, performance metrics).' },
                { name: 'Bar Chart', desc: 'Categorical comparison with vertical or horizontal bars. Compare values across discrete categories or time periods.' },
                { name: 'Pareto Chart', desc: 'Bar chart sorted descending with a cumulative-percentage line overlay. Identifies the vital few causes (80/20 rule).' },
                { name: 'Pie Chart', desc: 'Part-to-whole composition for 2-6 categories. Best when the whole is meaningful (budget breakdown, effort split).' },
                { name: 'Burn-Up Chart', desc: 'Cumulative scope completed vs. total scope over time. Distinguishes scope growth from delivery progress.' },
                { name: 'Burn-Down Chart', desc: 'Remaining work over time with an ideal trajectory reference line. Tracks sprint or release schedule performance.' },
                { name: 'Cumulative Flow Diagram', desc: 'Stacked area chart of work in each workflow state over time. Band widths reveal WIP; band slopes reveal throughput.' },
                { name: 'Control Chart', desc: 'Time series with Upper (UCL) and Lower (LCL) Control Limits. Points outside limits or non-random patterns indicate special-cause variation.' },
              ] as chart}
                <div>
                  <p class="text-sm font-medium text-slate-100">{chart.name}</p>
                  <p class="text-xs text-slate-400 mt-0.5">{chart.desc}</p>
                </div>
              {/each}
            </div>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-3">Matrix Engine — Grid Diagrams (5 charts)</h3>
            <p class="text-xs text-slate-500 mb-3">Two-dimensional grids relating two sets of items or dimensions.</p>
            <div class="space-y-3">
              {#each [
                { name: 'RACI Matrix', desc: 'Responsibility assignment: Responsible (does the work), Accountable (owns the outcome), Consulted (input needed), Informed (kept updated) — per task/role cell. One A per task.' },
                { name: 'SWOT Matrix', desc: '2x2 grid: Strengths / Weaknesses (internal) vs. Opportunities / Threats (external). Favorable vs. unfavorable.' },
                { name: 'Stakeholder Analysis Matrix', desc: 'Stakeholders plotted on a Power vs. Interest grid. Position drives engagement strategy: high power/high interest = manage closely.' },
                { name: 'Matrix Diagram', desc: 'Generic m×n grid for relating any two dimensions — requirements traceability, quality function deployment, or any custom comparison.' },
                { name: 'Risk Matrix', desc: '5×5 probability and impact heat map for risks, active issues, and opportunities. Records include owner, status, response, and an optional linked task. A Risk Register document can explicitly refresh its linked matrix after validation.' },
              ] as chart}
                <div>
                  <p class="text-sm font-medium text-slate-100">{chart.name}</p>
                  <p class="text-xs text-slate-400 mt-0.5">{chart.desc}</p>
                </div>
              {/each}
            </div>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-3">Flow Engine — Process Diagrams (2 charts)</h3>
            <div class="space-y-3">
              {#each [
                { name: 'Workflow Diagram', desc: 'Process flow with decisions, gates, and parallel paths. Use for current-state and future-state process mapping in Lean, BPM, or SOP documentation.' },
                { name: 'Activity Diagram', desc: 'UML-style activity flow with swimlanes for different actors or systems. Use when the process crosses organizational or system boundaries.' },
              ] as chart}
                <div>
                  <p class="text-sm font-medium text-slate-100">{chart.name}</p>
                  <p class="text-xs text-slate-400 mt-0.5">{chart.desc}</p>
                </div>
              {/each}
            </div>
          </section>

        <!-- ── Documents Reference ─────────────────────────────────── -->
          <section class="mb-6 border-t border-slate-800 pt-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Editing schedule diagrams (WBS, Network, PERT, CPM, Workflow, Activity)</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li><span class="font-medium text-slate-100">+ Node</span> adds an activity; click a node (or Tab to it and press Enter/Space) and edit its fields in the side panel.</li>
              <li>To add a dependency: select the source node, click <span class="font-medium text-slate-100">Connect…</span>, then click the destination node. <span class="font-medium text-slate-100">Clear edges</span> removes a node's links; <span class="font-medium text-slate-100">Delete node</span> removes it and its edges.</li>
              <li>In CPM (and Gantt links), the edge label sets the <span class="font-medium text-slate-100">dependency type and lag</span>: <span class="font-mono text-xs">FS</span>, <span class="font-mono text-xs">SS</span>, <span class="font-mono text-xs">FF</span>, or <span class="font-mono text-xs">SF</span> with optional <span class="font-mono text-xs">+n</span>/<span class="font-mono text-xs">-n</span> days — e.g. <span class="font-mono text-xs">SS+2</span> (start 2 days after the predecessor starts) or <span class="font-mono text-xs">FS-1</span> (overlap by a day). Blank means FS. This drives the computed schedule.</li>
              <li>Activity diagrams add <span class="font-medium text-slate-100">swimlanes</span> (one per role) and UML node shapes — initial, activity, decision, fork/join, final — chosen when adding the node.</li>
            </ul>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Gantt editor specifics</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li>The left grid edits tasks (name, duration in days, % complete, milestone ◆, delete); the right canvas draws the bars. <span class="font-medium text-slate-100">− / +</span> zoom the day scale.</li>
              <li>Bar colours carry meaning: <span class="text-red-400 font-medium">red</span> = critical path, teal strip = % complete, grey ghost = baseline, orange outline = over-allocated resource, amber ! = violated constraint. Hover any bar for its full story.</li>
              <li><span class="font-medium text-slate-100">Set baseline</span> snapshots today's schedule; ghost bars then show drift against it as the plan changes.</li>
              <li>Links are managed in the panel below the grid — pick from/to tasks and an optional type/lag label (same <span class="font-mono text-xs">FS/SS/FF/SF±n</span> syntax as CPM).</li>
              <li>Real calendar dates appear on bars once the project has a start date in <button onclick={() => nav('project-settings')} class="text-cyan-400 underline hover:text-cyan-300">Project Settings</button>.</li>
            </ul>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Monte Carlo schedule risk (CPM)</h3>
            <ol class="space-y-1.5 text-sm text-slate-300 list-decimal list-inside">
              <li>In the CPM editor, give uncertain tasks optimistic / most-likely / pessimistic duration estimates (tasks without estimates keep their fixed duration).</li>
              <li>Choose a sampling distribution — triangular, beta-PERT, or normal — and run the simulation from the editor's side panel.</li>
              <li>Read the results: <span class="font-medium text-slate-100">P50 / P80 / P90</span> finish-day confidence points, a cumulative finish-probability S-curve, and a <span class="font-medium text-slate-100">tornado ranking</span> of which tasks drive the risk (critical-path frequency × P90−P50 spread).</li>
              <li>Use <span class="font-medium text-slate-100">Export PDF/A</span> to save a signed-off-ready risk report with the summary, S-curve, tornado drivers, and narrative.</li>
            </ol>
            <p class="text-sm text-slate-400 mt-2">
              CPM can also generate a <span class="font-medium text-slate-100">Resource Histogram</span> —
              demand bars with dashed capacity lines from stakeholder availability and Project
              Settings resource calendars.
            </p>
          </section>

          <section class="mb-4">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Grid &amp; data-series editors</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li><span class="font-medium text-slate-100">Stats charts</span> (Line, Bar, Pie, Pareto, Burn-Up/Down, Cumulative Flow, Control): enter series and values in the form under the live preview — it re-renders as you type. Control charts flag out-of-control points in red with the rule in the tooltip.</li>
              <li><span class="font-medium text-slate-100">RACI</span>: add roles and tasks, then set each cell to R/A/C/I from its dropdown. The validation tray warns when a task lacks exactly one Accountable.</li>
              <li><span class="font-medium text-slate-100">Matrix</span>: add rows and columns, type in any cell; row/column headers rename inline.</li>
              <li><span class="font-medium text-slate-100">SWOT</span>: four fixed quadrants; add/remove bullet items per quadrant.</li>
              <li><span class="font-medium text-slate-100">Fishbone / Cause-and-Effect</span>: manage categories and causes in the side panel (Fishbone can seed the classic six Ms); the diagram lays itself out.</li>
              <li>All editors save with <span class="font-mono text-xs">Ctrl/⌘+S</span> and participate in auto-save — see <button onclick={() => nav('shortcuts')} class="text-cyan-400 underline hover:text-cyan-300">Keyboard Shortcuts</button>.</li>
            </ul>
          </section>

        <!-- ── Documents ──────────────────────────────── -->
        {:else if active === 'documents'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Documents Reference</h2>
          <p class="text-sm text-slate-400 mb-2">25 structured document types organized by PMBOK process group. Each has a structured editor with typed fields. Chart references can be embedded in documents. Export formats depend on document type.</p>
          <p class="text-xs text-slate-500 mb-5">Create documents from the Project Dashboard's searchable controlled-document index; filter by lifecycle phase when you know where the artifact belongs.</p>

          {#each [
            { phase: 'Initiation', count: 5, docs: [
              { name: 'Project Charter (Word)', desc: 'Formally authorizes the project. Captures purpose, objectives, scope, stakeholders, high-level schedule and budget, and sponsor sign-off. The foundational document referenced by all other planning artifacts.' },
              { name: 'Project Charter (Excel)', desc: 'Same content as the Word charter; default export format is XLSX.' },
              { name: 'Business Case', desc: 'Justifies the project investment: costs, benefits, risks, NPV/ROI analysis, and strategic alignment. Primary input to the go/no-go decision.' },
              { name: 'Project Proposal', desc: 'Persuasive overview to win stakeholder buy-in before formal authorization. Less rigorous than the Charter.' },
              { name: 'Stakeholder Analysis Document', desc: 'Narrative companion to the Stakeholder Analysis Matrix chart. Documents individual engagement strategies. Links to a Stakeholder Analysis chart.' },
            ]},
            { phase: 'Planning', count: 14, docs: [
              { name: 'Project Plan (Word)', desc: 'Most comprehensive PM document. Consolidates scope, schedule, budget, quality, risk, communications, procurement, and team plans. Links to CPM schedule, WBS, and RACI charts.' },
              { name: 'Project Plan (Excel)', desc: 'Same content as Word plan; exports to XLSX.' },
              { name: 'Project Schedule', desc: 'Authoritative timeline: every task with durations, predecessors, and the critical path. Requires a linked CPM or Gantt chart.' },
              { name: 'Work Breakdown Structure', desc: 'Narrative around the WBS chart: deliverable definitions, work-package owners, and acceptance criteria. Requires a linked WBS chart.' },
              { name: 'RACI Chart Document', desc: 'Prose companion to the RACI Matrix chart: role definitions and effective dates. Requires a linked RACI chart.' },
              { name: 'Risk Register', desc: 'Catalogue of potential risks with probability, impact, risk score, owner, mitigation strategy, and contingency plan.' },
              { name: 'Scope Statement', desc: 'Defines exactly what the project will and will not deliver. Includes deliverables, acceptance criteria, constraints, assumptions, and exclusions.' },
              { name: 'Project Budget', desc: 'Cost estimate broken down by category: labor, materials, equipment, subcontractors, contingency, and management reserve.' },
              { name: 'Communication Plan', desc: 'Who needs what information, in what format, on what cadence, and via which channel.' },
              { name: 'Project Execution Plan', desc: 'Operational plan: detailed task breakdown, resource assignments, and execution timeline.' },
              { name: 'Statement of Work', desc: 'Formal scope, deliverables, timeline, and responsibility definition issued to vendors or contractors.' },
              { name: 'Procurement Plan', desc: 'What will be procured externally, via which method, from which vendors, and on what schedule.' },
              { name: 'Requirements Document', desc: 'Functional and non-functional specifications the project must satisfy.' },
              { name: 'Team Charter', desc: 'Roles, responsibilities, deliverables, working agreements, decision rights, and escalation paths for the project team.' },
            ]},
            { phase: 'Execution', count: 2, docs: [
              { name: 'Project Brief', desc: 'Short, audience-oriented summary of the plan for non-PM stakeholders.' },
              { name: 'Project Overview', desc: '1-page snapshot: timeline, milestones, budget status, and key roles. Use at reviews and steering committee meetings.' },
            ]},
            { phase: 'Monitoring', count: 3, docs: [
              { name: 'Status Report', desc: 'Periodic check-in: work completed, upcoming work, schedule and budget variance, open risks, blockers, and optional linked CPM schedule for Earned Value in combined reports.' },
              { name: 'Issue Log', desc: 'Tracks problems that have occurred (vs. risks, which are potential). Each issue has owner, priority, and resolution plan.' },
              { name: 'Change Request Form', desc: 'Formally proposes a change to a project baseline. Includes impact analysis and approval fields.' },
            ]},
            { phase: 'Closing', count: 1, docs: [
              { name: 'Project Closure', desc: 'Formal end-of-project record: final deliverable acceptance, contract closure status, lessons learned, and sponsor sign-off.' },
            ]},
          ] as group}
            <section class="mb-6">
              <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-3">
                {group.phase} <span class="text-slate-500 font-normal">({group.count})</span>
              </h3>
              <div class="space-y-3">
                {#each group.docs as doc}
                  <div>
                    <p class="text-sm font-medium text-slate-100">{doc.name}</p>
                    <p class="text-xs text-slate-400 mt-0.5">{doc.desc}</p>
                  </div>
                {/each}
              </div>
            </section>
          {/each}

        <!-- ── DMAIC Pack ──────────────────────────────── -->
        {:else if active === 'sigma-pack'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Six Sigma DMAIC Pack</h2>
          <p class="text-sm text-slate-400 mb-3">Dedicated structured environment for DMAIC projects, separate from the standard Launchpad chart seeds. Provides two views: Sigma Workspace (project dashboard) and Sigma Project (active project editor with phase-by-phase structure).</p>
          <p class="text-xs bg-slate-900 border border-slate-700 rounded px-3 py-2 text-slate-400 mb-5">
            For how the Launchpad seeds initial charts when selecting Six Sigma as a methodology, see
            <button onclick={() => nav('six-sigma-method')} class="text-cyan-400 underline hover:text-cyan-300">Six Sigma (Methodology)</button>.
          </p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Belt Levels</h3>
            <dl class="text-sm space-y-1">
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-36 shrink-0">Green Belt</dt><dd class="text-slate-400">Part-time, localized process improvements.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-36 shrink-0">Black Belt</dt><dd class="text-slate-400">Full-time, cross-functional improvement projects.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-36 shrink-0">Master Black Belt</dt><dd class="text-slate-400">Strategic deployment; coaches Black Belts.</dd></div>
            </dl>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Define Phase</h3>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li><span class="font-medium text-slate-100">Project Charter</span> — problem statement, goal statement, scope, team roles, timeline, and business case.</li>
              <li><span class="font-medium text-slate-100">Voice of Customer / CTQ</span> — captures customer needs and maps each to a measurable Critical to Quality characteristic with lower/upper specification limits. Entries derive the CTQ tree used in the project charter.</li>
              <li><span class="font-medium text-slate-100">SIPOC</span> — high-level process map: Suppliers &rarr; Inputs &rarr; Process &rarr; Outputs &rarr; Customers. Defines process boundaries (start trigger, end trigger).</li>
            </ul>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Measure Phase</h3>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li><span class="font-medium text-slate-100">Voice of Customer (VoC)</span> — customer needs with CTQ, specifications (lower/upper limits), measurement method, data collection plan, and priority.</li>
              <li><span class="font-medium text-slate-100">Control Chart</span> — baseline process signature before any improvement changes.</li>
            </ul>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Analyze Phase</h3>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li><span class="font-medium text-slate-100">Fishbone Diagram</span> — structured root-cause analysis across cause categories.</li>
              <li><span class="font-medium text-slate-100">5 Whys</span> — iterative question technique stored per cause branch in the Fishbone structure. Drills to the root cause.</li>
              <li><span class="font-medium text-slate-100">Pareto Chart</span> — ranks defect causes by frequency or cost to identify the vital few (80/20).</li>
            </ul>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Improve Phase</h3>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li><span class="font-medium text-slate-100">Solution Matrix</span> — evaluates candidate solutions on Impact (1-10), Effort (1-10), Risk (1-10), and Cost. Statuses: Proposed, Pilot, Implemented.</li>
            </ul>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Control Phase</h3>
            <ul class="text-sm text-slate-300 space-y-1 ml-3">
              <li><span class="font-medium text-slate-100">Control Plan</span> — per process step: metric, specification limits, measurement method, frequency, owner, and response plan if out of control.</li>
              <li><span class="font-medium text-slate-100">Control Chart (ongoing)</span> — monitor the improved process with post-improvement control limits.</li>
            </ul>
          </section>

        <!-- ── Keyboard Shortcuts & Accessibility ──────────────────────────────── -->
        {:else if active === 'shortcuts'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Keyboard Shortcuts &amp; Accessibility</h2>
          <p class="text-sm text-slate-400 mb-5">
            GoPMgr is fully operable from the keyboard. Shortcuts use Ctrl on Windows/Linux and ⌘ on macOS.
          </p>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Application (File menu)</h3>
            <div class="overflow-x-auto">
              <table class="w-full text-sm border-collapse">
                <thead>
                  <tr class="border-b border-slate-700">
                    <th class="text-left py-1.5 pr-4 font-semibold text-slate-300 w-40">Shortcut</th>
                    <th class="text-left py-1.5 font-semibold text-slate-300">Action</th>
                  </tr>
                </thead>
                <tbody class="text-slate-300">
                  {#each [
                    ['Ctrl/⌘ + N', 'New project (opens the Launchpad)'],
                    ['Ctrl/⌘ + O', 'Open project (project picker)'],
                    ['Ctrl/⌘ + D', 'Portfolio dashboard'],
                    ['Ctrl/⌘ + ,', 'Application settings'],
                    ['Ctrl/⌘ + W', 'Close the current project'],
                    ['Ctrl/⌘ + Q', 'Quit GoPMgr'],
                    ['F11', 'Maximize / restore on Windows and Linux'],
                    ['macOS green control', 'Native Zoom / restore; also available from Window > Zoom'],
                  ] as [keys, action]}
                    <tr class="border-b border-slate-800">
                      <td class="py-1.5 pr-4 font-mono text-xs text-cyan-300 whitespace-nowrap">{keys}</td>
                      <td class="py-1.5">{action}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Editors &amp; diagrams</h3>
            <div class="overflow-x-auto">
              <table class="w-full text-sm border-collapse">
                <thead>
                  <tr class="border-b border-slate-700">
                    <th class="text-left py-1.5 pr-4 font-semibold text-slate-300 w-40">Shortcut</th>
                    <th class="text-left py-1.5 font-semibold text-slate-300">Action</th>
                  </tr>
                </thead>
                <tbody class="text-slate-300">
                  {#each [
                    ['Ctrl/⌘ + S', 'Save the open chart or document editor. Auto-save (App Settings) also saves on an interval.'],
                    ['Tab / Shift+Tab', 'Move between nodes in a diagram (WBS, Network, PERT, CPM, Workflow, Activity, Stakeholder, Cause-and-Effect).'],
                    ['Enter or Space', 'Select the focused diagram node; edit its fields in the side panel.'],
                    ['Enter', 'In the signature dialog password field: confirm the export.'],
                  ] as [keys, action]}
                    <tr class="border-b border-slate-800">
                      <td class="py-1.5 pr-4 font-mono text-xs text-cyan-300 whitespace-nowrap">{keys}</td>
                      <td class="py-1.5">{action}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Dialogs &amp; wizards</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li><span class="font-mono text-xs text-cyan-300">Esc</span> closes any dialog from anywhere inside it (for example the export Signature Options), cancelling the action.</li>
              <li><span class="font-mono text-xs text-cyan-300">Tab</span> cycles only through the dialog's own controls while it is open, and focus returns to the control that opened it when it closes.</li>
              <li>The New Project wizard moves focus to each step's heading as you advance, so keyboard and screen-reader users always know where they are.</li>
            </ul>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Assistive technology</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li>Every navigation announces the destination view ("Gantt chart", "Portfolio") to screen readers.</li>
              <li>Save confirmations, loading states, and errors are announced as they happen via live regions.</li>
              <li>Rendered charts expose text descriptions (kind, title, series); Gantt bars have hover tooltips with day ranges, % complete, and critical-path status.</li>
              <li>Animations are disabled automatically when your OS requests reduced motion; a light theme is available in <button onclick={() => nav('app-settings')} class="text-cyan-400 underline hover:text-cyan-300">App Settings</button>.</li>
            </ul>
          </section>

        <!-- ── Glossary ──────────────────────────────── -->
        {:else if active === 'glossary'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Glossary</h2>
          <p class="text-sm text-slate-400 mb-5">Definitions for project management terms, methodology-specific vocabulary, and GoPMgr-specific concepts.</p>

          <div class="space-y-3 text-sm">
            {#each [
              { term: 'Accountability', def: 'Ultimate ownership of an outcome. In RACI, the "A" — one person owns the result even if others do the work.' },
              { term: 'Agile', def: 'Umbrella term for iterative delivery methodologies valuing customer collaboration and responding to change. Scrum, Kanban, and Scrumban are Agile frameworks.' },
              { term: 'Backlog', def: 'Ordered list of work items waiting to be pulled into active work. In Scrum: the Product Backlog. In Scrumban: maintained continuously.' },
              { term: 'Baseline', def: 'The approved, time-phased plan for scope, schedule, or cost against which performance is measured. Changes require formal change control.' },
              { term: 'Belt Level', def: 'Six Sigma practitioner certification: Green Belt (part-time, local projects), Black Belt (full-time, cross-functional), Master Black Belt (strategic deployment).' },
              { term: 'Burn-Down Chart', def: 'Remaining work (y) over time (x) with an ideal trajectory. Used in Scrum to track sprint progress.' },
              { term: 'Burn-Up Chart', def: 'Cumulative scope completed vs. total scope. Distinguishes scope growth from delivery progress.' },
              { term: 'Business Case', def: 'Document justifying the project investment: costs, benefits, risks, NPV/ROI. Primary go/no-go input.' },
              { term: 'Card / Task (Kanban)', def: 'A single unit of work on the Kanban Board.' },
              { term: 'Change Failure Rate', def: 'DORA metric: the share of deployments that cause a failure in production. Computed on the DORA dashboard from the deployments you record.' },
              { term: 'Change Request', def: 'Formal proposal to modify a project baseline (scope, schedule, cost, quality). Requires review and approval.' },
              { term: 'Compliance Mode', def: 'Project Settings option that verifies the tamper-evident audit chain before a project opens and blocks the open if the chain has been altered.' },
              { term: 'Control Chart', def: 'Time series with UCL and LCL. Points outside limits or non-random patterns indicate special-cause variation.' },
              { term: 'Control Limits', def: 'Statistical boundaries on a Control Chart (±3σ from the process mean). Not the same as specification limits.' },
              { term: 'CPM', def: 'Critical Path Method. Network scheduling identifying the longest dependency chain. Critical path tasks have zero float.' },
              { term: 'Critical Path', def: 'Sequence of tasks from start to end with zero total float. Any delay to a critical path task delays the project end date.' },
              { term: 'CTQ', def: 'Critical to Quality. Measurable characteristic directly affecting customer perception of quality. Derived from VoC in Six Sigma.' },
              { term: 'Cumulative Flow Diagram', def: 'Stacked area chart of work items in each workflow state over time. Primary Kanban health indicator.' },
              { term: 'Cycle Time', def: 'Time from when a work item starts to when it is completed. Key delivery predictability metric in Kanban.' },
              { term: 'DEK', def: 'Data Encryption Key. The per-user key that encrypts project databases. It is stored only in wrapped (encrypted) form — unlocked by your passphrase or a recovery code, never persisted raw.' },
              { term: 'Deployment Frequency', def: 'DORA metric: how often the team ships to production. Recorded per deployment on the DORA dashboard.' },
              { term: 'DMAIC', def: 'Six Sigma improvement cycle: Define → Measure → Analyze → Improve → Control. For improving existing processes.' },
              { term: 'DORA Metrics', def: 'Four DevOps metrics: Deployment Frequency, Lead Time for Changes, Change Failure Rate, Mean Time to Restore. Available in GoPMgr\'s DORA dashboard.' },
              { term: 'Earned Value (EVM)', def: 'Progress accounting that compares work performed against plan and cost. SPI (Schedule Performance Index) and CPI (Cost Performance Index) read 1.0 as on-plan; below 1.0 means behind schedule / over cost. Surfaces on the Dashboard and in combined reports with a linked CPM schedule.' },
              { term: 'Epic', def: 'A large user story spanning multiple sprints, broken down into smaller stories before entering a sprint.' },
              { term: 'ES / EF', def: 'Earliest Start / Earliest Finish. Calculated during the forward pass through a CPM network.' },
              { term: 'Estimate (methodology-specific)', def: 'Scrum: Story Points. Kanban: Time Estimate. CPM: Duration. Waterfall: Duration. Lean: Effort. PRINCE2: Work Package Estimate. Six Sigma: Resource Plan.' },
              { term: 'Fishbone Diagram', def: 'Also Ishikawa Diagram. Root-cause analysis: effect at the "head"; default cause categories (People, Process, Equipment, Materials, Environment, Measurement) are the "bones." Categories are editable.' },
              { term: 'Float (Slack)', def: 'Amount of time a task can be delayed without delaying the project end date (total float) or a successor\'s start (free float).' },
              { term: 'Gantt Chart', def: 'Schedule bars on a time axis with dependency arrows, critical-path highlighting, and progress bars. Shares data with CPM in GoPMgr.' },
              { term: 'Issue Log', def: 'Tracks problems that have already occurred, with owner, priority, and resolution plan. Distinct from Risk Register (potential future events).' },
              { term: 'Iteration', def: 'Time-boxed work cycle. Scrum: Sprint. PRINCE2: Management Stage. Lean: Flow Cycle. Scrumban: continuous (no fixed iteration).' },
              { term: 'Kaizen', def: 'Lean concept of continuous, incremental improvement made frequently by everyone involved in the process.' },
              { term: 'Kanban Board', def: 'Visual work management: columns = workflow states; cards = work items. WIP limits cap items per column.' },
              { term: 'Key Result (KR)', def: 'OKRs: measurable, time-bound outcome tracking progress toward an Objective. Graded 0.0–1.0 at period close.' },
              { term: 'Launchpad', def: 'GoPMgr\'s project creation wizard. Guides through industry, methodology, and project name selection; seeds starter artifacts.' },
              { term: 'Lead Time', def: 'Total time from request to delivery. Includes queue time + cycle time.' },
              { term: 'LS / LF', def: 'Latest Start / Latest Finish. Calculated during the backward pass through a CPM network.' },
              { term: 'Milestone', def: 'Significant project event with no duration. Scrum: Definition of Done. Kanban: Throughput Target. CPM: Schedule Milestone.' },
              { term: 'Monte Carlo Simulation', def: 'Schedule-risk technique that samples uncertain task durations thousands of times to estimate finish-date probabilities. Run from the CPM editor; results include P50/P80/P90 dates, an S-curve, and a tornado driver ranking.' },
              { term: 'MSPDI', def: 'Microsoft Project Data Interchange — the .xml schedule format GoPMgr imports and exports. Resave .mpp/.pod/.mpx files as MS Project XML before importing.' },
              { term: 'MTTR', def: 'Mean Time to Restore. DORA metric: average hours to recover from a failed deployment.' },
              { term: 'Objective', def: 'OKRs: qualitative, inspiring goal statement answering "where do we want to go?" Supported by measurable Key Results.' },
              { term: 'OKRs', def: 'Objectives and Key Results. Goal-setting framework aligning teams to strategic outcomes through measurable, time-bound Key Results.' },
              { term: 'P50 / P80 / P90', def: 'Confidence points from a Monte Carlo run: the finish day the project beats in 50%, 80%, or 90% of simulations. Commit to P80/P90 dates when the cost of being late is high.' },
              { term: 'PAdES', def: 'PDF Advanced Electronic Signatures. Standard for embedding cryptographic signatures into PDF files. GoPMgr generates PAdES-compliant signed exports.' },
              { term: 'Pareto Chart', def: 'Bar chart sorted descending with a cumulative-percentage line. Based on the 80/20 principle: ~80% of effects come from ~20% of causes.' },
              { term: 'PERT', def: 'Program Evaluation and Review Technique. Network scheduling with three-point duration estimates per activity.' },
              { term: 'Planning Meeting', def: 'Scrum: Sprint Planning. Kanban: Replenishment Meeting. Lean: Plan-Do-Check-Act review. PRINCE2: Stage Planning Meeting.' },
              { term: 'PMBOK', def: 'Project Management Body of Knowledge. PMI\'s comprehensive PM framework. A knowledge standard, not a delivery methodology.' },
              { term: 'PRINCE2', def: 'Projects In Controlled Environments. Process-based framework with seven principles, themes, and processes. Strong governance and stage-gate controls.' },
              { term: 'Process Group', def: 'PMBOK grouping of related PM processes: Initiating, Planning, Executing, Monitoring & Controlling, Closing. Corresponds to GoPMgr\'s document Phase categories.' },
              { term: 'Project Charter', def: 'Foundational document formally authorizing a project. Captures purpose, objectives, scope, sponsor, high-level schedule and budget.' },
              { term: 'Pull System', def: 'Work produced when downstream capacity exists, not pushed by a schedule. Core Lean and Kanban principle.' },
              { term: 'RACI Matrix', def: 'Responsibility assignment: Responsible (does work), Accountable (owns outcome), Consulted (input needed), Informed (kept updated). One A per task.' },
              { term: 'Recovery Codes', def: 'GoPMgr one-time codes that allow passphrase reset from the login screen without the current passphrase. Generate from App Settings. Required before enabling database encryption.' },
              { term: 'Resource Leveling', def: 'Delaying contended tasks so resource demand fits capacity. Uses the named resource calendars (weekly capacity + day overrides) from Project Settings.' },
              { term: 'Retrospective', def: 'Process improvement ceremony. Scrum: Sprint Retrospective. Kanban: Retrospective. PRINCE2: End-Stage Assessment lessons.' },
              { term: 'Risk Register', def: 'Catalogue of potential future risks with probability, impact, owner, mitigation strategy, and contingency plan.' },
              { term: 'Scenario', def: 'An isolated what-if partition. Charts copied into a scenario can be edited and compared against their captured baseline without touching the live plan, then promoted to a named schedule baseline or discarded.' },
              { term: 'Scrum', def: 'Agile framework with time-boxed sprints, Product Backlog, and three roles. Four ceremonies: Sprint Planning, Daily Scrum, Sprint Review, Retrospective.' },
              { term: 'Scrumban', def: 'Hybrid of Scrum\'s prioritized backlog and Kanban\'s continuous pull flow. No fixed sprint cadence.' },
              { term: 'Sigma Level', def: 'Process quality metric. 6 sigma = 3.4 DPMO. Use Control Charts and Pareto Charts to track improvement.' },
              { term: 'SIPOC', def: 'High-level process map: Suppliers → Inputs → Process → Outputs → Customers. Defines process boundaries in DMAIC Define phase.' },
              { term: 'Solution Matrix', def: 'Six Sigma Improve tool. Evaluates solutions on Impact, Effort, Risk, and Cost. Statuses: Proposed, Pilot, Implemented.' },
              { term: 'Sprint', def: 'Scrum time-boxed iteration (typically 2 weeks). Contains a Sprint Goal, committed backlog items, Review, and Retrospective.' },
              { term: 'SQLCipher', def: 'Open-source encrypted SQLite extension used by GoPMgr for database encryption at rest. Applies AES-256 to the entire project database file.' },
              { term: 'Stakeholder', def: 'Individual or group with an interest in or influence over the project. Mapped by power/interest in the Stakeholder Analysis Matrix.' },
              { term: 'Story (Scrum)', def: 'User-facing work item: "As a [role], I want [feature] so that [benefit]." Equivalent to a Task in most other methodologies.' },
              { term: 'Story Points', def: 'Relative effort estimate in Scrum. Team-defined scale (typically Fibonacci). Not correlated to hours.' },
              { term: 'SWOT Matrix', def: '2x2 strategic analysis: Strengths, Weaknesses (internal) vs. Opportunities, Threats (external).' },
              { term: 'Throughput', def: 'Work items completed per unit time. Primary Kanban flow metric alongside cycle time.' },
              { term: 'Timeline', def: 'GoPMgr view showing the project\'s chronological event stream as an SVG strip: sprint bands, milestones, and holiday markers.' },
              { term: 'Tornado Ranking', def: 'Monte Carlo output ranking which tasks drive schedule risk, combining how often each lands on the critical path with its P90−P50 duration spread. Fix the top bars first.' },
              { term: 'User Story', def: 'See Story (Scrum).' },
              { term: 'Velocity', def: 'Average story points completed per sprint. Used for capacity planning and release forecasting.' },
              { term: 'VoC', def: 'Voice of Customer. Customer needs captured via surveys, interviews, or complaints. Input to CTQ derivation in Six Sigma.' },
              { term: 'WBS', def: 'Work Breakdown Structure. Hierarchical decomposition of total project scope into deliverables and work packages.' },
              { term: 'WIP Limit', def: 'Work-In-Progress Limit. Maximum items in a Kanban column simultaneously. Prevents overloading and surfaces bottlenecks.' },
            ] as entry}
              <div class="border-b border-slate-800 pb-3">
                <dt class="font-medium text-slate-100">{entry.term}</dt>
                <dd class="text-slate-400 mt-0.5">{entry.def}</dd>
              </div>
            {/each}
          </div>

        <!-- ── Installing & Running ──────────────────────────────── -->
        {:else if active === 'install'}
          <h2 class="text-xl font-bold text-slate-100 mb-2">Installing &amp; Running GoPMgr</h2>
          <p class="text-sm text-slate-400 mb-5">
            GoPMgr ships as a native installer for each platform. Download the
            file for your operating system from the project's Releases page and
            follow the steps below. The same guide lives in
            <code class="text-cyan-300">docs/INSTALL.md</code>.
          </p>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Which file to download</h3>
            <ul class="text-sm text-slate-300 space-y-1 list-disc pl-5">
              <li><strong>Windows</strong> — <code>GoPMgr-…-amd64-setup.exe</code> (installer)</li>
              <li><strong>macOS (Apple Silicon)</strong> — <code>GoPMgr-…-arm64.dmg</code></li>
              <li><strong>Debian / Ubuntu</strong> — <code>gopmgr-…-amd64.deb</code></li>
              <li><strong>Fedora / RHEL / openSUSE</strong> — <code>gopmgr-…-x86_64.rpm</code></li>
            </ul>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Install steps</h3>
            <ul class="text-sm text-slate-300 space-y-2 list-disc pl-5">
              <li><strong>Windows:</strong> double-click the <code>.exe</code> and follow the installer. Current builds are unsigned, so SmartScreen may warn — choose <em>More info → Run anyway</em>.</li>
              <li><strong>macOS:</strong> open the <code>.dmg</code> and drag <strong>GoPMgr</strong> to Applications. Unsigned builds trigger Gatekeeper — right-click the app then <em>Open</em> (or System Settings → Privacy &amp; Security → <em>Open Anyway</em>).</li>
              <li><strong>.deb:</strong> <code>sudo apt install ./gopmgr-*.deb</code></li>
              <li><strong>.rpm:</strong> <code>sudo dnf install ./gopmgr-*.rpm</code></li>
            </ul>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Run from source</h3>
            <p class="text-sm text-slate-300 mb-2">
              Requires Go, Node, and the Wails CLI (plus GTK/WebKit dev packages
              on Linux):
            </p>
            <ul class="text-sm text-slate-300 space-y-1 list-disc pl-5">
              <li>In <code>frontend/</code>, run <code>npm ci</code> — use <code>npm ci</code>, <strong>not</strong> <code>npm install</code>.</li>
              <li>On Ubuntu 24.04+ Linux hosts, install <code>libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config</code>. GoPMgr builds with the Wails <code>webkit2_41</code> tag; GTK4/WebKitGTK 6.0 support requires a future Wails migration.</li>
              <li><code>make build</code> — produce the desktop binary/app with DuckDB analytics and the current Linux WebKit tag.</li>
              <li><code>make dev</code> — hot-reload development mode.</li>
            </ul>
            <p class="text-xs text-slate-500 mt-2">
              Full prerequisites, the per-format packaging commands, and signing
              notes are in <code class="text-cyan-300">docs/INSTALL.md</code>.
            </p>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Your data stays local</h3>
            <p class="text-sm text-slate-300">
              However you install GoPMgr, every project lives in an encrypted
              file on your own machine — no account, cloud, or network is
              required. See <em>Database Encryption</em> for details.
            </p>
          </section>

        {/if}
