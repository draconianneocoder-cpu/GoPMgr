<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# PMForge security, quality & safety review — 2026-06-29

Static review of the Go backend, with emphasis on the surfaces the
2026-06-23 pass explicitly left out: the Wails binding / authorization
layer, the filesystem trust boundary between local PMForge users, and the
export sinks. Every finding cites `file:line` and was read directly from
the tree. No dynamic scanners were run this pass.

## Verdict

Strong security posture overall — the cryptography, password handling,
update channel, and admin authorization are correct and defensive. The new
findings are not in those areas. The one item worth acting on is an
**internal authorization-boundary gap**: a path-confinement control that
already exists (`projectPathFor`) is applied to some filesystem-mutating
IPC methods but not others. The remaining two are low-cost hardening items
(DSN construction, spreadsheet formula injection). No critical or high
issues found.

> **Status: open.** No code changed in this pass. Corrective code is
> deferred by decision — more application code is still landing, and a
> follow-up review will run against the updated tree before fixes are
> implemented together. The application has no users yet, so there is no
> live exposure to remediate under time pressure.

## What is already correct (grounded)

- **Crypto primitives** — AES-256-GCM with a fresh per-call salt and nonce
  and a length-checked open (`internal/crypto/encrypt.go:47-110`); DEK
  wrap/unwrap is authenticated and length-validated
  (`internal/crypto/keywrap.go:42-68`). No nonce reuse, `crypto/rand`
  throughout.
- **Password hashing** — Argon2id at OWASP-2023 params, versioned PHC
  strings with transparent re-hash on login, `subtle.ConstantTimeCompare`
  (`internal/auth/password.go:30-139`).
- **Update channel** — Ed25519-signed manifest, HTTPS-pinned, fail-closed
  on an empty key, size-limited body, downgrade-resistant
  (`internal/update/check.go`, `internal/update/manifest.go`).
- **Admin authorization** — every admin IPC method is gated server-side on
  the in-process session's `caller.IsAdmin`, never on a frontend-supplied
  flag (`main.go:175-263`). The webview cannot spoof it.
- **XSS** — no `{@html}` anywhere in the Svelte tree; the HTML export
  escapes every interpolated field via `html.EscapeString`
  (`internal/export/html.go`).
- **Prior F2 resolved (verified)** — `govulncheck ./...` is now a blocking
  CI job for both the default and the duckdb-tagged build
  (`.github/workflows/ci.yml:199-233`), closing the "scanners configured
  but never run" gap from the previous review.

## Findings

### F-1 — MEDIUM — Filesystem-mutating IPC methods skip the path-confinement check

`projectPathFor` (`main.go:450`) exists, in its own doc comment, *"so
DeleteProject and CloneProject can never touch arbitrary files on disk"*:
it proves a path is a `.pmforge` file inside the **signed-in user's own**
`projects/` directory and rejects anything outside it. `DeleteProject`
(`main.go:475`) and `CloneProject` (`main.go:510`) use it. Three other
path-taking IPC methods do not:

| Method | Location | Effect on an unvalidated path |
| --- | --- | --- |
| `EncryptProjectAtRest` | `main.go:949` | **Mutates the filesystem.** `MigratePlaintextToEncrypted` (`internal/db/encryption.go:55`) renames the target to `<path>.pre-encryption.bak`, writes a new file at `path`, and `chmod`s it. |
| `SecureArchive` | `main.go:2362` → `internal/admin/workflow.go:29` | Writes a backup artifact to a path derived from the unvalidated `projectPath`. |
| `OpenProject` / `IsProjectEncrypted` | `main.go:907` / `main.go:941` | Read-only and lower impact — but `OpenProject` also feeds the raw path into the SQLCipher DSN (see F-2). |

**Why it matters under this threat model.** PMForge's supported model is
multiple PMForge users sharing one OS account
(`internal/users/store.go:14-16`). The IPC boundary is the only thing
separating one PMForge user's files from another's within that OS account.
`EncryptProjectAtRest` is the sharp edge: a crafted path gives a logged-in
user a rename/overwrite/chmod primitive outside their own `projects/`
sandbox. The migration rejects already-encrypted targets and non-SQLite
files, which narrows the reach, but a plaintext SQLite file elsewhere on
disk can still be renamed and replaced. It is also the write primitive a
future webview compromise would reach for. The control already exists; it
is simply applied inconsistently.

**Corrective action**

1. Route `EncryptProjectAtRest`, `IsProjectEncrypted`, `OpenProject`, and
   `SecureArchive` through `projectPathFor` (or a shared
   `confineToProjects(path)` helper) so each returns the cleaned, confined
   path and rejects anything outside the user's `projects/` tree —
   mirroring `DeleteProject`/`CloneProject`.
2. Add a regression test alongside `user_isolation_test.go` asserting each
   of these methods rejects a path outside `DataDir/projects`.
3. Effort: ~1-2 hrs. Validation-only; low blast radius.

### F-2 — LOW — Encrypted DSN built by string concatenation; a `?` in the path injects `_pragma_*` options

`encryptedDSN` (`internal/db/encryption.go:156`) returns
`path + "?_pragma_key=x'" + hexKey + "'"` with the path neither
URL-escaped nor validated. A path containing `?` — reachable today only
through the unconfined `OpenProject` of F-1 — is parsed by `go-sqlcipher`
as additional DSN query options, letting an attacker-influenced path
append or override `_pragma_*` settings (including a competing
`_pragma_key`). This compounds F-1.

Separately, `exportEncryptedCopy` (`internal/db/encryption.go:170`)
interpolates `hexKey` into the `ATTACH … KEY` statement unescaped. Safe
**today** because `KeyspecHex` emits `%X` (hex only), but brittle if that
function ever changes.

**Corrective action**

1. Build the DSN with `net/url` (`url.Values`) — or reject any path
   containing `?`/`#` before constructing it. Fixing F-1's confinement
   also closes the practical vector.
2. Add a comment at `exportEncryptedCopy` pinning the hex-only invariant of
   `KeyspecHex` as a contract.
3. Effort: ~30 min.

### F-3 — LOW/MEDIUM — CSV (and likely XLSX) export has no formula-injection neutralization (CWE-1236)

`renderCSV` (`internal/export/csv.go:32`) writes `t.Title` and other
user-controlled fields raw. `encoding/csv` quotes correctly for *parsing*
but does not neutralize spreadsheet *formulas*: a task title beginning with
`=`, `+`, `-`, `@`, or a leading tab/CR is executed as a formula when the
export is opened in Excel or LibreOffice. The impact is realized when an
exported schedule is shared and opened by someone else — exactly the
"interchange format for spreadsheets" use the function advertises. The
XLSX path (`internal/export/xlsx.go`) should be audited the same way.

**Corrective action**

1. Add a `neutralizeSpreadsheetCell` helper that prefixes a single quote
   (`'`) when a cell begins with `= + - @` or a control character, and
   apply it to user-sourced fields in the CSV and XLSX exporters.
2. Add a unit test covering each dangerous prefix.
3. Effort: ~1 hr.

### F-4 — INFO (accepted) — No login throttling / lockout

`Login` (`main.go:269`) has no attempt counter or backoff. Acceptable for
a local-first app: each attempt pays the full Argon2id cost (64 MiB, t=3),
making online brute force impractical, and the database requires local
access anyway. **No change recommended** — recorded so the threat model
stays explicit if PMForge ever grows a remote or sync surface.

## Priority

1. **F-1** — apply `projectPathFor` confinement to the four IPC methods
   that mutate or open files by path; add isolation tests. (Medium)
2. **F-3** — neutralize spreadsheet cells in CSV + XLSX exports. (Low/Med)
3. **F-2** — build the DSN via `url.Values`/reject `?`; pin the hex
   contract. (Low)
4. **F-4** — document as accepted; revisit only if a sync surface is added.

## Scope / limits

Static read only. Not run this pass: `govulncheck`/`gosec`/`staticcheck`
(already gated in CI per the 2026-06-23 resolution), dynamic/runtime
testing, and a line-by-line audit of every chart/document renderer. The
XLSX exporter is flagged but was not read in full — confirm its cell
handling when F-3 is implemented. Re-run this review against the tree after
the pending application code lands, before corrective code is written.
