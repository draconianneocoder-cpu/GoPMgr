// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import "testing"

// requireSerial marks a test that replaces process-wide state, such as a
// package hook or crypto/rand.Reader, for its duration. t.Setenv panics in
// a parallel test, so calling a helper that uses this from a test marked
// t.Parallel() fails at once instead of leaking the replacement into tests
// running alongside it.
func requireSerial(t *testing.T) {
	t.Helper()
	t.Setenv("GOPMGR_TEST_REPLACES_GLOBALS", "1")
}
