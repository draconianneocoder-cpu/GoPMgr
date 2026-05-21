// SPDX-FileCopyrightText: 2026 The PMForge Contributors
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
          SecureArchive: (projectPath: string) => Promise<string>;

          // ----- V2: accounts & session -----
          ListUsers: () => Promise<Account[]>;
          CreateAccount: (
            username: string,
            displayName: string,
            password: string,
          ) => Promise<Account>;
          Login: (username: string, password: string) => Promise<Account>;
          Logout: () => Promise<void>;
          CurrentUser: () => Promise<Account | null>;

          // ----- V2: projects -----
          ListProjects: () => Promise<ProjectFile[]>;
          CreateProject: (name: string, description: string) => Promise<ProjectFile>;
          OpenProject: (path: string) => Promise<ProjectMeta>;
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
          RepairAndSwap: () => Promise<RepairResult>;

          // ----- V2.x: Agile Pack -----
          AgileEnabled: () => Promise<boolean>;
          SetAgileEnabled: (enabled: boolean) => Promise<void>;
          EnsureDefaultBoard: () => Promise<[AgileBoard, AgileColumn[]]>;
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
          ) => Promise<[ProjectMeta, SeedReceipt[], string]>;
          UpdateProjectIndustry: (
            industry: string,
            subCategory: string,
            methodology: string,
            countryCode: string,
          ) => Promise<ProjectMeta>;
          ListStakeholders: (category: string) => Promise<Stakeholder[]>;
          SaveStakeholder: (s: Stakeholder) => Promise<Stakeholder>;
          DeleteStakeholder: (id: string) => Promise<void>;
          BuildTimeline: () => Promise<TimelineEntry[]>;
          ListHolidays: (fromISO: string, toISO: string) => Promise<HolidayEvent[]>;
          ComputeBudget: () => Promise<BudgetSummary>;
          ExportProjectICS: (includeHolidays: boolean) => Promise<string>;

          // ----- V2.x: Remaining-TODOs Slice -----
          ChooseCertFile: () => Promise<string>;
          IssueRecoveryCodes: () => Promise<string[]>;
          RemainingRecoveryCodes: () => Promise<number>;
          ResetWithRecoveryCode: (username: string, code: string, newPassword: string) => Promise<void>;
          CheckLatestVersion: () => Promise<UpdateStatus>;
          ExportDocumentDOCX: (id: string) => Promise<string>;
          ExportDocumentODT: (id: string) => Promise<string>;

          // ----- Fonts -----
          ListFonts: () => Promise<FontFamilyInfo[]>;
          ImportFont: () => Promise<FontFamilyInfo>;
          GetDefaultFont: () => Promise<string>;
          SetDefaultFont: (family: string) => Promise<void>;
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
    hourly_rate: number;
    contract_value: number;
    notes: string;
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
    by_category: Record<string, number>;
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
  }

  interface Account {
    username: string;
    display_name: string;
    data_dir: string;
    created_at: string;
    last_login: string;
  }

  interface ProjectFile {
    path: string;
    name: string;
    modified: string;
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
    owner: string;
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
    precedents: string[];
    es: number;
    ef: number;
    ls: number;
    lf: number;
    float: number;
    is_critical: boolean;
  }
}
