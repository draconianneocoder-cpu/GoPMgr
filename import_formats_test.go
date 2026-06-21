// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"strings"
	"testing"
)

// TestImportScheduleFileRejectsBinaryFormats verifies the binary/serialized
// project formats (.mpp, .pod) and the legacy .mpx text format return a
// clear, actionable message pointing at the MS Project XML interchange path,
// rather than failing opaquely. These branches run before any project DB is
// needed, so a bare App is sufficient.
func TestImportScheduleFileRejectsBinaryFormats(t *testing.T) {
	app := &App{}
	cases := map[string]string{
		"/tmp/schedule.mpp": "Microsoft Project XML",
		"/tmp/schedule.MPP": "Microsoft Project XML",
		"/tmp/legacy.mpx":   "Microsoft Project XML",
		"/tmp/plan.pod":     "Microsoft Project XML",
	}
	for path, want := range cases {
		_, err := app.importScheduleFile(path)
		if err == nil {
			t.Errorf("%s: expected an error, got nil", path)
			continue
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error %q should mention %q", path, err.Error(), want)
		}
	}
}
