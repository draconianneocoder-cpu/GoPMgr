// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// localDateString returns the calendar date in the user's own time zone as
// YYYY-MM-DD. toISOString() would give the UTC date, which is a day off for
// part of every day outside UTC.
export function localDateString(date: Date = new Date()): string {
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${date.getFullYear()}-${month}-${day}`;
}

// levelledStartText is a levelled start as the editors show it: its date,
// and the time of day when levelling started the task part-way through the
// day (level_start_fraction). The time follows the kernel's working clock
// (kernel.StartClock): Microsoft Project's standard day, 08:00-12:00 and
// 13:00-17:00, so half-way through the day is 13:00.
export function levelledStartText(date: string, fraction?: number): string {
  if (!fraction || !(fraction > 0 && fraction < 1)) return date;
  const worked = Math.round(fraction * 480);
  const minutes = worked < 240 ? 8 * 60 + worked : 13 * 60 + worked - 240;
  const hh = String(Math.floor(minutes / 60)).padStart(2, '0');
  const mm = String(minutes % 60).padStart(2, '0');
  return `${date} ${hh}:${mm}`;
}
