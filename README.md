<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# GoPMgr

GoPMgr is a local-first desktop application for project controls. It combines
a Go backend, Wails v3 desktop shell, and Svelte 5 frontend to provide
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
- PDF/A-2u archival export and optional PAdES digital signing.

See [VISION.md](docs/VISION.md) for product principles and [ROADMAP.md](docs/ROADMAP.md)
for planned work.

## Build and verify

Prerequisites are Go 1.27.4 or later, Node.js, npm, Python 3, CGO, and the matching Wails CLI.
Wails: the project uses Wails v3.1.0. Install that exact CLI version with:

```sh
mkdir -p .tmp/wails-v3/bin
GOBIN="$PWD/.tmp/wails-v3/bin" go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
```

Linux source builds require GTK 4 and WebKitGTK 6.0 development packages
(`libgtk-4-dev`, `libwebkitgtk-6.0-dev`, and `pkg-config` on Debian/Ubuntu).
The Go Wails module, task-local CLI, and npm `@wailsio/runtime` are pinned to 3.1.0.
Optional auxiliary-tool provisioning requires Python 3.11+; see the
[ADR-013 setup and Make-only recovery guide](docs/engineering/adr-013-provisioning.md).

The Wails v3 migration was approved on 2026-11-03 and merged on 2026-11-09.
Frontend check/build and tests pass against the generated App bindings; native macOS bridge
qualification covers exact money, encrypted persistence, null/empty lists, and
cleanup. This is development evidence, not completed package or release
qualification. macOS 26 Tahoe and Ubuntu 26.04 execution remain mandatory release
gates; the macOS 26 host is currently unavailable, and Ubuntu 26.04 is the Linux
minimum since 2026-11-09 (see the [Ubuntu 26.04 decision](docs/review/LINUX-MINIMUM-UBUNTU-26.04-OWNER-DECISION.md)). Windows NSIS
qualification is pending. See [the migration design](docs/design/wails-v3-migration-design.md).
Run verification locally; workflow edits do not authorize hosted CI.

From a checkout:

```sh
go mod tidy
npm --prefix frontend ci
make dev
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
behavior, PDF/A, and PAdES regression harnesses. See [TESTING.md](docs/TESTING.md)
for focused commands and [docs/release-preflight.md](docs/release-preflight.md)
for the release checklist. The version-neutral plan for the next limited alpha
is [docs/next-alpha-release-plan.md](docs/next-alpha-release-plan.md).

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
or LUKS on Linux. See [SECURITY.md](docs/SECURITY.md) and
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

- [ARCHITECTURE.md](docs/ARCHITECTURE.md): runtime and package boundaries.
- [DEVELOPER_HANDBOOK.md](docs/DEVELOPER_HANDBOOK.md): development conventions and
  extension guidance.
- [TESTING.md](docs/TESTING.md): test and release gates.
- [DEPENDENCIES.md](docs/DEPENDENCIES.md): dependency and tool policy.
- [docs/SBOM.md](docs/SBOM.md): the SBOMs published with each release, how to
  verify them, and how they meet CISA's 2026 minimum elements.
- [docs/user-guide.md](docs/user-guide.md): user workflows.
- [docs/branding.md](docs/branding.md): Bobby Beaver identity, asset placement, platform behavior, and attribution.
- [docs/INSTALL.md](docs/INSTALL.md): installation and source-run guidance.
- [docs/beta-release-backlog.md](docs/beta-release-backlog.md): release
  readiness work.
- [docs/next-alpha-release-plan.md](docs/next-alpha-release-plan.md): go/no-go
  decisions, evidence matrix, publication sequence, and rollback conditions for
  the next alpha.
- [AGENTS.md](AGENTS.md): repository instructions for automated contributors.

## License

Source code is licensed under GPL-3.0-or-later. Documentation is generally
licensed under GFDL-1.3-or-later. Each file declares its authoritative SPDX
license identifier; GoPMgr-owned Bobby Beaver artwork is GPL-3.0-or-later
with attribution recorded in [docs/branding.md](docs/branding.md). Run `make
license-check` to verify REUSE compliance. See [LICENSE.md](LICENSE.md) and
[LICENSES.md](LICENSES.md) for details.
