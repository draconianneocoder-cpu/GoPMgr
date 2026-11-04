// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"errors"
	"testing"

	"gopmgr/internal/desktop/runtimehost"
)

func TestMessageDialogLeaseSurvivesAsynchronousShow(t *testing.T) {
	r := newWailsRuntime()
	var dismiss func()
	if err := r.withDismissibleUI(func(done func()) { dismiss = done }); err != nil {
		t.Fatal(err)
	}
	if !r.life.UIBusy() || !errors.Is(r.life.ReserveQuit(), runtimehost.ErrUIBusy) {
		t.Fatal("Show return released the active dialog")
	}
	if r.life.Context().Err() != nil {
		t.Fatal("busy quit canceled the usable session")
	}
	dismiss()
	dismiss() // Default/cancel notification must not release another dialog.
	if r.life.UIBusy() {
		t.Fatal("dismissal did not release native UI")
	}
	if err := r.life.ReserveQuit(); err != nil {
		t.Fatal(err)
	}
	if err := r.life.BeginShutdown(nil); err != nil {
		t.Fatal(err)
	}
	if err := r.life.FinishAfterRun(); err != nil {
		t.Fatal(err)
	}
}

func TestMessageDialogLaunchPanicReleasesLease(t *testing.T) {
	r := newWailsRuntime()
	func() {
		defer func() {
			if recover() != "launch failure" {
				t.Error("launch panic was suppressed")
			}
		}()
		_ = r.withDismissibleUI(func(func()) { panic("launch failure") })
	}()
	if r.life.UIBusy() {
		t.Fatal("failed dialog launch leaked native UI")
	}
}
