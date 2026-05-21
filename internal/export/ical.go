// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"pmforge/internal/calendar"
)

// ICalEvent is one timeline entry that ICalRender turns into a
// VEVENT. The fields map directly onto the iCal properties of the
// same name.
type ICalEvent struct {
	UID         string    // unique within the calendar; we use the source record's ID
	Summary     string    // event title
	Description string    // multi-line body (newlines are CRLF-escaped)
	Start       time.Time // event start (UTC)
	End         time.Time // event end; if zero, treated as all-day at Start
	Category    string    // optional CATEGORIES value (sprint / milestone / holiday / doc)
}

// ICalSpec is the input for ICalRender.
type ICalSpec struct {
	CalendarName string // "X-WR-CALNAME" — shown by clients as the calendar's display name
	ProjectID    string // used in PRODID for traceability
	Events       []ICalEvent
}

// ICalRender produces an RFC 5545 calendar document. The output is
// already wrapped at 75 octets and CRLF-terminated.
//
// Holiday events are emitted as separate VEVENTs (Category=holiday)
// by the caller — the calendar package supplies the dates. Keeping
// holiday assembly out of this function makes ICalRender pure-data:
// pass it events, get back text.
func ICalRender(spec ICalSpec) []byte {
	var buf bytes.Buffer
	w := newICalWriter(&buf)

	w.line("BEGIN:VCALENDAR")
	w.line("VERSION:2.0")
	w.line("PRODID:-//PMForge//" + exportVersion() + "//EN")
	w.line("CALSCALE:GREGORIAN")
	w.line("METHOD:PUBLISH")
	if spec.CalendarName != "" {
		w.kv("X-WR-CALNAME", spec.CalendarName)
	}
	if spec.ProjectID != "" {
		w.kv("X-WR-RELCALID", spec.ProjectID)
	}

	for _, ev := range spec.Events {
		w.line("BEGIN:VEVENT")
		w.kv("UID", ev.UID+"@pmforge.local")
		w.kv("DTSTAMP", iCalDateTime(time.Now().UTC()))
		if ev.End.IsZero() {
			// All-day event: DTSTART;VALUE=DATE with no time.
			w.line("DTSTART;VALUE=DATE:" + iCalDate(ev.Start))
		} else {
			w.line("DTSTART:" + iCalDateTime(ev.Start.UTC()))
			w.line("DTEND:" + iCalDateTime(ev.End.UTC()))
		}
		w.kv("SUMMARY", ev.Summary)
		if ev.Description != "" {
			w.kv("DESCRIPTION", ev.Description)
		}
		if ev.Category != "" {
			w.kv("CATEGORIES", strings.ToUpper(ev.Category))
		}
		w.line("END:VEVENT")
	}

	w.line("END:VCALENDAR")
	return buf.Bytes()
}

// AppendHolidayEvents augments a spec with one all-day VEVENT per
// holiday returned by the calendar wrapper. Call this when the user
// asks the GUI to "Include holidays in iCal export".
func AppendHolidayEvents(spec ICalSpec, cal *calendar.Calendar, from, to time.Time) ICalSpec {
	for _, h := range cal.HolidaysIn(from, to) {
		spec.Events = append(spec.Events, ICalEvent{
			UID:      fmt.Sprintf("holiday-%s-%s", cal.CountryCode, h.Date.Format("20060102")),
			Summary:  h.Name,
			Start:    h.Date,
			Category: "holiday",
		})
	}
	return spec
}

// ----- RFC 5545 encoding helpers -----

func iCalDate(t time.Time) string {
	return t.Format("20060102")
}

func iCalDateTime(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

// icalWriter handles the 75-octet line folding required by RFC 5545
// and the special character escaping for text values (commas,
// semicolons, backslashes, newlines).
type icalWriter struct {
	buf *bytes.Buffer
}

func newICalWriter(b *bytes.Buffer) *icalWriter { return &icalWriter{buf: b} }

func (w *icalWriter) line(s string) {
	w.write(s)
}

// kv writes a `KEY:VALUE` line with proper text escaping on VALUE.
// Use for human-readable fields (SUMMARY, DESCRIPTION, ...).
func (w *icalWriter) kv(key, value string) {
	w.write(key + ":" + escapeText(value))
}

func (w *icalWriter) write(s string) {
	// Fold any line exceeding 75 octets with CRLF + a single space.
	const limit = 75
	if len(s) <= limit {
		w.buf.WriteString(s)
		w.buf.WriteString("\r\n")
		return
	}
	first := s[:limit]
	rest := s[limit:]
	w.buf.WriteString(first)
	w.buf.WriteString("\r\n")
	for len(rest) > 0 {
		// Folded continuation lines are at most 75 octets INCLUDING
		// the leading space.
		n := limit - 1
		if n > len(rest) {
			n = len(rest)
		}
		w.buf.WriteByte(' ')
		w.buf.WriteString(rest[:n])
		w.buf.WriteString("\r\n")
		rest = rest[n:]
	}
}

func escapeText(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`;`, `\;`,
		`,`, `\,`,
		"\n", `\n`,
	)
	return r.Replace(s)
}
