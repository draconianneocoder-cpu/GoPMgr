// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package reporting owns application-level combined-report preflight, PDF
// assembly, and provenance output. It keeps Wails handlers thin while the
// documents package remains persistence-independent.
package reporting

import (
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
	"gopmgr/internal/kernel"
)

// Options controls the profile and quality mode used for a combined report.
type Options struct {
	ProfileID string
	Mode      documents.ReportMode
}

// EVMResolver supplies optional schedule metrics for referenced charts. The
// App owns the chart-to-kernel adaptation, so this package does not couple to
// the Wails layer's scheduling helpers.
type EVMResolver func(db.Project, map[string]documents.ResolvedChart, time.Time) map[string]*kernel.EVMetrics

// Service runs combined-report workflows against one already-open project
// database. Database is required; Now and ResolveEVM are optional seams for
// deterministic tests and the schedule-aware report integration.
type Service struct {
	Database   *db.Database
	Now        func() time.Time
	ResolveEVM EVMResolver
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

type provenanceManifest struct {
	Format      string                  `json:"format"`
	GeneratedAt string                  `json:"generated_at"`
	ReportTitle string                  `json:"report_title"`
	Profile     documents.ReportProfile `json:"profile"`
	Mode        documents.ReportMode    `json:"mode"`
	Issues      []documents.ReportIssue `json:"issues"`
	Documents   []provenanceArtifact    `json:"documents"`
	Charts      []provenanceArtifact    `json:"charts"`
}

// Preflight exposes report-quality findings before export. A custom selection
// intentionally has no required-kind policy so users can tailor a report.
func (s Service) Preflight(sections []documents.ReportSection, options Options) (documents.ReportPreflight, error) {
	if s.Database == nil {
		return documents.ReportPreflight{}, errors.New("no project open")
	}
	project, err := s.Database.GetProject()
	if err != nil {
		return documents.ReportPreflight{}, err
	}
	profile := documents.ReportProfileFor(options.ProfileID, project.Industry)
	inputs := make([]documents.ReportInput, 0, len(sections))
	issues := make([]documents.ReportIssue, 0)
	referencedKinds := make(map[string]bool)
	for _, section := range sections {
		document, err := s.Database.GetDocument(section.DocumentID)
		if err != nil {
			issues = append(issues, documents.ReportIssue{Severity: "error", Code: "document_missing", Message: "Selected report document is unavailable.", EntityID: section.DocumentID})
			continue
		}
		inputs = append(inputs, documents.ReportInput{ID: document.ID, Kind: documents.Kind(document.Kind), Status: document.Status})
		for _, chartID := range ChartReferences(document.Content, documents.EffectiveFields(documents.Kind(document.Kind))) {
			chart, err := s.Database.GetChart(chartID)
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
	preflight, err := s.Preflight(request.Sections, request.Options)
	if err != nil {
		return "", err
	}
	if request.Options.Mode == documents.ReportModeCertified && !preflight.Ready {
		return "", errors.New("certified report preflight failed; resolve the listed quality findings")
	}

	sections, documentManifest, chartIDs, err := s.resolveSections(request.Sections)
	if err != nil {
		return "", err
	}
	charts, chartManifest := s.resolveCharts(chartIDs)
	bytes, err := documents.BuildCombinedReport(documents.ReportSpec{
		ReportTitle:    request.ReportTitle,
		Subtitle:       request.Subtitle,
		Author:         request.Author,
		ProjectName:    project.Name,
		Sections:       request.Sections,
		ResolvedCharts: charts,
		ResolvedEVM:    s.resolveEVM(project, charts),
		Profile:        preflight.Profile,
		Mode:           preflight.Mode,
		QualityIssues:  preflight.Issues,
	}, sections)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(request.ExportsDir, 0o700); err != nil {
		return "", err
	}
	stamp := s.now().UTC().Format("20060102-150405")
	outputPath := filepath.Join(request.ExportsDir, fmt.Sprintf("%s-%s.pdf", request.FileStem, stamp))
	if err := os.WriteFile(outputPath, bytes, 0o600); err != nil {
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
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode report provenance: %w", err)
	}
	if err := os.WriteFile(outputPath+".manifest.json", manifestBytes, 0o600); err != nil {
		return "", fmt.Errorf("write report provenance: %w", err)
	}
	return outputPath, nil
}

func (s Service) resolveSections(sections []documents.ReportSection) ([]documents.ResolvedSection, []provenanceArtifact, map[string]struct{}, error) {
	resolved := make([]documents.ResolvedSection, 0, len(sections))
	manifest := make([]provenanceArtifact, 0, len(sections))
	chartIDs := make(map[string]struct{})
	for _, section := range sections {
		document, err := s.Database.GetDocument(section.DocumentID)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("section %s: %w", section.DocumentID, err)
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
	return resolved, manifest, chartIDs, nil
}

func (s Service) resolveCharts(chartIDs map[string]struct{}) (map[string]documents.ResolvedChart, []provenanceArtifact) {
	resolved := make(map[string]documents.ResolvedChart, len(chartIDs))
	manifest := make([]provenanceArtifact, 0, len(chartIDs))
	for id := range chartIDs {
		chart, err := s.Database.GetChart(id)
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

func (s Service) resolveEVM(project db.Project, charts map[string]documents.ResolvedChart) map[string]*kernel.EVMetrics {
	if s.ResolveEVM == nil {
		return nil
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
