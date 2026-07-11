<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# PMForge security, quality & stability review — 2026-07-11

Static review of the security-sensitive backend code that landed on `main`
after the 2026-07-05 pass and had not yet been reviewed:

- the **headless maintenance CLI** — `--stats`, `--schema-dump`, and
  `--export` (with optional `--encrypt`), plus the shared
  `openHeadlessDB` project opener (`main.go:4688-4914`); and
- **admin-issued recovery codes** for admin-created accounts —
  `App.AdminIssueRecoveryCodes` (`main.go:276`) and the store method it
  calls, `users.Store.IssueRecoveryCodes` (`internal/users/recovery.go:68`).

Everything else merged since 2026-07-05 is frontend accessibility work and a
`golang.org/x` dependency bump (both CI-verified), which carry no new
backend security surface and are out of scope here.

## Verdict

**No findings.** Both features uphold the codebase's established
invariants — auth/DEK confinement, no-secret-logging, secret-at-rest as
hashes/wrapped-keys only, parameterized SQL, and explicit authorization
gates. No confidentiality, integrity, or stability defect found. No
corrective action required.

## Verified correct (grounded, no action)

### Headless CLI cannot bypass project encryption

`openHeadlessDB` (`main.go:4688`) inspects the target with
`db.IsEncryptedFile` and branches:

- **Unencrypted project** → opened directly. Consistent with the app's
  threat model (an unencrypted `.pmforge` is plaintext by the owner's
  choice; anyone with filesystem access can already read it — SQLCipher is
  not claimed to protect it).
- **Encrypted project** → requires `--username` **and** `--password-env`,
  then runs the *same* trust path as the GUI: `store.Authenticate`,
  `store.UnlockDEK`, and `db.InitEncryptedDB(path, dek)`
  (`main.go:4696-4722`). Filesystem access alone therefore cannot decrypt a
  project via `--stats` / `--schema-dump` / `--export`; the user's password
  is mandatory. This is the F-1 boundary principle applied to the CLI entry
  point.

### Export-password input hygiene

The export/maintenance password is read from an **environment variable**
named by `--password-env` (`headlessExportPassword`, `main.go:4905`), never
from a command-line flag — so it does not leak through the process table
(`ps`), shell history, or `/proc/<pid>/cmdline`. `--encrypt` without
`--password-env`, or an unset/empty variable, fails closed with a clear
error before any work is done.

### Export output is owner-private

`runHeadlessExport` writes the rendered report with mode `0o600`
(`main.go:4873`); with `--encrypt` the bytes are AES-GCM sealed under the
resolved password via the same `export` path the GUI uses. `--schema-dump`
emits DDL structure only (no row data). User-supplied output paths are not a
privilege boundary here — the CLI runs as the invoking user against their
own filesystem.

### Recovery codes: admin gate + DEK proof-of-possession

`AdminIssueRecoveryCodes` (`main.go:276`) requires a signed-in **admin**
caller (`caller.IsAdmin`) and unwraps the target account's DEK with that
account's password (`store.UnlockDEK(username, password)`) before issuing.
The codes therefore wrap the real DEK, and issuance requires proof of the
account password rather than admin privilege alone. The password-check error
is only reachable by an already-authenticated admin, so it is not a
user-enumeration vector.

### Recovery codes: secret-at-rest is hash + wrapped-key only

`users.Store.IssueRecoveryCodes` (`internal/users/recovery.go:68`):

- generates each code from a CSPRNG (`crypto/rand` via `generateCode`,
  base32, `recovery.go:254`);
- persists **only** the Argon2id `code_hash` (`auth.HashPassword`) and a
  `wrapped_dek` produced by `crypto.WrapKey(dek, code)` — never the
  plaintext code and never the raw DEK;
- rotates atomically in a transaction (delete-then-insert), and returns the
  plaintext codes exactly once for immediate display. Storage is
  parameterized SQL throughout.

This matches the documented contract on the GUI `IssueRecoveryCodes` and
ADR-001's key hierarchy.

### No secret logging

A scan of the new paths (`internal/users/*`, the headless CLI handlers)
found no `log` / `fmt.Print` / `slog` / `applog` statement emitting a code,
DEK, password, or other recovery material.

### Test coverage present

The new surface ships with tests: `admin_recovery_test.go`
(`TestAdminIssueRecoveryCodesForCreatedUser`, and a
`...RequiresAdmin` authorization test) and `headless_cli_test.go`
(`TestRunHeadlessExportWritesFile`, `...Encrypts`, and
`...EncryptRequiresPassword`).

## Priority

None — no findings.

## Scope / limits

Static read of the two features named above, plus `go build ./...`,
`go build -tags duckdb ./...`, `go test ./...`, and `govulncheck ./...`
(all green on the current `main`; govulncheck: 0 reachable, the single
remaining `golang.org/x/crypto` GO-2026-5932 has no upstream fix and is not
called). Not covered: dynamic/runtime exercise of the CLI against a live
encrypted project, a line-by-line audit of every export renderer, and the
shell validation scripts under `scripts/` (carried over from the prior
review's out-of-scope list).
