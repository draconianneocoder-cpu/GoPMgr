// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

export interface HelpTerm {
  id: string;
  term: string;
  def: string;
  aliases?: readonly string[];
  see?: readonly string[];
}

// Stable IDs belong to the reference entries, not translated display labels.
export const HELP_GLOSSARY: readonly HelpTerm[] = [
  {
    term: "Accountability",
    def: "Ultimate ownership of an outcome. In RACI, the \"A\" — one person owns the result even if others do the work.",
    id: "accountability",
    see: [
      "raci-matrix"
    ]
  },
  {
    term: "Actual Cost (AC)",
    def: "Cost recorded for work performed on the stated accounting basis. It is not the budgeted value of that work or necessarily cash already paid.",
    id: "actual-cost-ac",
    aliases: [
      "AC"
    ],
    see: [
      "earned-value-ev",
      "planned-value-pv"
    ]
  },
  {
    term: "Administrator Escrow",
    def: "A separately protected administrator-key mechanism for enrolled user data. The documented recorded, read-only access rules still apply; escrow is not a user password or an unrestricted entitlement for every administrator.",
    id: "administrator-escrow",
    see: [
      "administrator-key-recovery-code",
      "recovery-codes"
    ]
  },
  {
    term: "Administrator-Key Recovery Code",
    def: "A code for restoring the administrator key under the documented recovery conditions. It is distinct from one-time account recovery codes and does not replace required account data or backups.",
    id: "administrator-key-recovery-code",
    see: [
      "recovery-codes",
      "administrator-escrow"
    ]
  },
  {
    term: "Agile",
    def: "Umbrella term for iterative delivery methodologies valuing customer collaboration and responding to change. Scrum, Kanban, and Scrumban are Agile frameworks.",
    id: "agile"
  },
  {
    term: "Audit Trail",
    def: "Recorded events and integrity evidence for project operations. An intact audit chain is not encryption and does not certify that every business fact is correct.",
    id: "audit-trail"
  },
  {
    term: "Autosave",
    def: "Saving changed editor state on the configured cycle. An edit waiting for or still undergoing autosave is not yet saved; autosave is not an independent backup.",
    id: "autosave",
    see: [
      "saved-change",
      "backup"
    ]
  },
  {
    term: "Backlog",
    def: "Ordered list of work items waiting to be pulled into active work. In Scrum: the Product Backlog. In Scrumban: maintained continuously.",
    id: "backlog"
  },
  {
    term: "Backup",
    def: "A retained copy of project data used for restoration. A report export is not a complete project backup; recovery codes are not copies of project data.",
    id: "backup",
    see: [
      "export",
      "recovery-codes"
    ]
  },
  {
    term: "Baseline",
    def: "A saved reference plan used to compare later scope, dates or costs. Capturing a GoPMgr baseline does not itself prove formal approval; distinguish it from the editable current plan.",
    id: "baseline",
    see: [
      "current-plan",
      "forecast"
    ]
  },
  {
    term: "Belt Level",
    def: "Six Sigma practitioner certification: Green Belt (part-time, local projects), Black Belt (full-time, cross-functional), Master Black Belt (strategic deployment).",
    id: "belt-level"
  },
  {
    term: "Budget",
    def: "A planned amount or approved limit on the stated basis. Distinguish budget from committed costs, actual costs and a forecast; do not add amounts with overlapping coverage.",
    id: "budget",
    see: [
      "actual-cost-ac",
      "forecast"
    ]
  },
  {
    term: "Burn-Down Chart",
    def: "Remaining work (y) over time (x) with an ideal trajectory. Used in Scrum to track sprint progress.",
    id: "burn-down-chart"
  },
  {
    term: "Burn-Up Chart",
    def: "Cumulative scope completed vs. total scope. Distinguishes scope growth from delivery progress.",
    id: "burn-up-chart"
  },
  {
    term: "Business Case",
    def: "Document justifying the project investment: costs, benefits, risks, NPV/ROI. Primary go/no-go input.",
    id: "business-case"
  },
  {
    term: "Card / Task (Kanban)",
    def: "A single unit of work on the Kanban Board.",
    id: "card-task-kanban"
  },
  {
    term: "Change Failure Rate",
    def: "DORA metric: the share of deployments that cause a failure in production. Computed on the DORA dashboard from the deployments you record.",
    id: "change-failure-rate"
  },
  {
    term: "Change Request",
    def: "Formal proposal to modify a project baseline (scope, schedule, cost, quality). Requires review and approval.",
    id: "change-request"
  },
  {
    term: "Compliance Mode",
    def: "Project Settings option that verifies the tamper-evident audit chain before a project opens and blocks the open if the chain has been altered.",
    id: "compliance-mode"
  },
  {
    term: "Control Chart",
    def: "Time series with UCL and LCL. Points outside limits or non-random patterns indicate special-cause variation.",
    id: "control-chart"
  },
  {
    term: "Control Limits",
    def: "Statistical boundaries on a Control Chart (±3σ from the process mean). Not the same as specification limits.",
    id: "control-limits"
  },
  {
    term: "Cost Performance Index (CPI)",
    def: "EV divided by AC on the stated cost/progress basis. Below one indicates cost inefficiency; a zero denominator makes the ratio undefined, not zero performance.",
    id: "cost-performance-index-cpi",
    aliases: [
      "CPI"
    ],
    see: [
      "schedule-performance-index-spi"
    ]
  },
  {
    term: "CPM",
    def: "Critical Path Method: calculates dependency-based start/finish dates and float from modeled durations and relationships. It is not resource leveling.",
    id: "cpm",
    aliases: [
      "Critical Path Method"
    ],
    see: [
      "resource-leveling",
      "critical-path"
    ]
  },
  {
    term: "Critical Path",
    def: "Dependency-connected tasks controlling the calculated project finish in the modeled schedule. Interpret float and constraints separately; a zero-float description alone is insufficient for constrained schedules.",
    id: "critical-path",
    see: [
      "float-slack",
      "resource-leveling"
    ]
  },
  {
    term: "CTQ",
    def: "Critical to Quality. Measurable characteristic directly affecting customer perception of quality. Derived from VoC in Six Sigma.",
    id: "ctq"
  },
  {
    term: "Cumulative Flow Diagram",
    def: "Stacked area chart of work items in each workflow state over time. Primary Kanban health indicator.",
    id: "cumulative-flow-diagram"
  },
  {
    term: "Current Plan",
    def: "The editable plan reflecting present choices. A baseline retains a reference for comparison; editing the current plan does not mean the reference was formally reapproved.",
    id: "current-plan",
    see: [
      "baseline"
    ]
  },
  {
    term: "Cycle Time",
    def: "Time from when a work item starts to when it is completed. Key delivery predictability metric in Kanban.",
    id: "cycle-time",
    see: [
      "lead-time"
    ]
  },
  {
    term: "DEK",
    def: "Data Encryption Key: the per-user key used for encrypted project databases. It is stored wrapped on disk and unlocked in memory; it is not your passphrase, a recovery code or a signing key.",
    id: "dek",
    aliases: [
      "Data Encryption Key"
    ],
    see: [
      "passphrase",
      "recovery-codes",
      "administrator-escrow"
    ]
  },
  {
    term: "Deployment Frequency",
    def: "DORA metric: how often the team ships to production. Recorded per deployment on the DORA dashboard.",
    id: "deployment-frequency"
  },
  {
    term: "Digital Signature",
    def: "Cryptographic evidence linking signed bytes to a signer under a validation policy. It is not encryption, archival conformance or a guarantee of factual correctness.",
    id: "digital-signature",
    see: [
      "encryption-at-rest",
      "pdf-a-2u"
    ]
  },
  {
    term: "DMAIC",
    def: "Six Sigma improvement cycle: Define → Measure → Analyze → Improve → Control. For improving existing processes.",
    id: "dmaic"
  },
  {
    term: "DORA Metrics",
    def: "Four DevOps metrics: Deployment Frequency, Lead Time for Changes, Change Failure Rate, Mean Time to Restore. Available in GoPMgr's DORA dashboard.",
    id: "dora-metrics"
  },
  {
    term: "Earned Value (EV)",
    def: "The budgeted value of completed work under the stated progress basis. It differs from planned value (PV) and actual cost (AC); EVM is the overall management method.",
    id: "earned-value-ev",
    aliases: [
      "EV"
    ],
    see: [
      "planned-value-pv",
      "actual-cost-ac",
      "earned-value-management-evm"
    ]
  },
  {
    term: "Earned Value Management (EVM)",
    def: "Earned Value Management compares planned value, earned value and actual cost. EVM names the method; EV is the budgeted value of completed work, not the money spent.",
    aliases: [
      "Earned Value (EVM)",
      "EVM"
    ],
    id: "earned-value-management-evm",
    see: [
      "earned-value-ev",
      "cost-performance-index-cpi",
      "schedule-performance-index-spi"
    ]
  },
  {
    term: "Encryption at Rest",
    def: "Protection for stored data using a key, such as an encrypted project database. It does not automatically protect exported files, an unlocked session or copies stored elsewhere.",
    id: "encryption-at-rest",
    see: [
      "digital-signature",
      "dek"
    ]
  },
  {
    term: "Epic",
    def: "A large user story spanning multiple sprints, broken down into smaller stories before entering a sprint.",
    id: "epic"
  },
  {
    term: "ES / EF",
    def: "Earliest Start / Earliest Finish. Calculated during the forward pass through a CPM network.",
    id: "es-ef"
  },
  {
    term: "Estimate (methodology-specific)",
    def: "What each methodology usually calls an estimate. Scrum: Story Points. Kanban: Time Estimate. CPM: Duration. Waterfall: Duration. Lean: Effort. PRINCE2: Work Package Estimate. Six Sigma: Resource Plan. In GoPMgr, a work item's estimate is labelled Story points in Scrum and Scrumban projects and Points in others; CPM and Gantt tasks are estimated as a duration in days.",
    id: "estimate-methodology-specific"
  },
  {
    term: "Export",
    def: "An output file made for another tool, viewing or distribution. It need not retain all editable project data and is not a backup or evidence of formal document approval.",
    id: "export",
    see: [
      "backup",
      "saved-change"
    ]
  },
  {
    term: "Fishbone Diagram",
    def: "Also Ishikawa Diagram. Root-cause analysis: effect at the \"head\"; default cause categories (People, Process, Equipment, Materials, Environment, Measurement) are the \"bones.\" Categories are editable.",
    id: "fishbone-diagram"
  },
  {
    term: "Float (Slack)",
    def: "Amount of time a task can be delayed without delaying the project end date (total float) or a successor's start (free float).",
    id: "float-slack",
    see: [
      "critical-path"
    ]
  },
  {
    term: "Forecast",
    def: "An estimate of a future outcome based on a named method and assumptions. It is neither an actual observation nor the approved reference plan.",
    id: "forecast"
  },
  {
    term: "Gantt Chart",
    def: "Schedule bars on a time axis with dependency arrows, critical-path highlighting, and progress bars. Shares data with CPM in GoPMgr.",
    id: "gantt-chart"
  },
  {
    term: "Hard Constraint",
    def: "A scheduling constraint that limits how late a task may run: start no later than (SNLT), finish no later than (FNLT), must start on (MSO) or must finish on (MFO). It hides what a delay does to that date; the schedule audit counts these under SQ-08. Start or finish no earlier than (SNET, FNET) are soft.",
    id: "hard-constraint",
    see: [
      "scheduling-constraint"
    ]
  },
  {
    term: "Issue",
    def: "A problem or condition that has occurred and needs handling. A risk is an uncertain event; a change request proposes an alteration to the plan.",
    id: "issue",
    see: [
      "risk",
      "change-request"
    ]
  },
  {
    term: "Issue Log",
    def: "Tracks problems that have already occurred, with owner, priority, and resolution plan. Distinct from Risk Register (potential future events).",
    id: "issue-log",
    see: [
      "risk-register"
    ]
  },
  {
    term: "Iteration",
    def: "A repeated work and review cycle, often time-boxed, such as a Scrum Sprint. A PRINCE2 management stage is a governance interval; continuous-flow methods need not use fixed iterations.",
    id: "iteration"
  },
  {
    term: "Kaizen",
    def: "Lean concept of continuous, incremental improvement made frequently by everyone involved in the process.",
    id: "kaizen"
  },
  {
    term: "Kanban Board",
    def: "Visual work management: columns = workflow states; cards = work items. WIP limits cap items per column.",
    id: "kanban-board"
  },
  {
    term: "Key Result (KR)",
    def: "OKRs: measurable, time-bound outcome tracking progress toward an Objective. Graded 0.0–1.0 at period close.",
    id: "key-result-kr"
  },
  {
    term: "Launchpad",
    def: "GoPMgr's project creation wizard. Guides through industry, methodology, and project name selection; seeds starter artifacts.",
    id: "launchpad"
  },
  {
    term: "Lead Time",
    def: "Total time from request to delivery. Includes queue time + cycle time.",
    id: "lead-time",
    see: [
      "cycle-time"
    ]
  },
  {
    term: "Levelled Start",
    def: "The start date resource levelling gave a CPM task. It delays the task like start no earlier than (SNET) but is not a constraint: each levelling run recalculates or clears it, and you can clear it in the task panel. A constraint date it pushes the task past is flagged.",
    id: "levelled-start",
    aliases: [
      "Leveled Start",
      "level_start"
    ],
    see: [
      "resource-leveling",
      "scheduling-constraint"
    ]
  },
  {
    term: "LS / LF",
    def: "Latest Start / Latest Finish. Calculated during the backward pass through a CPM network.",
    id: "ls-lf"
  },
  {
    term: "Milestone",
    def: "A significant zero-duration schedule event. A Definition of Done is a completion-quality standard; a throughput target is a delivery-rate goal, not a milestone.",
    id: "milestone"
  },
  {
    term: "Monte Carlo Simulation",
    def: "Schedule-risk technique that samples uncertain task durations thousands of times to estimate finish-date probabilities. Each run applies the chart's constraints and levelled starts as the schedule does. Run from the CPM editor; results include P50/P80/P90 dates, an S-curve, and a tornado driver ranking.",
    id: "monte-carlo-simulation"
  },
  {
    term: "MSPDI",
    def: "Microsoft Project Data Interchange — the .xml schedule format GoPMgr imports and exports. Resave .mpp/.pod/.mpx files as MS Project XML before importing.",
    id: "mspdi"
  },
  {
    term: "MTTR",
    def: "Mean Time to Restore. DORA metric: average hours to recover from a failed deployment.",
    id: "mttr"
  },
  {
    term: "Objective",
    def: "OKRs: qualitative, inspiring goal statement answering \"where do we want to go?\" Supported by measurable Key Results.",
    id: "objective"
  },
  {
    term: "OKRs",
    def: "Objectives and Key Results. Goal-setting framework aligning teams to strategic outcomes through measurable, time-bound Key Results.",
    id: "okrs"
  },
  {
    term: "P50 / P80 / P90",
    def: "Confidence points from a Monte Carlo run: the finish day the project beats in 50%, 80%, or 90% of simulations. Commit to P80/P90 dates when the cost of being late is high.",
    id: "p50-p80-p90"
  },
  {
    term: "PAdES",
    def: "PDF Advanced Electronic Signatures: signatures used to validate integrity and signer identity under a certificate/trust policy. Signing is not encryption and does not prove the report calculations are correct.",
    id: "pades",
    aliases: [
      "PDF Advanced Electronic Signatures"
    ],
    see: [
      "pdf-a-2u",
      "digital-signature"
    ]
  },
  {
    term: "Pareto Chart",
    def: "Bar chart sorted descending with a cumulative-percentage line. Based on the 80/20 principle: ~80% of effects come from ~20% of causes.",
    id: "pareto-chart"
  },
  {
    term: "Passphrase",
    def: "The account password used to authenticate and unlock protected key material. It is not the DEK itself; resetting a password is not equivalent to restoring encrypted project access.",
    id: "passphrase",
    aliases: [
      "Password"
    ],
    see: [
      "dek",
      "recovery-codes"
    ]
  },
  {
    term: "PDF/A-2u",
    def: "An archival PDF conformance target with Unicode text mapping. Conformance does not prove accessibility, correct content or indefinite preservation; it does not encrypt or digitally sign the file.",
    id: "pdf-a-2u",
    aliases: [
      "Archival PDF",
      "PDF/A"
    ],
    see: [
      "pades",
      "encryption-at-rest"
    ]
  },
  {
    term: "PERT",
    def: "Program Evaluation and Review Technique. Network scheduling with three-point duration estimates per activity.",
    id: "pert"
  },
  {
    term: "Planned Value (PV)",
    def: "The budgeted value of work scheduled by the stated cutoff and baseline. It is not actual spending or the value of work already completed.",
    id: "planned-value-pv",
    aliases: [
      "PV"
    ],
    see: [
      "earned-value-ev",
      "actual-cost-ac"
    ]
  },
  {
    term: "Planning Meeting",
    def: "Scrum: Sprint Planning. Kanban: Replenishment Meeting. Lean: Plan-Do-Check-Act review. PRINCE2: Stage Planning Meeting.",
    id: "planning-meeting"
  },
  {
    term: "PMBOK",
    def: "Project Management Body of Knowledge. PMI's comprehensive PM framework. A knowledge standard, not a delivery methodology.",
    id: "pmbok"
  },
  {
    term: "PRINCE2",
    def: "Projects In Controlled Environments. Process-based framework with seven principles, themes, and processes. Strong governance and stage-gate controls.",
    id: "prince2"
  },
  {
    term: "Process Group",
    def: "PMBOK grouping of related PM processes: Initiating, Planning, Executing, Monitoring & Controlling, Closing. Corresponds to GoPMgr's document Phase categories.",
    id: "process-group"
  },
  {
    term: "Project Charter",
    def: "Foundational document formally authorizing a project. Captures purpose, objectives, scope, sponsor, high-level schedule and budget.",
    id: "project-charter"
  },
  {
    term: "Project Database",
    def: "The .gopmgr database containing one project and its records. It is distinct from system.db, which holds local account metadata.",
    id: "project-database",
    see: [
      "system-database"
    ]
  },
  {
    term: "Pull System",
    def: "Work produced when downstream capacity exists, not pushed by a schedule. Core Lean and Kanban principle.",
    id: "pull-system"
  },
  {
    term: "RACI Matrix",
    def: "Responsibility assignment: Responsible (does work), Accountable (owns outcome), Consulted (input needed), Informed (kept updated). One A per task.",
    id: "raci-matrix"
  },
  {
    term: "Recovery Codes",
    def: "One-time account codes for password reset; valid wrapped codes can also unlock the DEK. Codes may need reissuing after encryption migration. Lost credentials are not always recoverable, and these are not administrator-key recovery codes.",
    id: "recovery-codes",
    aliases: [
      "Account recovery codes"
    ],
    see: [
      "dek",
      "administrator-key-recovery-code"
    ]
  },
  {
    term: "Resource Capacity",
    def: "The available working time or quantity for a resource over a period and calendar. Assignment demand may exceed capacity; capacity is not the amount of work already completed.",
    id: "resource-capacity"
  },
  {
    term: "Resource Leveling",
    def: "Delaying contended tasks so resource demand fits capacity. Uses the named resource calendars (weekly capacity + day overrides) from Project Settings. Each delay is kept as the task's levelled start; constraints are never changed.",
    id: "resource-leveling",
    see: [
      "cpm",
      "resource-capacity"
    ]
  },
  {
    term: "Retrospective",
    def: "Process improvement ceremony. Scrum: Sprint Retrospective. Kanban: Retrospective. PRINCE2: End-Stage Assessment lessons.",
    id: "retrospective"
  },
  {
    term: "Risk",
    def: "An uncertain event or condition that may affect objectives. Once a problem occurs it is handled as an issue; documenting a risk does not by itself implement a response.",
    id: "risk",
    see: [
      "issue",
      "risk-register"
    ]
  },
  {
    term: "Risk Register",
    def: "Catalogue of potential future risks with probability, impact, owner, mitigation strategy, and contingency plan.",
    id: "risk-register",
    see: [
      "issue-log"
    ]
  },
  {
    term: "Saved Change",
    def: "An edit whose save operation completed successfully. An in-flight or pending edit is not yet saved. Exporting a file and retaining a backup are separate actions.",
    id: "saved-change",
    see: [
      "autosave",
      "export",
      "backup"
    ]
  },
  {
    term: "Scenario",
    def: "An isolated what-if partition. Charts copied into a scenario can be edited and compared against their captured baseline without touching the live plan, then promoted to a named schedule baseline or discarded.",
    id: "scenario"
  },
  {
    term: "Schedule Performance Index (SPI)",
    def: "EV divided by PV on the stated baseline/progress basis. It compares budgeted work, not calendar finish-date slip; a zero denominator makes the ratio undefined.",
    id: "schedule-performance-index-spi",
    aliases: [
      "SPI"
    ],
    see: [
      "critical-path",
      "cost-performance-index-cpi"
    ]
  },
  {
    term: "Scheduling Constraint",
    def: "A rule tying a CPM task to a date or its late dates: ASAP, ALAP, SNET, SNLT, FNET, FNLT, MSO or MFO. Links always win: a constraint the links cannot meet is flagged as violated, not forced.",
    id: "scheduling-constraint",
    aliases: [
      "ASAP",
      "ALAP",
      "SNET",
      "SNLT",
      "FNET",
      "FNLT",
      "MSO",
      "MFO"
    ],
    see: [
      "hard-constraint",
      "float-slack",
      "critical-path"
    ]
  },
  {
    term: "Scrum",
    def: "Agile framework with time-boxed sprints, Product Backlog, and three roles. Four ceremonies: Sprint Planning, Daily Scrum, Sprint Review, Retrospective.",
    id: "scrum"
  },
  {
    term: "Scrumban",
    def: "Hybrid of Scrum's prioritized backlog and Kanban's continuous pull flow. No fixed sprint cadence.",
    id: "scrumban"
  },
  {
    term: "Sigma Level",
    def: "A measure of process performance in Six Sigma. A quoted 3.4 defects per million for six sigma assumes the conventional 1.5-sigma shift; it is not automatic proof of a process capability.",
    id: "sigma-level"
  },
  {
    term: "SIPOC",
    def: "High-level process map: Suppliers → Inputs → Process → Outputs → Customers. Defines process boundaries in DMAIC Define phase.",
    id: "sipoc"
  },
  {
    term: "Solution Matrix",
    def: "Six Sigma Improve tool. Evaluates solutions on Impact, Effort, Risk, and Cost. Statuses: Proposed, Pilot, Implemented.",
    id: "solution-matrix"
  },
  {
    term: "Sprint",
    def: "Scrum time-boxed iteration (typically 2 weeks). Contains a Sprint Goal, committed backlog items, Review, and Retrospective.",
    id: "sprint"
  },
  {
    term: "SQLCipher",
    def: "Open-source encrypted SQLite extension used by GoPMgr for database encryption at rest. Applies AES-256 to the entire project database file.",
    id: "sqlcipher"
  },
  {
    term: "Stakeholder",
    def: "Individual or group with an interest in or influence over the project. Mapped by power/interest in the Stakeholder Analysis Matrix.",
    id: "stakeholder"
  },
  {
    term: "Story (Scrum)",
    def: "User-facing work item: \"As a [role], I want [feature] so that [benefit].\" Equivalent to a Task in most other methodologies.",
    id: "story-scrum"
  },
  {
    term: "Story Points",
    def: "Relative effort estimate in Scrum. Team-defined scale (typically Fibonacci). Not correlated to hours.",
    id: "story-points"
  },
  {
    term: "SWOT Matrix",
    def: "2x2 strategic analysis: Strengths, Weaknesses (internal) vs. Opportunities, Threats (external).",
    id: "swot-matrix"
  },
  {
    term: "System Database",
    def: "The local system.db account-metadata database. It is distinct from each project database; encrypting a project does not mean every application file or export is encrypted.",
    id: "system-database",
    see: [
      "project-database"
    ]
  },
  {
    term: "Throughput",
    def: "Work items completed per unit time. Primary Kanban flow metric alongside cycle time.",
    id: "throughput"
  },
  {
    term: "Timeline",
    def: "GoPMgr view showing the project's chronological event stream as an SVG strip: sprint bands, milestones, and holiday markers.",
    id: "timeline"
  },
  {
    term: "Tornado Ranking",
    def: "Monte Carlo output ranking which tasks drive schedule risk, combining how often each lands on the critical path with its P90−P50 duration spread. Fix the top bars first.",
    id: "tornado-ranking"
  },
  {
    term: "User Story",
    def: "See Story (Scrum).",
    id: "user-story"
  },
  {
    term: "Velocity",
    def: "Average story points completed per sprint. Used for capacity planning and release forecasting.",
    id: "velocity"
  },
  {
    term: "VoC",
    def: "Voice of Customer. Customer needs captured via surveys, interviews, or complaints. Input to CTQ derivation in Six Sigma.",
    id: "voc"
  },
  {
    term: "WBS",
    def: "Work Breakdown Structure. Hierarchical decomposition of total project scope into deliverables and work packages.",
    id: "wbs",
    aliases: [
      "Work Breakdown Structure"
    ],
    see: [
      "work-package",
      "work-item"
    ]
  },
  {
    term: "WIP Limit",
    def: "Work-In-Progress Limit. Maximum items in a Kanban column simultaneously. Prevents overloading and surfaces bottlenecks.",
    id: "wip-limit"
  },
  {
    term: "Work Item",
    def: "An item managed in a work workflow, such as a task or story. Its workflow state or estimate is not automatically a connected schedule activity or WBS work package.",
    id: "work-item",
    see: [
      "work-package"
    ]
  },
  {
    term: "Work Package",
    def: "A manageable unit of project scope in a WBS. It differs from a scheduled activity, an assigned resource and a report describing that work.",
    id: "work-package",
    see: [
      "work-item",
      "wbs"
    ]
  }
];

export function helpTermId(id: string): string {
  return `help-term-${id}`;
}
