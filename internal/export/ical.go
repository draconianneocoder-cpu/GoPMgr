// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"gopmgr/internal/calendar"
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
	Category    string    // optional CATEGORIES value (sprint_start / sprint_end / deployment / milestone[_charter|_chart] / project_start / project_end / holiday)
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
	w.line("PRODID:-//GoPMgr//" + exportVersion() + "//EN")
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
		w.kv("UID", ev.UID+"@gopmgr.local")
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
	// Fold at 75 octets per line, a continuation's leading space included,
	// and never inside a UTF-8 character (RFC 5545 section 3.1), so each
	// line stays valid UTF-8. Invalid UTF-8 has no boundary and is cut at
	// the limit.
	const limit = 75
	room := limit
	for len(s) > room {
		cut := room
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		if cut == 0 {
			cut = room
		}
		w.buf.WriteString(s[:cut])
		w.buf.WriteString("\r\n ")
		s = s[cut:]
		room = limit - 1
	}
	w.buf.WriteString(s)
	w.buf.WriteString("\r\n")
}

func escapeText(s string) string {
	// A Windows or classic Mac line break is one escaped newline too; a
	// bare CR would end the content line early.
	r := strings.NewReplacer(
		`\`, `\\`,
		`;`, `\;`,
		`,`, `\,`,
		"\r\n", `\n`,
		"\r", `\n`,
		"\n", `\n`,
	)
	return r.Replace(s)
}
