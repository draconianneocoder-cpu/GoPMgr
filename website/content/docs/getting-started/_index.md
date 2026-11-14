+++
title = "Getting started"
description = "Prepare a safe evaluation workspace."
weight = 30
[params]
status = "development"
applies_to = "Development snapshot 88b92cf935c7"
last_reviewed = "2026-11-11"
claim_ids = ["page.docs.getting-started"]
[[params.sources]]
label = "docs/INSTALL.md"
url = "https://github.com/draconianneocoder-cpu/GoPMgr/blob/88b92cf935c77332cec4357789250126a5e6b314/docs/INSTALL.md"
[[params.sources]]
label = "docs/user-guide.md"
url = "https://github.com/draconianneocoder-cpu/GoPMgr/blob/88b92cf935c77332cec4357789250126a5e6b314/docs/user-guide.md"
+++

<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

## Before opening a project

1. Choose either the historical evaluation release or a source build. Read its requirements and known gaps before installation.
2. Back up existing GoPMgr data. Use fictional test data for evaluation.
3. On first launch, create a local account and store the one-time recovery codes safely. Do not put recovery codes in a project, screenshot, issue or shared log.
4. Follow [your first project](first-project/) to create a small test workspace.

## Keep versions distinct

The reviewed current source requires macOS 26 Tahoe or Ubuntu 26.04 for those release targets. Windows installer qualification remains separate. Published alpha.2 packages retain their own historical requirements and evidence; consult [downloads](../../downloads/).

## Backup and recovery

Removing the application may leave your project and account data on disk. Recovery codes are one-time credentials. New codes replace the previous codes only after you choose **Use the new codes**. Read the [canonical recovery guidance](https://github.com/draconianneocoder-cpu/GoPMgr/blob/88b92cf935c77332cec4357789250126a5e6b314/docs/user-guide.md#recovery-codes-and-encryption) before relying on them. This tutorial does not ask you to delete, migrate or reset existing data.
