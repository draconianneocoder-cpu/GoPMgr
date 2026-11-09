<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-013 P3: local shell formatting check

The maintainer approved tabs and shfmt v3.14.1's other printer defaults on
2026-11-05. See the [approved contract](adr-013-shfmt-plan.md). The checker is
opt-in; mandatory-gate adoption and hosted wiring remain separate decisions.

```sh
PATH="$PWD/.tmp/engineering/bin:$PATH" make shfmt-check
make shfmt-tests
```

Supply the pinned binary using [P1 provisioning or independent recovery](adr-013-provisioning.md).
Only shfmt is required; the runner verifies its executable hash before version
execution. There are no installs, lock changes or implicit tool downloads.
Python 3.11+ and system `/usr/bin/git` are host prerequisites.

## Contract

The checker reuses [P2's tracked first-party inventory](adr-013-shellcheck.md),
including ownership/generated exclusions, symlink refusal and dialect checks.
Each script is passed as bytes on stdin with explicit `-i=0` and `-ln=bash` or
`-ln=posix`. The explicit flags disable EditorConfig formatting settings;
stdin without a filename avoids ignore rules and path-option ambiguity.
It compares formatted bytes to the source, prints JSON-escaped drift paths and
returns failure if any differ. It never executes a source script or supplies
`-w`, `-s`, `-mn` or a file-write argument to the formatter.

Missing/unverified tools, invalid shell syntax, Git/inventory failures and
formatter timeouts fail visibly. Each format operation has a 30-second bound;
timeouts identify the source. It preserves source and index bytes and does not
write config, caches or generated output. Checks do not use the network.
A clean formatting report says nothing about shell security or runtime safety.

## Baseline and verification

At baseline `5024043`, the check reported 90 scripts and 48 needing formatting,
returning failure as designed. The checker was committed separately as `2f1720c`.
The subsequent formatting-only change normalizes those 48 scripts, including
reviewed indentation alignment of the release race witness. The final inventory
remains 90 scripts, with zero formatting differences and zero ShellCheck findings.

Fifteen behavioral tests use the real pinned formatter, disposable Git roots
and hand-derived expected output. They cover passing input, drift without writes,
parser errors, Bash versus POSIX, ambient configuration, unusual extensionless
paths, exclusions/untracked files, symlinks, empty inventory and missing or
substituted executables. The substituted executable cannot create its marker.
One injected subprocess timeout tests the wrapper's diagnostic without waiting
30 seconds; it is not evidence of a real formatter hang. Make is exercised
green, drift failure, then restored green with source/index bytes unchanged.
Heredoc and literal-output fixtures compare original/formatted runtime output
against the same hand-written byte expectation.

TDD began with the absent-checker failures. A timeout-path regression separately
failed before its diagnostic correction. The initial checksum-message assertion
was corrected to the existing P1 diagnostic without changing the execution
marker assertion. A disposable missing-drift-exit mutation is rejected by the
three intended CLI/Make/multi-file assertions. No mutant is retained.

Plan review required a correction: shfmt preserves heredoc whitespace, so the
byte-exact release race witness must be aligned with the normalized source.
Only expected indentation may change; all 24 race-policy mutation cases and
exact command/scope/failure checks remain required. Formatting and functional
repairs must stay separate.

Before normalization, all 15 checker tests passed on local macOS and Ubuntu
26.04.1. macOS also passed `make verify` (742 frontend tests), REUSE 3.3 lint
(2,056 files) and a check under a sandbox denying network and persistent writes
apart from Git's nonpersistent `/dev/null` sink. The sandbox check returned the
expected 48-file drift failure. Linux evidence at this stage is checker fixtures,
not a full application build or native package qualification.

After normalization, local macOS passed:

- `make shfmt-check shellcheck`: 90 scripts, zero drift/findings.
- `make shfmt-tests shellcheck-tests`: 15 formatter, 18 analyzer and five
  script-contract tests.
- `make verify`: full canonical Go/frontend regression gate, 742 frontend tests.
- All 18 changed shell regression scripts, including real macOS/Linux packaging
  fixture matrices and native-launch, Wails-build and qualification wrappers.
- `bash scripts/release-gate-scope-check_test.sh`: all 24 race-policy mutations.
- `make shfmt-check` under the same network/persistent-write-denied sandbox.

Before witness alignment, the normalized source failed the existing valid-scope
assertion as expected. Alignment restored it without relaxing the matcher.
An independent source review compared all 48 paths with pinned formatter output
and checked 76 heredoc payloads; only the approved race witness payload changed.
A supplementary position-free shfmt AST comparison matched 47 files; the sole
difference is that witness literal. This uses the same parser as formatting,
so it supports review rather than independently proving behavioral equivalence.

Local Ubuntu 26.04.1 passed the final 90-script formatting check and all 15
formatter tests. SHA-256 comparison confirmed all 90 input scripts matched the
macOS snapshot. The Linux formatter was provisioned only in a temporary task
directory from P1's pinned artifact and hash; no global installation changed.
The temporary checkout/tool fixtures were removed after retaining receipts.
Linux's full application build, native GUI and real packaging were not rerun for
this formatting change. Windows scripts were statically checked and their shell
fixtures ran on macOS, which is not native Windows execution evidence.

All checks are local. No required Make dependency or hosted job was added.
Native release/signing and unavailable minimum-OS checks remain separate gates.
[P4 actionlint inventory and adjudication](adr-013-actionlint.md) is implemented.
[P5's local zizmor policy](adr-013-zizmor.md) is implemented. Next: P6 contract
hardening, retaining the explicit policy gate before new mandatory adoption.
