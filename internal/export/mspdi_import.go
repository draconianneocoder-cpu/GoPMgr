// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"encoding/xml"
	"errors"
	"strconv"
	"strings"

	"pmforge/internal/kernel"
)

// ImportedTask is one schedulable activity read from an MSPDI file,
// already converted to PMForge conventions (durations in working
// days, link lag in days).
type ImportedTask struct {
	UID             string
	Name            string
	DurationDays    float64
	Milestone       bool
	PercentComplete float64
	Links           []kernel.Link
	Assignments     []kernel.Assignment
}

// ImportedProject is the result of FromMSPDI.
type ImportedProject struct {
	Title     string
	StartDate string // YYYY-MM-DD, or "" when the file has none
	Tasks     []ImportedTask
}

// mspdiImport mirrors the subset of the MSPDI schema PMForge reads.
// Field names follow the Microsoft Project Data Interchange spec.
type mspdiImport struct {
	XMLName   xml.Name `xml:"Project"`
	Title     string   `xml:"Title"`
	Name      string   `xml:"Name"`
	StartDate string   `xml:"StartDate"`
	Tasks     []struct {
		UID             string  `xml:"UID"`
		Name            string  `xml:"Name"`
		Duration        string  `xml:"Duration"`
		Milestone       string  `xml:"Milestone"`
		Summary         string  `xml:"Summary"`
		IsNull          string  `xml:"IsNull"`
		PercentComplete float64 `xml:"PercentComplete"`
		Predecessors    []struct {
			PredecessorUID string  `xml:"PredecessorUID"`
			Type           *int    `xml:"Type"`
			LinkLag        float64 `xml:"LinkLag"`
		} `xml:"PredecessorLink"`
	} `xml:"Tasks>Task"`
	Resources []struct {
		UID  string `xml:"UID"`
		Name string `xml:"Name"`
	} `xml:"Resources>Resource"`
	Assignments []struct {
		TaskUID     string  `xml:"TaskUID"`
		ResourceUID string  `xml:"ResourceUID"`
		Units       float64 `xml:"Units"`
	} `xml:"Assignments>Assignment"`
}

// mspdiHoursPerDay is the working-day length MSPDI durations and lags
// are converted with, matching the exporter's isoDurationDays.
const mspdiHoursPerDay = 8.0

// FromMSPDI parses a Microsoft Project Data Interchange XML document
// into PMForge's import shape.
//
// Conversions and conventions:
//
//   - Durations: ISO 8601 PT<h>H<m>M<s>S → working days at 8 h/day.
//   - PredecessorLink Type: 0=FF, 1=FS (default when absent),
//     2=SF, 3=SS. LinkLag is in tenths of a minute → days.
//   - Summary tasks (containers) and null rows are skipped — PMForge
//     CPM charts are flat activity graphs.
//   - Resource assignments are flattened to resource NAMES (PMForge's
//     assignment key); Units pass through (MSPDI uses 1.0 =
//     full-time, same as PMForge).
//   - The project StartDate is reduced to YYYY-MM-DD for
//     project.start_date compatibility.
func FromMSPDI(data []byte) (ImportedProject, error) {
	var raw mspdiImport
	if err := xml.Unmarshal(data, &raw); err != nil {
		return ImportedProject{}, err
	}

	out := ImportedProject{Title: raw.Title}
	if out.Title == "" {
		out.Title = raw.Name
	}
	if len(raw.StartDate) >= 10 {
		out.StartDate = raw.StartDate[:10]
	}

	resourceNames := make(map[string]string, len(raw.Resources))
	for _, r := range raw.Resources {
		if r.Name != "" {
			resourceNames[r.UID] = r.Name
		}
	}

	assignmentsByTask := make(map[string][]kernel.Assignment)
	for _, a := range raw.Assignments {
		name, ok := resourceNames[a.ResourceUID]
		if !ok {
			continue
		}
		assignmentsByTask[a.TaskUID] = append(assignmentsByTask[a.TaskUID], kernel.Assignment{
			Resource: name,
			Units:    a.Units,
		})
	}

	imported := make(map[string]bool)
	for _, t := range raw.Tasks {
		if t.UID == "" || t.IsNull == "1" || t.Summary == "1" {
			continue
		}
		task := ImportedTask{
			UID:             t.UID,
			Name:            t.Name,
			DurationDays:    isoHoursToDays(t.Duration),
			Milestone:       t.Milestone == "1",
			PercentComplete: t.PercentComplete,
			Assignments:     assignmentsByTask[t.UID],
		}
		for _, p := range t.Predecessors {
			if p.PredecessorUID == "" {
				continue
			}
			typ := kernel.FinishToStart // MSPDI default Type=1
			if p.Type != nil {
				switch *p.Type {
				case 0:
					typ = kernel.FinishToFinish
				case 2:
					typ = kernel.StartToFinish
				case 3:
					typ = kernel.StartToStart
				}
			}
			task.Links = append(task.Links, kernel.Link{
				Pred: p.PredecessorUID,
				Type: typ,
				// Tenths of a minute → days.
				Lag: p.LinkLag / 10 / 60 / mspdiHoursPerDay,
			})
		}
		out.Tasks = append(out.Tasks, task)
		imported[t.UID] = true
	}

	if len(out.Tasks) == 0 {
		return ImportedProject{}, errors.New("mspdi: no importable tasks found")
	}

	// Drop links pointing at skipped rows (summary/null parents).
	for i := range out.Tasks {
		kept := out.Tasks[i].Links[:0]
		for _, l := range out.Tasks[i].Links {
			if imported[l.Pred] {
				kept = append(kept, l)
			}
		}
		out.Tasks[i].Links = kept
	}

	return out, nil
}

// isoHoursToDays converts MSPDI's PT<h>H<m>M<s>S duration form into
// working days (8 h/day). Unparseable input yields 0 — MSPDI
// milestones legitimately carry PT0H0M0S.
func isoHoursToDays(iso string) float64 {
	s := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(iso)), "PT")
	if s == "" {
		return 0
	}
	var hours, minutes, seconds float64
	num := ""
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9' || r == '.':
			num += string(r)
		case r == 'H':
			hours = parseFloatSoft(num)
			num = ""
		case r == 'M':
			minutes = parseFloatSoft(num)
			num = ""
		case r == 'S':
			seconds = parseFloatSoft(num)
			num = ""
		default:
			return 0
		}
	}
	return (hours + minutes/60 + seconds/3600) / mspdiHoursPerDay
}

func parseFloatSoft(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}
