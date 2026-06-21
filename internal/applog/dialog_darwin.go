// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package applog

import "os/exec"

// ShowError displays a native macOS error alert via osascript. The title
// and message are passed as AppleScript run-handler arguments (not
// interpolated into the script source) so user/error text cannot break
// out of or inject into the script. Best-effort: any failure (e.g. no
// window server) is ignored because the error has already been logged.
func ShowError(title, message string) {
	const script = `on run argv
display dialog (item 2 of argv) with title (item 1 of argv) buttons {"OK"} default button "OK" with icon stop
end run`
	// title/message are positional argv to the run handler, never shell- or
	// script-interpolated.
	_ = exec.Command("/usr/bin/osascript", "-e", script, title, message).Run() // #nosec G204 -- fixed binary; dynamic text passed as argv.
}
