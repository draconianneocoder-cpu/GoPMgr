<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# SQLCipher WAL-reset correction evidence

## Risk and correction

The embedded SQLite 3.33.0 core predates SQLite's WAL-reset correction. During
a PASSIVE checkpoint, another connection can reset and reuse the WAL after the
checkpoint caches its header but before it takes read-lock slot zero. The old
checkpoint can then backfill frames or advance bookkeeping using a stale WAL
generation.

GoPMgr backports SQLite commit
`fe57e14b49f9189b56da8233ab3415ce5ff6b1ff`. Once the checkpoint owns read-lock
slot zero, it compares the live WAL salt with the cached header. A mismatch
skips all backfill I/O and bookkeeping for that stale generation while leaving
lock and iterator cleanup unchanged. This is a confirmed affected-source
condition, not evidence that a GoPMgr user database was corrupted.

## Test design

`TestWALResetCheckpointPreservesCommittedData` reconstructs the exact published
v4.4.2 amalgamation and refuses to run unless its SHA-256 matches the recorded
source. A temporary-only fault hook pauses the checkpoint at the upstream
interleaving point. A second real connection commits the write that resets the
WAL. The pristine plaintext and keyed controls must report stale-generation
backfill with their dedicated exit code; compile failures, crashes, and timeouts
do not count as defect detection. The corrected cases must preserve exact row
contents through later WAL growth, a fresh checkpoint, integrity checks, close,
and reopen.

`TestEncryptedFilesInteroperateWithUnmodifiedDriver` builds a separate helper
module with `GOWORK=off`, verifies the unmodified v4.4.2 amalgamation hash, and
tests encrypted files in both creation directions. Both drivers must read and
append the exact rows, reject a wrong key, pass integrity checks, and retain an
encrypted header. `TestVendoredSQLCipherProvenance` separately pins the runtime
inventory, tree hashes, and complete binary-notice artifact.

Run the ordinary gate with:

```sh
go test -count=1 ./internal/sqlitedriver
make check-encrypted-db
```

AddressSanitizer is an optional local diagnostic for the native harness on a
compiler that supports it:

```sh
GOPMGR_SQLCIPHER_ASAN=1 go test -count=1 ./internal/sqlitedriver \
  -run '^TestWALResetCheckpointPreservesCommittedData$'
```

## Evidence boundary

The deterministic harness exercises a real two-connection reset schedule on
the current native host. It does not prove absence of every SQLite defect,
power-loss durability, or Windows locking behavior. The Go race detector does
not instrument the C amalgamation, so the native causal control is the relevant
proof for this interleaving. Windows native execution and platform power-loss
testing remain separate release evidence.
