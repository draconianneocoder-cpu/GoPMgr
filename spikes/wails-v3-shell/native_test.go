// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestNativeShellCloseCycle(t *testing.T) {
	binary := os.Getenv("WAILS_SHELL_BIN")
	if binary == "" {
		t.Skip("set WAILS_SHELL_BIN to the separately built native shell")
	}
	scenarios := []string{"window", "quit", "cancel", "retry", "discard", "bootstrap", "startup-failure", "dispatch-failure", "update-quit", "inflight"}
	if runtime.GOOS != "windows" {
		scenarios = append(scenarios, "native-signal-int", "native-signal-term")
	}
	if os.Getenv("WAILS_SHELL_OS_UI") == "1" && runtime.GOOS == "darwin" {
		// Keyboard injection is meaningful only in an unlocked GUI session.
		// Do not inject keys into a locked screen or count a skip as evidence.
		state, err := exec.Command("/usr/sbin/ioreg", "-a", "-n", "Root", "-d", "1").Output()
		if err != nil {
			t.Fatal("OS UI qualification blocked: cannot inspect console session")
		}
		extract := func(key, format string) []byte {
			cmd := exec.Command("/usr/bin/plutil", "-extract", key, format, "-o", "-", "-")
			cmd.Stdin = bytes.NewReader(state)
			value, err := cmd.Output()
			if err != nil {
				t.Fatalf("OS UI qualification blocked: cannot inspect %s", key)
			}
			return value
		}
		if strings.TrimSpace(string(extract("IOConsoleLocked", "raw"))) != "false" {
			t.Fatal("OS UI qualification blocked: unlock the Mac desktop first")
		}
		var sessions []struct {
			OnConsole bool `json:"kCGSSessionOnConsoleKey"`
			LoggedIn  bool `json:"kCGSessionLoginDoneKey"`
			UID       int  `json:"kCGSSessionUserIDKey"`
		}
		if err := json.Unmarshal(extract("IOConsoleUsers", "json"), &sessions); err != nil {
			t.Fatal("OS UI qualification blocked: invalid console session state")
		}
		active := false
		for _, session := range sessions {
			active = active || session.OnConsole && session.LoggedIn && session.UID == os.Getuid()
		}
		if !active {
			t.Fatal("OS UI qualification blocked: current user is not active on the console")
		}

		scenarios = append(scenarios, "native-close-button", "native-cmd-q", "native-menu-quit", "native-open-dialog-cancel", "native-save-dialog-cancel")
	}
	if runtime.GOOS == "linux" && os.Getenv("WAILS_SHELL_X11_TOOL") != "" {
		if os.Getenv("WAILS_SHELL_PRIVATE_X11") != "1" || os.Getenv("DISPLAY") == "" || !filepath.IsAbs(os.Getenv("WAILS_SHELL_X11_TOOL")) {
			t.Fatal("X11 qualification requires an explicit tool and private Xvfb display")
		}
		scenarios = append(scenarios, "native-open-dialog-cancel", "native-save-dialog-cancel", "native-linux-menu-quit")
	}
	for _, scenario := range scenarios {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary)
			root := t.TempDir()
			cmd.Env = append(os.Environ(), "SHELL_SCENARIO="+scenario, "SHELL_RESOURCE="+root+"/resource", "HOME="+root, "XDG_DATA_HOME="+root, "CFFIXED_USER_HOME="+root)
			var output []byte
			var err error
			if strings.HasPrefix(scenario, "native-") {
				logPath := filepath.Join(root, "native.log")
				logFile, e := os.Create(logPath)
				if e != nil {
					t.Fatal(e)
				}
				cmd.Stdout = logFile
				cmd.Stderr = logFile
				if e = cmd.Start(); e != nil {
					_ = logFile.Close()
					t.Fatal(e)
				}
				done := make(chan error, 1)
				go func() { done <- cmd.Wait() }()
				ticker := time.NewTicker(10 * time.Millisecond)
				waiting := true
				for waiting {
					select {
					case e := <-done:
						ticker.Stop()
						_ = logFile.Close()
						body, _ := os.ReadFile(logPath)
						t.Fatalf("process exited before OS close trigger: %v\n%s", e, body)
					case <-ctx.Done():
						ticker.Stop()
						<-done
						_ = logFile.Close()
						t.Fatal("frontend did not become ready for OS trigger")
					case <-ticker.C:
						body, _ := os.ReadFile(logPath)
						waiting = !strings.Contains(string(body), "probe:waiting-os\n")
					}
				}
				ticker.Stop()
				if scenario == "native-signal-int" || scenario == "native-signal-term" {
					sig := os.Interrupt
					if scenario == "native-signal-term" {
						sig = syscall.SIGTERM
					}
					if e := cmd.Process.Signal(sig); e != nil {
						cancel()
						<-done
						_ = logFile.Close()
						t.Fatal(e)
					}
				} else {
					uiOutput, uiErr := triggerNativeUI(ctx, scenario, cmd.Process.Pid)
					if uiErr != nil {
						cancel()
						<-done
						_ = logFile.Close()
						t.Fatalf("OS UI trigger: %v %s", uiErr, uiOutput)
					}
				}
				err = <-done
				_ = logFile.Close()
				output, _ = os.ReadFile(logPath)
			} else {
				output, err = cmd.CombinedOutput()
			}
			if ctx.Err() != nil {
				t.Fatalf("native process did not terminate: %v\n%s", ctx.Err(), output)
			}
			if err != nil {
				t.Fatalf("native process: %v\n%s", err, output)
			}
			var trace []string
			for _, line := range strings.Split(string(output), "\n") {
				if strings.HasPrefix(line, "probe:") {
					trace = append(trace, line)
				}
			}
			want := []string{"probe:resource-open"}
			if scenario == "startup-failure" {
				want = append(want, "probe:service-starting", "probe:startup-failed", "probe:resource-closed")
			} else {
				want = append(want, "probe:ready")
				if scenario == "update-quit" {
					want = append(want, "probe:update-quit")
				}
				if scenario == "inflight" {
					want = append(want, "probe:work-started")
				}
				if strings.HasPrefix(scenario, "native-") {
					want = append(want, "probe:waiting-os")
					if strings.Contains(scenario, "dialog-cancel") {
						want = append(want, "probe:dialog-opened", "probe:dialog-cancelled")
					}
				}
				if scenario != "bootstrap" {
					want = append(want, "probe:prompt")
					if scenario == "cancel" {
						want = append(want, "probe:cancelled", "probe:prompt")
					}
					if scenario == "retry" {
						want = append(want, "probe:save-called", "probe:save-failed", "probe:draft-preserved")
					}
					if scenario == "discard" {
						want = append(want, "probe:discarded")
					} else {
						want = append(want, "probe:save-called", "probe:saved")
					}
					want = append(want, "probe:approved")
					if scenario == "dispatch-failure" {
						want = append(want, "probe:dispatch-failed", "probe:dispatch-recovery", "probe:prompt", "probe:save-called", "probe:saved", "probe:approved")
					}
				}
				if scenario == "inflight" {
					want = append(want, "probe:work-drained")
				}
				want = append(want, "probe:bindings-drained", "probe:shutdown")
				want = append(want, "probe:resource-closed")
				if scenario == "inflight" || scenario == "update-quit" {
					want = append(want, "probe:service-stopped")
				}
				want = append(want, "probe:post-shutdown")
			}
			if !reflect.DeepEqual(trace, want) {
				t.Fatalf("native event order/count\ngot %q\nwant %q\n%s", trace, want, output)
			}
			body, err := os.ReadFile(root + "/resource")
			if err != nil {
				t.Fatal(err)
			}
			wantBody := "unsaved draft"
			if scenario == "discard" || scenario == "bootstrap" || scenario == "startup-failure" {
				wantBody = "saved draft"
			}
			if string(body) != wantBody {
				t.Fatalf("saved data = %q, want %q", body, wantBody)
			}
		})
	}
}

// X11 actions run only with an explicitly opted-in private display. macOS
// actions address only the owned process after console-session preflight.
// GTK portals own their dialog processes, so X11 matches the unique title
// containing the probe PID within the private display, not the dialog PID.
func triggerNativeUI(ctx context.Context, scenario string, pid int) ([]byte, error) {
	if runtime.GOOS == "linux" {
		tool := os.Getenv("WAILS_SHELL_X11_TOOL")
		search := []string{"search", "--sync", "--onlyvisible", "--name", fmt.Sprintf("^GoPMgr dialog %d$", pid)}
		key := "Escape"
		if scenario == "native-linux-menu-quit" {
			search = []string{"search", "--sync", "--onlyvisible", "--all", "--pid", fmt.Sprint(pid), "--name", "^GoPMgr Wails v3 qualification$"}
			key = "ctrl+q"
		}
		window, err := exec.CommandContext(ctx, tool, search...).Output()
		if err != nil {
			return nil, err
		}
		id := strings.TrimSpace(string(window))
		if strings.Contains(id, "\n") || id == "" {
			return nil, fmt.Errorf("expected one owned dialog window")
		}
		if output, err := exec.CommandContext(ctx, tool, "windowfocus", "--sync", id).CombinedOutput(); err != nil {
			return output, err
		}
		return exec.CommandContext(ctx, tool, "key", "--clearmodifiers", key).CombinedOutput()
	}
	action := `keystroke "q" using command down`
	if scenario == "native-close-button" {
		action = `click (first button of window 1 whose subrole is "AXCloseButton")`
	}
	if strings.Contains(scenario, "dialog-cancel") {
		action = `repeat 100 times
if exists sheet 1 of window 1 then exit repeat
delay 0.05
end repeat
click button "Cancel" of splitter group 1 of sheet 1 of window 1`
	}
	if scenario == "native-menu-quit" {
		action = `click menu bar item 2 of menu bar 1
click (first menu item of menu 1 of menu bar item 2 of menu bar 1 whose name starts with "Quit")`
	}
	script := fmt.Sprintf(`tell application "System Events"
tell (first process whose unix id is %d)
set frontmost to true
repeat 100 times
if exists window 1 then exit repeat
delay 0.05
end repeat
%s
end tell
end tell`, pid, action)
	return exec.CommandContext(ctx, "/usr/bin/osascript", "-e", script).CombinedOutput()
}
