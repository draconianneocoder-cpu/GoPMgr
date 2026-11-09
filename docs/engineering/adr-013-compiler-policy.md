<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-013 P6-E: native compiler and deployment-target characterization

Date: 2026-11-05. Baseline: `ec79bc07eaeef279ab52b9c960065ebcde24dc5e`.
Characterization complete on the available macOS and Ubuntu hosts. These
measurements predate the subsequent owner decision adopting macOS 26 Tahoe.
See the [current support policy](macos26-support-policy.md). The 13.0 measurements
below remain historical; the proposed Make correction now targets 26.0 and is
not implemented by the support-version change. The subsequent
[Make environment slice](adr-013-native-make-environment.md) implements five
test/race/coverage paths; direct release/helper paths and full P6-E closure remain.

## Current behavior and evidence

`Makefile` assigns and exports `CC := gcc`. An environment `CC=clang` is
replaced by this file assignment; `make CC=clang` overrides it. These are
observed precedence rules, not a claim that environment overrides are intended.
On this macOS 27.2/arm64 host, `/usr/bin/gcc` and `/usr/bin/clang` both identify
Apple Clang 21.0.0. On upstairsserver, Ubuntu 26.04/amd64, `gcc` and `cc` both
identify GCC 15.2.0; Go's ordinary compiler default names
`x86_64-linux-gnu-gcc`. Matching banners are not full compiler-equivalence proof.

The reviewed Wails build and binding wrappers set `MACOSX_DEPLOYMENT_TARGET=13.0`
and append `-mmacosx-version-min=13.0` to C, C++ and linker flags. The Darwin
Taskfile also sets the deployment variable. Ordinary Make Go recipes export the
compiler but do not establish those four deployment settings.

Wails v3.1.0 contributes older per-package deployment flags, including
`-mmacosx-version-min=10.13`, in the actual desktop dependency graph. Therefore
an isolated cgo probe is insufficient to diagnose the application's linker
behavior. Actual desktop test-binary compilation was measured separately.

| Native experiment | Compiler/settings | Result |
|---|---|---|
| Minimal cgo executable, macOS | `CC=gcc`, ordinary Go defaults | Runs; Mach-O minimum 27.2; no newer-object warnings |
| Same minimal executable, macOS | `CC=gcc`, explicit four-variable 13.0 settings | Runs; Mach-O minimum 13.0; no newer-object warnings |
| Desktop test binary, macOS | `CC=gcc`, ordinary Go defaults | Minimum 11.0; 68 newer-object linker warnings |
| Same desktop test binary, macOS | `CC=gcc`, explicit four-variable 13.0 settings | Minimum 13.0; zero newer-object linker warnings |
| Minimal cgo executable, Ubuntu | `CC=gcc` and separately `CC=cc` | Both compile and run as native amd64 ELF executables |

Both desktop binaries pass the existing
`TestProjectMetaWireBudgetEditsAreExactAndCanClear` test on the current Mac.
This is one focused native persistence check, not the complete desktop suite,
window qualification or evidence of macOS 13 execution. The existing
`bash scripts/wails-build_test.sh` fixtures pass with no changes.

Compilation uses Go 1.27.3 and existing module/build caches. The desktop package
has real tests; an initial root-package `go test -c` experiment produced no
binary because that package has no tests and is excluded from the binary
comparison. Fresh output paths and `test -f` checks precede header inspection.
An existing app binary was observed with minimum 13.0, but was not rebuilt and
is not counted as current qualification evidence.

## Reproduction

Use an ignored task-owned output directory. Capture Go/compiler versions,
`go env CC CGO_CFLAGS CGO_CXXFLAGS CGO_LDFLAGS`, and relevant environment settings.
For Make precedence, use a supplementary Makefile whose only recipe prints
`$(CC)`, `$(origin CC)` and those Go environment values; run default,
environment-override and command-line-override cases. Do not edit production
Make rules for measurement.

The application comparison commands are:

```sh
CC=gcc go test -x -work -c -o .tmp/compiler-policy/desktop-default.test ./internal/desktop
MACOSX_DEPLOYMENT_TARGET=13.0 \
  CGO_CFLAGS='-O2 -g -mmacosx-version-min=13.0' \
  CGO_CXXFLAGS='-O2 -g -mmacosx-version-min=13.0' \
  CGO_LDFLAGS='-O2 -g -mmacosx-version-min=13.0' \
  CC=gcc go test -x -work -c -o .tmp/compiler-policy/desktop-floor13.test ./internal/desktop
otool -l .tmp/compiler-policy/desktop-default.test
otool -l .tmp/compiler-policy/desktop-floor13.test
```

Inspect the `LC_BUILD_VERSION` minimum and the actual compile/link traces,
including dependency flags. Run the focused test explicitly with `-test.run`
and `-test.v`, verifying that it ran rather than accepting an empty selection.
`-work` retains temporary build directories; remove only the paths recorded by
these task-owned invocations after retaining their evidence.

## Findings and proposed correction

The deployment-setting difference is confirmed. Compiler-name substitution is
not the root-cause fix: the two names already resolve to Apple Clang here.
Leaving the mismatch unresolved retains misleading test-binary metadata and
avoidable linker warnings. Changing the global compiler default or introducing
a compiler manager adds compatibility decisions without addressing it.

Recommended first correction, subject to approval:

1. Retain `CC := gcc`, environment/command-line precedence and existing platform
   support. Compiler-selection changes remain a separate decision.
2. Align canonical native macOS Make Go compilation with the owner-approved 26.0 app
   contract. Cover test/race/coverage and native build/binding callers in a
   reviewed caller map; ordinary commands outside Make need explicit documented
   settings, not an unsupported claim of automatic coverage.
3. Preserve existing non-target C/C++/link options and Go's `-O2 -g` defaults
   when unset. With no target supplied, establish 26.0. Accept equivalent 26.0
   settings; reject conflicting explicit deployment targets with a diagnostic
   rather than silently rewriting them. This includes a higher target that
   would cease testing the baseline. Apply the rule to environment and Make
   command-line inputs. Avoid repeated flag growth across recursive Make and
   wrapper calls.
4. Apply Darwin flags only to native Darwin work. Preserve Linux and Windows
   inputs. Cross-target handling must be explicit: do not infer target OS solely
   from host `uname`, or add mandatory Go execution while parsing `make help`.
5. Add behavioral tests first: capture compiler environments from actual Make
   recipes; require all four 26.0 settings on native Darwin; preserve unrelated
   options, existing compiler overrides, Linux/Windows inputs and help without
   Go. Add negative conflicting-floor and nested-caller cases. Keep existing
   race serialization and uncached-execution assertions intact.
6. Reproduce the original mismatch as RED before implementation. Rerun the
   real desktop binary/header comparison, focused/full relevant tests and local
   Ubuntu controls after correction. Do not suppress linker warnings or patch
   upstream module caches. Update the caller map and proof limits.

This is a proposed policy, not an implemented fix. The maintainer must approve
its native-target scope and deployment-flag precedence before Make changes.

## Review and limits

Independent plan review approved native characterization and required the
actual application-link comparison, preserving compiler-selection decisions
and distinguishing minimal probes from application evidence. Its requested
comparison is now recorded above. Independent final review approved the proposal
for an owner decision, with caller coverage and cross-target isolation remaining
implementation requirements. No owner approval or implemented correction is
implied. REUSE 3.3 and whitespace checks pass.

No hosted checks, full release, new tool installation, signing or minimum-OS
qualification occurred. At characterization time macOS 13/Ubuntu 24.04 hosts
were unavailable; the current minimum-OS gate is macOS 26. Windows
is unmeasured. A 13.0 load command and current-host success do not establish
that every dependency or code path runs on macOS 13. The full suite was not
rerun for this documentation-only characterization; it remains required for
any implementation.
