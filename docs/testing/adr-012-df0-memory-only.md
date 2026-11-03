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

- **Names, empty files, directories and extended attributes, during a job.**
  Under the limit the helper stored its marker in an attribute value and in a
  file name. DataFusion's failed spill created and removed an empty temporary
  file and created a directory. The condition 2 sweep removes all of these
  after each job; they persist until the next start only if the host dies.
- **Crash reports hold register values** (condition 3 above), so the "no
  sentinel in the report" check is weak in both directions.
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
- never follows a symlink. It walks directory handles opened without following
  symlinks and changes or removes entries only through handles, so a process
  still running in the container cannot redirect it outside `Data` (SEC-001).
  Restoring a closed directory's permissions by name uses a trailing slash
  and `AT_SYMLINK_NOFOLLOW_ANY`, so a hard link or symlink swapped in is
  refused; entries open with `O_NONBLOCK`, so a FIFO cannot block the host.
  A baseline entry is changed only after its inode matches the baseline on
  root's current device.

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
- **SEC-001 hardening:** tests swap an added and a baseline directory for a
  symlink to an outside directory between the sweep's look at the entry and
  its next step; replace a baseline file with a hard link to an outside file
  that has another mode and an extra attribute; swap an unreadable directory
  for a hard link or a symlink; leave an immutable FIFO; leave a file the
  worker made unreadable and immutable; and nest folders past the depth
  bound (64). The outside entries stay untouched, the FIFO does not block,
  and the last two fail closed. The earlier
  path-based sweep deleted outside files through the swap, changed the
  outside file's mode through the hard link, and had neither fail-closed
  limit. The no-follow opens and the identity checks each stop the swap
  alone; seeds that removed one showed the other catching it, and removing
  both was caught. On the real container the native sweeps reported the same
  counts as before.
- **Seeds:** seven compiling faults were each caught against the earlier
  path-based sweep; five were re-run against the handle-based sweep and
  caught again (see the seed record). One first form that did
  not compile was replaced. The seeds skipped attribute removal, followed
  symlinks, swept only after a completed job, forced the clean verdict, put
  the path in the failure text, and skipped mode restoration or flag
  clearing.
- **Cross-process lock (added later the same day).** Two host processes,
  such as two app instances, share one container. `Container.Do` and `Sweep`
  now hold an exclusive `flock` on the slot's lock file, in a folder only the
  host's user can reach, from before the job folder is made until the sweep
  ends. A slot another process holds is refused at once and never marked
  unclean. Under the lock, a container left changed by an earlier holder is
  swept before the job, and the result reports that sweep's counts.
- **Limits:** the lock lasts as long as the host's descriptor, so a host that
  dies while its worker runs releases it, and another host can start a job
  beside the orphaned worker until that worker exits. ReportCrash writes its
  in-container plist asynchronously. If that write lands between a sweep's
  walk and its re-check, the container is marked unclean and refuses jobs
  until it is swept again; a plist that lands after the sweep is now swept
  before the next job. That is an availability limit; the comparison stays
  strict.
  The sweep fails closed, and so marks the container unclean, on an entry it
  cannot handle through a handle: a file the worker made unreadable and
  immutable, a baseline file it made unreadable, or a tree deeper than 64
  levels or larger than 131,072 entries (the entry cap is untested).
  `AT_SYMLINK_NOFOLLOW_ANY` is verified only on macOS 27.2; a kernel that
  ignored the flag instead of rejecting it would follow a swapped symlink,
  so condition 4's macOS 13 run must include
  `TestContainerSweepIgnoresASwappedDirectory`. Identity now includes the
  inode, so a system service that rewrites a baseline file atomically, even
  with the same content, marks the container unclean; the native runs did
  not hit this.
- **Constraint for the owner:** every worker signed with one bundle identifier
  shares one container, so jobs on it run one at a time. That conflicts with
  DF-064's concurrent 2/4/8-worker cohorts. The options are:
  - serialize jobs;
  - give each concurrent job a separately signed identity and container, which
    is not a deployable design today;
  - accept per-job-folder removal while jobs overlap, and sweep only when none
    is in flight.

**Condition 3: crash reports.** ReportCrash writes a report outside the sandbox
for any process that dies from a fatal signal. Automatic submission to Apple
is on for this machine. `TestMacOSCrashReportOptions` measured what a report
holds and whether a process can avoid one, using a sandboxed helper with its
own name:

- **Reports carry register values.** A known 8-byte pattern loaded into ten
  registers before a trap appeared in the report. The report holds the
  crashing thread's register set (up to 29 general-purpose registers of 8
  bytes on arm64), so a crash can leak that much of whatever the worker was
  handling, beyond the backtrace.
- **A process can avoid the report.** With fatal-signal handlers that call
  `_exit` first, a trap and an abort each ended with status 70 and no report.
  Plain crashes under the same name were reported both before and after these
  runs, so the absence is not throttling.
- **Reports are not reliable either way.** ReportCrash stopped writing reports
  for a name that had crashed many times, so deleting reports afterwards is
  neither complete nor timely when submission is on.

Options for the owner. The owner chose (ii) with (iv); see
[Owner decision](#owner-decision).

| Option | What it does | Cost |
| --- | --- | --- |
| (i) Accept reports | Keep the default and record the register-sized residual | Reports may leave the machine with submission on |
| (ii) Panic hook in safe Rust | Exit the worker on a Rust panic before it aborts | Covers panics only; native faults still crash |
| (iii) (ii) plus a fatal-signal handler | Exit before any fatal signal can produce a report | Needs `unsafe` async-signal-safe code in the worker and its own review; guard and resource exceptions and memory-pressure kills are not covered |
| (iv) Deployment guidance | Ask managed deployments to turn off crash-report submission | Outside the app's control |

**Condition 3 passed (accepted by the owner 2026-11-02): panic hook and deployment guidance.** At start-up
the worker installs a panic hook (safe Rust, in `main.rs`). At the first panic
it prints one line with the source location and exits with status 70, before
unwinding starts. It never prints the panic message, which can hold row
values. A second thread that panics meanwhile waits for the exit.

`TestMacOSMemoryOnlyWorker` checks it against the signed, sandboxed worker
with test-only faults inside the query task. Each fault panics with a row
holding the sentinel in its message:

- `panic`, an ordinary panic, and `panic_abort`, a panic whose unwinding
  runs a destructor that panics again, each ended with exit 70, no crash
  report, and no sentinel in anything the host received.
- A host-sent SIGABRT to the same worker name produced a report both before
  (`crash`) and after (`crash_trailing`) the panic runs, so the missing
  reports are not throttling. Native faults keep reporting, as chosen.
- **Without the hook** (seed P1), `panic_abort` died by SIGABRT and an
  instrumented run observed a crash report for its PID. The plain panic
  reached the host as an `INTERNAL` failure whose text included the panic
  message, a path to the host that the hook also closes.
- Seeds that print the message (P2) or return without exiting (P3) were also
  caught. No seed reaches the "no crash report" assertion on its own: each
  one that lets the abort happen first changes the exit status or leaks the
  message. The instrumented P1 run is the evidence that the abort path
  reports.

Limits: the hook covers panics only. Allocation failure, stack overflow and
other native faults abort without running it and still produce reports. The
hook exits through `std::process::exit`, which runs C `atexit` handlers while
other threads keep running; a crash during that teardown would be reported.
None was seen. With the hook, the destructor's second panic never runs,
because the first panic exits before unwinding. The location line names the source
file: for a panic inside a dependency crate that is an absolute path on the
build machine, including its home folder, unless release builds remap source
paths.

Draft deployment guidance, to move into `docs/INSTALL.md` when the
DataFusion worker ships:

> If the analytics worker crashes from a native fault, macOS writes a crash
> report to `~/Library/Logs/DiagnosticReports`. The report can include a
> few dozen bytes of the data the worker was handling. If macOS is set to
> share analytics with Apple, the report is sent automatically. On managed
> Macs, turn off automatic submission with a Restrictions profile
> (`com.apple.applicationaccess`) that sets `allowDiagnosticSubmission` to
> `false` (macOS 10.13 and later). On an unmanaged Mac, turn off
> **Share Mac Analytics** and **Share with app developers** in System
> Settings > Privacy & Security > Analytics & Improvements. These settings
> stop sending; reports are still written locally.

Sources, checked 2026-11-02: the managed-profile key, its default (`true`) and
platform support are from
[Apple's Restrictions payload reference](https://developer.apple.com/documentation/devicemanagement/restrictions);
the System Settings path is from
[Share analytics information from your Mac with Apple](https://support.apple.com/guide/mac-help/share-analytics-information-mac-apple-mh27990/mac).

**Condition 4: signed, installed app on the oldest supported macOS.**
**BLOCKED.** This machine has no code-signing identity (no Developer ID
certificate) and no macOS 13 host. Ad-hoc signed bundles in a temporary
folder are not an installed app, and a cross-compile is not a target run.

**Condition 5: volume-wide watch without admin rights.**
`TestMacOSVolumeWideFSEvents` watches the whole boot volume with an
unprivileged FSEvents stream. Events under task-owned folders are reported in
full. Everything else is counted by its first path component only, so no other
application's paths are recorded.

- It saw marker writes in `/private/tmp`, the user temp folder, the home folder
  and the sandboxed container, with nothing dropped.
- On an idle machine, 20,000 create/delete pairs elsewhere on the volume in
  about 1.3 seconds caused no drop, and a marker written afterwards was seen.
  With every CPU core busy, fseventsd dropped one or two events in most such
  runs, and the observer reported each drop. **For the owner:** a production
  observation that fails on any drop would then fail jobs on a busy machine.
- A consumer that stopped reading made fseventsd drop events, and the observer
  reported it. This is the positive control for drop detection. A dropped
  event fails any observation that relies on the stream.

It is a candidate stand-in for the root-only tracing DF-062 lacks, not a
replacement verdict. It shows paths, not processes or contents. It covers one
volume, and fseventsd keeps its own history of names. Its positive controls are
direct writes. A write another process makes on the worker's behalf outside
the container, such as ReportCrash's report, was not checked through it.

## Owner decision

**Decided 2026-11-02:** (d) stays in force now, and (a) is approved as the
target once the [five conditions](#owner-conditions-for-option-a) pass. (a)
is not adopted, and DF-062 stays BLOCKED until the owner confirms that the
conditions have passed. Recorded in the
[ACR addendum](../review/ADR-012-macos-containment-ACR.md#memory-only-filesystem-finding-2026-11-02).
The options were:

- **(a)** accept content-only protection for the trusted worker as an
  explicit, recorded relaxation;
- **(b)** replace App Sandbox with a custom deprecated profile;
- **(c)** design separate-user, privileged or VM isolation, which needs new
  authority;
- **(d)** stay BLOCKED and defer cutover.

**Choices for the conditions (2026-11-02).** Conditions 1 and 3 have passed;
the owner accepted condition 3 after it was implemented, as described below.

- **Condition 2:** one signed worker copy per concurrency slot, each with its
  own bundle identifier, so each slot gets its own container and sweep. DF-064
  cohorts then run in parallel. Each container still needs a cross-process
  lock: an `flock` on one lock file per slot in the host's own Application
  Support folder, which the kernel releases if the app crashes and the worker
  cannot reach. The lock is built and tested; the per-slot signed copies and
  native cohort tests are not.
- **Condition 3:** option (ii), a panic hook in safe Rust, plus (iv),
  deployment guidance. Native faults keep producing reports.
- **Condition 4:** deferred to release qualification. (a) cannot be adopted
  before it passes.
- **Condition 5:** the watch replaces OS-wide tracing once one more test shows
  it sees a write that a macOS helper makes outside the container on the
  worker's behalf. That test uses a separate test-only helper signed with the
  worker's exact entitlements, which asks `cfprefsd` and then Launch Services
  to write for it, so the shipped worker gains no code. Not built yet.

## Hardening after the security scan (2026-11-02)

A scan of the spike found no Critical or High issues. Its five Low findings
are fixed; SEC-001 is described above.

- **SEC-002, processes outliving the job.** On macOS the worker now sets
  `RLIMIT_NPROC` to 0 itself, before it reads input, so the limit no longer
  depends on the test launcher. Teardown stops and kills the worker's
  descendants while it runs, kills its group before reaping it when it exits on
  its own, and a kqueue watch reports any process it created (including through
  `posix_spawn`). Either case fails the job as `worker_forked`. A child that
  left the group and outlived the worker is reported on macOS but not killed.
  On Linux it is neither found nor reported, and the Linux paths were compiled
  but not run.
- **SEC-003, decompression on the host.** The Arrow verifier decodes through an
  allocator capped at 64 MiB, because a compressed buffer states its own decoded
  size and the decoder allocates that much first.
- **SEC-004, the DF0-B import.** Non-regular files are refused before they are
  opened, and JSON is read with a 256 MiB cap. DataFusion still reopens CSV and
  Parquet sources by path.
- **SEC-005.** `golang.org/x/net` is now v0.62.0 in the app and the spike.

The worker built with these changes has SHA-256
`d4da6f3759a29c8ce706fddc076d179b0fafe362165195054468c1dc872be9b9`. The four
observation files were regenerated with it, after the flake fix below, and
their outcomes match the earlier run. The full race suite passed four times in
a row with it. The build SBOM still describes source snapshot `8746d38`:
regenerating it needs the pinned validator environment and SPDX resources,
which are not on this machine and would need the owner's approval to install.

`TestMacOSVolumeWideFSEvents` failed in three of four full race runs. With
every CPU core busy it failed in three of four runs alone, for two reasons:

- The load phase allows drops but still required every event, and fseventsd
  dropped one or two events whenever the machine was saturated.
- When the stalled-consumer phase made fseventsd drop events, the drop was
  reported on an event outside the reported folders. The probe only counted
  such events, so a lost closing barrier looked like silent loss.

The probe now announces such a drop at once, without its path. A barrier
that is not seen after a reported drop is followed by a fresh one, and the
load phase requires every event only when no drop was reported. Each event
that arrives with a path outside the reported folders now fails the test,
whether or not drops occurred. With every core busy, six of six runs passed.
The retry only after a reported drop has no fast test (its wait is ten
seconds); a probe that hides drops still fails at the barrier.

## Verification

From `spikes/adr012-datafusion`, with a worker built from this revision:

```sh
DF0_WORKER=/absolute/path/to/df0-worker \
DF0_MEMORY_ONLY_EVIDENCE=/absolute/path/to/existing-directory \
  go test -race -count=1 -v ./internal/ipchost \
  -run '^(TestMacOSFileGrowthLimitChannels|TestMacOSMemoryOnlyWorker|TestFSEventsDropFlagsFailTheObservation|TestFSEventsObserverRecordsDropNotices|TestMacOSVolumeWideFSEvents|TestMacOSCrashReportOptions|TestContainer.*)$'
```

The full spike race suite and `go vet` passed with the same worker. The tests
skip as root and skip the worker scenarios without `DF0_WORKER`; neither is
qualification.

For the memory-only and container work, forty-four compiling seeds were each
caught by a named assertion (the SEC-002 to SEC-005 seeds are recorded in their
commit messages); four
first forms that did not compile were replaced and are recorded as not
counted. The seed record states which source version each group ran against. One seed
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
