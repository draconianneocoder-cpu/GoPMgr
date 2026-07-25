// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"pmforge/internal/db"
	"pmforge/internal/kernel"
)

func TestCpmChartDataToKernelTasksPreservesSplitPlanForEVM(t *testing.T) {
	tasks, err := cpmChartDataToKernelTasks(`{
		"nodes": [
			{"id": "P", "label": "Predecessor", "duration": 2},
			{
				"id": "A",
				"label": "Split task",
				"duration": 3,
				"budgeted_cost_minor_units": 30000,
				"work_segments": [
					{"start": 0, "end": 1},
					{"start": 2, "end": 3},
					{"start": 4, "end": 5}
				]
			}
		],
		"edges": [{"from": "P", "to": "A"}]
	}`)
	if err != nil {
		t.Fatalf("cpmChartDataToKernelTasks: %v", err)
	}

	task := tasks["A"]
	if task == nil {
		t.Fatal("task A was not mapped")
	}
	if len(task.PlannedWorkSegments) != 3 {
		t.Fatalf("PlannedWorkSegments = %+v, want 3 persisted segments", task.PlannedWorkSegments)
	}

	kernel.CalculateCPM(tasks)
	if task.ES != 2 {
		t.Fatalf("split task ES = %v, want 2 after predecessor", task.ES)
	}
	got := kernel.ComputeEVM(tasks, 3.5)
	if got.PVMinorUnits != 10000 {
		t.Fatalf("PVMinorUnits during split gap = %d, want 10000", got.PVMinorUnits)
	}
}

func TestCpmChartDataToKernelTasksRejectsOverlappingSplitSegments(t *testing.T) {
	_, err := cpmChartDataToKernelTasks(`{
		"nodes": [{
			"id": "A",
			"duration": 3,
			"work_segments": [
				{"start": 0, "end": 2},
				{"start": 1, "end": 3}
			]
		}]
	}`)
	if err == nil {
		t.Fatal("cpmChartDataToKernelTasks accepted overlapping work_segments")
	}
	if !strings.Contains(err.Error(), `task "A"`) || !strings.Contains(err.Error(), "overlaps") {
		t.Fatalf("error = %q, want task-scoped overlap explanation", err)
	}
}

func TestLoadCurrentProjectScheduleRejectsInvalidCurrentChart(t *testing.T) {
	d, err := db.InitDB(filepath.Join(t.TempDir(), "invalid-split.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	project, err := d.UpsertProject(db.Project{ID: "project-1", Name: "Invalid Split"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	_, err = d.SaveChart(db.Chart{
		ProjectID: project.ID,
		Kind:      "cpm",
		Title:     "Current Schedule",
		Data: `{
			"nodes": [{
				"id": "A",
				"duration": 3,
				"work_segments": [
					{"start": 0, "end": 2},
					{"start": 1, "end": 3}
				]
			}]
		}`,
	})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}

	_, err = loadCurrentProjectSchedule(d, project.ID)
	if err == nil {
		t.Fatal("loadCurrentProjectSchedule silently fell back from an invalid current CPM chart")
	}
	if !strings.Contains(err.Error(), "Current Schedule") || !strings.Contains(err.Error(), "overlaps") {
		t.Fatalf("error = %q, want chart title and overlap explanation", err)
	}
}
