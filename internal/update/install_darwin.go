// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package update

import "os/exec"

// defaultLauncher opens the downloaded .dmg in Finder — the platform's
// standard drag-to-Applications install flow. It deliberately does not
// attempt to mount, copy, or replace GoPMgr's own running .app bundle in
// Go code: Finder's own disk-image handling is the trusted, well-understood
// mechanism users already expect, and doing it ourselves would mean
// reasoning about replacing a running app bundle's contents in place, a
// materially higher-risk undertaking this feature deliberately does not
// take on. The path is passed as a discrete argv value, never interpolated
// into a shell string, matching internal/applog/opendir_darwin.go's
// established idiom.
func defaultLauncher(path string) error {
	return exec.Command("/usr/bin/open", path).Run() // #nosec G204 -- fixed binary; path passed as argv.
}
