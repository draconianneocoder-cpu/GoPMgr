<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-013 P6-D: mandatory release licensing contract

Status: owner-approved on 2026-11-05; implemented and locally qualified. Baseline
`ec561f1ec10989d806939081e18fb72b11aee7d8`; tracked tree/index initially clean.
The script now requires successful REUSE validation. Make recipes, workflow
files and tool pins remain unchanged.

## Evidence and priority

Before this repair, `scripts/check-release.sh` deleted `.DS_Store`, then skipped
licensing when REUSE was missing or its version probe failed. Its direct invocation and `make
check-release` shared that behavior. `make tag-preflight` first checks presence
only, so an installed but broken REUSE could still reach the skip. The release
workflow calls tag preflight; it installs the pinned validator, but installation
is not proof that it can run. No documented best-effort mode exists in the script.

Disposable characterization of the unchanged script confirms missing/broken
REUSE proceeds to a downstream Go marker and deletes Finder metadata. A
noncompliant validator stops before that marker, but still deletes metadata.
These are controlled caller-boundary fixtures, not completed release runs.

This release-claim gap ranks ahead of P6-C scanner pin maintenance and P6-A
setup usability. P6-B fixed the Make licensing target; it did not fix this script.

## Approved contract

Require successful REUSE licensing validation for direct script execution,
`make check-release`, and `make tag-preflight`. Missing executable, failed version
probe, or failed lint must stop before later gates, with actionable diagnostics.
Retain the existing pinned installation hint and tool/dependency versions.
Remove Finder cleanup from release validation; cleanup is available only through
P6-B's explicit maintenance target. This does not make the entire release gate
read-only: its builds, tests and package checks retain their existing effects.

Do not add a best-effort flag, environment override or optional release mode.
Developers can invoke existing individual targets when they need partial checks;
those do not establish a full release-gate result. Preserve target names, tag
validation, release workflow, credentials, required-check identities and all
unrelated gates. No hosted execution, signing or publication is authorized.

An explicit best-effort developer mode was considered and rejected. It adds another
policy branch and risks a partial result being mistaken for release assurance;
no caller requiring it was identified. Keeping the baseline skip behavior would leave
the confirmed release-claim defect open.

## Bounded implementation and verification plan

1. Obtain owner approval of mandatory validation for every release entry point.
2. Add real-script regression fixtures before edits. Exercise healthy,
   missing, non-executable/broken, noncompliant and recovery paths. Assert
   forbidden downstream work and Finder/source/index preservation in failures;
   assert actual lint invocation, visible failure diagnostics and continued
   downstream work only after successful licensing. Do not run real packaging.
3. Remove implicit cleanup. Keep presence/version diagnostics, but exit on
   either failure rather than skipping. Delegate lint to existing `make
   license-check`, with its failure propagated; do not maintain a duplicate lint
   recipe or introduce a second version authority.
4. Exercise direct script, Make check-release and tag-preflight callers,
   including present-but-broken REUSE and invalid-tag precedence. Preserve the
   current serialized, uncached race-test contract and existing gate order
   except the necessary licensing failure stop. Record any required order change
   before implementation. Fixtures must reach the licensing boundary rather
   than passing because an unrelated earlier prerequisite failed. Use the real
   copied Makefile at the licensing boundary; replace only unrelated expensive
   dependencies, so a fake Make command cannot hide a broken delegation.
5. Run focused tests on local macOS/Ubuntu, shell lint/format checks, canonical
   verification and real licensing validation. Prove test sensitivity with
   disposable skipped-validation and restored-cleanup mutations. Bind cross-host
   receipts to final input hashes. Full release qualification remains separate.
6. Independently review the plan, first meaningful implementation and final diff;
   resolve findings, update ADR/roadmap/coverage ledger and project memory, remove
   task fixtures and commit only reviewed changes.

Acceptance: every release caller rejects absent/broken/noncompliant licensing;
no licensing success or release success is printed for a skipped check; healthy
validation continues normally; Finder metadata survives validation; existing tag,
race and unrelated failure contracts remain intact. No new bypass or dependency.

Rollback would restore a known release-assurance gap and requires an explicit
policy decision. This change involves no application data or file migration.

## Qualification and limits

Regression tests failed against the original script before implementation. The
fixtures run the real copied Makefile for licensing delegation, while replacing
only known prerequisite checks and the downstream race process. Successful
licensing deliberately reaches that failing race process; it does not claim a
successful full release run. The matrix covers all three entry points, absent
and broken executables, failed version/lint, tag rejection and recovery, with
source/index/Finder sentinels preserved. The existing tag-presence check remains
unchanged, including its precedence when REUSE is absent.

Missing and broken REUSE now stop with the existing pinned installation hint.
The common `make license-check` owns lint execution and exposes validator errors.
Finder cleanup is available only through explicit maintenance. No bypass flag,
new mode or dependency was introduced. Other optional helper branches elsewhere
in the release script are unchanged; this contract is specific to licensing.
Full signing, packaging, minimum-OS and hosted release qualification are separate.

Final local evidence:

- `make license-check-tests`: eleven test methods pass on macOS and Ubuntu,
  including the three-entry-point negative/healthy matrices. Final cross-host
  source hashes match before and after fixture execution.
- Actual REUSE 6.2.0 and real Make delegation pass in minimal repository
  fixtures for direct and tag entry points on both hosts; source/index/Finder
  sentinels remain unchanged. Later race failure is intentional containment.
  This is not full-repository Ubuntu licensing or a completed release run.
- macOS `make verify` passes, including 742 frontend tests. Live repository
  `make license-check` passes. The existing release scope fixture suite and
  race-argument regression pass; ShellCheck/shfmt pass for all 90 scripts.
- Disposable skipped-validation and restored-cleanup mutants fail the intended
  regression assertions. Neither fault remains in the checkout.
- Existing macOS deployment-target linker warnings remain outside this tooling
  fix. No full release, signing, packaging, hosted CI or minimum-OS run occurred.

The lesson from the first test draft was to reject fixture errors as proof:
an escaped-newline bug initially stopped a fake prerequisite before licensing.
After repairing that harness, baseline failures demonstrated the actual skip,
cleanup and absent-delegation defects. Tests assert downstream arrival or its
absence, validator calls and preserved bytes rather than exit status alone.
