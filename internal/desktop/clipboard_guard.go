// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"crypto/sha256"
	"errors"
	"sync"
	"time"

	"gopmgr/internal/desktop/runtimehost"
)

// Copied recovery codes are cleared from the clipboard 60 seconds after the
// latest Copy, only if the clipboard still holds exactly that text (owner
// decision of 2026-11-14, SEC-04 item 1). A normal quit clears them then.
//
// Threads. Copy runs on a bound-RPC goroutine, the timer on its own, and
// flush on whichever goroutine reaches beginQuit (an RPC, the WindowClosing
// hook, or the main thread through ShouldQuit), always before shutdown
// begins, so the event loop is running. Every clipboard call is made inside
// one onMain closure (application.InvokeSync), which runs on the main thread,
// directly when already there. The mutex is never held across onMain or a
// clipboard call, and no method is called with App.mu held.
//
// Why no read-then-clear can wipe a newer Copy: copy bumps gen before its
// write, and every write runs on the main thread. A clear checks gen and
// pending before reading and again after (GTK's read spins a nested loop, in
// which a newer Copy's write or another clear can run), and its own write
// runs no nested loop. So a newer write either changed gen first, and the
// clear stops, or runs after the clear's write.
//
// No runtimehost BeginUI lease, deliberately: ReserveQuit and BeginShutdown
// refuse to quit while any lease is held, so a clear in progress would cost
// the user's quit. Admission is the closing flag instead: a Copy after flush
// is refused, a Copy that finishes during flush is cleared at once (its RPC
// is counted by the runtime middleware, so shutdown waits for it), and a
// later timer does nothing. A timer whose onMain is queued after the loop has
// stopped stays blocked, holding nothing, until the process exits.
//
// closing is one-way. That holds because every beginQuit follows a
// ReserveQuit, and CancelQuitReservation has no production caller; a future
// cancellable quit must reset it.
//
// Limits (docs/review/sec-04-triage.md, item 1): clearing writes an empty
// text item, since Wails has no true clear; clipboard history and sync may
// keep their own copy; a crash or forced kill leaves the codes.

const clipboardClearDelay = 60 * time.Second

var (
	errClipboardUnavailable = errors.New("the clipboard is not available")
	errClipboardWrite       = errors.New("could not copy to the clipboard")
)

// clipboardText is the system clipboard; *application.ClipboardManager
// implements it.
type clipboardText interface {
	SetText(text string) bool
	Text() (string, bool)
}

type clipboardGuard struct {
	clip   clipboardText
	onMain func(func())

	mu      sync.Mutex
	gen     uint64   // bumped by every Copy, before its write
	pending bool     // the text hashed in sum is on the clipboard, uncleared
	sum     [32]byte // SHA-256 of that text; the codes themselves are not kept
	timer   *time.Timer
	closing bool
}

// copy puts text on the clipboard and arms its clearing. A failed write keeps
// the earlier copy's clearing.
func (g *clipboardGuard) copy(text string) error {
	g.mu.Lock()
	if g.clip == nil || g.onMain == nil {
		g.mu.Unlock()
		return errClipboardUnavailable
	}
	if g.closing {
		g.mu.Unlock()
		return runtimehost.ErrClosing
	}
	g.gen++
	gen := g.gen
	prevPending, prevSum := g.pending, g.sum
	g.pending = false
	g.stopTimerLocked()
	g.mu.Unlock()

	var ok bool
	g.onMain(func() { ok = g.clip.SetText(text) })

	g.mu.Lock()
	if g.gen != gen {
		// A newer Copy owns the clipboard state now.
		g.mu.Unlock()
		if !ok {
			return errClipboardWrite
		}
		return nil
	}
	if ok {
		g.pending, g.sum = true, sha256.Sum256([]byte(text))
	} else {
		g.pending, g.sum = prevPending, prevSum
	}
	switch {
	case !g.pending:
	case g.closing:
		g.mu.Unlock()
		g.clear(gen)
		if !ok {
			return errClipboardWrite
		}
		return nil
	default:
		g.timer = time.AfterFunc(clipboardClearDelay, func() { g.clear(gen) })
	}
	g.mu.Unlock()
	if !ok {
		return errClipboardWrite
	}
	return nil
}

// clear empties the clipboard if copy generation armed is still the latest,
// still uncleared, and the clipboard still holds its text.
func (g *clipboardGuard) clear(armed uint64) {
	g.onMain(func() {
		g.mu.Lock()
		if !g.pending || g.gen != armed {
			g.mu.Unlock()
			return
		}
		want := g.sum
		g.mu.Unlock()

		text, ok := g.clip.Text()
		if !ok || sha256.Sum256([]byte(text)) != want {
			return
		}

		g.mu.Lock()
		if !g.pending || g.gen != armed {
			g.mu.Unlock()
			return
		}
		g.pending = false
		g.mu.Unlock()
		g.clip.SetText("")
	})
}

// flush closes the guard and clears now. beginQuit calls it before shutdown
// begins; it is idempotent.
func (g *clipboardGuard) flush() {
	g.mu.Lock()
	g.closing = true
	g.stopTimerLocked()
	gen, pending := g.gen, g.pending
	ready := g.clip != nil && g.onMain != nil
	g.mu.Unlock()
	if pending && ready {
		g.clear(gen)
	}
}

func (g *clipboardGuard) stopTimerLocked() {
	if g.timer != nil {
		g.timer.Stop()
		g.timer = nil
	}
}
