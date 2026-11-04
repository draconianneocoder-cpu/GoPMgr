<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-013 P0: baseline reconciliation

Recorded 2026-11-04. Status: provisional inventory, P0 exit gate open.
No mise/config/lock/static-gate implementation is claimed.

## Source identity and authority

- Checkout: `migration/wails-v3-gtk4`; production conversion commit
  `122fa63e803b7e99ef3372bf97820e09b664808e`.
- The isolated-shell parent is
  `5c3fefc03bcf0e2189ecc4a1587247c2c22c54e5`. The production conversion is now
  a separate local commit; its unrelated ancestor import fix is preserved.
  ADR-013 introduces only documentation/agent guidance in the following commit.
  Neither commit is published protected main or a qualified release candidate.
- The owner archive contains one 60,061-byte Markdown file. Its extracted
  SHA-256 is `930313e4001a35dfdd7833c8d341aa5954cc32177ff6f4fa8c4f6776671a6a12`.
  The [archived submission](../design/adr-013/v0.3-as-submitted/approved-phased-plan.md)
  is byte-identical, including six original Markdown hard-break lines. These
  intentionally trigger Git trailing-whitespace warnings; preserve them and
  verify the archive hash separately from formatting checks on authored files.
  The [current ADR](../design/ADR-013-engineering-build-assurance.md)
  governs its repository reconciliation.
- No unrelated changes are overwritten, staged or committed by ADR integration.
  The owner authorized a disclosed final self-review fallback for local commits.

## Current ownership and observable contracts

| Owner | Source of authority | Evidence / limitation |
|---|---|---|
| Go/native runtime | `go.mod`, build wrapper and platform tasks | Go 1.27.3, Wails 3.1.0, CGO; no runtime upgrade by ADR-013 |
| Frontend | npm manifest/lock, Vite and generated service bindings | Runtime 3.1.0; 209 App signatures preserved; no mise ownership |
| Rust experiments | Individual toolchain/lockfiles and accepted owner decisions | No production promotion; no Rust/toolchain edits in this integration |
| Native installers | `scripts/release-tool-versions.env` | nfpm v2.47.0; NSIS 3.12.0; fixture compilation is not Windows execution |
| Public commands | Root `Makefile` | Preserve targets and uncached `race -p=1 -count=1`; internal Wails Taskfiles are subordinate |
| CI definitions | Three files under `.github/workflows` | Source reviewed locally; hosted execution and protected-check settings not verified |

## Caller and resource map

| Entry point | Authoritative consumers | Effects / shared resources |
|---|---|---|
| `verify` | CI verify job, maintainer pre-commit procedure | Tests, frontend build/dist, code-map/notices checks and fixture outputs; not a source-read-only command |
| `race` | Maintainer, release script equivalent | First-party Go tests; `-race -p=1 -count=1`; Go caches and disposable DB/KDF files |
| `build` | Maintainer, platform packaging; CI uses same wrapper | Binding generation, `npm ci`, frontend dist, private native staging and exact build/bin publication |
| `dev` | Developer only | Wails development process and Vite 5173; not qualified here as a runtime test |
| `license-check` | Assurance job and maintainer | Currently deletes `.DS_Store` recursively before REUSE lint; P6-B remains open |
| `check-release` / `tag-preflight` | Release preflight and maintainer | Builds, tests, documents/encryption validation; explicit tag contract; REUSE skip branches remain P6-D |
| `package-macos` / `package-macos-installer` | Maintainer; release invokes DMG script after wrapper | App copies, signing, DMG/PKG and package fixture backup/restore; do not overlap package tests with real packaging |
| `package-linux-nfpm` | Maintainer; release invokes same tests/script after wrapper | .deb/.rpm, notices and ELF metadata; nfpm is explicit setup |
| Windows installer scripts | Windows release step | Scaffold -> native wrapper -> notices -> private NSIS packaging; native lifecycle unverified |
| `module-notices`, `frontend-notices`, `code-map` | Explicit maintainer regeneration | Tracked generated files; corresponding checks must reject drift |
| `tidy`, `fonts`, `icc` | Explicit maintainer setup/update | Dependency/source changes or downloads; never relabel as read-only |
| `clean`, `cache-maintenance`, `reset-clean-test` | Explicit maintainer operation | Output/cache removal or recoverable data moves; not automatic provisioning |

Do not use `make -j` until shared-resource ordering is qualified. In particular,
frontend generation/build, package fixture backup/restore and running native
apps can conflict. An aggregate verification attempt correctly refused its
reset fixture while the owned GUI was running; quit the fixture app before
rerunning. Do not bypass this protection.

## Installed-tool observations and initial static findings

Read-only version commands on this Mac reported Go 1.27.3, system GNU Make
3.81, ShellCheck 0.11.0 and actionlint 1.7.12. The default shell selected Node
24.18.0/npm 12.0.2; an existing Node 26.13.0/npm 12.2.1 installation was selected
explicitly for the final frontend test/check run. CI declares Node 26; Ubuntu
package validation used Node 26.8.2/npm 11.19.1. No global versions were changed.
mise, shfmt and zizmor were not available on the checked PATH; no installation
or substitute scanner pass is claimed.

`actionlint -oneline` initially exited 1 with 10 findings: eight unknown
`ubuntu-26.04` runner labels and two SC2012 `ls` selections in release SBOM
steps. The latter selected an unrelated app when it sorted first. A controlled
fixture demonstrated the failure; exact expected bundle paths fixed it and
removed both SC2012 findings. The eight runner findings remain unadjudicated:
verify official hosted-image availability before classifying them as an old
scanner inventory or an invalid workflow target. Do not add a blanket allowlist
or change runner policy merely to obtain a pass. No blocking scanner target was
added, no hosted workflow executed, and no full shell scan is claimed.

## Workflow inventory

- `ci.yml`: push main/master and PR triggers; jobs `verify`, `assurance`,
  `pades-harnesses`, `build`, `lint`, `security`, `vuln`. Ubuntu 26.04 except
  macOS 15 assurance. Actions use commit pins. Default token policy is not
  explicitly constrained at workflow root; assess permissions under P5.
- `release.yml`: version-tag trigger; preflight -> native matrix -> publish.
  Root `contents: read`, publish job requests write. Signing/update inputs are
  separate privilege boundaries; no credentials/settings changed here.
- `adr010-g0-spike.yml`: path-filtered PR experiment, read-only token, matrix
  native evidence. It must not become a production packaging dependency.

Read actual matrices, action commits and source contracts again at frozen P0
revision; this inventory does not assert remote branch protection or successful
hosted execution. Preserve exact job/check names when adding static jobs.

## Verified findings and dispositions

| Finding | Current evidence | Disposition |
|---|---|---|
| Wails v2 baseline in submission | Current migration go.mod/CLI/npm are 3.1.0 | Corrected in ADR; archive remains historical |
| Wails direct workflow build divergence (P6-F) | Workflow build steps now call `scripts/wails-build.sh` | Re-evaluate parity; do not blindly repeat obsolete fix |
| Mutable vulnerability scanner (P6-C) | CI installs `govulncheck@latest` | Open; select/check immutable release in its own scoped implementation |
| License check cleanup (P6-B) | Make recipe deletes `.DS_Store` | Open; separate maintenance from validation with sentinel regression |
| Release REUSE skips (P6-D) | Release script tolerates missing/broken executable | Open; strict release policy/failure injection required |
| Compiler policy (P6-E) | `CC := gcc`; current Apple compiler behavior differs by host | Characterize natively before changing it |
| Make-only usability | Current canonical targets do not depend on mise | Preserve; no claim for not-yet-created tool checks |
| Tool bootstrap trust | Latest API metadata only, no downloaded artifact verification | P1 gate, not a verified supply chain |
| Review availability | Earlier independent migration reviews found/fixed defects; final reviewers hit usage limit; owner approved self-review fallback | Local commit may use disclosed fallback; independent final approval is not claimed |

## Review and current execution status

The owner explicitly approved a disclosed self-review fallback for local
Wails/ADR-013 commits after delegated final review became unavailable. Earlier
migration reviews were independent; final approval is not represented as
independent. This exception authorizes neither hosted CI nor publication.

The initial actionlint run exited 1 with 10 findings. After the tested SBOM
path fix it still exits 1 with eight runner-label findings. This is an advisory
baseline, not a passing mandatory workflow-security check. ShellCheck version
was inspected, but no standalone whole-repository shell scan was run. Neither
shfmt nor zizmor was run. No new tools were installed or mandatory gates added.

## Evidence and proof limits

Current migration positive/negative tests are recorded in the
[Wails evidence](../testing/wails-v3-migration-evidence.md). Reuse the existing
release-scope, Wails pin/generator/build, packaging and config fixtures as the
baseline failure-propagation suite. This document does not claim a new mise or
scanner negative control has executed. The production source revision is now fixed above. P0 still needs complete
clean/dirty checkout comparison and repeatable timing/resource measurements;
independent final review was unavailable and the local fallback is disclosed. No P1 install or mandatory static gate may be claimed
complete from the inventory alone.

Native macOS 27.2 and Ubuntu 26.04 are available. macOS 13/Ubuntu 24.04 execution,
Windows installation and trusted distribution remain unverified release gates.
No tool installation, hosted CI, Git push, release or protected-setting mutation
was performed for ADR-013.

## Frozen build-input identities

SHA-256 of the files in the production conversion commit above, not a signed
release manifest or attestation. Documentation-only integration does not change
these inputs. Recompute at the next P0 baseline change.

| Input | SHA-256 |
|---|---|
| `Makefile` | `7f22b8c1a609e59f029bd70aa998ca75a075fb252ea01476fe4212650115c4dc` |
| `go.mod` | `8113890cebbe06ef9d282a63ff75c4ae51b007a97a0ec469c5fb112d7d5f5421` |
| `go.sum` | `e676f932c97716e10091408d0fbfaa56c4b5dc20fa574e358f92dd4919f00538` |
| `frontend/package-lock.json` | `2ad38fb4052e5da182d1adb923d7adaaec7583bb6f44d5700f514f3a682ec145` |
| `.github/workflows/ci.yml` | `de939465e0843c6a9e83115c5137840e75332bbc13fb151af8634295e4be96ea` |
| `.github/workflows/release.yml` | `a5832a5c374efe5c6dc6b35d9960ee0d57e23ccbb556d09c6642f57b7e2921bb` |
| `.github/workflows/adr010-g0-spike.yml` | `50bac8567a6d5c0f44773aa941d66fbd5a16ee2b9a6176c26f4097e7d47b18d2` |
| `scripts/release-tool-versions.env` | `3327d265a41d99d4b12d925c07074c3c80e9d874e903d3bbaadd5884a10ba195` |
| `build/config.yml` | `7797a4bf93c9e87ae20fb76572a8fb43eb1e1eee39881a42643eb59a48200560` |
| `scripts/wails-build.sh` | `5694860929bfbf20cb55b4608d5365299b999eba8013e4c114211f605f883f43` |
| `scripts/wails-v3-qualification.sh` | `6a5cbcc46be560dbb8abc8de279744784df410fc0bdd6e80e2e4a3c1a7e26d21` |

## Make target inventory snapshot

Static snapshot of the working Makefile. Recipes are not executed by this
inventory; a listed target is not evidence of a passing gate. Rebuild the
inventory at P0 freeze and inspect recipes/callers before modifying a target.

| Target | Direct Make prerequisites | Documented purpose |
|---|---|---|
| `help` | None | Show this help. |
| `build` | None | Build the production app with pinned Wails v3 tasks and native CGO. |
| `wails-build-tests` | None | Verify build failure, bundle, signing and platform contracts in isolated fixtures. |
| `dev` | None | Run pinned Wails v3 development mode (Svelte + Go). |
| `tidy` | None | go mod tidy + npm install. |
| `fonts` | None | Download the bundled TrueType fonts into internal/fonts/assets. |
| `required-font-assets` | None | Verify the tracked IBM Plex (PDF/A baseline, app UI) and Source Sans 3 fonts and their provenance. |
| `reset-clean-test` | None | Move GoPMgr data to a recoverable backup for first-launch testing (quit GoPMgr first). |
| `clean-test-reset-tests` | None | Verify clean-test reset/restore safety in isolated fixtures. |
| `native-isolation-launch-tests` | None | Verify the macOS isolated native-launch harness with a fake app bundle. |
| `icc` | None | Download the sRGB ICC profile for the PDF/A OutputIntent. |
| `check-pdfa` | None | Validate generated PDFs, including signed and CJK ones, for PDF/A-2u conformance using veraPDF (hard gate; GOPMGR_PDFA_STRICT=0 to skip locally). |
| `check-pades` | None | Generate and locally verify an embedded PAdES signed PDF sample. |
| `check-pades-external` | None | Generate a fresh PAdES sample and run available external validators. |
| `check-pades-trusted` | None | Classify trusted-source PAdES evidence; set GOPMGR_PADES_TRUSTED_REQUIRED=1 to require verified CLI trust. |
| `pades-harness-tests` | None | Run deterministic local, external, locking, and trusted-source PAdES shell regressions. |
| `pades-lock-test` | None | Verify PAdES lock acquisition is bounded and never reclaims an existing lock. |
| `check-encrypted-db` | None | Validate SQLCipher encrypted project DB create/open/migration/backup. |
| `test` | None | Run Go unit tests. |
| `race` | None | Run Go tests with the race detector (concurrency gate). |
| `verify` | `wails-build-tests config-check installer-tool-pins windows-installer-scaffold required-font-assets clean-test-reset-tests native-isolation-launch-tests wails-version package-version-lib-test brand-assets coverage-go-cleanup-tests workspace-hygiene-tests pades-publish-test module-notices-check sbom-check module-notices-tests frontend-notices-check frontend-notices-tests package-tarball-tests test code-map-current frontend-stability frontend-build-budget coverage-ledger-current no-text-timestamp-ordering no-raw-import-in-tests` | Fast pre-commit gate: config + packaging/toolchain/font/native-launch/reset/cleanup/code-map/notices contracts + Go tests + frontend checks. |
| `code-map` | None | Regenerate the portable first-party Go package dependency map. |
| `code-map-current` | None | Fail when the checked-in package dependency map is stale. |
| `brand-assets` | None | Verify Bobby branding assets and the Wails native icon source. |
| `coverage-ledger-current` | None | Fail if a *_test.go/*.{test,spec}.{ts,js} file exists that TEST_COVERAGE_LEDGER.md never mentions. |
| `coverage-ledger-drift` | None | Fail if a package-coverage heading in TEST_COVERAGE_LEDGER.md no longer matches live `go test -cover` output. Runs separately from local `verify` because it runs the full coverage pass; the blocking CI assurance job runs it on the pinned macOS baseline used by the ledger. |
| `no-text-timestamp-ordering` | None | Fail if internal/db ORDER BYs on a bare created_at/updated_at TEXT column instead of its _unixnano sort-key sibling. |
| `no-raw-import-in-tests` | None | Fail if a frontend *.test.ts/*.spec.ts file imports another file via Vite's ?raw query, which silently shadows that file's coverage.all statement count to a false 0/0. |
| `coverage-ratchet` | None | Fail only if statement coverage drops below its recorded high-water mark (Go and frontend, tracked independently). Remains a manual signal; the CI assurance job currently enforces ledger-heading drift, not high-water-mark policy. |
| `coverage-ratchet-update` | None | Re-run the ratchet and record any improved marks in coverage-baseline.json. Run this after adding tests; never after a real regression. |
| `coverage-go-cleanup-tests` | None | Verify successful Go coverage runs self-clean and failed runs retain diagnostics. |
| `workspace-hygiene` | None | Report repository scratch classifications and sizes without deleting anything. |
| `workspace-hygiene-tests` | None | Verify the workspace report is confined, symlink-safe, and read-only. |
| `pades-publish-test` | None | Verify PAdES sample replacement preserves recoverable evidence on publication faults. |
| `cache-maintenance` | None | Explicitly reclaim shared Go build/test cache and verify/garbage-collect npm cache; preserves module and node_modules dependencies. |
| `frontend-stability` | None | Run Svelte warning-clean and Sigma regression gates. |
| `frontend-build-budget` | None | Build frontend and enforce route-split bundle budgets. |
| `frontend-smoke` | None | Load + render App.svelte via Vite SSR to catch runtime mount crashes. |
| `release-scope` | None | Verify release gates target GoPMgr-owned source only. |
| `config-check` | None | Parse tracked YAML/TOML and enforce each tool's supported format. |
| `installer-tool-pins` | None | Reject mutable or mismatched native installer tool selections. |
| `windows-installer-scaffold` | None | Validate GoPMgr-owned NSIS templates and Windows release wiring. |
| `linux-runtime-target` | None | Verify GTK4/WebKitGTK6 build and package dependencies without legacy fallback. |
| `wails-version` | None | Verify Wails runtime, CLI, and current documentation pins match go.mod. |
| `wails-cli-version` | None | Verify the installed Wails CLI also matches the go.mod pin. |
| `wails-version-test` | None | Run isolated regression cases for the Wails version gate. |
| `package-version-lib-test` | None | Run isolated regression cases for the shared macOS package version derivation. |
| `help-guide-current` | None | Verify in-app Help Guide covers recent release corrections. |
| `memory-scan` | None | Run the memory-safety hardening gate. |
| `lint-go` | None | Lint Go packages with golangci-lint. |
| `lint-frontend` | None | Lint Svelte + TS with the npm lint script. |
| `lint-all` | `lint-go lint-frontend` | Run both linters. |
| `lint` | `lint-all` | Alias for lint-all. |
| `module-notices` | None | Regenerate third_party/GO-MODULE-NOTICES.txt from the Go modules linked into release builds (run after changing go.mod). |
| `module-notices-check` | None | Fail if third_party/GO-MODULE-NOTICES.txt no longer matches the modules release builds link. |
| `sbom-check` | None | Fail if tools/sbom/components.json does not review exactly what release builds ship (see docs/SBOM.md). |
| `frontend-notices` | None | Regenerate third_party/FRONTEND-NOTICES.txt from the production frontend bundle (needs npm ci). |
| `frontend-notices-check` | None | Fail if third_party/FRONTEND-NOTICES.txt no longer matches the npm packages in the production bundle. |
| `frontend-notices-tests` | None | Unit-test the frontend notices generator. |
| `module-notices-tests` | None | Verify the packaging check that compares a built binary with the Go module notices. |
| `license-check` | None | Verify REUSE/SPDX compliance. |
| `package-tarball-tests` | None | Verify the tarball layout (scripts/package.sh) and its notices with a stub build. |
| `package-linux` | None | Build a Linux tarball on a Linux host. |
| `package-windows` | None | Build a Windows tarball on a Windows host. |
| `package-darwin` | None | Build a macOS tarball on a macOS host. |
| `package-macos` | None | Build a macOS drag-to-Applications .dmg installer. |
| `macos-installer-tests` | None | Verify fixed-location PKG policy and failure-safe publication on macOS. |
| `package-macos-installer` | `macos-installer-tests` | Build a local macOS .pkg installer for /Applications. |
| `package-linux-nfpm` | None | Build .deb and .rpm via nfpm on a Linux host (distinct from package-linux, which builds a tarball). |
| `check-release` | None | Run the full release gate (versions, REUSE, memory-safety, race, frontend, build, encrypted DB, PDF/A, PAdES). |
| `tag-preflight` | None | Validate GOPMGR_RELEASE_TAG and run the full gate before tag-triggered packaging. |
| `clean` | None | Remove build artifacts while preserving source-owned platform assets. |
