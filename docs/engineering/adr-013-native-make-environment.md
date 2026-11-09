<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# P6-E: native Make test environments

Baseline `1e8d8b4`, 2026-11-05. This slice aligns five Make test/coverage
entry points with the owner's [macOS 26 minimum](macos26-support-policy.md).
Compiler selection, Go/dependency pins, package scope, race policy, coverage
baselines, application code and hosted workflows remain unchanged.

## Caller map and contract

| Make target | Probed Go executable | Guarded command |
|---|---|---|
| `test` | `$(GO)` | Existing Go test command and arguments |
| `race` | `$(GO)` | Existing `-race -p=1 -count=1` command and arguments |
| `coverage-ledger-drift` | PATH `go` | Actual coverage script; its fixture tests remain independent |
| `coverage-ratchet` | PATH `go` | Existing ratchet script |
| `coverage-ratchet-update` | PATH `go` | Existing ratchet script with `--update` |

Coverage intentionally retains its existing bare-PATH-Go behavior. A Make `GO`
override controls test/race, not coverage. Probing a different overridden binary
for coverage would misclassify its target; a distinct-executable regression
checks this boundary.

`scripts/native-go-env.py` runs only when a guarded recipe executes. It asks the
selected Go command for host and target OS, preserving command prefixes such
as `GO="env NAME=value go"`, then replaces itself with the
unchanged command/argument list. It never runs a shell to interpret the command.
There is no Go discovery during Make parsing/help and no installation or cache
cleanup. The launcher uses Python 3, already a repository host prerequisite;
regression tests use the documented Python 3.11+ environment.

Only native Darwin host plus Darwin target receives the 26.0 environment.
Linux, Windows and cross-target settings pass through unchanged. Missing,
malformed or empty target metadata and lookup failure/30-second timeout stop
before child execution. Missing commands and child failures remain failures.

For native Darwin:

- Establish `MACOSX_DEPLOYMENT_TARGET=26.0`; accept supplied `26`, `26.0` or
  `26.0.0`, and reject conflicting lower or higher settings.
- Preserve non-target option strings, including quoting. Unset C/C++/link flags
  retain Go's `-O2 -g` defaults; explicitly empty flags remain intentionally
  empty except for the required deployment flag.
- Accept equivalent `-mmacosx-version-min=` and Clang `-mmacos-version-min=`
  settings. Add the floor only when absent, so nested calls do not grow flags.
  Inspect CPP flags for conflicts without otherwise modifying them.
- Reject response files, including linker-forwarded `@file`, explicit target
  triples/`-mtargetos`, and forwarded OS-minimum/platform-version controls. These
  ambiguous target controls must be expressed through the supported baseline
  settings rather than silently hiding another floor. This is a build-contract
  guard, not a full compiler-argument parser or security sandbox.

Other Make Go tools, direct release/validator scripts, direct `go test`, and
app/development/binding wrappers are not newly guarded here. The app/binding
paths already set 26 in their own wrappers. A manual guarded command can use:

```sh
python3 scripts/native-go-env.py go -- go test ./internal/auth
```

This bounded completion does not close all of P6-E or establish macOS 26 native
execution. The original [characterization](adr-013-compiler-policy.md) remains
historical evidence for its recorded revision and compiler settings.

## Risk and verification

Twelve behavioral tests run the real Make recipes in disposable directories,
using external-command fakes to capture actual child arguments/environment.
They cover all five callers, compiler precedence, distinct Go executables,
quoted/empty/default options, conflicting environment and Make arguments,
native/cross-target isolation, idempotence, metadata failure, child errors and
help without Go discovery. Coverage fakes establish Make boundary behavior,
not coverage-script internals or actual coverage percentages.

Initial RED evidence showed missing native floor and ignored conflicts. An
independent plan review exposed the coverage executable-identity mismatch;
the plan was revised before implementation. Mid-review found that a forwarded
response file could hide target settings. New environment/Make cases failed
before that rejection was added. The first broad verification also caught
those newly added cases against the earlier launcher; it was rerun after repair.
A timeout test injects `TimeoutExpired`; it verifies error handling without
claiming a real 30-second hang was executed.

The release-scope guard now recognizes the launcher prefix while preserving
its exact race-entrypoint contract and first-party package roots. Its regression
suite retains all 24 race-policy mutations and adds a wrapped unscoped-Go case.
The prefix change exposed an old mutation that no longer applied; the fixture
now verifies every mutation changes its source before assessing detection.
A separate command-prefix regression exposed an executable-only probe that
would reject an existing Make `GO="env NAME=value go"` override. It failed before
word-list probe support was added; no shell evaluation is used by the launcher.
No test was removed or weakened to obtain a passing result.

Native macOS compilation through the launcher produces a desktop test binary
with Mach-O minimum 26.0. The focused persisted-budget edit test passes in that
binary on macOS 27.2. Focused Make race and guarded coverage tests for auth/crypto
pass. Shared compiler/module caches are reused; this is not a cold-build claim.
Native Ubuntu runs the same fake-Make matrix and a real Go-identity/non-Darwin
preservation control against hash-matched launcher inputs. This is not Linux
product qualification or native Windows execution.

Canonical `make verify` passes, including Go tests, all 742 frontend tests,
Svelte checks and build/contracts. ShellCheck/shfmt are clean for all 90 tracked
scripts; REUSE 3.3 passes. Existing browser-fixture canvas warnings remain
visible; no frontend behavior changed. The scope guard passes both the live source check and
its complete mutation suite. Full race/coverage suites and coverage-baseline
updates were not run; the native race/coverage executions above cover only
auth/crypto. Independent plan, mid-work and final reviews were completed;
findings about executable identity and forwarded response files were corrected.
Final review approved the complete slice with no blocking findings. Executions
were assessed from receipts rather than repeated by the reviewer. Local receipts and source/artifact hashes remain in
ignored task state. No hosted CI or publication is part of this slice.

## Remaining work

The subsequent [standalone release/validator slice](adr-013-release-native-environment.md)
aligns release race execution, encrypted-database tests and PDF generators.
Other direct analysis helpers and standalone coverage invocations still need
their own caller review; this Make-only evidence does not qualify those paths.
Minimum-version macOS 26 execution, installer and notarization evidence, and
hosted runner suitability remain release gates.
