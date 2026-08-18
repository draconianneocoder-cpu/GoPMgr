// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"time"

	"gopmgr/internal/calendar"
	"gopmgr/internal/charts"
	"gopmgr/internal/db"
	"gopmgr/internal/documents"
	"gopmgr/internal/export"
	"gopmgr/internal/kernel"
	sigmacharts "gopmgr/internal/sigma/charts"
	"gopmgr/internal/sigma/domain"
	"gopmgr/internal/sigma/service"
	"gopmgr/internal/sigma/stats"
	"gopmgr/internal/sigma/tollgate"
)

// =========================================================
// Process Excellence Suite (Six Sigma) — MVP 1
// =========================================================

func (a *App) SigmaCreateProject(title, description string, beltLevel string) (domain.Project, error) {
	svc := a.requireSigmaSvc()
	if svc == nil {
		return domain.Project{}, fmt.Errorf("sigma: no project open")
	}
	d := a.requireDB()
	if d == nil {
		return domain.Project{}, fmt.Errorf("sigma: no project open")
	}
	openProject, err := d.GetProject()
	if err != nil {
		return domain.Project{}, fmt.Errorf("sigma: resolve open project: %w", err)
	}
	input := domain.Project{
		GopmgrProjectID: openProject.ID,
		Title:           title,
		Description:     description,
		BeltLevel:       domain.BeltLevel(beltLevel),
	}
	p, err := svc.CreateProject(input)
	if err != nil {
		return domain.Project{}, err
	}
	return *p, nil
}

func (a *App) SigmaListProjects() ([]domain.Project, error) {
	svc := a.requireSigmaSvc()
	if svc == nil {
		return nil, fmt.Errorf("sigma: no project open")
	}
	d := a.requireDB()
	if d == nil {
		return nil, fmt.Errorf("sigma: no project open")
	}
	openProject, err := d.GetProject()
	if err != nil {
		return nil, fmt.Errorf("sigma: resolve open project: %w", err)
	}
	return svc.ListProjects(openProject.ID)
}

func (a *App) SigmaGetProject(id string) (domain.Project, error) {
	svc := a.requireSigmaSvc()
	if svc == nil {
		return domain.Project{}, fmt.Errorf("sigma: no project open")
	}
	d := a.requireDB()
	if d == nil {
		return domain.Project{}, fmt.Errorf("sigma: no project open")
	}
	openProject, err := d.GetProject()
	if err != nil {
		return domain.Project{}, fmt.Errorf("sigma: resolve open project: %w", err)
	}
	p, err := svc.GetProject(id)
	if err != nil {
		return domain.Project{}, err
	}
	// A Sigma project id from a stale session.editingId (left over from a
	// previously open GoPMgr file) must not resolve against whatever file
	// happens to be open now -- SigmaListProjects is scoped the same way,
	// and this is the chokepoint SigmaProjectView.svelte's loadProject()
	// calls before any sub-tab getter (Charter/Fishbone/SIPOC/VoC/...),
	// so a not-found here also stops those from ever being requested with
	// a foreign id in the normal UI flow.
	if p.GopmgrProjectID != openProject.ID {
		return domain.Project{}, fmt.Errorf("sigma: project %q not found", id)
	}
	return *p, nil
}

func (a *App) SigmaSaveCharter(c domain.Charter) error {
	svc := a.requireSigmaSvc()
	if svc == nil {
		return fmt.Errorf("sigma: no project open")
	}
	return svc.SaveCharter(c)
}

func (a *App) SigmaGetCharter(projectID string) (domain.Charter, error) {
	svc := a.requireSigmaSvc()
	if svc == nil {
		return domain.Charter{}, fmt.Errorf("sigma: no project open")
	}
	c, err := svc.GetCharter(projectID)
	if err != nil {
		return domain.Charter{}, err
	}
	return *c, nil
}

func (a *App) SigmaAdvancePhase(projectID, phase string) error {
	svc := a.requireSigmaSvc()
	if svc == nil {
		return fmt.Errorf("sigma: no project open")
	}
	// Check readiness of the CURRENT phase before allowing advance
	// We need to know the current phase to check it.
	// For MVP, we check Define readiness if moving FROM Define.
	// In a real app, we'd pass currentPhase or fetch it.
	// Let's fetch the project to get current phase.
	p, err := svc.GetProject(projectID)
	if err != nil {
		return err
	}

	// Only gate the Define phase for MVP 1
	if p.Phase == domain.PhaseDefine && phase != string(domain.PhaseDefine) {
		charter, _ := svc.GetCharter(projectID)
		sipoc, _ := svc.GetSIPOC(projectID)
		voc, _ := svc.GetVoC(projectID)
		res := tollgate.CheckDefineReadiness(*charter, sipoc, voc)
		if !res.CanAdvance {
			return fmt.Errorf("sigma: Define phase readiness is %.0f%% (need 80%%). Missing: %s", res.Score, res.MissingList)
		}
	}

	return svc.AdvancePhase(projectID, domain.Phase(phase))
}

// SigmaCalculateDescriptive returns mean, median, std dev, min, max for a dataset.
func (a *App) SigmaCalculateDescriptive(values []float64) (stats.DescriptiveResult, error) {
	return stats.CalculateDescriptive(values)
}

// SigmaCalculateCapability returns Cp, Cpk, Pp, Ppk, Sigma Level, DPMO.
func (a *App) SigmaCalculateCapability(values []float64, usl, lsl float64) (stats.CapabilityResult, error) {
	return stats.CalculateCapability(values, usl, lsl)
}

// SigmaCalculatePareto returns sorted categories with cumulative percentages.
func (a *App) SigmaCalculatePareto(categories []string, counts []int) ([]sigmacharts.ParetoItem, error) {
	return sigmacharts.CalculatePareto(categories, counts)
}

// SigmaCheckReadiness evaluates the current phase tollgate requirements.
func (a *App) SigmaCheckReadiness(projectID, phase string) (tollgate.Result, error) {
	svc := a.requireSigmaSvc()
	if svc == nil {
		return tollgate.Result{}, fmt.Errorf("sigma: no project open")
	}
	charter, err := svc.GetCharter(projectID)
	if err != nil {
		return tollgate.Result{}, err
	}
	sipoc, _ := svc.GetSIPOC(projectID)
	voc, _ := svc.GetVoC(projectID)
	fb, _ := svc.GetFishbone(projectID)
	solutions, _ := svc.GetSolutions(projectID)
	controlPlan, _ := svc.GetControlPlan(projectID)
	return tollgate.CheckPhase(domain.Phase(phase), *charter, sipoc, voc, fb, solutions, controlPlan), nil
}

// SigmaSaveFishbone persists the Fishbone diagram data.
func (a *App) SigmaSaveFishbone(projectID string, fb domain.FishboneData) error {
	svc := a.requireSigmaSvc()
	if svc == nil {
		return fmt.Errorf("sigma: no project open")
	}
	return svc.SaveFishbone(fb, projectID)
}

// SigmaGetFishbone retrieves the Fishbone diagram data.
func (a *App) SigmaGetFishbone(projectID string) (domain.FishboneData, error) {
	svc := a.requireSigmaSvc()
	if svc == nil {
		return domain.FishboneData{}, fmt.Errorf("sigma: no project open")
	}
	fb, err := svc.GetFishbone(projectID)
	if err != nil {
		return domain.FishboneData{}, err
	}
	return *fb, nil
}

// SigmaSaveSolutions persists the Solution Selection Matrix data.
func (a *App) SigmaSaveSolutions(projectID string, solutions []domain.Solution) error {
	svc := a.requireSigmaSvc()
	if svc == nil {
		return fmt.Errorf("sigma: no project open")
	}
	return svc.SaveSolutions(projectID, solutions)
}

// SigmaGetSolutions retrieves the Solution Selection Matrix data.
func (a *App) SigmaGetSolutions(projectID string) ([]domain.Solution, error) {
	svc := a.requireSigmaSvc()
	if svc == nil {
		return nil, fmt.Errorf("sigma: no project open")
	}
	return svc.GetSolutions(projectID)
}

// SigmaSaveControlPlan persists the Control Plan data.
func (a *App) SigmaSaveControlPlan(projectID string, items []domain.ControlPlanItem) error {
	svc := a.requireSigmaSvc()
	if svc == nil {
		return fmt.Errorf("sigma: no project open")
	}
	return svc.SaveControlPlan(projectID, items)
}

// SigmaGetControlPlan retrieves the Control Plan data.
func (a *App) SigmaGetControlPlan(projectID string) ([]domain.ControlPlanItem, error) {
	svc := a.requireSigmaSvc()
	if svc == nil {
		return nil, fmt.Errorf("sigma: no project open")
	}
	return svc.GetControlPlan(projectID)
}

// SigmaSaveSIPOC persists the SIPOC diagram data.
func (a *App) SigmaSaveSIPOC(projectID string, data domain.SIPOCData) error {
	svc := a.requireSigmaSvc()
	if svc == nil {
		return fmt.Errorf("sigma: no project open")
	}
	return svc.SaveSIPOC(projectID, data)
}

// SigmaGetSIPOC retrieves the SIPOC diagram data.
func (a *App) SigmaGetSIPOC(projectID string) (domain.SIPOCData, error) {
	svc := a.requireSigmaSvc()
	if svc == nil {
		return domain.SIPOCData{}, fmt.Errorf("sigma: no project open")
	}
	sipoc, err := svc.GetSIPOC(projectID)
	if err != nil {
		return domain.SIPOCData{}, err
	}
	return *sipoc, nil
}

// SigmaSaveVoC persists the Voice of Customer data.
func (a *App) SigmaSaveVoC(projectID string, data domain.VoCData) error {
	svc := a.requireSigmaSvc()
	if svc == nil {
		return fmt.Errorf("sigma: no project open")
	}
	return svc.SaveVoC(projectID, data)
}

// SigmaGetVoC retrieves the Voice of Customer data.
func (a *App) SigmaGetVoC(projectID string) (domain.VoCData, error) {
	svc := a.requireSigmaSvc()
	if svc == nil {
		return domain.VoCData{}, fmt.Errorf("sigma: no project open")
	}
	voc, err := svc.GetVoC(projectID)
	if err != nil {
		return domain.VoCData{}, err
	}
	return *voc, nil
}

// SigmaGetToolStatus returns the completion status of tools for the given phase.
func (a *App) SigmaGetToolStatus(projectID, phase string) (service.PhaseTools, error) {
	svc := a.requireSigmaSvc()
	if svc == nil {
		return service.PhaseTools{}, fmt.Errorf("sigma: no project open")
	}
	return svc.GetToolStatus(projectID, phase), nil
}

// SigmaExportProjectReport generates a PDF report of all phase deliverables.
// The report is written under the signed-in user's own exports/ folder, the
// same location every other export in the app uses.
func (a *App) SigmaExportProjectReport(projectID string) (string, error) {
	svc := a.requireSigmaSvc()
	u := a.requireUser()
	if svc == nil || u == nil {
		return "", fmt.Errorf("sigma: not signed in or no project open")
	}

	project, charter, sipoc, fishbone, solutions, controlPlan, err := svc.GetProjectReportData(projectID)
	if err != nil {
		return "", err
	}

	outDir := filepath.Join(u.DataDir, "exports")
	return export.GenerateSigmaReport(project, charter, sipoc, fishbone, solutions, controlPlan, outDir)
}

func trimExt(name string) string {
	ext := filepath.Ext(name)
	return name[:len(name)-len(ext)]
}

func combinedReportCheckpointID(projectID, reportTitle, subtitle string, sections []documents.ReportSection) string {
	payload := struct {
		ProjectID   string                    `json:"project_id"`
		ReportTitle string                    `json:"report_title"`
		Subtitle    string                    `json:"subtitle"`
		Sections    []documents.ReportSection `json:"sections"`
	}{
		ProjectID:   projectID,
		ReportTitle: reportTitle,
		Subtitle:    subtitle,
		Sections:    append([]documents.ReportSection(nil), sections...),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "report_invalid"
	}
	sum := sha256.Sum256(data)
	return "report_" + hex.EncodeToString(sum[:])
}

func logCombinedReportSignatureEvent(d *db.Database, projectID, reportID, reportTitle, subtitle string, sections []documents.ReportSection, signed bool, details, outputPath string) {
	signatureStatus := "failed"
	if signed {
		signatureStatus = "signed"
	}
	logCombinedReportSignatureEventWithStatus(d, projectID, reportID, reportTitle, subtitle, sections, signatureStatus, details, outputPath)
}

func logCombinedReportSignatureEventWithStatus(d *db.Database, projectID, reportID, reportTitle, subtitle string, sections []documents.ReportSection, signatureStatus, details, outputPath string) {
	if signatureStatus == "" {
		signatureStatus = "unsigned"
	}
	payload, err := json.Marshal(struct {
		ReportID     string                    `json:"report_id"`
		ReportTitle  string                    `json:"report_title"`
		Subtitle     string                    `json:"subtitle"`
		SectionCount int                       `json:"section_count"`
		Sections     []documents.ReportSection `json:"sections"`
		Status       string                    `json:"status"`
		Details      string                    `json:"details,omitempty"`
		OutputPath   string                    `json:"output_path,omitempty"`
	}{
		ReportID:     reportID,
		ReportTitle:  reportTitle,
		Subtitle:     subtitle,
		SectionCount: len(sections),
		Sections:     append([]documents.ReportSection(nil), sections...),
		Status:       signatureStatus,
		Details:      details,
		OutputPath:   outputPath,
	})
	if err != nil {
		log.Printf("combined report signature audit payload failed: %v", err)
		return
	}
	if _, err := d.AppendAuditEvent(db.AuditEventInput{
		ProjectID:       projectID,
		EventType:       "combined_report.signature",
		EntityType:      "combined_report",
		EntityID:        reportID,
		AfterJSON:       string(payload),
		SignatureStatus: signatureStatus,
	}); err != nil {
		log.Printf("combined report signature audit event failed: %v", err)
	}
}

func resolvedEVMForCharts(proj db.Project, resolvedCharts map[string]documents.ResolvedChart, asOf time.Time) (map[string]*kernel.EVMetrics, error) {
	if len(resolvedCharts) == 0 {
		return nil, nil
	}
	start, ok := parseProjectDate(proj.StartDate)
	if !ok {
		return nil, nil
	}
	cal := calendar.For(proj.CountryCode)
	day, ok := kernel.DayOffset(start, asOf, cal.IsWorkday)
	if !ok {
		return nil, nil
	}

	chartIDs := make([]string, 0, len(resolvedCharts))
	for id := range resolvedCharts {
		chartIDs = append(chartIDs, id)
	}
	sort.Strings(chartIDs)

	out := make(map[string]*kernel.EVMetrics)
	for _, id := range chartIDs {
		c := resolvedCharts[id]
		kind := charts.Kind(c.Kind)
		if kind != charts.KindCPM && kind != charts.KindGantt {
			continue
		}
		tasks, err := cpmChartDataToKernelTasks(c.Data)
		if err != nil || len(tasks) == 0 {
			continue
		}
		scheduleProjectTasks(proj, tasks)
		metrics, err := kernel.ComputeEVM(tasks, day)
		if err != nil {
			return nil, fmt.Errorf("compute EVM for chart %q: %w", id, err)
		}
		if metrics.BAC <= 0 {
			continue
		}
		m := metrics
		out[id] = &m
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// sanitizeFilename strips path separators and disallowed characters
// from a user-supplied project name so it is safe to use as a file
// name on every platform.
func sanitizeFilename(s string) string {
	var b []rune
	for _, r := range s {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			b = append(b, '_')
		default:
			if r >= 32 {
				b = append(b, r)
			}
		}
	}
	out := string(b)
	if out == "" {
		return ""
	}
	if len(out) > 80 {
		out = out[:80]
	}
	return out
}
