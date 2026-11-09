// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package runtimehost owns the desktop bridge's domain context and admission.
package runtimehost

import (
	"context"
	"errors"
	"net/http"
	"sync"
)

var (
	ErrClosing    = errors.New("application is closing")
	ErrUIBusy     = errors.New("finish or cancel the native dialog before closing")
	ErrNotDrained = errors.New("native loop returned with active desktop work")
)

// Lifetime drains handlers before native shutdown, while response delivery can
// still use the native event loop. It never waits for handlers on that loop.
// All native UI operations must hold a BeginUI lease until they return.
type Lifetime struct {
	mu                         sync.Mutex
	active                     sync.WaitGroup
	activeCount                int
	ui                         int
	closing, drained, reserved bool
	workerDone                 chan struct{}
	ctx                        context.Context
	cancel                     context.CancelFunc
}

func New() *Lifetime {
	ctx, cancel := context.WithCancel(context.Background())
	return &Lifetime{ctx: ctx, cancel: cancel}
}

func (l *Lifetime) Context() context.Context { return l.ctx }

// Middleware leaves asset serving unchanged and counts complete runtime HTTP
// handlers, including response delivery. It supplies no header authentication;
// native asset/origin and domain authorization remain separate boundaries.
func (l *Lifetime) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wails/runtime" {
			next.ServeHTTP(w, r)
			return
		}
		l.mu.Lock()
		if l.closing {
			l.mu.Unlock()
			http.Error(w, ErrClosing.Error(), http.StatusServiceUnavailable)
			return
		}
		l.active.Add(1)
		l.activeCount++
		l.mu.Unlock()
		defer func() {
			l.mu.Lock()
			l.activeCount--
			l.mu.Unlock()
			l.active.Done()
		}()
		next.ServeHTTP(w, r)
	})
}

// BeginUI prevents accepting quit while a native panel needs the event loop.
// A rejected quit does not cancel the session; the user can finish the panel.
func (l *Lifetime) BeginUI() (func(), error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closing || l.reserved {
		return nil, ErrClosing
	}
	l.ui++
	return func() { l.mu.Lock(); l.ui--; l.mu.Unlock() }, nil
}

// BeginShutdown is nonblocking and idempotent for one terminal cycle. It
// closes admission before canceling domain work. Its single owned goroutine
// joins handlers, marks quiescence, then calls after. That callback may queue
// native Quit, but must not wait for the native loop. FinishAfterRun joins the
// worker outside the loop. A busy UI refusal changes no lifecycle state.
func (l *Lifetime) BeginShutdown(after func()) error {
	l.mu.Lock()
	if l.closing {
		l.mu.Unlock()
		return nil
	}
	if l.ui != 0 {
		l.mu.Unlock()
		return ErrUIBusy
	}
	l.closing = true
	done := make(chan struct{})
	l.workerDone = done
	l.mu.Unlock()
	l.cancel()
	go func() {
		defer close(done)
		l.active.Wait()
		l.mu.Lock()
		l.drained = true
		l.mu.Unlock()
		if after != nil {
			after()
		}
	}()
	return nil
}

func (l *Lifetime) Ready() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.drained
}

// FinishAfterRun handles both normal completion and failure before dispatch.
// A late platform-loop exit is not assumed to be an empty startup failure:
// callers must not close stores when this reports outstanding execution/UI.
func (l *Lifetime) FinishAfterRun() error {
	l.mu.Lock()
	l.closing = true
	if l.activeCount != 0 || l.ui != 0 {
		l.mu.Unlock()
		l.cancel()
		return ErrNotDrained
	}
	done := l.workerDone
	if done == nil {
		l.drained = true
	}
	l.mu.Unlock()
	l.cancel()
	if done != nil {
		<-done
	}
	return nil
}

// ReserveQuit prevents a new native UI operation between approval and dispatch.
// Cancellation before dispatch can release it without canceling domain work.
func (l *Lifetime) ReserveQuit() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.ui != 0 {
		return ErrUIBusy
	}
	l.reserved = true
	return nil
}

func (l *Lifetime) CancelQuitReservation() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.closing {
		l.reserved = false
	}
}

func (l *Lifetime) UIBusy() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.ui != 0
}
