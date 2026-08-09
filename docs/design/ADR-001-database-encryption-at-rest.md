<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-001: Per-user database encryption at rest

**Status:** Implemented
**Decision date:** 2026-06-12

## Context

GoPMgr stores account bootstrap data in `system.db` and project data in
per-user `.gopmgr` databases. Local file permissions and whole-device
encryption reduce exposure, but do not protect a copied project database when
its storage location is exposed.

The design must preserve SQLite transactions, WAL behavior, backups, repair,
and password recovery without keeping a plaintext working copy on disk.

## Decision

Use `github.com/mutecomm/go-sqlcipher/v4` as the SQLite driver for encrypted
project databases. Each user has one randomly generated 32-byte data-encryption
key (DEK). The DEK is wrapped by the login password and by each valid recovery
code. New project databases are encrypted by default; eligible plaintext
projects can be migrated from Project Settings.

`system.db` stays plaintext by design. It contains account names, password
hashes, recovery-code metadata, and wrapped DEKs, but not project records.
Keeping it available before login avoids an unresolvable bootstrap problem.

## Key hierarchy

```text
login password ──Argon2id──► password KEK ──┐
recovery code ──Argon2id──► recovery KEK ──┼──► one user DEK
                                            │        │
                                            └────────┴──► SQLCipher project databases
```

Changing a password re-wraps the same DEK. Resetting with a valid recovery
code also re-wraps that DEK, so encrypted projects remain available. If the
password and every valid recovery code are lost, recovery is intentionally
impossible.

## Migration

The plaintext-to-encrypted migration verifies the source, exports into a
temporary encrypted sibling with `sqlcipher_export`, validates the destination,
then publishes it atomically. The original remains as a clearly named backup
until the user removes it. A failed migration leaves the original project
unchanged.

## Consequences

- Every database opener, including repair, backup verification, and headless
  maintenance, needs authenticated access to the DEK.
- PAdES and PDF/A validation are independent of the database driver.
- SQLCipher source and release builds require CGO, which this project already
  uses.
- Whole-device encryption remains recommended for host-level threats:
  FileVault, BitLocker, or LUKS complement project-file encryption.

## Rejected alternatives

- A whole-file AES-GCM envelope would require an on-disk plaintext working copy
  while SQLite uses WAL files.
- Field encryption would complicate querying, integrity, repair, and backup
  without protecting the database metadata consistently.
- Relying only on whole-device encryption does not protect copied project
  files.

## Verification

`internal/crypto`, `internal/users`, and `internal/db` cover DEK wrapping,
wrong-key rejection, encrypted opens, migration, and recovery-code behavior.
`make check-encrypted-db` and `make check-release` are the release gates for
the encrypted database contract.
