<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-013: Make-centered engineering build assurance

Date: 2026-11-04. Status: accepted maintainer direction; implementation staged.
P0 characterization is complete for the source/hosts recorded in the baseline.
P1 opt-in provisioning is implemented; its [qualification and recovery policy](../engineering/adr-013-provisioning.md)
records evidence and limits. Maintainer acceptance precedes mandatory lint
adoption. P2's [local ShellCheck inventory and adjudication](../engineering/adr-013-shellcheck.md)
is implemented. P3's [local shfmt check and normalization](../engineering/adr-013-shfmt.md)
is implemented under the approved style contract. Hosted/required-gate adoption
remains deferred. P4's [local actionlint check and adjudication](../engineering/adr-013-actionlint.md)
is implemented; P5–P9 are not implemented or accepted as complete.

## Decision and authority

Adopt mise for explicit, verified auxiliary-tool provisioning and add ShellCheck,
shfmt, actionlint and zizmor in separately qualified phases. GNU Make remains
the developer and automation command surface. The owner supplied the
[approved v0.3 plan](adr-013/v0.3-as-submitted/approved-phased-plan.md) and requested
analysis, improvement and integration. That submission is preserved verbatim;
this ADR records its reconciliation with the current repository. The
[baseline](../engineering/adr-013-baseline.md) records executable evidence and gaps.

Direction approval does not certify completed phases, approve publication or
change release credentials, protected checks, signing policy or support targets.
Use local macOS and local Ubuntu for execution. Hosted CI requires explicit
owner authorization. A PR-per-phase suggestion in the submission does not
require spending hosted minutes on each phase: use cohesive local commits and
one reviewed integration PR when authorized. Gates apply to each change even
when publication is batched.

## Reconciliation with the Wails migration

The submission's Wails v2.17.0 observation is historical. The active migration
uses Wails v3.1.0 for Go, CLI and npm runtime, GTK 4/WebKitGTK 6.0, with no v2 or
GTK 3 runtime fallback. See the separately approved
[Wails design](wails-v3-migration-design.md) and
[native evidence](../testing/wails-v3-migration-evidence.md).
ADR-013 neither initiates nor reverses that migration.

Make delegates native application compilation to Wails' checked-in Taskfiles.
Those are an implementation detail of the already approved Wails build, not a
replacement repository scheduler. Keep their ordering and generated-binding
contract. ADR-013 adds no mise tasks, competing Task/Just/Mage entry points,
background orchestration or shared locks. Go, npm/Vite and isolated Cargo/rustup
retain their language graphs. Rust/DataFusion experiments remain excluded from
production; DuckDB remains retired.

The mise-absent recovery requirement concerns build provisioning. It does not
permit a legacy desktop runtime or a GTK backend fallback.

## Provisioning and verification boundaries

- Preserve Go/Wails pins in `go.mod`, the frontend graph in its npm lockfile,
  installer pins in `scripts/release-tool-versions.env`, and isolated Rust pins.
  `mise.toml` may initially provision only the four auxiliary tools. No implicit
  Go/Node/Rust version takeover.
- Pin the mise bootstrap separately from its configuration. Select stable
  versions at adoption, then freeze exact artifacts. Published checksums and
  signed source tags are distinct from verification of a downloaded binary.
- Generate `mise.lock` with the reviewed tool, never hand-author successful
  checksums. Qualify backend/platform resolution, missing and corrupt checksums,
  safe-mode URL behavior, empty cache and wrong executables on PATH.
- No unreviewed hooks, plugins, tasks, environment secrets, shell activation,
  automatic updates, dotfile changes or skills synchronization. Isolate initial
  experiments from the owner's home/config, skills directories and tool caches.
- `make tools-check` is read-only and network-free. It must work with exact
  independently installed binaries when mise is absent. Do not invoke installers,
  `mise exec`, lock generation or source formatting from verification.
- Preserve current Make semantics until a separately tested correction changes
  them. Existing `license-check` cleanup, optional scanner modes and release
  REUSE skips are recorded defects/risks, not descriptions of read-only or
  fail-closed guarantees already achieved.

## Phased implementation and acceptance gates

| Phase | Deliverable | Exit evidence |
|---|---|---|
| P0 | Frozen Make/CI/tool/side-effect inventory | Exact revision, caller/resource map, existing positive and negative controls, platform gaps; independent baseline review |
| P1 | Verified mise bootstrap, minimal config, genuine lock and read-only tool check | Artifact integrity, version/path drift, corrupt/missing locks, safe mode, isolated skills/config, network/cache failure and Make-only recovery |
| P2 | ShellCheck first-party inventory and adjudicated findings | Dialect-aware scope, seeded unsafe script fails, useful fixes tested; no blanket suppressions |
| P3 | shfmt check and documented style | Check does not write; invalid style fails; formatting-only changes separate from fixes |
| P4 | actionlint workflow validation | All intended workflows covered; invalid expressions/schema fail; protected job identities preserved |
| P5 | zizmor security findings and policy | Threat-specific dispositions, narrow exceptions, seeded injection/permission risk detected |
| P6 | Individually reviewed contract repairs | Preserve aliases, failure propagation, serialized uncached race tests, read-only check boundaries and authoritative version sources |
| P7 | Final-artifact identity/provenance pilot | Digest after last signing/packaging mutation; substitution/wrong-ref/missing-evidence rejected |
| P8 | Native and recovery qualification | Applicable OS execution, Make-only/locked-tool paths, offline limits, stale bindings, dirty tree and recovery evidence |
| P9 | Maintainer acceptance and durable policy | Complete evidence, explicit exceptions, independent review and owner sign-off |

P0 precedes provisioning. P1 precedes adopting mise-supplied mandatory tools.
P2–P5 may be investigated independently after P1; serialize edits to shared
Make/workflow files. Advisory means visible findings, not an always-green
wrapper. Blocking status requires a policy decision and a failing negative
control. Do not add a required job before baseline findings are resolved.

The Wails conversion is recorded separately as local commit `122fa63`. Its
local gates and owner-authorized self-review fallback are disclosed; this is
not protected main or a qualified release candidate. P0 records the exact
observation revision `bdbdfa3`, checkout controls,
resource/timing samples and independent evidence review. It is complete within
that declared scope; P1 must still qualify its own provisioning artifacts.
Unavailable minimum OS/Windows tests remain named release gates, not inferred
passes or permission to delete support.

## Improvements to the submitted plan

1. Keep immutable submission provenance and one current ADR, rather than silently
   rewriting the owner's approved document or treating it as executed evidence.
2. Make canonical ownership explicit for Wails' internal Taskfiles; avoid
   introducing a second scheduler while retaining a qualified native build.
3. Separate development integration from release/hosted execution. Existing local
   CI preferences and unavailable-platform limits continue to apply.
4. Pin by actual artifact/backend availability. Latest-release API inspection on
   2026-11-04 reported mise v2026.11.0 with 55 assets, not the submission's 58.
   Asset counts can change and are not integrity checks. No binary/tag signature
   verification is claimed by this inspection.
5. Do not presume the P6 observations are all still defects. For example, Wails
   workflow builds now use the reviewed wrapper. Re-evaluate each row against
   current source/tests before implementing it.
6. Reuse existing fixtures, configuration registry and provenance generators.
   Add tests for a new contract before implementation; do not add parallel
   inventories or weaken old checks to introduce the new tools.

## Version selection and proof limits

Official latest-release APIs were read on 2026-11-04: mise v2026.11.0,
ShellCheck v0.11.0, shfmt v3.14.1, actionlint v1.7.12 and zizmor v1.32.0;
all reported `prerelease: false`. P1 freezes these exact versions in reviewed
provisioning records and checks declaration/lock/artifact consistency. GNU Make 4.4.1 from the submission is not a mandate to
replace macOS' system Make. Qualify required behavior before raising its floor.
Resolve govulncheck independently at P6; the submission explicitly does not
establish a latest version. P1 installs only auxiliary tools in ignored task-local
state; it does not alter runtime toolchains.

Primary sources: [mise release](https://github.com/jdx/mise/releases/tag/v2026.11.0),
[mise lockfile](https://mise.jdx.dev/dev-tools/mise-lock.html),
[mise install](https://mise.jdx.dev/cli/install.html),
[ShellCheck](https://github.com/koalaman/shellcheck/releases),
[shfmt](https://github.com/mvdan/sh/releases),
[actionlint](https://github.com/rhysd/actionlint/releases),
[zizmor](https://github.com/zizmorcore/zizmor/releases).

## Recovery and closure

Each tool integration is independently reversible. Reverting mise configuration
must leave Make with independently provisioned exact binaries usable; do not
revert valid correctness/security fixes merely to remove a scanner. Preserve
release signature and encryption requirements. Production data is never part
of tool experiments. No phase is complete until its actual diff, mandatory
checks, negative controls and review evidence pass. Missing review/platform/
trust evidence stays open in the baseline and roadmap.
