<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# PDF/A validator version and failure handling

Implemented locally on 2026-11-05 against baseline `24e68c9`. The validator
remains veraPDF 1.30.2. No toolchain, PDF format, workflow or hosted gate changes.

## Problem and design

The gate previously accepted any executable named `verapdf` on PATH. This host's
Homebrew path named 1.30.3 while its executable reported 1.30.0. The legacy
broken-jar refresh path could also delete a PATH-selected wrapper. Neither a
filename nor a successful compliance fragment establishes the selected version.

The bounded repair keeps the existing pin instead of upgrading the validator or
trusting installation paths. The selected CLI or complete Docker command must
successfully report exactly one `veraPDF 1.30.2` identity line before sample
generation. Other output, including Java warnings and build metadata, is allowed;
duplicate, conflicting, malformed, missing or failed identity reports are not.
Reported-version consistency is not binary authenticity. Installer SHA-256
verification remains separate; the Docker version tag is not a digest pin.

PATH executables are never removed or replaced. A new managed installation is
version-checked before promotion. An existing unusable cache, including a
symlink, is refused with explicit repair guidance rather than overwritten.
Installer and promotion failures propagate. A validator's per-document execution
must also succeed; positive-looking output cannot override a nonzero exit.

Strict mode fails unmet preconditions. The existing `GOPMGR_PDFA_STRICT=0` local
mode explicitly skips unmet preconditions; a skip never reports conformance
success. Actual document
validation failures still fail regardless of that local setting. Release entry
points continue to require strict PDF/A validation and mandatory REUSE checks.

## Plan and acceptance

1. Verify the existing official installer checksum and actual version output.
2. Add failing behavioral regressions for identity, preservation and installation.
3. Probe the selected command before sample mutation, preserve external tools,
   and check installed candidates before promotion.
4. Reject document execution errors, even with compliant-looking output.
5. Run shell checks, harness regressions, real pinned PDF/A validation and native
   Ubuntu negative-path contracts; review the complete diff and record limits.

| Risk | Required evidence | Result |
|---|---|---|
| Wrong/ambiguous or failed validator selected | CLI and Docker fixtures, strict/non-strict outcomes, sample sentinel | Pass |
| External executable mutated | Legacy-jar wrapper bytes and permissions unchanged | Pass |
| Bad installation promoted or old cache destroyed | Wrong candidate, existing directory/symlink and failed promotion fixtures | Pass |
| Validator execution error hidden by output | Zero-status identity followed by nonzero validation with positive XML | Pass |
| Correct validator blocked | Warning-bearing successful probe; all nine CLI/Docker fixture invocations | Pass |
| Real pinned engine incompatible | Official checksum-verified 1.30.2, all nine generated PDFs | Pass on current macOS |

## Verification and review

`bash scripts/validate-pdfa-lib_test.sh` passes. Three tests were observed failing
against defective behavior before their fixes: wrong identity reached generation,
wrong installed identity was promoted, and failed document validation was accepted.
These executed regressions demonstrate sensitivity; no injected defect remains.
Fixtures use disposable roots and controlled external-command boundaries. Their
compliant XML proves harness behavior, not real document conformance.

The official [1.30.2 installer](https://software.verapdf.org/rel/1.30/verapdf-greenfield-1.30.2-installer.zip)
matches the existing SHA-256
`6cc6341cb1af644044054b81f00a6590a7918abb18f762243de115258bcad838`.
Its task-local installation reports 1.30.2. With that executable first on PATH,
`GOPMGR_VERAPDF_FORCE_CLI=1 make check-pdfa` passes all nine samples: schedule,
charter, multilingual, Japanese, Korean, Chinese, combined report, Monte Carlo
risk report and signed document. The system-installed tool remains untouched.
Java emits upstream reflective-final-field warnings; these are not compliance
failures, but future Java compatibility is not established by this run.

Native Ubuntu executes the same identity/install negative-path portion from
hash-matched script inputs. It does not run the real validator or PDF generators.
The first Ubuntu run exposed a fixture-only wildcard `chmod` that followed tool
symlinks; it was corrected to name only fixture executables, then rerun green.

Local ShellCheck and shfmt checks pass for all 90 tracked scripts. Canonical
macOS `make verify` passes, including Go tests, 742 frontend tests, Svelte checks
and build/contracts. REUSE 3.3 passes for all 2073 files. Independent plan,
mid-work and final reviews approved the bounded repair with no blocking findings;
final test execution was assessed from receipts, not independently repeated.
All in-scope acceptance criteria pass. The actual system CLI mismatch is also
refused with existing sample bytes unchanged. No hosted, minimum-OS, real-Docker,
packaged-release or remote-signing qualification is claimed.

## Recovery and follow-up

For an identity failure, supply the exact 1.30.2 CLI on PATH or use the pinned
Docker image. Correct PATH explicitly to allow the repository-managed installer
when no CLI is present. An unusable managed cache needs deliberate repair; the
gate does not delete it or a system tool. Retained samples are not new validation
evidence after a failed probe.

Keep the earlier x/crypto report's 1.30.0 result as historical evidence. This
1.30.2 run supplies the previously missing pinned-validator evidence for this
local source tree, not general release approval.

Next planned work is ADR-013 P6-E compiler-policy characterization: inspect the
effective compiler and deployment target on macOS and Ubuntu before proposing
Make changes. The recorded `CC := gcc` policy and newer-object/older-target linker
warnings need native evidence; do not infer minimum-OS compatibility or change
toolchains from those warnings alone.
