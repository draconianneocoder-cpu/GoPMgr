<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-013 P4: local workflow correctness

P4 adds opt-in, read-only `make workflow-lint` using the existing P1 pin,
actionlint 1.7.12. Findings and operational failures return failure; advisory
means invocation is optional, not that errors are hidden. It is not a dependency
of `make verify` or a hosted workflow. Mandatory adoption remains a separate gate.

```sh
PATH="$PWD/.tmp/engineering/bin:$PATH" make workflow-lint
make workflow-lint-tests
```

Use [P1 provisioning or independent recovery](adr-013-provisioning.md) to supply
the exact executable. The runner checks its hash before version execution.
No installers, updates, workflow execution or network calls occur in checking.
Python 3.11+ and system Git are host prerequisites.

## Workflow inventory and preserved contracts

Baseline: `91863e66fbe0b425a392a38b45ebfcad624db36f`, clean tree/index. There are
three tracked workflows and 11 job definitions; no reusable workflow or local
action reference is currently declared. All six external action references are
commit-pinned: checkout, setup-go, setup-node, upload-artifact, download-artifact
and golangci-lint-action. Actions are not downloaded or executed by this check.

| Workflow | Trigger and jobs | Dependencies, permissions and matrices |
|---|---|---|
| [CI](../../.github/workflows/ci.yml) | Push/PR to main or master; `verify`, `assurance`, `pades-harnesses`, `build`, `lint`, `security`, `vuln` | No job `needs`; CI token permissions remain inherited. Ubuntu 26.04 except assurance on macOS 15. Existing security `continue-on-error` remains visible and unchanged. |
| [Release](../../.github/workflows/release.yml) | Push tags `v*`; `preflight`, `build`, `publish` | `preflight` then `build` then `publish`; contents read by default and write only for publishing. Package matrix remains Ubuntu 24.04, macos-latest and windows-latest; preflight remains Ubuntu 26.04. |
| [G0 spike](../../.github/workflows/adr010-g0-spike.yml) | PR path filter for isolated spike and its workflow; `evidence` | contents read; matrix now six rows: Ubuntu 26.04/x64, Ubuntu 24.04/arm64, macOS 15/arm64, Windows 2025/x64, Windows 2022/x64 and Windows 11/arm64. This remains evidence, not a production gate. |

All current job names, triggers, conditions, dependencies, permissions, action
references and canonical Make calls are preserved. GitHub branch-protection
settings were not inspected; source-level preservation is not remote required-
check qualification. The only workflow edit is the owner-approved removal of
G0's retired `macos-14` row and its obsolete comment. macOS 13/Ubuntu 24.04
minimum-version release execution remains mandatory on separate hosts.

## Finding adjudication and configuration

The baseline produced eight `runner-label` findings, all `ubuntu-26.04`.
[GitHub's official image inventory](https://github.com/actions/runner-images#available-images)
listed that exact x64 label on 2026-11-05. The pinned analyzer's built-in list
is stale; downgrading the workflow target would change an unrelated policy.
The reviewed `.github/actionlint.yaml` extends only that exact label using
[actionlint's documented label mechanism](https://github.com/rhysd/actionlint/blob/v1.7.12/docs/config.md).
Its key is named `self-hosted-runner`, but this does not classify the project's
hosted runner as self-hosted, provision a runner or alter scheduling. It has no
wildcards or ignore rules. Unknown-label negative controls still fail.

The same official source reports macOS 14 images retired on November 2, 2026.
The owner explicitly approved removing that unavailable G0 matrix row. This is
a runner-availability correction, not removal of an application support target.
It also shows a limit of static tooling: actionlint still recognizes that
retired label, so a green report does not establish runner availability.

The checker supplies this reviewed config explicitly. Caller/ambient config
cannot mask errors. The tracked config itself remains review-controlled source,
not hostile input. Registering the new YAML in the existing configuration
inventory preserves canonical validation; a test first reproduced the
unclassified-file failure. A malformed-config case verifies YAML parsing. The
old error message's nonexistent script path now points to its actual owning Go
file. No mandatory actionlint invocation was added to that existing gate.

External ShellCheck and Pyflakes integrations are explicitly disabled, with a
fixed child PATH. This keeps results independent of unverified sidecar binaries.
P4 checks workflow syntax, structure and expressions, not embedded shell/Python
correctness or workflow security. P2's standalone shell inventory does not
include inline `run` bodies. Inline-code assurance remains a separate gap;
P5's security review must not misrepresent it as already covered.

## Read-only boundary and evidence

The runner uses NUL-delimited tracked paths under `.github/workflows` and checks
working-tree contents, so newly staged workflows and pre-staging edits to
tracked files are covered. Untracked workflows are intentionally excluded until
tracked. Both `.yml` and `.yaml` are eligible. Nested paths, symlinks, missing
files, merge conflicts and an empty inventory fail visibly. Git configuration
and fsmonitor execution are disabled. Each analyzer invocation has a 120-second
bound; all workflow paths are explicit operands. Local reusable-workflow/action
metadata may be read by actionlint, but sources are never executed.

Twenty real-analyzer fixture tests cover valid workflows, the exact configured
label and self-hosted example, unknown labels, expressions, undefined job/output
references, malformed YAML, reusable calls, exclusions, ambient ignore rules,
unexecuted inline scripts and sidecars, binary substitution, missing tools,
empty/nested/symlink inventory, odd filenames and a defective later workflow. Make goes green, fails on a
seeded expression, then returns green with source/index bytes unchanged.

A disposable mutant that ignored actionlint's nonzero result fails all three
intended expression/output/Make assertions. The initial test-first run recorded
15 absent-runner failures and one missing-helper fixture setup error; the
separate Make control confirmed the missing target. These are feature-absence
receipts, not evidence of semantic detection. The subsequent real-tool negatives
and controlled mutation supply that evidence. No fault remains in the checkout.

Local macOS and Ubuntu 26.04.1 both passed `make workflow-lint` (all three
workflows) and all 20 fixture tests, plus `go test ./tools/config-check -count=1`.
macOS passed canonical `make verify` after the registry repair, including 742
frontend tests, and the workflow check under a sandbox denying network and
persistent writes except Git's nonpersistent `/dev/null` sink. REUSE/SPDX,
configuration and diff-whitespace checks also pass.

An initial generic positive configuration test missed the registry omission
because its inventory was derived from the registry itself. The explicit tracked-
file case now supplies an independent inventory and demonstrated the required
failure before repair. This is a reusable lesson for inventory-driven tests.

These are local static/fixture results, not hosted execution or branch-protection
proof. The Go configuration package ran on Linux, but the full Linux application,
native GUI and real packaging were not rebuilt for this tooling phase. Release,
signing, Windows and unavailable minimum-OS gates remain separate. Temporary
Linux checkout/tool fixtures are cleaned after receipts are retained.

[P5 local zizmor policy and mitigations](adr-013-zizmor.md) are implemented.
Next: P6 contract hardening, including a proposal to close the recorded
credential-detection gap. No mandatory or hosted integration was introduced.
