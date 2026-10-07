// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

// Administrator escrow tables (ADR-004). Each account's rows go with its
// users row (ON DELETE CASCADE); escrow keys are machine-wide and kept when
// retired, so rows sealed to an old key still name it.
//
//   - escrow_keys: the escrow key pairs' public halves. At most one is
//     active (an empty retired_at), enforced by a partial unique index.
//   - personal_keys: each account's key pair; the private key is wrapped
//     under a subkey of the account's DEK, and attestation is the escrow
//     key's MAC of the public key.
//   - sealed_deks: each account's DEK sealed to an escrow key.
//   - escrow_pins: each account's MAC of the escrow key it trusts.
//   - escrow_grants: the escrow private key sealed to an administrator's
//     personal key.
const escrowSchema = `
CREATE TABLE IF NOT EXISTS escrow_keys (
	id         TEXT PRIMARY KEY,
	kem_id     INTEGER NOT NULL,
	public_key BLOB NOT NULL,
	created_at TEXT NOT NULL,
	retired_at TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS escrow_keys_one_active
	ON escrow_keys(retired_at) WHERE retired_at = '';
CREATE TABLE IF NOT EXISTS personal_keys (
	username               TEXT PRIMARY KEY REFERENCES users(username) ON DELETE CASCADE,
	public_key             BLOB NOT NULL,
	wrapped_private_key    BLOB NOT NULL,
	attestation            BLOB NOT NULL DEFAULT x'',
	attested_escrow_key_id TEXT NOT NULL DEFAULT '',
	created_at             TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS sealed_deks (
	username      TEXT PRIMARY KEY REFERENCES users(username) ON DELETE CASCADE,
	escrow_key_id TEXT NOT NULL REFERENCES escrow_keys(id),
	sealed        BLOB NOT NULL,
	sealed_at     TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS escrow_pins (
	username      TEXT PRIMARY KEY REFERENCES users(username) ON DELETE CASCADE,
	escrow_key_id TEXT NOT NULL REFERENCES escrow_keys(id),
	pin           BLOB NOT NULL
);
CREATE TABLE IF NOT EXISTS escrow_grants (
	admin_username TEXT PRIMARY KEY REFERENCES users(username) ON DELETE CASCADE,
	escrow_key_id  TEXT NOT NULL REFERENCES escrow_keys(id),
	sealed         BLOB NOT NULL,
	granted_at     TEXT NOT NULL
);
`

// migrateEscrowTables creates the escrow tables. Idempotent.
func (s *Store) migrateEscrowTables() error {
	_, err := s.conn.Exec(escrowSchema)
	return err
}
