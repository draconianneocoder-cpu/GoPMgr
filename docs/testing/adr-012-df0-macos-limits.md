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
  sets the VM-map size limit.
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
