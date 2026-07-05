<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# PMForge security, quality & stability review — 2026-07-05

Static review of the code that landed on `main` after the 2026-06-29 pass —
the Monte Carlo schedule-risk engine, the scenario-analysis store, the
tamper-evident audit hash-chain, the exact-money package, and resource
calendars — plus the new Wails IPC methods that expose them.

## Verdict

One genuine **stability** defect (unbounded Monte Carlo `iterations`/
`workers` at the Go IPC boundary — a crash/OOM vector), fixed in this pass.
Everything else in the new surface follows the codebase's established
defensive patterns: parameterized SQL throughout, a sound tamper-evident
audit hash chain, export writers confined to the user's private
`exports/` directory with sanitized names, exact integer money arithmetic,
and no new frontend HTML-injection sinks. No confidentiality or integrity
issues found.

## Findings

### S-1 — LOW/MED (stability) — Monte Carlo iterations/workers were unbounded at the backend boundary — RESOLVED

`RunChartMonteCarlo` / `ExportChartMonteCarloRiskReport` (`main.go:1393`,
`main.go:1422`) pass the frontend-supplied `iterations` and `workers`
straight into `kernel.RunMonteCarlo` (`internal/kernel/montecarlo.go`).
`validateMonteCarloInputs` required `iterations > 0` but imposed **no upper
bound**, and `workers` was only clamped to `iterations`.

- Simulation memory is O(iterations × tasks): `finishes` and each task's
  `durationSamples` are preallocated with `iterations` capacity. A large
  `iterations` (e.g. 2e9) either exhausts memory or panics in `makeslice`,
  and an unrecovered panic in a Wails-bound method **crashes the whole
  process**.
- A large `workers` value spawns that many goroutines.

The GUI clamps `iterations` to `[100, 10000]` and passes `workers = 0`
(`CPMEditor.svelte:170-189`), so normal use is safe — but that is a
client-side control. The Go IPC method is the real trust boundary (the same
principle as F-1 in the previous review), reachable by a compromised
webview, the CLI, or a future caller that does not clamp.

**Resolution (2026-07-05).** Added hard ceilings in
`internal/kernel/montecarlo.go`, so every caller is protected at the point
the values are used:

- `maxMonteCarloIterations = 100_000` — `validateMonteCarloInputs` now
  returns an error above this (10× the GUI max, ample for CLI/power use,
  bounded well below the OOM/panic range). The error surfaces cleanly to
  the caller before any large allocation.
- `maxMonteCarloWorkers = 128` — `RunMonteCarlo` clamps the fan-out.
  Results are deterministic per iteration index regardless of worker count,
  so the clamp is invisible.

Tests: an "iterations over maximum" case added to
`TestRunMonteCarloRejectsInvalidInputs`, plus `TestRunMonteCarloClampsWorkers`.
`go test ./...` green.

## Verified correct (grounded, no action)

- **Audit hash chain** (`internal/db/audit.go`,
  `internal/db/audit_checkpoints.go`) — each event's hash is
  `sha256(previous_event_hash || canonicalJSON(payload))`, and
  `VerifyAuditChain` recomputes every hash while checking sequence
  continuity and previous-hash links. `OpenProject` runs
  `verifyProjectAuditForOpen` (when `ComplianceMode` is on) and refuses to
  activate a project whose chain is broken (`main.go:1010`). Canonical JSON
  (marshal→unmarshal→marshal) makes the digest independent of key order.
  Sound tamper-evident design.
- **New SQL** — `scenarios.go`, `resource_calendars.go`,
  `audit_checkpoints.go`, `settings.go` use parameterized queries
  exclusively. The only string-built SQL in the tree remains
  `analytics/duckdb.go:233`, unchanged and still safe (single-quote-escaped
  path + whitelisted reader + const `LIMIT`).
- **Report exports** — `ExportChartMonteCarloRiskReport`,
  `ExportAuditVerificationReport`, `ExportAuditRepairEvidence` all write to
  `filepath.Join(u.DataDir, "exports")` with a `sanitizeFilename(proj.Name)`
  basename and `0o600`/`0o700` permissions. No frontend-controlled path.
- **Money** (`internal/money/money.go`) — exact integer minor units with
  `math/big.Rat` for rate×quantity and a single rounding at the boundary;
  guards `NaN`/`Inf`. No float accumulation error.
- **Monte Carlo RNG** — `math/rand/v2` PCG seeded deterministically per
  iteration index. Correct for a reproducible simulation; not a security
  context (no key or token derives from it).
- **Frontend** — no `{@html}` or `innerHTML` in the new Svelte components
  (scenario editor, MC panel); Svelte auto-escaping holds.
- **F-1 confinement intact post-merge** — all six path-taking IPC methods
  still route through `projectPathFor`, including the new
  `appendProjectDeleteAudit` path (`main.go:492`, via the confined
  `DeleteProject`).

## Priority

1. **S-1** — ~~bound Monte Carlo iterations/workers at the kernel~~ **Done
   (2026-07-05).** (Low/Med, stability)

## Scope / limits

Static read plus `go test ./...`, `go vet`, and `gofmt` (all clean on the
patched tree). Not run locally: `golangci-lint` / `govulncheck` (built
against go1.25, cannot analyze this go1.26.4 tree — CI runs both). Not
covered: dynamic/runtime testing, the new shell validation scripts under
`scripts/`, and a line-by-line audit of every new chart/document renderer.
