// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"bytes"
	"errors"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"gopmgr/internal/desktop/runtimehost"
)

// testMainLoop stands in for Wails' main thread: it runs one closure at a
// time, and a closure that asks for the main thread from the loop itself runs
// directly (application.dispatchOnMainThread). pumpOne runs queued work
// inline, as GTK's nested loop does during a clipboard read.
type testMainLoop struct {
	work chan func()
	id   uint64
}

func goroutineID() uint64 {
	buf := make([]byte, 64)
	buf = buf[:runtime.Stack(buf, false)]
	buf = bytes.TrimPrefix(buf, []byte("goroutine "))
	id, _ := strconv.ParseUint(string(buf[:bytes.IndexByte(buf, ' ')]), 10, 64)
	return id
}

func startTestMainLoop() (*testMainLoop, func()) {
	l := &testMainLoop{work: make(chan func())}
	started := make(chan struct{})
	go func() {
		l.id = goroutineID()
		close(started)
		for fn := range l.work {
			fn()
		}
	}()
	<-started
	return l, func() { close(l.work) }
}

func (l *testMainLoop) run(fn func()) {
	if goroutineID() == l.id {
		fn()
		return
	}
	done := make(chan struct{})
	l.work <- func() { fn(); close(done) }
	<-done
}

// pumpOne waits for the next queued closure and runs it inline.
func (l *testMainLoop) pumpOne() {
	(<-l.work)()
}

// fakeClipboard is the system clipboard, the external boundary.
type fakeClipboard struct {
	mu       sync.Mutex
	text     string
	failSet  bool
	failRead bool
	onRead   func() // runs inside Text, once
}

func (c *fakeClipboard) SetText(text string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failSet {
		return false
	}
	c.text = text
	return true
}

func (c *fakeClipboard) Text() (string, bool) {
	c.mu.Lock()
	text, fail, hook := c.text, c.failRead, c.onRead
	c.onRead = nil
	c.mu.Unlock()
	if hook != nil {
		hook()
	}
	return text, !fail
}

func (c *fakeClipboard) get() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.text
}

func (c *fakeClipboard) configure(change func(c *fakeClipboard)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	change(c)
}

func (c *fakeClipboard) set(text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.text = text
}

const codesA, codesB = "AAAAAAAA-BBBBBBBB\nCCCCCCCC-DDDDDDDD", "EEEEEEEE-FFFFFFFF"

// guardTest runs body in a synctest bubble with a guard, its clipboard and
// a main loop.
func guardTest(t *testing.T, body func(t *testing.T, g *clipboardGuard, clip *fakeClipboard, loop *testMainLoop)) {
	t.Helper()
	synctest.Test(t, func(t *testing.T) {
		loop, stop := startTestMainLoop()
		defer stop()
		clip := &fakeClipboard{}
		g := &clipboardGuard{clip: clip, onMain: loop.run}
		body(t, g, clip, loop)
		g.mu.Lock()
		g.stopTimerLocked()
		g.mu.Unlock()
	})
}

func mustCopy(t *testing.T, g *clipboardGuard, text string) {
	t.Helper()
	if err := g.copy(text); err != nil {
		t.Fatalf("copy: %v", err)
	}
}

func TestClipboardClearsCopiedCodesAfterSixtySeconds(t *testing.T) {
	cases := []struct {
		name  string
		after func(clip *fakeClipboard) // during the 60 s
		want  string
	}{
		{"still holding the codes", func(*fakeClipboard) {}, ""},
		{"the user copied something else", func(c *fakeClipboard) { c.set("a shopping list") }, "a shopping list"},
		{"the clipboard cannot be read", func(c *fakeClipboard) { c.configure(func(c *fakeClipboard) { c.failRead = true }) }, codesA},
		{"another app changed one character", func(c *fakeClipboard) { c.set(codesA + "\n") }, codesA + "\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			guardTest(t, func(t *testing.T, g *clipboardGuard, clip *fakeClipboard, _ *testMainLoop) {
				mustCopy(t, g, codesA)
				if clip.get() != codesA {
					t.Fatalf("clipboard = %q after copy", clip.get())
				}
				tc.after(clip)
				time.Sleep(clipboardClearDelay - time.Second)
				synctest.Wait()
				if clip.get() == "" {
					t.Fatal("cleared before 60 seconds")
				}
				time.Sleep(time.Second)
				synctest.Wait()
				if got := clip.get(); got != tc.want {
					t.Fatalf("clipboard after 60 s = %q, want %q", got, tc.want)
				}
			})
		})
	}
}

func TestClipboardClearsOnlyTheLatestCopy(t *testing.T) {
	guardTest(t, func(t *testing.T, g *clipboardGuard, clip *fakeClipboard, _ *testMainLoop) {
		mustCopy(t, g, codesA)
		time.Sleep(30 * time.Second)
		mustCopy(t, g, codesB)
		time.Sleep(30 * time.Second) // A's 60 s
		synctest.Wait()
		if clip.get() != codesB {
			t.Fatalf("clipboard = %q at A's deadline, want B kept", clip.get())
		}
		time.Sleep(30 * time.Second) // B's 60 s
		synctest.Wait()
		if clip.get() != "" {
			t.Fatalf("clipboard = %q at B's deadline, want cleared", clip.get())
		}
	})
}

// A failed Copy keeps the earlier Copy's clearing, so codes already on the
// clipboard are still cleared.
func TestClipboardFailedCopyKeepsTheEarlierClearing(t *testing.T) {
	guardTest(t, func(t *testing.T, g *clipboardGuard, clip *fakeClipboard, _ *testMainLoop) {
		mustCopy(t, g, codesA)
		clip.configure(func(c *fakeClipboard) { c.failSet = true })
		if err := g.copy(codesB); !errors.Is(err, errClipboardWrite) {
			t.Fatalf("failed copy = %v, want errClipboardWrite", err)
		}
		clip.configure(func(c *fakeClipboard) { c.failSet = false })
		time.Sleep(clipboardClearDelay)
		synctest.Wait()
		if clip.get() != "" {
			t.Fatalf("clipboard = %q, want A cleared after the failed copy", clip.get())
		}
	})
}

// GTK's read spins a nested loop. A Copy whose write runs inside it must not
// be wiped by the clear that was reading.
func TestClipboardClearSparesACopyMadeDuringItsRead(t *testing.T) {
	guardTest(t, func(t *testing.T, g *clipboardGuard, clip *fakeClipboard, loop *testMainLoop) {
		mustCopy(t, g, codesA)
		copied := make(chan error, 1)
		clip.configure(func(c *fakeClipboard) {
			c.onRead = func() {
				go func() { copied <- g.copy(codesB) }()
				loop.pumpOne() // the nested loop runs B's write
				if err := <-copied; err != nil {
					t.Error(err)
				}
			}
		})
		time.Sleep(clipboardClearDelay)
		synctest.Wait()
		if clip.get() != codesB {
			t.Fatalf("clipboard = %q, want B kept", clip.get())
		}
	})
}

// A second clear running inside the first one's read (a quit while the timer
// reads) clears once; the first must not clear again over a newer copy.
func TestClipboardClearRunsOnceWhenTwoOverlap(t *testing.T) {
	guardTest(t, func(t *testing.T, g *clipboardGuard, clip *fakeClipboard, _ *testMainLoop) {
		mustCopy(t, g, codesA)
		clip.configure(func(c *fakeClipboard) {
			c.onRead = func() {
				g.flush() // on the main loop, so its clear runs inline
				clip.set("copied in another app")
			}
		})
		time.Sleep(clipboardClearDelay)
		synctest.Wait()
		if got := clip.get(); got != "copied in another app" {
			t.Fatalf("clipboard = %q, want the other app's copy kept", got)
		}
	})
}

func TestClipboardQuitClearsNowAndClosesTheGuard(t *testing.T) {
	guardTest(t, func(t *testing.T, g *clipboardGuard, clip *fakeClipboard, _ *testMainLoop) {
		mustCopy(t, g, codesA)
		g.flush()
		if clip.get() != "" {
			t.Fatalf("clipboard = %q after quit, want cleared", clip.get())
		}
		if err := g.copy(codesB); !errors.Is(err, runtimehost.ErrClosing) {
			t.Fatalf("copy after quit = %v, want ErrClosing", err)
		}
		clip.set("copied after quit")
		time.Sleep(2 * clipboardClearDelay)
		synctest.Wait()
		if got := clip.get(); got != "copied after quit" {
			t.Fatalf("a timer acted after quit: clipboard = %q", got)
		}
	})
}

// A Copy whose write is in progress when quit begins is cleared at once.
func TestClipboardQuitDuringACopyClearsIt(t *testing.T) {
	guardTest(t, func(t *testing.T, g *clipboardGuard, clip *fakeClipboard, loop *testMainLoop) {
		g.onMain = func(fn func()) {
			loop.run(func() {
				fn()
				if clip.get() == codesA {
					g.flush() // quit lands just after the write
				}
			})
		}
		mustCopy(t, g, codesA)
		if clip.get() != "" {
			t.Fatalf("clipboard = %q, want the copy cleared at quit", clip.get())
		}
	})
}

// Through the close hook's bootstrap path, a quit clears copied codes.
func TestNativeQuitClearsCopiedRecoveryCodes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		loop, stop := startTestMainLoop()
		defer stop()
		app := &App{native: newWailsRuntime()}
		clip := &fakeClipboard{}
		app.native.clipboard.clip, app.native.clipboard.onMain = clip, loop.run
		mustCopy(t, &app.native.clipboard, codesA)
		if !app.beforeClose(t.Context()) {
			t.Fatal("bootstrap close was not deferred to the quit path")
		}
		if clip.get() != "" {
			t.Fatalf("clipboard = %q after quit, want cleared", clip.get())
		}
		synctest.Wait()
	})
}

func TestCopyRecoveryCodesWiring(t *testing.T) {
	app := newAdminTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "passphrase-long", true); err != nil {
		t.Fatal(err)
	}
	if err := app.CopyRecoveryCodes([]string{"AAAAAAAA-BBBBBBBB"}); !errors.Is(err, errClipboardUnavailable) {
		t.Fatalf("copy without a native runtime = %v, want errClipboardUnavailable", err)
	}
	app.native = newWailsRuntime()
	clip := &fakeClipboard{}
	app.native.clipboard.clip, app.native.clipboard.onMain = clip, func(fn func()) { fn() }
	if err := app.CopyRecoveryCodes([]string{"not a code"}); err == nil || clip.get() != "" {
		t.Fatalf("malformed code: err %v, clipboard %q; want refused and nothing written", err, clip.get())
	}
	if err := app.CopyRecoveryCodes([]string{"AAAAAAAA-BBBBBBBB", "CCCCCCCC-DDDDDDDD"}); err != nil {
		t.Fatal(err)
	}
	if got := clip.get(); got != codesA {
		t.Fatalf("clipboard = %q, want the codes one per line", got)
	}
	app.native.clipboard.flush()
}

// A timer that fired just before a new Copy (so stopping it came too late)
// must not clear the new codes when its clear reaches the main thread.
func TestClipboardLateTimerSparesTheNextCopy(t *testing.T) {
	guardTest(t, func(t *testing.T, g *clipboardGuard, clip *fakeClipboard, loop *testMainLoop) {
		var (
			holdMu sync.Mutex
			hold   chan struct{}
		)
		g.onMain = func(fn func()) {
			holdMu.Lock()
			h := hold
			hold = nil
			holdMu.Unlock()
			if h != nil {
				<-h // the fired timer waits here for the main thread
			}
			loop.run(fn)
		}
		mustCopy(t, g, codesA)
		held := make(chan struct{})
		holdMu.Lock()
		hold = held
		holdMu.Unlock()
		time.Sleep(clipboardClearDelay) // A's timer fires and is held
		synctest.Wait()
		mustCopy(t, g, codesB)
		close(held)
		synctest.Wait()
		if clip.get() != codesB {
			t.Fatalf("clipboard = %q, want B kept from A's late timer", clip.get())
		}
	})
}
