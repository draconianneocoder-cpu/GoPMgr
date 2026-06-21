<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Style

PMForge favors conservative, explicit, locally testable code. Match the
existing package patterns and keep edits scoped to the behavior being
changed.

## Repository Conventions

- Every tracked file needs SPDX metadata or a `REUSE.toml` annotation.
- Go files use `gofmt`; shell scripts use `set -eu` when practical.
- Keep public release claims tied to gates that actually verify them.
- Avoid unrelated refactors in feature or fix commits.
- Prefer explicit file and hunk staging in a dirty worktree.

## Go

- Keep domain logic in `internal/...` packages and Wails orchestration in
  the root `main.go` (the main package must live at the repo root for
  `wails build`).
- Return contextual errors with `%w` where callers can use wrapping.
- Keep database writes transactional when multiple records or files must
  change together.
- Use existing DB helpers, repair patterns, and migration style before
  adding new abstractions.
- Do not bypass `internal/sqlitedriver` for SQLite handles. It centralizes
  the SQLCipher-capable driver.
- Prefer deterministic order in exported data and tests.

## Frontend

- Use Svelte 5 and TypeScript patterns already present in `frontend/src`.
- Keep desktop workflows dense, clear, and operational. Avoid
  marketing-page composition for app screens.
- Keep Wails bridge types in `frontend/src/wails-window.d.ts` synchronized
  with exported backend methods.
- Put Svelte runes only in Svelte-aware files: `.svelte`, `.svelte.ts`,
  or `.svelte.js`.
- Validate important UI changes with runtime smoke or browser testing, not
  only type checks.

## Documentation

- Keep docs factual and current with code. If a feature is only designed,
  say so. If it is gated, name the gate.
- Prefer commands that exist in `Makefile` or scripts.
- Do not duplicate long generated outputs. Summarize evidence and point to
  the command.
- Root project docs use `GFDL-1.3-or-later` SPDX metadata.

## Security-Sensitive Code

- Treat authentication, recovery, encryption, signing, database repair,
  and export validation as security-sensitive.
- Add regression tests before changing key wrapping, recovery-code reset,
  SQLCipher open/migration logic, PDF signing, or release gates.
- Zero sensitive in-memory material where the surrounding code already
  does so, and avoid logging secrets, raw keys, passwords, recovery codes,
  certificate private keys, or SQLCipher key material.
