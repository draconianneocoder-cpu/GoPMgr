+++
title = "Roadmap"
description = "Direction and evidence gaps, without release promises."
weight = 60
layout = "wide"
[params]
status = "planned"
applies_to = "Application snapshot 88b92cf935c7; planning review 8aa1944f67cf"
last_reviewed = "2026-11-13"
claim_ids = ["page.roadmap"]
[[params.sources]]
label = "docs/ROADMAP.md"
url = "https://github.com/draconianneocoder-cpu/GoPMgr/blob/8aa1944f67cfba57f2eabb2d73d2619fb21279e9/docs/ROADMAP.md"
[[params.sources]]
label = "docs/review/ADR-012-DUCKDB-RETIREMENT-OWNER-DECISION.md"
url = "https://github.com/draconianneocoder-cpu/GoPMgr/blob/88b92cf935c77332cec4357789250126a5e6b314/docs/review/ADR-012-DUCKDB-RETIREMENT-OWNER-DECISION.md"
+++

<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

## Development source

Local project storage, scheduling, earned-value analysis, reports and exports exist in the reviewed source. See [capabilities](../features/) for what each one covers. None is a guarantee of support on your platform.

## Current priorities

The canonical roadmap prioritizes dependable save/recovery workflows, native installation and upgrade evidence, trustworthy distribution, and keyboard/assistive-technology coverage.

## Reporting and extension plans

The proposed work starts with reporting definitions and truthful findings, then consistent snapshots and core reports on qualified inputs. Immutable issuance and connected records require their own decisions. Executable extensions have separate security and qualification gates; host-native reports need not await Wasm. These are plans, not implemented or qualified capabilities. See the canonical roadmap for the complete dependency sequence.

## Planned, not measured

High-scale scheduling is a direction, not an achieved performance benchmark. No dates, user counts or scalability numbers on this site are promises.

## Historical decisions

DuckDB was retired in favor of Go-native analytics. The DataFusion spike concluded without adoption. Neither is part of GoPMgr.

[Read the complete canonical roadmap](https://github.com/draconianneocoder-cpu/GoPMgr/blob/8aa1944f67cfba57f2eabb2d73d2619fb21279e9/docs/ROADMAP.md).
