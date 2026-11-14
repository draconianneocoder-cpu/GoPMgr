+++
title = "Security and reporting"
description = "Understand the desktop boundary and report a vulnerability in private."
weight = 80
layout = "wide"
[params]
status = "development"
applies_to = "Development snapshot 88b92cf935c7"
last_reviewed = "2026-11-11"
claim_ids = ["page.security"]
[[params.sources]]
label = "docs/SECURITY.md"
url = "https://github.com/draconianneocoder-cpu/GoPMgr/blob/88b92cf935c77332cec4357789250126a5e6b314/docs/SECURITY.md"
[[params.sources]]
label = "docs/user-guide.md"
url = "https://github.com/draconianneocoder-cpu/GoPMgr/blob/88b92cf935c77332cec4357789250126a5e6b314/docs/user-guide.md"
+++

<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

## Desktop security boundary

GoPMgr is local-first. Operating-system account security, backups and careful handling of recovery codes remain essential. Project encryption, local authentication and document signatures address different risks; none is an absolute guarantee against attacks or data loss.

[Read the canonical security model](https://github.com/draconianneocoder-cpu/GoPMgr/blob/88b92cf935c77332cec4357789250126a5e6b314/docs/SECURITY.md) and [recovery-code guidance](https://github.com/draconianneocoder-cpu/GoPMgr/blob/88b92cf935c77332cec4357789250126a5e6b314/docs/user-guide.md#recovery-codes-and-encryption).

## Report a vulnerability

Report a security vulnerability in private. Open the [GitHub vulnerability report](https://github.com/draconianneocoder-cpu/GoPMgr/security/advisories/new).

Include a summary, the details, a proof of concept, and the impact. Do not put vulnerability details or secrets in a public issue.

## Website privacy

This static companion site adds no analytics, telemetry, login, service worker or project uploads. Search runs locally in your browser using a locally served index. Outbound links open external services only when you follow them. GitHub Pages, if enabled later, applies its own [hosting privacy policy](https://docs.github.com/en/site-policy/privacy-policies/github-general-privacy-statement).
