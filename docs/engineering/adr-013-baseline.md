<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-013 P0: baseline reconciliation

Recorded 2026-11-04. Status: P0 complete for the source and available hosts
recorded below; independent final evidence review approved P0 exit.
No mise/config/lock/static-gate implementation is claimed.

## Source identity and authority

- Observation checkout: `migration/wails-v3-gtk4`, frozen at
  `bdbdfa3335a3e44ca73df433eaf75bd1856788be`.
- Production conversion: `122fa63e803b7e99ef3372bf97820e09b664808e`;
  isolated-shell parent: `5c3fefc03bcf0e2189ecc4a1587247c2c22c54e5`.
  The unrelated ancestor import fix is preserved. The observation revision adds
  only ADR-013 documentation/agent guidance to those production build inputs.
  It is not published main or a qualified release candidate.
- The owner archive contains one 60,061-byte Markdown file. Its extracted
  SHA-256 is `930313e4001a35dfdd7833c8d341aa5954cc32177ff6f4fa8c4f6776671a6a12`.
  The [archived submission](../design/adr-013/v0.3-as-submitted/approved-phased-plan.md)
  is byte-identical, including six original Markdown hard-break lines. These
  intentionally trigger Git trailing-whitespace warnings; preserve them and
  verify the archive hash separately from formatting checks on authored files.
  The [current ADR](../design/ADR-013-engineering-build-assurance.md)
  governs its repository reconciliation.
- No unrelated changes are overwritten, staged or committed by ADR integration.
  Earlier Wails work used the owner-authorized self-review fallback; P0 has
  its own independent plan/evidence review and does not certify Wails release.

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
| `license-check` | Assurance job and maintainer | P0 observed recursive `.DS_Store` deletion; [P6-B](adr-013-license-check.md) now separates cleanup from validation |
| `check-release` / `tag-preflight` | Release preflight and maintainer | Builds, tests, documents/encryption validation; explicit tag contract; [P6-D](adr-013-release-license-plan.md) removes the P0 REUSE skips |
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
mise, shfmt and zizmor were not available on the checked PATH; no new
assurance-tool installation or substitute scanner pass is claimed. P0 used
the existing required Wails CLI, with task-only compilation on Ubuntu.

`actionlint -oneline` initially exited 1 with 10 findings: eight unknown
`ubuntu-26.04` runner labels and two SC2012 `ls` selections in release SBOM
steps. The latter selected an unrelated app when it sorted first. A controlled
fixture demonstrated the failure; exact expected bundle paths fixed it and
removed both SC2012 findings. The eight runner findings remain unadjudicated:
verify official hosted-image availability before classifying them as an old
scanner inventory or an invalid workflow target. Do not add a blanket allowlist
or change runner policy merely to obtain a pass. No blocking scanner target was
added, no hosted workflow executed, and no full shell scan is claimed.

## Command scope and Make consumers

The root Makefile's `GO_PACKAGES` is `. ./internal/... ./tools/...`.
`test` uses those roots with configurable `GO_TEST_TAGS` (empty by default);
`race` adds `-race -p=1 -count=1`, and `lint-go` passes the same roots to
`golangci-lint run`. Release and memory-scan scripts instead declare
`. ./internal/... ./tools/update-manifest`. Memory scan's vet adds the
`production` tag by default; optional staticcheck/gosec/govulncheck do not use
that tag argument. CI's separate vulnerability job currently runs
`govulncheck ./...`. These are observed scope differences, not an assertion that
all entry points cover the same package set. Review parity in P6 without
silently narrowing the broader checks. No experimental Rust module is executed by the package commands. Memory-scan
also has filesystem heuristics: its `os.Open` helper walks the checkout
recursively (excluding `/vendor/`), so generated/experimental Go text may be
inspected without compiling its module; other heuristics use `main.go`,
`internal` and `tools`. Filesystem scope is not package scope.

Direct script consumers include `check-release.sh` (Make gate invocations),
`package-macos-installer.sh` and `package.sh` (build), and
`validate-pades-parallel_test.sh` (parallel PAdES gate test). Their fixtures stub
or exercise those calls. `release-gate-scope-check.sh` parses exact Make target,
recipe and scope text; `check-linux-runtime-target.sh` checks backend policy;
`workspace-hygiene.sh` verifies repository identity through the Makefile. Their
regression scripts copy/mutate those contracts. Do not casually rename targets
or reformat parsed recipes. Other text matches in font/ICC/PDF/notice scripts
are operator hints or embedded Go `make(...)`, not recursive GNU Make calls.
`verify → brand-assets → check-brand-assets.mjs` also copies the Makefile
into a temporary fixture and invokes `make clean` there, checking preservation
of the native icon source. The config-check tool separately enforces exact
workflow gate names/commands.

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

### Source-derived CI metadata receipt

These are declarations at the observation revision, not executed hosted jobs
or assurance that branch protection requires their status.

| Workflow / jobs | Ordering and native matrix | Cache and failure behavior | Artifacts / privileges |
|---|---|---|---|
| CI `verify`, `assurance`, `pades-harnesses`, `build`, `lint`, `security`, `vuln` | Independent jobs, no `needs`; Ubuntu 26.04 except macOS 15 assurance | Go cache enabled with `go-version-file: go.mod`; frontend jobs use Node 26/npm cache keyed by `frontend/package-lock.json`; no Go cache-dependency-path override; `security` is explicitly nonblocking | `build` uploads `build/bin/`, seven-day retention, missing files error; upload step tolerates failure. Other jobs fail the workflow by default; this is not a required-merge-check claim |
| Release `preflight` | Ubuntu 26.04; version-tag trigger | Go/npm caches and lockfile declarations as above | Full tag preflight before packaging; root token `contents: read` |
| Release `build` | `needs: preflight`; `fail-fast: false`; `ubuntu-24.04`/linux-amd64, `macos-latest`/darwin-arm64, `windows-latest`/windows-amd64 | Go/npm caches as above; native packaging and SBOM steps fail on error | `build/packages/*`, seven-day retention, missing artifact error; installer and SBOM outputs consumed by publish |
| Release `publish` | `needs: build`; Ubuntu latest | Go cache disabled; no npm setup | Downloads merged artifacts into `dist`; requests `contents: write`; signs SBOMs/update manifests through job-step secrets and creates a draft release |
| ADR-010 `evidence` | Path-filtered PR; seven rows: Ubuntu 26.04 x64, Ubuntu 24.04 Arm, macOS 15/14 Arm, Windows 2025/2022 x64 and Windows 11 Arm; `fail-fast: false`; 60-minute timeout | Go cache disabled; no npm setup; evidence scenarios can report FAIL/BLOCKED and are explicitly not a release gate | Always uploads `runner.temp/g0-*.jsonl`; missing files warn, retention not explicitly set; root token `contents: read` |

All executable actions are commit-pinned in these workflow files:

| Action | SHA |
|---|---|
| `actions/checkout` | `3d3c42e5aac5ba805825da76410c181273ba90b1` |
| `actions/setup-go` | `b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` |
| `actions/setup-node` | `949feb2413d6458794dcd2491c4babbbce0c15c1` |
| `actions/upload-artifact` | `cf430e030ddbb5b0abf93d22962f4752f3646cd9` |
| `actions/download-artifact` | `9000827ccba6bdab643e8b6fd33ac0654aef8333` |
| `golangci/golangci-lint-action` | `ba0d7d2ec06a0ea1cb5fa41b2e4a3ab91d21278a` |

Go lint requests v2.14.0. Go action cache-key details and the lint action's
own caching follow the pinned actions' defaults, not a custom key declared
here. Root CI token permissions are unspecified; do not equate that with least
privilege. Release uses the public `GOPMGR_UPDATE_PUBLIC_KEY` variable for build
identity, `GOPMGR_SBOM_PRIVATE_KEY` and `GOPMGR_UPDATE_PRIVATE_KEY` only in publish
steps, and the GitHub token for draft creation. Developer ID/notarization hooks
are not enabled by these source declarations. No secret values, effective
repository permissions, ruleset enforcement or hosted job outcomes were read
or changed. Workflow source hashes below bind this receipt to its inputs.

## Verified findings and dispositions

| Finding | Current evidence | Disposition |
|---|---|---|
| Wails v2 baseline in submission | Current migration go.mod/CLI/npm are 3.1.0 | Corrected in ADR; archive remains historical |
| Wails direct workflow build divergence (P6-F) | Workflow build steps now call `scripts/wails-build.sh` | Re-evaluate parity; do not blindly repeat obsolete fix |
| Mutable vulnerability scanner (P6-C) | P0 CI installed `govulncheck@latest` | Fixed in [P6-C](adr-013-govulncheck.md); immutable v1.9.0 and scanner boundary qualification |
| License check cleanup (P6-B) | P0 Make recipe deleted `.DS_Store` | Fixed in [P6-B](adr-013-license-check.md); explicit maintenance and sentinel/failure regressions |
| Release REUSE skips (P6-D) | P0 script tolerated missing/broken executable | Fixed in [P6-D](adr-013-release-license-plan.md); mandatory licensing, no skip mode |
| Compiler policy (P6-E) | `CC := gcc`; native probes confirm test/app deployment-setting differences | [Characterized on macOS/Ubuntu](adr-013-compiler-policy.md); correction awaits policy approval |
| Make-only usability | Current canonical targets do not depend on mise | Preserve; no claim for not-yet-created tool checks |
| Tool bootstrap trust | Latest API metadata only, no downloaded artifact verification | P1 gate, not a verified supply chain |
| Review scope | Independent P0 plan review passed; inventory corrections incorporated; final evidence review approved P0 exit | P0 review does not replace native Wails release qualification |

## Review and current execution status

Independent P0 plan review approved the measurement method. The inventory
review requested CI metadata, the branding Make consumer and filesystem-scope
clarifications; all three are incorporated. Independent final evidence review
approved P0 exit after checking the raw rows, arithmetic, checkout receipts,
controls and source hashes. This is P0 characterization, not release approval.
No hosted CI or publication authority is inferred from local review.

P0 ran installed ShellCheck 0.11.0 over all 81 tracked `.sh` files under
`scripts/`: exit 1, 53 findings (five warnings, 48 information-level findings;
no error-level findings). The five SC1007 warnings are intentional empty
`VERSION=`/`GOFLAGS=` assignments in fixtures, verified against the source.
The information findings comprise SC2016 (27), SC2086 (10), SC2059 (7),
SC1091 (3) and SC2012 (1). At P0 these formed an advisory baseline, not confirmed
vulnerabilities or approved blanket exceptions. Their later dispositions are
recorded in the [P2 adjudication](adr-013-shellcheck.md). The SC2012 match is a package
listing for display, not executable/artifact selection. P0 added no suppression, autofix
or new blocking gate. Current actionlint 1.7.12 still exits 1 with the
eight runner-label findings recorded above. Neither shfmt nor zizmor ran.

## Evidence and proof limits

Migration assurance remains separately recorded in the
[Wails evidence](../testing/wails-v3-migration-evidence.md). P0 adds current
checkout, timing/resource and failure-propagation evidence below. It runs the
canonical build/verify targets, not all 72 targets: destructive maintenance,
publication and credential-dependent release operations are not benchmarked.
The controlled race probe does not constitute a new full production race run.

Native macOS 27.2 and Ubuntu 26.04 are available. macOS 13/Ubuntu 24.04 execution,
Windows installation and trusted distribution remain unverified release gates.
No mise/static-tool adoption, hosted CI, Git push, release or protected-setting
mutation occurred. Build prerequisites use the already-required pinned CLI.
No P1 implementation is claimed from these observations.

## P0 checkout/performance execution (2026-11-04)

The observation revision is `bdbdfa3335a3e44ca73df433eaf75bd1856788be`.
Its build inputs match the production revision below; it adds only ADR-013
integration documentation. The original checkout/index were clean. Independent
plan review approved the method with four incorporated corrections: separate
bootstrap cache, cache-equivalent race controls, semantic index comparison, and
validator-failure provenance plus successful restoration.

A read-only GitHub branch request matched local `main` and `origin/main` at
`7a2908644d80b9630197930dfc605e19271c0060`. The branch API reported
`protected: false` and protection disabled. No repository settings were changed;
this is a branch-endpoint observation, not a complete ruleset/permission audit.
The measured migration branch is not published main or a release candidate.

### Measurement contract

- Exact-revision disposable clones on macOS 27.2/Apple Silicon and local Ubuntu
  26.04/amd64; no builds or destructive fixtures in the owner's working checkout.
- Private Go build cache and HOME/XDG/temp outputs. Existing Go module and npm
  caches are reused and disclosed. The first build is **empty Go build cache,
  populated dependency caches**, not a clean machine or cold filesystem benchmark.
- macOS uses Go 1.27.3, Node 26.13.0/npm 12.2.1, Make 3.81, Xcode 27.2 and Apple
  Clang 21.0.0, with all four macOS 13 deployment-floor variables. Hardware:
  10 logical/physical CPUs, 32 GiB RAM. This does not qualify macOS 13 execution.
- Ubuntu uses Go 1.27.3, Node 26.8.2/npm 11.19.1, Make 4.4.1, GCC 15.2.0,
  GTK 4.22.4 and WebKitGTK 2.52.6. Hardware: four Intel i5-4570S cores, roughly
  30.3 GiB RAM. These different hosts/tool patches are not a controlled speed
  comparison. The system Go installation and package settings remain unchanged.
- The macOS pinned CLI is copied from the existing task tool and identified by
  digest. Ubuntu compiles the already-required v3.1.0 CLI using a separate
  bootstrap cache. The measurement cache is checked empty before build 1.
- Commands are serial per host, without `make -j`. The two hosts may run in
  parallel; they share ordinary network availability. This is not an idle lab.
  `make build` includes generation, `npm ci`, frontend build, compilation and
  signing where applicable; setup/network effects remain part of its timing.
- After one build, take three successive warm build samples. Verification's
  first sample uses the prepared checkout; take three further warm samples and
  one dirty-state sample. Canonical verification may reuse Go test successes;
  only the separate race control establishes forced execution.
- Record elapsed/user/system seconds, command exit and native maximum-RSS
  counters. Darwin reports RSS bytes; GNU Time reports KiB. Convert to MiB
  using 1,048,576 or 1,024 respectively. These are the tool's reported maxima,
  not summed concurrent process-tree memory. Do not sum them or compare host
  memory as though the collection methods were identical.
- Publish all attempts and the three successful warm median/range only when all
  three comparable samples succeed. No p90, improvement claim, performance
  threshold or tuning decision is inferred from three observations.

### Checkout and failure controls

For the freshly cloned first Mac build, verify clean HEAD before/after;
for subsequent samples and all Ubuntu samples, compare NUL-delimited Git status and index
path/mode/blob/stage records plus staged and unstaged binary diffs. Git index
metadata refresh is not a source-change failure. The dirty variant stages one
README comment, adds a different unstaged comment, and creates an untracked
`P0 notes '資料'.txt` sentinel whose content digest must remain unchanged.
Restore only those owned fixture edits afterward.

The first macOS prepared-verification attempt exited 2 after 17.42 seconds:
the native-launch fixture substitutes an unquoted stub executable path and
cannot handle spaces in `TMPDIR`. The test fails before its intended path
assertion. This is an existing fixture limitation exposed by the chosen
private temp path, not an application data-loss result. Keep the failed sample;
use a separate owned, spaceless temporary directory for the remaining verify
samples. Do not change/disable the test or silently count this as a pass.
A portable-TMPDIR fixture repair remains a scoped test-maintenance follow-up.

Failure propagation is observed by replacing the first existing verify
validator in a disposable clone with a unique diagnostic and exit 73. Make
must fail nonzero with that diagnostic, then the original validator must pass
after bytes/mode restoration. Cache sensitivity uses a disposable Go test
that prints a nanosecond receipt: prime a race-enabled cache hit, invoke the
actual Make race recipe twice, then remove only `-count=1` in the cloned
Makefile. The unmodified recipe must produce distinct uncached receipts;
the mutation must replay the primed cached receipt. Package/path/compiler,
environment, tags and verbosity remain identical. This proves the recipe's
cache semantics for the fixture, not a new full production race-suite pass.

### Observed samples

All values are native tool observations, rounded as emitted by the timer.
The failed Mac attempt remains in the table. `verify-portable-*` uses the
separate spaceless temp directory; the other Mac build samples used the
original private temp directory. CPU columns are seconds, not CPU percentages.

| Host | Sample | Exit | Wall s | User s | System s | Native max RSS | RSS unit |
|---|---|---:|---:|---:|---:|---:|---|
| macos-arm64 | `build-cold` | 0 | 52.86 | 240.43 | 61.21 | 1537392640 | bytes |
| macos-arm64 | `build-warm-1` | 0 | 9.82 | 12.34 | 5.87 | 1487323136 | bytes |
| macos-arm64 | `build-warm-2` | 0 | 7.90 | 12.46 | 5.91 | 1506017280 | bytes |
| macos-arm64 | `build-warm-3` | 0 | 7.83 | 12.46 | 5.81 | 1470349312 | bytes |
| macos-arm64 | `verify-prepared-first` | 2 | 17.42 | 19.86 | 9.87 | 283115520 | bytes |
| macos-arm64 | `verify-portable-first` | 0 | 207.29 | 887.85 | 121.98 | 1032912896 | bytes |
| macos-arm64 | `verify-portable-warm-1` | 0 | 48.51 | 110.55 | 32.68 | 892780544 | bytes |
| macos-arm64 | `verify-portable-warm-2` | 0 | 49.86 | 106.13 | 34.82 | 914325504 | bytes |
| macos-arm64 | `verify-portable-warm-3` | 0 | 55.45 | 100.39 | 30.52 | 900136960 | bytes |
| macos-arm64 | `verify-portable-dirty` | 0 | 47.47 | 105.66 | 32.90 | 876232704 | bytes |
| ubuntu-amd64 | `build-cold` | 0 | 162.30 | 368.67 | 27.81 | 1558700 | KiB |
| ubuntu-amd64 | `build-warm-1` | 0 | 45.77 | 54.75 | 3.19 | 1509316 | KiB |
| ubuntu-amd64 | `build-warm-2` | 0 | 15.72 | 24.42 | 3.01 | 1627140 | KiB |
| ubuntu-amd64 | `build-warm-3` | 0 | 15.61 | 23.65 | 3.06 | 1586004 | KiB |
| ubuntu-amd64 | `verify-prepared-first` | 0 | 460.52 | 1131.44 | 162.89 | 1095524 | KiB |
| ubuntu-amd64 | `verify-warm-1` | 0 | 90.32 | 178.85 | 26.93 | 794008 | KiB |
| ubuntu-amd64 | `verify-warm-2` | 0 | 90.37 | 178.60 | 27.05 | 802188 | KiB |
| ubuntu-amd64 | `verify-warm-3` | 0 | 90.39 | 178.50 | 26.90 | 812668 | KiB |
| ubuntu-amd64 | `verify-dirty` | 0 | 91.58 | 181.30 | 27.60 | 782076 | KiB |

| Host / three successful warm group | Median wall s | Observed range s |
|---|---:|---|
| macos-arm64 / `build-warm-*` | 7.90 | 7.83–9.82 |
| macos-arm64 / `verify-portable-warm-*` | 49.86 | 48.51–55.45 |
| ubuntu-amd64 / `build-warm-*` | 15.72 | 15.61–45.77 |
| ubuntu-amd64 / `verify-warm-*` | 90.37 | 90.32–90.39 |

The first Ubuntu warm build is retained at 45.77 s. Its `npm ci` log reports
35 seconds versus five seconds on the next two runs; generation and frontend
bundle durations are similar. The difference is concentrated in that install/
audit phase, but the underlying latency cause was not instrumented. Do not
exclude the observation, invent a percentile, or attribute a host-to-host speed
difference to architecture or tooling from these samples.

### Executed assurance and exit disposition

| P0 requirement | Executed evidence | Result |
|---|---|---|
| Exact comparable source | Both clones at the observation SHA; build-input hashes unchanged | Passed |
| Prepared clean-checkout commands | Cold/warm native builds and first/three warm verify runs on each host | Passed under declared setup; failed spaced-TMPDIR attempt retained |
| Dirty checkout preservation | One full verify per host; semantic index, staged/unstaged patches and quoted/Unicode sentinel digest unchanged | Passed |
| Essential validator must fail closed | Injected exit 73 with unique marker: Make exited 2; restored validator exited 0 | Passed on both hosts |
| Race must not reuse cached test success | Primed cache replay; two canonical distinct nonces; removal of only `-count=1` replayed cached nonce and failed the same freshness assertion | Passed on both hosts, scoped probe only |
| Fault cleanup | Original script/Makefile bytes and modes restored, probe removed, clones clean | Passed |
| Honest performance interpretation | All 19 attempts recorded; groups of three successful runs checked independently by sorted middle value; native RSS units retained | Passed; no budget/speedup/p90 claim |
| Inventory/review | Scope, Make consumers, source CI/cache/artifact/privilege metadata and platform limits documented | Independent plan/final evidence reviews passed |

Private reproducibility material is retained under `.tmp/adr-013-p0/`: raw
per-command output, native time counters/exits/load, before/after Git receipts,
control logs, source hashes and the temporary analysis drivers. The raw sample
values above are committed; the private absolute paths and full logs are not.
The first Mac cold build starts from the freshly checked-out HEAD and is checked
clean afterward; later Mac samples and all Ubuntu samples have explicit paired
Git receipts. No baseline source mutation remains in the owner's checkout.

To reproduce, clone the exact revision, prepare the pinned Wails CLI in
`.tmp/wails-v3/bin/` using a bootstrap cache separate from measurement, select
Node 26, preserve existing module/npm caches explicitly, and use a new private
`GOCACHE`, HOME/XDG and conventional spaceless temp directory. On macOS export
`MACOSX_DEPLOYMENT_TARGET=13.0` and C/C++/linker flags
`-mmacosx-version-min=13.0`. Example measurement commands after that setup:

```sh
# macOS; replace build with verify for that sample group.
/usr/bin/time -lp -o sample.time make build >sample.log 2>&1
# Ubuntu: GNU Time emits explicit seconds, KiB and command exit status.
/usr/bin/time -f 'real %e\nuser %U\nsys %S\nmaxrss_kib %M\ncommand_exit %x' -o sample.time make build >sample.log 2>&1
```

Make is serial. Follow the sample order above; never call `go clean` or delete
shared caches to manufacture a cold sample.

P0 changes only documentation and agent guidance. No production behavior,
Make recipe, CI definition, tool pin or supported-platform policy changes.
Mise provisioning and new mandatory scanners remain P1–P9 work.

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
