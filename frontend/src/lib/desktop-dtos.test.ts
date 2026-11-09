// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import { normaliseDocumentDefinition, normaliseResourceCalendar, normaliseFishbone, enumValue, definedEntries, signatureMethodValue, exportThemeValue, normaliseSimulation, normaliseCharter, normaliseReportPreflight, normaliseAppInfo, workItemPayload, stakeholderPayload, sprintPayload, reportOptionsPayload } from './desktop-dtos';
import { WorkItemType, Priority } from '../../bindings/gopmgr/internal/agile/models';
import { FieldKind, Kind, Phase, ReportMode } from '../../bindings/gopmgr/internal/documents/models';

describe('generated DTO boundaries', () => {
  it('renders empty and nested document fields without nullable collections', () => {
    const base = { kind: Kind.$zero, name: 'Test', phase: Phase.$zero, description: '' };
    expect(normaliseDocumentDefinition({ ...base, fields: null }).fields).toEqual([]);
    const result = normaliseDocumentDefinition({ ...base, fields: [{ key: 'rows', label: 'Rows', type: FieldKind.$zero, object_shape: null }] });
    expect(result.fields[0].object_shape).toEqual([]);
  });

  it('preserves resource capacities and omits absent map entries', () => {
    const result = normaliseResourceCalendar({ id: 'r', project_id: 'p', resource: 'Alice', name: 'Calendar', default_capacity: 0, weekly_capacity: { '1': 0, '2': undefined }, overrides: null, skill_tags: null, notes: null, created_at: '', updated_at: '' });
    expect(result.weekly_capacity).toEqual({ 1: 0 });
    expect(result.overrides).toEqual({});
    expect(result.skill_tags).toEqual([]);
    expect(result.notes).toEqual({});
  });

  it('makes empty fishbone branches and causes editable', () => {
    expect(normaliseFishbone({ problem_statement: 'Problem', branches: null }).branches).toEqual([]);
    expect(normaliseFishbone({ problem_statement: '', branches: [{ category: 'People', causes: null }] }).branches[0].causes).toEqual([]);
  });

  it('keeps zero values in generated maps and removes absent entries', () => {
    expect(definedEntries({ zero: 0, absent: undefined, positive: 2 })).toEqual({ zero: 0, positive: 2 });
    expect(definedEntries(null)).toEqual({});
  });

  it('uses the generated enum values and rejects unsupported wire values', () => {
    expect(enumValue(WorkItemType, 'story')).toBe(WorkItemType.WorkItemStory);
    expect(enumValue(Priority, 'urgent')).toBe(Priority.PrioUrgent);
    expect(() => enumValue(WorkItemType, 'unsupported')).toThrow('Unsupported desktop enum value');
  });
});


describe('editor DTO compatibility', () => {
  it('keeps scalar and timestamp fields intact while converting outgoing enums', () => {
    const item: AgileWorkItem = { id: 'i', project_id: 'p', type: 'story', title: 'Draft', description: '', state: 'backlog', points: 0, assignee: '', sprint_id: '', priority: 'urgent', order_idx: 0, created_at: '0001-01-01T00:00:00Z', updated_at: '2026-11-03T00:00:00Z', closed_at: undefined };
    expect(workItemPayload(item)).toEqual(item);
    const stakeholder: Stakeholder = { id: 's', project_id: 'p', name: 'Vendor', role: '', organisation: '', email: '', phone: '', category: 'vendor', availability: 0, hourly_rate: 0, contract_value: 0, notes: '', created_at: '', updated_at: '' };
    expect(stakeholderPayload(stakeholder)).toEqual(stakeholder);
    const sprint: AgileSprint = { id: '', project_id: 'p', name: 'Sprint', goal: '', status: 'active', start_date: '', end_date: '', capacity: 0, created_at: '0001-01-01T00:00:00Z' };
    expect(sprintPayload(sprint)).toEqual(sprint);
    expect(reportOptionsPayload({ profile_id: 'report', mode: 'certified' })).toEqual({ profile_id: 'report', mode: 'certified' });
  });

  it('preserves settings choices and the existing missing-method default', () => {
    expect(signatureMethodValue(undefined, true)).toBe('pades');
    expect(signatureMethodValue(undefined, false)).toBe('none');
    for (const value of ['none', 'pades', 'gpg']) expect(signatureMethodValue(value)).toBe(value);
    for (const value of ['modern', 'classic', 'archival']) expect(exportThemeValue(value)).toBe(value);
    expect(() => signatureMethodValue('unsupported')).toThrow('Unsupported signature method');
    expect(() => exportThemeValue('unsupported')).toThrow('Unsupported export theme');
  });

  it('keeps simulation results renderable when Go returns nil collections', () => {
    const result = normaliseSimulation({ valid: true, iterations: 0, workers: 0, p50: 0, p80: 0, p90: 0, finish_cdf: null, critical_path_frequency: null, duration_percentiles: null, tornado_drivers: null });
    expect(result).toEqual({ valid: true, iterations: 0, workers: 0, p50: 0, p80: 0, p90: 0, finish_cdf: [], critical_path_frequency: {}, duration_percentiles: {}, tornado_drivers: [] });
  });

  it('keeps empty charter lists editable without changing the incoming DTO', () => {
    const wire = { id: 'charter', project_id: 'p', problem_statement: 'Problem', business_case: '', goal_statement: '', scope_in: null, scope_out: null, ctqs: null, sponsor: '', updated_at: '' };
    const result = normaliseCharter(wire);
    expect(result.scope_in).toEqual([]);
    expect(result.scope_out).toEqual([]);
    expect(result.ctqs).toEqual([]);
    expect(wire.scope_in).toBeNull();
  });

  it('keeps report preflight lists usable without changing readiness', () => {
    const result = normaliseReportPreflight({ profile: { id: 'report', name: 'Report', industry: '', required_document_kinds: null, recommended_document_kinds: null, recommended_chart_kinds: null, customizable: false }, mode: ReportMode.ReportModeDraft, issues: null, ready: false });
    expect(result.ready).toBe(false);
    expect(result.issues).toEqual([]);
    expect(result.profile.required_document_kinds).toEqual([]);
    expect(result.profile.recommended_document_kinds).toEqual([]);
    expect(result.profile.recommended_chart_kinds).toEqual([]);
  });

  it('keeps settings usable with no fonts and no nested style list', () => {
    const base = { version: '1', data_location: '', username: 'alice', settings: { default_font: '', default_theme: 'modern', app_theme: 'dark', auto_save_seconds: 0, auto_check_updates: false }, logs_dir: '' };
    expect(normaliseAppInfo({ ...base, fonts: null }).fonts).toEqual([]);
    const result = normaliseAppInfo({ ...base, fonts: [{ name: 'Test', category: 'user', description: '', license: '', origin: 'user', styles: null }] });
    expect(result.fonts[0].styles).toEqual([]);
    expect(result.settings.auto_check_updates).toBe(false);
  });
});
