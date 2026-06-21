// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package service

import (
	"strings"
	"testing"

	"pmforge/internal/sigma/domain"
)

// nil DB is safe for all tests below because the early-return
// validation paths return before any s.DB method is called, and the
// GetToolStatus "measure" and "default" cases contain no DB calls.
var nilSvc = &ProjectService{DB: nil}

// --- input validation ---

func TestCreateProject_EmptyTitle_ReturnsError(t *testing.T) {
	_, err := nilSvc.CreateProject(domain.Project{})
	if err == nil {
		t.Fatal("CreateProject({}) returned nil error, want error")
	}
	if !strings.Contains(err.Error(), "title required") {
		t.Errorf("error %q does not mention 'title required'", err.Error())
	}
}

func TestSaveCharter_EmptyProjectID_ReturnsError(t *testing.T) {
	err := nilSvc.SaveCharter(domain.Charter{})
	if err == nil {
		t.Fatal("SaveCharter({}) returned nil error, want error")
	}
	if !strings.Contains(err.Error(), "project_id required") {
		t.Errorf("error %q does not mention 'project_id required'", err.Error())
	}
}

func TestSaveSolutions_EmptyProjectID_ReturnsError(t *testing.T) {
	err := nilSvc.SaveSolutions("", nil)
	if err == nil {
		t.Fatal("SaveSolutions(\"\") returned nil error, want error")
	}
	if !strings.Contains(err.Error(), "project_id required") {
		t.Errorf("error %q does not mention 'project_id required'", err.Error())
	}
}

func TestSaveControlPlan_EmptyProjectID_ReturnsError(t *testing.T) {
	err := nilSvc.SaveControlPlan("", nil)
	if err == nil {
		t.Fatal("SaveControlPlan(\"\") returned nil error, want error")
	}
	if !strings.Contains(err.Error(), "project_id required") {
		t.Errorf("error %q does not mention 'project_id required'", err.Error())
	}
}

func TestSaveSIPOC_EmptyProjectID_ReturnsError(t *testing.T) {
	err := nilSvc.SaveSIPOC("", domain.SIPOCData{})
	if err == nil {
		t.Fatal("SaveSIPOC(\"\") returned nil error, want error")
	}
	if !strings.Contains(err.Error(), "project_id required") {
		t.Errorf("error %q does not mention 'project_id required'", err.Error())
	}
}

func TestSaveVoC_EmptyProjectID_ReturnsError(t *testing.T) {
	err := nilSvc.SaveVoC("", domain.VoCData{})
	if err == nil {
		t.Fatal("SaveVoC(\"\") returned nil error, want error")
	}
	if !strings.Contains(err.Error(), "project_id required") {
		t.Errorf("error %q does not mention 'project_id required'", err.Error())
	}
}

// --- GetToolStatus ---

func TestGetToolStatus_MeasurePhase_ThreeNotStartedTools(t *testing.T) {
	result := nilSvc.GetToolStatus("p1", "measure")
	if result.Phase != "measure" {
		t.Errorf("Phase = %q, want %q", result.Phase, "measure")
	}
	if len(result.Tools) != 3 {
		t.Fatalf("len(Tools) = %d, want 3", len(result.Tools))
	}
	for _, tool := range result.Tools {
		if tool.Status != "not_started" {
			t.Errorf("tool %q Status = %q, want %q", tool.Name, tool.Status, "not_started")
		}
	}
}

func TestGetToolStatus_UnknownPhase_ReturnsEmptyTools(t *testing.T) {
	result := nilSvc.GetToolStatus("p1", "unknown_phase")
	if result.Phase != "unknown_phase" {
		t.Errorf("Phase = %q, want %q", result.Phase, "unknown_phase")
	}
	if len(result.Tools) != 0 {
		t.Errorf("len(Tools) = %d, want 0", len(result.Tools))
	}
}

func TestGetToolStatus_EmptyPhase_ReturnsEmptyTools(t *testing.T) {
	result := nilSvc.GetToolStatus("p1", "")
	if len(result.Tools) != 0 {
		t.Errorf("len(Tools) = %d, want 0 for empty phase", len(result.Tools))
	}
}
