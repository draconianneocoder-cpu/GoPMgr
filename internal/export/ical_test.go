// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"gopmgr/internal/calendar"
)

// icalLines splits a calendar into its physical lines, failing on a line
// that does not end in CRLF.
func icalLines(t *testing.T, raw []byte) []string {
	t.Helper()
	text := string(raw)
	if !strings.HasSuffix(text, "\r\n") {
		t.Fatalf("calendar does not end in CRLF: %q", text[max(0, len(text)-20):])
	}
	lines := strings.Split(strings.TrimSuffix(text, "\r\n"), "\r\n")
	for _, l := range lines {
		if strings.ContainsAny(l, "\r\n") {
			t.Fatalf("line %q holds a bare CR or LF", l)
		}
	}
	return lines
}

// unfold joins folded continuation lines (RFC 5545 section 3.1).
func unfold(lines []string) []string {
	var out []string
	for _, l := range lines {
		if strings.HasPrefix(l, " ") && len(out) > 0 {
			out[len(out)-1] += l[1:]
			continue
		}
		out = append(out, l)
	}
	return out
}

func withExportVersion(t *testing.T, v string) {
	t.Helper()
	old := exportVersion
	SetVersion(v)
	t.Cleanup(func() { exportVersion = old })
}

func TestICalRenderWritesAPublishableCalendar(t *testing.T) {
	withExportVersion(t, "9.9.9")
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	raw := ICalRender(ICalSpec{
		CalendarName: "Line 3, Phase 2",
		ProjectID:    "prj-1",
		Events: []ICalEvent{
			{
				UID: "sprint-4", Summary: "Sprint 4; review", Description: "Demo\nRetro",
				Start: time.Date(2026, 10, 24, 9, 0, 0, 0, tokyo), End: time.Date(2026, 10, 24, 17, 30, 0, 0, tokyo),
				Category: "sprint_end",
			},
			{UID: "m1", Summary: "Go-live", Start: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Category: "milestone"},
		},
	})
	lines := unfold(icalLines(t, raw))

	if lines[0] != "BEGIN:VCALENDAR" || lines[len(lines)-1] != "END:VCALENDAR" {
		t.Fatalf("calendar is not wrapped in VCALENDAR: first %q, last %q", lines[0], lines[len(lines)-1])
	}
	for _, want := range []string{
		"VERSION:2.0",
		"PRODID:-//GoPMgr//9.9.9//EN",
		"CALSCALE:GREGORIAN",
		"METHOD:PUBLISH",
		`X-WR-CALNAME:Line 3\, Phase 2`,
		"X-WR-RELCALID:prj-1",
		"UID:sprint-4@gopmgr.local",
		// Timed events are written in UTC: 09:00 in Tokyo is 00:00Z.
		"DTSTART:20261024T000000Z",
		"DTEND:20261024T083000Z",
		`SUMMARY:Sprint 4\; review`,
		`DESCRIPTION:Demo\nRetro`,
		"CATEGORIES:SPRINT_END",
		"UID:m1@gopmgr.local",
		// An event with no end is all-day, a date with no time.
		"DTSTART;VALUE=DATE:20261101",
		"CATEGORIES:MILESTONE",
	} {
		if !slices.Contains(lines, want) {
			t.Errorf("calendar is missing %q", want)
		}
	}
	if n := strings.Count(string(raw), "BEGIN:VEVENT"); n != 2 || strings.Count(string(raw), "END:VEVENT") != 2 {
		t.Errorf("calendar holds %d events, want 2", n)
	}
	stamp := regexp.MustCompile(`^DTSTAMP:\d{8}T\d{6}Z$`)
	stamps := 0
	for _, l := range lines {
		if stamp.MatchString(l) {
			stamps++
		}
	}
	if stamps != 2 {
		t.Errorf("%d events carry a UTC DTSTAMP, want 2", stamps)
	}
	if slices.Contains(lines, "DTEND:") || strings.Contains(string(raw), "DTEND:20261101") {
		t.Error("the all-day event has a DTEND")
	}
}

// Properties with no value are left out rather than written empty.
func TestICalRenderLeavesOutEmptyProperties(t *testing.T) {
	raw := string(ICalRender(ICalSpec{Events: []ICalEvent{{UID: "e", Summary: "Plain", Start: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)}}}))
	for _, absent := range []string{"X-WR-CALNAME", "X-WR-RELCALID", "DESCRIPTION", "CATEGORIES"} {
		if strings.Contains(raw, absent) {
			t.Errorf("calendar writes %s with no value", absent)
		}
	}
}

// Text values escape backslashes, semicolons, commas, and line breaks. A
// Windows line break used to leave a bare CR inside the content line.
func TestICalEscapesText(t *testing.T) {
	cases := []struct{ in, want string }{
		{`a,b`, `a\,b`},
		{`a;b`, `a\;b`},
		{`C:\temp`, `C:\\temp`},
		{"one\ntwo", `one\ntwo`},
		{"one\r\ntwo", `one\ntwo`},
		{"one\rtwo", `one\ntwo`},
		{"plain", "plain"},
	}
	for _, c := range cases {
		if got := escapeText(c.in); got != c.want {
			t.Errorf("escapeText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// Long lines fold at 75 octets, a continuation's leading space included,
// and never inside a UTF-8 character. Unfolding restores the value. A
// Japanese title used to be cut mid-character.
func TestICalFoldsLongLinesAtCharacterBoundaries(t *testing.T) {
	for _, summary := range []string{
		strings.Repeat("Commissioning review ", 12),
		strings.Repeat("検収レビュー", 15),
		strings.Repeat("é", 100),
		strings.Repeat("x", 74),
		strings.Repeat("x", 75),
	} {
		raw := ICalRender(ICalSpec{Events: []ICalEvent{{UID: "e", Summary: summary, Start: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)}}})
		physical := icalLines(t, raw)
		for _, l := range physical {
			if len(l) > 75 {
				t.Errorf("line of %d octets, want at most 75: %q", len(l), l)
			}
			if !utf8.ValidString(l) {
				t.Errorf("line %q splits a UTF-8 character", l)
			}
		}
		if !slices.Contains(unfold(physical), "SUMMARY:"+summary) {
			t.Errorf("unfolded calendar does not read back SUMMARY:%s", summary)
		}
	}
}

// Invalid UTF-8 has no character boundaries; folding still ends.
func TestICalFoldsInvalidUTF8(t *testing.T) {
	raw := ICalRender(ICalSpec{Events: []ICalEvent{{UID: "e", Summary: strings.Repeat("\x80", 200), Start: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)}}})
	for _, l := range icalLines(t, raw) {
		if len(l) > 75 {
			t.Errorf("line of %d octets, want at most 75", len(l))
		}
	}
}

// Each holiday in the range becomes an all-day HOLIDAY event, after the
// project's own events.
func TestAppendHolidayEventsAddsAllDayHolidays(t *testing.T) {
	cal := calendar.For("US")
	from, to := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	holidays := cal.HolidaysIn(from, to)
	if len(holidays) == 0 {
		t.Fatal("fixture: the US calendar has no 2026 holidays")
	}

	own := ICalEvent{UID: "m1", Summary: "Go-live", Start: from}
	spec := AppendHolidayEvents(ICalSpec{Events: []ICalEvent{own}}, cal, from, to)
	if len(spec.Events) != 1+len(holidays) || spec.Events[0] != own {
		t.Fatalf("events = %d (first %+v), want the project's event then %d holidays", len(spec.Events), spec.Events[0], len(holidays))
	}
	for i, h := range holidays {
		got := spec.Events[1+i]
		want := ICalEvent{UID: "holiday-US-" + h.Date.Format("20060102"), Summary: h.Name, Start: h.Date, Category: "holiday"}
		if got != want {
			t.Errorf("holiday %d = %+v, want %+v", i, got, want)
		}
	}

	lines := unfold(icalLines(t, ICalRender(spec)))
	first := holidays[0]
	for _, want := range []string{
		"UID:holiday-US-" + first.Date.Format("20060102") + "@gopmgr.local",
		"DTSTART;VALUE=DATE:" + first.Date.Format("20060102"),
		"SUMMARY:" + escapeText(first.Name),
		"CATEGORIES:HOLIDAY",
	} {
		if !slices.Contains(lines, want) {
			t.Errorf("calendar is missing %q", want)
		}
	}
}
