<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# DF0-D2: native macOS no-spill experiment

This isolated slice investigates the confidentiality part of DF0-D2 after
[host-mediated reads](adr-012-df0-c2.md). It does not complete DF-062, select a
production sandbox, or authorize production integration or DuckDB removal.

## Scope and controls

The existing source context defaults to a 64 MiB query pool and disabled disk
manager. Experimental source requests can choose a pool between 64 KiB and
64 MiB, with zero retaining the default. Invalid limits are typed input errors.
These are fixture limits, not production admission or RSS limits. The worker
preserves structured DataFusion errors until the protocol boundary: resource
exhaustion and permission denial return `RESOURCE_LIMIT` with constant details.

The pressure fixture adds an explicit physical full sort after the bounded
optimized input. A logical sort was insufficient: the optimizer inferred a
TopK, which cannot spill. The positive control must therefore demonstrate
nonzero typed spill counters and a sentinel in an actual spill file. The
source has 4096 rows, exact int64 IDs above 2^53 and synthetic string payloads;
no user project or SQLCipher data is involved.

An explicit `source_spill_control` fault and a scratch directory enable the
synthetic positive control only. A directory without that opt-in is rejected
before query execution. Ordinary requests keep the disk manager disabled.
Positive-control scratch is limited to 64 MiB and merge fan-in to two. Metrics
report cumulative spill bytes/count and pool reservation after stream drain;
they do not measure total RSS or peak scratch use.

The Darwin-only `NoFileWrites` launch option wraps the worker in the installed
`/usr/bin/sandbox-exec` with a constant profile denying `file-write*`. Source
filenames and credentials are not inserted into the profile or command line.
The host still launches one expected PID, uses the same authenticated UDS gRPC
stream, passes an empty environment and reaps the worker. The profile has no
network or file-read restrictions: this is file-write denial, not a complete
sandbox. Linux rejects this requested mode before launching a worker; it never
falls back to an unguarded launch.

The installed Apple manual marks `sandbox-exec` deprecated. Its use here is
experimental evidence on the observed host, not a supported distribution or
production containment decision. No global OS configuration changes are made.

## Assurance and proof limits

The helper control first writes normally, then must receive a permission error
when creating a file, overwriting it or writing through a selected symlink under
the same denial profile. The existing file must remain unchanged. Its subprocess
has a bounded lifetime. The native pressure test then checks:

- real spill and sentinel detection in the unguarded positive control;
- rejection of a spill directory without explicit positive-control opt-in;
- typed memory exhaustion with disk disabled;
- denial when the spill-enabled control runs under OS write restrictions;
- correct descending results under the default pool and write denial;
- cancellation, forced kill, endpoint removal and subsequent recovery;
- no sentinel in worker diagnostics/errors and no files left in owned scratch.

The source fixture and legitimate Arrow result frames intentionally contain the
sentinel. Leak checks cover diagnostics/errors and owned scratch, not authorized
data returned to the host. The observer checks regular files while the positive
control is streaming. It can miss transient files and is not an OS-wide event
monitor. The observer tolerates only child paths that vanish during cleanup;
root and permission errors still fail. The kernel write-denial probe is separate evidence; neither observation
is presented as universal filesystem or confidentiality proof.

The guarded native worker reports the environment name
`__CF_USER_TEXT_ENCODING` even though its launcher passes an empty environment.
The record includes names, never values; the parent sentinel variable must not
appear. A direct empty-environment `sandbox-exec /usr/bin/env` probe reports no
names, so the origin of the worker's runtime-added name is not independently
traced. No unsafe environment mutation is added to force a zero count.

`fs_usage -t 1 -f filesys <own PID>` reports that root privileges are required.
No elevation was attempted. OS-wide creation tracing is **BLOCKED** here.
Host-wide combined RSS/CPU/job admission, the required 2/4/8-worker runs,
malformed-parser/security corpora and native Linux/Windows qualification remain
**NOT-RUN** or independently **BLOCKED**. DF-062 remains **BLOCKED**, not PASS.
This slice does not establish no OS swap/core-dump persistence or behavior of
other processes or privileged services.

## Related framing defect

The pressure fixture exposed an existing IPC defect: sliced Arrow string/binary
views retained unselected multi-megabyte backing buffers. A small selected row
could fail the frame cap. Before serialization, the worker now compacts visible
top-level Utf8View/BinaryView values through Arrow's safe `gc` API. Types, schema
and selected values are unchanged. The regression first failed with a roughly
5 MiB one-row frame; after correction it fits 64 KiB and contains no unselected
payload. Visible view bytes are preflighted before copying; oversized batches
are split first and frames are encoded lazily through the existing send path.
This avoids expanding a whole shared-view batch or collecting all encoded
frames before backpressure. The shared-buffer regression also observes the
actual compaction call sizes. Nested view framing remains unqualified; this is
not a claim of complete nested-schema transport coverage.

## Reproduction

With the pinned Rust 1.99.1 installed, from the repository root:

```sh
DF0_NO_SPILL_EVIDENCE=/absolute/path/to/no-spill.json \
  bash spikes/adr012-datafusion/scripts/run-df0d2-no-spill.sh
```

The command requires native Darwin and the installed OS tool. An unavailable
mechanism must fail or be recorded BLOCKED, never produce an unguarded PASS.
Generated bindings use the existing pinned task-only generators; no production
Go module or dependency versions/features are changed.

## Validation record

The [machine-readable record](../../spikes/adr012-datafusion/evidence/df0-d2-macos/environment.json)
binds the source, worker, toolchain and eight scenario results by SHA-256.
The [raw scenario observations](../../spikes/adr012-datafusion/evidence/df0-d2-macos/no-spill.json)
retain the original unavailable process inspection explicitly; their zero or
negative RSS fields are not measurements or proof of a memory bound. A later
[host-accounting correction](adr-012-df0-resources.md) records real per-child
memory/CPU on macOS without rewriting these historical observations. Socket,
secret and OS-wide filesystem inspection gaps remain open.

Acceptance criteria for this slice passed:

- The positive control recorded 126 spills, 32,162,448 cumulative spilled bytes
  and a visible spill sentinel. Owned scratch was empty after every scenario.
- Memory-only and OS-denied controls returned typed `RESOURCE_LIMIT` failures.
  A directory without opt-in returned `INPUT_INVALID` before source reads.
- Guarded normal/recovery queries returned all 4096 exact IDs in descending
  order. Cancellation and forced termination reaped workers and removed endpoints.
  Cancellation can return a provisional prefix; failure is not a complete result.
- The native reproduction passed all eight new and 21 existing worker scenarios,
  plus the OS create/overwrite/symlink controls. Five repeated pressure runs
  passed after correcting the transient-child observer race.
- Rust locked workspace tests passed in development and release (20 tests each),
  as did formatting and strict Clippy. Go spike race tests and vet passed with
  the freshly rebuilt release worker. The Linux host test package cross-compiled;
  native Linux execution was not performed.
- Root `make verify`, coverage-ledger drift, release-scope and REUSE checks passed.
  Generated bindings reproduced byte-for-byte. Dependency versions/features and
  the 94-case manifest stayed unchanged; all 121 prior candidate records matched
  after excluding `wall_ns`. A fresh advisory audit reported zero vulnerabilities
  and zero warnings.

Adversarial controls used disposable copies: removing OS denial failed the
permission assertion; changing the detector needle failed the actual-spill
assertion; disabling view preflight failed the observed compaction bound. The
original sliced-view serializer failed the small-row regression before its fix.
One initial detector mutant did not compile and was discarded as evidence.
No meaningful tests were weakened.

The development engine test binary emitted a linker warning that its large
`__eh_frame` section can affect exception-handling performance. Release build,
release tests and native worker execution passed. This is recorded, not suppressed.
Independent reviewers were unavailable because their quota was exhausted;
structured plan, mid-work and final adversarial self-reviews were used as the
explicit fallback. This is not independent review or production approval.

Public interface, production schemas, application dependencies and frontend
behavior are unchanged. This experiment uses additive spike protocol minor 2
fields and rejects older workers when those fields are requested. The remaining
qualification gaps above must be closed before recommending DuckDB retirement.
