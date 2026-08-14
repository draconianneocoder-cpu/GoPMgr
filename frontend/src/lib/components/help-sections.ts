// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

export type SectionId =
  | 'getting-started'
  | 'quick-start'
  | 'industry-matrix'
  | 'scrum'
  | 'kanban'
  | 'scrumban'
  | 'lean'
  | 'okrs'
  | 'waterfall'
  | 'prince2'
  | 'pmbok'
  | 'cpm'
  | 'six-sigma-method'
  | 'portfolio'
  | 'project-dashboard'
  | 'agile-boards'
  | 'budget'
  | 'timeline'
  | 'stakeholders'
  | 'project-settings'
  | 'scenarios'
  | 'import-export'
  | 'report-composer'
  | 'export-signing'
  | 'encryption'
  | 'backups'
  | 'admin-panel'
  | 'app-settings'
  | 'charts'
  | 'documents'
  | 'sigma-pack'
  | 'shortcuts'
  | 'glossary'
  | 'install'
  | 'troubleshooting'
  | 'cli';

export const sidebar: { group: string; items: { id: SectionId; label: string }[] }[] = [
  {
    group: 'Overview',
    items: [
      { id: 'getting-started', label: 'Getting Started' },
      { id: 'industry-matrix', label: 'Industry & Methodology Matrix' },
    ],
  },
  {
    group: 'Tutorials',
    items: [{ id: 'quick-start', label: 'Quick Start: Your First Project' }],
  },
  {
    group: 'Methodologies',
    items: [
      { id: 'scrum', label: 'Scrum' },
      { id: 'kanban', label: 'Kanban' },
      { id: 'scrumban', label: 'Scrumban' },
      { id: 'lean', label: 'Lean' },
      { id: 'okrs', label: 'OKRs' },
      { id: 'waterfall', label: 'Waterfall' },
      { id: 'prince2', label: 'PRINCE2' },
      { id: 'pmbok', label: 'PMBOK' },
      { id: 'cpm', label: 'Critical Path (CPM)' },
      { id: 'six-sigma-method', label: 'Six Sigma' },
    ],
  },
  {
    group: 'Features',
    items: [
      { id: 'portfolio', label: 'Portfolio' },
      { id: 'project-dashboard', label: 'Project Dashboard' },
      { id: 'agile-boards', label: 'Kanban, Sprints & DORA' },
      { id: 'budget', label: 'Budget' },
      { id: 'timeline', label: 'Timeline' },
      { id: 'stakeholders', label: 'Stakeholder Manager' },
      { id: 'project-settings', label: 'Project Settings' },
      { id: 'scenarios', label: 'Scenarios & What-If' },
      { id: 'import-export', label: 'Schedule Import & Export' },
      { id: 'report-composer', label: 'Report Composer' },
      { id: 'export-signing', label: 'Export & Digital Signing' },
      { id: 'encryption', label: 'Database Encryption' },
      { id: 'backups', label: 'Backups & Data Safety' },
      { id: 'admin-panel', label: 'Admin Panel' },
      { id: 'app-settings', label: 'App Settings' },
    ],
  },
  {
    group: 'Reference',
    items: [
      { id: 'charts', label: 'Charts' },
      { id: 'documents', label: 'Documents' },
      { id: 'sigma-pack', label: 'DMAIC Pack' },
      { id: 'shortcuts', label: 'Keyboard Shortcuts & Accessibility' },
      { id: 'glossary', label: 'Glossary' },
      { id: 'install', label: 'Installing & Running' },
    ],
  },
  {
    group: 'Help & Troubleshooting',
    items: [
      { id: 'troubleshooting', label: 'Troubleshooting & FAQ' },
      { id: 'cli', label: 'Command-Line Maintenance' },
    ],
  },
];

// Only the active section's body is rendered, so the sidebar search matches
// against this hand-curated keyword index plus the section labels/groups.
// Keep entries lowercase; extend when adding sections -- the Record<SectionId,
// string> annotation makes a missing entry a compile-time (svelte-check)
// error, not a silently-incomplete search index. This is also why this
// index stays centralized here rather than splitting one slice per
// Help*.svelte content component: distributing it would mean the
// completeness guarantee could only be enforced by re-deriving the full
// SectionId union from every content component's own exports, instead of
// TypeScript checking one Record literal directly against the union.
export const KEYWORDS: Record<SectionId, string> = {
  'getting-started':
    'first launch create account admin administrator passphrase password login sign in users data directory navigation menu recovery codes new project launchpad',
  'quick-start':
    'tutorial beginner walkthrough first project step by step example schedule task dependency export pdf report onboarding start here how to begin',
  'industry-matrix':
    'seeded artifacts starter templates software construction engineering business administration custom combination launchpad',
  scrum: 'sprint backlog product owner scrum master velocity retrospective standup agile ceremonies story points',
  kanban: 'board columns wip limit work in progress flow pull continuous cards lanes cycle time',
  scrumban: 'hybrid sprint flow wip planning trigger bucket agile mix',
  lean: 'waste value stream pull kaizen continuous improvement muda flow efficiency',
  okrs: 'objectives key results goals alignment scoring quarterly cadence outcomes',
  waterfall: 'phases sequential requirements design implementation verification maintenance gantt milestones',
  prince2: 'stages governance business case project board tolerance exception work packages themes processes',
  pmbok: 'pmi process groups knowledge areas initiating planning executing monitoring closing pmp',
  cpm: 'critical path float slack forward pass backward pass network dependencies es ef ls lf duration schedule',
  'six-sigma-method': 'dmaic define measure analyze improve control defects variation belts quality spc',
  portfolio: 'dashboard all projects overview rollup analytics duckdb import csv xlsx status filter search cost',
  'project-dashboard':
    'open project charts documents budget committed contracts labour earned value evm spi cpi new chart export delete',
  'agile-boards':
    'kanban board columns cards drag drop wip limit work items backlog reorder priority story points sprints start complete active capacity goal dora deployment frequency lead time change failure rate mttr restore trend record deployment',
  budget:
    'budget panel committed remaining contracts labour estimate rollup cost money cents category breakdown over budget stakeholder rates hourly',
  'project-settings':
    'project settings name description owner industry methodology country status phase dates budget signing defaults certificate resource capacity calendars weekly overrides fonts import ttf compliance mode audit chain scenarios encryption migration schedule reports',
  scenarios:
    'scenario what if what-if copy chart baseline partition compare promote schedule baselines isolated experiment planning alternatives',
  'import-export':
    'import export ms project xml mspdi mpp interchange schedule report pdf docx odt csv html spreadsheet round trip',
  backups:
    'backup data safety copy project files gopmgr folder pre-encryption backup retained restore integrity check repair vacuum maintenance cli recovery',
  timeline: 'calendar holidays country milestones dates schedule view months workdays',
  stakeholders: 'stakeholder manager power interest grid raci contacts influence engagement contract rates',
  'report-composer': 'combined report multiple documents charts assemble pdf export sections cover page',
  'export-signing':
    'pdf export sign pades digital signature certificate p12 pfx password gpg gnupg detached asc encrypt aes verify docx odt xlsx csv html mspdi xml formats',
  encryption:
    'sqlcipher database encryption at rest dek key passphrase lock secure recovery codes migrate plaintext',
  'admin-panel': 'user management create accounts admin recovery codes provision reset',
  'app-settings':
    'theme light dark auto save interval become administrator logs folder diagnostics preferences application settings',
  charts:
    'chart types catalog wbs network pert cpm gantt fishbone cause effect workflow activity raci swot stakeholder matrix line bar pareto pie burn up down cumulative flow control engines editors connect nodes dependencies baseline monte carlo histogram',
  documents:
    'charter scope statement risk register communication plan status report statement of work project plan word templates edit export',
  'sigma-pack': 'dmaic pack six sigma tollgate ctq sipoc capability control charts project view',
  shortcuts:
    'keyboard shortcuts hotkeys ctrl cmd save accessibility screen reader focus escape tab enter space navigate a11y announce reduced motion',
  glossary: 'terms definitions vocabulary jargon meaning dictionary',
  install: 'install run linux windows macos webkit dependencies build wails cli headless requirements',
  troubleshooting:
    'troubleshoot faq problem help forgot password passphrase locked out reset recovery code admin lockout corrupt database repair check crash startup logs diagnostics font missing glyph webkit blank window signature unsigned agile views missing compliance blocked audit',
  cli: 'command line cli headless terminal maintenance check repair vacuum export audit stats schema dump format encrypt password env username scripts automation batch',
};
