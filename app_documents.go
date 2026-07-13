// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"pmforge/internal/admin"
	"pmforge/internal/calendar"
	"pmforge/internal/charts"
	"pmforge/internal/charts/dag"
	"pmforge/internal/crypto"
	"pmforge/internal/db"
	"pmforge/internal/documents"
	"pmforge/internal/export"
	"pmforge/internal/kernel"
	"pmforge/internal/pdfmeta"
	"pmforge/internal/sigma/service"
	"pmforge/internal/signing"
	"sort"
	"strings"
	"time"
)

// =========================================================
// Documents
// =========================================================

func (a *App) ListDocumentKinds() []documents.Definition { return documents.All() }

func (a *App) ListDocuments(kind string) ([]db.Document, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	return d.ListDocuments(p.ID, kind)
}

func (a *App) GetDocument(id string) (db.Document, error) {
	d := a.requireDB()
	if d == nil {
		return db.Document{}, errors.New("no project open")
	}
	return d.GetDocument(id)
}

// NewDocument creates a fresh document with default content for the
// requested kind.
func (a *App) NewDocument(kind, title string) (db.Document, error) {
	d := a.requireDB()
	if d == nil {
		return db.Document{}, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return db.Document{}, err
	}
	def, ok := documents.Get(documents.Kind(kind))
	if !ok {
		return db.Document{}, fmt.Errorf("unknown document kind %q", kind)
	}
	if title == "" {
		title = def.Name
	}
	return d.SaveDocument(db.Document{
		ProjectID: p.ID,
		Kind:      kind,
		Title:     title,
		Content:   documents.DefaultContent(documents.Kind(kind)),
		Version:   1,
		Status:    "draft",
	})
}

func (a *App) SaveDocument(doc db.Document) (db.Document, error) {
	d := a.requireDB()
	if d == nil {
		return db.Document{}, errors.New("no project open")
	}
	return d.SaveDocument(doc)
}

func (a *App) DeleteDocument(id string) error {
	d := a.requireDB()
	if d == nil {
		return errors.New("no project open")
	}
	actor := "unknown"
	if u := a.requireUser(); u != nil {
		actor = u.Username
	}
	_ = d.LogAction(actor, "delete_document", id, "")
	return d.DeleteDocument(id)
}

// ExportCombinedReport assembles multiple documents into one PDF.
// `sections` is an ordered list of {document_id, title, description}
// tuples — the report renders sections in that order. Returns the
// absolute path the PDF was written to (under the user's exports/).
func (a *App) ExportCombinedReport(reportTitle, subtitle string, sections []documents.ReportSection) (string, error) {
	d := a.requireDB()
	u := a.requireUser()
	if d == nil || u == nil {
		return "", errors.New("not signed in or no project open")
	}
	if len(sections) == 0 {
		return "", errors.New("report has no sections")
	}

	proj, err := d.GetProject()
	if err != nil {
		return "", err
	}

	// Resolve each section to a (doc kind + content) pair, and along
	// the way collect every chart_ref value so we can pre-fetch the
	// referenced charts in one pass.
	resolved := make([]documents.ResolvedSection, 0, len(sections))
	chartIDs := make(map[string]struct{})
	for _, s := range sections {
		doc, err := d.GetDocument(s.DocumentID)
		if err != nil {
			return "", fmt.Errorf("section %s: %w", s.DocumentID, err)
		}
		if s.Title == "" {
			s.Title = doc.Title
		}
		resolved = append(resolved, documents.ResolvedSection{
			Section: s,
			Kind:    documents.Kind(doc.Kind),
			Content: doc.Content,
			Version: doc.Version,
			Status:  doc.Status,
		})

		// Scan the document's content for chart_ref values. We
		// don't unmarshal the JSON twice — that work happens again
		// in renderSectionBody — but a cheap string-key lookup is
		// fine because chart_ref values are short opaque IDs.
		for _, id := range collectChartRefs(doc.Content, documents.EffectiveFields(documents.Kind(doc.Kind))) {
			chartIDs[id] = struct{}{}
		}
	}

	// Pre-fetch every referenced chart.
	resolvedCharts := make(map[string]documents.ResolvedChart, len(chartIDs))
	for id := range chartIDs {
		c, err := d.GetChart(id)
		if err != nil {
			// Skip silently; report.go's fallback handles missing charts.
			continue
		}
		resolvedCharts[id] = documents.ResolvedChart{
			Kind:  c.Kind,
			Title: c.Title,
			Data:  c.Data,
		}
	}

	bytes, err := documents.BuildCombinedReport(documents.ReportSpec{
		ReportTitle:    reportTitle,
		Subtitle:       subtitle,
		Author:         u.DisplayName,
		ProjectName:    proj.Name,
		Sections:       sections,
		ResolvedCharts: resolvedCharts,
		ResolvedEVM:    resolvedEVMForCharts(proj, resolvedCharts, time.Now().UTC()),
	}, resolved)
	if err != nil {
		return "", err
	}

	outDir := filepath.Join(u.DataDir, "exports")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", err
	}
	stamp := time.Now().UTC().Format("20060102-150405")
	outPath := filepath.Join(outDir, fmt.Sprintf("%s-%s.pdf", sanitizeFilename(reportTitle), stamp))
	if err := os.WriteFile(outPath, bytes, 0o600); err != nil {
		return "", err
	}
	return outPath, nil
}

// ExportCombinedReportSigned is like ExportCombinedReport but applies a
// real PAdES B-B digital signature (with visual appearance page) using
// the supplied certificate.
func (a *App) ExportCombinedReportSigned(reportTitle, subtitle string, sections []documents.ReportSection, certPath, certPassword string) (string, error) {
	d := a.requireDB()
	u := a.requireUser()
	if d == nil || u == nil {
		return "", errors.New("not signed in or no project open")
	}
	if len(sections) == 0 {
		return "", errors.New("report has no sections")
	}

	proj, err := d.GetProject()
	if err != nil {
		return "", err
	}
	reportID := combinedReportCheckpointID(proj.ID, reportTitle, subtitle, sections)

	// Resolve sections + charts (same logic as unsigned version)
	resolved := make([]documents.ResolvedSection, 0, len(sections))
	chartIDs := make(map[string]struct{})
	for _, s := range sections {
		doc, err := d.GetDocument(s.DocumentID)
		if err != nil {
			logCombinedReportSignatureEvent(d, proj.ID, reportID, reportTitle, subtitle, sections, false, fmt.Sprintf("section %s: %v", s.DocumentID, err), "")
			return "", fmt.Errorf("section %s: %w", s.DocumentID, err)
		}
		if s.Title == "" {
			s.Title = doc.Title
		}
		resolved = append(resolved, documents.ResolvedSection{
			Section: s,
			Kind:    documents.Kind(doc.Kind),
			Content: doc.Content,
			Version: doc.Version,
			Status:  doc.Status,
		})
		for _, id := range collectChartRefs(doc.Content, documents.EffectiveFields(documents.Kind(doc.Kind))) {
			chartIDs[id] = struct{}{}
		}
	}

	resolvedCharts := make(map[string]documents.ResolvedChart, len(chartIDs))
	for id := range chartIDs {
		c, err := d.GetChart(id)
		if err != nil {
			continue
		}
		resolvedCharts[id] = documents.ResolvedChart{Kind: c.Kind, Title: c.Title, Data: c.Data}
	}

	bytes, err := documents.BuildCombinedReport(documents.ReportSpec{
		ReportTitle:       reportTitle,
		Subtitle:          subtitle,
		Author:            u.DisplayName,
		ProjectName:       proj.Name,
		Sections:          sections,
		ResolvedCharts:    resolvedCharts,
		ResolvedEVM:       resolvedEVMForCharts(proj, resolvedCharts, time.Now().UTC()),
		AddSignatureBlock: true,
	}, resolved)
	if err != nil {
		logCombinedReportSignatureEvent(d, proj.ID, reportID, reportTitle, subtitle, sections, false, fmt.Sprintf("build report: %v", err), "")
		return "", err
	}

	// Apply real PAdES B-B signature
	signer, err := crypto.LoadCertificate(certPath, certPassword)
	if err != nil {
		logCombinedReportSignatureEvent(d, proj.ID, reportID, reportTitle, subtitle, sections, false, fmt.Sprintf("load certificate: %v", err), "")
		return "", fmt.Errorf("load certificate: %w", err)
	}

	signedBytes, err := pdfmeta.InjectPAdESSignature(bytes, signer.SignPDFCMS)
	if err != nil {
		logCombinedReportSignatureEvent(d, proj.ID, reportID, reportTitle, subtitle, sections, false, fmt.Sprintf("pades embedding: %v", err), "")
		return "", fmt.Errorf("pades embedding: %w", err)
	}

	outDir := filepath.Join(u.DataDir, "exports")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		logCombinedReportSignatureEvent(d, proj.ID, reportID, reportTitle, subtitle, sections, false, fmt.Sprintf("create exports dir: %v", err), "")
		return "", err
	}
	stamp := time.Now().UTC().Format("20060102-150405")
	outPath := filepath.Join(outDir, fmt.Sprintf("%s-%s-signed.pdf", sanitizeFilename(reportTitle), stamp))
	if err := os.WriteFile(outPath, signedBytes, 0o600); err != nil {
		logCombinedReportSignatureEvent(d, proj.ID, reportID, reportTitle, subtitle, sections, false, fmt.Sprintf("write signed report: %v", err), "")
		return "", err
	}
	logCombinedReportSignatureEvent(d, proj.ID, reportID, reportTitle, subtitle, sections, true, "Combined report signed successfully.", outPath)
	return outPath, nil
}

// ExportCombinedReportGnuPG writes an unsigned combined PDF plus a detached
// ASCII-armored GnuPG signature sidecar.
func (a *App) ExportCombinedReportGnuPG(reportTitle, subtitle string, sections []documents.ReportSection, keyID string) (GnuPGExportResult, error) {
	d := a.requireDB()
	if d == nil {
		return GnuPGExportResult{}, errors.New("no project open")
	}
	proj, err := d.GetProject()
	if err != nil {
		return GnuPGExportResult{}, err
	}
	reportID := combinedReportCheckpointID(proj.ID, reportTitle, subtitle, sections)

	pdfPath, err := a.ExportCombinedReport(reportTitle, subtitle, sections)
	if err != nil {
		logCombinedReportSignatureEvent(d, proj.ID, reportID, reportTitle, subtitle, sections, false, fmt.Sprintf("build report: %v", err), "")
		return GnuPGExportResult{}, err
	}
	sigPath := pdfPath + ".asc"
	if err := a.signFileWithGnuPG(pdfPath, sigPath, keyID); err != nil {
		logCombinedReportSignatureEvent(d, proj.ID, reportID, reportTitle, subtitle, sections, false, fmt.Sprintf("gpg detached signature: %v", err), "")
		return GnuPGExportResult{}, err
	}
	logCombinedReportSignatureEventWithStatus(d, proj.ID, reportID, reportTitle, subtitle, sections, "gpg_signed", "Detached GnuPG signature written.", sigPath)
	return GnuPGExportResult{PDFPath: pdfPath, SignaturePath: sigPath, Method: db.SignatureMethodGnuPG}, nil
}

// RepairAndSwap runs InformativeSelfHeal and, on success, calls
// SwapInSnapshot to atomically replace the live file. The handle on
// `a.db` is refreshed in place.
func (a *App) RepairAndSwap() (db.RepairResult, error) {
	a.mu.RLock()
	d := a.db
	path := a.dbPath
	var dek []byte
	if len(a.dek) == crypto.DEKSize {
		dek = make([]byte, len(a.dek))
		copy(dek, a.dek)
	}
	a.mu.RUnlock()
	if d == nil {
		return db.RepairResult{}, errors.New("no project open")
	}

	result, err := d.InformativeSelfHeal(path)
	if err != nil || !result.Success {
		return result, err
	}
	// If the result.Log mentions a snapshot, do the swap. We detect
	// this by checking for a .bak file rather than re-parsing the log.
	if _, statErr := os.Stat(path + ".bak"); statErr == nil {
		encrypted, err := db.IsEncryptedFile(path)
		if err != nil {
			result.Log = append(result.Log, "Swap failed: "+err.Error())
			return result, err
		}
		var fresh *db.Database
		if encrypted {
			if len(dek) != crypto.DEKSize {
				err := errors.New("database key is locked; sign in again")
				result.Log = append(result.Log, "Swap failed: "+err.Error())
				return result, err
			}
			fresh, err = d.SwapInEncryptedSnapshot(path, dek)
		} else {
			fresh, err = d.SwapInSnapshot(path)
		}
		if err != nil {
			result.Log = append(result.Log, "Swap failed: "+err.Error())
			return result, err
		}
		a.mu.Lock()
		a.db = fresh
		a.adminSvc = admin.NewService(fresh)
		a.sigmaSvc = service.NewProjectService(fresh)
		a.mu.Unlock()
		result.Log = append(result.Log, "Snapshot swapped into place; live file is now the healed copy.")
	}
	return result, nil
}

// ExportDocumentDOCX renders the document to a Microsoft Word file
// under the user's exports/ folder and returns the absolute path
// written. Uses gomutex/godocx under the hood.
func (a *App) ExportDocumentDOCX(id string) (string, error) {
	return a.exportDocumentAs(id, ".docx", func(kind documents.Kind, content, projectName string) ([]byte, error) {
		return export.RenderDocumentDOCX(kind, content, projectName)
	})
}

// ExportDocumentODT renders the document to an OpenDocument Text
// file. Sibling to ExportDocumentDOCX; uses the hand-built ODT
// generator in internal/export/odt.go.
func (a *App) ExportDocumentODT(id string) (string, error) {
	return a.exportDocumentAs(id, ".odt", func(kind documents.Kind, content, projectName string) ([]byte, error) {
		return export.RenderDocumentODT(kind, content, projectName)
	})
}

// ExportScheduleReportDOCX generates a Microsoft Word report of the
// current project's CPM schedule (tasks with full ES/EF/LS/LF/Float/
// Critical data) and saves it to the user's exports folder.
func (a *App) ExportScheduleReportDOCX() (string, error) {
	return a.exportScheduleReportAs(export.FormatDOCX)
}

// ExportScheduleReportODT generates an OpenDocument Text report of the
// current project's CPM schedule and saves it to the user's exports folder.
func (a *App) ExportScheduleReportODT() (string, error) {
	return a.exportScheduleReportAs(export.FormatODT)
}

// ExportScheduleReportPDF generates a PDF report of the current project's
// CPM schedule (for completeness with the other formats).
func (a *App) ExportScheduleReportPDF() (string, error) {
	return a.exportScheduleReportAs(export.FormatPDF)
}

// ExportScheduleReportCSV writes the current project's schedule (tasks with
// CPM fields) as a UTF-8 CSV for spreadsheets (Excel, Google Sheets).
func (a *App) ExportScheduleReportCSV() (string, error) {
	return a.exportScheduleReportAs(export.FormatCSV)
}

// ExportScheduleReportHTML writes a self-contained, printable HTML report of
// the current project's schedule for publishing or viewing in a browser.
func (a *App) ExportScheduleReportHTML() (string, error) {
	return a.exportScheduleReportAs(export.FormatHTML)
}

// ExportScheduleReportMSPDI writes the current project's schedule as Microsoft
// Project MSPDI XML (.xml) for interchange with MS Project, GanttProject,
// ProjectLibre, and other tools that read the MSPDI schema.
func (a *App) ExportScheduleReportMSPDI() (string, error) {
	return a.exportScheduleReportAs(export.FormatMSPDI)
}

// exportDocumentAs is the shared body of every per-format export
// method on App: fetch the document, call the format-specific
// renderer, write to the user's exports/ folder.
func (a *App) exportDocumentAs(
	id, extension string,
	renderer func(documents.Kind, string, string) ([]byte, error),
) (string, error) {
	d := a.requireDB()
	u := a.requireUser()
	if d == nil || u == nil {
		return "", errors.New("not signed in or no project open")
	}
	doc, err := d.GetDocument(id)
	if err != nil {
		return "", err
	}
	proj, err := d.GetProject()
	if err != nil {
		return "", err
	}
	bytes, err := renderer(documents.Kind(doc.Kind), doc.Content, proj.Name)
	if err != nil {
		return "", err
	}
	outDir := filepath.Join(u.DataDir, "exports")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", err
	}
	outPath := filepath.Join(outDir, fmt.Sprintf("%s-%s%s",
		sanitizeFilename(doc.Title),
		time.Now().UTC().Format("20060102-150405"),
		extension,
	))
	if err := os.WriteFile(outPath, bytes, 0o600); err != nil {
		return "", err
	}
	return outPath, nil
}

// exportScheduleReportAs is the shared implementation for exporting
// the current project's CPM schedule (Administrative Pack report) in
// DOCX or ODT.
func (a *App) exportScheduleReportAs(format export.ExportFormat) (string, error) {
	d := a.requireDB()
	u := a.requireUser()
	if d == nil || u == nil {
		return "", errors.New("not signed in or no project open")
	}

	proj, err := d.GetProject()
	if err != nil {
		return "", err
	}

	// Best-effort load of current schedule data.
	// V2 priority: active CPM chart (the one the user is actually maintaining).
	// Fallback: legacy V1 tasks table (for old projects).
	kernelTasks, err := loadCurrentProjectSchedule(d, proj.ID)
	if err != nil {
		// Non-fatal for export — we can still produce an empty report.
		kernelTasks = make(map[string]*kernel.Task)
	}

	if len(kernelTasks) > 0 {
		// Full scheduling pipeline: arm date constraints, run CPM,
		// anchor onto real dates (the latter two steps only when the
		// project has a parseable start date).
		scheduleProjectTasks(proj, kernelTasks)
	}

	payload := export.ReportPayload{Tasks: kernelTasks}

	// Earned-value summary at today's status date — only when the
	// project is anchored (offsets map to dates); renderers further
	// suppress the section when there is no cost data.
	if start, ok := parseProjectDate(proj.StartDate); ok && len(kernelTasks) > 0 {
		cal := calendar.For(proj.CountryCode)
		if day, dok := kernel.DayOffset(start, time.Now().UTC(), cal.IsWorkday); dok {
			m := kernel.ComputeEVM(kernelTasks, day)
			payload.EVM = &m
		}
	}

	opts := export.ExportOptions{
		Format: format,
		Title:  proj.Name,
	}

	raw, err := export.GenerateArchivalReport(payload, opts)
	if err != nil {
		return "", err
	}

	outDir := filepath.Join(u.DataDir, "exports")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", err
	}

	var ext string
	switch format {
	case export.FormatODT:
		ext = ".odt"
	case export.FormatPDF:
		ext = ".pdf"
	case export.FormatCSV:
		ext = ".csv"
	case export.FormatHTML:
		ext = ".html"
	case export.FormatMSPDI:
		ext = ".xml"
	default:
		ext = ".docx"
	}

	outPath := filepath.Join(outDir, fmt.Sprintf("Schedule-Report-%s%s",
		time.Now().UTC().Format("20060102-150405"),
		ext,
	))

	if err := os.WriteFile(outPath, raw, 0o600); err != nil {
		return "", err
	}

	// Best-effort audit (the audit table exists; we log via a simple insert if the helper is available in future).
	// For now we rely on the command_log + file presence for traceability.

	return outPath, nil
}

// stakeholderCapacities builds the resource-capacity map the kernel's
// overallocation detection and levelling consume: stakeholder name →
// availability in units (1.0 = full-time). Assignments naming
// resources that are not stakeholders fall back to the kernel's 1.0
// default. Best-effort: a lookup failure returns nil (default
// capacities) rather than blocking scheduling.
func stakeholderCapacities(d *db.Database, projectID string) map[string]float64 {
	list, err := d.ListStakeholders(projectID, "")
	if err != nil || len(list) == 0 {
		return nil
	}
	out := make(map[string]float64, len(list))
	for _, s := range list {
		if s.Name != "" && s.Availability > 0 {
			out[s.Name] = s.Availability
		}
	}
	return out
}

func resourceCapacityPlan(d *db.Database, projectID string) kernel.ResourceCapacityPlan {
	plan := kernel.ResourceCapacityPlan{
		DefaultCapacity: 1,
		Capacities:      stakeholderCapacities(d, projectID),
	}
	calendars, err := d.ListResourceCalendars(projectID)
	if err != nil || len(calendars) == 0 {
		return plan
	}
	plan.Calendars = make(map[string]kernel.ResourceCalendar, len(calendars))
	for _, c := range calendars {
		cal := kernel.ResourceCalendar{
			ID:              c.ID,
			Resource:        c.Resource,
			DefaultCapacity: c.DefaultCapacity,
			Overrides:       c.Overrides,
			WeeklyCapacity:  c.WeeklyCapacity,
			Notes:           c.Notes,
			SkillTags:       c.SkillTags,
		}
		if c.Resource != "" {
			plan.Calendars[c.Resource] = cal
		}
		if c.ID != "" {
			plan.Calendars[c.ID] = cal
		}
		if c.Name != "" {
			plan.Calendars[c.Name] = cal
		}
	}
	return plan
}

// scheduleProjectTasks runs the full scheduling pipeline on a kernel
// task map: date constraints are armed against the project start date
// and country work calendar, CPM computes the schedule, and the
// offsets are anchored onto real dates. Projects without a parseable
// start date still get plain CPM (date constraints stay dormant and
// no calendar dates are emitted), preserving legacy behaviour.
func scheduleProjectTasks(proj db.Project, tasks map[string]*kernel.Task) {
	if len(tasks) == 0 {
		return
	}
	start, ok := parseProjectDate(proj.StartDate)
	if !ok {
		kernel.CalculateCPM(tasks)
		return
	}
	c := calendar.For(proj.CountryCode)
	kernel.ApplyConstraintDates(tasks, start, c.IsWorkday)
	kernel.CalculateCPM(tasks)
	kernel.AnchorSchedule(tasks, start, c.IsWorkday)
}

// parseProjectDate accepts the two date shapes stored in
// project.start_date: plain YYYY-MM-DD and full RFC3339.
func parseProjectDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// loadCurrentProjectSchedule returns the best available schedule data
// as a kernel.Task map (with CPM fields computed).
//
// V2 path: newest CPM chart for the project.
// V1 fallback: legacy tasks table.
func loadCurrentProjectSchedule(d *db.Database, projectID string) (map[string]*kernel.Task, error) {
	// 1. Try current V2 CPM chart (preferred)
	if chs, err := d.ListCharts(projectID, string(charts.KindCPM)); err == nil && len(chs) > 0 {
		// Most recently updated
		// UpdatedAt is an RFC3339Nano string; lexicographic order matches
		// chronological order, so ">" yields most-recent-first.
		sort.Slice(chs, func(i, j int) bool { return chs[i].UpdatedAt > chs[j].UpdatedAt })
		if tasks, err := cpmChartDataToKernelTasks(chs[0].Data); err == nil && len(tasks) > 0 {
			return tasks, nil
		}
	}

	// 2. Fallback to V1 tasks table
	return loadV1TasksAsKernel(d)
}

func cpmChartDataToKernelTasks(dataJSON string) (map[string]*kernel.Task, error) {
	if dataJSON == "" {
		return nil, nil
	}
	var doc struct {
		Nodes []struct {
			ID                     string                  `json:"id"`
			Label                  string                  `json:"label"`
			Duration               float64                 `json:"duration"`
			DurationEstimate       kernel.DurationEstimate `json:"duration_estimate"`
			Constraint             string                  `json:"constraint"`
			ConstraintDate         string                  `json:"constraint_date"`
			PercentComplete        float64                 `json:"percent_complete"`
			Milestone              bool                    `json:"milestone"`
			ActualStart            string                  `json:"actual_start"`
			ActualFinish           string                  `json:"actual_finish"`
			BudgetedCost           float64                 `json:"budgeted_cost"`
			BudgetedCostMinorUnits int64                   `json:"budgeted_cost_minor_units"`
			ActualCost             float64                 `json:"actual_cost"`
			ActualCostMinorUnits   int64                   `json:"actual_cost_minor_units"`
			Assignments            []struct {
				Resource   string   `json:"resource"`
				Units      float64  `json:"units"`
				CalendarID string   `json:"calendar_id"`
				SkillTags  []string `json:"skill_tags"`
				MaxUnits   float64  `json:"max_units"`
			} `json:"assignments"`
		} `json:"nodes"`
		Edges []struct {
			From  string `json:"from"`
			To    string `json:"to"`
			Label string `json:"label"`
		} `json:"edges"`
	}
	if err := json.Unmarshal([]byte(dataJSON), &doc); err != nil {
		return nil, err
	}

	tasks := make(map[string]*kernel.Task, len(doc.Nodes))
	for _, n := range doc.Nodes {
		t := &kernel.Task{
			ID:                     n.ID,
			Title:                  n.Label,
			Duration:               n.Duration,
			DurationEstimate:       n.DurationEstimate,
			Constraint:             kernel.ConstraintType(strings.ToUpper(strings.TrimSpace(n.Constraint))),
			ConstraintDate:         n.ConstraintDate,
			PercentComplete:        n.PercentComplete,
			Milestone:              n.Milestone,
			ActualStart:            n.ActualStart,
			ActualFinish:           n.ActualFinish,
			BudgetedCost:           n.BudgetedCost,
			BudgetedCostMinorUnits: n.BudgetedCostMinorUnits,
			ActualCost:             n.ActualCost,
			ActualCostMinorUnits:   n.ActualCostMinorUnits,
		}
		for _, a := range n.Assignments {
			t.Assignments = append(t.Assignments, kernel.Assignment{
				Resource:   a.Resource,
				Units:      a.Units,
				CalendarID: a.CalendarID,
				SkillTags:  a.SkillTags,
				MaxUnits:   a.MaxUnits,
			})
		}
		tasks[n.ID] = t
	}
	for _, e := range doc.Edges {
		if t, ok := tasks[e.To]; ok {
			typ, lag := dag.ParseLinkLabel(e.Label)
			t.Links = append(t.Links, kernel.Link{Pred: e.From, Type: typ, Lag: lag})
		}
	}
	return tasks, nil
}

func loadV1TasksAsKernel(d *db.Database) (map[string]*kernel.Task, error) {
	rows, err := d.Conn.Query(`SELECT id, title, duration, precedents FROM tasks ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	tasks := make(map[string]*kernel.Task)
	for rows.Next() {
		var id, title, precJSON string
		var duration float64
		if err := rows.Scan(&id, &title, &duration, &precJSON); err != nil {
			// A Scan failure is data corruption, not a row to skip:
			// silently dropping tasks would corrupt CPM/EVM results.
			return nil, fmt.Errorf("scan v1 task: %w", err)
		}
		var precedents []string
		_ = json.Unmarshal([]byte(precJSON), &precedents)

		tasks[id] = &kernel.Task{
			ID:         id,
			Title:      title,
			Duration:   duration,
			Precedents: precedents,
		}
	}
	// A mid-iteration error ends the loop silently; without this check a
	// partial task map would be treated as the complete schedule.
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tasks, nil
}

// ExportDocumentPDF renders the document to PDF under the user's
// exports/ folder and returns the absolute path written.
func (a *App) ExportDocumentPDF(id string) (string, error) {
	d := a.requireDB()
	u := a.requireUser()
	if d == nil || u == nil {
		return "", errors.New("not signed in or no project open")
	}
	doc, err := d.GetDocument(id)
	if err != nil {
		return "", err
	}
	proj, err := d.GetProject()
	if err != nil {
		return "", err
	}
	bytes, err := documents.Render(documents.Kind(doc.Kind), doc.Content, proj.Name)
	if err != nil {
		return "", err
	}
	outDir := filepath.Join(u.DataDir, "exports")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", err
	}
	outPath := filepath.Join(outDir, fmt.Sprintf("%s-%s.pdf",
		sanitizeFilename(doc.Title), time.Now().UTC().Format("20060102-150405")))
	if err := os.WriteFile(outPath, bytes, 0o600); err != nil {
		return "", err
	}
	return outPath, nil
}

type GnuPGExportResult struct {
	PDFPath       string `json:"pdf_path"`
	SignaturePath string `json:"signature_path"`
	Method        string `json:"method"`
}

// ExportDocumentPDFGnuPG renders the document as a plain PDF and writes a
// detached ASCII-armored GnuPG signature sidecar. The PDF bytes are not
// modified, so PDF/A validation and print-and-wet-sign workflows remain intact.
func (a *App) ExportDocumentPDFGnuPG(id, keyID string) (GnuPGExportResult, error) {
	d := a.requireDB()
	if d == nil {
		return GnuPGExportResult{}, errors.New("no project open")
	}
	pdfPath, err := a.ExportDocumentPDF(id)
	if err != nil {
		return GnuPGExportResult{}, err
	}
	sigPath := pdfPath + ".asc"
	if err := a.signFileWithGnuPG(pdfPath, sigPath, keyID); err != nil {
		admin.NewService(d).LogSignatureEvent(id, false, err)
		return GnuPGExportResult{}, err
	}
	admin.NewService(d).LogDocumentSignatureOutcome(id, "gpg_signed", "Detached GnuPG signature written.", sigPath)
	return GnuPGExportResult{PDFPath: pdfPath, SignaturePath: sigPath, Method: db.SignatureMethodGnuPG}, nil
}

// ExportDocumentPDFSigned is like ExportDocumentPDF but applies a real
// PAdES B-B digital signature using the provided certificate.
func (a *App) ExportDocumentPDFSigned(id, certPath, certPassword string) (string, error) {
	d := a.requireDB()
	u := a.requireUser()
	if d == nil || u == nil {
		return "", errors.New("not signed in or no project open")
	}
	doc, err := d.GetDocument(id)
	if err != nil {
		return "", err
	}
	proj, err := d.GetProject()
	if err != nil {
		return "", err
	}

	bytes, err := documents.RenderSigned(documents.Kind(doc.Kind), doc.Content, proj.Name, certPath, certPassword)
	if err != nil {
		admin.NewService(d).LogSignatureEvent(doc.ID, false, err)
		return "", err
	}

	outDir := filepath.Join(u.DataDir, "exports")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", err
	}
	outPath := filepath.Join(outDir, fmt.Sprintf("%s-%s-signed.pdf",
		sanitizeFilename(doc.Title), time.Now().UTC().Format("20060102-150405")))
	if err := os.WriteFile(outPath, bytes, 0o600); err != nil {
		admin.NewService(d).LogSignatureEvent(doc.ID, false, err)
		return "", err
	}
	admin.NewService(d).LogSignatureEvent(doc.ID, true, nil)
	return outPath, nil
}

func (a *App) signFileWithGnuPG(inputPath, signaturePath, keyID string) error {
	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, 2*time.Minute)
	defer cancel()
	return signing.SignDetachedASCIIArmored(ctx, signing.ExecCommandRunner, inputPath, signaturePath, strings.TrimSpace(keyID))
}
