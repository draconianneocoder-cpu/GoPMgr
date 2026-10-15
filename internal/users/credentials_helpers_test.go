// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

// authenticate signs username in through SignInHeadless and returns the
// account, for tests that check credentials but not the key.
func authenticate(s *Store, username, password string) (Account, error) {
	acc, _, dek, err := s.SignInHeadless(username, password)
	clear(dek)
	return acc, err
}

// unlockDEK signs username in through SignInHeadless and returns the DEK,
// for tests that need an account's key.
func unlockDEK(s *Store, username, password string) ([]byte, error) {
	_, _, dek, err := s.SignInHeadless(username, password)
	return dek, err
}
