// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !unix

package analytics

import "testing"

func makeFIFO(t *testing.T) string {
	t.Skip("named pipes in the file system are a Unix feature")
	return ""
}
