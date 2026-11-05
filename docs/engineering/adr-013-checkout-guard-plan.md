<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-013 P6: checkout credential guard

Status: owner-approved on 2026-11-05 and implemented in the common Go workflow
validator. Baseline `1807d6a74bb2e75268ba5b6e3375d2ce017d847e` had a clean
tracked tree/index. Workflow files remain unchanged.
All 11 current checkout steps declare `persist-credentials: false`.

## Priority and evidence

P5 demonstrated that zizmor 1.32.0 regular mode reports omitted credential inputs
but accepts explicit `true`. The existing characterization test still passes,
confirming that gap. Before this repair, the Go configuration checker validated workflow shape and
selected assurance contracts without checking checkout credential inputs. A
credential-persistence regression could therefore pass both checks.

Close this focused security-policy gap before lower-consequence P6 housekeeping.
P6's REUSE failure handling, dependency checks, tool pinning and concurrency
characterization remain separate work packages, not silently completed here.

## Approved policy

Every locally declared workflow step referencing the `actions/checkout`
repository must have an effective `with.persist-credentials` value that is the
YAML boolean `false`. This includes checkout references with any ref and repository
name casing and optional action paths. Repository components follow the runner
split on slash/backslash, ignoring empty components; action pins themselves
remain governed by existing checks. Input names are case-insensitive. Every
spelling of the credential input must be boolean false, preventing conflicting
cased keys from hiding an unsafe value.

Reject omission, null, `true`, quoted strings such as `"false"`, numeric values
and expressions, including `${{ false }}`. No conditional-step exemption or
checkout-specific suppression is proposed. Conditions do not make a future
credential-persistence change acceptable.

Use the existing YAML parser's alias/merge resolution: an effective boolean
`false` passes; an effective unsafe value fails. This is semantic validation,
not an indentation or spelling rule. Duplicate keys remain errors under the
existing parser. Do not add regex-based YAML parsing or another parser dependency.

Run steps and unrelated actions remain valid. Reusable-workflow jobs without
local steps remain valid; this guard does not inspect remote reusable workflows,
third-party composite internals, shell `git` commands or arbitrary token use.
Lookalike repository names such as `actions/checkout-extra` must not be mistaken
for checkout. Malformed local job/step/input structures must fail visibly rather
than silently bypassing inspection.

## Implementation location and alternatives

The implementation extends the common workflow validator in `tools/config-check/config_check.go`;
its CI-specific validator already delegates to that common function. All three
registered workflows receive the same rule. The existing closed configuration
inventory refuses new unclassified workflow files until registered.

This intentionally adds enforcement to existing `make config-check` and therefore
`make verify` and release callers that already invoke it. It does not add a new
scanner dependency, hosted job, required-check name, tool pin or new Make recipe.
`make workflow-security` retains the separately approved P5 regular-persona
contract and its characterization test. Users need the configuration check for
this additional structural guarantee.

Broadening zizmor to auditor mode adds unrelated style findings and changes P5's
policy. A second YAML parser or compiling Go inside the read-only Python scanner
would duplicate parsing or add execution/cache boundaries. Reusing the existing
Go consumer validator is the smallest coherent option.

## Tests, review and acceptance

1. Obtain explicit owner approval of this blocking policy and placement.
2. Add behavior-focused Go tests through the existing parser/validator, using an
   independently supplied workflow fixture. Demonstrate failure for explicit
   `true` before implementation; do not derive expected input inventory solely
   from the registry under test.
3. Implement the narrow common validation. Report file/job/step context. Reject
   malformed structures without panics or fail-open branches; keep output stable
   when multiple jobs are invalid.
4. Exercise boolean false, missing input/with, true, strings, null/numbers,
   expressions, aliases/merge overrides, duplicate keys, conditional steps,
   checkout refs/casing, lookalikes, local paths, malformed `uses`, unrelated
   run/action steps and reusable jobs. Cover aliases at job/step/input levels,
   merge sequences and both override directions. Reusable-job syntax must not
   exempt locally present steps from inspection. Insert unsafe fixtures
   independently into each registered workflow path and assert checkout-specific
   diagnostics, so an unrelated schema error cannot satisfy the negative case.
   Verify all current checkout sites pass without workflow changes.
5. Independently review the first meaningful implementation. Seed a controlled
   bypass in a disposable copy and show the intended assertions detect it.
6. Run focused Go tests, local macOS and Ubuntu configuration qualification,
   canonical macOS `make verify`, and existing actionlint/zizmor checks. Record
   source/index preservation and platform proof limits. Run no hosted workflow.
7. Review the complete diff independently, fix findings, update ADR/roadmap/test
   ledger and project memory, clean task fixtures and commit only task-owned work.

Acceptance: unsafe checkout inputs fail through the real YAML consumer on every
registered workflow; safe existing inputs and unrelated valid steps/jobs pass;
application source, workflow files, pins, job contracts, publishing credentials
and P5 scanner policy remain unchanged. No new dependencies, suppression or hidden bypass.

Recovery: reverting this independently reviewed guard would reopen a documented
credential-policy gap and require an explicit security-policy decision. Existing
P5 credential-disabling mitigations must remain intact. No production data,
file-format migration or runtime interface change is involved.

## Evidence and lessons

The new parser/consumer tests failed against the baseline for explicit true,
missing inputs and malformed structures before implementation. They exercise
all three registered workflows independently, aliases and merge precedence,
conditional steps, stable diagnostics and safe unrelated actions/reusable jobs.

Mid-review found a repository-matching bypass: optional action paths and repeated
separators can still identify checkout. Self-review also found that differently
cased input names could override a safe value. New failing cases preceded both
corrections. The [runner parser](https://github.com/actions/runner/blob/main/src/Sdk/DTPipelines/Pipelines/ObjectTemplating/PipelineTemplateConverter.cs)
confirms repository splitting and case-insensitive inputs. Unsafe alias cases
were strengthened to fail at the alias consumer, not its anchor declaration.

Final local qualification:

- `go test -count=1 ./tools/config-check` and `make config-check` pass on
  macOS and Ubuntu using Go 1.27.3; all 27 tracked configurations pass.
- macOS `make verify` passes, including 742 frontend tests. Existing native
  linker deployment-target warnings remain; this tooling change does not
  qualify older macOS deployment compatibility.
- Local actionlint/zizmor checks pass for all three workflows; ShellCheck
  and shfmt pass for all 90 inventoried scripts.
- A disposable explicit-true bypass mutation fails the intended assertions.
  On Ubuntu, injecting true into a snapshot workflow fails `make config-check`
  with the checkout-specific diagnostic; restoring exact bytes passes.
- Final Ubuntu receipts bind all 574 snapshot inputs to matching macOS hashes
  before and after execution, including the existing DF0 test fixtures. An
  earlier incomplete snapshot lacked those fixtures; rebuilding the snapshot
  resolved that qualification setup failure.
- The 11 live checkout sites and all workflow hashes are unchanged. No tool
  pins, module dependencies, publishing credentials or Make recipes changed.

This guard
covers local workflow definitions only. It does not inspect remote reusable or
composite internals, execute workflows, or qualify signing/release/native OS
behavior. No hosted run or new mandatory scanner target is introduced. P6's
other contract repairs remain open.
