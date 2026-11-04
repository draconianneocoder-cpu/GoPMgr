// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import type * as Desktop from '../../bindings/gopmgr/internal/desktop/models';
import type * as Documents from '../../bindings/gopmgr/internal/documents/models';
import type * as Kernel from '../../bindings/gopmgr/internal/kernel/models';
import type * as Fonts from '../../bindings/gopmgr/internal/fonts/models';
import type * as DB from '../../bindings/gopmgr/internal/db/models';
import type * as Sigma from '../../bindings/gopmgr/internal/sigma/domain/models';
import { WorkItemType, Priority, SprintStatus, type WorkItem, type Sprint } from '../../bindings/gopmgr/internal/agile/models';
import { StakeholderCategory } from '../../bindings/gopmgr/internal/db/models';
import { ReportMode } from '../../bindings/gopmgr/internal/documents/models';

// Go nil collections arrive as null. Editors own arrays/maps so empty wire
// collections become empty editable collections; scalar values stay unchanged.
export function definedEntries<T>(value: Record<string, T | undefined> | null): Record<string, T> {
  return Object.fromEntries(Object.entries(value ?? {}).filter((entry): entry is [string, T] => entry[1] !== undefined));
}

export function enumValue<T extends string>(values: Record<string, T>, value: string): T {
  const result = Object.values(values).find((candidate) => candidate === value);
  if (result === undefined) throw new Error(`Unsupported desktop enum value: ${value}`);
  return result;
}

export function workItemPayload(value: AgileWorkItem): WorkItem {
  return { ...value, type: enumValue(WorkItemType, value.type), priority: enumValue(Priority, value.priority) };
}

export function sprintPayload(value: AgileSprint): Sprint {
  return { ...value, status: enumValue(SprintStatus, value.status) };
}

export function stakeholderPayload(value: Stakeholder): DB.Stakeholder {
  return { ...value, category: enumValue(StakeholderCategory, value.category) };
}

export function reportOptionsPayload(value: CombinedReportOptions): Desktop.CombinedReportOptions {
  return { ...value, mode: enumValue(ReportMode, value.mode) };
}

function normaliseDocumentField(value: Documents.Field): DocumentField {
  return { ...value, object_shape: value.object_shape?.map(normaliseDocumentField) ?? [] };
}

export function normaliseDocumentDefinition(value: Documents.Definition): DocumentDefinition {
  return { ...value, fields: (value.fields ?? []).map(normaliseDocumentField) };
}

export function normaliseResourceCalendar(value: DB.ResourceCalendar): ResourceCalendar {
  return {
    ...value,
    weekly_capacity: definedEntries(value.weekly_capacity),
    overrides: definedEntries(value.overrides),
    skill_tags: value.skill_tags ?? [],
    notes: definedEntries(value.notes),
  };
}

export function normaliseFishbone(value: Sigma.FishboneData): FishboneData {
  return {
    ...value,
    branches: (value.branches ?? []).map((branch) => ({
      ...branch,
      causes: (branch.causes ?? []).map((cause) => ({ ...cause, five_whys: cause.five_whys ?? [] })),
    })),
  };
}

export function normaliseAppInfo(value: Desktop.AppInfo): AppInfo {
  return { ...value, fonts: (value.fonts ?? []).map(normaliseFont) };
}

export function normaliseReportProfile(value: Documents.ReportProfile): ReportProfile {
  return {
    ...value,
    required_document_kinds: value.required_document_kinds ?? [],
    recommended_document_kinds: value.recommended_document_kinds ?? [],
    recommended_chart_kinds: value.recommended_chart_kinds ?? [],
  };
}

export function normaliseReportPreflight(value: Documents.ReportPreflight): ReportPreflight {
  return { ...value, profile: normaliseReportProfile(value.profile), issues: value.issues ?? [] };
}

export function normaliseCharter(value: Sigma.Charter): SigmaCharter {
  return { ...value, scope_in: value.scope_in ?? [], scope_out: value.scope_out ?? [], ctqs: value.ctqs ?? [] };
}

export function normaliseFont(value: Fonts.FamilyInfo): FontFamilyInfo {
  return { ...value, styles: value.styles ?? [] };
}

export function normaliseSimulation(value: Kernel.SimResult): SimResult {
  return {
    ...value,
    finish_cdf: value.finish_cdf ?? [],
    critical_path_frequency: definedEntries(value.critical_path_frequency),
    duration_percentiles: definedEntries(value.duration_percentiles),
    tornado_drivers: value.tornado_drivers ?? [],
  };
}

export function signatureMethodValue(value: string | undefined, enabled = false): SignatureMethod {
  if (value === undefined) return enabled ? 'pades' : 'none';
  if (value === 'none' || value === 'pades' || value === 'gpg') return value;
  throw new Error(`Unsupported signature method: ${value}`);
}

export function exportThemeValue(value: string): 'modern' | 'classic' | 'archival' {
  if (value === 'modern' || value === 'classic' || value === 'archival') return value;
  throw new Error(`Unsupported export theme: ${value}`);
}
