<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Dependencies

PMForge is a CGO-enabled Go and Svelte desktop application. Dependency
changes affect build reproducibility, release packaging, security
posture, and validator coverage, so keep them intentional and verified.

## Toolchain

- Go: 1.26.4 from `go.mod`.
- Wails: v2.9.2.
- Node frontend: Vite, Svelte 5, TypeScript, and npm scripts in
  `frontend/package.json`.
- CGO: required for the SQLite/SQLCipher driver path.

## Core Go Dependencies

- `github.com/wailsapp/wails/v2`: Desktop app runtime and Go/JS bridge.
- `github.com/mutecomm/go-sqlcipher/v4`: SQLCipher-capable SQLite driver
  registered through `internal/sqlitedriver`.
- `golang.org/x/crypto`: Argon2id and related cryptographic support.
- `github.com/digitorus/pkcs7`: CMS/PKCS#7 parsing and OID support for
  PAdES-related code.
- `github.com/jung-kurt/gofpdf`: PDF generation.
- `github.com/gomutex/godocx`: DOCX generation.
- `github.com/xuri/excelize/v2`: XLSX generation.
- `github.com/rickar/cal/v2`: Country holiday calendars.
- `github.com/gorules/zen-go`: JDM launchpad template-seeding rules.
- `gonum.org/v1/gonum`: Numerical/statistical support.

Check `go.mod` for the authoritative version list.

## Frontend Dependencies

Runtime:

- `chart.js`: Chart rendering in the frontend.
- `xlsx`: Spreadsheet handling on the frontend side.

Development:

- Svelte 5, Vite, TypeScript, svelte-check, ESLint, Tailwind CSS,
  PostCSS, and Autoprefixer.

Check `frontend/package.json` for the authoritative version list.

## External Tools

Some gates use optional or required tools outside Go/npm:

- `reuse`: REUSE/SPDX license checks.
- `veraPDF` or Docker with the veraPDF image: strict PDF/A-3 validation.
- `qpdf`: PDF syntax validation in external PAdES checks.
- `pdfsig`: Poppler signature validation in external PAdES checks.
- `openssl`: CMS ASN.1 and detached signature verification.
- `dss-validation-tool`: DSS PAdES baseline classification when
  installed.
- `wails`: development server and desktop packaging workflow.

`make check-release` is strict where release correctness requires proof.
If a required validator is missing, install the tool rather than
weakening the release claim.

## Dependency Change Rules

1. Read the current code path before adding a dependency.
2. Prefer standard library or existing dependencies when they are
   adequate.
3. Avoid dependencies that duplicate existing project abstractions.
4. For security-sensitive dependencies, inspect maintenance status,
   licenses, native build requirements, and release packaging impact.
5. Run `go mod tidy` or npm install only when dependency metadata should
   actually change.
6. Verify with focused tests plus the relevant release gates.

For SQLCipher specifically, remember that the selected driver owns the
SQLite implementation in the binary. Driver changes need encryption
tests, migration tests, build checks, and packaging review.
