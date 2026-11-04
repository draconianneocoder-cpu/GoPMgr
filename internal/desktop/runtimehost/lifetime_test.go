// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package runtimehost

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"testing/synctest"
)

// A synchronous handler join or early quit must fail this assertion while
// the response writer is waiting for a simulated native/UI turn.
func TestShutdownLeavesUIAvailableUntilResponseDeliveryFinishes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		life := New()
		f, err := os.Create(filepath.Join(t.TempDir(), "resource"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		entered, uiTurn, responseWaiting := make(chan struct{}), make(chan struct{}), make(chan struct{})
		turnGiven := false
		t.Cleanup(func() {
			if !turnGiven {
				close(uiTurn)
			}
		})
		writer := &uiResponseWriter{turn: uiTurn, waiting: responseWaiting, header: make(http.Header)}
		handler := life.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(entered)
			<-life.Context().Done()
			if _, err := f.WriteString("drained"); err != nil {
				t.Error(err)
			}
			_, _ = w.Write([]byte("completed response"))
		}))
		requestDone := make(chan struct{})
		go func() {
			handler.ServeHTTP(writer, httptest.NewRequest("POST", "/wails/runtime", nil))
			close(requestDone)
		}()
		<-entered
		var quits atomic.Int32
		returned := make(chan error, 1)
		go func() { returned <- life.BeginShutdown(func() { quits.Add(1) }) }()
		synctest.Wait()
		select {
		case err := <-returned:
			if err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatal("shutdown blocked its native caller")
		}
		select {
		case <-responseWaiting:
		default:
			t.Fatal("domain work was not canceled before draining")
		}
		if life.Ready() || quits.Load() != 0 {
			t.Fatal("native quit preceded response delivery")
		}
		if _, err := f.Stat(); err != nil {
			t.Fatal("resource closed during active execution", err)
		}
		close(uiTurn)
		turnGiven = true
		<-requestDone
		synctest.Wait()
		if !life.Ready() || quits.Load() != 1 {
			t.Fatal("drained request did not permit exactly one quit")
		}
		if writer.body != "completed response" {
			t.Fatalf("response = %q", writer.body)
		}
		if err := life.FinishAfterRun(); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(f.Name())
		if err != nil || string(body) != "drained" {
			t.Fatalf("persisted result = %q, %v", body, err)
		}
	})
}

func TestShutdownRefusesLateRPCBeforeCallingDomainHandler(t *testing.T) {
	life := New()
	if err := life.BeginShutdown(func() {}); err != nil {
		t.Fatal(err)
	}
	var calls int
	h := life.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls++; w.WriteHeader(http.StatusNoContent) }))
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, httptest.NewRequest("POST", "/wails/runtime", nil))
	if recorder.Code != http.StatusServiceUnavailable || calls != 0 {
		t.Fatalf("late RPC status=%d dispatched=%d", recorder.Code, calls)
	}
	recorder = httptest.NewRecorder()
	h.ServeHTTP(recorder, httptest.NewRequest("GET", "/index.html", nil))
	if recorder.Code != http.StatusNoContent || calls != 1 {
		t.Fatal("non-RPC asset routing changed")
	}
	if err := life.FinishAfterRun(); err != nil {
		t.Fatal(err)
	}
}

func TestBusyNativeUIRefusesQuitWithoutCancelingTheSession(t *testing.T) {
	life := New()
	release, err := life.BeginUI()
	if err != nil {
		t.Fatal(err)
	}
	released := false
	t.Cleanup(func() {
		if !released {
			release()
		}
		_ = life.FinishAfterRun()
	})
	if err := life.BeginShutdown(func() { t.Error("quit during native UI") }); !errors.Is(err, ErrUIBusy) {
		t.Fatalf("busy quit = %v", err)
	}
	if life.Context().Err() != nil {
		t.Fatal("rejected quit canceled a live session")
	}
	release()
	released = true
	if err := life.BeginShutdown(func() {}); err != nil {
		t.Fatal(err)
	}
	if _, err := life.BeginUI(); !errors.Is(err, ErrClosing) {
		t.Fatalf("UI admitted after accepted quit: %v", err)
	}
}

type uiResponseWriter struct {
	header  http.Header
	turn    <-chan struct{}
	waiting chan struct{}
	body    string
}

func (w *uiResponseWriter) Header() http.Header { return w.header }
func (w *uiResponseWriter) WriteHeader(int)     {}
func (w *uiResponseWriter) Write(b []byte) (int, error) {
	close(w.waiting)
	<-w.turn
	w.body += string(b)
	return len(b), nil
}

func TestFinishBeforeDispatchCancelsOwnedContext(t *testing.T) {
	life := New()
	if err := life.FinishAfterRun(); err != nil {
		t.Fatal(err)
	}
	if life.Context().Err() == nil || !life.Ready() {
		t.Fatal("startup failure left its domain lifetime open")
	}
	if err := life.FinishAfterRun(); err != nil {
		t.Fatal("duplicate cleanup", err)
	}
}

func TestFinishAfterLoopExitRefusesActiveWork(t *testing.T) {
	life := New()
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	h := life.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		_, _ = w.Write([]byte("last result"))
	}))
	go func() {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/wails/runtime", nil))
		close(finished)
	}()
	<-entered
	t.Cleanup(func() { close(release); <-finished; _ = life.FinishAfterRun() })
	if err := life.FinishAfterRun(); !errors.Is(err, ErrNotDrained) {
		t.Fatalf("late loop exit with active work = %v", err)
	}
	if life.Context().Err() == nil {
		t.Fatal("late loop exit did not cancel domain work")
	}
	if life.Ready() {
		t.Fatal("active work incorrectly marked drained")
	}
}

func TestRepeatedShutdownStartsOnlyOneTerminalWorker(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		life := New()
		var quits atomic.Int32
		if err := life.BeginShutdown(func() { quits.Add(1) }); err != nil {
			t.Fatal(err)
		}
		if err := life.BeginShutdown(func() { quits.Add(1) }); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if quits.Load() != 1 {
			t.Fatalf("terminal callbacks=%d", quits.Load())
		}
		if err := life.FinishAfterRun(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestRejectedDispatchCanReleaseQuitReservationAndRetry(t *testing.T) {
	life := New()
	if err := life.ReserveQuit(); err != nil {
		t.Fatal(err)
	}
	if _, err := life.BeginUI(); !errors.Is(err, ErrClosing) {
		t.Fatalf("UI during reserved quit = %v", err)
	}
	life.CancelQuitReservation()
	if life.Context().Err() != nil {
		t.Fatal("failed dispatch canceled the live session")
	}
	release, err := life.BeginUI()
	if err != nil {
		t.Fatal("failed dispatch left the native UI unusable", err)
	}
	release()
	if err := life.ReserveQuit(); err != nil {
		t.Fatal(err)
	}
	if err := life.BeginShutdown(func() {}); err != nil {
		t.Fatal(err)
	}
	life.CancelQuitReservation()
	if _, err := life.BeginUI(); !errors.Is(err, ErrClosing) {
		t.Fatalf("accepted shutdown was reopened: %v", err)
	}
	if err := life.FinishAfterRun(); err != nil {
		t.Fatal(err)
	}
}

func TestLoopExitWithMenuUILeaseDoesNotAuthorizeCleanup(t *testing.T) {
	life := New()
	release, err := life.BeginUI()
	if err != nil {
		t.Fatal(err)
	}
	if err := life.FinishAfterRun(); !errors.Is(err, ErrNotDrained) {
		t.Fatalf("loop exit with native UI = %v", err)
	}
	if life.Ready() {
		t.Fatal("live menu panel incorrectly marked drained")
	}
	release()
	if err := life.FinishAfterRun(); err != nil || !life.Ready() {
		t.Fatalf("cleanup after UI release: %v", err)
	}
}
