<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Reusable procurement catalog

**Status:** Approved product boundary; catalog foundation and project-ledger
procurement detail delivered. Catalog-assisted ledger selection remains pending.

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

## Delivered project-ledger detail

`cost_entries` has additive, project-local display-snapshot fields for:

- an optional exact three-decimal quantity and unit, with quantity requiring a
  unit;
- item name, SKU, supplier display name, and a bounded invoice reference; and
- bounded encrypted file attachments per entry.

The existing exact money amount remains the authoritative line total. Quantity
does not calculate or overwrite money, and no unit price is introduced by this
scope. Quantity aggregation is permitted only for the same item identity and
identical canonical unit; GoPMgr does not convert kilograms to pounds, hours
to days, or other units implicitly.

The ledger searches description, item, SKU, supplier, and invoice-reference
text. Its printable financial report includes procurement detail and same-item,
same-unit quantity aggregates. These records are independent project snapshots:
they have no live cross-database foreign key to the catalog and remain readable
when the catalog is unavailable.

## Remaining catalog connection

The Cost Control form currently accepts the delivered procurement detail as
plain text. Selecting a reusable catalog supplier or item from that form,
autofilling its fields, and retaining a catalog ID or catalog version are not
yet delivered. If this connection is added, it must still save only the
project-local display snapshot, never make a historical ledger row depend on a
live catalog record.

Catalog changes never alter posted ledger entries. Supplier address/contact PII
must not be copied into a project entry, audit event, routine financial report,
log, or support bundle.

## Attachment and export contract

Attachments are a project-local feature. They use bounded
SQLCipher BLOBs inside the project database, not external plaintext or
application-encrypted sidecar files. This keeps content in the existing
project encryption, recovery, and backup lifecycle.

The ledger detail UI exposes attachment metadata, while the printable financial
report never embeds attachment bytes. A separate explicit, user-selected,
no-overwrite ZIP bundle exports original attachment files plus a manifest. The
bundle contains no absolute source paths, and every exported attachment has its
stored byte count and SHA-256 verified immediately before its archive entry is
created. This is a consistency check against stored metadata, not a claim that
a privileged local attacker cannot alter both a project database and its hash.

## Non-goals

This is not a supplier payment system, purchasing workflow, stock inventory,
tax engine, currency conversion system, portfolio accounting store, or
financial authority model. It does not change Cost Control Phase 2.
