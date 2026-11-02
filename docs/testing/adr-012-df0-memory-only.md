<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# DF0: memory-only filesystem policy for the sandboxed macOS worker

**Outcome:** no supported, unprivileged mechanism tested here makes the App
Sandbox worker memory-only. One combination stops **file content** from
reaching disk: App Sandbox plus a zero file-size limit set before exec. It
leaves other channels open, so DF-062 stays **BLOCKED**. The owner decision
is recorded in the [containment ACR](../review/ADR-012-macos-containment-ACR.md#memory-only-filesystem-finding-2026-11-02).

This is isolated feasibility work under the authority the
[observer/App Sandbox experiment](adr-012-df0-observer-sandbox.md) records.
It adds no privilege, entitlement, service, VM, transport or tool, and changes
no production code, dependency or release input.

## The gap it closes and the one it leaves

The earlier experiment showed the App Sandbox worker can write anywhere in its
own container. This experiment asks what can stop that, and checks the answer
with an observer the worker cannot influence.

| Candidate | Result |
| --- | --- |
| App Sandbox alone | Allows everything inside the container. Positive control: a forced DataFusion spill wrote 126 files and the sentinel reached disk |
| App Sandbox + `RLIMIT_FSIZE=0`, `SIGXFSZ` ignored | **Stops file content.** `write` and `ftruncate` return `EFBIG`; a forced spill fails after creating one empty file. Does **not** stop file or directory creation, renames or extended attributes |
| `sandbox-exec` write-denial profile, then App Sandbox | **Cannot be combined.** App Sandbox stops an already-sandboxed process with `SIGTRAP` before it runs |
| `sandbox-exec` profile alone ([earlier](adr-012-df0-no-spill.md)) | Denies all writes, but the tool is deprecated and gives none of App Sandbox's read and network restrictions |

The policy under test is App Sandbox (standard entitlement only) plus
child-local `RLIMIT_FSIZE=0`, an ignored `SIGXFSZ`, `RLIMIT_CORE=0` and
`RLIMIT_NPROC=0`. The existing native launcher installs all of them before
exec and reads them back, and an ignored signal survives exec. `RLIMIT_FSIZE`
limits regular files only, so the gRPC Unix socket keeps working. The worker's
stdout and stderr must be pipes: a regular-file stderr would fail too.

## What ran

`TestMacOSFileGrowthLimitChannels` runs a C helper three ways in a unique test
container and tries each way of putting data on disk. `TestMacOSMemoryOnlyWorker`
runs a freshly built Rust worker, as a signed sandboxed copy behind the
launcher, over the 4096-row sentinel fixture:

| Scenario | Policy | Observed |
| --- | --- | --- |
| Spill positive control | App Sandbox only | Completed; 126 spills, 32,162,448 bytes; sentinel found in spill files during the run; FSEvents saw the files modified |
| Forced spill | Policy | `RESOURCE_LIMIT` ("source query file writes are not permitted") after 2 of 17 source reads; one empty spill file created and removed; no content |
| Memory pressure, spill off | Policy | `RESOURCE_LIMIT`, no files |
| Normal, cancel, recovery | Policy | Exact 4096 descending rows; `USER_CANCELLED`; exact rows again |
| Kill after 15 of 17 source reads | Policy | `SIGKILL`, no files |
| Crash (`SIGABRT`) after 15 of 17 source reads | Policy | Crash report written outside the container and a CrashReporter plist inside it; neither held the sentinel |

Three independent checks run after every scenario:

1. **FSEvents** over the container. A barrier file before and after each run
   orders the events, and a dropped-event flag fails the observation. The
   policy runs must show no content modification by the worker.
2. **A host scan** of every regular file and extended attribute in the
   container, without following symlinks. The sentinel must not appear in
   content, attributes or names, and no unexpected non-empty file may remain.
   The next scenario re-checks for leftovers before it starts.
3. **The crash report.** For the crash, the test waits a bounded time for
   ReportCrash's report on that PID, scans its bytes and deletes it. A missing
   report fails the observation rather than passing it.

Raw records: [channels](../../spikes/adr012-datafusion/evidence/df0-memory-only-macos/growth-channels.json),
[worker scenarios](../../spikes/adr012-datafusion/evidence/df0-memory-only-macos/memory-only-worker.json),
[fault seeds](../../spikes/adr012-datafusion/evidence/df0-memory-only-macos/fault-seeds.json)
and the [manifest](../../spikes/adr012-datafusion/evidence/df0-memory-only-macos/environment.json),
which binds sources, SDK headers and the worker by SHA-256.

## What stays open

These are why the policy is content-only, not memory-only:

- **Names, empty files, directories and extended attributes.** Under the limit
  the helper stored its marker in an attribute value and in a file name.
  DataFusion's failed spill created and removed an empty temporary file, and
  created a directory whose persistence was not observed.
- **Crash reports are not guaranteed.** ReportCrash stopped writing reports
  for a process name that had crashed many times this session; naming each
  test's executable uniquely made all 8 later crashes report. A production
  worker with a fixed name would be throttled the same way.
- **Writes other processes make on the worker's behalf.** ReportCrash wrote
  the crash report and the in-container plist. Crash reports hold backtraces,
  not heap, so finding no sentinel in one is a weak check; they can also leave
  the machine if the user shares analytics with Apple. `containermanagerd`
  rewrites the container's metadata. `cfprefsd` could write preferences.
- **fseventsd's own record** of file names, swap, hibernation and the unified
  log were not observed. FSEvents watched only the container; an
  unprivileged volume-wide watch, as a possible stand-in for the OS-wide
  observer DF-062 lacks, was not tested.
- **The error code (fixed, owner condition 1).** The first build reported the
  `EFBIG` spill failure as `INPUT_INVALID` with the OS error text. The OS error
  sits inside an `io::Error` whose `source()` skips the wrapped error, so the
  worker now walks the chain with `get_ref()` and reports `RESOURCE_LIMIT` with
  a constant detail. A Rust unit test models the exact native shape; the
  native forced-spill run confirms it, and reverting the mapping is caught by
  both.

The threat each closes is different. Content denial covers an accidental
plaintext spill by trusted engine code. It does not contain a worker that
deliberately encodes data into names or attributes.

## Owner conditions for option (a)

On 2026-11-02 the owner kept (d) in force and approved (a) as the target once
five conditions pass. Status is tracked in the
[ACR](../review/ADR-012-macos-containment-ACR.md#memory-only-filesystem-finding-2026-11-02).

**Condition 2: fresh folder per job, wiped afterwards.** `Container.Do`
records a baseline of the container's `Data` tree:
- entry types and permissions;
- symlink targets;
- file content digests;
- attribute names with value digests.

It runs each job alone in a fresh, empty job folder. After the worker has
exited, on every outcome, it sweeps the whole tree back to the baseline:
- removes added entries and attributes, anywhere under `Data`, not only in the
  job folder;
- restores modes and clears immutable flags;
- never follows a symlink.

It then compares the result with the baseline. A container it cannot restore
is marked unclean, and further jobs are refused. Reports hold counts and the
system error only; names could carry data.

- **Reach:** a sandboxed helper could create entries in `Data`, and create,
  tag and `chmod` entries below it. It could not touch the container root or
  tag `Data` itself. The sweep therefore covers `Data`.
- **Native:** every worker scenario ran through `Container.Do`, and every sweep
  was clean. After the zero file-size helper run, the names and attribute
  values carrying its marker were gone. After the crash, the next sweep
  removed the delegated CrashReporter plist.
- **Unit:** the sweep restored the baseline after names, attribute values, a
  `chmod 000` directory, an immutable file, a mode change on a baseline
  directory, and symlinks to an outside file and an outside directory. The
  outside entries were left untouched. Further tests cover a changed
  baseline, which refuses the next job; a host that died mid-job, which is
  swept from a saved baseline; jobs running one at a time; and failure text
  that names nothing.
- **Seeds:** seven compiling faults were each caught. One first form that did
  not compile was replaced. The seeds skipped attribute removal, followed
  symlinks, swept only after a completed job, forced the clean verdict, put
  the path in the failure text, and skipped mode restoration or flag
  clearing.
- **Constraint for the owner:** every worker signed with one bundle identifier
  shares one container, so jobs on it run one at a time. That conflicts with
  DF-064's concurrent 2/4/8-worker cohorts. The options are:
  - serialize jobs;
  - give each concurrent job a separately signed identity and container, which
    is not a deployable design today;
  - accept per-job-folder removal while jobs overlap, and sweep only when none
    is in flight.

## Decision for the owner

Recorded in the [ACR addendum](../review/ADR-012-macos-containment-ACR.md#memory-only-filesystem-finding-2026-11-02).
In short:

- **(a)** accept content-only protection for the trusted worker as an
  explicit, recorded relaxation;
- **(b)** replace App Sandbox with a custom deprecated profile;
- **(c)** design separate-user, privileged or VM isolation, which needs new
  authority;
- **(d)** stay BLOCKED and defer cutover.

Accepted requirements stay in force until the owner chooses.

## Verification

From `spikes/adr012-datafusion`, with a worker built from this revision:

```sh
DF0_WORKER=/absolute/path/to/df0-worker \
DF0_MEMORY_ONLY_EVIDENCE=/absolute/path/to/existing-directory \
  go test -race -count=1 -v ./internal/ipchost \
  -run '^(TestMacOSFileGrowthLimitChannels|TestMacOSMemoryOnlyWorker|TestFSEventsDropFlagsFailTheObservation)$'
```

The full spike race suite and `go vet` passed with the same worker. The tests
skip as root and skip the worker scenarios without `DF0_WORKER`; neither is
qualification.

Ten compiling seeds were each caught by a named assertion; two first forms
that did not compile were replaced and are recorded as not counted. One seed
removed the file-size limit **and** disabled the launcher's readback check:
the content check alone still failed the forced-spill run, so detection does
not rest on the worker's own report.

Fixes made during the work:

- A new FSEvents stream delivered events from just before it started; a start
  barrier now discards them.
- The container manager's metadata rewrites were counted as worker writes;
  only `Data/` is now treated as worker-writable.
- Kill and crash first fired at the second source read, before the query had
  done much; they now fire after 15 of 17 reads have been served.

Not run: native Linux, Windows, minimum macOS, Developer ID or installed-app
signing, other DataFusion operators, and independent review (structured
self-review and an advisor review were the fallback).
