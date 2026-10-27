// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"strings"
	"testing"

	"gopmgr/internal/db"
)

// BenchmarkProjectsOverview lists a portfolio of 20 current-schema projects,
// each with 40 charts of about 100 KB and 10 documents of about 50 KB. It uses
// only APIs that exist on both sides of Slice B, so the same file measures
// the old open-and-migrate path and the read-only path.
//
//	go test -run '^$' -bench ProjectsOverview -benchmem .
func BenchmarkProjectsOverview(b *testing.B) {
	app := newEncryptionProjectTestApp(b)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		b.Fatal(err)
	}
	acceptNoRecoveryCodes(b, app)
	dek, err := app.sessionDEK(app.currentSession())
	if err != nil {
		b.Fatal(err)
	}
	chart := `{"blob":"` + strings.Repeat("x", 100_000) + `"}`
	doc := strings.Repeat("y", 50_000)
	for i := 0; i < 20; i++ {
		pf, err := app.CreateProject(fmt.Sprintf("Project %02d", i), "")
		if err != nil {
			b.Fatal(err)
		}
		if err := app.CloseProject(); err != nil {
			b.Fatal(err)
		}
		d, err := db.InitEncryptedDB(pf.Path, dek)
		if err != nil {
			b.Fatal(err)
		}
		p, err := d.GetProject()
		if err != nil {
			b.Fatal(err)
		}
		for c := 0; c < 40; c++ {
			if _, err := d.SaveChart(db.Chart{ProjectID: p.ID, Kind: "gantt", Title: fmt.Sprintf("C%d", c), Data: chart, Config: "{}"}); err != nil {
				b.Fatal(err)
			}
		}
		for k := 0; k < 10; k++ {
			if _, err := d.SaveDocument(db.Document{ProjectID: p.ID, Kind: "charter", Title: fmt.Sprintf("D%d", k), Content: doc}); err != nil {
				b.Fatal(err)
			}
		}
		_ = d.Close()
	}
	b.ReportAllocs()
	for b.Loop() {
		got, err := app.ProjectsOverview()
		if err != nil || len(got) != 20 || got[0].Charts != 40 || got[0].Documents != 10 {
			b.Fatalf("overview = %d projects (first %+v), %v", len(got), got[0], err)
		}
	}
}
