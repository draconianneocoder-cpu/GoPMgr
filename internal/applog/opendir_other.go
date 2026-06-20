// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !darwin && !windows

package applog

import (
	"fmt"
	"os/exec"
)

// OpenFolder opens path in the desktop file manager on Linux/BSD. Tries
// xdg-open first (freedesktop standard), then nautilus and thunar as
// specific fallbacks. The path is a discrete argv value, never
// shell-interpolated.
func OpenFolder(path string) error {
	for _, bin := range []string{"xdg-open", "nautilus", "thunar"} {
		if _, err := exec.LookPath(bin); err != nil {
			continue
		}
		if err := exec.Command(bin, path).Run(); err == nil { // #nosec G204 -- allow-listed; path passed as argv.
			return nil
		}
	}
	return fmt.Errorf("no file manager found (tried xdg-open, nautilus, thunar)")
}
