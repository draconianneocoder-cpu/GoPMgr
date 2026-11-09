// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !darwin && !linux && !windows

package db

import "errors"

func publishRestoredFile(_, _ string) error {
	return errors.New("atomic no-replace restore publication is unsupported on this platform")
}
