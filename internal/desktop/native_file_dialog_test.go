// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const nativeFileFolder = "owned folder '資料'"
const nativeFileInput = "資料 input 'quoted'.txt"
const nativeFileOutput = "報告 output 'quoted'.txt"
const nativeFileContents = "Native selection must leave these input bytes untouched.\n"

// Each native fixture is a separate process. Count entry to the selection
// service reached by the real browser runtime RPC, not direct Go invocation.
var nativeFileSelectionCalls atomic.Int32

type nativeFileSelectionResult struct {
	Mode  string `json:"mode"`
	Root  string `json:"root"`
	Path  string `json:"path"`
	Calls int32  `json:"calls"`
}

func (*nativeWireFixtures) FileSelectionRoot() (string, error) {
	root := os.Getenv("GOPMGR_NATIVE_WIRE_ROOT")
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("file selection requires an absolute owned root")
	}
	if _, err := os.Stat(filepath.Join(root, ".native-wire-fixture")); err != nil {
		return "", err
	}
	return root, nil
}

func (f *nativeWireFixtures) SelectFile(mode string) (string, error) {
	root, err := f.FileSelectionRoot()
	if err != nil {
		return "", err
	}
	if (mode != "open" && mode != "save") || mode != os.Getenv("GOPMGR_NATIVE_FILE_MODE") {
		return "", fmt.Errorf("unexpected file selection mode %q", mode)
	}
	if calls := nativeFileSelectionCalls.Add(1); calls != 1 {
		return "", fmt.Errorf("selection RPC dispatched %d times", calls)
	}
	folder := filepath.Join(root, nativeFileFolder)
	fmt.Printf("native-file:opening %s\n", mode)
	var selected string
	if mode == "open" {
		selected, err = f.app.native.openFileDialog(f.app.ctx, application.OpenFileDialogOptions{
			Title: "GoPMgr native Open qualification", Directory: folder,
			AllowsMultipleSelection: false, CanChooseDirectories: false,
		})
	} else {
		selected, err = f.app.native.saveFileDialog(f.app.ctx, application.SaveFileDialogOptions{
			Title: "GoPMgr native Save qualification", Directory: folder,
			Filename: nativeFileOutput, AllowOtherFileTypes: true,
		})
	}
	if err != nil {
		return "", err
	}
	expected := nativeFileInput
	if mode == "save" {
		expected = nativeFileOutput
	}
	if selected != filepath.Join(folder, expected) {
		return "", fmt.Errorf("native callback selected %q, want %q", selected, filepath.Join(folder, expected))
	}
	if err := verifyNativeFileContents(root); err != nil {
		return "", err
	}
	fmt.Printf("native-file:callback mode=%s path=%q calls=%d\n", mode, selected, nativeFileSelectionCalls.Load())
	return selected, nil
}

func (f *nativeWireFixtures) FileSelectionReady() error {
	if !f.app.native.life.UIBusy() || nativeFileSelectionCalls.Load() != 1 {
		return fmt.Errorf("file selection is not pending")
	}
	if err := f.SessionUsable(); err != nil {
		return err
	}
	fmt.Println("native-file:waiting-selection")
	return nil
}

func (f *nativeWireFixtures) ConfirmFileSelection(selected string) (int32, error) {
	root, err := f.FileSelectionRoot()
	if err != nil {
		return 0, err
	}
	mode := os.Getenv("GOPMGR_NATIVE_FILE_MODE")
	name := nativeFileInput
	if mode == "save" {
		name = nativeFileOutput
	} else if mode != "open" {
		return 0, fmt.Errorf("unexpected mode")
	}
	if selected != filepath.Join(root, nativeFileFolder, name) || nativeFileSelectionCalls.Load() != 1 || f.app.native.life.UIBusy() {
		return 0, fmt.Errorf("file selection confirmation failed: %q", selected)
	}
	if err := verifyNativeFileContents(root); err != nil {
		return 0, err
	}
	result := nativeFileSelectionResult{mode, root, selected, nativeFileSelectionCalls.Load()}
	body, err := json.Marshal(result)
	if err != nil {
		return 0, err
	}
	if err := os.WriteFile(filepath.Join(root, "native-file-result.json"), body, 0600); err != nil {
		return 0, err
	}
	fmt.Println("native-file:confirmed browser path and untouched filesystem")
	return result.Calls, nil
}

func verifyNativeFileContents(root string) error {
	body, err := os.ReadFile(filepath.Join(root, nativeFileFolder, nativeFileInput))
	if err != nil {
		return err
	}
	if string(body) != nativeFileContents {
		return fmt.Errorf("input file changed")
	}
	if _, err := os.Lstat(filepath.Join(root, nativeFileFolder, nativeFileOutput)); !os.IsNotExist(err) {
		return fmt.Errorf("Save selection created an output or could not verify absence: %v", err)
	}
	return nil
}

func TestNativeFileSelectionAndQuitRetry(t *testing.T) {
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
		t.Fatal("requires macOS or an explicit private X11 tool/display")
	}
	for _, mode := range []string{"open", "save"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, ".native-wire-fixture"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			folder := filepath.Join(root, nativeFileFolder)
			if err := os.Mkdir(folder, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(folder, nativeFileInput), []byte(nativeFileContents), 0600); err != nil {
				t.Fatal(err)
			}
			if err := verifyNativeFileContents(root); err != nil {
				t.Fatal(err)
			}
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, executable)
			cmd.Env = append(os.Environ(), "GOPMGR_NATIVE_WIRE_CHILD=1", "GOPMGR_NATIVE_WIRE_ROOT="+root,
				"GOPMGR_NATIVE_FILE_MODE="+mode, "GOPMGR_NATIVE_ABOUT_MODE=", "HOME="+root,
				"CFFIXED_USER_HOME="+root, "XDG_CONFIG_HOME="+root, "XDG_DATA_HOME="+root, "XDG_CACHE_HOME="+root)
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
					t.Fatalf("exited before native selection: %v\n%s", err, body)
				case <-ctx.Done():
					<-done
					body, _ := os.ReadFile(logPath)
					t.Fatalf("file dialog never became ready: %v\n%s", ctx.Err(), body)
				case <-ticker.C:
					body, _ := os.ReadFile(logPath)
					waiting = !strings.Contains(string(body), "native-file:waiting-selection")
				}
			}
			output, err := acceptNativeFile(ctx, mode, cmd.Process.Pid)
			t.Log(string(output))
			if err != nil {
				cancel()
				<-done
				body, _ := os.ReadFile(logPath)
				t.Fatalf("accept native %s: %v\n%s", mode, err, body)
			}
			err = <-done
			body, _ := os.ReadFile(logPath)
			if err != nil || !strings.Contains(string(body), "native-wire:passed generated App calls") || !strings.Contains(string(body), "native-file:confirmed browser path") {
				t.Fatalf("native selection and quit retry: %v\n%s", err, body)
			}
			var result nativeFileSelectionResult
			record, err := os.ReadFile(filepath.Join(root, "native-file-result.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(record, &result); err != nil {
				t.Fatal(err)
			}
			name := "資料 input 'quoted'.txt"
			if mode == "save" {
				name = "報告 output 'quoted'.txt"
			}
			want := filepath.Join(root, "owned folder '資料'", name)
			if result.Mode != mode || result.Root != root || result.Path != want || result.Calls != 1 {
				t.Fatalf("independent parent path/RPC check: %+v, want %q", result, want)
			}
			// The parent independently checks the filesystem after native shutdown.
			input, err := os.ReadFile(filepath.Join(folder, "資料 input 'quoted'.txt"))
			if err != nil || string(input) != "Native selection must leave these input bytes untouched.\n" {
				t.Fatalf("input changed: %q, %v", input, err)
			}
			if _, err := os.Lstat(filepath.Join(folder, "報告 output 'quoted'.txt")); !os.IsNotExist(err) {
				t.Fatalf("Save prompt created output or absence not verifiable: %v", err)
			}
			t.Log(string(body))
		})
	}
}

func acceptNativeFile(ctx context.Context, mode string, pid int) ([]byte, error) {
	driverCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	if runtime.GOOS == "linux" {
		// These coordinates were inspected in the pinned GTK4 private-Xvfb
		// fixture. Different layouts fail explicitly rather than clicking blindly.
		script := `import re
import subprocess
import sys
import time

tool, pid, mode = sys.argv[1:]
if mode not in ("open", "save"):
    raise RuntimeError(f"unknown file mode: {mode}")
def run(*args, search=False):
    result = subprocess.run(args, capture_output=True, text=True, timeout=2)
    if result.returncode and not (search and result.returncode == 1 and not result.stderr):
        raise RuntimeError(f"{args}: {result.returncode}: {result.stdout}{result.stderr}")
    return result.stdout

title = "GoPMgr native " + mode.title() + " qualification"
deadline = time.monotonic() + 4
while True:
    owned = run(tool, "search", "--onlyvisible", "--all", "--pid", pid, "--name", ".*", search=True).split()
    dialogs = [w for w in owned if run(tool, "getwindowname", w).strip() == title]
    if dialogs:
        if len(dialogs) != 1:
            raise RuntimeError(f"expected one owned {title}: {dialogs}")
        break
    if time.monotonic() >= deadline:
        raise RuntimeError(f"no visible owned {title} for PID {pid}: {owned}")
    time.sleep(0.05)
window = dialogs[0]
def inspect():
    props = run("xprop", "-id", window, "_NET_WM_PID", "WM_NAME", "WM_TRANSIENT_FOR")
    parent = re.search(r"WM_TRANSIENT_FOR.*window id # (0x[0-9a-fA-F]+)", props)
    owner = re.search(r"_NET_WM_PID\(CARDINAL\) = (\d+)", props)
    if not owner or owner[1] != pid or not parent or str(int(parent[1], 16)) not in owned:
        raise RuntimeError(f"dialog ownership changed: {props}")
    if run(tool, "getwindowname", window).strip() != title:
        raise RuntimeError("dialog title changed")
    geometry = dict(line.split("=", 1) for line in run(tool, "getwindowgeometry", "--shell", window).splitlines())
    expected = ("821", "326" if mode == "open" else "360")
    if (geometry["WIDTH"], geometry["HEIGHT"]) != expected:
        raise RuntimeError(f"unqualified file-dialog geometry: {geometry}")
    print(f"native-file X11 before action: mode={mode} geometry={expected}\n{props}", flush=True)
inspect()
run(tool, "windowfocus", window)
if mode == "open":
    # The owned directory contains exactly the inspected literal input row.
    inspect()
    run(tool, "mousemove", "--window", window, "300", "87", "click", "1")
inspect()
y = "303" if mode == "open" else "337"
run(tool, "mousemove", "--window", window, "780", y, "click", "1")
print(f"native-file: clicked owned {mode.title()} at dialog-relative 780,{y}", flush=True)
`
		return exec.CommandContext(driverCtx, "python3", "-c", script, os.Getenv("GOPMGR_NATIVE_X11_TOOL"), fmt.Sprint(pid), mode).CombinedOutput()
	}
	script := `on run argv
set ownerPID to item 1 of argv as integer
set mode to item 2 of argv
tell application "System Events"
tell (first process whose unix id is ownerPID)
repeat 100 times
if exists sheet 1 of window 1 then exit repeat
delay 0.05
end repeat
if mode is "open" then
repeat 100 times
if exists button "Open" of splitter group 1 of sheet 1 of window 1 then exit repeat
delay 0.05
end repeat
set panel to splitter group 1 of sheet 1 of window 1
set fileList to outline 1 of scroll area 1 of splitter group 1 of panel
if (count of rows of fileList) is not 1 then error "expected one owned input file"
set fileRow to row 1 of fileList
set literal to value of text field 1 of UI element 1 of fileRow
if literal is not "資料 input 'quoted'.txt" then error "unexpected native input row: " & literal
log "native-file AX before select: input=" & literal
select fileRow
repeat 100 times
if (selected of fileRow is true) and (enabled of button "Open" of panel is true) then exit repeat
delay 0.05
end repeat
log "native-file AX before accept: selected=" & (selected of fileRow as text) & " enabled=" & (enabled of button "Open" of panel as text)
if selected of fileRow is not true then error "input row is not selected"
if enabled of button "Open" of panel is not true then error "Open is disabled"
click button "Open" of panel
return "native-file: selected owned input and clicked Open button"
else
repeat 100 times
if exists button "Save" of splitter group 1 of sheet 1 of window 1 then exit repeat
delay 0.05
end repeat
set panel to splitter group 1 of sheet 1 of window 1
set literal to value of text field "Save As:" of panel
set destination to value of pop up button "Where:" of panel
log "native-file AX before Save: name=" & literal & " folder=" & destination & " enabled=" & (enabled of button "Save" of panel as text)
if literal is not "報告 output 'quoted'.txt" then error "unexpected native Save name: " & literal
if destination is not "owned folder '資料'" then error "unexpected native Save folder: " & destination
if enabled of button "Save" of panel is not true then error "Save is disabled"
click button "Save" of panel
return "native-file: clicked owned Save button"
end if
end tell
end tell
end run`
	out, err := exec.CommandContext(driverCtx, "/usr/bin/osascript", "-e", script, fmt.Sprint(pid), mode).CombinedOutput()
	if err != nil {
		return out, err
	}
	return out, nil
}
