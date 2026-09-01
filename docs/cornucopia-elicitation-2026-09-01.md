<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# OWASP Cornucopia security review

GoPMgr is a local-first Wails desktop app: local account login, SQLCipher
encryption at rest, PAdES document signing, and an Ed25519-signed update
channel, with no cloud backend, public web surface, or in-product AI
features. Against the OWASP Cornucopia deck's 12 suits (158 cards), that
scopes 7 suits (91 cards) as relevant — **Data Validation & Encoding,
Authentication, Authorization, Cryptography, the general Cornucopia suit,
DevOps, and Frontend** — via the checklist in
[.claude/skills/cornucopia-risk-cards/](../.claude/skills/cornucopia-risk-cards/).
Session Management, Cloud, Automated Threats, and the LLM/Agentic AI suits
don't map to anything in GoPMgr's current architecture and are out of
scope; a remote backend, multi-tenant surface, or in-product AI feature
would change that and should trigger re-scoring.

This document records what a review against those 7 suits found. Last
reviewed: 2026-09-01.

## Data Validation & Encoding

The `os.Stat`+`io.LimitReader` bounded-read guard convention covers the
"validation/sanitization not applied everywhere" card cluster for every
user-file-pick site audited to date: `internal/crypto/pdf_sign.go`'s
`LoadCertificate`, `internal/fonts/manager.go`'s `ImportFont`/`RegisterAs`,
and the cost-control attachment path (`readBoundedFile`/
`readBoundedReader`). See `docs/beta-release-backlog.md`'s PKCS#12/TrueType
and cost-control-attachment rows for the guards' fault-seed evidence.

Injection-style cards (client/server-side interpreter injection) haven't
been independently audited against GoPMgr's SQL and PDF-generation code
paths beyond a read-through; no gap is currently known there, but that
isn't asserted as verified.

## Authentication

[`Login`](../app_session.go) collapses "no such user" and "wrong password"
into one identical `"invalid credentials"` error, closing username
enumeration via distinguishable error messages as a checked property.

Brute-force/lockout behavior and re-authentication requirements for
critical functions haven't been independently audited this review.

## Authorization

[`projectPathFor`](../app_projects.go) rejects any path whose parent
directory isn't the signed-in user's own `projects/` directory (or an
immediate subfolder of it) — the primary access-control boundary for
project-file access can't be bypassed through another path.

Business-rule bypass and function/property-level authorization beyond the
file-path boundary haven't been independently audited this review.

## Cryptography

[`internal/auth/password.go`](../internal/auth/password.go) uses
`golang.org/x/crypto/argon2`, not a homebrew hash, for password hashing.
`DownloadAndInstall` (`internal/update/install.go`) re-runs `CheckLatest`
and never accepts a URL or hash from its caller, so the Ed25519 signature
over the update manifest is re-verified on every call rather than cached
or trusted from a prior check.

The DEK-wrapping hierarchy (`internal/db`) and PAdES signing
(`internal/signing`) haven't been independently audited against this
suit's cards this review.

## Cornucopia (general)

[`VerifyAuditChain`](../internal/db/audit.go) recomputes every
`event_hash` and checks sequence continuity, so project-lifecycle actions
can't be silently altered without detection.

Error-handling consistency and denial-of-service resistance haven't been
independently audited this review.

## DevOps

Every third-party GitHub Action referenced in `.github/workflows/ci.yml`
and `.github/workflows/release.yml` was pinned to a mutable version tag
(`@v7`/`@v8`/`@v9`) rather than an immutable commit SHA — a tag can be
repointed by the upstream maintainer or, in a compromise, by an attacker
with write access to the action's repository, the mechanism the 2025
`tj-actions/changed-files` incident used. Every `uses:` line now pins the
resolved commit SHA with the original tag kept as a trailing comment for
readability.

Artifact-integrity-in-transit is covered under Cryptography above (the
Ed25519 re-verify on every update-install call).

## Frontend

GoPMgr's frontend has no `{@html}` block, `innerHTML`/`outerHTML`
assignment, or `eval`/`new Function` call anywhere in `frontend/src`.
Svelte's default `{expression}` interpolation is text-escaped, so there is
no DOM-XSS sink for user-controlled or imported data to reach.

Third-party-script/CDN compromise and clickjacking don't apply to a
Wails-rendered local webview with no remote script loading.

## Related repository changes

- `.gitignore` narrows `.claude/`'s exclusion to `/.claude/*` plus
  `!/.claude/skills/`, so the Cornucopia reference skill is tracked while
  local settings, worktrees, and other machine-specific state stay
  ignored.
- `.claude/skills/cornucopia-risk-cards/` packages the 91 relevant cards,
  keyed by a path→suit lookup table, as a project skill consulted when
  building a risk/requirement matrix for GoPMgr code.
