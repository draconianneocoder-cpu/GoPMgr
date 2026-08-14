<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Dependencies

GoPMgr is a CGO-enabled Go and Svelte desktop application. Dependency
changes affect build reproducibility, release packaging, security
posture, and validator coverage, so keep them intentional and verified.

## Toolchain

- Go: 1.26.6 from `go.mod`.
- Wails: v2.13.0.
- Node frontend: Node.js 26 in CI/release workflows, with Vite 8, Svelte 5,
  TypeScript 6, and npm scripts in `frontend/package.json`. Browser storage
  code uses `window.localStorage`, and Vitest workers disable Node's
  process-wide Web Storage so jsdom remains the authoritative per-test browser
  environment.
- CGO: required for the SQLite/SQLCipher driver path and the shipped DuckDB
  analytics build.

## Core Go Dependencies

- `github.com/wailsapp/wails/v2`: Desktop app runtime and Go/JS bridge.
- `github.com/mutecomm/go-sqlcipher/v4`: SQLCipher-capable SQLite driver
  registered through `internal/sqlitedriver`.
- `golang.org/x/crypto`: Argon2id and related cryptographic support.
- `github.com/digitorus/pkcs7`: CMS/PKCS#7 parsing and OID support for
  PAdES-related code.
- `github.com/digitorus/timestamp`: BSD-2-Clause RFC 3161 request, token, and
  test-fixture primitives. `internal/rfc3161` adds GoPMgr's HTTPS, response
  binding, TSA certificate, size, redirect, and optional trust-root policy.
- `github.com/go-pdf/fpdf`: PDF generation (community-maintained continuation
  of the archived jung-kurt/gofpdf; see ADR-003).
- `github.com/gomutex/godocx`: DOCX generation.
- `github.com/xuri/excelize/v2`: XLSX generation.
- `github.com/rickar/cal/v2`: Country holiday calendars.
- `github.com/gorules/zen-go`: JDM launchpad template-seeding rules.
- `gonum.org/v1/gonum`: Numerical/statistical support.
- `gopkg.in/yaml.v3` and `github.com/pelletier/go-toml/v2`: build-time
  configuration parsing for `make config-check`. They are imported only by
  the `scripts` command and are not linked into the GoPMgr desktop binary.
- `github.com/duckdb/duckdb-go/v2`: in-memory DuckDB analytics engine
  (ADR-002 Option B), compiled under the `duckdb` build tag
  (`internal/analytics`). Production/package builds enable that tag so
  installers include the analytics engine; untagged developer builds link the
  stub. See `docs/design/duckdb-analytics-engine.md`.

Check `go.mod` for the authoritative version list.

## Frontend Dependencies

Runtime:

- `chart.js`: Chart rendering in the frontend.
- `read-excel-file`: `.xlsx` parsing for the Sigma data import (replaced
  the dead-ended SheetJS `xlsx`; see ADR-002 file-import notes).

Development:

- Svelte 5, Vite 8, TypeScript 6, svelte-check, ESLint 10,
  eslint-plugin-svelte 3, Tailwind CSS 4, PostCSS, and
  `@tailwindcss/postcss`.

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
- `wails`: development server and desktop packaging workflow; also builds
  the Windows NSIS installer (`wails build -nsis`).
- `nfpm` v2.47.0: builds the Linux `.deb` and `.rpm` packages from
  `build/linux/nfpm.yaml`.
- `hdiutil`: the built-in macOS release path for the `.dmg`. `create-dmg`
  remains an optional local-only presentation path when
  `GOPMGR_FANCY_DMG=1`; the tag workflow does not install or enable it.
- NSIS (`makensis`) 3.12.0: the Chocolatey-pinned toolchain behind
  `wails build -nsis` on Windows.
- `ripgrep` (`rg`): required by release-scope and packaging source contracts;
  the tag preflight installs and discovers it explicitly instead of depending
  on the current hosted-runner image.

`scripts/release-tool-versions.env` is the version record for installer tools
that are fetched by the tag workflow. `make installer-tool-pins` verifies the
workflow, the record, and local Linux packaging guidance as one contract.
GoPMgr tracks `build/windows/installer/project.nsi`, `info.json`, and the
application manifest. The pinned Wails CLI regenerates `wails_tools.nsh`, the
WebView2 bootstrapper, and platform icon during native builds; those derived
files remain ignored.

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
