<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010 S0 baseline harness

Measurement tools for the ADR-010 P02/S0 baseline. They record how the
current GoPMgr code behaves today. They do not change it, and nothing
here ships with the application.

The findings are written up in
[`docs/testing/adr-010-s0-baseline.md`](../../docs/testing/adr-010-s0-baseline.md).

## What is here

| Path | What it does |
| --- | --- |
| `cmd/s0bench` | Builds a seeded synthetic schedule at four sizes (500, 10,000, 50,000 and 100,000 tasks), then times CPM, Gantt layout, chart JSON, MSPDI export and import, encrypted storage, one-field edits and audit-chain checks. |
| `cmd/s0fs` | Probes the temp folder, the home folder and the default project root for file-system behavior that ADR-010 storage depends on: no-replace rename, case sensitivity, exclusive locks, folder fsync and long paths. |
| `cmd/s0sql` | Opens a fresh encrypted project database the way the app does and records the engine version, effective pragmas, compile options and which SQL features work (JSON, FTS5, `RETURNING`, `STRICT`, `DROP COLUMN` and others). |
| `evidence/` | JSON Lines output from recorded runs. Each record names the commit, OS, CPU and Go version. |

## Run it

CGO and a C compiler are required, because the harness links the
project's SQLCipher driver.

```sh
cd spikes/adr010-s0-baseline
head=$(git rev-parse --short HEAD)
go run ./cmd/s0fs -head "$head" -out evidence/<platform>-fs.jsonl
go run ./cmd/s0sql -head "$head" -out evidence/<platform>-sql.jsonl
go run ./cmd/s0bench -head "$head" -out evidence/<platform>-bench.jsonl
```

Useful `s0bench` flags:

- `-max-tasks 10000` skips the 50,000 and 100,000 tiers for a quick run.
- `-cpuprofile s0.prof` writes a CPU profile; read it with
  `go tool pprof -top s0.prof`.

A full run on a 4-core container took 11 minutes and needed about
1 GB of free disk for the temporary encrypted databases, which are
deleted afterwards. Most of the time is the 100,000-task Gantt layout.

## Reading the results

- Times are wall-clock milliseconds with p50, p95 and max over the
  samples taken. `n` is the number of samples.
- Gantt layout has a total time budget per tier (120 s). Large tiers
  record fewer samples and say so in `notes`.
- A container is not an approved hardware profile. These numbers are a
  baseline for comparison, not a pass or fail against ADR-010 budgets.
  The hardware profiles and budgets are owner decision OWN-10.
- `sync_hint` in `s0fs` output is a naming-convention hint only. Per
  INT-03 it must never be used to delete, move or reject project data.
