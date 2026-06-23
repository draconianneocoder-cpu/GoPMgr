<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Design: DuckDB as a complementary analytics engine

**Status:** Accepted, not yet implemented
**Date:** 2026-06-23
**Decision record:** [ADR-002](ADR-002-duckdb-vs-sqlcipher-evaluation.md) (Option B)
**Owner:** James L. Burns

## Goal

Add DuckDB's analytical horsepower (rich aggregates, window functions,
native file readers) to PMForge **without** touching the SQLCipher
system-of-record or the ADR-001 at-rest security model. SQLCipher stays
the transactional store; DuckDB is an **optional, in-memory, ephemeral**
engine used only for read-side analytics and data import.

## Core principles (what makes this near-zero-risk)

1. **In-memory only — no persistent DuckDB file.** The engine opens
   `:memory:`. Because nothing DuckDB writes ever lands on disk, there is
   **no new at-rest encryption surface**, no `httpfs`/OpenSSL write
   dependency, no storage-format-stability exposure, and no offline-write
   problem — exactly the issues that made a full migration unattractive
   in ADR-002.
2. **DuckDB never touches the encrypted `.pmforge` file.** The app reads
   rows from SQLCipher (already decrypted in-process) and **feeds them
   into DuckDB in memory** via the bulk Appender API. Sensitive data
   stays in process memory — the same exposure the app already has while
   running; no new on-disk plaintext.
3. **Opt-in via build tag.** DuckDB (`github.com/duckdb/duckdb-go/v2`) is
   a heavy CGO dependency that inflates binary size. All DuckDB code sits
   behind `//go:build duckdb`. Default builds compile a no-op stub and
   link nothing DuckDB-related; only the explicitly tagged "analytics"
   build links the engine. The standard desktop download stays lean.
4. **The pure-Go kernel stays the source of truth for scheduling math.**
   CPM/EVM/MSPDI computations remain in `internal/kernel` (deterministic,
   testable, no I/O). DuckDB does **cross-cutting aggregation** over many
   rows/projects and **file ingestion** — it does not reimplement or
   replace the kernel.
5. **Hardened DuckDB configuration.** Every DuckDB session disables
   extension autoinstall/autoload, disables external file access by
   default, and locks configuration — applying the DuckDB "Securing
   DuckDB" guidance. File access is opened only for an explicit,
   user-chosen path during import (`allowed_paths`).

## Architecture

```
                SQLCipher .pmforge (system of record, encrypted at rest)
                                  │  app reads decrypted rows (in process)
                                  ▼
   internal/analytics  ──Engine interface──┐
        ├─ stub.go        (default build; ErrAnalyticsUnavailable)
        └─ duckdb.go      (//go:build duckdb)
                                  │  Appender bulk-load rows  →  in-memory DuckDB
                                  │  run aggregation / window queries
                                  ▼
                          results structs  →  App methods  →  Svelte UI
```

- New package **`internal/analytics`** exposes an `Engine` interface
  (e.g. `RunPortfolioRollup(ctx, snapshot) (Result, error)`,
  `ImportTabularFile(ctx, path) (Dataset, error)`), plus the sentinel
  `ErrAnalyticsUnavailable`.
- **`stub.go`** (no build tag) returns `ErrAnalyticsUnavailable`. This is
  what default builds use, so the UI degrades gracefully ("Analytics
  build not installed").
- **`duckdb.go`** (`//go:build duckdb`) is the real implementation:
  open `:memory:`, apply hardening pragmas, bulk-load via Appender, query.
- `main.go` wires whichever implementation is compiled in; App methods
  surface analytics to the frontend and always handle
  `ErrAnalyticsUnavailable`.

### Dependency handling note

A file guarded by `//go:build duckdb` still causes `go mod tidy` to
record `duckdb-go/v2` in `go.mod`/`go.sum` (tidy considers all build
tags). That is expected and fine: **the dependency is declared, but
default `go build` does not compile or link it**, so untagged binaries
carry zero DuckDB code or size. Only `go build -tags duckdb` links it.

## Security posture (must hold)

- No persistent DuckDB file (`:memory:` only); verified in tests.
- DuckDB is never handed the encrypted file path; it only receives rows
  the app already decrypted in memory.
- Session hardening on every connection:
  `SET autoinstall_known_extensions=false; SET autoload_known_extensions=false;`
  `SET enable_external_access=false;` (relaxed to a single `allowed_paths`
  entry only for an explicit user import), then `SET lock_configuration=true`.
- No network, no telemetry, no community extensions.
- ADR-001 and the SQLCipher release gates are unchanged and stay green.

## Scope

**In scope (DuckDB's real value):**

- **Portfolio / cross-project analytics** — rollups and comparisons
  across many projects (the existing `Portfolio.svelte` is the natural
  first surface).
- **Aggregate/statistical reporting** — feeding Six Sigma stats and
  EVM/portfolio summaries that benefit from set-based aggregation.
- **Native local-file ingestion** — CSV, Parquet, and JSON are
  *Primary*-tier readers bundled in standard DuckDB builds (offline, no
  download); they power a new "import any tabular data" capability.
  Excel (`.xlsx`) is a different case — see *File-import evaluation* below.

**Out of scope (explicitly):**

- Replacing the SQLCipher store or any transactional CRUD path.
- Moving the CPM/EVM/MSPDI kernel math into SQL.
- Any persistent DuckDB artifact on disk.

## File-import evaluation (and the `read-excel-file` question)

DuckDB's readers split into two tiers for an offline, local-first app:

| Format | DuckDB reader | Bundled & offline? |
|---|---|---|
| CSV | `read_csv` / `FROM 'f.csv'` | **Yes** — core, always available |
| Parquet | `read_parquet` | **Yes** — *Primary* tier, in standard builds |
| JSON | `read_json` | **Yes** — *Primary* tier, in standard builds |
| Excel `.xlsx` | `read_xlsx` / `FROM 'f.xlsx'` | **No** — the `excel` extension is *Secondary* tier and **auto-downloads from the official extension repository on first use** |

**CSV / Parquet / JSON — strong fit.** Use DuckDB to add a new "import
any data" feature for these. They are bundled, offline, extension-free,
and exactly DuckDB-shaped. The backend takes a user-chosen path (native
dialog, like the existing `ImportMSPDIChart`), reads it into in-memory
DuckDB under a single `allowed_paths` grant, then the app persists
results into SQLCipher as needed. This is genuinely new capability —
PMForge cannot read Parquet at all today.

**Excel / retiring `read-excel-file` — not worth it now.** Three reasons:

1. **The `excel` extension auto-downloads from the internet.** To stay
   offline/local-first it must be pre-bundled with `autoinstall`/
   `autoload` disabled — the same packaging burden that counted against a
   full migration (cf. `httpfs` for encryption in ADR-002).
2. **It would force "DuckDB always-on."** The Sigma `.xlsx` import is a
   *frontend* path; `read-excel-file` is in every build, DuckDB (build-tag
   gated) is not. Retiring `read-excel-file` would either break `.xlsx`
   import in default builds or require shipping the heavy DuckDB engine in
   *every* build. We just deliberately migrated to `read-excel-file`
   (small, maintained, npm-native, zero CVEs); swapping it for the one
   format DuckDB makes *harder* is churn, not progress.
3. **`.xls` parity is no longer a factor.** Legacy `.xls` support was
   deprioritized by the owner (2026-06-23), so it is neither a blocker nor
   a reason to switch — it simply drops out of the comparison.

**Recommendation:** keep `read-excel-file` as the universal `.xlsx` path
**for now**; position DuckDB as the engine that *adds* CSV/Parquet/JSON
import plus the analytics over it. **Future consolidation:** PMForge plans
external-database access via a plugin/extension mechanism (see *Future
direction*). The offline, controlled extension-loading capability that
requires is the *same* capability the `excel` extension needs — so once
it lands, retiring `read-excel-file` in favor of DuckDB's `read_xlsx`
becomes a clean consolidation (with `.xls` no longer a concern). Until
then, `read-excel-file` stays and Phase D ships CSV/Parquet/JSON only.

## Future direction: external databases

PMForge plans to access external databases via a plugin/extension
mechanism (owner direction, 2026-06-23). This is squarely DuckDB
territory and reinforces Option B: DuckDB ships connector extensions —
`postgres` (postgres_scanner), `mysql` (mysql_scanner), `sqlite`
(sqlite_scanner), and `odbc` — that `ATTACH` external databases and let
you query and join across them and the in-memory analytics set. Two
implications:

- The build-tag-gated analytics engine is a natural home for (or sibling
  of) that future "data/connectors plugin" — the optional-capability
  packaging already matches a plugin model.
- Those connectors are *Secondary*-tier extensions with the same offline
  discipline as `excel`: pre-bundle the needed extensions, disable
  internet autoinstall/autoload, and load locally. Solving that once for
  the external-DB feature also unlocks `excel`/`read_xlsx`. Any network
  egress for a connector must stay **explicit and user-initiated** to
  preserve the local-first posture.

## Phased task plan

- **Phase A — Interface + stub (no new dependency).** Create
  `internal/analytics` with the `Engine` interface, `stub.go`,
  `ErrAnalyticsUnavailable`, and App-method wiring that degrades
  gracefully. Fully testable in the default build; `make verify`/`race`
  stay green. *No `go.mod` change yet.*
- **Phase B — DuckDB engine behind `//go:build duckdb`.** Add
  `duckdb-go/v2`; implement `duckdb.go`: in-memory open, hardening
  pragmas, Appender bulk-load, query execution, result mapping. Unit
  tests under the tag.
- **Phase C — First feature end-to-end: Portfolio rollup.** Backend
  `App.RunPortfolioAnalytics()` (DuckDB-backed, stub fallback) + wire
  into `Portfolio.svelte`. Proves the whole path with a real, visible
  benefit.
- **Phase D — Local-file ingestion (CSV / Parquet / JSON)** with the
  `allowed_paths` hardening; expose an "import dataset for analysis"
  surface. (`.xlsx` stays on `read-excel-file`; Excel-via-DuckDB only if
  DuckDB goes always-on and the `excel` extension is bundled offline —
  see *File-import evaluation*.)
- **Phase E — CI, docs, budgets.** Add a `-tags duckdb` build+test CI job
  (at least Linux) so it can't bitrot; record the dependency in
  `DEPENDENCIES.md` with justification; note the analytics build in
  `AGENT.md`/`README.md`; add a binary-size budget check for the tagged
  build.

## Testing & verification

- Default build: stub path unit-tested; existing `make verify` / `make
  race` unaffected (no DuckDB linked).
- Tagged build: `go test -tags duckdb ./internal/analytics/...` with
  golden-result assertions on representative aggregations; an
  in-memory-only invariant test (no files created); a hardening test
  (external access denied).
- CI: a dedicated `-tags duckdb` job, kept separate from the default
  gate.

## Open decisions (recommended defaults in **bold**)

- Packaging — the pivotal decision: **build-tag gated, shipped as a
  separate "PMForge (Analytics)" artifact** vs. always-on (simpler, but
  every download pays the size cost). *Recommend build-tag.* Note:
  owning `.xlsx` import / retiring `read-excel-file` is only possible
  under *always-on*.
- First feature: **Portfolio cross-project rollup** vs. statistical
  reporting first. *Recommend Portfolio.*
- Excel-via-DuckDB / retiring `read-excel-file`: **no for now** — keep
  `read-excel-file` (see *File-import evaluation*); the `excel` extension
  auto-downloads and would force always-on with no capability gain.

## Sources / related

- [ADR-002 — DuckDB vs SQLCipher evaluation](ADR-002-duckdb-vs-sqlcipher-evaluation.md)
- [ADR-001 — Per-user database encryption at rest](ADR-001-database-encryption-at-rest.md)
- [DuckDB Appender](https://duckdb.org/docs/current/data/appender)
- [Securing DuckDB](https://duckdb.org/docs/current/operations_manual/securing_duckdb/overview)
