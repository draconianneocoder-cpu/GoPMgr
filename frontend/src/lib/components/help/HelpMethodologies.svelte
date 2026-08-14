<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import type { SectionId } from '../help-sections';

  let { active, nav }: { active: SectionId; nav: (id: SectionId) => void } = $props();
</script>

        <!-- ── Scrum ──────────────────────────────── -->
        {#if active === 'scrum'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Scrum</h2>
          <p class="text-sm text-slate-400 mb-5">Iterative agile framework using time-boxed sprints with defined roles, events, and artifacts.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">When to Use</h3>
            <p class="text-sm text-slate-300">Software development and product teams where requirements evolve frequently. Best when stakeholders can commit to a regular feedback cadence and the team has 3-9 members working on a single product.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">GoPMgr Setup</h3>
            <p class="text-sm text-slate-300">Launchpad: Software &rarr; Scrum. Seeds: Kanban Board, Project Charter, Agile Backlog, Sprint 1. The Kanban Board is the daily tracking surface. The Backlog holds the ordered list of user stories. Sprint 1 is a pre-created sprint container ready for backlog items.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Workflow</h3>
            <ol class="space-y-2 text-sm text-slate-300 list-decimal list-inside">
              <li><span class="font-medium text-slate-100">Populate the Backlog.</span> Add user stories with story-point estimates and priority.</li>
              <li><span class="font-medium text-slate-100">Sprint Planning.</span> Pull stories from Backlog into a sprint. Define the Sprint Goal.</li>
              <li><span class="font-medium text-slate-100">Daily execution.</span> Move stories across Kanban Board columns (To Do &rarr; In Progress &rarr; Done). Limit WIP.</li>
              <li><span class="font-medium text-slate-100">Sprint Review.</span> Mark stories done. Unfinished stories return to Backlog.</li>
              <li><span class="font-medium text-slate-100">Retrospective.</span> Record improvements. Create the next sprint.</li>
            </ol>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Recommended Charts</h3>
            <ul class="text-sm text-slate-300 space-y-1">
              <li><span class="font-medium text-slate-100">Burn-Down Chart</span> — remaining story points per sprint day vs. ideal trajectory.</li>
              <li><span class="font-medium text-slate-100">Burn-Up Chart</span> — cumulative scope completed vs. total scope; useful when scope changes.</li>
              <li><span class="font-medium text-slate-100">Cumulative Flow Diagram</span> — WIP and flow health over time.</li>
              <li><span class="font-medium text-slate-100">DORA Metrics</span> — deployment frequency, lead time, change failure rate, mean time to restore.</li>
            </ul>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Key Terminology</h3>
            <dl class="text-sm space-y-1">
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-32 shrink-0">Story / Task</dt><dd class="text-slate-400">User-facing work item with acceptance criteria.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-32 shrink-0">Story Points</dt><dd class="text-slate-400">Relative effort estimate; team-calibrated, not hours.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-32 shrink-0">Sprint</dt><dd class="text-slate-400">Time-boxed iteration (typically 2 weeks).</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-32 shrink-0">Velocity</dt><dd class="text-slate-400">Story points completed per sprint; used for capacity planning.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-32 shrink-0">DoD</dt><dd class="text-slate-400">Definition of Done — shared criteria that make a story complete.</dd></div>
            </dl>
          </section>

        <!-- ── Kanban ──────────────────────────────── -->
        {:else if active === 'kanban'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Kanban</h2>
          <p class="text-sm text-slate-400 mb-5">Visual pull-system for continuous flow. Cards move through columns representing workflow states; WIP limits prevent overloading any stage.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">When to Use</h3>
            <p class="text-sm text-slate-300">Continuous delivery, support queues, maintenance, or any context with unpredictable demand and variable item sizes. No fixed iteration length.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">GoPMgr Setup</h3>
            <p class="text-sm text-slate-300">Launchpad: Software &rarr; Kanban. Seeds: Kanban Board, Project Charter, Agile Backlog. Columns represent workflow states (e.g., Backlog, In Progress, Review, Done).</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Workflow</h3>
            <ol class="space-y-2 text-sm text-slate-300 list-decimal list-inside">
              <li><span class="font-medium text-slate-100">Design board columns</span> to match your actual workflow states.</li>
              <li><span class="font-medium text-slate-100">Set WIP limits per column.</span> Forces team to finish before starting new work, exposing bottlenecks.</li>
              <li><span class="font-medium text-slate-100">Pull cards forward</span> only when downstream capacity exists.</li>
              <li><span class="font-medium text-slate-100">Monitor flow metrics</span> via Cumulative Flow Diagram.</li>
              <li><span class="font-medium text-slate-100">Replenishment meeting</span> (periodic) — review priorities and add new cards.</li>
            </ol>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Recommended Charts</h3>
            <ul class="text-sm text-slate-300 space-y-1">
              <li><span class="font-medium text-slate-100">Cumulative Flow Diagram</span> — primary Kanban health indicator; widening bands signal WIP growth.</li>
              <li><span class="font-medium text-slate-100">Pareto Chart</span> — identify categories consuming the most cycle time.</li>
            </ul>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Key Terminology</h3>
            <dl class="text-sm space-y-1">
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-36 shrink-0">Card / Task</dt><dd class="text-slate-400">Single unit of work; one request or feature.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-36 shrink-0">WIP Limit</dt><dd class="text-slate-400">Maximum cards in a column simultaneously.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-36 shrink-0">Cycle Time</dt><dd class="text-slate-400">Time from card start to done.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-36 shrink-0">Throughput</dt><dd class="text-slate-400">Cards completed per unit time (week/month).</dd></div>
            </dl>
          </section>

        <!-- ── Scrumban ──────────────────────────────── -->
        {:else if active === 'scrumban'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Scrumban</h2>
          <p class="text-sm text-slate-400 mb-5">Hybrid of Scrum's prioritized backlog and Kanban's continuous pull flow. No fixed sprint boundaries.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">When to Use</h3>
            <p class="text-sm text-slate-300">Teams transitioning from Scrum to Kanban, or teams with a mix of planned feature work and unplanned support/maintenance. Provides backlog discipline without rigid sprint ceremonies.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">GoPMgr Setup</h3>
            <p class="text-sm text-slate-300">Launchpad: Software &rarr; Scrumban. Seeds: Kanban Board, Project Charter, Agile Backlog. The Backlog provides priority ordering; the Kanban Board drives daily flow. No sprint containers — work is pulled continuously.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Workflow</h3>
            <ol class="space-y-2 text-sm text-slate-300 list-decimal list-inside">
              <li><span class="font-medium text-slate-100">Maintain a prioritized Backlog.</span> Order by business value; estimates are optional.</li>
              <li><span class="font-medium text-slate-100">Pull from Backlog when capacity opens.</span> When a Kanban column falls below WIP limit, pull the top-priority item.</li>
              <li><span class="font-medium text-slate-100">Replenish periodically.</span> Review and reorder backlog on a cadence rather than at sprint boundaries.</li>
              <li><span class="font-medium text-slate-100">Optional timeboxed reviews</span> for stakeholder alignment without full sprint commitment.</li>
            </ol>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Recommended Charts</h3>
            <ul class="text-sm text-slate-300 space-y-1">
              <li><span class="font-medium text-slate-100">Cumulative Flow Diagram</span> — primary flow health indicator.</li>
              <li><span class="font-medium text-slate-100">Burn-Down Chart</span> — optional; useful for time-bounded work even without formal sprints.</li>
            </ul>
          </section>

        <!-- ── Lean ──────────────────────────────── -->
        {:else if active === 'lean'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Lean</h2>
          <p class="text-sm text-slate-400 mb-5">Maximize customer value while eliminating waste. Originated in manufacturing (Toyota Production System); applied across construction, services, and business operations.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">When to Use</h3>
            <p class="text-sm text-slate-300">Process improvement projects focused on efficiency — Construction (reducing rework, material waste), Business operations (reducing approval delays, handoff waste), service delivery.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">GoPMgr Setup</h3>
            <p class="text-sm text-slate-300">Construction &rarr; Lean seeds: WBS, Cumulative Flow Diagram, Risk Register. Business &rarr; Lean seeds: Pareto Chart, Cumulative Flow Diagram, SWOT Matrix. The Pareto Chart identifies the vital few waste sources (80/20 rule); the Cumulative Flow Diagram tracks process throughput.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Workflow</h3>
            <ol class="space-y-2 text-sm text-slate-300 list-decimal list-inside">
              <li><span class="font-medium text-slate-100">Define value</span> from the customer's perspective. Non-value-adding steps are waste.</li>
              <li><span class="font-medium text-slate-100">Map the value stream.</span> Document every step. Use a Workflow Diagram.</li>
              <li><span class="font-medium text-slate-100">Identify waste</span> (overproduction, waiting, transport, over-processing, inventory, motion, defects). Rank with a Pareto Chart.</li>
              <li><span class="font-medium text-slate-100">Create flow.</span> Remove waste; redesign so value-adding steps flow without interruption.</li>
              <li><span class="font-medium text-slate-100">Establish pull.</span> Produce only what downstream demands.</li>
              <li><span class="font-medium text-slate-100">Pursue perfection (Kaizen).</span> Repeat the cycle continuously.</li>
            </ol>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Recommended Charts</h3>
            <ul class="text-sm text-slate-300 space-y-1">
              <li><span class="font-medium text-slate-100">Pareto Chart</span> — rank waste categories to focus improvement effort.</li>
              <li><span class="font-medium text-slate-100">Line Chart</span> — track a KPI (cycle time, defect rate) over time.</li>
              <li><span class="font-medium text-slate-100">Workflow Diagram</span> — current-state and future-state process maps.</li>
            </ul>
          </section>

        <!-- ── OKRs ──────────────────────────────── -->
        {:else if active === 'okrs'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">OKRs</h2>
          <p class="text-sm text-slate-400 mb-5">Objectives and Key Results: a goal-setting framework that aligns teams to strategic outcomes through measurable, time-bound targets.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">When to Use</h3>
            <p class="text-sm text-slate-300">Strategic planning cycles (quarterly or annually), cross-functional alignment initiatives, or any Business project where the challenge is agreement on what success looks like.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">GoPMgr Setup</h3>
            <p class="text-sm text-slate-300">Launchpad: Business &rarr; OKRs. Seeds: Project Plan (Word), Stakeholder Analysis Document, Status Report. The Stakeholder Analysis Document maps who influences which Objectives; the Status Report provides the check-in template for KR progress updates.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Structure</h3>
            <div class="bg-slate-900 rounded p-3 text-sm text-slate-300 mb-3">
              <p class="font-medium text-slate-100 mb-1">Objective — qualitative, inspiring direction.</p>
              <div class="pl-4 border-l border-slate-700 space-y-1">
                <p class="text-xs text-slate-400">Key Result 1 — measurable, time-bound. Graded 0.0–1.0 at period close.</p>
                <p class="text-xs text-slate-400">Key Result 2 — each KR has a numeric target. 3-5 KRs per Objective is typical.</p>
              </div>
            </div>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Workflow</h3>
            <ol class="space-y-2 text-sm text-slate-300 list-decimal list-inside">
              <li><span class="font-medium text-slate-100">Draft Objectives</span> at organization or team level. Ambitious but achievable.</li>
              <li><span class="font-medium text-slate-100">Define Key Results</span> — 3-5 measurable outcomes per Objective, each with baseline and target.</li>
              <li><span class="font-medium text-slate-100">Assign ownership</span> for each KR. Document in Charter or Status Reports.</li>
              <li><span class="font-medium text-slate-100">Check-ins</span> (weekly or bi-weekly) — update KR progress in Status Reports.</li>
              <li><span class="font-medium text-slate-100">Grade at period close</span> — score each KR 0.0–1.0. A score of 0.7 is typical target; 1.0 may mean the bar was set too low.</li>
            </ol>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Recommended Charts &amp; Documents</h3>
            <ul class="text-sm text-slate-300 space-y-1">
              <li><span class="font-medium text-slate-100">Stakeholder Analysis Matrix</span> — seeded; map stakeholder power/interest per Objective.</li>
              <li><span class="font-medium text-slate-100">Bar Chart</span> — compare KR scores at period close.</li>
              <li><span class="font-medium text-slate-100">Line Chart</span> — track KR progress trend during the measurement period.</li>
              <li><span class="font-medium text-slate-100">Status Report</span> — periodic written check-in against each KR target.</li>
            </ul>
          </section>

        <!-- ── Waterfall ──────────────────────────────── -->
        {:else if active === 'waterfall'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Waterfall</h2>
          <p class="text-sm text-slate-400 mb-5">Sequential, phase-gated delivery. Each phase completes fully before the next begins. Scope, schedule, and budget are defined upfront.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">When to Use</h3>
            <p class="text-sm text-slate-300">Well-defined, stable scope. Construction, infrastructure, compliance-driven projects, regulated environments, or any project where mid-stream rework is prohibitively expensive.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">GoPMgr Setup</h3>
            <p class="text-sm text-slate-300">Construction &rarr; Waterfall seeds: WBS, Statement of Work, Risk Register, CPM Chart. Administration &rarr; Waterfall seeds: Project Charter, Scope Statement, Risk Register, Communication Plan. Extend with Budget, Team Charter, and RACI Matrix as planning matures.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Phases &amp; Key Artifacts</h3>
            <div class="space-y-2 text-sm">
              <div class="flex gap-3"><span class="w-28 shrink-0 font-medium text-slate-100">Initiation</span><span class="text-slate-300">Charter, Business Case, Proposal, Stakeholder Analysis</span></div>
              <div class="flex gap-3"><span class="w-28 shrink-0 font-medium text-slate-100">Planning</span><span class="text-slate-300">Project Plan, WBS, Gantt/CPM Schedule, Risk Register, Budget, Communication Plan, Scope Statement</span></div>
              <div class="flex gap-3"><span class="w-28 shrink-0 font-medium text-slate-100">Execution</span><span class="text-slate-300">Project Brief, Project Overview, Status Reports, Issue Log, Change Requests</span></div>
              <div class="flex gap-3"><span class="w-28 shrink-0 font-medium text-slate-100">Monitoring</span><span class="text-slate-300">Status Reports, Variance Analysis, Issue Log updates</span></div>
              <div class="flex gap-3"><span class="w-28 shrink-0 font-medium text-slate-100">Closing</span><span class="text-slate-300">Project Closure, lessons learned</span></div>
            </div>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Recommended Charts</h3>
            <ul class="text-sm text-slate-300 space-y-1">
              <li><span class="font-medium text-slate-100">Gantt Chart</span> — primary schedule view; dependencies and critical path.</li>
              <li><span class="font-medium text-slate-100">WBS</span> — hierarchical scope decomposition.</li>
              <li><span class="font-medium text-slate-100">RACI Matrix</span> — responsibility across all work packages.</li>
              <li><span class="font-medium text-slate-100">Network Diagram</span> — visualize task dependencies before building the Gantt.</li>
            </ul>
          </section>

        <!-- ── PRINCE2 ──────────────────────────────── -->
        {:else if active === 'prince2'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">PRINCE2</h2>
          <p class="text-sm text-slate-400 mb-5">Projects In Controlled Environments. A process-based framework with seven principles, seven themes, and seven processes. Strong governance and stage-gate controls.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">When to Use</h3>
            <p class="text-sm text-slate-300">Government or public-sector projects, formal multi-organization initiatives, or wherever an auditable trail of decisions and approvals is required. Common in UK, European, and Commonwealth environments.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">GoPMgr Setup</h3>
            <p class="text-sm text-slate-300">Launchpad: Administration &rarr; PRINCE2. Seeds: Project Charter (approximates PID), Project Plan (Word), Risk Register. Extend with Communication Plan and Team Charter.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Processes in GoPMgr</h3>
            <div class="space-y-2 text-sm text-slate-300">
              <div class="flex gap-3"><span class="w-44 shrink-0 font-medium text-slate-100">Starting Up (SU)</span><span>Project Brief — use Charter. Appoint Project Board and PM roles.</span></div>
              <div class="flex gap-3"><span class="w-44 shrink-0 font-medium text-slate-100">Initiating (IP)</span><span>PID = Charter + Project Plan + Risk Register + Communication Plan.</span></div>
              <div class="flex gap-3"><span class="w-44 shrink-0 font-medium text-slate-100">Directing (DP)</span><span>Project Board uses Status Reports and Change Requests for decision-point documents.</span></div>
              <div class="flex gap-3"><span class="w-44 shrink-0 font-medium text-slate-100">Controlling (CS)</span><span>Team Manager tracks work packages via Issue Log and Status Reports.</span></div>
              <div class="flex gap-3"><span class="w-44 shrink-0 font-medium text-slate-100">Managing Delivery (MP)</span><span>Work Package execution; Project Overview for acceptance snapshots.</span></div>
              <div class="flex gap-3"><span class="w-44 shrink-0 font-medium text-slate-100">Closing (CP)</span><span>Project Closure document; follow-on action recommendations in notes field.</span></div>
            </div>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Recommended Charts</h3>
            <ul class="text-sm text-slate-300 space-y-1">
              <li><span class="font-medium text-slate-100">Gantt Chart</span> — stage and work-package schedule.</li>
              <li><span class="font-medium text-slate-100">WBS</span> — deliverable hierarchy by management stage.</li>
              <li><span class="font-medium text-slate-100">RACI Matrix</span> — responsibility across Project Board, PM, and Team Managers.</li>
            </ul>
          </section>

        <!-- ── PMBOK ──────────────────────────────── -->
        {:else if active === 'pmbok'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">PMBOK</h2>
          <p class="text-sm text-slate-400 mb-5">Project Management Body of Knowledge. PMI's comprehensive framework of processes, knowledge areas, and best practices. A knowledge standard — paired with a delivery approach (Waterfall, CPM, Agile, etc.) rather than used alone.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Five Process Groups</h3>
            <p class="text-sm text-slate-300 mb-3">PMBOK's five process groups correspond directly to GoPMgr's document Phase categories. Browsing documents by phase is browsing by PMBOK process group.</p>
            <div class="space-y-2 text-sm">
              <div class="flex gap-3"><span class="w-36 shrink-0 font-medium text-slate-100">Initiating</span><span class="text-slate-300">Charter, Business Case, Stakeholder Analysis. 5 GoPMgr documents.</span></div>
              <div class="flex gap-3"><span class="w-36 shrink-0 font-medium text-slate-100">Planning</span><span class="text-slate-300">Define scope, schedule, cost, quality, risk, comms, procurement. 14 GoPMgr documents.</span></div>
              <div class="flex gap-3"><span class="w-36 shrink-0 font-medium text-slate-100">Executing</span><span class="text-slate-300">Carry out the plan. Project Brief, Project Overview. 2 GoPMgr documents.</span></div>
              <div class="flex gap-3"><span class="w-36 shrink-0 font-medium text-slate-100">Monitoring</span><span class="text-slate-300">Track and regulate performance. Status Report, Issue Log, Change Request Form. 3 documents.</span></div>
              <div class="flex gap-3"><span class="w-36 shrink-0 font-medium text-slate-100">Closing</span><span class="text-slate-300">Formally close the project. Project Closure. 1 document.</span></div>
            </div>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Ten Knowledge Areas</h3>
            <div class="grid grid-cols-2 gap-x-6 gap-y-1 text-xs text-slate-300">
              <span>Integration &rarr; Charter, Project Plan</span>
              <span>Scope &rarr; Scope Statement, WBS</span>
              <span>Schedule &rarr; Project Schedule, Gantt, Network, CPM</span>
              <span>Cost &rarr; Project Budget</span>
              <span>Quality &rarr; Control Chart, Pareto, Fishbone</span>
              <span>Resource &rarr; RACI, Team Charter</span>
              <span>Communications &rarr; Communication Plan, Status Report</span>
              <span>Risk &rarr; Risk Register, SWOT Matrix</span>
              <span>Procurement &rarr; Statement of Work, Procurement Plan</span>
              <span>Stakeholder &rarr; Stakeholder Analysis Matrix &amp; Document</span>
            </div>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Using PMBOK in GoPMgr</h3>
            <p class="text-sm text-slate-300">PMBOK does not have a dedicated Launchpad option. Use it as vocabulary and process reference while running a Waterfall, CPM, or PRINCE2 project. The Documents view's phase-based organization mirrors PMBOK's process group structure.</p>
          </section>

        <!-- ── Critical Path (CPM) ──────────────────────────────── -->
        {:else if active === 'cpm'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Critical Path Method (CPM)</h2>
          <p class="text-sm text-slate-400 mb-5">Network-based scheduling that identifies the longest dependency chain through the project. Tasks on the critical path have zero float; any delay directly delays project completion.</p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">When to Use</h3>
            <p class="text-sm text-slate-300">Engineering and construction projects with tightly coupled task dependencies, complex parallel workstreams, or where identifying "must-finish-on-time" tasks is essential.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">GoPMgr Setup</h3>
            <p class="text-sm text-slate-300">Launchpad: Engineering &rarr; CPM. Seeds: CPM Chart, WBS, Risk Register, Project Charter. The CPM Chart displays activity nodes with ES/EF/LS/LF and critical-path highlighting; the WBS structures the deliverable hierarchy; the Risk Register surfaces schedule threats early.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Key Concepts</h3>
            <dl class="text-sm space-y-1">
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-20 shrink-0">ES / EF</dt><dd class="text-slate-400">Earliest Start / Earliest Finish — forward-pass through the network.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-20 shrink-0">LS / LF</dt><dd class="text-slate-400">Latest Start / Latest Finish — backward-pass through the network.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-20 shrink-0">Float</dt><dd class="text-slate-400">LS - ES. Time a task can slip without delaying the project end date.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-20 shrink-0">Critical Path</dt><dd class="text-slate-400">Longest path through the network; all tasks on it have zero float.</dd></div>
            </dl>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Workflow</h3>
            <ol class="space-y-2 text-sm text-slate-300 list-decimal list-inside">
              <li><span class="font-medium text-slate-100">Open Network Diagram.</span> Add each activity as a node. Define predecessor relationships.</li>
              <li><span class="font-medium text-slate-100">Enter durations.</span> The chart displays ES/EF/LS/LF values and highlights the critical path.</li>
              <li><span class="font-medium text-slate-100">Add risk estimates.</span> For uncertain tasks, enter optimistic, likely, and pessimistic durations, then run Monte Carlo from the CPM editor aside to review P50/P80/P90 finish days, the finish-probability S-curve, and tornado risk drivers. Export PDF/A saves the same risk evidence as a shareable report.</li>
              <li><span class="font-medium text-slate-100">Open Gantt Chart</span> to view activities on a time axis with dependency arrows.</li>
              <li><span class="font-medium text-slate-100">Generate Resource Histogram</span> to compare resource demand bars with dashed capacity lines from stakeholder availability and Resource Capacity calendars.</li>
              <li><span class="font-medium text-slate-100">Update actuals</span> as work progresses. Track completion percentages.</li>
              <li><span class="font-medium text-slate-100">Link a Project Schedule document</span> as the official schedule reference.</li>
            </ol>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">PERT vs CPM</h3>
            <p class="text-sm text-slate-300">The PERT chart computes expected duration and variance from three-point estimates. CPM uses deterministic durations for the live schedule, with optional Monte Carlo estimates for probabilistic finish-date analysis. Both use activity-on-node notation.</p>
          </section>

        <!-- ── Six Sigma ──────────────────────────────── -->
        {:else if active === 'six-sigma-method'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Six Sigma</h2>
          <p class="text-sm text-slate-400 mb-5">Data-driven quality improvement targeting 3.4 defects per million opportunities (6&sigma; from the mean). Uses DMAIC for existing processes.</p>

          <p class="text-xs bg-slate-900 border border-slate-700 rounded px-3 py-2 text-slate-400 mb-5">
            Six Sigma exists in GoPMgr in two forms: as a Launchpad methodology (described here, seeds initial charts) and as a dedicated
            <button onclick={() => nav('sigma-pack')} class="text-cyan-400 underline hover:text-cyan-300">DMAIC Pack</button>
            with structured project and dashboard views for the full 5-phase workflow.
          </p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">GoPMgr Launchpad Setup</h3>
            <p class="text-sm text-slate-300">Launchpad: Engineering &rarr; Six Sigma. Seeds: Control Chart, Pareto Chart, Fishbone Diagram. Control Chart establishes the baseline process signature (Measure phase); Pareto ranks defect categories (Analyze phase); Fishbone begins root-cause analysis (Analyze phase). Add a Project Charter manually to complete the Define phase, or use the full <button onclick={() => nav('sigma-pack')} class="text-cyan-400 underline hover:text-cyan-300">DMAIC Pack</button>.</p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">DMAIC at a Glance</h3>
            <div class="space-y-1 text-sm">
              <div class="flex gap-3"><span class="w-20 font-medium text-slate-100">Define</span><span class="text-slate-300">Problem statement, CTQ tree, SIPOC, project charter, team &amp; scope.</span></div>
              <div class="flex gap-3"><span class="w-20 font-medium text-slate-100">Measure</span><span class="text-slate-300">Data collection plan, baseline metrics, process capability, VoC.</span></div>
              <div class="flex gap-3"><span class="w-20 font-medium text-slate-100">Analyze</span><span class="text-slate-300">Root-cause analysis via Fishbone + 5 Whys, Pareto analysis.</span></div>
              <div class="flex gap-3"><span class="w-20 font-medium text-slate-100">Improve</span><span class="text-slate-300">Solution Matrix (impact/effort/risk scoring), pilot testing.</span></div>
              <div class="flex gap-3"><span class="w-20 font-medium text-slate-100">Control</span><span class="text-slate-300">Control Plan, Control Chart monitoring, handover to process owner.</span></div>
            </div>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Belt Levels</h3>
            <dl class="text-sm space-y-1">
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-28 shrink-0">Green Belt</dt><dd class="text-slate-400">Part-time, local improvement projects.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-28 shrink-0">Black Belt</dt><dd class="text-slate-400">Full-time, cross-functional project leadership.</dd></div>
              <div class="flex gap-2"><dt class="font-medium text-slate-200 w-28 shrink-0">Master Black Belt</dt><dd class="text-slate-400">Strategic deployment; coaches Black Belts.</dd></div>
            </dl>
          </section>

        {/if}
