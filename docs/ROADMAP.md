<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# GoPMgr Roadmap

This roadmap describes intended product direction, not a release promise.
Current release-readiness work is tracked in
[beta-release-backlog.md](beta-release-backlog.md). The next limited alpha is
planned in [next-alpha-release-plan.md](next-alpha-release-plan.md). Decisions
that change architecture belong in [design/](design/).

## Current focus

GoPMgr already provides local project storage, scheduling, earned-value
analysis, charts, documents, export, SQLCipher support, and PDF signing. The
near-term priority is making those workflows dependable for wider testing.

1. Qualify the Wails v3 and GTK 4 runtime, merged on 2026-11-09, for release:
   native save/recovery, data and binding contracts on the macOS 26 and
   Ubuntu 26.04 minimums, and Windows installer checks (see below).
2. Prevent data loss in stateful editors and verify save, discard, and cancel
   behavior in native application workflows.
3. Complete real-target installation and upgrade evidence for every advertised
   package format.
4. Establish trustworthy distribution: code signing, notarization where
   applicable, and release-certificate PAdES evidence. The Ed25519 signed
   update-manifest mechanism itself (`internal/update`, `tools/update-manifest`)
   is already built and tested end-to-end; what remains is standing up a live
   channel — generating and provisioning the signing keypair as CI secrets so
   `ManifestURL`/`UpdateChannelPublicKey` are actually populated in release
   builds, which today ship with the update channel disabled by design.
5. Improve keyboard and assistive-technology coverage before broad release.

## Wails v3 and GTK 4 migration (2026-11-03)

The maintainer approved [Wails v3.1.0 and GTK 4 migration](design/wails-v3-migration-design.md)
with no v2/GTK 3 fallback. The isolated shell's core close/save/recovery and
resource-lifecycle gate passed on available macOS and Ubuntu hosts; see the
[evidence and limits](testing/wails-v3-migration-evidence.md).
The migration was merged into `main` on 2026-11-09. It uses Wails v3.1.0, generated bindings that kept
all 209 App methods unchanged (one unused method was removed on 2026-11-09),
and the typed frontend runtime adapter. Local
production bridge/dialog tests pass on macOS and Ubuntu. Local package builds
and macOS/Linux bundled GUI checks pass; Linux packaged GUI under additional outer
containment, installed-package lifecycle and final release checks remain open.
Final integration review uses the owner-authorized self-review fallback after
delegated reviewers reached their usage limit. macOS 26 Tahoe and Ubuntu 26.04 remain
support targets; their execution checks are mandatory before release, as
approved by the maintainer. On 2026-11-09 the maintainer raised the Linux
minimum from Ubuntu 24.04 to 26.04
([decision](review/LINUX-MINIMUM-UBUNTU-26.04-OWNER-DECISION.md)). No full v3 release readiness is claimed.

## Engineering build assurance (ADR-013, 2026-11-04)

The owner approved Make-centered build assurance and bounded mise provisioning.
The [repository ADR](design/ADR-013-engineering-build-assurance.md) reconciles the
[submitted v0.3 plan](design/adr-013/v0.3-as-submitted/approved-phased-plan.md)
with Wails 3.1.0 and the local-CI policy. The
[P0 baseline](engineering/adr-013-baseline.md) is complete for the recorded
macOS/Ubuntu hosts: exact-source clean/dirty checkout controls, cold/warm command
measurements and independent evidence review passed. The failed spaced-TMPDIR
fixture attempt and advisory scanner findings remain explicit follow-ups.
P1 opt-in auxiliary provisioning is implemented with pinned mise, a genuine
platform lock, isolated installs and Make-only recovery. See the
[P1 setup and qualification record](engineering/adr-013-provisioning.md), including
the safe-mode URL limitation and its enforced wrapper mitigation. Maintainer
acceptance precedes mandatory lint adoption. P2's
[ShellCheck inventory and adjudication](engineering/adr-013-shellcheck.md) covers
90 tracked scripts with zero findings and approved, statement-scoped literal
exceptions. P3's [shfmt style and read-only check](engineering/adr-013-shfmt.md)
is implemented; 48 scripts were normalized separately, with all 90 now clean.
P4's [local actionlint inventory and adjudication](engineering/adr-013-actionlint.md)
is implemented for all three workflows. Eight stale-label findings are resolved
by an exact verified label entry; the owner-approved retired G0 macOS 14 row is
removed. P5's [local zizmor policy and mitigations](engineering/adr-013-zizmor.md) are
implemented: all three workflows have zero reported regular-persona findings.
P6's separately approved [checkout guard](engineering/adr-013-checkout-guard-plan.md)
closes the explicit-true detection gap through the existing configuration gate.
P6-B's [licensing check repair](engineering/adr-013-license-check.md) separates
read-only validation from explicit Finder cleanup. P6-D
[mandatory release licensing](engineering/adr-013-release-license-plan.md) requires
successful validation at every entry point and removes implicit cleanup.
P6-C [freezes govulncheck installation](engineering/adr-013-govulncheck.md) at the
verified stable v1.9.0 release and tests text-mode failure propagation. Other
P6 repairs and P7–P9 remain. The separately owner-approved
[x/crypto dependency repair](engineering/xcrypto-advisory-plan.md) removes the
module-only OpenPGP advisory while preserving existing crypto implementations.
[Decompression dependency repair](engineering/decompression-advisory-repair.md)
addresses the subsequently reported called Brotli/compress advisories through
two bounded upstream pin changes. Scanner environment, helper failure handling
remain separate follow-up work.
[Gosec adjudication](engineering/gosec-adjudication.md) traces all fourteen
baseline findings and repairs restore publication/scratch races with the
owner-approved native no-replace contract. Extraction rechecks SHA-256 before
publication. The [escrow length repair](engineering/escrow-field-length.md)
checks the existing uint32 context-prefix limit and preserves frozen v1
ciphertexts without a format migration. The GnuPG output-symlink mechanism is
repaired with the owner-approved
[private staging helper](engineering/gnupg-private-staging-plan.md). Both export
handlers validate output and finish staging cleanup before sidecar publication.
The [scanner-integrity repair](engineering/memory-scan-integrity-plan.md)
propagates required check failures, scopes the checked-in reader and applies
the native environment guard; its grep-based stages fail on operational
errors (2026-11-09). Optional scanner policy remains unchanged; standalone
coverage alignment remains separate.
No finding is suppressed.
[PDF/A validator qualification](engineering/verapdf-provenance.md) now checks
the selected 1.30.2 engine before sample generation, preserves external tools
and propagates validator execution errors. P6-E [native compiler characterization](engineering/adr-013-compiler-policy.md)
confirms a macOS test/app deployment-setting mismatch. Make
[test/race/coverage alignment](engineering/adr-013-native-make-environment.md)
is implemented for five canonical Make paths.
[Standalone release/validator alignment](engineering/adr-013-release-native-environment.md)
extends the guard to release race execution, encrypted-database tests and
PDF/A/PAdES generators. Direct analysis helpers and standalone coverage remain
outside these slices. The owner adopted the
[macOS 26 Tahoe minimum](engineering/macos26-support-policy.md) for current
builds and package metadata; compiler selection is unchanged. No new mandatory
scanner target or hosted run was added.

## DuckDB retirement (2026-11-02)

DuckDB is retired. On 2026-11-02 the maintainer directed its immediate
replacement (macOS first, Linux second, Windows not a priority) and chose a
Go-native engine for both analytics operations; see the
[owner decision](review/ADR-012-DUCKDB-RETIREMENT-OWNER-DECISION.md).

- `PortfolioRollup` sums exact minor units in Go with checked 128-bit
  accumulation and refuses a total that does not fit.
- `ImportTabular` reads CSV, TSV, TXT, JSON and NDJSON with the Go standard
  library, and Parquet with Apache Arrow for Go. It keeps the 10,000-row cap
  and now reports a larger file as truncated instead of omitting rows silently.
- Every difference from DuckDB's output is recorded with its reason in the
  [parity record](testing/duckdb-retirement-parity.md), against goldens
  captured from DuckDB before removal.
- The DuckDB module, build tag, native libraries, notices, TPC-exclusion
  checks and packaging steps are gone. `scripts/verify-module-notices.sh`
  now fails any binary that links a DuckDB module.

For the DF0 branch, the maintainer chose on 2026-11-02 to push it, open its
PR and merge it when checks pass, instead of holding all replacement work for
one final PR as the 2026-11-01 instruction said.

**DataFusion concluded (2026-11-09).** The maintainer closed the DataFusion
spike without adoption: the Go engine covers current analytics, and the
remaining security, performance and licensing gates were large. The
[DF0-G report](testing/adr-012-df0-g-report.md) records what DF0 established
and why. The spike's code is archived at the tag
`archive/adr012-datafusion-spike`; its [evidence record](testing/adr-012-df0-evidence.md)
and raw records stay in `docs/testing/`. Reopening a columnar engine needs a
new owner decision tied to a defined PS-09 need.

Follow-ups:

1. **Linux.** On 2026-11-03 the Go engine was verified on Ubuntu 26.04 with
   the then-current GTK 3 build. Linux packaging now targets GTK 4 and
   WebKitGTK 6.0, and Ubuntu 26.04 is the Linux minimum (see the Wails v3
   section). A real import through the native file dialog has not been
   exercised on either platform.
2. **Done:** the read-only regression tests from the retired Slice B branch
   were ported to `main` in #186 and the branch was deleted.

## Strategic direction: professional scheduling

On 2026-10-26 the maintainer approved a strategic direction: GoPMgr is to
exceed ProjectLibre Desktop in scheduling correctness, project controls,
interoperability, usability, security, documentation and analysis
([decision record](review/PS-professional-scheduling-ACCEPTED-OWNER-DECISION.md)).
The [modernization plan](design/professional-scheduling-modernization-plan.md)
is the canonical plan. It keeps the accepted architecture (ADR-010, ADR-011's
reserved decisions, ADR-012's gates) and changes none of it.

Approved priority order, with today's status from the
[capability matrix](review/PS-capability-matrix.md):

| Work package | Priority | Status |
| --- | --- | --- |
| PS-00 Verified baseline | Immediate | Partial: budgets (OWN-10) open |
| PS-01 Normalized connected model (ADR-011, ADR-010 S1–S4) | Critical, foundational | Proposed |
| PS-02 Schedule-quality audit | High, early parallel | Proposed: [specification](testing/PS-02-schedule-quality-audit-spec.md) |
| PS-03 File inspection and interoperability (ADR-010 G1–G4) | Critical | Partial |
| PS-04 Large-schedule storage, audit and Gantt | Critical | Blocked by architecture (S0 F1, F2); F3 fixed by RES-A-1 |
| PS-05 Connected scheduling workspace | High | Proposed |
| PS-06 Task and Resource Usage | High | Proposed |
| PS-07 Resource and financial planning | High, after PS-01 and PS-06 | Partial |
| PS-08 Advanced scheduling | Medium | Partial |
| PS-09 Analysis, comparison and reporting (ADR-012) | High, parallel | Partial; DataFusion not adopted (2026-11-09) |
| PS-10 Advanced local-first capabilities | Later, separate approval | Proposed |

Nothing in this table is a release promise. Schema activation, migrations,
financial authority, mandatory dependencies, scheduling-semantic changes and
release or signing policy each need separate approval. Within the scheduling
stream, the proposed next implementation slice is
PS-02 slice 1, a read-only audit, which also awaits authorization.

## Planned product work

### Authentication and account security — tiered hardware security keys

- **Tier 1: user-optional FIDO2 USB security keys.** Make an optional, offline-capable, device-bound FIDO2/passkey security-key sign-in capability available to all local users. Enabling key-first passwordless access requires a primary and a separately enrolled backup physical key, independent hardware-mediated unwrap of the user's existing SQLCipher DEK, verified user presence and verification, and preservation of password, recovery codes and administrator escrow. FIDO2 authentication-only devices may support a separately labelled password-plus-key MFA mode, not passwordless project unlock.
- **Tier 2: administrator-enforced FIPS 140-3.** An authorized administrator may turn on a higher security-key requirement. Installation-wide policy is owned by the existing super administrator (ADR-005); subordinate administrators may only apply an already approved policy to authorized standard accounts under explicit delegation. Two independently verified qualifying keys are provisioned before enforcement. Each token must match an **issued NIST CMVP FIPS 140-3** module certificate, hardware/firmware variant, initialization/approved mode, cryptographic service and actual FIDO2+DEK-unlock profile. A password remains available for approved restricted recovery, **not to bypass enforced routine login**.
- **Reliability and recoverability.** Keep the original per-user DEK, encrypted project files, password-derived wrap, recovery codes, authenticated account incarnation, audit and administrator escrow/succession. Require transactional enrollment, no-bypass policy evaluation for GUI/CLI/restore, crash-safe key rotation, independently tested backup key and accessible offline recovery/reenrollment. Disallow synced software passkeys in the initial physical-key profiles. No FIPS/AAL3 full-product claim from a hardware certificate.
- **Roadmap placement.** Define and review architecture, cryptographic profiles, policy scopes, real device/OS proof and risk assessment first (A-00–A-03). Follow with account version gates, Tier-1 enrollment/login (A-04–A-06), then mandatory FIPS policy, privileged recovery and production certification/packaging gates (A-07–A-09). Design/POCs may proceed without disrupting current beta P0 work. Security-key authentication is a trusted host capability, not an ADR-010 plugin, and does not block Jira, MPXJ, scheduling or analytics.

**Companion decisions:** proposed `ADR-TBD-TIERED-SECURITY-KEY-POLICY-v4.md` and the A-00–A-09 roadmap; accepted ADR-001, ADR-004 and ADR-005 retain their current authority. Design-in-principle approval is **not** source code implementation approval.

Approved in principle on 2026-10-28. Tests have not been run. Recorded proposal:

- [ADR-TBD-TIERED-SECURITY-KEY-POLICY-v4.md](design/ADR-TBD-TIERED-SECURITY-KEY-POLICY-v4.md)
- [POLICY-AND-ENROLLMENT-STATE-MACHINE.md](design/POLICY-AND-ENROLLMENT-STATE-MACHINE.md)
- [CRYPTO-AND-STORAGE-CONTRACT.md](design/CRYPTO-AND-STORAGE-CONTRACT.md)
- [ROADMAP-TIERED-SECURITY-KEYS.md](design/ROADMAP-TIERED-SECURITY-KEYS.md)
- [TEST-TIERED-SECURITY-KEYS.md](design/TEST-TIERED-SECURITY-KEYS.md)
- [CRITICAL-REVIEW-AND-DECISION-DELTA.md](review/CRITICAL-REVIEW-AND-DECISION-DELTA.md)

### Provider-neutral integrations

The provider-neutral integration framework was approved in principle on
2026-10-28. The supplemental ADR, contract, staged roadmap, and test plan
are proposed. No implementation work is authorized. The tests have not been
run. C-00 stays design review. C-01 and later packages in the linked roadmap
stay unauthorized.

Current focus and beta P0 release work stay on their current list, ahead of
any Jira connector. A larger-schedule or cloud change to the product vision
needs its own maintainer decision.

SQLCipher remains the authoritative project store. The Go calculation kernel
remains the calculation authority. ADR-011 gates remain in force. DuckDB was
retired on 2026-11-02. Accepted ADR-001, ADR-004, and
ADR-005 remain the authority for current storage, escrow, and succession.

Recorded proposal:

- [ADR-TBD-provider-neutral-integration-framework.md](design/ADR-TBD-provider-neutral-integration-framework.md)
- [CONTRACT-AND-RECOVERY-REFERENCE.md](design/CONTRACT-AND-RECOVERY-REFERENCE.md)
- [ROADMAP-provider-integrations.md](design/ROADMAP-provider-integrations.md)
- [TEST-provider-integrations.md](design/TEST-provider-integrations.md)
- [CRITICAL-REVIEW-and-DECISIONS.md](review/CRITICAL-REVIEW-and-DECISIONS.md)

### Scheduling and controls

- Deeper cost forecasting and formal rebaselining workflows.
- Additional contract, procurement, and stakeholder controls.
- Continued validation of resource leveling and schedule-risk behavior against
  real project data.

### Usability and scale

- Accessible, keyboard-complete workflows.
- Large-project Gantt performance and rendering improvements.
- Localization ([ADR-008](design/ADR-008-localization.md), accepted). Phase 1
  is in progress: the catalog, its checks and the account screens are done.
  The next slice extracts the work-item editors and turns the methodology
  words in `terminology.ts` (for example "user story" for Scrum) into
  catalog messages; until then no screen uses them.

### Interoperability

- Broader project-data import and export compatibility.
- Optional local collaboration only when file-locking and conflict behavior are
  explicit and testable.
- Community extensions only after a narrow, permissioned application boundary
  has proven safe in production use.

## Guardrails

- Local use must not require a cloud account or mandatory network connection.
- New dependencies need a documented maintenance, license, security, and
  packaging review.
- Project data and export compatibility take precedence over feature breadth.
- Release claims require current, reproducible evidence.

## Implemented architecture decisions

| ADR | Decision | Status |
| --- | --- | --- |
| [ADR-001](design/ADR-001-database-encryption-at-rest.md) | Per-user database encryption at rest | Implemented |
| [ADR-002](design/ADR-002-duckdb-vs-sqlcipher-evaluation.md) | DuckDB analytics and SQLCipher project storage | SQLCipher implemented; DuckDB retired 2026-11-02 |
| [ADR-003](design/ADR-003-gofpdf-to-go-pdf-fpdf-migration.md) | PDF library migration | Implemented |

## Not planned

GoPMgr will not require hosted accounts, mandatory telemetry, or online access
for core project management and export workflows. Real-time collaborative
editing remains out of scope until the project adopts a conflict model that
protects local project data.
