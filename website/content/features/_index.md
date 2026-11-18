+++
title = "Capabilities in development"
description = "What current source provides and what remains unqualified."
weight = 10
layout = "wide"
[params]
status = "development"
applies_to = "Application snapshot 88b92cf935c7; planning review ba39f39fc8c9; manual review 4f4ab0c7572a, f54b26428f70"
last_reviewed = "2026-11-18"
claim_ids = ["page.features"]
[[params.sources]]
label = "docs/ROADMAP.md"
url = "https://github.com/draconianneocoder-cpu/GoPMgr/blob/156a8eded0088d608f39c78e7c04a6089f9bbc8a/docs/ROADMAP.md"
[[params.sources]]
label = "docs/user-guide.md"
url = "https://github.com/draconianneocoder-cpu/GoPMgr/blob/156a8eded0088d608f39c78e7c04a6089f9bbc8a/docs/user-guide.md"
+++

<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

The application capabilities below describe the recorded application snapshot. The roadmap source was reviewed separately at planning commit `ba39f39fc8c9`. The published alpha may not include it, and it has not been qualified on every platform.

## Scheduling and baselines

**Development:** model task dependencies, use business calendars and capture baselines. Read the [scheduling guide](https://github.com/draconianneocoder-cpu/GoPMgr/blob/156a8eded0088d608f39c78e7c04a6089f9bbc8a/docs/user-guide.md#stakeholders-timeline-budget-and-cost-control). No task-count or performance figure has been verified.

## Costs and earned value

**Development:** track budgets and committed costs, then review portfolio rollups with explicit coverage and currency rules. GoPMgr has no foreign-exchange conversion. [Read the portfolio contract](https://github.com/draconianneocoder-cpu/GoPMgr/blob/156a8eded0088d608f39c78e7c04a6089f9bbc8a/docs/user-guide.md#portfolio-and-dashboard).

## Charts, documents and exports

**Development:** work with project charts, documents and export formats. Exported PDFs can be archival (PDF/A) and digitally signed (PAdES). How far a signature can be trusted depends on your certificate and on the reader that validates it. [Read report and signing guidance](https://github.com/draconianneocoder-cpu/GoPMgr/blob/156a8eded0088d608f39c78e7c04a6089f9bbc8a/docs/user-guide.md#pdf-signing).

## Local data protection

**Development:** local accounts, encrypted project databases and one-time recovery codes have distinct limits. They do not replace operating-system security or independent backups. [Understand the security model](../security/).
