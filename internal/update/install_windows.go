// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package update

import "os/exec"

// defaultLauncher starts the downloaded NSIS installer as a detached
// process and returns immediately — it does not wait for the installer to
// finish. The installer requests its own UAC elevation (see
// build/windows/installer/project.nsi) and, on success, offers to relaunch
// GoPMgr via its finish-page "Launch GoPMgr" option.
//
// project.nsi has no running-process detection of its own: its file-copy
// step (wails.files) will fail to overwrite GoPMgr.exe while it is still
// running. The App layer prompts the user to quit GoPMgr — through the
// same guarded quit path every other quit trigger uses, so any
// unsaved-work protection still applies — after this call returns
// successfully. This function only launches the installer; it does not
// wait for or otherwise enforce that the quit happens before the
// installer reaches its file-copy step. In practice the NSIS wizard's
// own Welcome/License/Directory pages require several user clicks before
// that step, which is what leaves time for the prompt — not any ordering
// guarantee between this process and the installer.
func defaultLauncher(path string) error {
	cmd := exec.Command(path) // #nosec G204 -- path is our own verified download, passed as argv.
	return cmd.Start()
}
