// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"context"
	"testing"
)

func TestAcceptedCloseRemainsAuthorizedAcrossNativeHooks(t *testing.T) {
	app := &App{ctx: context.Background()}
	app.EnableNativeCloseGuard()
	if err := app.completeNativeClose(func(context.Context) {}); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if app.shouldPreventNativeClose() {
			t.Fatal("accepted terminal cycle prompted again")
		}
	}
	app.EnableNativeCloseGuard()
	if app.shouldPreventNativeClose() {
		t.Fatal("late guard enable revoked accepted shutdown")
	}
}

func TestCloseCompletionDispatchesOnceForAcceptedTerminalCycle(t *testing.T) {
	app := &App{ctx: context.Background()}
	app.EnableNativeCloseGuard()
	calls := 0
	for range 2 {
		if err := app.completeNativeClose(func(context.Context) { calls++ }); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatalf("native dispatches=%d, want one", calls)
	}
}

func TestCloseCompletionRefusesCanceledLifetimeBeforeDispatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	app := &App{ctx: ctx}
	app.EnableNativeCloseGuard()
	calls := 0
	if err := app.completeNativeClose(func(context.Context) { calls++ }); err == nil {
		t.Fatal("close accepted a canceled runtime lifetime")
	}
	if calls != 0 || app.nativeClosePermit {
		t.Fatal("invalid runtime received terminal permission")
	}
}
