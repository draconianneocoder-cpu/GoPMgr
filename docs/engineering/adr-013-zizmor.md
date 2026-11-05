<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-013 P5: local workflow security policy

The maintainer approved the [P5 policy and mitigations](adr-013-zizmor-plan.md)
on 2026-11-05 after independent plan review. The check is opt-in and local;
its invocation fails on every reported regular-persona finding, without
severity/confidence thresholds, suppressions or automatic fixes. No new required
Make dependency, protected job or hosted run is introduced.

```sh
PATH="$PWD/.tmp/engineering/bin:$PATH" make workflow-security
make workflow-security-tests
```

Use [P1 provisioning or independent recovery](adr-013-provisioning.md) for the
exact pinned zizmor 1.32.0 executable. Only zizmor is required. The helper checks
its hash before version execution, reuses [P4's tracked workflow inventory](adr-013-actionlint.md)
and supplies explicit file operands. It rejects empty, missing, nested, symlinked
or unmerged input inventories. Untracked workflows are excluded until tracked.

## Baseline findings and mitigations

Baseline `4f7703a` had 23 findings across three workflows. Counts are findings,
not distinct exploit paths or demonstrated incidents:

| Rule | Count | Disposition |
|---|---:|---|
| `artipacked` | 11 | Accepted-fixed: all checkouts now declare `persist-credentials: false`. No existing checkout options or explicit publishing credentials changed. |
| `excessive-permissions` | 8 | Accepted-fixed: CI declares default `contents: read`, resolving workflow/job inheritance warnings. Publishing retains its existing job-scoped `contents: write`. |
| `cache-poisoning` | 4 | Accepted-fixed: release preflight/build disable Go caches and both explicit/automatic npm caching. Publishing's Go cache was already off. Ordinary CI caches remain unchanged. |

The inverse workflow diff matches the baseline exactly after reversing only
these three approved mitigation groups. Triggers, matrices, job identities,
`needs`, action pins, signing steps and secret scopes are preserved. Disabling
release caches can increase future release setup time. These are risk reductions,
not proof that credentials previously leaked or a cache was poisoned.

## Checker boundaries and known detection gap

The helper sets `--offline`, `--strict-collection`, `--persona=regular`,
`--no-config`, `--no-ignores`, `--no-progress` and JSON output. The child
receives only a fixed PATH and locale, excluding ambient tokens/configuration.
No workflow is executed. Network and persistent-write denial was qualified on
macOS, with only Git's nonpersistent `/dev/null` sink writable.

It validates the required JSON fields, inventoried location, severity/confidence
values, and exit status. Malformed output, tool/collection errors and a 120-second
scanner timeout fail. Nonempty findings fail even if the process exits zero.
A finding marked `ignored: true` still fails: zizmor retains that annotation
metadata under `--no-ignores`, so the wrapper never filters on it. JSON/SARIF
format differences cannot silently turn finding data into a successful result;
the wrapper neither requests nor accepts SARIF as its finding-list schema.

**Known gap:** zizmor 1.32.0 reports omitted `persist-credentials` in regular
mode, but reserves explicit `true` for auditor mode. A future edit from `false`
to explicit `true` can therefore pass this check. This is not safe credential
handling, a waiver or a guarantee that literal `false` is enforced. A dedicated
characterization test records the limitation and must be reassessed on scanner
upgrade. A structural literal-`false` guard needs separate owner approval;
switching the entire persona would also extend the approved policy.

Offline auditing does not qualify current remote action/advisory integrity,
repository secret settings, protected environments or branch protection.
Static analysis is not complete inline-shell/Python execution assurance. Release,
signing, minimum-OS and native Windows gates remain separate.

## Verification and lessons

- `make workflow-security-tests`: 18 cases pass on local macOS and Ubuntu,
  including injection, permissions, omitted credentials, release caching,
  strict malformed/unknown input handling, ambient suppression resistance,
  unexecuted inline code, binary substitution, inventory boundaries and Make
  green/fault/restored recovery with unchanged source/index bytes.
- Real-scanner cases use the pinned binary. Only malformed-output and zero-exit
  boundary cases replace the scanner subprocess; the rest of the wrapper and
  integrity check remain real. The zero-exit case uses hand-written valid JSON.
- The target failed on the real 23-finding baseline before mitigation and now
  reports three workflows with zero findings. Both hosts pass the live scan.
- A disposable missing-finding-exit mutation is rejected by four intended
  injection/permission/zero-exit/Make assertions. No mutant remains.
- The first credential fixtures incorrectly expected regular-mode detection of
  explicit `true`; adding artifact upload did not change persona selection.
  Direct false/omitted/true comparisons in both regular/auditor modes exposed
  the mistaken oracle. Corrected inputs reproduce the actual baseline omission
  while retaining failure assertions; the explicit-`true` gap remains recorded.
- The ambient-ignore test exposed the `ignored` metadata assumption. Its failing
  diagnostic assertion preceded the boolean-schema correction; annotated
  findings remain visible and blocking.
- macOS `make verify` passed, including 742 frontend tests. Existing actionlint,
  shfmt and ShellCheck checks also pass. The checker ran under a sandbox denying
  network and persistent writes. REUSE/SPDX and diff-whitespace checks pass.

Linux qualification uses a task-only pinned artifact and a source snapshot;
no global tools change. Linux application/GUI/packaging, hosted execution,
publication and native Windows were not rerun for this tooling phase. Task
checkouts and duplicate tool/export fixtures are removed after receipts are
retained. Independent reviewers inspect code and receipts; their approval does
not imply they independently reran the tests.

Next is P6's individually reviewed contract hardening. The most relevant security
follow-up is proposing a literal-`false` checkout guard, before implementing any
new blocking rule. P6 has not been started by this work.
