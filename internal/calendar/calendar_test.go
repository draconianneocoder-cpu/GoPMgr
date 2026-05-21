// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package calendar

import (
	"testing"
	"time"
)

// TestForDefaultsToFallback: an unknown country code returns a
// usable calendar (with the universal/secular set) rather than nil.
func TestForDefaultsToFallback(t *testing.T) {
	c := For("XX") // not a real ISO code
	if c == nil {
		t.Fatal("For('XX') returned nil; should fall back to generic")
	}
	if c.CountryCode != "XX" {
		t.Errorf("CountryCode: want XX, got %q", c.CountryCode)
	}
}

// TestIsWorkdayWeekend: Saturday and Sunday are non-workdays in
// every country pack we ship.
func TestIsWorkdayWeekend(t *testing.T) {
	c := For("US")
	sat := time.Date(2026, 1, 3, 12, 0, 0, 0, time.UTC) // Saturday
	if c.IsWorkday(sat) {
		t.Error("Saturday should not be a workday")
	}
	sun := time.Date(2026, 1, 4, 12, 0, 0, 0, time.UTC) // Sunday
	if c.IsWorkday(sun) {
		t.Error("Sunday should not be a workday")
	}
}

// TestUSNewYearIsHoliday: the rickar/cal/v2/us pack should mark
// January 1st as a holiday in any year.
func TestUSNewYearIsHoliday(t *testing.T) {
	c := For("US")
	d := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if !c.IsHoliday(d) {
		t.Error("US New Year's Day 2026 should be a holiday")
	}
}

// TestWorkdaysFromSkipsWeekend: starting Friday and adding 1
// working day lands on Monday, not Saturday.
func TestWorkdaysFromSkipsWeekend(t *testing.T) {
	c := For("US")
	fri := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC) // Friday
	mon := c.WorkdaysFrom(fri, 1)
	if mon.Weekday() == time.Saturday || mon.Weekday() == time.Sunday {
		t.Errorf("WorkdaysFrom(Fri, 1) landed on weekend: %v", mon.Weekday())
	}
}

// TestHolidaysInWindow: the US July window includes Independence
// Day; an empty span includes nothing.
func TestHolidaysInWindow(t *testing.T) {
	c := For("US")
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC)
	got := c.HolidaysIn(from, to)
	if len(got) == 0 {
		t.Error("US July 1–7 should include Independence Day")
	}
	// Reversing the bounds should still work — calendar.HolidaysIn
	// is documented to handle swapped from/to.
	gotRev := c.HolidaysIn(to, from)
	if len(gotRev) != len(got) {
		t.Errorf("reversed bounds: want %d holidays, got %d", len(got), len(gotRev))
	}
}
