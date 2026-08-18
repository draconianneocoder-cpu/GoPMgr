<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import type { SectionId } from '../help-sections';

  let { active, nav }: { active: SectionId; nav: (id: SectionId) => void } = $props();
</script>

        <!-- ── Getting Started ──────────────────────────────── -->
        {#if active === 'getting-started'}
          <h2 class="text-xl font-bold text-slate-100 mb-4">Getting Started</h2>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">First Launch</h3>
            <p class="text-sm text-slate-300 mb-2">
              On first launch GoPMgr has no accounts. Enter a username, display name, and
              passphrase on the Create Account screen. The first account is prompted to become
              the administrator. At least one admin must exist before additional users can be added.
            </p>
            <p class="text-sm text-slate-300">
              If you skipped the admin claim, open App Settings and use "Become administrator"
              while no other admin exists on the machine.
            </p>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Adding Users</h3>
            <p class="text-sm text-slate-300">
              Administrators create additional accounts from the
              <button onclick={() => nav('admin-panel')} class="text-cyan-400 underline hover:text-cyan-300">Admin Panel</button>.
              Each GoPMgr user gets their own isolated data directory. Multiple GoPMgr users
              can share a single OS account; project files are stored per-user and are not
              cross-accessible through the app.
            </p>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Creating a Project — The Launchpad</h3>
            <p class="text-sm text-slate-300 mb-3">
              From the Portfolio screen, click "New Project" (or File &rarr; New Project). The Launchpad
              walks through four steps:
            </p>
            <ol class="space-y-2 text-sm text-slate-300 list-decimal list-inside">
              <li><span class="font-medium text-slate-100">Industry</span> — Software, Construction, Engineering, Business, Administration, or Custom.</li>
              <li><span class="font-medium text-slate-100">Focus area</span> — narrows the industry (for example Web Dev, Civil, or Marketing).</li>
              <li>
                <span class="font-medium text-slate-100">Methodology</span> — delivery approach.
                Each combination seeds different starter artifacts. See the
                <button onclick={() => nav('industry-matrix')} class="text-cyan-400 underline hover:text-cyan-300">Industry &amp; Methodology Matrix</button>.
              </li>
              <li><span class="font-medium text-slate-100">Details &amp; starter artifacts</span> — review the blueprint, enter the project name and optional description, choose a business-calendar policy and schedule/chart time zone, then review the suggested starter artifacts. Click Create Project to finish.</li>
            </ol>
            <p class="text-sm text-slate-300 mt-3">
              A selection stays on each step until you click Continue. Business-calendar policies
              determine holidays and working days; the selected IANA time zone determines schedule
              boundaries and time-series chart dates.
            </p>
            <p class="text-sm text-slate-300 mt-3">
              New to GoPMgr? The
              <button onclick={() => nav('quick-start')} class="text-cyan-400 underline hover:text-cyan-300">Quick Start tutorial</button>
              walks the whole journey — account to exported report — in about ten minutes.
            </p>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Navigation</h3>
            <p class="text-sm text-slate-300">
              The top bar provides: Portfolio (all projects), Projects (project picker), App Settings,
              Help (this guide), and Sign Out. Within an open project the sidebar gives access to Charts,
              Documents, and methodology-specific views (Kanban, Backlog, Sprints, DORA, Six Sigma, etc.).
              Use File &rarr; Close Project to return to the Portfolio without signing out.
            </p>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Account Recovery</h3>
            <p class="text-sm text-slate-300">
              GoPMgr is a local-first application with no cloud backup. Generate recovery codes
              immediately after creating your account (App Settings &rarr; Recovery Codes section).
              Store them securely. Recovery codes let you reset your passphrase from the login screen.
              Recovery codes must be current before enabling database encryption.
            </p>
          </section>

        <!-- ── Industry & Methodology Matrix ──────────────────────────────── -->
        {:else if active === 'industry-matrix'}
          <h2 class="text-xl font-bold text-slate-100 mb-2">Industry &amp; Methodology Matrix</h2>
          <p class="text-sm text-slate-400 mb-5">
            The Launchpad seeds starter artifacts automatically for these combinations.
            All other industry/methodology pairings receive a Project Charter only. Additional
            artifacts can always be created manually after project creation.
          </p>

          <div class="overflow-x-auto mb-6">
            <table class="w-full text-sm border-collapse">
              <thead>
                <tr class="border-b border-slate-700">
                  <th class="text-left py-2 pr-4 font-semibold text-slate-300 whitespace-nowrap">Industry</th>
                  <th class="text-left py-2 pr-4 font-semibold text-slate-300 whitespace-nowrap">Methodology</th>
                  <th class="text-left py-2 font-semibold text-slate-300">Seeded Artifacts</th>
                </tr>
              </thead>
              <tbody class="text-slate-300">
                {#each [
                  { ind: 'Software', meth: 'scrum' as SectionId, mLabel: 'Scrum', arts: 'Kanban Board, Project Charter, Agile Backlog, Sprint 1' },
                  { ind: 'Software', meth: 'kanban' as SectionId, mLabel: 'Kanban', arts: 'Kanban Board, Project Charter, Agile Backlog' },
                  { ind: 'Software', meth: 'scrumban' as SectionId, mLabel: 'Scrumban', arts: 'Kanban Board, Project Charter, Agile Backlog' },
                  { ind: 'Construction', meth: 'waterfall' as SectionId, mLabel: 'Waterfall', arts: 'WBS, Statement of Work, Risk Register, CPM Chart' },
                  { ind: 'Construction', meth: 'lean' as SectionId, mLabel: 'Lean', arts: 'WBS, Cumulative Flow Diagram, Risk Register' },
                  { ind: 'Engineering', meth: 'cpm' as SectionId, mLabel: 'CPM', arts: 'CPM Chart, WBS, Risk Register, Project Charter' },
                  { ind: 'Engineering', meth: 'six-sigma-method' as SectionId, mLabel: 'Six Sigma', arts: 'Control Chart, Pareto Chart, Fishbone Diagram' },
                  { ind: 'Business', meth: 'lean' as SectionId, mLabel: 'Lean', arts: 'Pareto Chart, Cumulative Flow Diagram, SWOT Matrix' },
                  { ind: 'Business', meth: 'okrs' as SectionId, mLabel: 'OKRs', arts: 'Project Plan (Word), Stakeholder Analysis Document, Status Report' },
                  { ind: 'Administration', meth: 'waterfall' as SectionId, mLabel: 'Waterfall', arts: 'Project Charter, Scope Statement, Risk Register, Communication Plan' },
                  { ind: 'Administration', meth: 'prince2' as SectionId, mLabel: 'PRINCE2', arts: 'Project Charter, Project Plan (Word), Risk Register' },
                ] as row}
                  <tr class="border-b border-slate-800">
                    <td class="py-2 pr-4 whitespace-nowrap">{row.ind}</td>
                    <td class="py-2 pr-4">
                      <button onclick={() => nav(row.meth)} class="text-cyan-400 hover:underline">{row.mLabel}</button>
                    </td>
                    <td class="py-2">{row.arts}</td>
                  </tr>
                {/each}
                <tr>
                  <td class="py-2 pr-4 text-slate-500 italic">All others</td>
                  <td class="py-2 pr-4 text-slate-500 italic">Any</td>
                  <td class="py-2 text-slate-500 italic">Project Charter</td>
                </tr>
              </tbody>
            </table>
          </div>

        {/if}
