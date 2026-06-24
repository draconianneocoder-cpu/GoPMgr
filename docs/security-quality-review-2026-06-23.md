<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# PMForge security, quality & safety review — 2026-06-23

Static review of the Go backend, packaging scripts, and CI config. Every
finding cites `file:line` and was read directly from the tree; no dynamic
scanners (gosec/govulncheck) were run in this pass — see F2.

## Verdict

Strong security posture. The cryptography, SQL, command-exec, and
path-handling code follow correct, defensive patterns with no high or
critical issues found. The two actionable items are **operational** —
unverified build-tool downloads and security scanners that are configured
but never actually execute in CI — not flaws in the application logic.

## What is already correct (grounded)

- **Password hashing** — Argon2id at `t=3, m=64 MiB, p=4, keyLen=32,
  salt=16` (`internal/auth/password.go:30-37`), meeting/exceeding OWASP
  2023. The stored hash is **versioned** with its parameters
  (`password.go:62-66, 91`), so parameters can be raised later without
  breaking existing accounts.
- **Constant-time verification** — `subtle.ConstantTimeCompare`
  (`password.go:119`); no `==`/`bytes.Equal` on secret material.
- **Randomness** — `crypto/rand` exclusively for every salt, GCM nonce,
  DEK, and signature (`encrypt.go:53,68`, `keywrap.go:32`,
  `pdf_sign.go:103`, `pdf_cms.go:150`). No `math/rand` in any security
  path.
- **Symmetric encryption** — AES-GCM with a fresh random nonce and a
  length-checked open (`encrypt.go:84-91`); no nonce reuse.
- **SQL injection** — parameterized queries throughout. The **only**
  string-built SQL in the entire tree is the DuckDB import
  (`internal/analytics/duckdb.go:208`), and it is safe: the path is
  single-quote-escaped (`sqlSingleQuote`), the reader is from a fixed
  whitelist (`read_csv_auto`/`read_parquet`/`read_json_auto`), and the
  limit is a compile-time constant. See F5 for the maintainer guard-rail.
- **Command execution** — every `exec.Command` uses a fixed binary and
  passes dynamic text as `argv` (never a shell string):
  `internal/applog/*` dialogs and open-dir helpers. The Windows
  PowerShell path passes text via environment, not the command line. All
  `#nosec G204` annotations are justified.
- **Path traversal** — `sanitizeFilename` (`main.go:3656`) maps
  `/ \ : * ? " < > |` to `_` and drops control chars, and
  `newProjectPath` (`main.go:591`) **always** prefixes the folder with a
  `20060102-150405-` timestamp. A malicious name of `..` therefore lands
  as the literal folder `…-..` under the projects dir — it cannot escape,
  because no standalone `..` component can survive separator stripping.
- **Key model** — `system.db` is intentionally plaintext and holds only
  Argon2id password hashes and *wrapped* DEKs, not project data
  (ADR-001). Per-project `.pmforge` files are SQLCipher-encrypted with the
  user DEK. Documented and coherent (F6).
- **Dependencies** — `x/crypto v0.47.0`, `x/net v0.49.0` (`go.mod`) are
  current post-DuckDB tidy; the earlier Dependabot Go advisories are
  addressed. DuckDB is `duckdb`-tag-gated and absent from default builds.

## Findings

### F1 — MEDIUM — AppImage build tools are unpinned and unverified (supply chain)

`scripts/package-appimage.sh:34-36` downloads `linuxdeploy` and
`linuxdeploy-plugin-gtk` from the **rolling `continuous`** GitHub release,
`chmod +x`, and executes them inside the release pipeline:

```sh
fetch() { curl -fsSL "$1" -o "$2"; chmod +x "$2"; }
fetch ".../releases/download/continuous/linuxdeploy-x86_64.AppImage" ...
```

`continuous` is a moving target (non-reproducible), and there is no
checksum check before execution. A compromised, MITM'd, or silently
re-pushed artifact would run with the build and could tamper the published
AppImage — the one format end users are told is "portable, just run it."

**Fix:** pin a specific `linuxdeploy` release tag, record its SHA256, and
verify (`sha256sum -c`) before `chmod +x`. Fail the build on mismatch.
This is the highest-value item and is contained to one script.

### F2 — MEDIUM — Security scanners are configured but never run in CI

`make memory-scan` runs `gosec`, `staticcheck`, and `govulncheck` **only
if already on PATH** and *skips silently* otherwise
(`scripts/memory-safety-scan.sh:22-25,238-248`). The CI security job runs
`make memory-scan` but does not install those tools, so in practice **no
SAST and no dependency-vulnerability scan executes automatically.**
`.golangci.yml` separately disables `staticcheck`, so it runs nowhere.

Given the recent Dependabot churn (xlsx, x/crypto, x/net), an automated
`govulncheck` gate is the single most useful addition.

**Fix:** add a CI step that installs and runs `govulncheck ./...`
(fail on finding) and `gosec`. Treat them as a real gate, not best-effort.

### F3 — LOW — Disabled linters reduce ongoing coverage

`.golangci.yml` disables `errcheck`, `staticcheck`, and `unused` to keep CI
deterministic against a legacy baseline. Reasonable as a temporary
paydown, but new unchecked errors and dead code now land unflagged.

**Fix:** re-enable incrementally with an exclude/baseline so only *new*
findings fail, then burn down the baseline.

### F4 — INFO (accepted risk) — DEK lives as an immutable hex string

`KeyspecHex` (`internal/crypto/keywrap.go:75-79`) renders the raw DEK as a
64-char hex **string** for the SQLCipher `_pragma_key` DSN. Go strings are
immutable and cannot be zeroed, so that copy of the key persists on the
heap until GC, defeating `[]byte` wiping for that value. Inherent to the
DSN keyspec approach and acknowledged by ADR-001; practical risk is low
for a single-user local-desktop threat model (at-rest, not live-memory
extraction).

**Optional hardening:** keep the hex string's scope as narrow as possible;
if the driver supports it, set the key as `[]byte` via `PRAGMA key` on the
open connection rather than embedding it in the DSN.

### F5 — INFO (maintainer guard-rail) — keep DuckDB SQL parameter-safe

`duckdb.go:208` is safe today (escaped path + whitelisted reader + const
`LIMIT`). Guard-rail for future edits: never interpolate untrusted table
or column identifiers into that string, and keep `tabularReader` a fixed
whitelist. Memory is bounded by `maxImportRows=10000` and
`SetMaxOpenConns(1)` serializes the in-memory engine — keep both.

### F6 — INFO (confirm, no action) — plaintext `system.db` is by design

Documented in README "Security Model" and ADR-001: holds Argon2id hashes
and wrapped DEKs only. Whole-disk encryption (FileVault/BitLocker/LUKS) is
the recommended complement. No change; just keep the threat model stated
as the schema evolves.

## Priority

1. **F1** — pin + checksum the AppImage build tools.
2. **F2** — make `govulncheck` (and gosec) a real CI gate.
3. **F3** — re-enable `errcheck`/`staticcheck`/`unused` incrementally.
4. **F4 / F5** — hardening notes; no urgent action.

## Scope / limits

Static read only. Not run this pass: `govulncheck`/`gosec`/`staticcheck`
(F2 is exactly that gap), the Svelte frontend bridge surface beyond the
two analytics methods added this session, and any dynamic/runtime testing.
Re-run after F2 lands to confirm a clean dependency scan.
