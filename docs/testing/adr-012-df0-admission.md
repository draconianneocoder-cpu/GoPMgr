<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# DF0: pre-launch reservation admission

This isolated Go host slice follows [native resource accounting](adr-012-df0-resources.md).
It adds one shared `Admission` controller for configured job, memory-byte and
CPU-slot reservations. It is not the production G1 supervisor or a replacement
for OS resource enforcement. DF-064 remains unqualified.

## Contract and ownership

All configured capacities and claim dimensions must be positive. Each admitted
job reserves one job slot plus its declared memory bytes and CPU slots. Under
one mutex, the controller compares each claim with remaining capacity before
adding it, avoiding unsigned overflow. Excess claims return
`admission_rejected` / `RESOURCE_LIMIT`; invalid claims return
`admission_rejected` / `INPUT_INVALID`. No source grant, endpoint or worker is
created for a rejected request. Unmeasured resource fields remain -1. Rejection
has no endpoint to remove; its `endpoint_removed` field stays false.

The host holds the claim through the complete raw `Run` call, releasing it only
after worker reaping, endpoint removal and other deferred cleanup. Startup
failure, query failure, cancellation and forced kill must leave no reservation.
There is no admission queue, retry loop, global mutable controller or new
host-driver goroutine. The owner supplies a shared controller and must not copy
it. Separate controllers and raw baseline `Run` calls are outside its accounting.

Investigation exposed a related cleanup defect: `OnSpawn` ran after starting
the child but before registering the wait/kill/reap defer. If a launch hook
panicked, a reservation could be released while its child remained. Cleanup is
now registered before the hook; bootstrap input also closes on early returns.
The panic still propagates. A waiting synthetic child verifies that the panic
leaves neither child, endpoint nor reservation. The test's own cleanup also
reaps a child when running a deliberately broken disposable copy.

## Evidence

The [environment record](../../spikes/adr012-datafusion/evidence/df0-admission-macos/environment.json)
contains source and unchanged-worker hashes. The
[native cohorts](../../spikes/adr012-datafusion/evidence/df0-admission-macos/cohorts.json)
hold 2, 4 and 8 launched Rust children behind a bootstrap barrier, verify the
full reservation snapshot, reject one excess claim before launch, then release
and verify every child's rows, resources, exit and endpoint cleanup. A subsequent
real worker must succeed with zero claims afterward. The fixture reserves
64 MiB and one CPU slot per job; these are experimental settings, not approved
product budgets or proof of actual memory/CPU use.

The [lifecycle record](../../spikes/adr012-datafusion/evidence/df0-admission-macos/lifecycle.json)
runs the eight earlier pressure/denial/cancel/kill/recovery cases through a
shared controller and checks zero claims after every result. Existing C2 and
resource-accounting regressions remain intact. Native tests use barriers and
bounded lifetimes; cleanup releases and collects the cohort even on assertion
failure.

Reproduce from the spike directory with the pinned release worker:

```sh
DF0_WORKER=/absolute/path/to/df0-worker \
DF0_ADMISSION_EVIDENCE=/absolute/path/to/cohorts.json \
DF0_NO_SPILL_EVIDENCE=/absolute/path/to/lifecycle.json \
  go test -race -count=1 -v ./internal/... ./cmd/...
go vet ./...
```

Focused native and full spike race tests passed, including 60 real Rust worker
jobs across prior scenarios, accounting cohorts, admission cohorts and recovery.
Unit tests cover each limit, independent near-uint64 overflow boundaries,
invalid/exact claims, 32 cold contenders, 32 saturated contenders, failed launch
recovery and launch-hook panic cleanup. Six compiling disposable mutations
failed their intended checks: bypass jobs/memory/CPU, omit release, remove the
reservation lock (race detected), and move the hook before cleanup registration.
The disposable baseline passed and every fault copy was removed.

`make coverage-ledger-current`, `make release-scope`, `make license-check` and
`git diff --check` passed. Linux's host package cross-compiles; native Linux execution is NOT-RUN in this
slice. No Rust source, protocol, dependency, product budget or shipped behavior
changes. Independent reviewer quota was unavailable; structured plan, mid-work
and final self-reviews are the disclosed fallback.

## Remaining qualification

Reservations measure promises, not actual RSS or CPU rate. Source capture can
already have allocated memory before admission; no OS limit or whole-process-tree
bound is established. Kernel per-child lifetime peaks cannot be summed into a
simultaneous host-wide peak. Full DF-033/DF-064 still needs combined host/worker
measurement and independently enforceable resource limits, including crash and
over-quota behavior. Full DF-062 still needs OS-wide file observation and the
remaining containment/security work. macOS is first, Linux second and Windows
execution deferred; historical platform requirements stay visible. This is a
qualified experiment, not permission to remove DuckDB from production.
