// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build duckdb

package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"gopmgr/internal/db"
)

// Portfolio analytics reads every project read-only (Slice B). A project
// with an older schema or a newer format stamp is named in one typed error
// and left unchanged; it is never upgraded to include it.
func TestRunPortfolioAnalyticsNeverWritesProjects(t *testing.T) { // B-03
	f := newReadOnlyFixture(t)
	f.app.ctx = context.Background()
	before := f.digests(t)

	_, err := f.app.RunPortfolioAnalytics()
	if !errors.Is(err, db.ErrProjectNeedsUpgrade) || !errors.Is(err, db.ErrUnsupportedProjectFormat) {
		t.Fatalf("err = %v, want both format errors", err)
	}
	for _, name := range []string{f.olderName, f.futureName} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s", err, name)
		}
	}
	f.requireUnchanged(t, before)

	// With only current and unreadable projects left, the rollup runs over
	// the current one, read-only.
	for _, p := range []string{f.older, f.future} {
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
	}
	got, err := f.app.RunPortfolioAnalytics()
	if err != nil {
		t.Fatalf("current-only rollup: %v", err)
	}
	if got.ProjectCount != 1 {
		t.Errorf("project count = %d, want 1 (the other-key project is skipped, as before)", got.ProjectCount)
	}
	delete(before, f.older)
	delete(before, f.future)
	f.older, f.future = f.current, f.current
	f.requireUnchanged(t, before)
}
