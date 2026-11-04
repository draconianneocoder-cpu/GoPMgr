// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package closeguard

import (
	"errors"
	"sync"
)

// Gate joins window close and application quit into one frontend decision.
// An approval remains valid for the terminal close cycle, so observing it
// from both native hooks does not consume it twice. The host must terminate
// that cycle; this gate is not a reusable authorization for later requests.
type Gate struct {
	mu                       sync.Mutex
	ready, pending, approved bool
}

// Enable is called only after the frontend listener is installed.
func (g *Gate) Enable() { g.mu.Lock(); defer g.mu.Unlock(); g.ready = true }

// Cancel returns to normal use with the draft untouched.
func (g *Gate) Cancel() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.pending = false
	g.approved = false
}

// Approve follows a successful save or an explicit discard decision.
func (g *Gate) Approve() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.ready || !g.pending {
		return errors.New("close: no pending frontend decision")
	}
	g.pending = false
	g.approved = true
	return nil
}

// Request returns whether native termination may proceed, and whether the
// host must notify the frontend. The caller emits outside the gate's lock.
func (g *Gate) Request() (allow, prompt bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.ready || g.approved {
		return true, false
	}
	if g.pending {
		return false, false
	}
	g.pending = true
	return false, true
}
