<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# x/crypto advisory: bounded dependency repair

Status: exact upstream pin approved on 2026-11-05; implementation locally qualified; independent final review approved.
Baseline
`2e2f081299550174aa31dc2bca2b448ec4a50bdf`; tracked tree/index initially clean.
Only x/crypto is upgraded in the selected graph. Runtime application source,
cryptographic formats, workflow and toolchain pins remain unchanged.

## Evidence and decision

The [official advisory](https://vuln.go.dev/ID/GO-2026-5932.json) affects the
unmaintained OpenPGP packages bundled in `golang.org/x/crypto` v0.58.0.
The previous macOS scan found the module advisory but no affected package imports
or reachable calls. This does not establish safety of other builds or OpenPGP.

On 2026-11-05 the official module proxy and upstream tags still identify v0.58.0
as newest stable. The advisory's fixed boundary is the exact pseudo-version
`v0.58.1-0.20261009181718-2f1f834330b2`, commit
`2f1f834330b2d36ab3db8bfb33aaa1ac16794588`. Its parent is the v0.58.0 tag commit
`8fdf07b3114278fb3b78c1cf1814e08bd851eb4c`. It carves OpenPGP into a separate
module; it does not repair the deprecated package's design.

A normalized comparison of both official module archives found 51 removed files,
all under `openpgp/`, with no other added or changed files. All nine Argon2 files,
all 15 PKCS#12 files, and root `go.mod`/`go.sum` are byte-identical. The Go minimum
remains 1.26.0. Source equality reduces compatibility risk; it is not a substitute
for exercising the application and resolved module graph.

Decision: adopt this exact reviewed upstream commit with local qualification. This removes unused deprecated code from the selected
main module without changing the crypto implementations GoPMgr uses. Do not use
`master`, a moving selector, a replacement fork or an additional OpenPGP module.
The alternative is retaining v0.58.0 until a fixed stable tag exists, explicitly
retaining the module-level advisory and rechecking reachability meanwhile.

## Implementation and validation plan

1. Obtain approval to use this exact pseudo-version because no fixed stable
   release exists. This is a dependency decision, not a toolchain upgrade.
2. Capture baseline focused auth/crypto/users tests and the actual resolved graph;
   use existing compatibility tests first. Add behavior regressions before any
   necessary code repair if investigation exposes a gap or defect. Do not invent
   version-string tests or change cryptographic formats to obtain green results.
3. Update only x/crypto through checksum-verified Go module resolution. Check
   that every other selected module and the Go requirement remain unchanged.
   Do not use blanket upgrades. Stop for an unexpected graph or API change.
4. Refresh module notices and SBOM/version records only through repository
   generators where their authoritative inputs require changes. Explain any
   generated difference; retain existing licenses and public attribution.
5. Run password/Argon2, encrypted-buffer/account recovery, PKCS#12 and CMS/PAdES
   regressions, canonical macOS verification, and relevant native Ubuntu tests
   against a hash-bound final snapshot. Run PDF/A/PAdES gates where available;
   keep missing external signing/validator evidence distinct from unit proof.
6. Rerun verbose vulnerability analysis. Require removal of this main-module
   advisory and confirm the separate OpenPGP module is absent from the resolved
   graph, as well as affected imports remaining absent in supported builds. Do not suppress
   findings, disable checksums or add unsafe OpenPGP to make metadata pass.
7. Independently review the plan, first meaningful change and final diff. Resolve
   findings, update dependency documentation/roadmap/ledger and private project
   memory, clean task fixtures and commit only validated task-owned files.

Acceptance: selected x/crypto equals the approved fixed commit, other module and
Go pins stay unchanged, existing crypto behavior and formats pass regression
checks, generated notices are current, and the advisory is absent from the
selected main module without introducing affected OpenPGP imports. No hosted CI,
push, publication, live user-data migration or new global tools are authorized.

If a check fails, investigate and correct within this scope or stop before
commit. A dependency revert would restore the documented module advisory; it
would require an explicit risk decision. This proposal does not qualify a
release artifact, minimum OS, remote signing or the deprecated OpenPGP API.

## Implementation evidence and limits

Default `sum.golang.org` verification accepts the module and manifest checksums;
`go mod verify` passes. Before/after module lists differ only in x/crypto and
contain no separate OpenPGP module. The regenerated release module notices change
only two version references; the existing SBOM metadata check passes without an
unnecessary manifest rewrite. Old Go checksum entries are retained as emitted by
targeted module resolution; they do not select the old module for builds.

Existing password and encrypted-buffer tests primarily generated and consumed
new data in the same version. Two persisted fixtures were captured through public
GoPMgr APIs while v0.58.0 was still selected. They passed before the dependency
change and now verify candidate decoding/authentication, wrong-password refusal
and modified-tag rejection. These are cross-version compatibility oracles, not
independent proofs of Argon2 or AES-GCM algorithm correctness. PKCS#12 import and
CMS/PAdES tests retain their existing independent certificate/verification paths.

The baseline verbose scan reported GO-2026-5932; the candidate scan does not.
The current macOS package graph has no affected OpenPGP imports. The fixed commit
separates deprecated code from this selected module; it does not make OpenPGP
safe, and no OpenPGP implementation was added. Full release, remote signing,
minimum-OS and hosted execution remain separate gates.

Local validation evidence:

- Before and after the pin change, focused auth/crypto/users tests pass on macOS.
  Frozen legacy password and ciphertext fixtures pass with both dependency
  versions. Disposable password-bypass and salt-layout faults fail the intended
  compatibility assertions; no fault remains.
- Native Ubuntu auth/crypto/users tests pass using Go 1.27.3. All 1909 archived
  runtime/build/test inputs match the macOS snapshot before and after execution.
  Selected modules and tested-package imports contain no OpenPGP.
- `go mod verify`, generated module notices and SBOM checks pass. The verbose
  macOS scanner no longer reports GO-2026-5932.
- `make check-pades` passes local PAdES-T verification. Strict `make check-pdfa`
  passes nine samples using the installed veraPDF 1.30.0 CLI; this is not the
  repository's pinned 1.30.2 validator evidence (see the finding below).
- Existing macOS deployment-target linker warnings and Linux SQLCipher
  discarded-qualifier compiler warnings remain outside this dependency change.
  They are not treated as proof of older-OS or release readiness.

Canonical macOS `make verify` passes: Go tests, configuration and packaging
contracts, Svelte checking (zero errors/warnings), all 742 frontend tests across
88 files, frontend build and repository guards. Independent adversarial reviews
approved the plan, first implementation and final ten-file diff. Final review
found no blocking findings; test execution was assessed from receipts rather
than independently repeated. All in-scope acceptance criteria pass.
No hosted, full-release, remote-signing or minimum-OS qualification is implied.

## Related validator drift discovered during qualification

The installed PATH veraPDF reports 1.30.0, while the repository's installer and
Docker pins name 1.30.2. Its executable wrapper is under a Homebrew 1.30.3 path;
that directory name is not evidence of the running validator version. The
existing CLI fallback accepts an executable without checking its reported
version. Any PDF/A result from this host must therefore name the observed
validator and must not be represented as pinned-1.30.2 release evidence.

This is deferred to a separate validator-provenance repair. Plan: reproduce the
mismatched-version acceptance in a disposable fixture, agree the authoritative
validator pin/update policy, then check the reported CLI version before use.
Reject a mismatched or broken executable with clear remediation. Qualify the
approved exact artifact and PDF/A samples; preserve read-only validation and do
not remove or replace a system-installed executable. No global tool change is
part of this crypto dependency repair.
