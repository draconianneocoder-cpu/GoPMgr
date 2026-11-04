// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"gopmgr/internal/db"
	"gopmgr/internal/users"
)

// TestMain runs native child processes on the process's original main thread.
// Normal unit tests do not create a window or touch an account root.
func TestMain(m *testing.M) {
	if os.Getenv("GOPMGR_NATIVE_WIRE_CHILD") == "1" {
		if err := runNativeBridgeFixture(); err != nil {
			fmt.Fprintln(os.Stderr, "native-wire:failed", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runNativeBridgeFixture() error {
	root := os.Getenv("GOPMGR_NATIVE_WIRE_ROOT")
	if _, err := os.Stat(filepath.Join(root, ".native-wire-fixture")); err != nil {
		return fmt.Errorf("isolated fixture marker: %w", err)
	}
	store, err := users.Open(filepath.Join(root, "accounts"))
	if err != nil {
		return err
	}
	app := &App{store: store}
	passed := make(chan string, 1)
	failed := make(chan string, 1)
	options := buildAppOptions(app, os.DirFS(os.Getenv("GOPMGR_NATIVE_WIRE_ASSETS")))
	options.Services = append(options.Services, application.NewService(&nativeWireFixtures{app: app}))
	shutdown := options.OnShutdown
	options.OnShutdown = func() {
		app.mu.RLock()
		live := app.user != nil && app.db != nil && len(app.dek) != 0
		key := app.dek // Retain the original backing array to verify zeroing.
		app.mu.RUnlock()
		if !live {
			fmt.Fprintln(os.Stderr, "native-wire:failed shutdown lacked live session preconditions")
			os.Exit(1)
		}
		shutdown()
		if !bytes.Equal(key, make([]byte, len(key))) {
			fmt.Fprintln(os.Stderr, "native-wire:failed original session key was not zeroed")
			os.Exit(1)
		}
	}
	options.PostShutdown = func() {
		if err := verifyNativeBridgeShutdown(app, store, passed, failed); err != nil {
			fmt.Fprintln(os.Stderr, "native-wire:failed", err)
			os.Exit(1)
		}
	}
	gui := application.New(options)
	windowOptions := mainWindowOptions()
	if mode := os.Getenv("GOPMGR_NATIVE_ABOUT_MODE"); mode != "" {
		windowOptions.URL = "/?about=" + mode
	}
	if mode := os.Getenv("GOPMGR_NATIVE_FILE_MODE"); mode != "" {
		windowOptions.URL = "/?file-select=" + mode
	}
	window := gui.Window.NewWithOptions(windowOptions)
	app.native.app, app.native.window = gui, window
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if app.beforeClose(app.ctx) {
			event.Cancel()
		}
	})
	gui.Event.On("qualification:passed", func(event *application.CustomEvent) {
		path, ok := event.Data.(string)
		if !ok {
			failed <- "unexpected pass event payload"
			return
		}
		passed <- path
	})
	gui.Event.On("qualification:failed", func(event *application.CustomEvent) {
		// A failed fixture must not be trapped behind its enabled close guard
		// or print the success marker. Process exit closes its isolated files.
		fmt.Fprintln(os.Stderr, "native-wire:failed browser assertion:", event.Data)
		os.Exit(1)
	})
	gui.Event.On("qualification:waiting-about", func(*application.CustomEvent) {
		fmt.Println("native-wire:waiting-about")
	})
	err = gui.Run()
	if finishErr := app.native.life.FinishAfterRun(); finishErr != nil {
		return finishErr
	}
	app.shutdown(app.ctx)
	if err != nil {
		return err
	}
	return nil
}

// This fixture service is compiled only in tests. The production service stays
// the unchanged 209-method App. These literals qualify native JSON boundaries
// not represented by a safe standalone production operation.
type nativeWireFixtures struct{ app *App }

func (f *nativeWireFixtures) ShowAbout() {
	f.app.native.showAbout("Native About dismissal qualification")
}
func (f *nativeWireFixtures) NativeUIBusy() bool { return f.app.native.life.UIBusy() }
func (f *nativeWireFixtures) SessionUsable() error {
	f.app.mu.RLock()
	defer f.app.mu.RUnlock()
	if err := f.app.ctx.Err(); err != nil {
		return err
	}
	if f.app.user == nil || f.app.db == nil || len(f.app.dek) == 0 {
		return fmt.Errorf("busy quit changed the session")
	}
	return f.app.db.Conn.Ping()
}

type nativeWireValues struct {
	Zero         int            `json:"zero"`
	SafeInteger  int64          `json:"safe_integer"`
	LargeInteger string         `json:"large_integer"`
	Bytes        []byte         `json:"bytes"`
	Timestamp    time.Time      `json:"timestamp"`
	NilList      []string       `json:"nil_list"`
	EmptyList    []string       `json:"empty_list"`
	NilMap       map[string]int `json:"nil_map"`
	EmptyMap     map[string]int `json:"empty_map"`
}

func (*nativeWireFixtures) Values() nativeWireValues {
	return nativeWireValues{SafeInteger: 9007199254740991, LargeInteger: "9223372036854775807",
		Bytes: []byte{0, 1, 254, 255}, Timestamp: time.Date(2026, 11, 3, 12, 34, 56, 123456789, time.UTC),
		EmptyList: []string{}, EmptyMap: map[string]int{}}
}

func (*nativeWireFixtures) Multiple() (string, int, error) { return "literal", 0, nil }

func verifyNativeBridgeShutdown(app *App, store *users.Store, passed, failed <-chan string) error {
	select {
	case failure := <-failed:
		return fmt.Errorf("browser assertion: %s", failure)
	default:
	}
	select {
	case path := <-passed:
		encrypted, err := db.IsEncryptedFile(path)
		if err != nil || !encrypted {
			return fmt.Errorf("project encryption=%v: %w", encrypted, err)
		}
		if app.user != nil || app.db != nil || len(app.dek) != 0 {
			return fmt.Errorf("session survived shutdown")
		}
		if _, err := store.HasAnyAccount(); err == nil {
			return fmt.Errorf("account store survived shutdown")
		}
		fmt.Println("native-wire:passed generated App calls, encrypted persistence and shutdown")
		return nil
	default:
		return fmt.Errorf("native fixture exited without success")
	}
}

func TestNativeProductionBridge(t *testing.T) {
	assets := os.Getenv("GOPMGR_NATIVE_WIRE_ASSETS")
	if assets == "" {
		t.Skip("set GOPMGR_NATIVE_WIRE_ASSETS to the Vite-built testdata/index.html fixture")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".native-wire-fixture"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable)
	cmd.Env = append(os.Environ(), "GOPMGR_NATIVE_WIRE_CHILD=1", "GOPMGR_NATIVE_WIRE_ROOT="+root,
		"HOME="+root, "CFFIXED_USER_HOME="+root, "XDG_CONFIG_HOME="+root, "XDG_DATA_HOME="+root, "XDG_CACHE_HOME="+root)
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "native-wire:passed generated App calls") {
		t.Fatalf("native production bridge: %v\n%s", err, output)
	}
	t.Log(string(output))
}

func TestNativeAboutDismissalAndQuitRetry(t *testing.T) {
	if os.Getenv("GOPMGR_NATIVE_OS_UI") != "1" || os.Getenv("GOPMGR_NATIVE_WIRE_ASSETS") == "" {
		t.Skip("requires explicit native UI qualification and built bridge fixture")
	}
	if runtime.GOOS == "darwin" {
		state, err := exec.Command("/usr/sbin/ioreg", "-a", "-n", "Root", "-d", "1").Output()
		if err != nil {
			t.Fatal(err)
		}
		check := exec.Command("/usr/bin/plutil", "-extract", "IOConsoleLocked", "raw", "-o", "-", "-")
		check.Stdin = bytes.NewReader(state)
		locked, err := check.Output()
		if err != nil || strings.TrimSpace(string(locked)) != "false" {
			t.Fatal("native UI qualification requires an unlocked desktop")
		}
	} else if runtime.GOOS != "linux" || os.Getenv("GOPMGR_NATIVE_PRIVATE_X11") != "1" || !filepath.IsAbs(os.Getenv("GOPMGR_NATIVE_X11_TOOL")) {
		t.Fatal("native UI qualification requires macOS or an explicit private X11 tool/display")
	}
	modes := []string{"ok", "cancel", "return", "escape"}
	if runtime.GOOS == "linux" {
		modes = append(modes, "close")
	}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, ".native-wire-fixture"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, executable)
			cmd.Env = append(os.Environ(), "GOPMGR_NATIVE_WIRE_CHILD=1", "GOPMGR_NATIVE_WIRE_ROOT="+root, "GOPMGR_NATIVE_ABOUT_MODE="+mode,
				"HOME="+root, "CFFIXED_USER_HOME="+root, "XDG_CONFIG_HOME="+root, "XDG_DATA_HOME="+root, "XDG_CACHE_HOME="+root)
			logPath := filepath.Join(root, "native.log")
			log, err := os.Create(logPath)
			if err != nil {
				t.Fatal(err)
			}
			defer log.Close()
			cmd.Stdout, cmd.Stderr = log, log
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			ticker := time.NewTicker(20 * time.Millisecond)
			defer ticker.Stop()
			for waiting := true; waiting; {
				select {
				case err := <-done:
					body, _ := os.ReadFile(logPath)
					t.Fatalf("exited before dismissal: %v\n%s", err, body)
				case <-ctx.Done():
					<-done
					body, _ := os.ReadFile(logPath)
					t.Fatalf("dialog never became ready: %v\n%s", ctx.Err(), body)
				case <-ticker.C:
					body, _ := os.ReadFile(logPath)
					waiting = !strings.Contains(string(body), "native-wire:waiting-about")
				}
			}
			if output, err := dismissNativeAbout(ctx, mode, cmd.Process.Pid); err != nil {
				cancel()
				<-done
				body, _ := os.ReadFile(logPath)
				t.Fatalf("dismiss About: %v\n%s\n%s", err, output, body)
			} else {
				t.Log(string(output))
			}
			err = <-done
			body, _ := os.ReadFile(logPath)
			if err != nil || !strings.Contains(string(body), "native-wire:passed generated App calls") {
				t.Fatalf("quit retry: %v\n%s", err, body)
			}
			t.Log(string(body))
		})
	}
}

func dismissNativeAbout(ctx context.Context, mode string, pid int) ([]byte, error) {
	if runtime.GOOS == "darwin" {
		action := `click button "OK" of sheet 1 of window 1`
		if mode == "cancel" {
			action = `click button "Cancel" of sheet 1 of window 1`
		}
		if mode == "escape" {
			action = `key code 53`
		}
		if mode == "return" {
			action = `key code 36`
		}
		script := fmt.Sprintf(`tell application "System Events"
tell (first process whose unix id is %d)
set frontmost to true
repeat 100 times
if exists sheet 1 of window 1 then exit repeat
delay 0.05
end repeat
%s
end tell
end tell`, pid, action)
		return exec.CommandContext(ctx, "/usr/bin/osascript", "-e", script).CombinedOutput()
	}
	// GTK4's message dialog sets its heading, not its X11 title. Discover only
	// visible windows of this fixture PID, then require an owned transient parent.
	// No --sync commands: a missing dialog must fail before the browser deadline.
	driverCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	script := `import ctypes as C
import re
import subprocess
import sys
import time

tool, pid, mode = sys.argv[1:]
def run(*args, search=False):
    result = subprocess.run(args, capture_output=True, text=True, timeout=2)
    if result.returncode and not (search and result.returncode == 1):
        raise RuntimeError(f"{args}: {result.returncode}: {result.stdout}{result.stderr}")
    return result.stdout

deadline = time.monotonic() + 3
while True:
    owned = run(tool, "search", "--onlyvisible", "--all", "--pid", pid, "--name", ".*", search=True).split()
    dialogs = []
    diagnostics = []
    for window in owned:
        props = run("xprop", "-id", window, "_NET_WM_PID", "WM_NAME", "WM_TRANSIENT_FOR", "WM_PROTOCOLS")
        diagnostics.append(f"window={window}\n{props}")
        parent = re.search(r"WM_TRANSIENT_FOR.*window id # (0x[0-9a-fA-F]+)", props)
        if parent and str(int(parent[1], 16)) in owned:
            dialogs.append(window)
    if dialogs:
        if len(dialogs) != 1:
            raise RuntimeError(f"expected one owned transient dialog: {diagnostics}")
        break
    if time.monotonic() >= deadline:
        raise RuntimeError(f"no visible owned transient dialog for PID {pid}: {diagnostics}")
    time.sleep(0.05)
window = dialogs[0]
print(f"native-about: pid={pid} mode={mode} selected={window}\n" + "".join(diagnostics), flush=True)
if mode == "close":
    # xdotool 3.20160805.1 windowclose uses XDestroyWindow. Send the ICCCM
    # WM_DELETE_WINDOW client message instead, exercising GTK close-request.
    if "WM_DELETE_WINDOW" not in run("xprop", "-id", window, "WM_PROTOCOLS"):
        raise RuntimeError("dialog does not advertise WM_DELETE_WINDOW")
    x11 = C.CDLL("libX11.so.6")
    x11.XOpenDisplay.argtypes, x11.XOpenDisplay.restype = [C.c_char_p], C.c_void_p
    x11.XInternAtom.argtypes, x11.XInternAtom.restype = [C.c_void_p, C.c_char_p, C.c_int], C.c_ulong
    class Data(C.Union):
        _fields_ = [("b", C.c_char * 20), ("s", C.c_short * 10), ("l", C.c_long * 5)]
    class Message(C.Structure):
        _fields_ = [("type", C.c_int), ("serial", C.c_ulong), ("send_event", C.c_int),
                    ("display", C.c_void_p), ("window", C.c_ulong), ("message_type", C.c_ulong),
                    ("format", C.c_int), ("data", Data)]
    class Event(C.Union):
        _fields_ = [("xclient", Message), ("pad", C.c_long * 24)]
    x11.XSendEvent.argtypes, x11.XSendEvent.restype = [C.c_void_p, C.c_ulong, C.c_int, C.c_long, C.POINTER(Event)], C.c_int
    x11.XSync.argtypes = [C.c_void_p, C.c_int]
    x11.XCloseDisplay.argtypes = [C.c_void_p]
    display = x11.XOpenDisplay(None)
    if not display:
        raise RuntimeError("cannot open private X11 display")
    try:
        event = Event()
        event.xclient.type, event.xclient.display = 33, display
        event.xclient.window = int(window)
        event.xclient.message_type = x11.XInternAtom(display, b"WM_PROTOCOLS", False)
        event.xclient.format = 32
        event.xclient.data.l[0] = x11.XInternAtom(display, b"WM_DELETE_WINDOW", False)
        event.xclient.data.l[1] = 0 # CurrentTime
        if not x11.XSendEvent(display, int(window), False, 0, C.byref(event)):
            raise RuntimeError("WM_DELETE_WINDOW send failed")
        x11.XSync(display, False)
        print("native-about: sent WM_DELETE_WINDOW", flush=True)
    finally:
        x11.XCloseDisplay(display)
elif mode in ("ok", "cancel"):
    geometry = dict(line.split("=", 1) for line in run(tool, "getwindowgeometry", "--shell", window).splitlines())
    # Verified pinned GTK4 fixture at scale 1: 24px margins, centered 54px OK
    # and 80px Cancel buttons, 8px spacing. Fail on a different layout.
    if (geometry["WIDTH"], geometry["HEIGHT"]) != ("300", "214"):
        raise RuntimeError(f"unqualified About button geometry: {geometry}")
    x, y = (106 if mode == "ok" else 181), 173
    run(tool, "mousemove", "--window", window, str(x), str(y), "click", "1")
    print(f"native-about: clicked {mode} at dialog-relative {x},{y}", flush=True)
elif mode in ("return", "escape"):
    run(tool, "windowfocus", window)
    run(tool, "key", "--clearmodifiers", "Return" if mode == "return" else "Escape")
    print(f"native-about: sent {mode}", flush=True)
else:
    raise RuntimeError(f"unknown dismissal mode: {mode}")
`
	return exec.CommandContext(driverCtx, "python3", "-c", script, os.Getenv("GOPMGR_NATIVE_X11_TOOL"), fmt.Sprint(pid), mode).CombinedOutput()
}
