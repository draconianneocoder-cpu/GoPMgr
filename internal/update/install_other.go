// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !darwin && !windows

package update

import "fmt"

// defaultLauncher is unreachable in practice: installerExtension already
// rejects every GOOS except darwin and windows, and the release pipeline
// never signs a manifest for any other platform (see check.go's package
// doc and .github/workflows/release.yml), so Status.Configured is false
// there by construction. Defined anyway so the package builds on every
// platform GoPMgr supports, and so a future platform addition fails loudly
// here instead of silently doing nothing.
func defaultLauncher(path string) error {
	return fmt.Errorf("update: automatic install is not supported on this platform")
}
