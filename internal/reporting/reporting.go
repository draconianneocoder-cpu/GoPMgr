// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package reporting owns application-level combined-report preflight, PDF
// assembly, and provenance output. It keeps Wails handlers thin while the
// documents package remains persistence-independent.
package reporting

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopmgr/internal/db"
	"gopmgr/internal/documents"
	"gopmgr/internal/exportfs"
	"gopmgr/internal/kernel"
	"gopmgr/internal/schedaudit"
)

// Options controls the profile and quality mode used for a combined report.
type Options struct {
	ProfileID string
	Mode      documents.ReportMode
}

// EVMResolver supplies optional schedule metrics for referenced charts. The
// App owns the chart-to-kernel adaptation, so this package does not couple to
// the Wails layer's scheduling helpers. An error prevents report output.
type EVMResolver func(db.Project, map[string]documents.ResolvedChart, time.Time) (map[string]*kernel.EVMetrics, error)

// AuditResolution is one schedule audit run for a report, with the inputs it
// read. The App runs the audit, so this package stays free of the chart
// adaptation; the fields bind the result to what it was computed from.
type AuditResolution struct {
	Result     schedaudit.Result
	ChartTitle string
	// ChartSHA256 is sha256(data + "\n" + config) of the audited chart, the
	// digest the manifest also records for linked charts.
	ChartSHA256 string
	// ProjectStart is the start date the audit used (YYYY-MM-DD), or "".
	ProjectStart string
	// CountryCode names the work calendar the audit used.
	CountryCode string
	// CapacitySHA256 is sha256 of the JSON of the capacity plan SQ-16 used.
	CapacitySHA256 string
	// BaselineIDs lists the chart's baselines. The audit uses only whether
	// one exists (SQ-18); the IDs say which existed.
	BaselineIDs []string
}

// AuditResolver runs the schedule audit a report section names. An error
// means the section cannot be produced; the report never omits it silently.
type AuditResolver func(context.Context, documents.ScheduleAuditSection) (AuditResolution, error)

// Service runs combined-report workflows against one already-open project
// database. Database is required; Now and ResolveEVM are optional seams for
// deterministic tests and the schedule-aware report integration.
// ResolveAudit runs schedule-audit sections; without it such a section is an
// error. AppVersion is recorded with each audit, because the audit engine
// version does not cover changes in the scheduler or calendars it uses.
type Service struct {
	Database     *db.Database
	Now          func() time.Time
	ResolveEVM   EVMResolver
	ResolveAudit AuditResolver
	AppVersion   string
}

// ExportRequest contains the application-owned values needed to write a
// combined report. FileStem must already be sanitized by the caller's input
// boundary and may not contain either path separator.
type ExportRequest struct {
	ReportTitle string
	Subtitle    string
	Sections    []documents.ReportSection
	Options     Options
	Author      string
	ExportsDir  string
	FileStem    string
	// OutputPath, when non-empty, is the caller-selected primary PDF path.
	// It must be a new file; the caller is responsible for paired-artifact
	// collision policy before Export runs.
	OutputPath string
	// AddSignatureBlock draws the signature box a PAdES signature fills.
	AddSignatureBlock bool
	// Finalize, when set, is the last change to the PDF: it receives the
	// rendered bytes, after PDF/A has been applied, and returns the bytes
	// that are written (a PAdES signature). Its error stops the export
	// before anything is written.
	Finalize func([]byte) ([]byte, error)
}

type provenanceArtifact struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Version   int    `json:"version,omitempty"`
	Status    string `json:"status,omitempty"`
	UpdatedAt string `json:"updated_at"`
	SHA256    string `json:"sha256"`
	Data      string `json:"data,omitempty"`
	Config    string `json:"config,omitempty"`
}

// auditProvenance records one schedule-audit section: what was audited, the
// inputs it read and a digest of the result printed in the report.
// ResultSHA256 is sha256 of encoding/json's encoding of schedaudit.Result
// (fields in declaration order).
type auditProvenance struct {
	SectionIndex   int            `json:"section_index"`
	ChartID        string         `json:"chart_id"`
	ChartTitle     string         `json:"chart_title"`
	Revision       string         `json:"revision"`
	ChartSHA256    string         `json:"chart_sha256"`
	ProjectStart   string         `json:"project_start"`
	CountryCode    string         `json:"country_code"`
	CapacitySHA256 string         `json:"capacity_sha256"`
	BaselineIDs    []string       `json:"baseline_ids"`
	Profile        string         `json:"profile"`
	ReportingDate  string         `json:"reporting_date"`
	EngineVersion  string         `json:"engine_version"`
	AppVersion     string         `json:"app_version"`
	Counts         map[string]int `json:"counts"`
	NotEvaluated   []string       `json:"not_evaluated"`
	ResultSHA256   string         `json:"result_sha256"`
}

// auditOutcome is the single resolution of one audit section in a request.
type auditOutcome struct {
	resolution AuditResolution
	err        error
}

type provenanceManifest struct {
	Format      string                  `json:"format"`
	GeneratedAt string                  `json:"generated_at"`
	ReportTitle string                  `json:"report_title"`
	Profile     documents.ReportProfile `json:"profile"`
	Mode        documents.ReportMode    `json:"mode"`
	Issues      []documents.ReportIssue `json:"issues"`
	Documents   []provenanceArtifact    `json:"documents"`
	Charts      []provenanceArtifact    `json:"charts"`
	// Audits is additive within format v1: existing keys keep their meaning.
	Audits []auditProvenance `json:"audits,omitempty"`
}

// Preflight exposes report-quality findings before export. A custom selection
// intentionally has no required-kind policy so users can tailor a report.
func (s Service) Preflight(sections []documents.ReportSection, options Options) (documents.ReportPreflight, error) {
	if s.Database == nil {
		return documents.ReportPreflight{}, errors.New("no project open")
	}
	return s.preflight(sections, options, s.resolveAudits(sections))
}

// sectionSourceError reports a section that names both or neither of its two
// possible sources.
func sectionSourceError(section documents.ReportSection) error {
	hasDocument := section.DocumentID != ""
	hasAudit := section.ScheduleAudit != nil
	switch {
	case hasDocument && hasAudit:
		return errors.New("names both a document and a schedule audit")
	case !hasDocument && !hasAudit:
		return errors.New("names neither a document nor a schedule audit")
	}
	return nil
}

// resolveAudits runs each schedule-audit section once. Preflight issues, the
// PDF and the manifest of one export all use these results.
func (s Service) resolveAudits(sections []documents.ReportSection) map[int]auditOutcome {
	out := make(map[int]auditOutcome)
	for i, section := range sections {
		if section.ScheduleAudit == nil || sectionSourceError(section) != nil {
			continue
		}
		if s.ResolveAudit == nil {
			out[i] = auditOutcome{err: errors.New("schedule audits are not available here")}
			continue
		}
		// The export entry points carry no cancellable context; the audit
		// cannot be cancelled during an export (recorded limit, MEM-07).
		resolution, err := s.ResolveAudit(context.Background(), *section.ScheduleAudit)
		out[i] = auditOutcome{resolution: resolution, err: err}
	}
	return out
}

func (s Service) preflight(sections []documents.ReportSection, options Options, audits map[int]auditOutcome) (documents.ReportPreflight, error) {
	project, err := s.Database.GetProject()
	if err != nil {
		return documents.ReportPreflight{}, err
	}
	profile := documents.ReportProfileFor(options.ProfileID, project.Industry)
	inputs := make([]documents.ReportInput, 0, len(sections))
	issues := make([]documents.ReportIssue, 0)
	referencedKinds := make(map[string]bool)
	for i, section := range sections {
		if err := sectionSourceError(section); err != nil {
			issues = append(issues, documents.ReportIssue{Severity: "error", Code: "section_source_invalid", Message: fmt.Sprintf("Report section %d %v.", i+1, err)})
			continue
		}
		if section.ScheduleAudit != nil {
			issues = append(issues, auditIssues(section, audits[i], options.Mode)...)
			continue
		}
		document, err := s.documentForProject(project.ID, section.DocumentID)
		if err != nil {
			issues = append(issues, documents.ReportIssue{Severity: "error", Code: "document_missing", Message: "Selected report document is unavailable.", EntityID: section.DocumentID})
			continue
		}
		inputs = append(inputs, documents.ReportInput{ID: document.ID, Kind: documents.Kind(document.Kind), Status: document.Status})
		for _, chartID := range ChartReferences(document.Content, documents.EffectiveFields(documents.Kind(document.Kind))) {
			chart, err := s.chartForProject(project.ID, chartID)
			if err != nil {
				issues = append(issues, documents.ReportIssue{Severity: "error", Code: "linked_chart_missing", Message: "A linked chart cannot be resolved and will not be silently omitted.", EntityID: chartID})
				continue
			}
			referencedKinds[chart.Kind] = true
		}
	}
	preflight := documents.Preflight(profile, options.Mode, inputs)
	for _, kind := range profile.RecommendedChartKinds {
		if !referencedKinds[kind] {
			preflight.Issues = append(preflight.Issues, documents.ReportIssue{Severity: "warning", Code: "recommended_chart_missing", Message: "Recommended profile chart is not linked in the report: " + kind})
		}
	}
	preflight.Issues = append(preflight.Issues, issues...)
	if options.Mode == documents.ReportModeCertified {
		for _, issue := range preflight.Issues {
			if issue.Severity == "error" {
				preflight.Ready = false
				break
			}
		}
	}
	return preflight, nil
}

// auditIssues are the report-quality issues of one audit section. Findings
// are the section's content and never change readiness; an audit that cannot
// run, an audit in a certified report, and rules that were not evaluated do.
func auditIssues(section documents.ReportSection, outcome auditOutcome, mode documents.ReportMode) []documents.ReportIssue {
	chartID := section.ScheduleAudit.ChartID
	if outcome.err != nil {
		return []documents.ReportIssue{{Severity: "error", Code: "schedule_audit_unavailable", Message: "The schedule audit cannot run: " + outcome.err.Error(), EntityID: chartID}}
	}
	var issues []documents.ReportIssue
	if mode == documents.ReportModeCertified {
		issues = append(issues, documents.ReportIssue{Severity: "error", Code: "schedule_audit_not_certifiable", Message: "A schedule audit is live, unapproved content and cannot be part of a certified report.", EntityID: chartID})
	}
	if skipped := notEvaluated(outcome.resolution.Result); len(skipped) > 0 {
		issues = append(issues, documents.ReportIssue{Severity: "warning", Code: "schedule_audit_rules_not_evaluated", Message: "Schedule audit rules not evaluated: " + strings.Join(skipped, ", "), EntityID: chartID})
	}
	return issues
}

func notEvaluated(r schedaudit.Result) []string {
	var out []string
	for _, rule := range r.Rules {
		if !rule.Evaluated {
			out = append(out, string(rule.Rule))
		}
	}
	return out
}

// Export assembles the report and writes a sibling provenance manifest. The
// manifest failure contract intentionally matches the prior App workflow: a
// written PDF is retained while the manifest error is returned to the caller.
func (s Service) Export(request ExportRequest) (string, error) {
	if s.Database == nil {
		return "", errors.New("no project open")
	}
	if len(request.Sections) == 0 {
		return "", errors.New("report has no sections")
	}
	if request.ExportsDir == "" {
		return "", errors.New("report exports directory is required")
	}
	if strings.ContainsAny(request.FileStem, `/\\`) {
		return "", errors.New("report output name must not contain path separators")
	}

	project, err := s.Database.GetProject()
	if err != nil {
		return "", err
	}
	audits := s.resolveAudits(request.Sections)
	preflight, err := s.preflight(request.Sections, request.Options, audits)
	if err != nil {
		return "", err
	}
	if request.Options.Mode == documents.ReportModeCertified && !preflight.Ready {
		return "", errors.New("certified report preflight failed; resolve the listed quality findings")
	}

	sections, documentManifest, auditManifest, chartIDs, err := s.resolveSections(project.ID, request.Sections, audits)
	if err != nil {
		return "", err
	}
	charts, chartManifest := s.resolveCharts(project.ID, chartIDs)
	resolvedEVM, err := s.resolveEVM(project, charts)
	if err != nil {
		return "", fmt.Errorf("resolve report EVM: %w", err)
	}
	bytes, err := documents.BuildCombinedReport(documents.ReportSpec{
		ReportTitle:    request.ReportTitle,
		Subtitle:       request.Subtitle,
		Author:         request.Author,
		ProjectName:    project.Name,
		Currency:       project.CurrencyCode,
		Sections:       request.Sections,
		ResolvedCharts: charts,
		ResolvedEVM:    resolvedEVM,
		Profile:        preflight.Profile,
		Mode:           preflight.Mode,
		QualityIssues:  preflight.Issues,
		// The box a PAdES signature fills, when the caller signs.
		AddSignatureBlock: request.AddSignatureBlock,
	}, sections)
	if err != nil {
		return "", err
	}
	if request.Finalize != nil {
		if bytes, err = request.Finalize(bytes); err != nil {
			return "", err
		}
	}

	if err := os.MkdirAll(request.ExportsDir, 0o700); err != nil {
		return "", err
	}
	stamp := s.now().UTC().Format("20060102-150405")
	outputPath := request.OutputPath
	if outputPath == "" {
		outputPath = filepath.Join(request.ExportsDir, fmt.Sprintf("%s-%s.pdf", request.FileStem, stamp))
	}
	if err := exportfs.WriteNewPrivate(outputPath, bytes); err != nil {
		return "", err
	}
	manifest := provenanceManifest{
		Format:      "gopmgr-report-provenance/v1",
		GeneratedAt: s.now().UTC().Format(time.RFC3339Nano),
		ReportTitle: request.ReportTitle,
		Profile:     preflight.Profile,
		Mode:        preflight.Mode,
		Issues:      preflight.Issues,
		Documents:   documentManifest,
		Charts:      chartManifest,
		Audits:      auditManifest,
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode report provenance: %w", err)
	}
	if err := exportfs.WriteNewPrivate(outputPath+".manifest.json", manifestBytes); err != nil {
		return "", fmt.Errorf("write report provenance: %w", err)
	}
	return outputPath, nil
}

func (s Service) resolveSections(projectID string, sections []documents.ReportSection, audits map[int]auditOutcome) ([]documents.ResolvedSection, []provenanceArtifact, []auditProvenance, map[string]struct{}, error) {
	resolved := make([]documents.ResolvedSection, 0, len(sections))
	manifest := make([]provenanceArtifact, 0, len(sections))
	var auditManifest []auditProvenance
	chartIDs := make(map[string]struct{})
	for i, section := range sections {
		if err := sectionSourceError(section); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("section %d %w", i+1, err)
		}
		if section.ScheduleAudit != nil {
			outcome := audits[i]
			if outcome.err != nil {
				return nil, nil, nil, nil, fmt.Errorf("section %d: schedule audit: %w", i+1, outcome.err)
			}
			result := outcome.resolution.Result
			if section.Title == "" {
				section.Title = "Schedule audit: " + outcome.resolution.ChartTitle
			}
			resolved = append(resolved, documents.ResolvedSection{Section: section, Audit: &result})
			record, err := s.auditRecord(i, outcome.resolution)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			auditManifest = append(auditManifest, record)
			continue
		}
		document, err := s.documentForProject(projectID, section.DocumentID)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("section %s: %w", section.DocumentID, err)
		}
		if section.Title == "" {
			section.Title = document.Title
		}
		resolved = append(resolved, documents.ResolvedSection{
			Section: section,
			Kind:    documents.Kind(document.Kind),
			Content: document.Content,
			Version: document.Version,
			Status:  document.Status,
		})
		digest := sha256.Sum256([]byte(document.Content))
		manifest = append(manifest, provenanceArtifact{ID: document.ID, Kind: document.Kind, Title: document.Title, Version: document.Version, Status: document.Status, UpdatedAt: document.UpdatedAt, SHA256: fmt.Sprintf("%x", digest)})
		for _, chartID := range ChartReferences(document.Content, documents.EffectiveFields(documents.Kind(document.Kind))) {
			chartIDs[chartID] = struct{}{}
		}
	}
	return resolved, manifest, auditManifest, chartIDs, nil
}

// auditRecord is the manifest entry of the audit section at index i.
func (s Service) auditRecord(i int, r AuditResolution) (auditProvenance, error) {
	encoded, err := json.Marshal(r.Result)
	if err != nil {
		return auditProvenance{}, fmt.Errorf("encode schedule audit result: %w", err)
	}
	counts := map[string]int{"error": 0, "warning": 0, "info": 0}
	for _, f := range r.Result.Findings {
		counts[string(f.Severity)]++
	}
	baselines := append([]string{}, r.BaselineIDs...)
	skipped := append([]string{}, notEvaluated(r.Result)...)
	return auditProvenance{
		SectionIndex:   i,
		ChartID:        r.Result.Source.ChartID,
		ChartTitle:     r.ChartTitle,
		Revision:       r.Result.Source.Revision,
		ChartSHA256:    r.ChartSHA256,
		ProjectStart:   r.ProjectStart,
		CountryCode:    r.CountryCode,
		CapacitySHA256: r.CapacitySHA256,
		BaselineIDs:    baselines,
		Profile:        r.Result.Profile,
		ReportingDate:  r.Result.ReportingDate,
		EngineVersion:  r.Result.EngineVersion,
		AppVersion:     s.AppVersion,
		Counts:         counts,
		NotEvaluated:   skipped,
		ResultSHA256:   fmt.Sprintf("%x", sha256.Sum256(encoded)),
	}, nil
}

func (s Service) resolveCharts(projectID string, chartIDs map[string]struct{}) (map[string]documents.ResolvedChart, []provenanceArtifact) {
	resolved := make(map[string]documents.ResolvedChart, len(chartIDs))
	manifest := make([]provenanceArtifact, 0, len(chartIDs))
	for id := range chartIDs {
		chart, err := s.chartForProject(projectID, id)
		if err != nil {
			// Preflight records unresolved chart references as explicit findings.
			continue
		}
		resolved[id] = documents.ResolvedChart{Kind: chart.Kind, Title: chart.Title, Data: chart.Data}
		digest := sha256.Sum256([]byte(chart.Data + "\n" + chart.Config))
		manifest = append(manifest, provenanceArtifact{ID: chart.ID, Kind: chart.Kind, Title: chart.Title, UpdatedAt: chart.UpdatedAt, SHA256: fmt.Sprintf("%x", digest), Data: chart.Data, Config: chart.Config})
	}
	return resolved, manifest
}

func (s Service) documentForProject(projectID, documentID string) (db.Document, error) {
	document, err := s.Database.GetDocument(documentID)
	if err != nil {
		return db.Document{}, err
	}
	if document.ProjectID != projectID {
		return db.Document{}, db.ErrNoDocument
	}
	return document, nil
}

func (s Service) chartForProject(projectID, chartID string) (db.Chart, error) {
	chart, err := s.Database.GetChart(chartID)
	if err != nil {
		return db.Chart{}, err
	}
	if chart.ProjectID != projectID {
		return db.Chart{}, db.ErrNoChart
	}
	return chart, nil
}

func (s Service) resolveEVM(project db.Project, charts map[string]documents.ResolvedChart) (map[string]*kernel.EVMetrics, error) {
	if s.ResolveEVM == nil {
		return nil, nil
	}
	return s.ResolveEVM(project, charts, s.now().UTC())
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// ChartReferences scans a document JSON payload for declared chart-reference
// fields. Invalid content has no usable references and is handled by the
// document renderer's existing validation path.
func ChartReferences(contentJSON string, fields []documents.Field) []string {
	if contentJSON == "" || len(fields) == 0 {
		return nil
	}
	var content map[string]interface{}
	if err := json.Unmarshal([]byte(contentJSON), &content); err != nil {
		return nil
	}
	var references []string
	for _, field := range fields {
		if field.Type != documents.FieldChartRef {
			continue
		}
		if id, ok := content[field.Key].(string); ok && id != "" {
			references = append(references, id)
		}
	}
	return references
}
