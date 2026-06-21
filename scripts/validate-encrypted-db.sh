#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Deterministic SQLCipher encrypted database validation gate.
#
# The targeted Go tests create real encrypted .pmforge databases,
# verify encrypted headers and wrong-key rejection, migrate plaintext
# fixtures with SQLCipher integrity checks, exercise encrypted repair
# snapshot swapping, verify .pmba backups preserve encrypted
# project.pmforge bytes, and prove headless maintenance can unlock an
# encrypted project through system.db using --username/--password-env.

set -eu
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "=== Encrypted Database Validation Gate ==="

go test -count=1 -run 'Test(InitEncryptedDBCreatesEncryptedDatabase|InitEncryptedDBRejectsWrongKey|MigratePlaintextToEncryptedPreservesData|OpenEncryptedDBRejectsBadDEKLength|SwapInEncryptedSnapshotPreservesEncryptionAndReopensWithDEK|CreateArchivalBundlePreservesEncryptedProjectBytes)$' ./internal/db

go test -count=1 -run 'Test(CreateProjectEncryptsAndReopensWithSessionDEK|CreateProjectFromLaunchpadEncryptsProject|OpenProjectRejectsDifferentUsersDEK|OpenProjectPlaintextRequiresMigration|EncryptProjectAtRest|OpenHeadlessDB)' .

echo "Encrypted database validation gate PASSED."
