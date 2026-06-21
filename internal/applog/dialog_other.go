// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !darwin && !windows

package applog

import "os/exec"

// ShowError displays a native error dialog on Linux/BSD desktops, trying
// the common helpers in order (zenity, then kdialog, then notify-send as
// a notification fallback). Title and message are passed as discrete
// argv values (no shell), so they cannot be interpreted as commands.
// Best-effort: if none of the helpers exist or no display is available,
// the call is a no-op because the error has already been logged.
func ShowError(title, message string) {
	candidates := [][]string{
		{"zenity", "--error", "--title", title, "--text", message},
		{"kdialog", "--error", message, "--title", title},
		{"notify-send", title, message},
	}
	for _, args := range candidates {
		if _, err := exec.LookPath(args[0]); err != nil {
			continue
		}
		// Helper name is from the fixed allow-list above; title and message
		// are discrete argv values, never shell-interpreted.
		if err := exec.Command(args[0], args[1:]...).Run(); err == nil { // #nosec G204 -- allow-listed helper; dynamic text passed as argv.
			return
		}
	}
}
