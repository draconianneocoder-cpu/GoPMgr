<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Earned-value status date: through the end of the day

**Date:** 2026-11-13
**Decider:** GoPMgr maintainer
**Replaces:** the start-of-day status date that portfolio earned value has
used since 2026-07-25 (commit `8b78436`). Report earned value adopted that
convention in PR 213 (KRN-02), and this decision supersedes it.

## Decision

A status date counts **through the end of that day**. Planned value includes
the work scheduled on the status date. Earned value and actual cost already
include the progress and costs recorded "as of" that date, so all three now
cover the same period.

When no status date is entered, the status date is **the end of the previous
working day**: the last day whose progress could have been recorded. The
output shows the date it used.

The same rule applies to portfolio earned value, report exports, the
combined report, the CPM editor's earned-value panel, and the PS-02 audit's
reporting date (SQ-15). The audit takes the inclusive reading only: it has
no default reporting date, so without one SQ-15 is not evaluated.

A milestone is planned once the day it is shown on is complete, so a status
date on that day includes it and the day before does not.

## Why

- GAO's *Schedule Assessment Guide* (GAO-16-89G), "Statusing Progress", treats
  the status date as the line between work performed and work remaining: all
  work before and through it is completed effort.
- Microsoft Project's documentation says its earned-value calculations reflect
  costs up to and including the status date.
- Primavera P6's data date is the first moment new work can begin. Setting it
  to the next morning expresses the same cut-off.
- Counting planned value only to the start of the status date, while earned
  value and actual cost include that day, made SPI and schedule variance read
  better than they were by one day's planned work.

## Consequences

- Earned value for an entered status date rises by one day's planned work,
  compared with PR 213.
- Default-dated figures are unchanged, with one exception: the working days
  through the previous working day are the working days before today, so
  only the shown date changes, from "today" to that date. The exception is
  a budgeted milestone shown today, which was counted a day early before and
  is not counted until today is complete.
- **Portfolio exception:** projects can use different country calendars, so
  no single previous working day fits them all. The portfolio shows the
  previous calendar day (UTC), "through the end of" that date. For every
  project the figures are the same as through its own previous working day.
- **Time zone limit:** "today" is the UTC date, as before. A project's own
  time zone is not used, so near midnight UTC the default can be a day from
  the local date. All outputs use the same rule, so they agree with one
  another.
