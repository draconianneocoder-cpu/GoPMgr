// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"encoding/xml"
	"time"

	"pmforge/internal/kernel"
)

// MSPDI Project root element.
type mspdiProject struct {
	XMLName xml.Name    `xml:"Project"`
	Xmlns   string      `xml:"xmlns,attr"`
	Title   string      `xml:"Title"`
	Tasks   []mspdiTask `xml:"Tasks>Task"`
}

type mspdiTask struct {
	UID      string `xml:"UID"`
	ID       string `xml:"ID"`
	Name     string `xml:"Name"`
	Start    string `xml:"Start"`
	Finish   string `xml:"Finish"`
	Duration string `xml:"Duration"`
}

// ToMSPDI converts a CPM task map into a Microsoft Project Data
// Interchange XML document. The output starts with `xml.Header` so
// Microsoft Project 2016+ accepts the file as archival-quality.
//
// Durations are encoded in ISO 8601 (PT<hours>H0M0S) — MSPDI's native
// representation. Start/Finish are derived from a project epoch of
// "today" because the kernel currently models duration in days
// without an absolute anchor. When PMForge gains a project_start_date
// field, replace `epoch` below with that value.
func ToMSPDI(title string, tasks map[string]*kernel.Task) ([]byte, error) {
	epoch := time.Now().UTC()
	project := mspdiProject{
		Xmlns: "http://schemas.microsoft.com/project",
		Title: title,
	}

	for _, t := range tasks {
		project.Tasks = append(project.Tasks, mspdiTask{
			UID:      t.ID,
			ID:       t.ID,
			Name:     t.Title,
			Start:    epoch.AddDate(0, 0, int(t.ES)).Format("2006-01-02T15:04:05"),
			Finish:   epoch.AddDate(0, 0, int(t.EF)).Format("2006-01-02T15:04:05"),
			Duration: isoDurationDays(t.Duration),
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
