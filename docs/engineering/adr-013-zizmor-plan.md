<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-013 P5: approved local security policy

Status: maintainer approved on 2026-11-05 after independent plan review.
Baseline: `4f7703ad2b3a49276be2f9e47aad5e7749bdaf56`, clean tree/index.
The P1-pinned zizmor 1.32.0 executable was hash-verified before execution.
An offline regular-persona scan of all three tracked workflows, with configuration
and ignores disabled, returned exit 14 and 23 findings.

## Findings and proposed dispositions

| Rule | Count | Evidence and proposed mitigation |
|---|---:|---|
| `artipacked` | 11 | Every checkout omits `persist-credentials: false`. This is a low-confidence credential-persistence risk, not proof that an artifact leaked a token. Set false on all checkouts. No workflow shell step currently declares an authenticated Git push/fetch requirement; explicit publish-step `GH_TOKEN` remains available. |
| `excessive-permissions` | 8 | CI workflow and its seven jobs inherit repository defaults. Declare workflow `contents: read`; leave release publishing's existing job-scoped `contents: write` unchanged. |
| `cache-poisoning` | 4 | Release preflight/build explicitly restore Go and npm caches; setup-node also has automatic package-manager caching. Set their Go `cache: false`, remove npm cache inputs, and set `package-manager-cache: false`. Publishing's Go cache is already disabled. |

These findings identify plausible risk paths, not a demonstrated incident.
The changes reduce credential persistence and privilege, and remove cross-run
cache inputs from release jobs. Disabling release caches may increase future
release setup time; ordinary CI caches remain unchanged. No exception or risk
acceptance is proposed merely to obtain a clean scan.

## Approved invocation policy

`make workflow-security` will be opt-in, local, read-only and offline. A selected
invocation fails on **any regular-persona finding**, regardless of severity or
confidence, and on missing/unverified tools, malformed/skipped inputs, malformed
scanner output or operational errors. There are no severity/confidence filters,
ignore comments, suppressing configuration, autofixes or always-green wrappers.
Use explicit `--offline`, `--strict-collection`, `--persona=regular`, `--no-config`,
`--no-ignores` and JSON output. Never infer a pass from SARIF's zero exit status.
Strip token/config environment influence and verify the pinned executable before
version execution. Reuse P4's tracked workflow inventory and refuse an empty scan.

This does not add a required Make dependency, protected job or hosted run.
Mandatory/hosted adoption remains a separate owner decision. Offline checks
cannot establish remote action integrity, current vulnerability/advisory status,
runner availability, actual secret exposure or protected-environment settings.

## Implementation and validation plan

1. Obtain maintainer approval of this policy and the three mitigation groups.
2. Write real-scanner regression tests before implementation: clean input,
   injection, excessive permissions, persisted credentials, release caching,
   malformed YAML, unknown/missing inputs, ambient ignores/tokens, substituted
   tools, source/index preservation and Make green/fault/restored transitions.
3. Add the small opt-in checker and tests. Validate JSON and findings as well as
   exit status so skipped collection or output-format behavior cannot pass.
4. Apply only the approved workflow changes. Preserve triggers, matrices, job
   names/dependencies, action pins, explicit publishing/signing steps and secrets.
   Preserve checkout options already present. Test local configuration/contract
   guards and repair only directly related witness or inventory failures.
5. Obtain independent mid-work review. Seed controlled faults in disposable
   copies to prove realistic injection/permission regressions are detected.
6. Run local macOS and Ubuntu scanner fixtures, all three real workflows,
   existing actionlint/ShellCheck/shfmt checks, affected shell/configuration
   regressions and canonical macOS `make verify`. Use a write/network-denied
   sandbox to qualify the checker. Run no workflow, publish or signing operation.
7. Review the complete diff independently, correct findings, update ADR/roadmap/
   coverage records and project memory, clean task fixtures and commit locally.

Acceptance: every baseline finding has an evidence-backed disposition; approved
mitigations remove the reported risk patterns without weakening release/job
contracts; clean and negative controls behave correctly; checks are read-only,
fail closed and work with P1's verified binary without mise or network access.
No unapproved exception or unresolved high-confidence security finding remains.
Record unrun platform/online/release evidence explicitly. Independent mitigations
must remain preserved if scanner wiring later needs to be reverted.

References: [zizmor usage and exit semantics](https://docs.zizmor.sh/usage/),
[audit rules](https://docs.zizmor.sh/audits/), and the exact pinned
[setup-node inputs](https://github.com/actions/setup-node/blob/949feb2413d6458794dcd2491c4babbbce0c15c1/action.yml) /
[checkout inputs](https://github.com/actions/checkout/blob/3d3c42e5aac5ba805825da76410c181273ba90b1/action.yml).
