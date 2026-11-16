// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import "strings"

// CopyRecoveryCodes puts freshly shown recovery codes on the clipboard, one
// per line, and clears them 60 seconds later if the clipboard still holds
// them, or at a normal quit before that (clipboard_guard.go). It accepts only
// what the Save methods accept.
func (a *App) CopyRecoveryCodes(codes []string) error {
	if err := a.checkShownRecoveryCodes(codes); err != nil {
		return err
	}
	if a.native == nil {
		return errClipboardUnavailable
	}
	return a.native.clipboard.copy(strings.Join(codes, "\n"))
}
