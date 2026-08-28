<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Project cost ledger scope

**Status:** Current Phase 1 product boundary.

## Purpose

The Cost Control ledger is a practical, **project-local** record of expected
inputs, commitments, actual expenses, and overhead. It appears alongside, but
does not import, the separate legacy Project Budget. It helps a project manager
understand what the Cost Control ledger plans to spend, has committed to spend,
and has spent in each category.

The ledger is not a portfolio accounting system, a foreign-exchange system, or
an enterprise financial-governance product.

## Required Phase 1 behavior

For one project and its reporting currency, the ledger must let the user:

1. record planned, committed, and actual cost amounts exactly;
2. classify an entry by its cost type, including direct/indirect attribution,
   fixed/variable behavior, and CapEx/OpEx treatment where applicable;
3. record materials and other inputs as ordinary cost entries, with an
   optional structured quantity, unit, supplier, item, SKU, and
   invoice-reference record (delivered 2026-08-24; a project-local snapshot
   taken at save time, never a live reference into the separate catalog
   database; quantity requires a unit), plus optional bounded, encrypted file
   attachments per entry;
4. search the ledger by item, SKU, supplier, invoice reference, or
   description, and view the same-item/same-unit quantity aggregated across
   entries;
5. make indirect or material overhead visible through an ordinary cost type or
   cost entry rather than a hidden adjustment; and
6. review project-scoped planned, committed, actual, and classification
   summaries, and export the project financial snapshot, classified ledger
   rows, and every ledger attachment (as a single no-overwrite ZIP with a
   metadata manifest; each fetched attachment is checked against its stored
   byte count and SHA-256 immediately before archiving) without changing the
   source entries.

Amounts remain exact minor-unit values in Go and canonical decimal strings at
the Wails boundary. The ledger records the project's chosen reporting currency;
it does not convert it.

## Scope guard for developers and advisors

Do not introduce any of the following merely to add, classify, or report a
ledger entry:

- portfolio or organization persistence;
- FX rates, currency conversion, rate provenance, or cross-project totals;
- financial roles, approval workflows, or four-eyes controls;
- transaction, tax, statutory-accounting, or payment semantics;
- forecast/EAC, reserve drawdown, allocation, or remaining-spend behavior; or
- a separate audit system beyond the project's existing audit chain.

These may become separate, explicitly approved features when a demonstrated
project need requires them. They are not prerequisites for a useful ledger.

## Boundary with deferred work

The existing Phase 1 ledger lifecycle and its current baseline/reserve display
remain intact. [Cost Control Phase 2](cost-control-phase-2-lifecycle.md) is a
separate proposed contract for reserve movements, authority, and forecasting;
it must not redefine ordinary ledger entry tracking. The proposed
[presentation-only Portfolio FX contract](governed-presentation-only-fx.md) is
also separate and must not add FX behavior or governance to the project ledger.

When a request concerns budget, expenses, inputs, materials, or overhead,
extend this project-local model first. Reusable supplier and item selection
(full spec: [reusable procurement catalog](reusable-procurement-catalog.md))
is an approved convenience boundary, not portfolio finance: the Cost Control form
queries active catalog records and copies an item name/SKU/default unit or a
supplier display name into editable ledger fields. A posted project row retains
only its own display snapshot, no catalog ID or version, and remains readable
if the catalog is unavailable. Automatic catalog copying excludes supplier
addresses and contact details from project audit payloads and ordinary
financial exports; user-entered free text is outside that automatic-copying
guarantee. Escalate only when the request explicitly needs one of the deferred
capabilities above.
