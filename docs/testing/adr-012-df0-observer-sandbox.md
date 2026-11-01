<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# DF0: native macOS observation and App Sandbox feasibility

On 2026-11-01 the owner authorized native monitoring/termination design and
measurement, then requested stronger isolation as well. This isolated experiment
implements both sets of building blocks. It does not approve a resource-policy
relaxation, production integration, final budgets or DuckDB removal. The
[containment ACR](../review/ADR-012-macos-containment-ACR.md) remains proposed.

## What ran

The [environment and source manifest](../../spikes/adr012-datafusion/evidence/df0-observer-sandbox-macos/environment.json)
binds integration baseline `298f6b9`, macOS 27.2 arm64, installed SDK/compiler
headers, the system App Sandbox profile and the unchanged Rust worker input.
The C probe is a separate executable, never cgo or Rust FFI. No production
launcher, dependency, entitlement, source grant or transport changed.

| Experiment | Observed result | Proof limit |
| --- | --- | --- |
| Current usage for 2/4/8 held children | External `proc_pid_rusage` reads see each child's resident growth after a touched 32 MiB allocation. Start identity and timestamps are checked; independent `wait4` peaks confirm allocation | Synthetic children, not concurrent DataFusion queries. Sequential observations expose skew; no atomic host-wide snapshot, unique physical-memory total or quota |
| RSS/CPU/identity-failure reaction | Following an intentional 25 ms postponement, observation completed in 27.971–30.190 ms; direct-child kill to reap took 0.313–0.577 ms in these three cases. Stale identity is refused; a reaped PID or partially failed cohort cannot produce a usable observation; a subsequent allocation succeeds | Test-controlled reaction, not a continuously operating watchdog. No maximum scheduling delay, observer-starvation recovery, arbitrary descendant teardown or production budget |
| Inter-sample resident burst | Before and after RSS were both 1,736,704 bytes, below the fixture trigger of 18,513,920; independent peak RSS was 35,291,136 bytes | A concrete missed-burst counterexample. Sampling cannot establish a never-exceed memory guarantee |
| Signed App Sandbox access controls | Unrestricted signed control reads/writes the canary, connects to both fixtures and forks. Sandboxed controls return `EPERM` for private-file/symlink read, outside create, outside-container UDS and IPv4 loopback TCP. With child-local hard/soft NPROC=0, direct fork returns `EAGAIN` | Actual effects in these cases, not universal file/network/process-tree containment or Developer ID/minimum-OS qualification |
| Own-container positive control | Sandboxed helper writes a marker and connects to a host UDS inside its container, while outside-file and TCP access remain denied | Standard App Sandbox permits container writes. This does **not** enforce memory-only spill |
| Real Rust compatibility | A task-only signed worker copy behind the NPROC=0 launcher completes generation (128 rows) and host-mediated Parquet (2,048 exact ID/value pairs), with matching digests, normal exits and endpoint cleanup. Direct reading of that Parquet path is denied to the helper | Same-PID launcher readback is before exec; Rust does not itself report its inherited NPROC value. The C probe demonstrates direct fork denial separately. No general parser/operator, cross-job or production G1 qualification |

Raw observations: [cohorts](../../spikes/adr012-datafusion/evidence/df0-observer-sandbox-macos/cohorts.json),
[reaction](../../spikes/adr012-datafusion/evidence/df0-observer-sandbox-macos/termination.json),
[missed burst](../../spikes/adr012-datafusion/evidence/df0-observer-sandbox-macos/missed-burst.json),
[access controls](../../spikes/adr012-datafusion/evidence/df0-observer-sandbox-macos/sandbox-access.json),
[native jobs and artifact hashes](../../spikes/adr012-datafusion/evidence/df0-observer-sandbox-macos/sandbox-native.json).
Timings are individual development-machine observations, not latency budgets
or statistically qualified tails. The CPU trigger is cumulative time; interval
core-equivalent usage is descriptive, not a CPU-rate control.

## Design implications

- The standard [App Sandbox](https://developer.apple.com/documentation/security/protecting-user-data-with-app-sandbox)
  is distinct from the older experimental `sandbox-exec` write-denial profile.
  This test signs temporary independent app bundles with only the App Sandbox
  entitlement and hardened runtime, verifies their signatures, then requires
  actual positive and denial effects. No private entitlement, privileged helper,
  VM, network entitlement or installed service is used.
- The native authenticated gRPC protocol is unchanged. The host places its
  private endpoint inside the test container because the outside UDS is denied.
  No TCP transport fallback is introduced; loopback TCP is only an access-test
  fixture. Production endpoint placement and signed bundle identity still need
  an approved G1-compatible design.
- Each test refuses an existing container, uses a unique identifier, and checks
  removal of its new container after all processes finish. This is test cleanup,
  not a deployable per-job container/signing strategy. Own-container data,
  inherited descriptors, shared app identity and stale data need separate
  lifecycle and cross-session qualification before adoption.
- App Sandbox's own-container writes, allowed system services and the limited
  network probes prevent a blanket confidentiality claim. IPv6, UDP, DNS,
  delegated services, inherited-handle escape, host death, kill failure and
  signed/minimum-OS behavior remain unqualified. DF-062 still needs independent
  OS-wide observation and the rest of its no-spill controls.
- The usage observer excludes host/source/observer allocations and descendants
  in these cohorts. It does not replace the Darwin socket/secret inspector,
  which remains unimplemented. DF-033 and full DF-064 remain unqualified.

The initial CPU test failed: `ri_user_time` and `ri_system_time` were treated
as nanoseconds, but the Apple Silicon counters are Mach ticks. The probe now
uses `mach_timebase_info` and widened checked arithmetic. A 50 ms
`CLOCK_PROCESS_CPUTIME_ID` workload and independent `getrusage` accounting
calibrate the result. Pinned upstream context is the XNU
[rusage producer](https://github.com/apple-oss-distributions/xnu/blob/f6217f891ac0bb64f3d375211650a4c1ff8ca1ea/osfmk/kern/bsd_kern.c)
and [timeval conversion](https://github.com/apple-oss-distributions/xnu/blob/f6217f891ac0bb64f3d375211650a4c1ff8ca1ea/bsd/kern/kern_resource.c),
not provenance for the installed kernel. Another self-review correction uses
an independent ID/value set oracle for unordered Parquet results rather than
assuming scan order.

## Reproduction and verification

From `spikes/adr012-datafusion`, with an existing evidence directory:

```sh
DF0_WORKER=/absolute/path/to/reviewed/df0-worker \
DF0_OBSERVER_EVIDENCE=/absolute/path/to/existing-directory \
  go test -race -count=1 -v ./internal/ipchost \
  -run '^TestMacOS(CurrentUsageCohorts|ObservedTermination|ObserverCanMissResidentBurst|AppSandboxAccess)$'
DF0_WORKER=/absolute/path/to/reviewed/df0-worker \
  go test -race -count=1 ./internal/... ./cmd/...
go vet ./internal/... ./cmd/...
```

All four focused tests, the two native jobs, the full native spike race suite
and vet passed. Missing `DF0_WORKER` explicitly skips the native subtest;
root skips the unprivileged App Sandbox/child-limit experiment. The recorded
run supplied the worker and ran as non-root. No Rust rebuild was needed;
only task-owned copies were signed, and the source executable hash remained
unchanged. This is not Developer ID, notarization or installed-app evidence.

Eight compiling faults in disposable copies failed their intended assertions:
raw CPU ticks, omitted resident touch, ignored process identity, normal exit
instead of kill, omitted App Sandbox, accidentally enabled network-client
entitlement, omitted direct-fork limit and omitted native launcher limit.
Baseline and restored controls passed; disposable source copies were removed.
The manifest records each exit and target assertion. No timeout or compile
failure was counted as fault detection.

Structured plan, mid-work and final self-review are the disclosed fallback
while independent review is unavailable. This is not independent approval.
Linux host cross-compilation, source/configuration guards, Clang static analysis
and repository hygiene are recorded separately from native execution. Linux
native, Windows, minimum macOS, release signing/packaging, PDF/UI and hosted
checks were not run. Resource policy, full DF0 and LC-GATE remain open.
