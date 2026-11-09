<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-013 P6-C: immutable govulncheck installation

The CI vulnerability job installs `golang.org/x/vuln/cmd/govulncheck@v1.9.0`
instead of `@latest`. This implements the approved P6-C freeze-at-adoption
contract. Baseline `707aa877765a5fcde1aa5e9eb815e9e35c973f03` had a clean tree/index.

## Selection and scope

On 2026-11-05, the [official module proxy](https://proxy.golang.org/golang.org/x/vuln/@latest)
resolved v1.9.0, tag commit `e672cbe8fbc8c76e28fcb6db0efd9ce354e0dac5`.
The [tagged module](https://go.googlesource.com/vuln/+/refs/tags/v1.9.0/go.mod)
requires Go 1.26.0, compatible with the repository's Go 1.27.3 requirement.
Installed macOS scanner module metadata identifies v1.9.0, built with Go 1.27.2.
No replacement binary, toolchain or application dependency was installed.

The workflow remains the executable pin authority. Tests consume its decoded
installation and scan steps through the existing YAML parser and execute their
Bash bodies with controlled tool boundaries. They require the exact versioned
installation operand, unchanged text-mode `./...` arguments, visible diagnostics
and failure propagation. A timeout bounds each subprocess. They do not duplicate
YAML parsing or add a production validator or dependency.

Only the install selector and an inconsistent comment changed in the workflow.
The removed comment suggested weakening a job already described as blocking.
No job, trigger, condition, runner, permission, action pin, package scope or
credential changed. Branch protection was not inspected or qualified. Local
`memory-scan` retains its existing optional-tool behavior; P6-C does not silently
make that separate target mandatory.

## Plan and acceptance

1. Verify latest stable upstream metadata, language minimum and installed scanner.
2. Demonstrate baseline failure through decoded workflow execution before editing.
3. Freeze the install selector; preserve text-mode finding exits and diagnostics.
4. Test actual-scanner finding detection and unaffected recovery in an isolated
   versioned module/database fixture; controlled subprocess exits alone are
   insufficient. Run focused and canonical checks on local hosts.
5. Review the first implementation and final diff independently; update the
   roadmap, ledger and private project memory and commit only this task.

Acceptance: installation requests the frozen release, failed installation/scans
remain nonzero, exact text-scan scope is preserved, actual synthetic findings
block, and healthy/unaffected scans return zero. No JSON/SARIF finding evaluator,
policy waiver, hosted run or scanner-mode change is introduced.

## Qualification and lessons

The workflow test first failed because the actual install operand was `@latest`.
The real installed scanner detects synthetic `GO-9999-9999` and exits 3, then
exits zero after the affected version range is corrected. The fixture adapts the
pinned upstream fake advisory to an exported safe test function, served through
a task-only file module proxy and local vulnerability database. No vulnerable
third-party code is required. Checksums are disabled only for this synthetic
local proxy; repository and global Go settings are unchanged.

Main-module and local-replacement attempts returned clean results and were not
accepted as finding-detection evidence. Using a versioned dependency exposed the
intended finding. This illustrates why a clean scan or a fake nonzero executable
alone cannot qualify a vulnerability scanner.

Final local results:

- macOS configuration package tests and `make verify` pass (742 frontend tests).
  Focused decoded-step tests pass on Ubuntu with matching final input hashes.
  actionlint/zizmor report three clean workflows. No hosted job was launched.
- Three disposable regressions restore a floating pin, suppress a finding exit,
  or mask it with a pipeline; intended assertions reject all three. The shell
  harness matches the workflow's
  unspecified Linux shell (`bash -e`), without imposing stronger `pipefail`
  behavior than [Actions documents](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax).
- The macOS repository scan exits zero with no reachable findings. It reports
  one module-only advisory, detailed below. This is not an advisory-free graph.
- A native Linux scanner is not installed; no installation was performed.
  Actual finding/recovery qualification is macOS evidence; Ubuntu evidence
  covers the workflow command boundary, not native scanner execution.

Live vulnerability data remains mutable independently of the frozen scanner; a clean result is bounded
by the current build/platform, code reachability and database contents. Full
release, signing, hosted and minimum-OS evidence remain separate.

## Maintenance and follow-up

At a deliberate upgrade, recheck upstream stable metadata and language support,
update the workflow pin and independent expected operand together, then repeat
actual finding/recovery and command-failure checks. Never resolve `latest` during
required scanner installation. Review vulnerabilities rather than suppressing a
nonzero result. P6-I's machine-readable finding-policy work stays separate; this
job continues to use text output and its finding-sensitive exit status.

## Discovered dependency finding and fix plan

`govulncheck -show verbose ./...` reports
[GO-2026-5932](https://vuln.go.dev/ID/GO-2026-5932.json) for required
`golang.org/x/crypto` v0.58.0. The affected OpenPGP packages are not imported or
reachable in the current macOS scan. This was deferred from the scanner-pin slice and is addressed by the separately
owner-approved [dependency repair](xcrypto-advisory-plan.md); it is not a waiver
of a reachable finding. The advisory's
fixed main-module boundary is v0.58.1-0.20261009181718-2f1f834330b2. Updating
versions must never be presented as making the deprecated OpenPGP package safe.

No fixed stable tag existed at repair time. The owner approved the exact fixed
pseudo-version; its evidence and platform limits are recorded in the repair
report. This cryptography dependency change stays separate from the scanner pin
so compatibility evidence remains reviewable. Historical scan results above
remain scoped to their original dependency revision.
