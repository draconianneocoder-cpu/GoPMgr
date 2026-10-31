<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# DF0: host-observed child resources on macOS

This corrects a measurement defect before aggregate admission work. The Rust
worker's self-report reads Linux `/proc/self/status`; on macOS it returned zero.
The Go live-process fallback was also unavailable. Failed or killed jobs lacked
usable memory observations. Zero was not evidence of zero memory use.

The isolated Go host now obtains the launched child's final `ProcessState`
resource usage after its existing wait channel closes. That channel synchronizes
`cmd.Wait` with collection on every post-start return path, including bootstrap
failure, typed query failure, cancellation and forced kill. The host uses the
child's kernel accounting, not a worker-supplied metric, a reusable PID lookup
or the global `RUSAGE_CHILDREN` accumulator. No new host-driver goroutine, unsafe code,
FFI, dependency, protocol or production interface is introduced.

`worker_peak_rss_bytes` now records final kernel peak resident bytes. Darwin
reports bytes and Linux reports KiB; the host normalizes the latter. The installed
`getrusage(2)` manual and Go's `os` wait implementation establish the native
boundary. Nonpositive or overflowing memory values remain -1. New
`worker_user_cpu_ns` and `worker_system_cpu_ns` fields record lifetime child CPU
time, including startup and shutdown; they remain -1 when no state is available.
`worker_resource_method` records provenance. Failure before launch keeps all
three measurements unknown and still removes the private endpoint.

## Native evidence and reproducibility

The [environment and hashes](../../spikes/adr012-datafusion/evidence/df0-resources-macos/environment.json)
identify the code and unchanged release worker. The
[lifecycle observations](../../spikes/adr012-datafusion/evidence/df0-resources-macos/lifecycle.json)
cover the eight spill/denial/normal/cancel/kill/recovery scenarios. The
[concurrent observations](../../spikes/adr012-datafusion/evidence/df0-resources-macos/concurrent.json)
cover 2, 4 and 8 launched Rust children, held behind a bootstrap barrier before
release. Every child returns independently verified generated rows, records its
own resources, exits and removes its endpoint. These are accounting cohorts,
not aggregate-admission qualification or latency benchmarks.

From the spike directory, with a freshly built pinned worker:

```sh
DF0_WORKER=/absolute/path/to/df0-worker \
DF0_NO_SPILL_EVIDENCE=/absolute/path/to/lifecycle.json \
DF0_RESOURCE_EVIDENCE=/absolute/path/to/concurrent.json \
  go test -race -count=1 -v ./internal/... ./cmd/...
go vet ./...
```

The allocation control launches a bounded subprocess that touches 64 MiB and
performs CPU work. A broad 64 MiB to 4 GiB bound detects a 1024-fold native-unit
mistake while allowing test instrumentation. It is not a product memory budget.
Tests also check invalid/overflow values, unavailable state and launch failure.
Disposable copies passed before faults; omitting final accounting failed the
native lifecycle assertion, and the wrong Darwin scale failed the allocation
bound. Compilation failures are not counted as detected defects.

The native macOS race suite and vet passed, including the 29 earlier worker
scenarios and 14 concurrent children. The Linux host test package cross-compiled;
native Linux execution is NOT-RUN. `make coverage-ledger-current`, `make release-scope`, `make license-check`
and `git diff --check` passed separately. No Rust source changed or new binary was
built for this host-only correction. The worker matches the reviewed no-spill
artifact by SHA-256. Independent review was unavailable under the reviewer quota;
structured plan, mid-work and final self-reviews are the disclosed fallback.

## Remaining work and proof limits

A lifetime per-child peak is not instantaneous RSS, a sum across processes,
process-tree accounting, CPU rate, OS enforcement, an admission reservation or
an application memory budget. Summing independent peaks would not establish a
simultaneous host-wide peak. DF-033's aggregate limits and DF-064 remain
unqualified. A later [reservation admission slice](adr-012-df0-admission.md)
adds pre-launch claim rejection and cleanup/recovery evidence. Actual combined
host/worker limits and native measurements remain necessary.

Full DF-062 still needs independent OS-wide filesystem observation and the
remaining security controls. Existing no-spill and C2 raw records are preserved
as historical observations; their zero/-1 fields are not retroactively fixed.
Socket/credential inspection is still unavailable on Darwin. macOS is the
maintainer's current first priority, Linux second, and Windows execution is
deferred; historical acceptance criteria and platform gaps stay visible.
Comparative performance/numeric fidelity, packaging/licensing and the final
report precede a separate production integration and cutover decision.
