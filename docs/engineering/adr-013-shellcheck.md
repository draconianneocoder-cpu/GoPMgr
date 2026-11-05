<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-013 P2: ShellCheck inventory and adjudication

P2's local inventory and finding adjudication are implemented. The target is
opt-in, read-only and offline; findings and operational errors return failure.
It is not a prerequisite of `make verify` or a new hosted CI job. Required-gate
adoption remains separate from this local qualification and the owner's P1
policy-acceptance gate.

```sh
PATH="$PWD/.tmp/engineering/bin:$PATH" make shellcheck
make shellcheck-inventory  # JSON, no analyzer required
make shellcheck-tests
```

Use P1's [verified provisioning or independent recovery](adr-013-provisioning.md)
to supply the exact ShellCheck binary. The runner verifies only ShellCheck,
not the other auxiliary executables or native development dependencies. It
rejects a same-version substituted binary before executing it. Python 3.11+
and the qualified system `/usr/bin/git` are host dependencies.

## Scope and source boundary

The [baseline record](adr-013-shellcheck-findings.json) preserves all 90 tracked
first-party scripts, their dialects, the original 53 findings and dispositions
at `f8b83b9`. Of these, 81 are under `scripts/`; nine are isolated transport or
DataFusion spike scripts. They are analyzed, never executed or promoted into
production. All current candidates declare Bash.

The inventory uses NUL-delimited tracked Git paths and reads working-tree
content, so edits to tracked scripts are checked before staging. Newly tracked
extensionless scripts enter automatically. Direct Bash/POSIX `sh`, `env bash/sh`
and common `env -S bash/sh` shebangs are recognized; unsupported shell or ambiguous
env declarations fail visibly. Unknown `.sh` dialects are not guessed. Symlinks,
unmerged paths, a non-root scan directory and an empty inventory fail closed.
`vendor`, `third_party`, `node_modules`, generated frontend bindings/dist and
build output boundaries are excluded. Shell-named excluded paths appear with
reasons in the inventory; excluded trees are not recursively inspected.

Each invocation has one root script and at most four reviewed Bash libraries:
`package-version-lib.sh`, `pades-lock.sh`, `pades-publish.sh` and
`validate-pdfa-lib.sh`. Libraries must themselves be tracked first-party inputs.
This avoids batch-dependent source authorization and bounds operands without
building an unbounded command line. Long/space/newline paths are qualified on
the available macOS and Ubuntu hosts, not every possible platform.

The runner sets the declared dialect, `--norc`, `--check-sourced` and
`--source-path=SCRIPTDIR`. It deliberately omits `--external-sources`: unrelated
or excluded sources remain SC1090/SC1091 failures. Inline attempts to enable
external sources remain analyzer errors. Ambient `.shellcheckrc` cannot disable
checks. Git global/system configuration and fsmonitor execution are disabled,
and optional Git writes are disabled. System Git, Python and the OS remain a
separate trust boundary from the pinned analyzer.

## Finding dispositions

| Original rule | Count | Resolution |
|---|---:|---|
| SC2016 | 27 | Intentional literal source patterns, Go/NSIS fixtures and copyable commands. Maintainer approved statement-scoped exceptions in nine scripts on 2026-11-04; each directive explains why expansion is wrong. |
| SC2086 | 10 | Nine intentional fixed-list expansions use nonempty Bash arrays. One rejected-module diagnostic now preserves the module/version pair on one line. |
| SC2059 | 7 | Constant `printf` formats; color escapes and message text remain separate arguments. These fixed color variables were not established injection vulnerabilities. |
| SC1007 | 5 | Explicit empty environment values in fixtures, with unchanged semantics. |
| SC1091 | 3 | Correct script-relative resolution of the explicitly supplied library. |
| SC2012 | 1 | Human artifact display uses an empty-safe glob and escaped basenames; it does not choose artifacts for packaging. |

The initial all-file exploratory invocation used `--external-sources`; the
qualified source context removes the three SC1091 findings before code changes.
The final runner reports **90 scripts, zero findings** with no severity filter
or command-wide rule exclusion. Existing negative fixtures still prove that
literal checks reject changed workflow pins, publication rules, installer data
and timestamp ordering. Exceptions must be reviewed when their statements
change; a clean report does not certify every shell path.

One older SC2086 directive in `coverage-go.sh` is retained unchanged. It covers
trusted `go list` splitting and the documented Bash 3.2 empty-array/tag workaround;
it is not a new P2 waiver. Obsolete tag plumbing and the previously recorded
package-versus-filesystem scan-scope discrepancy remain P6 follow-ups. Release
REUSE skipping and `.DS_Store` deletion are also existing P6 issues, not fixed
or legitimized by a clean static report.

## Verification and limits

The fixture suite has 18 scanner/inventory cases and five repaired-script
contract cases. It covers valid/unsafe/mixed-dialect scripts, newly tracked
extensionless and `env -S` paths, unlisted and approved sources, removal of the
library operand, source/index preservation, excluded and symlinked files,
ambient configuration, analyzer errors and missing/unverified binaries. A real
Make fixture goes green, fails on seeded SC2086, then goes green after restoration.
Many-script assertions check exact finding file identities, not only counts.

Script-contract fixtures preserve race arguments `-race -p=1 -count=1`, original
package membership, strict/advisory optional-scan failure behavior, ANSI/non-TTY
output, and odd package-display filenames. The strengthened notices regression
failed against the former module/version splitting. Existing release-source
guards retain all 24 race-policy mutation cases. Bash 3.2 compatibility is
exercised through macOS `/bin/bash`; no newer Bash array conveniences are used.

Validation is local. The macOS scan took about 9.5 seconds in the measured run;
this target does not extend canonical verification's time budget. Hosted CI
wiring and required-check conversion from the submitted phase remain deferred
under the local-CI direction. No hosted/blocking-branch result is claimed.
macOS passed the full `make verify` regression gate (742 frontend tests), the
focused existing shell regressions and a scan under a sandbox denying network
and persistent writes. `/dev/null` remained writable because Git opens that
nonpersistent sink read/write. The Git failure diagnostic was preserved rather
than hidden. Initial fixture corrections included draining macOS PTY output
concurrently and giving Linux's clone-based regression fixture a real snapshot
commit; neither correction relaxed assertions.

Linux also exposed a pre-existing test dependency: GTK/header rejection fixtures
required nFPM before reaching their asserted branch. Those fixtures now supply
only the prerequisite metadata and remove the doubles before testing missing or
unverifiable nFPM and real package contents. Actual application metadata still
uses Go. The nFPM-dependent matrix passed on macOS. Linux explicitly skips its
missing-module-notices packaging rejection, real package-content inspection and
Git-derived version case because pinned nFPM is unavailable. Standalone notices
regressions still run on Linux. This is not a native
Linux packaging or release qualification claim. Two SC2155 warnings introduced
during this repair were corrected with separate assignment/export statements.

The unchanged Go/GUI/Rust implementation was not rebuilt or requalified on Linux
for this shell-only change. Linux qualification covers the real pinned analyzer,
script contracts and the existing affected shell regression matrices. Full
release, signing, native Windows and unavailable minimum-OS qualification
are not established by this work. Static analysis never establishes execution
coverage, parser completeness or freedom from shell defects.

Next is P3's reviewed [shfmt](https://github.com/mvdan/sh) style and read-only
formatting check; no formatting
rewrite has been started.
