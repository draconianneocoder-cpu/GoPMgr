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

> **Status (updated): all findings resolved (2026-06-23).** F1 — AppImage
> delivery removed entirely (supply-chain surface gone; was pinned in `e862c4e`
> before removal); F2 — `govulncheck` is a
> blocking CI gate (`e862c4e`), Windows installer collection hardened
> (`9dfcd2f`); F3 — all three linters re-enabled and the first-party backlog
> cleared in code (`63b664f`/`6079e90`/`b1030e6`), `make verify` + a clean
> `golangci-lint run` confirmed; F4 — raw-key string scope narrowed and the
> SQLCipher keyspec constraint documented (`6575f69`, with the regression fix
> `77e8aa8`); F5/F6 — confirmed, no action needed. A follow-up deep review of
> the DuckDB engine, the Wails analytics bridge, concurrency, recovery-code
> entropy, and resource handling found no further vulnerabilities or bugs.

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
  addressed. DuckDB is still `duckdb`-tag-gated, and production/package builds
  now enable that tag so installers include analytics.

## Findings

### F1 — MEDIUM (resolved) — AppImage build tools were unpinned (supply chain)

Original finding: `scripts/package-appimage.sh` downloaded `linuxdeploy` and
`linuxdeploy-plugin-gtk` from the rolling `continuous` GitHub release and
executed them in the pipeline with no checksum — a compromised or re-pushed
artifact could have tampered the build.

**Resolution.** First pinned + verified fail-closed by SHA-256 (`e862c4e`),
then the **AppImage format was removed entirely** (2026-06-23): the script and
its tool downloads no longer exist, so the supply-chain surface is gone. `.deb`
and `.rpm` (built by `nfpm` from tracked config, no network tool downloads)
cover Linux.

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

### F3 — LOW (resolved) — Disabled linters reduce ongoing coverage

`.golangci.yml` had disabled `errcheck`, `staticcheck`, and `unused`.

**Resolution (2026-06-23).** Root-caused and fixed, not deferred. The
"legacy baseline" was partly `node_modules` noise (a third-party Go file
vendored in an npm package, now excluded) and partly a real ~43-issue
first-party backlog that the disabled linters had been masking. All three
linters are re-enabled (golangci-lint v2 default set) and the backlog is
cleared **in code**: unchecked errors wrapped with explicit `_ =`, four dead
functions removed, several `staticcheck` simplifications (`S1016`, `QF*`,
`SA9009`), and one genuine latent test bug (`SA5011`: `t.Error` then deref →
`t.Fatal`). Only two exclusions remain, both justified in the config:
`errcheck` on `_test.go` (test cleanup conventionally ignores `Close`) and
`ST1005` (user-facing error strings are intentionally capitalized; one is
matched verbatim by the frontend, so the text is a contract).
`issues.max-same-issues: 0` keeps the full count visible. Verified:
`golangci-lint run` = 0 issues, `make verify` green.

### F4 — INFO (accepted risk, resolved) — DEK lives as an immutable hex string

`KeyspecHex` (`internal/crypto/keywrap.go`) renders the raw DEK as a
64-char hex **string** for the SQLCipher `_pragma_key` DSN. Go strings are
immutable and cannot be zeroed, so that copy of the key persists on the
heap until GC, defeating `[]byte` wiping for that value.

**Resolution (2026-06-23).** Investigated and accepted with a narrower
scope. Two facts decided it:

- **A `[]byte` key path does not exist.** SQLCipher's `PRAGMA key` takes a
  string literal (`x'<hex>'`); SQLite cannot bind a PRAGMA value as a
  parameter, so the hex *string* is intrinsic regardless of driver. The
  `KeyspecHex` doc comment now records this so it is not re-chased.
- **The exposure a connection-hook would remove is already closed.** The
  DEK `[]byte` is zeroed at logout (ADR-001), and nothing in the codebase
  logs or prints the DSN (verified by grep), so the key never reaches a
  log or error string. Moving the key out of the DSN into a per-connection
  `PRAGMA key` hook would *not* shorten its in-memory lifetime (a pooled
  `*sql.DB` must retain it to re-key every new connection) and would add
  real risk to the at-rest encryption path for no measurable benefit.

**Action taken:** narrowed the hex string's scope at the one site where it
was held longer than needed — `MigratePlaintextToEncrypted` now derives
the keyspec immediately before use rather than across the pre-flight
checks. The DSN keying path is unchanged by design.

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

1. **F1** — done (AppImage delivery removed; no tool downloads remain).
2. **F2** — done (`govulncheck` is a blocking CI gate; Windows installer collection hardened).
3. **F3** — done (`errcheck`/`staticcheck`/`unused` re-enabled; first-party backlog cleared).
4. **F4 / F5 / F6** — hardening notes; no urgent action.

## Scope / limits

Static read only. Not run this pass: `govulncheck`/`gosec`/`staticcheck`
(F2 is exactly that gap), the Svelte frontend bridge surface beyond the
two analytics methods added this session, and any dynamic/runtime testing.
Re-run after F2 lands to confirm a clean dependency scan.
