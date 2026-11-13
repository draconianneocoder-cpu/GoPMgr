// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"testing"
	"time"
)

// Friday 2026-06-05 is a holiday in this calendar.
func weekdaysWithFridayHoliday(t time.Time) bool {
	return weekdaysOnly(t) && t.Format(DateLayout) != "2026-06-05"
}

func day(s string) time.Time {
	d, err := time.Parse(DateLayout, s)
	if err != nil {
		panic(err)
	}
	return d
}

// A status date counts through the end of that day
// (docs/review/EVM-STATUS-DATE-INCLUSIVE-OWNER-DECISION.md): StatusDay is
// the number of working days on or before it. The project starts Monday
// 2026-06-01; the counts below are by hand.
func TestStatusDayCountsThroughTheEndOfTheDay(t *testing.T) {
	cases := []struct {
		name      string
		date      time.Time
		isWorkday WorkdayFunc
		want      float64
	}{
		{"start day", day("2026-06-01"), weekdaysOnly, 1},
		{"Thursday", day("2026-06-04"), weekdaysOnly, 4},
		{"Friday", day("2026-06-05"), weekdaysOnly, 5},
		// A weekend date adds nothing: DayOffset(Saturday)+1 would wrongly
		// count the following Monday.
		{"Saturday", day("2026-06-06"), weekdaysOnly, 5},
		{"Sunday", day("2026-06-07"), weekdaysOnly, 5},
		{"next Monday", day("2026-06-08"), weekdaysOnly, 6},
		{"day before the start", day("2026-05-31"), weekdaysOnly, 0},
		{"long before the start", day("2026-05-20"), weekdaysOnly, 0},
		{"holiday Friday", day("2026-06-05"), weekdaysWithFridayHoliday, 4},
		{"Saturday after a holiday", day("2026-06-06"), weekdaysWithFridayHoliday, 4},
		{"time of day ignored", time.Date(2026, 6, 5, 23, 0, 0, 0, time.FixedZone("-05:00", -5*3600)), weekdaysOnly, 5},
	}
	for _, c := range cases {
		got, ok := StatusDay(conStart, c.date, c.isWorkday)
		if !ok || got != c.want {
			t.Errorf("%s: StatusDay(%s) = (%v, %v), want (%v, true)", c.name, c.date.Format(time.RFC3339), got, ok, c.want)
		}
	}
}

// With no entered date, the status date is the previous working day.
func TestDefaultStatusDateIsThePreviousWorkingDay(t *testing.T) {
	cases := []struct {
		name      string
		now       time.Time
		isWorkday WorkdayFunc
		want      string
	}{
		{"Tuesday morning", time.Date(2026, 6, 2, 9, 0, 0, 0, time.UTC), weekdaysOnly, "2026-06-01"},
		{"Monday after a weekend", time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC), weekdaysOnly, "2026-06-05"},
		{"Sunday", time.Date(2026, 6, 7, 23, 59, 0, 0, time.UTC), weekdaysOnly, "2026-06-05"},
		{"Monday after a holiday Friday", time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC), weekdaysWithFridayHoliday, "2026-06-04"},
	}
	for _, c := range cases {
		if got := DefaultStatusDate(c.now, c.isWorkday).Format(DateLayout); got != c.want {
			t.Errorf("%s: DefaultStatusDate(%s) = %s, want %s", c.name, c.now.Format(time.RFC3339), got, c.want)
		}
	}
}

// Default-dated figures do not move: the working days through the previous
// working day are exactly the working days before today.
func TestDefaultStatusDayEqualsWorkingDaysBeforeToday(t *testing.T) {
	for _, isWorkday := range []WorkdayFunc{weekdaysOnly, weekdaysWithFridayHoliday} {
		for i := 0; i < 60; i++ {
			now := time.Date(2026, 5, 25, 15, 0, 0, 0, time.UTC).AddDate(0, 0, i)
			got, ok := StatusDay(conStart, DefaultStatusDate(now, isWorkday), isWorkday)
			want, wok := DayOffset(conStart, now, isWorkday)
			if !ok || !wok || got != want {
				t.Fatalf("now %s: StatusDay(default) = %v, want working days before today %v", now.Format(DateLayout), got, want)
			}
		}
	}
}
