<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# DF0: native macOS limit primitives

These isolated, synthetic probes expose insufficient containment mechanisms
before a production launcher is selected. They are not DataFusion enforcement,
aggregate admission, installed-app or minimum-macOS qualification. DF-033 and
full DF-064 remain unqualified; full DF-062 remains BLOCKED. No accepted
requirement, production policy, entitlement or dependency changes here.

## Observations

The [environment and source hashes](../../spikes/adr012-datafusion/evidence/df0-limits-macos/environment.json)
freeze main `6102a28`, macOS 27.2 arm64, Go 1.27.3 and the exact probe source.
The [primitive records](../../spikes/adr012-datafusion/evidence/df0-limits-macos/primitives.json)
were produced by a native race-enabled run:

| Probe | Observed result | Supported conclusion |
| --- | --- | --- |
| Allocation control | Touching 128 MiB grew peak RSS from 43,696,128 to 177,913,856 bytes | The resident-growth detector sees the controlled allocation |
| Address-space growth | Set/read back a limit 8 MiB above current VM; a new 128 MiB mapping failed with `ENOMEM` | The tested address-space limit restricts new VM mappings |
| Existing mapping | Under the same kind of limit, a new mapping was denied in that process, but touching its pre-reserved mapping grew peak RSS from 44,089,344 to 178,962,432 bytes | An 8 MiB VM-growth allowance is not an 8 MiB resident-growth budget |
| CPU limit | A shell builtin loop, after exec with default signal dispositions, terminated with `SIGXCPU` under a one-second soft/two-second hard per-process limit | A per-process CPU-time control works for this fixture; no CPU-rate or aggregate guarantee follows |
| Parent and child CPU | Each inherited/set the one-second soft limit; both completed about 0.6 CPU seconds, totaling 1,203,635,000 ns | Both stayed below their own soft limit and rejected any observed `SIGXCPU`; no shared CPU-time budget followed |
| Group escape | A member joined a controlled group, called `setsid`, then answered a handshake after the old group's control process terminated from its group `SIGKILL` | Signaling a process group does not contain members that leave it |

The [group record](../../spikes/adr012-datafusion/evidence/df0-limits-macos/process-group.json)
contains ephemeral IDs, not secrets or user data. Both group processes are
owned directly by the test and waited for on normal and failure paths. No
orphan or privileged observer is needed. All children have bounded parent
contexts; the Go helper disables core dumps before probing limits. Parent
limits are never changed. These observations do not rule out useful absolute
VM limits in other process layouts; they disprove equating a VM-growth
allowance with a resident-growth allowance.

An initial Go CPU-loop probe timed out rather than dying from `SIGXCPU`.
Changing the loop from system calls to arithmetic did not fix it. Go's
[documented signal behavior](https://pkg.go.dev/os/signal@go1.27.3#hdr-Default_behavior_of_signals_in_Go_programs)
catches that signal without terminating by default. The final positive control
execs the shell fixture; it requires `SIGXCPU` specifically and rejects a
parent-timeout `SIGKILL`. This does not establish the Rust worker's signal
behavior, nor prove fatal hard-limit behavior for a process that handles the
soft-limit signal. No timeout was relabelled as successful enforcement.

## Absolute limits across exec and child-creation follow-up

The 2026-11-01 follow-up uses a standalone C image before a fresh `exec`,
avoiding Go runtime signal/mapping assumptions. The
[environment, source hashes and fault results](../../spikes/adr012-datafusion/evidence/df0-exec-limits-macos/environment.json)
identify the run on macOS 27.2 arm64, the unchanged reviewed Rust worker and
the source baseline. This is development-machine evidence, not minimum-OS or
installed-app qualification.

| Probe | Observed effect | Limit of the evidence |
| --- | --- | --- |
| 128 MiB allocation control | Every 4 KiB position was written/read, checksum 2,949,120; peak RSS exceeded 128 MiB | Detects actual touching rather than only virtual reservation |
| Absolute AS ceilings of 64 MiB, 512 MiB and 8 GiB | Each setter returned `EINVAL`; the C image already had about 500 GB of virtual address space with about 1.7 MB RSS | Those requested ceilings were not installed; no enforcement success or Rust execution follows |
| AS ceiling at current VM + 64 MiB | Soft/hard readback survived exec; the new 128 MiB mapping failed with `ENOMEM` | A useful VM-mapping control, with a roughly 500 GB absolute ceiling; not an economical RSS cap |
| Child-local hard/soft `RLIMIT_NPROC=0` | Both values survived exec; direct `fork` and `posix_spawn` returned `EAGAIN` (35). Both succeeded and were reaped in the unrestricted control | Direct creation by this non-root real UID only; no delegated XPC, privilege-transition or arbitrary descendant-tree guarantee |
| Real authenticated Rust worker under NPROC=0 | Generated and independently verified all 128 rows; exit 0, positive child RSS and endpoint cleanup | Launcher readback plus successful Rust execution; the worker does not itself report its inherited NPROC limit. Other query/parser operations are not qualified by this job |
| Real-worker launch requesting 8 GiB AS, alone or combined | Setup failed before Rust exec: exit 78, zero rows, host outcome `worker_exit_before_stream`, endpoint removed | Local setup refusal, not typed gRPC `RESOURCE_LIMIT`. The combined case stops at AS failure and does not establish both controls |
| Subsequent unrestricted Rust job | All 128 rows verified, exit 0 and cleanup | Demonstrates recovery after these setup refusals |

Raw records: [AS](../../spikes/adr012-datafusion/evidence/df0-exec-limits-macos/absolute-exec.json),
[child creation](../../spikes/adr012-datafusion/evidence/df0-exec-limits-macos/child-creation.json),
[Rust jobs](../../spikes/adr012-datafusion/evidence/df0-exec-limits-macos/native-worker.json).
Parent AS/NPROC limits remained unchanged in the corresponding synthetic tests.
These numbers are observations, not approved product budgets. A different OS
result must be investigated rather than relabelled as equivalent enforcement.

Reproduce from the spike directory, with an existing absolute evidence directory:

```sh
DF0_WORKER=/absolute/path/to/reviewed/df0-worker \
DF0_EXEC_LIMIT_EVIDENCE=/absolute/path/to/existing-directory \
  go test -race -count=1 -v ./internal/ipchost \
  -run '^TestMacOS(AbsoluteExecLimits|ChildCreationLimit|NativeWorkerExecLimits)$'
```

Without `DF0_WORKER`, the native-worker test explicitly skips; the two synthetic
tests still run. NPROC tests skip as root because that would test a different
privilege contract. Each probe is compiled with the existing `xcrun clang`,
`-std=c11 -Wall -Wextra -Werror`. The C source is neither cgo nor linked into
Rust. Its fixed 128 MiB mapping is checked before access and unmapped once;
volatile bounded byte accesses make touching observable. A forked child only
calls `_exit`; the parent waits for exact owned children, retrying interrupted
waits. Probe groups have a ten-second timeout, which bounds these controlled,
non-escaping fixtures rather than containing arbitrary process trees. Core
dumps are disabled only in the child. The test launcher preserves the worker
PID and authenticated stdin; it does not change the production launch path.
The existing macOS allowance for `__CF_USER_TEXT_ENCODING` remains the only
permitted environment key.

Targeted checks and the full native spike Go race suite passed with the
hash-bound, cached worker. Vet, source-pin guards, native-source tool tests,
the existing default/DuckDB analytics tests and repository hygiene gates also
passed. Seven compiling faults in disposable copies were rejected by the
intended assertions: omit AS installation, corrupt resident values, request a
wrong fixed budget, omit NPROC installation, skip fork, skip spawn, and execute
Rust after setup refusal. Clean and restored controls passed, and fault copies
were removed. Structured mid-work review added exact-budget checks and NPROC
readback/effect after exec, rather than accepting configuration alone.

Rust was not rebuilt; no Rust source, dependencies, unsafe surface or FFI changed.
Linux native execution, Windows, minimum-OS/signed-app, production packaging,
PDF and UI gates were not run for this isolated follow-up. Independent review
remains unavailable; structured plan/mid/final self-review is the disclosed
fallback. No sampling, RSS, CPU-rate, aggregate enforcement or confidentiality
gate is closed by these tests. The current containment ACR remains unresolved.

## SDK and upstream cross-check

The installed SDK aliases `RLIMIT_RSS` to `RLIMIT_AS`. Its `libproc.h`
declares resource inspection, but the inspected headers do not declare
`proc_set_cpumon_params_fatal`. Its upstream implementation includes
`libproc_internal.h`, which is absent from the installed SDK. That is not proof
that every CPU-control API is unavailable.

The SDK **does** declare `task_set_phys_footprint_limit`. A separate
[standalone C probe](../../spikes/adr012-datafusion/tools/macos_footprint_probe.c)
compiled against it. A normal unprivileged call to `task_info` succeeded, while
setting the current task's footprint limit returned `KERN_NO_ACCESS` (8), as
[recorded](../../spikes/adr012-datafusion/evidence/df0-limits-macos/footprint.json).
The probe does not allocate after that call, grant privileges, load private
symbols or link into either worker or Go. A declared API is not permission to
use it; a physical-footprint measure is also not automatically RSS.

For context, Apple XNU revision
`f6217f891ac0bb64f3d375211650a4c1ff8ca1ea`:

- [Address-space implementation](https://github.com/apple-oss-distributions/xnu/blob/f6217f891ac0bb64f3d375211650a4c1ff8ca1ea/bsd/kern/kern_resource.c)
  sets the VM-map size limit and refuses an invalid bound.
- [Exec implementation](https://github.com/apple-oss-distributions/xnu/blob/f6217f891ac0bb64f3d375211650a4c1ff8ca1ea/bsd/kern/kern_exec.c)
  applies the process AS limit to the new map.
- [Fork/spawn implementation](https://github.com/apple-oss-distributions/xnu/blob/f6217f891ac0bb64f3d375211650a4c1ff8ca1ea/bsd/kern/kern_fork.c)
  checks the non-root real-UID count against the calling process's NPROC limit.
- [Footprint-limit implementation](https://github.com/apple-oss-distributions/xnu/blob/f6217f891ac0bb64f3d375211650a4c1ff8ca1ea/osfmk/kern/task.c)
  checks privilege before setting the limit.
- [Memorystatus implementation](https://github.com/apple-oss-distributions/xnu/blob/f6217f891ac0bb64f3d375211650a4c1ff8ca1ea/bsd/kern/kern_memorystatus.c)
  gates the examined memory-limit commands on root/private entitlement.
- [CPU-monitor implementation](https://github.com/apple-oss-distributions/xnu/blob/f6217f891ac0bb64f3d375211650a4c1ff8ca1ea/libsyscall/wrappers/libproc/libproc.c)
  uses an internal process-policy interface.

These are pinned upstream references, not proven provenance for the installed
kernel. Hashes are recorded without vendoring Apple source. No unprivileged
host-wide RSS/CPU/process-tree solution is qualified by this slice; the
observations do not prove that such a solution is universally impossible.

## Reproduction and assurance

From the spike directory on native macOS:

```sh
DF0_LIMIT_EVIDENCE=/absolute/path/to/primitives.json \
DF0_GROUP_EVIDENCE=/absolute/path/to/process-group.json \
  go test -race -count=1 -v ./internal/ipchost \
  -run 'TestMacOS(LimitPrimitives|ProcessGroupMemberCanEscape)$'
xcrun clang -std=c11 -Wall -Wextra -Werror tools/macos_footprint_probe.c \
  -o /absolute/path/to/df0-footprint-probe
/absolute/path/to/df0-footprint-probe
```

The evidence paths are optional. A failed primitive subtest prevents writing
its aggregate evidence file. Successful group evidence is separate; the
runner's overall exit status still matters. On another OS/kernel, report a
changed result or failure rather than silently changing the expectation.
The C probe prints the actual API status; exit zero means its control read and
output succeeded, not that memory enforcement succeeded. An unexpected API
success requires fresh enforcement qualification.

Three repeated race-enabled probe runs, a separate recorded run, and the full
spike Go race suite with the unchanged reviewed Rust worker passed. That full
suite includes the earlier 60 native worker scenarios; the new capability
probes do not execute DataFusion. Vet and a Linux host cross-compile passed;
Linux native execution and Windows are NOT-RUN. Rust was not rebuilt, because
no Rust source or dependency changed. Root PDF, packaging, frontend and hosted
checks were not run for this isolated test/documentation slice.

Four compiling faults in disposable copies were detected after a passing
baseline: omit AS installation, omit resident touching, omit CPU-limit
installation, and omit session escape. A further injected `SIGXCPU` failed the Go
parent/child probe after adversarial review exposed its default-ignore risk;
the detector now rejects a received signal instead of silently ignoring it.
In particular the CPU-limit omission timed out and failed, and the AS fault
failed readback/denial controls. Temporary fault
copies were removed. Structured plan, mid-work and final self-reviews are the
disclosed fallback while independent reviewer capacity is unavailable; this is
not independent review or security clearance.

The [containment ACR](../review/ADR-012-macos-containment-ACR.md) keeps the
remaining decision explicit. The [roadmap](../ROADMAP.md) sequences the other
DF0 gates and eventual owner-approved cutover. Do not reinterpret these green
counterexample tests as green resource-governance acceptance tests.
