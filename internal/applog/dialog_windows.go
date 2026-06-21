// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package applog

import "os/exec"

// ShowError displays a native Windows error dialog via PowerShell's
// MessageBox. Title and message are passed through environment variables
// (PMFORGE_DIALOG_TITLE / PMFORGE_DIALOG_MESSAGE) rather than embedded in
// the command string, so they cannot inject into the PowerShell command.
// Best-effort: failures are ignored because the error is already logged.
func ShowError(title, message string) {
	const command = `Add-Type -AssemblyName PresentationFramework; ` +
		`[System.Windows.MessageBox]::Show($env:PMFORGE_DIALOG_MESSAGE, $env:PMFORGE_DIALOG_TITLE, 'OK', 'Error') | Out-Null`
	// Dynamic text arrives via env vars, not via the command string.
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", command) // #nosec G204 -- fixed command string; text passed via env.
	cmd.Env = append(cmd.Environ(),
		"PMFORGE_DIALOG_TITLE="+title,
		"PMFORGE_DIALOG_MESSAGE="+message,
	)
	_ = cmd.Run()
}
