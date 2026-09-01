---
name: cornucopia-risk-cards
description: >-
  GoPMgr-specific security-requirement checklist derived from OWASP
  Cornucopia (v3.0 Website App + v1.0 Companion editions), organized by
  suit and cross-referenced to ASVS/STRIDE/CAPEC/CWE. Use this whenever
  building a risk/requirement matrix for GoPMgr code — in particular
  during go-high-assurance's Phase 2 (requirements/risk matrix) and
  before writing security-relevant tests or fixes — for any change under
  internal/crypto, internal/auth, internal/db, app_projects.go,
  app_settings.go (or anywhere projectPathFor is called), internal/fonts,
  internal/signing, PDF/XLSX import-export code, internal/db/audit.go,
  .github/workflows, internal/update, go.mod/go.sum, or frontend/src.
  Also use the aai.md reference when auditing an AI coding agent's own
  tool-use conduct on this repository (not GoPMgr's product code).
---

<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Cornucopia risk cards for GoPMgr

This skill packages the subset of the OWASP Cornucopia card deck relevant
to GoPMgr — a local-first Wails desktop app with no cloud backend, no
public web surface, and no LLM/agent features of its own — as a
requirement-elicitation checklist. Background and the relevance analysis
that produced this scope live in `docs/cornucopia-elicitation-2026-09-01.md`.

Each reference file is one suit: the card text plus its ASVS (or AISVS)
and STRIDE/CAPEC/CWE cross-references, extracted directly from OWASP's
source YAML. **These are questions to ask about the code being touched,
not evidence the questions are already answered.** A card listed here
is a prompt, never a coverage claim — don't cite "this suit is in the
skill" as proof a requirement is met; the evidence has to come from the
actual code, the way go-high-assurance's own risk matrix requires.

## Path → suit lookup

When starting a risk matrix (or any change) under one of these paths,
load *only* the matching reference file(s) below — not the whole set.

| Path pattern | Load |
|---|---|
| `internal/crypto/**`, `internal/auth/**` | `references/cr.md`, `references/at.md` |
| `internal/db/**` (SQLCipher, wrapped-DEK path) | `references/cr.md` |
| `internal/db/audit.go` and its callers | `references/c.md` |
| `app_projects.go`, `app_settings.go`, or any new call site of `projectPathFor` | `references/az.md` |
| `internal/fonts/**`, `internal/signing/**`, any PDF/XLSX import-export path | `references/ve.md` |
| `.github/workflows/**`, `internal/update/**`, `go.mod`/`go.sum` changes | `references/dvo.md`, `references/cr.md` (`internal/update` carries the Ed25519 manifest-verification path — DevOps *and* Cryptography territory) |
| `frontend/src/**` (Svelte, rendered inside the Wails webview) | `references/fre.md` |
| Auditing an AI agent's own conduct on this repo (not GoPMgr's product) | `references/aai.md` |

Suits intentionally not included here (Session Management, Cloud,
Automated Threats, and standalone product-code use of LLM/Agentic AI):
none of them map to anything in GoPMgr's current architecture — see the
elicitation doc for why. If GoPMgr ever grows a remote backend, a
multi-tenant surface, or an in-product AI feature, re-derive relevance
before assuming this table still covers everything.

## Attribution

Card text is CC-BY-SA-4.0 (Elevation-of-Privilege-derived text
CC-BY-SA-3.0), OWASP Cornucopia project (https://cornucopia.owasp.org),
Website App edition v3.0 and Companion edition v1.0.
