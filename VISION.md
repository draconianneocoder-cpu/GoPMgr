<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# PMForge Vision

## Mission

PMForge is a local-first desktop application that gives individual project
managers and small teams professional-grade project controls without
requiring a cloud account, a subscription, or network connectivity.

Your projects live in encrypted files on your own machine. PMForge processes
them there and nowhere else.

## Design Principles

**Local first, always.**
Every feature that could be implemented locally must be implemented locally.
No capability may require an external server as a hard dependency. Cloud
connectivity, when it appears at all, must be strictly optional and
user-initiated.

**Security by default, not by configuration.**
New projects are encrypted. Authentication uses Argon2id. PDFs are
validated against PDF/A-3. Digital signatures follow PAdES. None of these
require the user to opt in.

**Professional tools, not enterprise complexity.**
PMForge targets project managers, engineers, and technical leads who need
CPM scheduling, EVM, risk registers, and document generation — not IT
departments managing a SaaS rollout. The UI must be learnable without a
training programme.

**The kernel is the product.**
CPM, EVM, baselines, resource levelling, and Monte Carlo simulation are the
core of PMForge's value. Every UI feature, export, and document must be
grounded in correct kernel behaviour. Kernel correctness takes precedence
over feature breadth.

**Stability over novelty.**
Dependencies must be actively maintained and, where possible, written in Go
or Rust. Archived, abandoned, or experimental libraries are not acceptable
in the production build. CGO is permitted only where a pure-Go alternative
does not exist with equivalent correctness and performance.

**Exports are first-class citizens.**
PDF/A-3, DOCX, XLSX, MSPDI, iCal, and ODT outputs must be correctly
formed, interoperable, and generated without a hosted service. Validation
gates (veraPDF for PDF/A, PAdES for signing) are required — not optional.

**Encryption at rest, honest security model.**
SQLCipher-encrypted project files protect against raw-disk theft.
`system.db` stores hashes and wrapped DEKs, never plaintext secrets. The
security model is documented, not implied. No security-by-obscurity.

## Anti-Features

The following will never be required features of PMForge:

- A cloud account or user registration to open, save, or export a project.
- Remote telemetry, usage analytics, or crash reporting sent without
  explicit user consent.
- A mandatory internet connection for any scheduling, EVM, document
  generation, or export capability.
- Proprietary binary formats that cannot be round-tripped through an open
  standard (MSPDI, PDF/A, DOCX/OOXML, ODT/ODF).
- Bundled update mechanisms that execute remote code without user approval.
- Lock-in via encrypted-only export: any project file the user created can
  be exported to an open, portable format.

## Target Users

**Primary:** Project managers in engineering, IT, construction, and
administrative roles managing projects of 20–500 tasks with budgets,
resources, and milestone accountability.

**Secondary:** Technical leads and architects who need baseline-vs-actual
tracking, EVM reporting, and exportable project artefacts for client
delivery.

**Out of scope (for now):** Multi-hundred-user enterprise PMO deployments,
real-time collaborative editing over a network, and portfolio management
across organisations.

## Relationship to the Roadmap

This vision document is stable. The roadmap (`ROADMAP.md`) describes
time-phased work. Individual implementation decisions are recorded in
Architecture Decision Records under `docs/design/`. The vision takes
precedence over the roadmap when they conflict.
