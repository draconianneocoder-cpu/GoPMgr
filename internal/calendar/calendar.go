// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package calendar is a thin wrapper around github.com/rickar/cal/v2
// that exposes the holiday lookup PMForge needs (timeline markers,
// working-day skipping in iCal export) without leaking the rickar
// API surface into other packages.
//
// Why a wrapper
//
//   - rickar/cal needs per-country imports (cal/v2/us, cal/v2/gb, ...)
//     that resolve to its various holiday packs. Funneling them
//     through one factory lets PMForge map ISO country codes to
//     rickar packs in one place.
//   - The wrapper API is stdlib-time-only, so callers don't import
//     rickar transitively.
package calendar

import (
	"strings"
	"time"

	"github.com/rickar/cal/v2"
	"github.com/rickar/cal/v2/aa" // generic / business-day defaults
	"github.com/rickar/cal/v2/au"
	"github.com/rickar/cal/v2/ca"
	"github.com/rickar/cal/v2/de"
	"github.com/rickar/cal/v2/fr"
	"github.com/rickar/cal/v2/gb"
	"github.com/rickar/cal/v2/us"
)

// Calendar is the PMForge-facing handle. Construct one per project
// via For(countryCode); reuse it for as many queries as needed —
// rickar/cal is safe for read-only reuse.
type Calendar struct {
	bc          *cal.BusinessCalendar
	CountryCode string
}

// For returns a Calendar populated with the holiday list for the
// given ISO 3166-1 alpha-2 country code. Unknown codes fall back to
// the generic / weekend-only calendar so the rest of PMForge keeps
// working in an unsupported region.
func For(countryCode string) *Calendar {
	bc := cal.NewBusinessCalendar()
	// Default workweek (Mon–Fri 09:00–17:00) is set by cal v2's
	// constructor; no extra config needed.

	code := strings.ToUpper(strings.TrimSpace(countryCode))
	switch code {
	case "US":
		bc.AddHoliday(us.Holidays...)
	case "GB", "UK":
		bc.AddHoliday(gb.Holidays...)
	case "CA":
		bc.AddHoliday(ca.Holidays...)
	case "DE":
		bc.AddHoliday(de.Holidays...)
	case "FR":
		bc.AddHoliday(fr.Holidays...)
	case "AU":
		bc.AddHoliday(au.HolidaysNSW...)
	default:
		bc.AddHoliday(
			aa.NewYear,
			aa.GoodFriday,
			aa.Easter,
			aa.EasterMonday,
			aa.WorkersDay,
			aa.ChristmasDay,
			aa.ChristmasDay2,
		)
	}
	return &Calendar{bc: bc, CountryCode: code}
}

// IsHoliday reports whether t falls on a recognised holiday in the
// calendar's country.
func (c *Calendar) IsHoliday(t time.Time) bool {
	actual, observed, _ := c.bc.IsHoliday(t)
	return actual || observed
}

// IsWorkday reports whether t is a normal working day (workweek and
// not a holiday).
func (c *Calendar) IsWorkday(t time.Time) bool {
	return c.bc.IsWorkday(t)
}

// WorkdaysFrom returns the date that is `days` working days after
// `start`. Useful for due-date math: "this milestone is 10 working
// days after the sprint begins".
//
// A negative `days` walks backward.
func (c *Calendar) WorkdaysFrom(start time.Time, days int) time.Time {
	step := 1
	if days < 0 {
		step = -1
		days = -days
	}
	d := start
	for i := 0; i < days; {
		d = d.AddDate(0, 0, step)
		if c.bc.IsWorkday(d) {
			i++
		}
	}
	return d
}

// HolidaysIn returns every (actual or observed) holiday between from
// and to inclusive, ordered ascending. Used by the Timeline view to
// overlay holiday markers and by the iCal exporter when a project
// option asks for holidays to be included.
type HolidayEvent struct {
	Date time.Time `json:"date"`
	Name string    `json:"name"`
}

// HolidaysIn walks the range one day at a time and asks
// BusinessCalendar.IsHoliday for the underlying Holiday struct so we
// can surface its name. The implementation is O(days) which is fine
// for the project-scale ranges (typically < 730 days).
func (c *Calendar) HolidaysIn(from, to time.Time) []HolidayEvent {
	if to.Before(from) {
		from, to = to, from
	}
	out := []HolidayEvent{}
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		actual, observed, h := c.bc.IsHoliday(d)
		if (actual || observed) && h != nil {
			out = append(out, HolidayEvent{Date: d, Name: h.Name})
		}
	}
	return out
}
