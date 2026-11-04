// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package closeguard

import "testing"

func TestReadyCloseRequiresOneFrontendDecision(t *testing.T) {
	var g Gate
	if allow, prompt := g.Request(); !allow || prompt {
		t.Fatalf("bootstrap: %v %v", allow, prompt)
	}
	g.Enable()
	if allow, prompt := g.Request(); allow || !prompt {
		t.Fatalf("dirty close: allow=%v prompt=%v", allow, prompt)
	}
	if allow, prompt := g.Request(); allow || prompt {
		t.Fatalf("duplicate close: allow=%v prompt=%v", allow, prompt)
	}
	g.Cancel()
	if allow, prompt := g.Request(); allow || !prompt {
		t.Fatalf("after cancel: allow=%v prompt=%v", allow, prompt)
	}
}

func TestApprovalCoversWindowAndApplicationHooksWithoutSecondPrompt(t *testing.T) {
	var g Gate
	g.Enable()
	g.Request()
	if err := g.Approve(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if allow, prompt := g.Request(); !allow || prompt {
			t.Fatalf("approved hook %d: allow=%v prompt=%v", i, allow, prompt)
		}
	}
}

func TestApprovalWithoutPendingDecisionIsRefused(t *testing.T) {
	var g Gate
	if err := g.Approve(); err == nil {
		t.Fatal("approval before ready succeeded")
	}
	g.Enable()
	if err := g.Approve(); err == nil {
		t.Fatal("approval without request succeeded")
	}
	g.Request()
	g.Cancel()
	if err := g.Approve(); err == nil {
		t.Fatal("approval after cancel succeeded")
	}
}

func TestFailedSaveKeepsCloseBlockedUntilExplicitRetrySucceeds(t *testing.T) {
	var g Gate
	g.Enable()
	g.Request()
	// Failed save does not call Approve: both hooks must continue to veto.
	if allow, prompt := g.Request(); allow || prompt {
		t.Fatalf("failed save: allow=%v prompt=%v", allow, prompt)
	}
	if err := g.Approve(); err != nil {
		t.Fatal(err)
	}
	if allow, prompt := g.Request(); !allow || prompt {
		t.Fatalf("successful retry: allow=%v prompt=%v", allow, prompt)
	}
}

func TestFailedTerminalDispatchRevokesApproval(t *testing.T) {
	var g Gate
	g.Enable()
	g.Request()
	if err := g.Approve(); err != nil {
		t.Fatal(err)
	}
	// The host calls Cancel when dispatch fails before terminal shutdown.
	g.Cancel()
	if allow, prompt := g.Request(); allow || !prompt {
		t.Fatalf("failed dispatch left authorization active: %v %v", allow, prompt)
	}
}
