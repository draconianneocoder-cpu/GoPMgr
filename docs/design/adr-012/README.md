<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-012 document set

> **2026-11-02:** DuckDB is retired. The maintainer chose a Go-native engine
> for both analytics operations instead of the DataFusion worker; see the
> [retirement decision](../../review/ADR-012-DUCKDB-RETIREMENT-OWNER-DECISION.md). This record is kept as history and as evidence
> for possible future columnar analytics.

ADR-012 v0.2 was **accepted on 2026-10-26** as the binding target
architecture for replacing DuckDB analytics. Only the DF0 feasibility spike is
authorized. See the
[acceptance record](../../review/ADR-012-v0.2-ACCEPTED-OWNER-DECISIONS.md).

## Canonical (accepted)

| Document | Role |
|---|---|
| [ADR-012](../ADR-012-datafusion-analytics-replacement.md) | Decisions DF-01 to DF-15 and amendments DF-A01 to DF-A14 |
| [DF0 specification](../../testing/adr-012-datafusion-feasibility-spike.md) | Phases DF0-A to DF0-G, tests DF-001 to DF-065 |
| [Agent instructions](../../agent/adr-012-coding-agent-instructions.md) | Binding architecture-preservation rules |
| [Owner prompt](../../agent/adr-012-owner-prompt-to-paste.md) | The maintainer's task directive |
| [v0.2 traceability](../../review/adr-012-v0.2-improvements-and-traceability.md) | Amendment dispositions |
| [Repository verification](../../review/adr-012-repository-verification.md) | Errata and findings checked against the code before DF0 |

## Historical (unchanged)

- [`v0.2-as-submitted/`](v0.2-as-submitted/): the v0.2 files exactly as
  received, before their status lines were updated.
- [`v0.1/`](v0.1/): the v0.1 draft bundle. It is not operative.

The canonical copies differ from `v0.2-as-submitted/` only in their status
lines. Both copies keep their Markdown hard line breaks, so `.gitattributes`
exempts them from whitespace checks.
