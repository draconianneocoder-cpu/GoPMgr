<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-013 P6-B: separate licensing validation from cleanup

Status: implemented under the approved P6-B contract.
Baseline: `cd271a19013098fd0182c970e42f7f32752dfdcc`, clean tree/index.

## Problem and bounded plan

`make license-check` recursively deleted `.DS_Store` before running REUSE.
Validation could remove local files even when the validator was missing or failed.
This is P6-B in the approved submission; runtime/application work is unaffected.

1. Reproduce deletion in disposable fixtures using the real Makefile.
2. Move the existing cleanup recipe to explicit `make clean-finder-metadata`.
   Preserve its existing matching/traversal semantics. Do not invoke maintenance
   from validation, verification, packaging or release prerequisites.
3. Keep `make license-check` as `reuse lint`, with visible diagnostics and
   nonzero Make status for missing, broken or noncompliant validators.
4. Add a fixture regression target to canonical verification. Check sentinel and
   index bytes, exact lint arguments, failure/recovery and explicit cleanup.
5. Qualify local macOS and Ubuntu, inspect the diff, document findings and commit
   only this bounded repair. Do not launch hosted CI or perform live cleanup.

## Acceptance and evidence boundaries

| Risk | Required evidence |
|---|---|
| Implicit deletion during checks | Source, nested Finder metadata and index sentinel bytes unchanged on success, failure and missing tool |
| Suppressed or skipped validator | Actual Make invocation observes `reuse lint`; nonzero exit/missing executable fails; a file named after the target cannot skip it |
| Lost cleanup or accidental adoption | Explicit maintenance removes matching fixture entries only; failed traversal propagates; no dependency from checks |
| False-positive tests | Regression fails against baseline; controlled cleanup/failure bypass mutations fail intended assertions |
| Host or tool differences | Same final Make/test inputs on local macOS and Ubuntu; real REUSE result kept separate from controlled validator tests |

The fixture validator records arguments and supplies controlled exit statuses;
it does not establish REUSE's licensing correctness. Real `reuse lint` checks the
repository separately. Read-only here describes the Make recipe, not containment
of an arbitrary executable substituted on PATH. No scanner/tool pin changes.
Explicit maintenance retains the old recursive deletion behavior and must be
invoked deliberately by the maintainer. It is not rollback or recovery machinery.

## Related finding: P6-D follow-up

P6-B originally left the release script's independent Finder cleanup and
missing/broken-REUSE skip behavior open. The separately owner-approved
[P6-D contract](adr-013-release-license-plan.md) now requires successful licensing
for every release entry point and removes that cleanup. It reuses this Make
license gate. Neither slice claims the whole release process is read-only or
that signing, packaging and publication have been qualified.

## Results

- Baseline fixture tests reproduced deletion on success, validator failure and
  missing tool before the recipe change. All seven final tests pass on macOS
  and Ubuntu. Source and index sentinel bytes survive; maintenance runs only
  when explicitly requested, including when a same-named file exists.
- Actual `make license-check` with REUSE 6.2.0 passes on the macOS repository
  with source/index hashes and task-owned ignored Finder metadata unchanged.
  Ubuntu's hash-bound minimal repository fixture passes the real check too;
  that fixture does not establish full-repository Linux licensing compliance.
- Canonical macOS `make verify` passes, including 742 frontend tests and the
  new license regression target. Existing deployment-target linker warnings
  remain; no minimum-OS or release qualification is claimed.
- Two disposable mutations restore implicit deletion or suppress validator
  failure. Each fails its intended regression assertion. No mutant remains.
- The implementation moves the original cleanup command unchanged. Tests use
  the real Makefile; controlled subprocess statuses test failure propagation
  without running destructive maintenance in the checkout. No new dependencies,
  scanner pins, workflow or release-script edits, tools installation or hosted CI.

The main lesson is to test prohibited effects as well as exit status: a correct
validation result does not excuse deleting files first. Missing and broken tools
must be distinguishable from successful validation at the caller boundary.
Independent plan and mid-work review approved this scope; final diff review and
local commit state are reported in the task handoff.
