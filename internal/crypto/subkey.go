// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
)

// DeriveSubkey deterministically derives a 32-byte key for one local storage
// domain from the account's session DEK. The context must be a stable,
// domain-specific ASCII label; callers must zero the returned key when done.
func DeriveSubkey(dek []byte, context string) ([]byte, error) {
	if len(dek) != DEKSize {
		return nil, ErrBadDEK
	}
	if context == "" {
		return nil, errors.New("crypto: empty subkey context")
	}
	mac := hmac.New(sha256.New, dek)
	_, _ = mac.Write([]byte(context))
	return mac.Sum(nil), nil
}
