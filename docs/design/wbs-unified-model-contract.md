<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Unified local-first WBS model contract

Status: Pure model and the bounded internal transactional storage foundation are
approved; production activation, migration and UI remain gated. On 2026-10-13
the user approved local-first
file-based sharing, designing one authoritative WBS across linked views, and
requiring the new application version for upgraded WBS files. These decisions
do not approve the exact persistence/API contract, financial Phase 2, or claim
that old-version protection has been implemented. On 2026-10-14 the user approved
256 committed receipts per chart, existing encrypted full-text audit retention,
atomic revision/receipt/audit writes and unresolved deleted-source links. The
[storage boundary](wbs-storage-boundary.md) records the implemented foundation
and its explicit production activation gates. The first pure
model/identity/revision/validation slice and the limits below were approved earlier.

## Ownership and identity

| Owner | Authoritative information | Other views |
| --- | --- | --- |
| WBS | Containment, ordered children, titles, classifications and dictionary definitions | Tree, outline and dictionary edit the same breakdown |
| Schedule | Activities, precedence, dates, calendars, assignments and execution progress | WBS shows explicitly linked activity records |
| Cost ledger | Exact amounts, currency, classification and financial states | WBS shows associated ledger contributions |

Keep existing valid chart/node IDs. Identity is scoped by project and WBS chart;
display codes are derived from order and are never reference keys. New identities
require sufficient entropy plus uniqueness checks, not a collision assumption.
Imported copies receive new target chart/node IDs, with an explicit remapping.

Preserve multiple existing breakdowns without merging or deleting them. The
workspace explicitly selects one active breakdown, and all scope views refer to
that same record. Alternative breakdowns are not additive project totals.

Each upgraded working copy has a distinct persistent copy identity and each open
project lifetime has a server-issued handle. Writes bind both, project/chart
identity and expected revision. A stale request after switching projects or
copies is rejected, even where copied records retain the same IDs. A handle is
not authorization by itself; ordinary current-project checks remain mandatory.

## Candidate storage and write protocol

Prefer one versioned JSON scope aggregate in the existing chart store over a
second scope database. The candidate protected chart kind is `wbs_v2`; this is a
design name, not an existing registry entry. Its envelope distinguishes schema
version, server-issued edit revision and immutable snapshot identity. Revision
tokens travel as decimal strings, never precision-sensitive JavaScript numbers.

Nodes extend existing fields with dictionary content and optional classification.
Activity links identify a scope node, schedule chart and activity. Cost links
identify a scope node and ledger entry. Dates/amounts are not editable copied
fields inside the scope aggregate. Unsupported schema is read-only; unknown
fields are preserved losslessly or editing is rejected, never silently dropped.

Logical payload for the approved pure model slice:

| Field | Meaning and ownership |
| --- | --- |
| `schema_version` | Format discriminator, value 2 in the pure model; not an edit counter |
| `revision` | Server-issued canonical decimal token; submitted values do not establish authority |
| `root` | Existing recursive node shape, with preserved `id`, `title`, `note`, `owner`, `effort`, ordered `children` |
| Node `classification` | Optional scope classification; unspecified legacy values are not inferred from titles |
| Node `dictionary` | Plain-text `description`, `scope_in`, `scope_out`, `acceptance_criteria`; no separate live narrative copy |
| `activity_links` | Entries containing `node_id`, `schedule_chart_id`, `activity_id` |
| `cost_links` | Entries containing `node_id`, `cost_entry_id`; no embedded live amounts |

Copy identity, open-lifetime handle, expected revision and request identity belong
to the server-bound operation context, not trusted import metadata. Classification
is optional uninterpreted text; no vocabulary or business rule is inferred.
Receipt retention is 256 distinct committed requests per chart; replay does not
refresh that bound. Production session and recovery integration remain gated.

### Approved pure validation boundary

`internal/wbs` is an isolated model package, not a registered chart kind or an
application write/import path. `Parse` performs strict JSON preflight and typed
decoding, then `Validate`; failures return no partial aggregate. The pinned Go
1.27 standard-library JSON v2 decoder rejects duplicate decoded keys, invalid
Unicode and case aliases. Unknown members and all explicit nulls are rejected.
Unsupported schemas cannot be edited through this parser; a future read-only
viewer must retain the original bytes rather than reserialize a partial model.

| Limit | Boundary |
| --- | --- |
| Payload | 8 MiB original input including whitespace; also maximum encoded model size |
| Nodes | 10,000 distinct identities within one aggregate |
| Hierarchy | 64 levels, root at level 1; separate preflight container-depth bound |
| Text | 64 KiB decoded UTF-8 bytes per field, including classification and display number |
| Identifier | 256 decoded UTF-8 bytes; nonblank and preserved without normalization |
| Associations | 50,000 activity and cost rows combined |

Schema version 2, a canonical revision string, root and every node ID are required.
All identifiers in association rows are required. Other node text may be omitted
or empty; dictionary and its text fields, children and association arrays may be
omitted. Omitted and empty collections are semantically equivalent. Display
`number` is retained as metadata, never a link key. Effort remains any finite
legacy generic-unit value, without inventing a nonnegative or duration policy.

Common validation rejects nil children, cycles, repeated node identities and
aliases, excessive hierarchy, invalid text, nonfinite effort, missing/nonleaf
scope targets and duplicate source assignments. Activity identity is the tuple
`(schedule_chart_id, activity_id)`; cost identity is `cost_entry_id`. Referenced
source records are not queried or implicitly resolved. External existence,
project ownership and unresolved-link detection still require the owning domains.

`NextRevision` accepts canonical decimal strings from zero through `uint64` maximum
and rejects overflow rather than wrapping. This does not decide the initial
persisted revision or issue server authority. `CheckWriteContext` validates and
matches copy/project/chart/lifetime identity and expected revision; it is only a
pure predicate, not authorization, a lock or a database compare-and-swap.
Callers must not mutate an aggregate concurrently with validation. No ID generator,
receipt store, transition command, projection or migration is implemented here.

A dedicated internal WBS store accepts expected revision and stable request
identity; no production Wails API exposes it yet. In one transaction it
compares revision, replaces the aggregate, advances
revision, records the request result and appends the audit event. A pre-commit
failure rolls back all effects; a commit error requires outcome reconciliation.
Source links are refused in this foundation until incarnation-aware validation
is established. Repeating a committed request returns its original result;
reusing its identity for a different payload is refused. Lost acknowledgement
must not cause a second revision or audit event. Stale writes keep the draft and
present a conflict; they never overwrite automatically.

Receipts identify the committed revision, not necessarily the latest revision.
A replay or delayed response must not replace a newer acknowledged draft/revision.
Store request digests and committed-revision references, not another full copy of
every aggregate. Define bounded receipt retention and uncertain-outcome recovery;
after a receipt expires, reconcile against current state rather than retrying a
write blindly. Current authorization is still checked on replay.

New-app generic mutation paths must reject protected stored records or submitted
v2 payloads. Changing the submitted kind cannot bypass protection. Include save,
retype, deletion, repair and import paths in the mutation inventory. Registered
frontend writes retain their single-writer ordering; backend revision checks
cover additional windows/callers that the frontend gate cannot protect.

WBS-family routing, chart pickers, document references, report preflight, layout
and export must recognize both legacy and v2 kinds without rewriting v2 as v1.

## Associations and structural transitions

Within one breakdown, an activity or ledger entry is associated with at most one
leaf; a leaf may reference many activities/entries. No inferred links from matching
names, display codes or IDs in another file. Schedule chart selection is explicit.
Each source record remains editable only through its owning domain.

Adding children to a linked leaf requires explicit association reassignment or
unlinking first; do not guess a destination or silently lose context. Moving a
subtree preserves identity and associations without creating dependencies or
changing schedule dates. Deleting a subtree enumerates its affected associations
and removes only scope/links, never activities or posted ledger records. Undo is
a new revision-checked action and cannot recreate a source record deleted later.

For separately deleted activity/cost targets, retain an explicitly unresolved
association with an actionable warning and exclude it from live totals. Validate
links in the WBS transaction, and make target deletion coordinate with association
validation so the result is either a valid link or an explicitly unresolved one.
Out-of-band changes are detected on read, never rebound to similarly named data.

## Financial and progress projections

For each financial state separately:

`project ledger total = unique mapped entry contributions + unmapped entries`

Ancestor subtotals, classification views and alternative WBS views are derived
presentations, never additional contributions. Do not sum displayed hierarchy
rows. Exclude schedule EVM budget/actual fields from ledger totals unless a
separate reconciliation policy is approved. Do not convert resource estimates
into posted expenses or guess shared-cost splits.

Retain exact checked arithmetic, decimal-string wire values, currency/JPY rules,
and immutable baselines. Association is not funding allocation. Phase 2 reserve,
financial-authority and forecasting decisions remain separately gated.

Legacy effort stays in its recorded generic units, not automatically hours or
elapsed duration. Initially show explicit activity status counts and source
percentages, not invented weighted parent progress. Weighted summaries and
actual-hours correction rules need approved equations before implementation.

## Legacy dictionary and migration

Original documents remain intact. Reconciliation records document ID/version,
source row and reviewed target node. Even apparently unique code-only matches
need human confirmation because historical codes may have been renumbered.
Conflicting, missing and duplicate rows remain visible; no first-row-wins policy.
Future dictionary documents are explicitly generated, versioned snapshots, not
another editable authority. Mixed scope/schedule/cost reports include each source
revision or frozen values; a WBS revision alone cannot reproduce their contents.

Legacy files open without automatic upgrade. Upgrade is explicit and uses a
consistent database snapshot, not a raw copy of an open SQLite file. Preserve
source encryption, transform a private staging copy, reopen it and validate
content, links and audit, then publish a distinct working copy without overwrite.
Only validated copies are published; interruption leaves the original untouched
and incomplete staging artifacts clearly unusable. Backup success includes
reopen verification. Old and new copies must not be mistaken for one another.

### Old-version protection is a release blocker, not a solved claim

A new kind rejects ordinary legacy saves that submit that unknown kind, but an
older caller can submit `kind: wbs` against the same ID, or invoke deletion.
Current `App.SaveChart` checks the submitted kind and `db.SaveChart` allows kind
replacement. New-app guards cannot retrofit those older binaries.

Before enabling upgrade, select and validate an enforceable protected-format or
file-level mechanism against actual older applications, including save, retype,
delete and whole-file repair. A schema marker, filename change or UI warning is
not sufficient evidence. Keep migration disabled until this gate passes; no
unapproved encryption/key-hierarchy redesign may be introduced to bypass it.

Legacy export is a separate, explicitly lossy projection with an omission preview.
It cannot replace/downgrade the canonical v2 record or its file. Support for new
applications reading legacy files does not imply old applications can safely edit
upgraded files. Retain a tested legacy-compatible copy for older installations.

## File-based sharing

Initial exchange is a detached, versioned JSON scope snapshot and review document,
not a project database, credential archive, synchronization or automatic merge.
Export allowlists selected fields. Preview the exact frozen payload that will be
written; changes after preview require a new preview. Exclude account material,
keys, certificates, attachments and financial/context details by default.
Free-text titles, owners, notes and dictionary fields may still contain sensitive
information: show retained text and require review, without claiming automatic
redaction. Publish atomically without overwriting an existing file.

Initial JSON/review-document exports are unencrypted; warn before publication.
Do not imply that encrypted project storage encrypts exported files. Encrypted
exchange would require its own reviewed protocol, not sharing account keys.

Imported source references are inert provenance; they cannot resolve against
destination IDs automatically. Import creates a separate breakdown with remapped
IDs; any external association requires explicit local rebinding. Optional source
context remains labelled historical/read-only and never contributes to live cost
or progress calculations. No chart or partial association exists on failure.

Define byte/node/depth/string bounds before implementation and validate them
before recursive processing. Reject duplicate JSON member names, duplicate node
IDs, invalid structure and malformed references. Treat all text as data: no code,
HTML execution, external fetch or importer-directed filesystem access. A checksum
supports integrity, not publisher identity or permission to trust the payload.

## Verification and first implementation slice

| Risk | Required counterexample or fixture |
| --- | --- |
| Old writer damage | Actual older binaries save/retype/delete/repair protected copies |
| Copy/lifetime confusion | Switch copies/projects with a pending write; no wrong-target mutation |
| Concurrent or uncertain writes | Stale revisions, lost acknowledgement, repeated request, audit failure |
| Link races and restructuring | Delete target during save; linked leaf gains child; subtree delete/undo |
| Duplicate financial totals | Parent plus child rows, alternative views, schedule EVM fields, unmapped entries |
| Dictionary misbinding | Renumbered codes, duplicate rows and conflicting documents remain reviewable |
| Partial upgrades | Interrupted plaintext/SQLCipher staging and publication; original survives |
| Hostile exchange | Duplicate keys/IDs, limits, foreign refs and failed remap create no partial import |
| Disclosure or export drift | Preview bytes equal exported bytes; excluded fields absent; no overwrite |
| Snapshot inconsistency | Reports identify all source versions and remain reproducible after edits |

The first pure aggregate validation/identity/revision slice is implemented in
`internal/wbs`, without enabling legacy upgrade or changing existing project data.
Its tests are in `model_test.go`; broader rows in this risk map still describe
future integration evidence. Production API/storage changes need their own
bounded plan and review. Native UI layout/interaction work additionally needs
the previously planned visual selection and usability baseline.

Independent review first required six contract corrections: legacy-write
protection claims, copy/lifetime identity, link/retry races, linked-leaf
transitions, double counting and sharing trust boundaries. They are addressed
above. The revised documentation-only packet was approved for handoff; actual
compatibility enforcement remains unresolved and gates upgrade enablement.
The earlier documentation-only pass checked REUSE on 834/834 files and found no
trailing whitespace or conflict markers. That historical pass ran no model or
migration tests; [current model validation evidence](../testing/wbs-model-validation-2026-10-13.md)
is recorded separately.
