<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# DF0-E: partial macOS portfolio backend characterization

2026-10-31. Isolated spike evidence, not a performance acceptance verdict or
authorization to replace the production engine. The
[DF0 specification](adr-012-datafusion-feasibility-spike.md) still requires
controlled cold/warm, larger-scale, multi-process and representative end-to-end
measurements. DF-041–DF-045 and DF-050 remain unqualified.

## Method and boundaries

The same deterministic 0/100/1,000-project fixtures run through three paths:

- **A:** the shipped `analytics.New` / `PortfolioRollup` / `Close` path,
  unchanged. The application also creates a new engine per portfolio request.
- **B:** normalization, source JSON, fresh native Rust/DataFusion worker over
  gRPC/UDS, result conversion and worker/endpoint cleanup.
- **C:** the existing exact Go rollup. No worker is launched for portfolio
  arithmetic; DataFusion's separate tabular role is outside this measurement.

Each timed region includes provider-specific input preparation, execution,
cleanup and common backend summary JSON serialization. It excludes fixture
generation, the independent oracle, evidence-file writes, project enumeration,
SQLCipher reads, Wails encoding and native UI delivery. This is backend latency,
not application end-to-end latency or the production wire representation.

Each provider/case has one separately recorded first observation, three
warm-ups and 30 measured calls: 306 calls total, 270 in the distributions.
Provider/case order is shuffled each round with recorded fixed seeds. Every
result must match the independent big-integer oracle, including coverage counts,
exact minor-unit totals, display fields and the existing two-ULP index tolerance.
Rust additionally requires matching source/batch digests, normal exit, kernel
resource accounting and endpoint removal. Failures stop the command without a
completion summary; they cannot count as fast results.

The local Apple M4 / 32 GiB / macOS 27.2 machine used AC power and reported no
recorded thermal/performance warning. Power/thermal settings and other machine
activity were not controlled. Caches were neither purged nor measured. Neither
the first observation nor later calls qualify cold filesystem or persistent
warm-worker performance. Rust is fresh for every call.

Thread settings also differ: the existing Rust query session targets one
partition, its Tokio runtime uses two worker threads, the Go host reports
`GOMAXPROCS=10`, and the unchanged DuckDB engine uses its default compute-thread
configuration. One DuckDB pooled connection does not imply one compute thread;
actual thread utilization was not measured or normalized.

## Observed results

Milliseconds, nearest-rank descriptive p50/p95 from 30 measured calls per cell:

| Projects | A DuckDB p50 / p95 | B Rust/DataFusion p50 / p95 | C Go rollup p50 / p95 |
| ---: | ---: | ---: | ---: |
| 0 | 4.65667 / 5.23837 | 10.64979 / 12.46117 | 0.00150 / 0.00346 |
| 100 | 12.67238 / 13.94729 | 11.12196 / 12.45133 | 0.00613 / 0.01492 |
| 1,000 | 85.78012 / 86.90571 | 13.77754 / 15.68996 | 0.03000 / 0.07683 |

These observations favor further qualification of C for portfolio arithmetic;
they do not select the production architecture. Microsecond Go measurements
approach timer/accounting overhead. No speedup ratio, credible tail bound or
owner-approved budget PASS follows from this experiment. `p99_ns` is explicitly
null; the specified ≥1,000-sample/multi-process qualification has not run.

Raw samples include host CPU-time deltas and, for B, child lifetime CPU, peak
RSS, spawn-to-Hello latency and source-payload bytes. Host plus child CPU times
may be added as CPU integrals, not treated as wall latency. Source bytes do not
measure total transport framing. The one shared host binary links DuckDB for
all candidates. Its final lifetime peak includes fixture setup and all earlier
providers, cannot be assigned to one query, and cannot be summed with worker
peaks to claim simultaneous aggregate RSS. No memory-bound or bundle-size
qualification is claimed.

## Reproduction and detector checks

From `spikes/adr012-datafusion`, with the reviewed existing native worker:

```sh
go build -tags duckdb -o /absolute/path/to/df0-performance ./cmd/df0-performance
/absolute/path/to/df0-performance -worker /absolute/path/to/df0-worker \
  -samples 30 > /absolute/path/to/portfolio.jsonl
DF0_WORKER=/absolute/path/to/df0-worker \
  go test -race -tags duckdb -count=1 -v ./cmd/df0-performance
```

[Raw samples](../../spikes/adr012-datafusion/evidence/df0-performance-macos/portfolio.jsonl)
include fixture/oracle hashes, every phase and a final summary.
[Environment/provenance](../../spikes/adr012-datafusion/evidence/df0-performance-macos/environment.json)
records source, runner, unchanged worker and raw-evidence hashes. Build time is
excluded. This Go-only slice reused the previously reviewed Rust binary; it
does not provide a new reproducible Rust build or package qualification.

Focused tests cover all nine real provider/case combinations, failed launches,
incomplete evidence, wrong totals, resource/cleanup refusals and summary
statistics. Four compiling faults (wrong p95, ignored cleanup, ignored oracle,
missing fixture)
failed at their intended assertions; an unmodified control passed. Python
independently recomputed all nine distributions, phase/sample counts, means and
population variances from the archived raw values. The initial statistics test
exposed avoidable rounding from dividing each term before summation; mean and
variance now divide once after accumulation. Expected values were unchanged.
Final review added a required-fixture guard and moved size assertions ahead of
the native-worker skip, preventing silently incomplete workloads.

The full spike race suite with the explicit Rust worker, `go vet -tags duckdb
./...`, and root application/default analytics tests passed. Independent review
was unavailable because the agent quota was exhausted; structured self-review
is a disclosed fallback, not an independent-review PASS. Native Linux, Windows,
minimum-macOS, signed-package, UI and full performance qualification were not
performed. Production code, dependencies, strict resource guarantees and the
unapproved containment decision request are unchanged.
