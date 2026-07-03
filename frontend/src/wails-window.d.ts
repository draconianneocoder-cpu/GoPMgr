// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Type declarations for the Wails-injected `window.go.main.App` bridge.
// `wails dev` regenerates strongly-typed bindings under
// frontend/wailsjs/; this file provides the same surface so Svelte
// components type-check before that generation has run.

export {};

declare global {
  interface Window {
    go: {
      main: {
        App: {
          // ----- V1 -----
          Greet: () => Promise<string>;
          GetSettings: () => Promise<UserSettings>;
          SaveSettings: (s: UserSettings) => Promise<void>;
          ResetProjectSettings: () => Promise<UserSettings>;
          SecureArchive: (projectPath: string) => Promise<string>;

          // ----- V2: accounts & session -----
          ListUsers: () => Promise<Account[]>;
          HasAnyAdmin: () => Promise<boolean>;
          CreateAccount: (
            username: string,
            displayName: string,
            password: string,
            isAdmin: boolean,
          ) => Promise<Account>;
          BecomeAdmin: () => Promise<void>;
          AdminListUsers: () => Promise<Account[]>;
          AdminDeleteUser: (username: string) => Promise<void>;
          AdminSetUserRole: (username: string, isAdmin: boolean) => Promise<void>;
          Login: (username: string, password: string) => Promise<Account>;
          Logout: () => Promise<void>;
          CurrentUser: () => Promise<Account | null>;

          // ----- V2: projects -----
          ListProjects: () => Promise<ProjectFile[]>;
          CreateProject: (name: string, description: string) => Promise<ProjectFile>;
          DeleteProject: (path: string) => Promise<void>;
          CloneProject: (path: string) => Promise<ProjectFile>;
          ProjectsOverview: () => Promise<ProjectSummary[]>;
          GetAppInfo: () => Promise<AppInfo>;
          SaveAppSettings: (s: AppSettings) => Promise<void>;
          ResetAppSettings: () => Promise<AppSettings>;
          OpenProject: (path: string) => Promise<ProjectMeta>;
          IsProjectEncrypted: (path: string) => Promise<boolean>;
          EncryptProjectAtRest: (path: string) => Promise<string>;
          CloseProject: () => Promise<void>;
          GetProjectMeta: () => Promise<ProjectMeta>;
          UpdateProjectMeta: (p: ProjectMeta) => Promise<ProjectMeta>;

          // ----- V2: charts -----
          ListChartKinds: () => Promise<ChartDefinition[]>;
          ListCharts: (kind: string) => Promise<ChartRecord[]>;
          GetChart: (id: string) => Promise<ChartRecord>;
          SaveChart: (c: ChartRecord) => Promise<ChartRecord>;
          DeleteChart: (id: string) => Promise<void>;
          LayoutChart: (id: string) => Promise<ChartLayoutResult>;

          // ----- Schedule baselines (roadmap item 17) -----
          SetScheduleBaseline: (chartId: string, name: string) => Promise<BaselineRecord>;
          ListScheduleBaselines: (chartId: string) => Promise<BaselineRecord[]>;
          DeleteScheduleBaseline: (id: string) => Promise<void>;
          CompareScheduleBaseline: (
            chartId: string,
            baselineId: string
          ) => Promise<Record<string, ScheduleVariance>>;
          ComputeScheduleEVM: (chartId: string, asOfDate: string) => Promise<EVMetrics>;
          RunChartMonteCarlo: (chartId: string, iterations: number, workers: number) => Promise<SimResult>;
          ExportChartMonteCarloRiskReport: (chartId: string, iterations: number, workers: number) => Promise<string>;
          LevelChartResources: (chartId: string, strategy: string) => Promise<LevelResult>;
          GenerateResourceHistogram: (chartId: string) => Promise<ChartRecord>;
          ImportMSPDIChart: () => Promise<ChartRecord>;

          // ----- V2: documents -----
          ListDocumentKinds: () => Promise<DocumentDefinition[]>;
          ListDocuments: (kind: string) => Promise<DocumentRecord[]>;
          GetDocument: (id: string) => Promise<DocumentRecord>;
          NewDocument: (kind: string, title: string) => Promise<DocumentRecord>;
          SaveDocument: (d: DocumentRecord) => Promise<DocumentRecord>;
          DeleteDocument: (id: string) => Promise<void>;
          ExportDocumentPDF: (id: string) => Promise<string>; // returns path written
          ExportCombinedReport: (
            reportTitle: string,
            subtitle: string,
            sections: ReportSection[],
          ) => Promise<string>;
          ExportCombinedReportSigned: (
            reportTitle: string,
            subtitle: string,
            sections: ReportSection[],
            certPath: string,
            certPassword: string,
          ) => Promise<string>;
          RepairAndSwap: () => Promise<RepairResult>;

          // ----- V2.x: Agile Pack -----
          AgileEnabled: () => Promise<boolean>; // persists to project settings
          SetAgileEnabled: (enabled: boolean) => Promise<void>; // persists to project settings
          EnsureDefaultBoard: () => Promise<{ board: AgileBoard; columns: AgileColumn[] }>;
          SaveColumn: (c: AgileColumn) => Promise<void>;
          DeleteColumn: (id: string) => Promise<void>;
          SaveWorkItem: (wi: AgileWorkItem) => Promise<AgileWorkItem>;
          GetWorkItem: (id: string) => Promise<AgileWorkItem>;
          ListWorkItems: (sprintID: string, state: string, assignee: string) => Promise<AgileWorkItem[]>;
          DeleteWorkItem: (id: string) => Promise<void>;
          MoveWorkItem: (id: string, newState: string, newOrder: number) => Promise<void>;
          WIPCounts: () => Promise<Record<string, number>>;
          SaveSprint: (s: AgileSprint) => Promise<AgileSprint>;
          ListSprints: () => Promise<AgileSprint[]>;
          DeleteSprint: (id: string) => Promise<void>;
          SaveDeployment: (d: AgileDeployment) => Promise<AgileDeployment>;
          ListDeployments: (sinceISO: string) => Promise<AgileDeployment[]>;
          DeleteDeployment: (id: string) => Promise<void>;
          ComputeDORA: (windowDays: number) => Promise<DORAResult>;

          // ----- V2.x: Foundation Slice -----
          LaunchpadEvaluate: (industry: string, methodology: string) => Promise<string[]>;
          CreateProjectFromLaunchpad: (
            name: string,
            description: string,
            industry: string,
            subCategory: string,
            methodology: string,
            countryCode: string,
            seeds: string[],
          ) => Promise<{ project: ProjectMeta; seeds: SeedReceipt[]; path: string }>;
          UpdateProjectIndustry: (
            industry: string,
            subCategory: string,
            methodology: string,
            countryCode: string,
          ) => Promise<ProjectMeta>;
          ListStakeholders: (category: string) => Promise<Stakeholder[]>;
          SaveStakeholder: (s: Stakeholder) => Promise<Stakeholder>;
          DeleteStakeholder: (id: string) => Promise<void>;
          ListResourceCalendars: () => Promise<ResourceCalendar[]>;
          SaveResourceCalendar: (c: ResourceCalendar) => Promise<ResourceCalendar>;
          DeleteResourceCalendar: (id: string) => Promise<void>;
          ListScenarios: () => Promise<Scenario[]>;
          GetScenario: (id: string) => Promise<Scenario>;
          SaveScenario: (s: Scenario) => Promise<Scenario>;
          DeleteScenario: (id: string) => Promise<void>;
          BranchScenarioChart: (
            scenarioID: string,
            chartID: string,
            baselineID: string,
          ) => Promise<ScenarioChart>;
          ListScenarioCharts: (scenarioID: string) => Promise<ScenarioChart[]>;
          GetScenarioChart: (id: string) => Promise<ScenarioChart>;
          SaveScenarioChart: (c: ScenarioChart) => Promise<ScenarioChart>;
          PromoteScenarioChartToBaseline: (
            scenarioChartID: string,
            name: string,
          ) => Promise<BaselineRecord>;
          CompareScenarioChart: (
            scenarioChartID: string,
          ) => Promise<Record<string, ScheduleVariance>>;
          BuildTimeline: () => Promise<TimelineEntry[]>;
          MoveTimelineEntry: (
            kind: TimelineKind,
            sourceID: string,
            dateISO: string,
          ) => Promise<TimelineEntry[]>;
          ListHolidays: (fromISO: string, toISO: string) => Promise<HolidayEvent[]>;
          ComputeBudget: () => Promise<BudgetSummary>;
          RunPortfolioAnalytics: () => Promise<PortfolioSummary>;
          ImportDatasetForAnalysis: () => Promise<Dataset>;
          ExportProjectICS: (includeHolidays: boolean) => Promise<string>;

          // ----- V2.x: Remaining-TODOs Slice -----
          ChooseCertFile: () => Promise<string>;
          IssueRecoveryCodes: () => Promise<string[]>;
          RemainingRecoveryCodes: () => Promise<number>;
          ResetWithRecoveryCode: (username: string, code: string, newPassword: string) => Promise<void>;
          CheckLatestVersion: () => Promise<UpdateStatus>;
          ExportDocumentDOCX: (id: string) => Promise<string>;
          ExportDocumentODT: (id: string) => Promise<string>;
          ExportDocumentPDFSigned: (
            id: string,
            certPath: string,
            certPassword: string,
          ) => Promise<string>;
          ExportDocumentPDFGnuPG: (
            id: string,
            keyID: string,
          ) => Promise<GnuPGExportResult>;
          ExportCombinedReportGnuPG: (
            reportTitle: string,
            subtitle: string,
            sections: ReportSection[],
            keyID: string,
          ) => Promise<GnuPGExportResult>;
          ExportScheduleReportDOCX: () => Promise<string>;
          ExportScheduleReportODT: () => Promise<string>;
          ExportScheduleReportPDF: () => Promise<string>;
          ExportScheduleReportCSV: () => Promise<string>;
          ExportScheduleReportHTML: () => Promise<string>;
          ExportScheduleReportMSPDI: () => Promise<string>;
          ExportAuditVerificationReport: () => Promise<string>;
          ExportAuditRepairEvidence: () => Promise<string>;

          // ----- Process Excellence Suite (Six Sigma) -----
          SigmaCreateProject: (
            title: string,
            description: string,
            beltLevel: string,
          ) => Promise<SigmaProject>;
          SigmaListProjects: () => Promise<SigmaProject[]>;
          SigmaGetProject: (id: string) => Promise<SigmaProject>;
          SigmaSaveCharter: (c: SigmaCharter) => Promise<void>;
          SigmaGetCharter: (projectID: string) => Promise<SigmaCharter>;
          SigmaAdvancePhase: (projectID: string, phase: string) => Promise<void>;
          SigmaCalculateDescriptive: (values: number[]) => Promise<DescriptiveResult>;
          SigmaCalculateCapability: (
            values: number[],
            usl: number,
            lsl: number,
          ) => Promise<CapabilityResult>;
          SigmaCalculatePareto: (
            categories: string[],
            counts: number[],
          ) => Promise<ParetoItem[]>;
          SigmaCheckReadiness: (projectID: string, phase: string) => Promise<TollgateResult>;
          SigmaSaveFishbone: (projectID: string, fb: FishboneData) => Promise<void>;
          SigmaGetFishbone: (projectID: string) => Promise<FishboneData>;
          SigmaSaveSolutions: (
            projectID: string,
            solutions: SigmaSolution[],
          ) => Promise<void>;
          SigmaGetSolutions: (projectID: string) => Promise<SigmaSolution[]>;
          SigmaSaveControlPlan: (
            projectID: string,
            items: SigmaControlPlanItem[],
          ) => Promise<void>;
          SigmaGetControlPlan: (projectID: string) => Promise<SigmaControlPlanItem[]>;
          SigmaSaveSIPOC: (projectID: string, data: SIPOCData) => Promise<void>;
          SigmaGetSIPOC: (projectID: string) => Promise<SIPOCData>;
          SigmaGetToolStatus: (projectID: string, phase: string) => Promise<PhaseTools>;
          SigmaExportProjectReport: (projectID: string) => Promise<string>;
          SigmaSaveVoC: (projectID: string, data: VoCData) => Promise<void>;
          SigmaGetVoC: (projectID: string) => Promise<VoCData>;

          // ----- Fonts -----
          ListFonts: () => Promise<FontFamilyInfo[]>;
          ImportFont: () => Promise<FontFamilyInfo>;
          GetDefaultFont: () => Promise<string>;
          SetDefaultFont: (family: string) => Promise<void>;

          // ----- Diagnostics -----
          OpenLogsFolder: () => Promise<void>;
          GenerateBugReport: () => Promise<string>;
        };
      };
    };
  }

  interface FontFamilyInfo {
    name: string;
    category: string;     // "sans" | "serif" | "mono" | "user"
    description: string;
    license: string;
    origin: string;       // "bundled" | "user"
    styles: string[];     // e.g. ["Regular", "Bold", "Italic", "Bold Italic"]
  }

  interface UpdateStatus {
    configured: boolean;
    current: string;
    latest?: string;
    update_available: boolean;
    release_notes?: string;
    download_url?: string;
    error?: string;
  }

  // ----- V2.x: Foundation-Slice types -----

  interface SeedReceipt {
    seed: string;
    kind: 'chart' | 'document' | 'board' | 'sprint';
    id: string;
    name: string;
  }

  type StakeholderCategory = 'team' | 'vendor' | 'sponsor' | 'external';

  interface Stakeholder {
    id: string;
    project_id: string;
    name: string;
    role: string;
    organisation: string;
    email: string;
    phone: string;
    category: StakeholderCategory;
    availability: number;
    hourly_rate: number;
    hourly_rate_minor_units?: number;
    contract_value: number;
    contract_value_minor_units?: number;
    notes: string;
    created_at: string;
    updated_at: string;
  }

  interface ResourceCalendar {
    id: string;
    project_id: string;
    resource: string;
    name: string;
    default_capacity: number;
    weekly_capacity: Record<number, number>;
    overrides: Record<number, number>;
    skill_tags: string[];
    notes: Record<number, string>;
    created_at: string;
    updated_at: string;
  }

  type TimelineKind =
    | 'sprint_start'
    | 'sprint_end'
    | 'deployment'
    | 'milestone'
    | 'project_start'
    | 'project_end';

  interface TimelineEntry {
    kind: TimelineKind;
    title: string;
    date: string;
    end_date?: string;
    description?: string;
    source_id?: string;
    editable?: boolean;
    edit_field?: 'start_date' | 'end_date' | string;
  }

  interface HolidayEvent {
    date: string;
    name: string;
  }

  interface BudgetSummary {
    budget: number;
    contract_value: number;
    labour_estimate: number;
    committed: number;
    remaining: number;
    budget_minor_units: number;
    contract_value_minor_units: number;
    labour_estimate_minor_units: number;
    committed_minor_units: number;
    remaining_minor_units: number;
    by_category: Record<string, number>;
    by_category_minor_units: Record<string, number>;
  }

  interface ReportSection {
    document_id: string;
    title: string;
    description: string;
  }

  interface RepairResult {
    success: boolean;
    log: string[];
  }

  interface UserSettings {
    default_password: string;
    export_theme: 'modern' | 'classic' | 'archival';
    auto_repair: boolean;
    cert_path: string;
    signature_enabled: boolean;
    signature_method?: SignatureMethod;
    gpg_key_id?: string;
    default_font?: string;
    agile_enabled?: boolean;
    compliance_mode?: boolean;
  }

  type SignatureMethod = 'none' | 'pades' | 'gpg';

  interface SignatureExportOptions {
    method: SignatureMethod;
    cert_path: string;
    cert_password: string;
    gpg_key_id: string;
  }

  interface GnuPGExportResult {
    pdf_path: string;
    signature_path: string;
    method: string;
  }

  interface Account {
    username: string;
    display_name: string;
    data_dir: string;
    created_at: string;
    last_login: string;
    is_admin: boolean;
  }

  interface ProjectFile {
    path: string;
    name: string;
    modified: string;
  }

  interface ProjectSummary {
    path: string;
    name: string;
    status: string;
    phase: string;
    start_date: string;
    end_date: string;
    modified: string;
    charts: number;
    documents: number;
    readable: boolean;
  }

  interface PortfolioSummary {
    project_count: number;
    total_budgeted_cost: number;
    total_actual_cost: number;
    total_earned_value: number;
    total_planned_value: number;
    total_budgeted_cost_minor_units: number;
    total_actual_cost_minor_units: number;
    total_earned_value_minor_units: number;
    total_planned_value_minor_units: number;
    schedule_performance_index: number;
    cost_performance_index: number;
  }

  interface Dataset {
    columns: string[];
    rows: unknown[][];
  }

  interface AppSettings {
    default_font: string;
    default_theme: string;
    app_theme: string;
    auto_save_seconds: number;
  }

  interface AppInfo {
    version: string;
    data_location: string;
    username: string;
    settings: AppSettings;
    fonts: FontFamilyInfo[];
    logs_dir: string;
  }

  interface ProjectMeta {
    id: string;
    name: string;
    description: string;
    status: string;
    phase: string;
    start_date: string;
    end_date: string;
    budget: number;
    budget_minor_units?: number;
    owner: string;
    industry: string;
    sub_category: string;
    methodology: string;
    country_code: string;
    created_at: string;
    updated_at: string;
  }

  interface ChartDefinition {
    kind: string;
    name: string;
    engine: string;
    description: string;
    data_example: string;
  }

  interface ChartRecord {
    id: string;
    project_id: string;
    kind: string;
    title: string;
    data: string;
    config: string;
    template_id: string;
    created_at: string;
    updated_at: string;
  }

  interface ChartLayoutResult {
    engine: string;
    kind: string;
    title: string;
    body: unknown;
  }

  interface DocumentDefinition {
    kind: string;
    name: string;
    phase: string;
    description: string;
    fields: DocumentField[];
  }

  type FieldKind =
    | 'string'
    | 'text'
    | 'number'
    | 'date'
    | 'bool'
    | 'string_array'
    | 'object_array'
    | 'chart_ref';

  interface DocumentField {
    key: string;
    label: string;
    type: FieldKind;
    help?: string;
    required?: boolean;
    object_shape?: DocumentField[];
    chart_kind?: string; // restrict ChartPicker to this kind when set
  }

  interface DocumentRecord {
    id: string;
    project_id: string;
    kind: string;
    title: string;
    content: string;
    template_id: string;
    version: number;
    status: string;
    created_at: string;
    updated_at: string;
  }

  // ----- V2.x: Agile Pack types -----

  type AgileWorkItemType = 'story' | 'bug' | 'task' | 'epic';
  type AgilePriority = 'low' | 'medium' | 'high' | 'urgent';
  type AgileSprintStatus = 'planning' | 'active' | 'complete';
  type DORAClass = 'elite' | 'high' | 'medium' | 'low' | 'unknown';

  interface AgileBoard {
    id: string;
    project_id: string;
    name: string;
    is_default: boolean;
    created_at: string;
    updated_at: string;
  }

  interface AgileColumn {
    id: string;
    board_id: string;
    name: string;
    order_idx: number;
    wip_limit: number;
  }

  interface AgileWorkItem {
    id: string;
    project_id: string;
    type: AgileWorkItemType;
    title: string;
    description: string;
    state: string;       // column ID, or "backlog"
    points: number;
    assignee: string;
    sprint_id: string;
    priority: AgilePriority;
    order_idx: number;
    created_at: string;
    updated_at: string;
    closed_at?: string;
  }

  interface AgileSprint {
    id: string;
    project_id: string;
    name: string;
    goal: string;
    status: AgileSprintStatus;
    start_date: string;
    end_date: string;
    capacity: number;
    created_at: string;
  }

  interface AgileDeployment {
    id: string;
    project_id: string;
    ts: string;
    version: string;
    successful: boolean;
    lead_time_hours: number;
    restore_time_hours: number;
    notes: string;
  }

  interface DORAMetric {
    value: number;
    class: DORAClass;
    label: string;
    caption: string;
  }

  interface DORADailyPoint {
    date: string;
    count: number;
  }

  interface DORAResult {
    window_days: number;
    from: string;
    to: string;
    total_deploys: number;
    successful_deploys: number;
    failed_deploys: number;
    deploy_frequency: DORAMetric;
    lead_time: DORAMetric;
    change_failure_rate: DORAMetric;
    mttr: DORAMetric;
    daily_deploy_trend: DORADailyPoint[];
  }

  interface KernelTask {
    id: string;
    title: string;
    duration: number;
    duration_estimate?: DurationEstimate;
    precedents: string[];
    es: number;
    ef: number;
    ls: number;
    lf: number;
    float: number;
    is_critical: boolean;
    /** Calendar-anchored dates (YYYY-MM-DD), present once the backend
     *  has applied kernel.AnchorSchedule; empty/undefined otherwise. */
    start_date?: string;
    finish_date?: string;
    constraint?: string;
    constraint_date?: string;
    constraint_violated?: boolean;
    percent_complete?: number;
    milestone?: boolean;
    actual_start?: string;
    actual_finish?: string;
    budgeted_cost?: number;
    budgeted_cost_minor_units?: number;
    actual_cost?: number;
    actual_cost_minor_units?: number;
    assignments?: ResourceAssignment[];
    overallocated?: boolean;
  }

  interface ResourceAssignment {
    resource: string;
    units?: number;
    calendar_id?: string;
    skill_tags?: string[];
    max_units?: number;
  }

  type MonteCarloDistribution = 'triangular' | 'beta-pert' | 'normal';

  interface DurationEstimate {
    optimistic?: number;
    most_likely?: number;
    pessimistic?: number;
    distribution?: MonteCarloDistribution | string;
  }

  interface LevelResult {
    pinned: number;
    unplaced_task_ids?: string[];
    unplaced_labels?: string[];
  }

  interface SimResult {
    valid: boolean;
    error?: string;
    iterations: number;
    workers: number;
    p50: number;
    p80: number;
    p90: number;
    finish_cdf: ProbabilityPoint[];
    critical_path_frequency: Record<string, number>;
    duration_percentiles: Record<string, [number, number, number]>;
    tornado_drivers: TornadoDriver[];
  }

  interface ProbabilityPoint {
    day: number;
    probability: number;
  }

  interface TornadoDriver {
    task_id: string;
    critical_frequency: number;
    p50_duration: number;
    p80_duration: number;
    p90_duration: number;
    duration_spread: number;
    score: number;
  }

  interface BaselineRecord {
    id: string;
    project_id: string;
    chart_id: string;
    name: string;
    data: string;
    created_at: string;
  }

  interface Scenario {
    id: string;
    project_id: string;
    name: string;
    source_baseline_id: string;
    description: string;
    is_active: boolean;
    created_at: string;
    updated_at: string;
  }

  interface ScenarioChart {
    id: string;
    scenario_id: string;
    project_id: string;
    source_chart_id: string;
    source_baseline_id: string;
    kind: string;
    title: string;
    data: string;
    config: string;
    baseline_data: string;
    created_at: string;
    updated_at: string;
  }

  interface ScheduleVariance {
    task_id: string;
    baseline_start?: string;
    baseline_finish?: string;
    start_var_days: number;
    finish_var_days: number;
  }

  interface TaskEV {
    task_id: string;
    title: string;
    bac: number;
    pv: number;
    ev: number;
    ac: number;
    bac_minor_units: number;
    pv_minor_units: number;
    ev_minor_units: number;
    ac_minor_units: number;
  }

  interface EVMetrics {
    as_of_day: number;
    bac: number;
    pv: number;
    ev: number;
    ac: number;
    bac_minor_units: number;
    pv_minor_units: number;
    ev_minor_units: number;
    ac_minor_units: number;
    sv: number;
    cv: number;
    spi: number;
    cpi: number;
    eac: number;
    etc: number;
    vac: number;
    sv_minor_units: number;
    cv_minor_units: number;
    eac_minor_units: number;
    etc_minor_units: number;
    vac_minor_units: number;
    tasks: TaskEV[];
  }

  interface CTQ {
    customer_need: string;
    ctq: string;
    lower_spec: number;
    upper_spec: number;
  }

  interface VoCEntry {
    id: string;
    customer_need: string;
    ctq: string;
    lower_spec: number;
    upper_spec: number;
    measurement: string;
    data_collection: string;
    priority: number;
    source: string;
  }

  interface VoCData {
    project_id: string;
    entries: VoCEntry[];
  }

  type SigmaPhase = 'define' | 'measure' | 'analyze' | 'improve' | 'control';
  type SigmaProjectStatus = 'active' | 'on_hold' | 'complete';
  type SigmaBeltLevel = 'green' | 'black' | 'master';

  interface SigmaProject {
    id: string;
    title: string;
    description: string;
    belt_level: SigmaBeltLevel;
    phase: SigmaPhase;
    status: SigmaProjectStatus;
    sponsor: string;
    process_owner: string;
    belt_lead: string;
    created_at: string;
    updated_at: string;
  }

  interface SigmaCharter {
    id: string;
    project_id: string;
    problem_statement: string;
    business_case: string;
    goal_statement: string;
    scope_in: string[];
    scope_out: string[];
    ctqs: CTQ[];
    sponsor: string;
    updated_at: string;
  }

  interface DescriptiveResult {
    mean: number;
    median: number;
    std_dev: number;
    min: number;
    max: number;
    count: number;
  }

  interface CapabilityResult {
    cp: number;
    cpk: number;
    pp: number;
    ppk: number;
    sigma_level: number;
    dpmo: number;
  }

  interface ParetoItem {
    category: string;
    count: number;
    percentage: number;
    cumulative_percentage: number;
  }

  interface TollgateCheck {
    name: string;
    passed: boolean;
    message: string;
  }

  interface TollgateResult {
    score: number;
    can_advance: boolean;
    checks: TollgateCheck[];
    missing_list: string;
  }

  interface FishboneData {
    problem_statement: string;
    branches: FishboneBranch[];
  }

  interface FishboneBranch {
    category: string;
    causes: FishboneCause[];
  }

  interface FishboneCause {
    id: string;
    description: string;
    is_root_cause: boolean;
    five_whys: string[];
    evidence: string;
  }

  interface SigmaSolution {
    id: string;
    title: string;
    description: string;
    impact: number;
    effort: number;
    risk: number;
    cost: number;
    selected: boolean;
    status: string;
  }

  interface SigmaControlPlanItem {
    id: string;
    process_step: string;
    metric: string;
    specification: string;
    measurement_method: string;
    frequency: string;
    owner: string;
    response_plan: string;
  }

  interface SIPOCElement {
    id: string;
    category: string;
    description: string;
    owner: string;
    requirements: string;
    order: number;
  }

  interface SIPOCData {
    project_id: string;
    process_name: string;
    process_scope: string;
    start_trigger: string;
    end_trigger: string;
    elements: SIPOCElement[];
  }

  interface ToolStatus {
    name: string;
    icon: string;
    status: 'completed' | 'active' | 'not_started' | string;
  }

  interface PhaseTools {
    phase: string;
    tools: ToolStatus[];
  }
}
