// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Frontend domain and editor view types. Transport calls are typed by the
// generated Wails v3 bindings through lib/desktop-api.ts.

export {};

declare global {
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
    sha256?: string;
    channel: string;
    error?: string;
    platform: string;
  }

  // ----- V2.x: Foundation-Slice types -----

  interface SeedReceipt {
    seed: string;
    kind: 'chart' | 'document' | 'board' | 'sprint';
    id: string;
    name: string;
  }

  type StakeholderCategory = `${import('../bindings/gopmgr/internal/db/models').StakeholderCategory}`;

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

  type TimelineKind = `${import('../bindings/gopmgr/internal/timeline/models').EntryKind}`;

  interface TimelineEntry {
    kind: TimelineKind;
    title: string;
    date: string;
    end_date?: string;
    description?: string;
    source_id?: string;
    editable?: boolean;
    edit_field?: 'start_date' | 'end_date' | string;
    // Only meaningful when kind === 'milestone': distinguishes a Charter
    // document milestone from a scheduled Gantt/CPM chart milestone.
    milestone_source?: import('../bindings/gopmgr/internal/timeline/models').Entry['milestone_source'];
  }

  interface HolidayEvent {
    date: string;
    name: string;
  }

  interface BudgetSummary {
    budget: string;
    contract_value: string;
    labour_estimate: string;
    committed: string;
    remaining: string;
    by_category: Record<string, string>;
  }

  interface ReportSection {
    document_id: string;
    title: string;
    description: string;
  }

  type ReportMode = `${import('../bindings/gopmgr/internal/documents/models').ReportMode}`;

  interface CombinedReportOptions {
    profile_id: string;
    mode: ReportMode;
  }

  interface ReportProfile {
    id: string;
    name: string;
    industry: string;
    reference?: string;
    required_document_kinds: string[];
    recommended_document_kinds: string[];
    recommended_chart_kinds: string[];
    customizable: boolean;
  }

  interface ReportIssue {
    severity: import('../bindings/gopmgr/internal/documents/models').ReportIssue['severity'];
    code: string;
    message: string;
    entity_id?: string;
  }

  interface ReportPreflight {
    profile: ReportProfile;
    mode: ReportMode;
    issues: ReportIssue[];
    ready: boolean;
  }

  interface MSPDIImportOptions {
    include_dependencies: boolean;
    include_progress: boolean;
    include_assignments: boolean;
  }

  interface OpenProjectCheck {
    checked: boolean;
    damaged: boolean;
    dismissed: boolean;
  }

  interface RepairResult {
    success: boolean;
    log: string[];
    snapshot?: string;
    swapped: boolean;
    damaged_copy?: string;
  }

  interface UserSettings {
    default_password: string;
    export_theme: 'modern' | 'classic' | 'archival';
    auto_repair: boolean;
    cert_path: string;
    signature_enabled: boolean;
    signature_method?: SignatureMethod;
    gpg_key_id?: string;
    timestamp_enabled?: boolean;
    tsa_endpoint?: string;
    tsa_policy_oid?: string;
    tsa_root_cert_path?: string;
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
    disabled: boolean;
  }

  // One entry in the account history (users.AccountEvent).
  interface AccountEvent {
    id: number;
    occurred_at: string;
    actor: string;
    username: string;
    action: import('../bindings/gopmgr/internal/users/models').AccountEvent['action'];
    detail: string;
  }

  // The signed-in user's recovery codes (RecoveryCodeStatusWire).
  interface RecoveryCodeStatus {
    unused: number;
    total: number;
    legacy: boolean;
    encryption_ready: boolean;
  }

  // First-run state for the sign-in screen (AccountSetupWire).
  interface AccountSetup {
    has_accounts: boolean;
    has_admin: boolean;
  }

  interface ProjectFile {
    path: string;
    name: string;
    modified: string;
  }

  // A project an administrator views read-only (AdminViewProjectWire).
  // audit_warning is '' unless a compliance-mode project's audit trail
  // fails its check.
  interface AdminViewProject {
    project: ProjectMeta;
    audit_warning: string;
  }

  // One schedule (Gantt or CPM chart) in the viewer. note explains a
  // schedule whose data could not be read; its tasks are then empty.
  interface AdminViewSchedule {
    chart_id: string;
    title: string;
    kind: string;
    tasks: AdminViewTask[];
    note: string;
  }

  interface AdminViewTask {
    id: string;
    title: string;
    start_date: string;
    finish_date: string;
    duration: number;
    percent_complete: number;
    milestone: boolean;
    critical: boolean;
  }

  interface AdminViewCosts {
    entries: CostEntry[];
    baselines: CostBaseline[];
  }

  // A document in the viewer: what it is, never its contents.
  interface AdminViewDocument {
    id: string;
    kind: string;
    title: string;
    version: number;
    status: string;
    updated_at: string;
  }

  // One entry of the signed-in user's deletion log. outcome is '' when the
  // app stopped before it could record how the deletion ended; project_id is
  // '' for an empty file that held no project.
  interface ProjectDeletion {
    id: string;
    project_id: string;
    project_name: string;
    location: string;
    deleted_by: string;
    requested_at: string;
    outcome: import('../bindings/gopmgr/internal/desktop/models').ProjectDeletionWire['outcome'];
    outcome_at: string;
    detail: string;
    audit_events: number;
    audit_valid: boolean;
    audit_terminal_hash: string;
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
    evm_project_count: number;
    evm_unavailable_project_count: number;
    as_of_date: string;
    currency_code: string;
    total_budgeted_cost: string;
    total_committed_cost: string;
    total_actual_cost: string;
    total_earned_value: string;
    total_planned_value: string;
    remaining: string;
    schedule_performance_index: number;
    cost_performance_index: number;
  }

  interface Dataset {
    columns: string[];
    rows: unknown[][];
    /** True when the file holds more rows than were read (the import cap). */
    truncated: boolean;
  }

  interface AppSettings {
    default_font: string;
    default_theme: string;
    app_theme: string;
    auto_save_seconds: number;
    auto_check_updates: boolean;
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
    budget: string;
    currency_code: string;
    owner: string;
    industry: string;
    sub_category: string;
    methodology: string;
    country_code: string;
    time_zone: string;
    created_at: string;
    updated_at: string;
  }

  interface CostType {
    id: string;
    project_id: string;
    code: string;
    name: string;
    attribution: import('../bindings/gopmgr/internal/db/models').CostType['attribution'];
    behavior: import('../bindings/gopmgr/internal/db/models').CostType['behavior'];
    treatment: import('../bindings/gopmgr/internal/db/models').CostType['treatment'];
    active: boolean;
  }

  interface CostEntry {
    id: string;
    cost_type_id: string;
    kind: import('../bindings/gopmgr/internal/desktop/models').CostEntryWire['kind'];
    cost_date: string;
    description: string;
    amount: string;
    quantity: string;
    unit: string;
    item_name: string;
    sku: string;
    supplier_name: string;
    invoice_reference: string;
  }

  interface CostQuantityAggregate {
    item_name: string;
    unit: string;
    total_quantity: string;
    entry_count: number;
  }

  interface CostEntryAttachment {
    id: string;
    cost_entry_id: string;
    filename: string;
    content_type: string;
    size_bytes: number;
    sha256: string;
    created_at: string;
  }

  interface CatalogVendor {
    id: string; version: number; name: string; address: string; phone: string; fax: string;
    email: string; primary_contact: string; notes: string; archived: boolean;
    created_at: string; updated_at: string;
  }
  interface CatalogItem {
    id: string; version: number; name: string; sku: string; kind: string; default_unit: string;
    description: string; archived: boolean; created_at: string; updated_at: string;
  }

interface CostSummary {
  currency_code: string;
  mutation_disabled_reason: string;
    legacy_budget: string;
    planned: string;
    contingency: string;
    cost_baseline: string;
    management_reserve: string;
    authorised_funding: string;
    commitment: string;
    actual: string;
  }
  interface CostClassificationRow { value: string; planned: string; commitment: string; actual: string; }
  interface CostClassificationSummary {
    attribution: CostClassificationRow[];
    behavior: CostClassificationRow[];
    treatment: CostClassificationRow[];
  }
  interface CostReserve { kind: import('../bindings/gopmgr/internal/desktop/models').CostReserveWire['kind']; amount: string; description: string; }
  interface CostBaseline {
    version: number; currency_code: string; planned: string; contingency: string;
    cost_baseline: string; management_reserve: string; authorised_funding: string;
    approved_by: string; approval_note: string; approved_at: string;
  }

  interface CalendarPolicy {
    country_code: string;
    name: string;
    time_zones: string[];
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
	  time_zone?: string;
    body: unknown;
  }

  interface DocumentDefinition {
    kind: string;
    name: string;
    phase: string;
    description: string;
    fields: DocumentField[];
  }

  type FieldKind = `${import('../bindings/gopmgr/internal/documents/models').FieldKind}`;

  interface DocumentField {
    key: string;
    label: string;
    type: FieldKind;
    help?: string;
    required?: boolean;
    object_shape?: DocumentField[];
    chart_kind?: string; // restrict ChartPicker to this kind when set
    money?: boolean; // an amount in the project's reporting currency
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

  type AgileWorkItemType = `${import('../bindings/gopmgr/internal/agile/models').WorkItemType}`;
  type AgilePriority = `${import('../bindings/gopmgr/internal/agile/models').Priority}`;
  type AgileSprintStatus = `${import('../bindings/gopmgr/internal/agile/models').SprintStatus}`;
  type DORAClass = `${import('../bindings/gopmgr/internal/agile/models').DORAClass}`;

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
    split_labels?: string[];
  }

  interface SplitLevelingPreview {
    split_task_labels?: string[];
    resolves_overallocation: boolean;
    remaining_overallocated_resources?: string[];
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
    duration_percentiles: Record<string, number[]>;
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

  type SigmaPhase = `${import('../bindings/gopmgr/internal/sigma/domain/models').Phase}`;
  type SigmaProjectStatus = `${import('../bindings/gopmgr/internal/sigma/domain/models').ProjectStatus}`;
  type SigmaBeltLevel = `${import('../bindings/gopmgr/internal/sigma/domain/models').BeltLevel}`;

  interface SigmaProject {
    id: string;
    gopmgr_project_id: string;
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
