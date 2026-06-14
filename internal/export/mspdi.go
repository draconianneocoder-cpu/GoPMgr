// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"encoding/xml"
	"sort"
	"time"

	"pmforge/internal/kernel"
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
	UID             string      `xml:"UID"`
	ID              string      `xml:"ID"`
	Name            string      `xml:"Name"`
	Start           string      `xml:"Start"`
	Finish          string      `xml:"Finish"`
	Duration        string      `xml:"Duration"`
	Milestone       string      `xml:"Milestone"`
	PercentComplete int         `xml:"PercentComplete"`
	Predecessors    []mspdiLink `xml:"PredecessorLink,omitempty"`
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
// those real dates are emitted with the standard 08:00 start and
// 17:00 finish times. Un-anchored task maps fall back to the legacy
// behaviour of offsetting from "today".
//
// Tasks are emitted in (ES, ID) order so the XML is deterministic for
// a given schedule — byte-identical output for snapshot tests and
// reproducible archives.
func ToMSPDI(title string, tasks map[string]*kernel.Task) ([]byte, error) {
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
			start = t.StartDate + "T08:00:00"
		}
		if t.FinishDate != "" {
			finish = t.FinishDate + "T17:00:00"
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
		// Typed links (legacy Precedents become FS+0 via the same
		// merge the scheduler uses).
		for _, l := range kernel.EffectiveLinks(t) {
			task.Predecessors = append(task.Predecessors, mspdiLink{
				PredecessorUID: l.Pred,
				Type:           mspdiLinkType(l.Type),
				// Days -> tenths of a minute at 8 h/day.
				LinkLag: int(l.Lag*8*60*10 + 0.5),
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
