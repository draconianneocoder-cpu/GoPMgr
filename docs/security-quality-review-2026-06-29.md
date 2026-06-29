<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# PMForge security, quality & safety review — 2026-06-29

Static review of the Go backend, with emphasis on the surfaces the
2026-06-23 pass explicitly left out: the Wails binding / authorization
layer, the filesystem trust boundary between local PMForge users, and the
export sinks. Every finding cites `file:line` and was read directly from
the tree. This pass also reconciles the 5 open GitHub Dependabot alerts
against the frontend lockfile (F-5). No SAST scanners were run.

## Verdict

Strong security posture overall — the cryptography, password handling,
update channel, and admin authorization are correct and defensive. The new
findings are not in those areas. The one item worth acting on in
application code is an **internal authorization-boundary gap**: a
path-confinement control that already exists (`projectPathFor`) is applied
to some filesystem-mutating IPC methods but not others. Two low-cost
hardening items follow (DSN construction, spreadsheet formula injection).
Separately, the 5 open Dependabot alerts all live in frontend **build/dev**
tooling and do not reach the shipped desktop binary (F-5). No critical or
high issues were found in application logic.

> **Status: F-1, F-2, F-3, F-5 resolved (2026-06-29); F-4 accepted.** All
> four actionable findings were implemented and verified in this pass (see
> their resolution notes). F-4 (login throttling) is documented as accepted
> with no change. The application has no users yet, so there is no live
> exposure.

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

**Resolution (2026-06-29).** All four methods now route through
`projectPathFor` before any filesystem work — `OpenProject` and
`EncryptProjectAtRest` validate *before* taking the write lock (the
validator read-locks `a.mu` via `requireUser`, so validating under the
write lock would deadlock), and both now operate on the cleaned, confined
path. New test `TestPathTakingIPCMethodsConfineToOwnProjectsDir`
(`project_path_confinement_test.go`) asserts each method rejects a path
outside the user's projects dir; two pre-existing migration tests that had
seeded the project in a bare `TempDir` were corrected to use the user's own
`projects/` dir. `go test ./...` green.

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

**Resolution (2026-06-29).** `encryptedDSN` (`internal/db/encryption.go`)
now rejects any path containing `?` or `#` before constructing the DSN,
with a comment explaining the go-sqlcipher parsing behaviour that makes
those characters unsafe. This is defence-in-depth behind F-1's confinement
(confined project paths never contain them). New test
`TestEncryptedDSNRejectsAmbiguousPath` covers it. The `hexKey` interpolation
in `exportEncryptedCopy` remains hex-only by `KeyspecHex`'s contract.

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

**Resolution (2026-06-29).** Added a shared leaf package
`internal/exportsafe` with `Cell(s)`, which prepends a single quote when a
value begins with `= + - @` or a leading tab/CR/LF. Applied it to the two
CSV sinks that emit user-controlled text: the schedule exporter's task
`Title` (`internal/export/csv.go`) and the audit-log exporter's
`actor`/`action`/`target`/`details` columns (`internal/db/audit.go`). The
leaf package exists because `internal/db` cannot import `internal/export`
(cycle). **XLSX was deliberately not changed:** excelize stores Go strings
as string-typed cells, which Excel/LibreOffice never evaluate as formulas —
verified empirically against excelize v2.8.1 (a string `"=1+1"` is stored
with no `<f>` element and an empty `GetCellFormula`). Neutralizing XLSX
would corrupt legitimate values like `"-5%"` for no security benefit;
`internal/export/xlsx.go` now carries a comment recording this. Tests:
`exportsafe_test.go` (helper) and `csv_test.go` (exporter). `go test ./...`
green.

### F-4 — INFO (accepted) — No login throttling / lockout

`Login` (`main.go:269`) has no attempt counter or backoff. Acceptable for
a local-first app: each attempt pays the full Argon2id cost (64 MiB, t=3),
making online brute force impractical, and the database requires local
access anyway. **No change recommended** — recorded so the threat model
stays explicit if PMForge ever grows a remote or sync surface.

### F-5 — LOW (dependencies) — All 5 Dependabot alerts are in frontend build/dev tooling, not the shipped binary

GitHub Dependabot reports 5 open alerts on `main` (1 high, 4 moderate).
Reproduced locally with `npm audit` against `frontend/package-lock.json`;
all five are in the Vite/esbuild build-and-dev-server chain, and **none
ship in the packaged Wails desktop binary** (which embeds the *built* Vite
output, not Vite itself):

| Package | Sev | Advisory | Surface |
| --- | --- | --- | --- |
| `vite` (direct devDep, `^5.4.10`) | **High** | GHSA-4w7w-66w2-5vf9 (path traversal in optimized-deps `.map` handling), GHSA-fx2h-pf6j-xcff (`server.fs.deny` bypass on Windows), GHSA-v6wh-96g9-6wx3 (launch-editor NTLMv2 hash disclosure) | Dev server only |
| `esbuild` (transitive via vite) | Moderate | GHSA-67mh-4wv8-2f99 (any website can send requests to the dev server and read responses) | Dev server only |
| `@sveltejs/vite-plugin-svelte` + `-inspector` (direct devDep, `^4.0.0`) | Moderate | Flagged for depending on the vulnerable `vite` range | Build tooling |
| `js-yaml` (nested transitive) | Moderate | GHSA-h67p-54hq-rp68 (quadratic-complexity DoS via repeated merge-key aliases) | Build/lint config parsing |

**Real-world exposure.** The dev-server advisories require a developer to
run `vite dev` with a malicious website open in the same browser; the
`js-yaml` DoS is a build-host concern. End users of the released binary are
unaffected. The value of fixing is (a) protecting the developer/CI host and
(b) keeping Dependabot — and the project's clean-posture goal — green.

**Corrective action**

1. Bump `vite` to `^8` and `@sveltejs/vite-plugin-svelte` to `^7` (both
   semver-major; the plugin major is the one built for Vite 8). This clears
   the `vite`, `esbuild`, and both `@sveltejs/*` alerts in one step.
2. Run `npm audit fix` to resolve the nested `js-yaml` (non-major).
3. Verify the upgrade through the existing gates: `svelte-check`, the Vite
   production build, and `frontend/scripts/smoke-mount.mjs` — Vite 8 is a
   major bump, so confirm the build output and a smoke mount before merge.
4. Effort: ~1-2 hrs including upgrade verification. Treat separately from
   the application-code findings — it touches only `frontend/`.

**Resolution (2026-06-29).** Bumped `vite` `^5.4.10`→`^8.1.0` and
`@sveltejs/vite-plugin-svelte` `^4.0.0`→`^7.1.2` and regenerated
`frontend/package-lock.json` from clean (the stale lock otherwise pinned
the old plugin's `vite-plugin-svelte-inspector@^3`, which conflicts with
Vite 8). The minimal `vite.config.ts` (only the `svelte()` plugin plus
`outDir`/`emptyOutDir`/`target`) needed no migration. `svelte` already
resolved to 5.55.9, satisfying the plugin-7 peer (`^5.46.4`); Node 22.22
satisfies Vite 8's engine. Verified: `npm audit` → **0 vulnerabilities**
(all five Dependabot alerts cleared), `npm run build` (Vite 8/rolldown),
`svelte-check` (0 errors), `eslint`, and `smoke-mount.mjs` all green.

> Note (out of scope, pre-existing): `npm run
> test:project-settings-encryption` fails on `HEAD` independent of this
> upgrade — its hardcoded assertion `onCreated(project, projectPath)` no
> longer matches the source, which calls `onCreated(res.project,
> res.path)`. The encryption wiring it checks is intact; only the literal
> string in the check script drifted. Flagged for a separate fix.

## Priority

1. **F-1** — ~~apply `projectPathFor` confinement to the four IPC methods~~
   **Done (2026-06-29).** (Medium)
2. **F-3** — ~~neutralize spreadsheet cells in CSV exports~~ **Done
   (2026-06-29).** (XLSX confirmed not vulnerable.) (Low/Med)
3. **F-5** — ~~bump `vite`→^8 / `vite-plugin-svelte`→^7; clears all 5
   Dependabot alerts~~ **Done (2026-06-29).** (Low)
4. **F-2** — ~~build the DSN safely / reject `?`~~ **Done (2026-06-29).**
   (Low)
5. **F-4** — document as accepted; revisit only if a sync surface is added.

## Scope / limits

Static read only. The 5 Dependabot alerts were reconciled with
`npm audit` against the frontend lockfile (F-5); the Go side stays clean
under the CI `govulncheck` gate. Not run this pass: `gosec`/`staticcheck`
(already gated in CI per the 2026-06-23 resolution), dynamic/runtime
testing, and a line-by-line audit of every chart/document renderer. The
XLSX exporter is flagged but was not read in full — confirm its cell
handling when F-3 is implemented. Re-run this review against the tree after
the pending application code lands, before corrective code is written.
