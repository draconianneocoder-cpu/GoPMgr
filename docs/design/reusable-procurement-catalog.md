<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Reusable procurement catalog

**Status:** Approved product boundary; catalog foundation delivered.

## Purpose

The Cost Control ledger records project budget inputs, commitments, actual
expenses, materials, and overhead. It must not require a project manager to
re-enter a recurring supplier or material for every project.

The reusable procurement catalog is a convenience library for the signed-in
local user. It stores supplier and item information independently from any
single project, while posted project ledger evidence remains portable and
self-contained.

## Delivered foundation

`internal/catalog` stores two encrypted, user-private record types in
`<user-data-dir>/catalog.gopmgr`:

| Record | Stored fields |
| --- | --- |
| Supplier | name, address, phone, fax, email, primary contact, notes, archive state |
| Item | name, SKU, kind, default unit, description, archive state |

The catalog database uses a SQLCipher key deterministically derived from the
signed-in session DEK with a catalog-only context. It is opened only after
login. `system.db` remains limited to bootstrap/account data and never stores
supplier contact information.

Records are server-generated, versioned, search-bounded, and archived rather
than hard-deleted. A stale edit is rejected instead of silently replacing a
newer edit. The **Suppliers & items** dashboard entry point exposes the
catalog to the user.

## Required ledger integration, not yet delivered

The catalog foundation does not yet change `cost_entries`. The next ledger
slice must add additive, project-local fields for:

- exact decimal quantity and canonical unit, where both are present or both
  absent;
- independent optional supplier and item catalog identifiers;
- supplier display name, item name, SKU, unit, catalog version snapshots; and
- a bounded invoice reference.

The existing exact money amount remains the authoritative line total. Quantity
does not calculate or overwrite money, and no unit price is introduced by this
scope. Quantity aggregation is permitted only for the same item identity and
identical canonical unit; GoPMgr does not convert kilograms to pounds, hours
to days, or other units implicitly.

Catalog changes affect future selection only. Project snapshots must never be
silently refreshed, and supplier address/contact PII must not be copied into a
project entry, audit event, routine report, log, or support bundle.

## Attachment and export contract

Attachments are a separate project-local slice. They will use bounded
SQLCipher BLOBs inside the project database, not external plaintext or
application-encrypted sidecar files. This keeps content in the existing
project encryption, recovery, and backup lifecycle.

The ledger report/CSV will show attachment metadata only. A separate explicit,
user-selected, no-overwrite ZIP bundle will export original attachment files
plus a manifest. The bundle must contain no absolute source paths, and every
exported attachment must have its stored byte count and SHA-256 verified.

## Non-goals

This is not a supplier payment system, purchasing workflow, stock inventory,
tax engine, currency conversion system, portfolio accounting store, or
financial authority model. It does not change Cost Control Phase 2.
