// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build duckdb

package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	"gopmgr/internal/db"
	"gopmgr/internal/sqlitedriver"
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

// A currency problem found during the loop must not hide the format
// problems: the error names both.
func TestRunPortfolioAnalyticsReportsFormatAndCurrencyTogether(t *testing.T) { // B-03
	f := newReadOnlyFixture(t)
	f.app.ctx = context.Background()
	euro, err := f.app.CreateProject("Euro", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.app.CloseProject(); err != nil {
		t.Fatal(err)
	}
	conn, err := sql.Open(sqlitedriver.Name, keyDSN(t, euro.Path, f.dek))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("UPDATE project SET currency_code = 'EUR'"); err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()

	_, err = f.app.RunPortfolioAnalytics()
	if !errors.Is(err, db.ErrProjectNeedsUpgrade) || !errors.Is(err, db.ErrUnsupportedProjectFormat) {
		t.Fatalf("err = %v, want the format errors", err)
	}
	if err == nil || !strings.Contains(err.Error(), "cannot combine EUR and USD") {
		t.Errorf("err = %v, want the mixed-currency error as well", err)
	}
	if !strings.Contains(err.Error(), f.olderName) {
		t.Errorf("err = %v, does not name %s", err, f.olderName)
	}
}
