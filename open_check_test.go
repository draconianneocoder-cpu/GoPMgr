// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"
)

// countOpenChecks counts how many integrity checks CheckOpenProject runs, by
// the beforeOpenCheckStored seam that each run passes through.
func countOpenChecks(t *testing.T, also func()) *int {
	t.Helper()
	n := 0
	previous := beforeOpenCheckStored
	t.Cleanup(func() { beforeOpenCheckStored = previous })
	beforeOpenCheckStored = func() {
		n++
		if also != nil {
			also()
		}
	}
	return &n
}

func mustCheckOpenProject(t *testing.T, app *App) OpenProjectCheck {
	t.Helper()
	got, err := app.CheckOpenProject()
	if err != nil {
		t.Fatalf("CheckOpenProject: %v", err)
	}
	return got
}

func TestCheckOpenProjectSkipsWhenTheSettingIsOff(t *testing.T) {
	app, _ := openHealableProject(t)
	d := app.requireDB()
	s, err := d.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	s.AutoRepair = false
	if err := d.SaveSettings(s); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	runs := countOpenChecks(t, nil)
	if got := mustCheckOpenProject(t, app); got.Checked || got.Damaged {
		t.Fatalf("CheckOpenProject with the setting off = %+v, want nothing checked", got)
	}
	if *runs != 0 {
		t.Fatalf("an integrity check ran with the setting off")
	}
}

func TestCheckOpenProjectReportsAHealthyProject(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Healthy")
	if got := mustCheckOpenProject(t, app); !got.Checked || got.Damaged {
		t.Fatalf("CheckOpenProject on a healthy project = %+v, want checked and not damaged", got)
	}
}

// The page-3 fixture opens and answers queries, but PRAGMA integrity_check
// reports damage (measured 2026-09-30: both quick_check and integrity_check
// flag it), so the check on open finds it once and remembers it.
func TestCheckOpenProjectFindsDamageOncePerOpen(t *testing.T) {
	app, _ := openHealableProject(t)
	runs := countOpenChecks(t, nil)
	for i := range 2 {
		if got := mustCheckOpenProject(t, app); !got.Checked || !got.Damaged || got.Dismissed {
			t.Fatalf("call %d: CheckOpenProject on a damaged project = %+v, want checked and damaged", i+1, got)
		}
	}
	if *runs != 1 {
		t.Fatalf("integrity check ran %d times in one open, want 1", *runs)
	}

	app.DismissOpenProjectCheck()
	if got := mustCheckOpenProject(t, app); !got.Dismissed || !got.Damaged {
		t.Fatalf("CheckOpenProject after dismissing = %+v, want damaged and dismissed", got)
	}
}

// A project reopened while a check runs has a new database handle. The
// result read from the old handle must not replace the new open's result:
// here the new open is checked and dismissed before the old check finishes,
// and that dismissal must survive.
func TestCheckOpenProjectDiscardsAResultFromAnEarlierOpen(t *testing.T) {
	app, path := openHealableProject(t)
	reopened := false
	runs := countOpenChecks(t, func() {
		if reopened {
			return
		}
		reopened = true
		if err := app.CloseProject(); err != nil {
			t.Fatalf("CloseProject: %v", err)
		}
		if _, err := app.OpenProject(path); err != nil {
			t.Fatalf("OpenProject: %v", err)
		}
		mustCheckOpenProject(t, app)
		app.DismissOpenProjectCheck()
	})
	mustCheckOpenProject(t, app)
	got := mustCheckOpenProject(t, app)
	if *runs != 2 || !got.Dismissed {
		t.Fatalf("after the old check finished: %d checks, result %+v; want 2 checks and the new open's dismissal kept", *runs, got)
	}
}

// Check and repair leaves a healthy project, so the next check on open runs
// again instead of reporting the old damage.
func TestCheckOpenProjectRechecksAfterARepair(t *testing.T) {
	app, _ := openHealableProject(t)
	if got := mustCheckOpenProject(t, app); !got.Damaged {
		t.Fatalf("before repair: %+v, want damaged", got)
	}
	if result, err := app.RepairAndSwap(); err != nil || !result.Swapped {
		t.Fatalf("RepairAndSwap = %+v, %v; want a swap", result, err)
	}
	if got := mustCheckOpenProject(t, app); !got.Checked || got.Damaged {
		t.Fatalf("after repair: %+v, want checked and healthy", got)
	}

	// A repair that finds the project healthy also clears a stored result.
	app.mu.Lock()
	app.openCheck = &openProjectCheck{db: app.db, result: OpenProjectCheck{Checked: true, Damaged: true}}
	app.mu.Unlock()
	if result, err := app.RepairAndSwap(); err != nil || result.Swapped || !result.Success {
		t.Fatalf("RepairAndSwap on the healthy project = %+v, %v", result, err)
	}
	if got := mustCheckOpenProject(t, app); got.Damaged {
		t.Fatalf("after a healthy repair: %+v, want the stale damage cleared", got)
	}
}

func TestCheckOpenProjectNeedsAnOpenProject(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CheckOpenProject(); err == nil {
		t.Fatal("CheckOpenProject with no project open: got nil, want error")
	}
	app.DismissOpenProjectCheck() // must not panic with nothing open
}
