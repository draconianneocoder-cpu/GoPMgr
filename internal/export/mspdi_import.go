// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"encoding/xml"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"gopmgr/internal/kernel"
)

// ImportedTask is one schedulable activity read from an MSPDI file,
// already converted to GoPMgr conventions (durations in working
// days, link lag in days).
type ImportedTask struct {
	UID             string
	Name            string
	DurationDays    float64
	Milestone       bool
	PercentComplete float64
	Links           []kernel.Link
	Assignments     []kernel.Assignment
	// Constraint and ConstraintDate (YYYY-MM-DD) are the task's
	// scheduling constraint; empty for ASAP or when not imported.
	Constraint     kernel.ConstraintType
	ConstraintDate string
	// LevelingDelay is the task's levelling delay read from the file
	// (LevelingDelay in LevelingDelayFormat units), nil when it has none
	// or it is not imported. The caller applies it on the file's full
	// network (ImportedProject.LevelingNetwork).
	LevelingDelay *kernel.LevelingDelay
}

// ImportedProject is the result of FromMSPDIWithOptions.
type ImportedProject struct {
	Title     string
	StartDate string // YYYY-MM-DD, or "" when the file has none
	Tasks     []ImportedTask
	Receipt   MSPDIImportReceipt
	// LevelingNetwork is every importable task with all its links and
	// constraints, whatever the options, set when some task has a
	// levelling delay: delays are measured on the file's own network, so
	// they give the same starts whether or not links and constraints are
	// imported.
	LevelingNetwork []ImportedTask
}

// MSPDIImportOptions lets a user decide which logical schedule fields become
// editable GoPMgr data before the file picker is opened.
type MSPDIImportOptions struct {
	IncludeDependencies bool `json:"include_dependencies"`
	IncludeProgress     bool `json:"include_progress"`
	IncludeAssignments  bool `json:"include_assignments"`
	// IncludeConstraints imports task constraint types and dates. Callers
	// that predate it leave it false, which keeps their earlier result.
	IncludeConstraints bool `json:"include_constraints"`
	// IncludeLevelingDelay imports each task's levelling delay as its
	// levelled start. Callers that predate it leave it false.
	IncludeLevelingDelay bool `json:"include_leveling_delay"`
}

// MSPDIImportReceipt records exactly which source fields were preserved,
// intentionally transformed, or excluded, so an import can be reviewed and
// repeated without guessing at data loss.
type MSPDIImportReceipt struct {
	ImportedFields     []string `json:"imported_fields"`
	ExcludedFields     []string `json:"excluded_fields"`
	Transformations    []string `json:"transformations"`
	SkippedSummaryRows int      `json:"skipped_summary_rows"`
	SkippedNullRows    int      `json:"skipped_null_rows"`
	// SkippedConstraints counts constraints left out because their type is
	// unknown or a dated type has no usable date.
	SkippedConstraints int `json:"skipped_constraints"`
	// SkippedLevelingDelays counts levelling delays left out: an
	// unreadable value, a format with no fixed length (months, percent)
	// or none, a delay on an ALAP task, one beyond the schedule limit,
	// or all of them when the project has no start date.
	SkippedLevelingDelays int `json:"skipped_leveling_delays"`
}

// mspdiImport mirrors the subset of the MSPDI schema GoPMgr reads.
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
		ConstraintType  *int    `xml:"ConstraintType"`
		ConstraintDate  string  `xml:"ConstraintDate"`
		// Read as text, so a malformed value is skipped, not fatal.
		LevelingDelay       string `xml:"LevelingDelay"`
		LevelingDelayFormat string `xml:"LevelingDelayFormat"`
		Predecessors        []struct {
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

// FromMSPDIWithOptions parses a Microsoft Project Data Interchange XML
// document into GoPMgr's import shape, importing the fields options
// selects (with every option on, everything below is imported).
//
// Conversions and conventions:
//
//   - Durations: ISO 8601 PT<h>H<m>M<s>S → working days at 8 h/day.
//   - PredecessorLink Type: 0=FF, 1=FS (default when absent),
//     2=SF, 3=SS. LinkLag is in tenths of a minute → days.
//   - Summary tasks (containers) and null rows are skipped — GoPMgr
//     CPM charts are flat activity graphs.
//   - Resource assignments are flattened to resource NAMES (GoPMgr's
//     assignment key); Units pass through (MSPDI uses 1.0 =
//     full-time, same as GoPMgr).
//   - The project StartDate is reduced to YYYY-MM-DD for
//     project.start_date compatibility.
//   - ConstraintType: 0=ASAP (stored as none), 1=ALAP, 2=MSO, 3=MFO,
//     4=SNET, 5=SNLT, 6=FNET, 7=FNLT; ConstraintDate is reduced to
//     YYYY-MM-DD. An unknown type, or a dated type without a usable date,
//     is left out and counted in the receipt.
func FromMSPDIWithOptions(data []byte, options MSPDIImportOptions) (ImportedProject, error) {
	var raw mspdiImport
	if err := xml.Unmarshal(data, &raw); err != nil {
		return ImportedProject{}, err
	}

	out := ImportedProject{Title: raw.Title, Receipt: MSPDIImportReceipt{
		ImportedFields:  []string{"project.title", "project.start_date", "task.uid", "task.name", "task.duration", "task.milestone"},
		Transformations: []string{"project.start_date converted to YYYY-MM-DD", "task.duration converted from ISO 8601 hours to 8-hour working days"},
	}}
	if !options.IncludeDependencies {
		out.Receipt.ExcludedFields = append(out.Receipt.ExcludedFields, "task.predecessor_links (user selection)")
	} else {
		out.Receipt.ImportedFields = append(out.Receipt.ImportedFields, "task.predecessor_links")
		out.Receipt.Transformations = append(out.Receipt.Transformations, "predecessor lag converted from tenths of minutes to working days")
	}
	if !options.IncludeProgress {
		out.Receipt.ExcludedFields = append(out.Receipt.ExcludedFields, "task.percent_complete (user selection)")
	} else {
		out.Receipt.ImportedFields = append(out.Receipt.ImportedFields, "task.percent_complete")
	}
	if !options.IncludeAssignments {
		out.Receipt.ExcludedFields = append(out.Receipt.ExcludedFields, "resource.assignments (user selection)")
	} else {
		out.Receipt.ImportedFields = append(out.Receipt.ImportedFields, "resource.assignments")
		out.Receipt.Transformations = append(out.Receipt.Transformations, "resource UID converted to resource name for GoPMgr assignments")
	}
	if !options.IncludeConstraints {
		out.Receipt.ExcludedFields = append(out.Receipt.ExcludedFields, "task.constraints (user selection)")
	} else {
		out.Receipt.ImportedFields = append(out.Receipt.ImportedFields, "task.constraint_type", "task.constraint_date")
		out.Receipt.Transformations = append(out.Receipt.Transformations,
			"task.constraint_date reduced to YYYY-MM-DD",
			"constraints keep their type, but GoPMgr schedules from the links first and flags a constraint they cannot meet, where Microsoft Project by default holds the task to the date")
	}
	if !options.IncludeLevelingDelay {
		out.Receipt.ExcludedFields = append(out.Receipt.ExcludedFields, "task.leveling_delay (user selection)")
	} else {
		out.Receipt.ImportedFields = append(out.Receipt.ImportedFields, "task.leveling_delay")
		out.Receipt.Transformations = append(out.Receipt.Transformations,
			"task.leveling_delay applied as a levelled start (date and share of the day) on the file's links and constraints, whatever was imported; elapsed delays follow the 08:00-12:00, 13:00-17:00 working day")
	}
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
		if !options.IncludeAssignments {
			break
		}
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
		if t.IsNull == "1" {
			out.Receipt.SkippedNullRows++
			continue
		}
		if t.Summary == "1" {
			out.Receipt.SkippedSummaryRows++
			continue
		}
		if t.UID == "" {
			continue
		}
		task := ImportedTask{
			UID:          t.UID,
			Name:         t.Name,
			DurationDays: isoHoursToDays(t.Duration),
			Milestone:    t.Milestone == "1",
			Assignments:  assignmentsByTask[t.UID],
		}
		if options.IncludeProgress {
			task.PercentComplete = t.PercentComplete
		}
		if options.IncludeLevelingDelay {
			switch delay, status := parseLevelingDelay(t.LevelingDelay, t.LevelingDelayFormat); status {
			case delayRead:
				task.LevelingDelay = &delay
			case delaySkipped:
				out.Receipt.SkippedLevelingDelays++
			}
		}
		if options.IncludeConstraints && t.ConstraintType != nil {
			c, date, ok := mspdiConstraint(*t.ConstraintType, t.ConstraintDate)
			if ok {
				task.Constraint, task.ConstraintDate = c, date
			} else {
				out.Receipt.SkippedConstraints++
			}
		}
		if !options.IncludeAssignments {
			task.Assignments = nil
		}
		if !options.IncludeDependencies {
			out.Tasks = append(out.Tasks, task)
			imported[t.UID] = true
			continue
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

	for _, t := range out.Tasks {
		if t.LevelingDelay == nil {
			continue
		}
		if options.IncludeDependencies && options.IncludeConstraints {
			out.LevelingNetwork = out.Tasks
			break
		}
		// Links and constraints on, so this parse does not recurse.
		full, err := FromMSPDIWithOptions(data, MSPDIImportOptions{IncludeDependencies: true, IncludeConstraints: true, IncludeLevelingDelay: true})
		if err != nil {
			return ImportedProject{}, err
		}
		out.LevelingNetwork = full.Tasks
		break
	}

	return out, nil
}

type delayStatus int

const (
	delayNone    delayStatus = iota // no delay, or zero
	delayRead                       // a delay to apply
	delaySkipped                    // a delay that cannot be read
)

// parseLevelingDelay converts LevelingDelay and LevelingDelayFormat. The
// value is in the format's units (Microsoft's documentation: a delay of 3
// with format 7 is 3 days): 3 working minutes (480 a day), 4 elapsed
// minutes, 5 working hours (8 a day), 6 elapsed hours, 7 working days, 8
// elapsed days, 9 working weeks (5 days), 10 elapsed weeks; the estimated
// codes 35 to 42 read as 3 to 10. Months, percent and unknown or missing
// formats, and a value that is not a finite, non-negative number, are
// skipped.
func parseLevelingDelay(rawValue, rawFormat string) (kernel.LevelingDelay, delayStatus) {
	text := strings.TrimSpace(rawValue)
	if text == "" {
		return kernel.LevelingDelay{}, delayNone
	}
	v, err := strconv.ParseFloat(text, 64)
	switch {
	case err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0:
		return kernel.LevelingDelay{}, delaySkipped
	case v == 0:
		return kernel.LevelingDelay{}, delayNone
	}
	format, err := strconv.Atoi(strings.TrimSpace(rawFormat))
	if err != nil {
		return kernel.LevelingDelay{}, delaySkipped
	}
	if format >= 35 && format <= 42 {
		format -= 32
	}
	const day, workMinutes = 24 * 60, 8 * 60
	switch format {
	case 3:
		return kernel.LevelingDelay{Amount: v / workMinutes}, delayRead
	case 4:
		return kernel.LevelingDelay{Amount: v, Elapsed: true}, delayRead
	case 5:
		return kernel.LevelingDelay{Amount: v / mspdiHoursPerDay}, delayRead
	case 6:
		return kernel.LevelingDelay{Amount: v * 60, Elapsed: true}, delayRead
	case 7:
		return kernel.LevelingDelay{Amount: v}, delayRead
	case 8:
		return kernel.LevelingDelay{Amount: v * day, Elapsed: true}, delayRead
	case 9:
		return kernel.LevelingDelay{Amount: v * 5}, delayRead
	case 10:
		return kernel.LevelingDelay{Amount: v * 7 * day, Elapsed: true}, delayRead
	}
	return kernel.LevelingDelay{}, delaySkipped
}

// mspdiConstraintTypes maps MSPDI ConstraintType codes to GoPMgr types.
var mspdiConstraintTypes = map[int]kernel.ConstraintType{
	0: kernel.AsSoonAsPossible,
	1: kernel.AsLateAsPossible,
	2: kernel.MustStartOn,
	3: kernel.MustFinishOn,
	4: kernel.StartNoEarlierThan,
	5: kernel.StartNoLaterThan,
	6: kernel.FinishNoEarlierThan,
	7: kernel.FinishNoLaterThan,
}

// mspdiConstraint converts an MSPDI constraint. ASAP is returned as no
// constraint. ok is false for an unknown type or a dated type whose date
// does not begin with a YYYY-MM-DD date.
func mspdiConstraint(code int, rawDate string) (kernel.ConstraintType, string, bool) {
	c, known := mspdiConstraintTypes[code]
	switch {
	case !known:
		return "", "", false
	case c == kernel.AsSoonAsPossible:
		return "", "", true
	case !kernel.IsDateConstraint(c):
		return c, "", true
	}
	date := strings.TrimSpace(rawDate)
	if len(date) < len(kernel.DateLayout) {
		return "", "", false
	}
	date = date[:len(kernel.DateLayout)]
	if _, err := time.Parse(kernel.DateLayout, date); err != nil {
		return "", "", false
	}
	return c, date, true
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
