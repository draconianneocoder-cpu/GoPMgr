// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"encoding/xml"
	"math"
	"sort"
	"strings"
	"time"

	"gopmgr/internal/kernel"
)

// MSPDI Project root element.
type mspdiProject struct {
	XMLName     xml.Name          `xml:"Project"`
	Xmlns       string            `xml:"xmlns,attr"`
	Title       string            `xml:"Title"`
	Tasks       []mspdiTask       `xml:"Tasks>Task"`
	Resources   []mspdiResource   `xml:"Resources>Resource,omitempty"`
	Assignments []mspdiAssignment `xml:"Assignments>Assignment,omitempty"`
}

type mspdiTask struct {
	UID             string `xml:"UID"`
	ID              string `xml:"ID"`
	Name            string `xml:"Name"`
	Start           string `xml:"Start"`
	Finish          string `xml:"Finish"`
	Duration        string `xml:"Duration"`
	Milestone       string `xml:"Milestone"`
	PercentComplete int    `xml:"PercentComplete"`
	// ConstraintType and ConstraintDate follow PercentComplete and come
	// before PredecessorLink, in the schema's element order. Both are
	// left out for ASAP and for a constraint that cannot be written.
	ConstraintType *int   `xml:"ConstraintType,omitempty"`
	ConstraintDate string `xml:"ConstraintDate,omitempty"`
	// LevelingDelay and LevelingDelayFormat follow ConstraintDate and
	// come before PredecessorLink (schema order); the delay is in the
	// format's units (8 = elapsed days, 4 = elapsed minutes). Left out
	// when levelling did not delay the task.
	LevelingDelay       *int        `xml:"LevelingDelay,omitempty"`
	LevelingDelayFormat *int        `xml:"LevelingDelayFormat,omitempty"`
	Predecessors        []mspdiLink `xml:"PredecessorLink,omitempty"`
}

type mspdiLink struct {
	PredecessorUID string `xml:"PredecessorUID"`
	Type           int    `xml:"Type"`
	LinkLag        int    `xml:"LinkLag"` // tenths of a minute
}

type mspdiResource struct {
	UID  string `xml:"UID"`
	Name string `xml:"Name"`
}

type mspdiAssignment struct {
	TaskUID     string  `xml:"TaskUID"`
	ResourceUID string  `xml:"ResourceUID"`
	Units       float64 `xml:"Units"`
}

// mspdiLinkType maps kernel link types onto MSPDI's PredecessorLink
// Type enumeration (0=FF, 1=FS, 2=SF, 3=SS).
func mspdiLinkType(t kernel.LinkType) int {
	switch t {
	case kernel.FinishToFinish:
		return 0
	case kernel.StartToFinish:
		return 2
	case kernel.StartToStart:
		return 3
	default:
		return 1 // FS
	}
}

// ToMSPDI converts a CPM task map into a Microsoft Project Data
// Interchange XML document. The output starts with `xml.Header` so
// Microsoft Project 2016+ accepts the file as archival-quality.
//
// Durations are encoded in ISO 8601 (PT<hours>H0M0S) — MSPDI's native
// representation.
//
// Start/Finish dates: if the task map has been calendar-anchored via
// kernel.AnchorSchedule (Task.StartDate / Task.FinishDate populated),
// those real dates are emitted with times on Microsoft Project's standard
// day (kernel.StartClock, kernel.FinishClock): a whole-day task starts at
// 08:00 and finishes at 17:00, and a part-day start or finish falls at its
// working minute. Un-anchored task maps fall back to the legacy behaviour
// of offsetting from "today".
//
// levelingDelays gives, by task ID, the levelling delay in elapsed minutes
// (kernel.LevelingDelayMinutes); each is written as LevelingDelay in tenths
// of elapsed minutes, shown in elapsed days (format 8) when it is a whole
// number of days, else in elapsed minutes (format 4) (owner decision
// 2026-11-23). nil writes none.
//
// Tasks are emitted in (ES, ID) order so the XML is deterministic for
// a given schedule — byte-identical output for snapshot tests and
// reproducible archives.
func ToMSPDI(title string, tasks map[string]*kernel.Task, levelingDelays map[string]int) ([]byte, error) {
	epoch := time.Now().UTC()
	project := mspdiProject{
		Xmlns: "http://schemas.microsoft.com/project",
		Title: title,
	}

	ordered := make([]*kernel.Task, 0, len(tasks))
	for _, t := range tasks {
		ordered = append(ordered, t)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].ES != ordered[j].ES {
			return ordered[i].ES < ordered[j].ES
		}
		return ordered[i].ID < ordered[j].ID
	})

	resourceUIDs := make(map[string]string) // name -> stable UID
	var resourceOrder []string

	for _, t := range ordered {
		start := epoch.AddDate(0, 0, int(t.ES)).Format("2006-01-02T15:04:05")
		finish := epoch.AddDate(0, 0, int(t.EF)).Format("2006-01-02T15:04:05")
		if t.StartDate != "" {
			start = t.StartDate + startClock(t)
		}
		if t.FinishDate != "" {
			finish = t.FinishDate + finishClock(t)
		}

		task := mspdiTask{
			UID:             t.ID,
			ID:              t.ID,
			Name:            t.Title,
			Start:           start,
			Finish:          finish,
			Duration:        isoDurationDays(t.Duration),
			Milestone:       boolFlag(t.Milestone || t.Duration == 0),
			PercentComplete: int(t.PercentComplete + 0.5),
		}
		task.ConstraintType, task.ConstraintDate = mspdiConstraintFor(t)
		task.LevelingDelay, task.LevelingDelayFormat = mspdiLevelingDelay(levelingDelays[t.ID])
		// Typed links (legacy Precedents become FS+0 via the same
		// merge the scheduler uses).
		for _, l := range kernel.EffectiveLinks(t) {
			task.Predecessors = append(task.Predecessors, mspdiLink{
				PredecessorUID: l.Pred,
				Type:           mspdiLinkType(l.Type),
				// Days -> tenths of a minute at 8 h/day, rounded to the
				// nearest, so a lead (negative lag) rounds as a lag does.
				LinkLag: int(math.Round(l.Lag * 8 * 60 * 10)),
			})
		}
		project.Tasks = append(project.Tasks, task)

		for _, a := range t.Assignments {
			if a.Resource == "" {
				continue
			}
			uid, seen := resourceUIDs[a.Resource]
			if !seen {
				uid = "r" + itoa(len(resourceUIDs)+1)
				resourceUIDs[a.Resource] = uid
				resourceOrder = append(resourceOrder, a.Resource)
			}
			units := a.Units
			if units <= 0 {
				units = 1
			}
			project.Assignments = append(project.Assignments, mspdiAssignment{
				TaskUID:     t.ID,
				ResourceUID: uid,
				Units:       units,
			})
		}
	}

	for _, name := range resourceOrder {
		project.Resources = append(project.Resources, mspdiResource{
			UID:  resourceUIDs[name],
			Name: name,
		})
	}

	body, err := xml.MarshalIndent(project, "", "  ")
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	buf.Write(body)
	return buf.Bytes(), nil
}

// startClock is the time a task's start is written with: 08:00 for a
// whole-day start, else the working minute its share of the day reaches.
func startClock(t *kernel.Task) string {
	_, f := kernel.SplitDayOffset(t.ES)
	h, m := kernel.StartClock(f)
	return clockText(h, m)
}

// finishClock is the time a task's finish is written with: 17:00 for work
// that ends with the day, else the working minute it ends at. A milestone
// finishes when it starts.
func finishClock(t *kernel.Task) string {
	if t.Duration <= 0 {
		if _, f := kernel.SplitDayOffset(t.ES); f > 0 {
			return startClock(t)
		}
		return "T17:00:00"
	}
	last := math.Ceil(t.EF-1e-9) - 1
	h, m := kernel.FinishClock(t.EF - last)
	return clockText(h, m)
}

func clockText(h, m int) string {
	return "T" + twoDigits(h) + ":" + twoDigits(m) + ":00"
}

func twoDigits(n int) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

// LevelingDelayFormat codes for the units GoPMgr writes (Microsoft's
// duration format codes; the import accepts every documented unit).
const (
	mspdiElapsedMinutes = 4
	mspdiElapsedDays    = 8
)

// mspdiLevelingDelay is a delay of minutes elapsed minutes as MSPDI's
// LevelingDelay and LevelingDelayFormat. The value is in tenths of elapsed
// minutes, as Microsoft Project files carry it (MPXJ reads and writes it
// so; owner decision 2026-11-23); the format is only the unit the delay is
// shown in: elapsed days for a whole number of days, else elapsed
// minutes. None for no delay.
func mspdiLevelingDelay(minutes int) (*int, *int) {
	if minutes <= 0 {
		return nil, nil
	}
	value, format := minutes*10, mspdiElapsedMinutes
	if minutes%(24*60) == 0 {
		format = mspdiElapsedDays
	}
	return &value, &format
}

// boolFlag renders a Go bool as MSPDI's "1"/"0" flag form.
func boolFlag(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// isoDurationDays converts a duration expressed in days into MSPDI's
// "PT<n>H0M0S" form, treating 8 working hours per day.
func isoDurationDays(days float64) string {
	hours := int(days*8 + 0.5)
	if hours < 0 {
		hours = 0
	}
	return formatISOHours(hours)
}

func formatISOHours(hours int) string {
	return "PT" + itoa(hours) + "H0M0S"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// mspdiConstraintFor is a task's constraint in MSPDI terms: the type code,
// and for a dated type its date at the start (08:00) or end (17:00) of the
// working day it limits. ASAP, an unknown type, and a dated type without a
// valid date write nothing.
func mspdiConstraintFor(t *kernel.Task) (*int, string) {
	c := kernel.ConstraintType(strings.ToUpper(strings.TrimSpace(string(t.Constraint))))
	if c == kernel.AsLateAsPossible {
		code := 1
		return &code, ""
	}
	if !kernel.IsDateConstraint(c) {
		return nil, ""
	}
	if _, err := time.Parse(kernel.DateLayout, t.ConstraintDate); err != nil {
		return nil, ""
	}
	for code, known := range mspdiConstraintTypes {
		if known == c {
			clock := "T08:00:00"
			if c == kernel.FinishNoEarlierThan || c == kernel.FinishNoLaterThan || c == kernel.MustFinishOn {
				clock = "T17:00:00"
			}
			return &code, t.ConstraintDate + clock
		}
	}
	return nil, ""
}
