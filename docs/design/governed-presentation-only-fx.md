<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Governed presentation-only FX for GoPMgr Portfolio reporting

**Status:** Proposed for product and architecture review.
**Supersedes:** [portfolio-fx-governance-options.md](portfolio-fx-governance-options.md).
**Repository review basis:** main through `343982316cbfd920d0b7ddaa442d84170d1d9c53` (2026-08-21).
**Implementation authority:** None. This proposes a bounded FX reporting
capability; it does not authorize Cost Control Phase 2, a portfolio persistence
domain, multi-user financial RBAC, statutory accounting, or transaction-price
functionality.

The [project cost ledger scope](project-cost-ledger-scope.md) remains separate:
FX must not add conversion, provenance, approval, or portfolio behavior to
ordinary project budget, expense, input, material, or overhead tracking.

## Executive decision

GoPMgr retains its mixed-currency Portfolio Analytics refusal as the safe
default. If FX-0 is explicitly approved, v1 would add a separate explicit
**FX-normalized portfolio report** for a user who needs one truthful
presentation-currency total.

The recommended v1 is **direct-pair, presentation-only FX using exact rational
rates, user-attested immutable rate-set files, independently consistent
per-project source snapshots, fail-closed completeness checks, and an export
evidence manifest**.

V1 deliberately excludes a portfolio database, portfolio tables in system.db,
FX tables in project databases, financial roles, mandatory four-eyes approval,
a second FX audit chain, anchor/cross-rate triangulation, online rate refresh,
historical project-state reconstruction, and generic currency exponents. Those
features are not needed to eliminate a false mixed-currency total.

## Repository facts and boundaries

| Current fact | Required v1 consequence |
| --- | --- |
| Portfolio is the post-login view over the signed-in user's project folder. There is no named persistent portfolio. | The current user's enumerated project files define scope. Do not add portfolio persistence to hold FX policy. |
| Portfolio Analytics opens each project with the session DEK, derives metrics in Go, and aggregates in in-memory DuckDB. | FX is a separate Go orchestration workflow before aggregation, never DuckDB or project storage. |
| The existing API explicitly refuses mixed currencies. | Preserve it; FX is an explicit new workflow, not an implicit fallback. |
| Money crosses Wails as canonical decimal strings; the money kernel uses signed int64 minor units. | Source, rate, converted values, and totals never cross Wails as JavaScript money numbers. |
| Money is fixed at 100 minor units per major unit; new JPY use is legacy/read-only. | FX supports only USD, EUR, GBP, CAD, AUD, and CHF. JPY and generic exponent support are excluded. |
| The project Financial Report uses one local read transaction. | The governed path reads each project's input in one consistent snapshot. It does not claim a cross-database transaction. |
| Project audit chains are project-local; Cost Control authority remains proposed. | FX v1 adds neither project audit events nor a competing portfolio authority model. Evidence is in sealed rate-set files and export manifests. |

### Metric scope

FX v1 converts only current Portfolio Analytics money: Budget, Committed Cost,
and EVM AC, EV, and PV where available. It does not add Cost Control ledger or
reserve totals, allocation, forecast/EAC, management-reserve authority, or new
project Financial Report behavior.

## Goals and non-goals

### Goals

1. Produce a truthful presentation total for projects in different supported
   currencies.
2. Preserve every source project currency and source amount exactly.
3. Require an explicit presentation currency and complete direct-rate coverage.
4. Convert with exact source-to-target rates and visible quotation direction.
5. Work offline after user entry or explicit local-file import.
6. Export enough evidence to replay every converted amount without reopening a
   source project.
7. Leave all source project databases and audit chains untouched.

### Non-goals

- Payment, settlement, card, bank, hedge, bid/ask, or executable price
  functionality.
- Exchange gain/loss accounting, tax, statutory close, or GAAP/IFRS/IAS 21
  claims.
- Automatic network fetch/refresh, automatic latest-rate selection, or a
  staleness-policy engine.
- Anchor/cross rates, historical source reconstruction, shared rate books,
  portfolios, or RBAC.
- JPY conversion, arbitrary ISO 4217 exponent handling, and Cost Control
  Phase 2 behavior.

## Product workflow

### Existing quick rollup is unchanged

The existing RunPortfolioAnalytics action continues to succeed only for
homogeneous-currency projects and retains its explicit mixed-currency error. No
rate set is selected silently and no conversion is added to that API.

After a refusal, the UI may offer **Create FX-normalized report**. The user
must opt in.

### Separate governed workflow

PreparePortfolioFXReport, a proposed application workflow, would:

1. enumerate the signed-in user's project files;
2. capture current Portfolio source metrics and currency per project;
3. require one target presentation currency;
4. load an explicitly selected sealed direct-pair rate set or create an
   in-memory draft;
5. validate complete direct coverage for the actual report scope;
6. convert every monetary metric in Go using exact rational arithmetic;
7. aggregate already-converted target minor units through the existing
   analytics engine;
8. show a read-only source-versus-converted preview; and
9. on explicit user export, write the final PDF plus evidence manifest.

Choosing a target, previewing, or editing a draft creates no durable record.
Durable evidence exists only when a set is sealed or a governed report exports.

## Currency and arithmetic contract

### Direct-rate representation

A quote always means:

~~~
1 source major unit = rate target major units
~~~

For example, 1 EUR = 1.0876500000 USD retains rate_text
"1.0876500000", while exact reduction stores rate_numerator "21753" and
rate_denominator "20000". All rate fields are decimal strings because rational
values may exceed int64, and the input text preserves source precision.

The parser admits only positive canonical ASCII decimal values: digits with at
most one decimal point; no sign, exponent, grouping separator, currency symbol,
or whitespace. Length and fractional-precision limits are Go constants in
internal/fx, boundary tested and fuzzed; the frontend must not duplicate them.

### Exact conversion and aggregation

All v1 currencies have 100 minor units per major unit:

~~~
target_minor_exact = source_minor × rate_numerator / rate_denominator
~~~

Use math/big.Int and math/big.Rat, never float64. Round once per project
metric, nearest with exact ties away from zero, to a signed target minor-unit
integer. Overflow outside money.Amount's signed int64 range fails the complete
request without partial output.

Convert Budget and Committed Cost independently. Convert AC, EV, and PV
independently where EVM is available. Sum target line-rounded minor units only
after conversion. Do not average rates or convert display strings. CPI and SPI
remain dimensionless ratios based on converted aggregate EV/PV/AC. Remaining is
converted_budget minus converted_committed, never an independently converted
native Remaining. The displayed total is the sum of displayed rounded lines;
v1 has no synthetic rounding-adjustment line. The manifest retains unrounded
rationals for replay.

The existing ScaleByRatioChecked cannot be the public FX primitive because its
numerator and denominator are int64. FX requires a dedicated validated,
arbitrary-precision, checked conversion API.

## User-private immutable rate sets

### Storage and state

~~~
<data-root>/<user>/fx/
  rate-sets/<rate-set-id>.json
  notices/<notice-id>.json
~~~

Files are user-scoped, restrictive-permission, no-replacement artifacts. They
store rates and provenance, not project money. Do not put them in system.db, a
project database, or a new portfolio database.

~~~
memory-only draft → validate → explicit user attestation → sealed file
~~~

The application never edits or overwrites a sealed set. A correction writes a
new sealed set with supersedes_rate_set_id and supersedes_content_hash. A
future withdrawal is a small immutable notice with original ID/hash and reason;
it blocks future use without modifying past reports.

### Attestation, not false enterprise assurance

Use **attested** and **sealed**, not *certified*. The same signed-in local
account may create and attest the set. Attestation records account, times, and
a statement that the user reviewed direction, date, provenance, and the
presentation-only purpose. Independent approval may be added only after a
reusable authority model and real collaboration need exist.

### Integrity hash and data contract

Normalize quote order, serialize a versioned canonical payload excluding its
hash field, compute SHA-256, and persist content_sha256. Verify it on every
load. This detects accidental or unsophisticated modification; it does not
claim to prevent a local administrator from replacing the file and recomputing
the hash.

A sealed file has this shape:

~~~json
{
  "schema_version": 1,
  "id": "fxrs_...",
  "target_currency": "USD",
  "rate_effective_date": "2026-08-21",
  "purpose": "portfolio_presentation",
  "created_by": "local-account",
  "created_at_utc": "2026-08-21T20:00:00Z",
  "attested_by": "local-account",
  "attested_at_utc": "2026-08-21T20:05:00Z",
  "supersedes_rate_set_id": "",
  "supersedes_content_hash": "",
  "quotes": [{
    "source_currency": "EUR",
    "target_currency": "USD",
    "rate_text": "1.0876500000",
    "rate_numerator": "21753",
    "rate_denominator": "20000",
    "source_name": "User-provided reference source",
    "source_reference": "reference or URL",
    "source_published_at": "",
    "entered_at_utc": "2026-08-21T20:00:00Z",
    "note": ""
  }],
  "attestation": "Reviewed for presentation-only project-controls reporting; not an executable transaction price.",
  "content_sha256": "..."
}
~~~

A usable set has a supported target, valid supported source currencies, one
direct quote per non-target source in current scope, no duplicate or
wrong-target quote, valid positive rate/rational agreement, effective date,
provenance, valid content hash, and no withdrawal notice. The target uses an
implicit identity 1/1; a same-currency quote is never persisted. There is no
newest-rate-set-wins behavior.

## Snapshot, export, and replay contract

There is no atomic transaction across project databases. A governed report is a
**collection of independently consistent project snapshots captured during one
bounded run**. It records capture start/end timestamps and deterministic
per-project capture sequence/time.

The governed path obtains every project's inputs from one consistent read
snapshot. It fails, rather than silently skips, an unreadable or invalid-
currency project. Existing EVM unavailability remains an explicit coverage
exclusion, never an error or substituted zero.

The governed artifact pair is user-selected and no-replacement:

~~~
<name>.pdf
<name>.pdf.manifest.json
~~~

Build both byte streams and hashes before publication, then reserve and check
both selected destinations under the repository's no-replacement rules. Publish
the manifest first and the PDF last: the PDF is the visible commit point. If
PDF publication fails, an orphan manifest is less misleading than a PDF without
evidence. Cleanup of that manifest is best-effort, must be reported, and cannot
be claimed atomic across two filesystem names.

The manifest embeds source project IDs/names/currencies and captured metrics,
EVM availability, full sealed rate-set payload, rate/source timestamps,
algorithm/rounding version, exact unrounded and rounded conversions, target
totals, EVM coverage, final PDF SHA-256, and manifest payload SHA-256. It never
contains DEKs, SQLCipher keys, passwords, recovery codes, absolute private
paths, or unrelated filesystem information.

Keep these hashes distinct:

1. rate-set content hash: quote and provenance payload;
2. manifest/source-result hash: captured evidence and conversion output; and
3. artifact hash: exact final PDF bytes.

The manifest alone replays every converted line, total, Remaining, and CPI/SPI
input after projects or rate-set files change or disappear. Reopening current
projects is not a historical replay contract.

## User experience and disclosure

After the existing refusal, present one secondary action: **Create
FX-normalized report**. Do not display a converted number before completion.

The compact v1 flow is:

1. choose one presentation currency from the six supported currencies;
2. select a compatible sealed set or prepare an in-memory direct-rate draft;
3. review each direct pair as a sentence, such as 1 EUR = 1.0876500000 USD,
   with effective date and provenance;
4. attest and seal only when saving a new set;
5. preview native and converted project values, source-currency subtotals,
   target totals, rate date, capture window, and EVM coverage; then
6. export the PDF plus manifest to a chosen destination.

The preview/PDF shows presentation code, rate-set ID/hash in details, rate
effective date, capture window, every native currency and native/converted
metric, direct rate/provenance, EVM coverage, “source projects unchanged,” and
“presentation/reference rates; not executable transaction prices.” Use ISO
codes beside values; symbols alone are insufficient.

If rate date differs from capture date, show a persistent warning and require
acknowledgement before export. V1 deliberately does not invent an
organization-wide staleness threshold.

## Code boundaries

| Area | Owns | Does not own |
| --- | --- | --- |
| internal/fx | Currency allowlist, parser/rational validation, exact conversion, rate-set validation/hash/files, replay helpers | Project reads, DuckDB, Wails, network, financial authority |
| Application orchestration, for example app_portfolio_fx.go | Project enumeration, DEK use, snapshots, coverage, conversion lines, Wails DTOs, export orchestration | Source-project financial writes |
| internal/analytics | Existing aggregation over target ProjectMetrics | FX parsing, policy, persistence, project access |
| Document/export renderer | Immutable report-model rendering and paired publication | Project reads or FX arithmetic |

All Wails monetary/rate/rational fields are strings. Only SPI/CPI may be
numbers because they are dimensionless; frontend code never calculates money.

## Delivery plan

### FX-0: approve the contract

Approve presentation-only purpose, six currencies, direct pairs, local-user
attestation, provenance, date-mismatch acknowledgement, rounding, and the
PDF-plus-manifest unit. This documentation commit does not complete FX-0 and
authorizes no code, migration, or role system.

**Exit:** an explicit, durable FX-0 approval record names the accepted contract
revision. Only then may FX-1 begin.

### FX-1: exact rate and sealed-set foundation

1. Add internal/fx parser/converter and versioned rate-set structure.
2. Add no-overwrite storage, canonical hash/load verification, supersession,
   and optional withdrawal notice.
3. Add Wails list/load/seal APIs without paths or numeric money/rates.

**Exit:** arithmetic, tamper, no-overwrite, supersession, privacy, and Wails
boundary tests pass. Portfolio conversion remains disabled. FX-2 may begin only
after this evidence and a separate adversarial review.

### FX-2: consistent capture and in-memory preview

1. Capture each source project in one consistent read snapshot.
2. Refuse unreadable/unsupported projects.
3. Preserve source evidence, validate direct coverage, convert in Go, and
   aggregate target integers.
4. Return a disclosure-rich preview DTO.

**Exit:** USD/EUR and six-currency fixtures reconcile exactly; before/after
tests prove no source financial or audit state changed. FX-3 may begin only
after this capture/conversion evidence and a separate adversarial review.

### FX-3: UI, report, and manifest

1. Add the explicit Portfolio action, target/set selection, direct-rate review,
   and attestation confirmation.
2. Add preview/disclosures, PDF renderer, self-contained manifest, and safe
   paired publication.
3. Add manifest-only replay verification.

**Exit:** isolated packaged native evidence covers success, cancellation,
manifest-first/PDF-last partial-publication handling, sealed-set restart/load,
and successor behavior.

### FX-4: demonstrated future need only

Signed reports, independent approval, named portfolios, shared rate books,
provider imports, staleness policy, anchor/cross rates, and generic exponents
need their own decision records. None is required for direct-pair v1.

## Mandatory verification matrix

| Risk | Required proof |
| --- | --- |
| Existing safety | Non-FX Portfolio Analytics still refuses mixed currencies. |
| Pair direction and precision | Known-rational/property tests in both directions; full signed-int64 money; arbitrary-precision rate; no float64. |
| Rounding and overflow | Positive, negative, half, near-half, zero, large-value vectors; target overflow has no partial result. |
| Rate validity | Missing, duplicate, unsupported, wrong-target, malformed, non-positive, tampered, superseded, and withdrawn sets fail before conversion. |
| Source consistency | Injected/interleaved tests prove per-project capture consistency where practical. |
| Omission and mutation | Unreadable project refuses; source rows, currency metadata, baselines, reserves, and audit count are unchanged. |
| Wails/replay/export | Values/rationals are strings; manifest-only verifier reproduces output; hashes match; collisions, cancellation, and faults leave no misleading pair. |
| Privacy/comprehension | No secrets or absolute paths; component/native evidence exposes rate direction, date, provenance, capture, source data, and EVM coverage. |

## Residual risks

| Risk | Honest v1 treatment |
| --- | --- |
| Unsuitable reference rate | Required provenance/attestation; no legal or accounting suitability claim. |
| Rate/capture date mismatch | Visible disclosure/acknowledgement; no historical reconstruction claim. |
| Cross-project capture is non-atomic | Capture window and per-project consistent evidence. |
| Local administrator changes files | Hash detects ordinary modification, not enterprise authenticity. |
| Rate later found wrong | Successor or immutable withdrawal; past report evidence remains unchanged. |
| Reference differs from payment/hedge | Explicit presentation/reference label; no executable-price claim. |
| Cost Control portfolio need | Deferred behind the separate Phase 2 contract. |

## Approval request and acceptance criteria

Approve or amend: presentation-only purpose; unchanged quick-rollup refusal;
six-currency scope; direct pairs; manual/optional local import; local-user
attestation; immutable user-private JSON; explicit capture/rate dates; mismatch
warning; exact big-rational arithmetic; stated rounding; PDF plus manifest; and
successor-only correction.

FX v1 is acceptable only when the existing refusal remains intact, supported
multi-currency reports have visible direct provenance, source projects remain
unchanged, all money has one named rounding boundary, JavaScript performs no
money/rate arithmetic, incomplete/unreadable scope prevents export, manifest
replay succeeds independently, file publication is safe, JPY remains contained,
Cost Control Phase 2 remains untouched, and isolated native evidence verifies
success, refusal, restart/load, cancellation, export, and successor handling.

The precise next work is **FX-0: explicitly approve or amend this bounded
contract and record the accepted revision**. Only after that approval may FX-1
implement exact direct-rate parsing/conversion and immutable user-private
rate-set storage, followed by adversarial review before Portfolio UI conversion
is enabled. Do not start with portfolio tables, role tables, FX audit tables,
or provider integration.
