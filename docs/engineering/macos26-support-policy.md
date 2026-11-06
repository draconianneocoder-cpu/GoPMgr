<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# macOS 26 Tahoe minimum support policy

The owner explicitly adopted **macOS 26 Tahoe or later** on 2026-11-05.
Earlier macOS versions are unsupported by current source builds. This is a
product support decision, not a new Go toolchain requirement. The Apple Silicon
architecture policy and Linux/Windows support are unchanged. Previously published
artifacts and dated validation reports retain their original requirements.

## Implementation and acceptance

This bounded slice updates the existing production/development bundle templates
to `LSMinimumSystemVersion=26.0.0`, the Darwin Taskfile to deployment target 26.0,
and the app-build/binding wrappers to the same 26.0 C/C++/linker settings. The
binding wrapper uses Go's selected target OS rather than the host name; an
unavailable target lookup stops before binding generation. Non-Darwin CGO options
are preserved. Nested binding generation retains an already-trailing 26.0 flag
instead of appending it again. Compiler selection and existing wrapper option
precedence are unchanged. No Make scheduler, runtime API, storage format, dependency pin,
workflow or global tool was changed.

Acceptance requires production and development metadata 26.0.0, observed 26.0
compiler/link settings and binding environment, failure on target-lookup errors,
preserved Linux options, and an actual production executable whose Mach-O
minimum matches its bundle metadata. These requirements have local regression
and artifact evidence; native 26 execution remains a release gate below.

The wider [P6-E Make alignment proposal](adr-013-compiler-policy.md) now targets 26
but remains separate and unimplemented. Ordinary Make tests can still compile
with their old settings; they do not establish support below 26 or qualify the
current app artifact. Historical 13.0 characterization has not been rewritten as
26.0 evidence.

## Verification

The build fixtures first failed against the old bundle/compiler floor, then
against the old binding floor. Independent mid-review found that a failed
`go env GOOS` inside an `if` could be ignored. A new regression failed for that
behavior before explicit failure handling was added. A development-bundle
assertion also failed with the old development template before its restoration
to 26. Exact environment-field assertions exposed duplicated inherited binding
flags; those assertions failed before the idempotence correction. A Taskfile-only
13.0 substitution was equivalent under the normal wrapper because the parent
environment wins; it is not counted as an executed defect detection.
The final fixture suite passes, including non-Darwin option preservation
and existing Linux/Windows failure and build paths. These are controlled-command
fixtures, not native Windows or Linux product qualification.

`make build` passes on macOS 27.2/arm64 with Go 1.27.3 and pinned Wails 3.1.0.
The generated production app has bundle minimum 26.0.0 and `LC_BUILD_VERSION`
minimum 26.0. `codesign --verify --deep --strict` passes for the ad-hoc signed
bundle. This is neither Developer ID/notarization nor installer qualification.
The build reuses existing module and compiler caches; it is not a cold-build
benchmark. Build outputs remain ignored local artifacts.

Canonical `make verify` passes, including Go tests, 742 frontend tests, Svelte
checking and build/contracts. ShellCheck and shfmt pass for all 90 tracked
scripts; REUSE 3.3 passes. Existing unflagged Go-test linker and browser-fixture
canvas warnings are not treated as new app-artifact or minimum-OS evidence.
Independent final review approved the complete slice with no blocking findings,
confirmed the actual app header, metadata, binary hash and signature, and assessed
the broader checks from their receipts. All bounded local acceptance criteria pass. Active install/developer guidance, agent instructions,
roadmap and dependency guidance now name 26. Historical migration, containment
and blocked-host records carry a current-policy notice rather than altered
old measurements.

## Remaining qualification and follow-up

- **Native macOS 26 execution is not available on this host.** Current macOS 27.2
  build/header evidence cannot replace a run on the minimum supported release.
  Ubuntu 24.04 and Windows qualification retain their separately documented gaps.
- Installer/install/upgrade, notarization and full release evidence remain open.
  No release or hosted execution is authorized or claimed by this change.
- The hosted assurance source still names `macos-15`, and release source uses
  `macos-latest`. Neither label establishes a macOS 26 qualification environment.
  Before hosted/release use, inspect the actual runner OS and SDK, choose an
  authorized suitable runner, and run the new app there. Do not infer the
  current value of a moving runner label. Workflows were not changed or run.
- Next implementation priority is native Make test/race/coverage alignment with
  the 26.0 baseline. Retain compiler rules, preserve unrelated flags/defaults,
  define conflict handling, and test target isolation before modifying Make.
  Do not hide linker warnings or patch the upstream Wails module cache.

The support-policy change supersedes macOS 13 as a current release target; it
does not turn prior macOS 13 or newer-host measurements into macOS 26 execution.
