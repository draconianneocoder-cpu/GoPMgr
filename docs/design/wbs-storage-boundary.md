<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# WBS transactional storage foundation

The user approved the persistence policies on 2026-10-14: revision-checked
atomic saves, 256 committed receipts per chart, existing encrypted full-text
audit history, conflict-preserving drafts, and unresolved deleted-source links.
This first implementation is the internal database foundation. No Wails method,
chart registry entry, frontend write path, automatic migration, or upgrade uses
it. Existing legacy projects continue through their existing writers.

## Operation contract

`db.OpenWBSStore` is explicit initialization, outside `Database.Migrate`.
It atomically creates three tables and a persistent random 128-bit copy token,
or reopens the existing token. Version1 table definitions (including constraints)
must match the initializer exactly; unknown layouts are refused. A partially
present schema, absent singleton,
or mismatched project is refused. Initialization reserves the writer before
reading schema state, so two initializers cannot issue different copy tokens.

A trusted caller supplies project, immutable actor-incarnation and open-lifetime
IDs. These are not request-controlled credentials; the store does not authenticate
users. The future session owner must invalidate the store before switching
projects, logout, account deletion/recreation, or authorization changes.
`Close` waits for pinned operations and invalidates only this store, leaving
the shared database pool open. Database operations have a ten-second bound,
shortened by caller cancellation. No background work or automatic retries run.

`WBSWrite` supplies copy/chart/lifetime/request IDs, an explicit create flag,
expected revision, title and immutable JSON. IDs use the existing 256-byte
model identifier bound; titles are valid UTF-8 up to 64 KiB. JSON retains the
approved 8 MiB aggregate bound. The writable surface fixes chart configuration
to `{}` and template ID to empty. Other chart metadata is not accepted.

Creation is separate from update: it requires an absent chart and revision
`"0"`, and commits revision `"1"`. Updates never recreate a missing chart.
Submitted aggregate revision must equal the expected canonical decimal token.
Every distinct successful request, including identical content submitted with
a fresh request ID, advances revision and records an audit event. Overflow is
refused. Revisions never pass through JavaScript numbers or SQLite integers.

## Transaction and replay order

1. Pin the trusted store binding; reject a closed store or mismatched copy/lifetime.
2. Strictly parse and freeze the submitted model; normalize empty dictionaries
   to absent dictionaries. Omitted/empty collections serialize identically;
   child order, identifiers and text are preserved.
3. Start a transaction and reserve SQLite's writer with a matched singleton
   update before any database read. Check copy and current project identity.
4. Look up the chart/request receipt before checking today's revision or sources.
   A matching fingerprint returns the original committed revision and audit ID.
   Different content, actor, operation, or expected revision is refused.
5. On a receipt miss, require the current chart and stored revision to agree,
   check expected revision, and validate the supported mutation.
6. Encode with the model's JSON v2 encoder and validate final bytes, including
   revision digit growth, against the aggregate size bound. Write chart content
   and revision, append the existing full before/after audit
   event, insert the receipt, and prune receipts in the same transaction.
7. Commit. Every pre-commit failure rolls back; a commit error is explicitly
   indeterminate and requires reconciliation. Initialization and writes own a
   dedicated `sql.Conn`. A failed commit discards that connection before pool
   reuse because the pinned driver otherwise leaves some failed transactions
   open after `database/sql` marks the transaction finished.

The fingerprint hashes protocol version, operation, copy/project/chart,
immutable actor, expected revision, title and normalized submitted model.
It excludes lifetime, generated timestamps and resulting revision. Receipt
uniqueness excludes actor, so another actor cannot reuse an old request identity.
A reopened authorized session can retry the same immutable request under its
new lifetime. A receipt is historical acknowledgement, not a historical content
snapshot: it must never replace a newer acknowledged draft. Read current content
separately using `Read`, which returns copy/lifetime/chart in one transaction.

Retention keeps the latest 256 distinct committed requests across all actors
per chart, ordered by canonical revision length then binary lexical value.
Replay does not refresh retention; conflicts and refusals consume no slots.
Pruning failure rolls back the entire save. This bounds receipt rows per chart,
not total chart count or full audit-history growth.

Receipt absence does not prove noncommit. An expired request's original expected
revision conflicts with advanced state; clients must preserve their draft and
read/reconcile rather than automatically minting a new request. Restored/cloned
writable copies must receive fresh copy identity before activation, preventing
rollback of revision history from reopening a stale-write window.

## Protection and activation gates

Generic `SaveChart` rejects submitted protected kinds or recognizable
`root`/`schema_version` envelopes. Stored kind, stored envelope or membership in
`wbs_records` blocks generic update/retyping/deletion inside the transaction.
Protection does not depend on successful strict model parsing. It protects new
application writers; it cannot retrofit old binaries or prohibit direct SQL.
Repair, whole-file replacement, import and older-application enforcement remain
separate activation gates.

Create/update reject every nonempty activity or cost association in this first
foundation. Read preserves stored links but makes no successful-resolution claim.
Existing source IDs have no incarnation contract: checking existence alone could
rebind a deleted/recreated ID. Before association activation, source owners must
enforce incarnation and coordinate deletion/replacement with coherent reads.
The approved later policy preserves previously valid deleted links as unresolved,
refuses newly missing/foreign links, derives resolution status on read, and never
rebinds by matching names or reused IDs.

The current account schema permits deleting/recreating usernames and does not
provide an immutable incarnation ID. The future authenticated session integration
must supply one. Copy reset after restore/clone, old-version file protection,
source incarnation, all mutation-path guards, bridge outcomes and frontend
acknowledgement handling must be established before enabling production writes.

Full before/after audit text stays in the same project database, inheriting its
encryption. This deliberately retains sensitive text history. Receipts and
digests are internal metadata, excluded from detached sharing and routine logs;
a digest is neither anonymization nor publisher authentication. No export is
implemented by this foundation.

## Verification

`internal/db/wbs_store_test.go` uses real file-backed plaintext and SQLCipher
databases for replay, close/reopen, immutable actor checks and audit verification.
Two independent handles race at a shared expected revision. Failure triggers
exercise audit insertion, receipt insertion and pruning rollback. Tests cover
receipt 257, expired replay, equivalent optional fields, generic mutation bypass,
missing copy metadata, cancellation, inconsistent revisions, uint64 overflow,
partial schema and deferred-constraint commit failure with connection cleanup.

Native Wails transport, UI conflict recovery, restored copies, old binaries,
source associations and power-loss durability are outside this slice's evidence.
