<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# GoPMgr Roadmap

This roadmap describes intended product direction, not a release promise.
Current release-readiness work is tracked in
[docs/beta-release-backlog.md](docs/beta-release-backlog.md). Decisions that
change architecture belong in [docs/design/](docs/design/).

## Current focus

GoPMgr already provides local project storage, scheduling, earned-value
analysis, charts, documents, export, SQLCipher support, and PDF signing. The
near-term priority is making those workflows dependable for wider testing.

1. Prevent data loss in stateful editors and verify save, discard, and cancel
   behavior in native application workflows.
2. Complete real-target installation and upgrade evidence for every advertised
   package format.
3. Establish trustworthy distribution: code signing, notarization where
   applicable, and release-certificate PAdES evidence. The Ed25519 signed
   update-manifest mechanism itself (`internal/update`, `tools/update-manifest`)
   is already built and tested end-to-end; what remains is standing up a live
   channel — generating and provisioning the signing keypair as CI secrets so
   `ManifestURL`/`UpdateChannelPublicKey` are actually populated in release
   builds, which today ship with the update channel disabled by design.
4. Improve keyboard and assistive-technology coverage before broad release.

## Planned product work

### Scheduling and controls

- Deeper cost forecasting and formal rebaselining workflows.
- Additional contract, procurement, and stakeholder controls.
- Continued validation of resource leveling and schedule-risk behavior against
  real project data.

### Usability and scale

- Accessible, keyboard-complete workflows.
- Large-project Gantt performance and rendering improvements.
- Internationalization after core interaction patterns are stable.

### Interoperability

- Broader project-data import and export compatibility.
- Optional local collaboration only when file-locking and conflict behavior are
  explicit and testable.
- Community extensions only after a narrow, permissioned application boundary
  has proven safe in production use.

## Guardrails

- Local use must not require a cloud account or mandatory network connection.
- New dependencies need a documented maintenance, license, security, and
  packaging review.
- Project data and export compatibility take precedence over feature breadth.
- Release claims require current, reproducible evidence.

## Implemented architecture decisions

| ADR | Decision | Status |
| --- | --- | --- |
| [ADR-001](docs/design/ADR-001-database-encryption-at-rest.md) | Per-user database encryption at rest | Implemented |
| [ADR-002](docs/design/ADR-002-duckdb-vs-sqlcipher-evaluation.md) | DuckDB analytics and SQLCipher project storage | Implemented |
| [ADR-003](docs/design/ADR-003-gofpdf-to-go-pdf-fpdf-migration.md) | PDF library migration | Implemented |

## Not planned

GoPMgr will not require hosted accounts, mandatory telemetry, or online access
for core project management and export workflows. Real-time collaborative
editing remains out of scope until the project adopts a conflict model that
protects local project data.
