<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# GoPMgr

GoPMgr is a local-first desktop application for project controls. It combines
a Go backend, Wails v2 desktop shell, and Svelte 5 frontend to provide
planning, scheduling, reporting, and export workflows without requiring a
hosted service.

Published release tags are recorded in
[docs/published-release-tags.txt](docs/published-release-tags.txt). Install
instructions and platform limitations are in [docs/INSTALL.md](docs/INSTALL.md).

## Capabilities

- CPM scheduling, baselines, earned-value calculations, resource leveling, and
  schedule-risk analysis.
- Project charts, documents, combined reports, and open-format exports.
- Local accounts, SQLCipher-encrypted per-user `.gopmgr` project databases,
  recovery codes, and tamper-evident audit records.
- PDF/A-3 export and optional PAdES digital signing.

See [VISION.md](VISION.md) for product principles and [ROADMAP.md](ROADMAP.md)
for planned work.

## Build and verify

Prerequisites are Go 1.27.0, Node.js, npm, CGO, and the matching Wails CLI.
Wails: the project uses Wails v2.15.0. Install that exact CLI version with:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
```

From a checkout:

```sh
go mod tidy
npm --prefix frontend ci
wails dev
make build
make verify
```

`make verify` is the regular local gate. Before a release, run:

```sh
make check-release
```

It verifies configuration, tool pins, licenses (when the `reuse` CLI is
installed — it skips rather than fails if not), frontend checks, release
claims, memory safety, race tests, production build inputs, encrypted database
behavior, PDF/A, and PAdES regression harnesses. See [TESTING.md](TESTING.md)
for focused commands and [docs/release-preflight.md](docs/release-preflight.md)
for the release checklist.

## Data and security

GoPMgr stores application data in its platform-selected private data directory.
It contains `system.db`, a per-user project directory, certificates, exports,
and diagnostic logs. The app migrates supported legacy data locations on first
launch; do not move or delete application data while an upgrade is in progress.

New per-user `.gopmgr` project databases are SQLCipher-encrypted with a
per-user data-encryption key. `system.db` contains login and wrapped-key
metadata, not project records. If the password and all valid recovery codes
are lost, encrypted projects cannot be recovered.

Use whole-device encryption as well: FileVault on macOS, BitLocker on Windows,
or LUKS on Linux. See [SECURITY.md](SECURITY.md) and
[ADR-001](docs/design/ADR-001-database-encryption-at-rest.md) for the threat
model and migration rules.

## PDF and signing

PDF/A metadata is applied before PAdES, because a signature must be the final
PDF mutation. `make check-pades` verifies deterministic local signature
structure. `make check-pades-external` adds installed validators; DSS
classifies the self-signed fixture as `PAdES-BASELINE-T`. That fixture does not
prove a trusted public certificate chain. Use
`make check-pades-trusted` with a separately supplied trusted sample before
making trust claims.

## Documentation

- [ARCHITECTURE.md](ARCHITECTURE.md): runtime and package boundaries.
- [DEVELOPER_HANDBOOK.md](DEVELOPER_HANDBOOK.md): development conventions and
  extension guidance.
- [TESTING.md](TESTING.md): test and release gates.
- [DEPENDENCIES.md](DEPENDENCIES.md): dependency and tool policy.
- [docs/user-guide.md](docs/user-guide.md): user workflows.
- [docs/branding.md](docs/branding.md): Bobby Beaver identity, asset placement, platform behavior, and attribution.
- [docs/INSTALL.md](docs/INSTALL.md): installation and source-run guidance.
- [docs/beta-release-backlog.md](docs/beta-release-backlog.md): release
  readiness work.
- [AGENTS.md](AGENTS.md): repository instructions for automated contributors.

## License

Source code is licensed under GPL-3.0-or-later. Documentation is generally
licensed under GFDL-1.3-or-later. Each file declares its authoritative SPDX
license identifier; GoPMgr-owned Bobby Beaver artwork is GPL-3.0-or-later
with attribution recorded in [docs/branding.md](docs/branding.md). Run `make
license-check` to verify REUSE compliance. See [LICENSE.md](LICENSE.md) and
[LICENSES.md](LICENSES.md) for details.
