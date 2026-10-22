<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Go 1.27.3 security baseline

This upgrade starts from `9c84367269374527174e2a36024e9d6b7ed734d1`.
The previous [CI vulnerability job](https://github.com/draconianneocoder-cpu/GoPMgr/actions/runs/39851646095)
reported GO-2026-6809 in Go 1.27.2 through RFC 3161 trust verification.
The root module and isolated SQLCipher compatibility helper now require
Go 1.27.3. CI/release jobs already select Go from `go.mod`. No application
logic, dependency versions, public interfaces or storage formats changed.
See [the security baseline](../DEPENDENCIES.md#go-security-baseline-2026-10-22)
for the trust-policy consequence and rebuild requirement.

## Verification, 2026-10-22

Local macOS 27.0.1, arm64, Go 1.27.3, CGO enabled, Wails 2.16.0,
Node 24.18.0 and npm 12.0.2. Native veraPDF reports version 1.30.0;
the pinned Docker validator was not run. Go/frontend lint used installed
golangci-lint 2.15.0 and the repository's ESLint configuration, not hosted CI.

These commands passed with `GOTOOLCHAIN=local GOFLAGS=-mod=readonly`:

```sh
make verify
make lint-all
make coverage-ledger-drift
go mod verify
go test -count=1 ./internal/rfc3161 ./internal/signing ./internal/sqlitedriver
go test -count=1 -tags duckdb ./internal/analytics/...
govulncheck -tags webkit2_41 ./...
govulncheck -tags duckdb,webkit2_41 ./internal/analytics/...
govulncheck -tags desktop,production,duckdb,webkit2_41,wv2runtime.download ./...
GOPMGR_VERAPDF_FORCE_CLI=1 make check-release
make check-pades-external
```

`make verify` passed 637 frontend tests across 81 files, warning-clean
Svelte checks, Go tests and build/source contracts. All 46 coverage headings
matched. The full release gate passed all 44 uncached race packages using
`-race -p=1 -count=1`, the native production build with DuckDB, encrypted
storage, strict PDF/A-3 validation of four samples and PAdES harnesses.
An earlier full run stopped because Docker was installed but its daemon was
unavailable. The documented native override retained strict validation;
the complete aggregate was rerun successfully, without source changes.

An actual Go 1.27.2 executable with `GOTOOLCHAIN=local` refused `go list -m`
because the module requires Go 1.27.3. Automatic upgrading was disabled for
this negative control. Existing tests accepted a configured valid TSA root,
rejected an empty trust pool and returned no signed output after timestamp
failure. They do not directly exercise a compromised intermediate fixture.

`go version -m` on the rebuilt executable reported Go 1.27.3 and the
`desktop,duckdb,webkit2_41,wv2runtime.download,production` tags. Strict
ad-hoc code-signature verification passed. This was a local working-tree
build, not a published or publisher-trusted artifact. The module graph was
unchanged; neither `go.sum` nor the frontend lockfile changed.

## Findings and evidence limits

- Source scans used govulncheck 1.9.0 and found no affected symbols or
  imported-package advisories. Default and production scans retained one
  required-module advisory; the DuckDB analytics scan found none.
- The stripped production binary (`-w -s`) scan exited 3 for the existing
  GO-2026-5932 OpenPGP advisory. The exact production import graph has no
  OpenPGP packages. The scanner documents a whole-module fallback when Go
  symbol information cannot be extracted; its implementation emits the
  reported wildcard package symbols in that case. This is not a clean binary
  scan or evidence that OpenPGP is linked. See the
  [scanner limitations](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck#hdr-Limitations).
- Optional gosec 2.29.0 could not decode Go 1.27 export data and reported four
  medium warnings in unchanged printing, restore and signature-export code.
  It is not clean security evidence. Standalone staticcheck was absent;
  golangci-lint ran its embedded checks. No scanner policy was weakened.
- jsdom emitted the existing canvas diagnostics during passing tests. The
  external PAdES fixture remained self-signed: signature integrity and
  PAdES-BASELINE-T classification passed, not public certificate-chain trust.
- The native build emitted Wails `time.Time` binding diagnostics and a warning
  about a macOS 13 object linked with an 11.0 target. Build and signature
  checks passed, no generated bindings changed, and the documented product
  requirement remains macOS 13 or later; older macOS support is not implied.
- No new hosted CI run, Windows/Linux native build, installer lifecycle,
  trusted-source signing verification or publication was performed.

The key lesson is to record the effective compiler: local Go 1.27.3 results
could not clear a CI baseline still selecting Go 1.27.2. Existing executables
remain unchanged until rebuilt and replaced; there is no data migration.
