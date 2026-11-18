<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Identifier generation: bounded Slice C repair

**Review date:** 2026-11-14. **Baseline:** `main` at
`44d9f6de883af21c6fef6e981cef0010c33a1d1c`.
This repair addresses generation width, not the whole Slice C contract.
Collision-safe creation followed on 2026-11-18 (see the last section).

## Contract and compatibility

The [maintainer directive](../review/architecture-directive-2026-10-27.md)
and ADR-011 O-2 require cryptographically generated 128-bit identifiers for
new authoritative records, retaining existing IDs and references.

`internal/db/ids.go` and `internal/agile/agile.go` previously read only four
random bytes. They now read 16 bytes with `io.ReadFull(crypto/rand.Reader, ...)`
and retain the existing `prefix_` followed by lowercase hexadecimal encoding.
The new body has 32 characters representing 128 random bits. This is the
permitted equivalent scheme, not UUIDv4 syntax. Catalog and deletion-log
generators already use this encoding and width.

No existing ID is rewritten, and no schema, migration, writer level, prefix,
public Go signature or wire field type changes. IDs remain opaque strings in
TEXT columns and JSON fields. The helpers still return entropy errors without
fallback. Existing storage/update, audit-reference, backup and legacy-read
tests remain applicable; installed older applications are not qualified here.

## Focused regression evidence

Each generator has one primary public-creation contract test:

| Test | Failure detected | Independent expectation |
| --- | --- | --- |
| `db.TestProjectIDRequiresFull128BitEntropy` | Short IDs; incomplete entropy accepted; partial project/audit persistence | Literal `prj_30313233343536373839616263646566` from the 16 ASCII input bytes `0123456789abcdef`; 15 bytes must return `io.ErrUnexpectedEOF` with no project or audit event |
| `agile.TestWorkItemIDRequiresFull128BitEntropy` | Short IDs; incomplete entropy accepted; partial work-item persistence | Literal `wi_30313233343536373839616263646566` from the same 16 input bytes; 15 bytes must return `io.ErrUnexpectedEOF` with no work item |

Both tests failed against the four-byte implementation: it returned
`prj_30313233` / `wi_30313233` and accepted the incomplete input. Both pass
after the width repair. Existing zero-byte entropy-failure tests are retained.
Reader replacement is test-only, restored with `t.Cleanup`, and not parallel.
These deterministic fixtures establish full input consumption and encoding;
they are not a statistical proof of randomness or collision impossibility.

The first repository verification exposed an existing encrypted-corruption
fixture's layout assumption: with longer IDs, byte `120*4096+1` no longer
made the stakeholder query fail. The severe-corruption fixture now locates
`idx_stakeholders_project`'s root page and the page size through SQLite
metadata before closing the file. Corrupting that page reproduces the same
observable contract: project open succeeds, the stakeholder query fails, and
repair fails at `SNAPSHOT_CREATION_FAILED` without reporting success. All
those assertions and the separate light-corruption offsets are retained.
No production repair behavior changed.

Reproduce the focused checks with the repository's native environment helper:

```sh
python3 scripts/native-go-env.py go -- go test -count=1 ./internal/db ./internal/agile
python3 scripts/native-go-env.py go -- go test -race -count=1 ./internal/db ./internal/agile
make verify
make license-check
git diff --check
```

## Collision-safe creation (2026-11-18)

The width repair left one gap, recorded here until 2026-11-18. Ten saves
generated an ID for an empty input and then wrote with
`ON CONFLICT(id) DO UPDATE`, so a generated ID equal to a stored one would
have updated that record instead of creating a new one.

**Policy.** A create whose generated ID is already taken is refused with
`ErrGeneratedIDCollision` (`internal/db`, `internal/agile`) and writes
nothing. It is not retried: with 128 random bits a repeat points to a broken
entropy source, which a retry would hide. Explicit-ID saves keep their
upsert behaviour.

**Where.**

- `UpsertProject`, `SaveChart`, `SaveDocument` and `SaveScenario` already read
  the record inside their write transaction. A generated ID that finds one is
  refused there, before any other in-transaction check, write or audit event. The two whose
  rollback depends on `err` set it first.
- `SaveStakeholder` and `SaveResourceCalendar` check the ID inside their write
  transaction (`refuseTakenIDTx`) before writing.
- The agile `SaveColumn`, `SaveWorkItem`, `SaveSprint` and `SaveDeployment`
  add `AND ?`, bound to "the ID was not generated", to their upsert's
  `DO UPDATE ... WHERE` guard. A generated ID therefore never updates, and
  the zero-row result is reported as the collision. Each is one statement.
- Every other generator caller already fails on the primary key:
  - plain `INSERT`s for sigma projects, attachments, baselines, scenario
    charts, cost entries, cost baselines, audit events and agile boards;
  - upserts keyed on another column (`cost_types`, `cost_reserves`);
  - the catalog's insert and version-checked update, the deletion log, and
    escrow key IDs.

**Concurrency.** These are deferred transactions in WAL mode. If another
writer commits after the read, the write fails with `SQLITE_BUSY_SNAPSHOT`,
which is an error, never a silent overwrite.

**Tests.**

| Test | What it covers |
| --- | --- |
| `db.TestGeneratedIDCollisionIsRefused` | Six saves |
| `agile.TestGeneratedIDCollisionIsRefused` | Four saves |
| `agile.TestSaveDeployment_ExplicitIDUpdatesInPlace` | The one explicit-ID update no test covered |

How the collision tests work:

- **Seeding.** Prerequisites use real entropy. Record A is stored under the
  literal ID `<prefix>_30313233343536373839616263646566`.
- **The collision.** `rand.Reader` then yields `0123456789abcdef` before real
  entropy, so the next create generates A's ID.
- **What they check.**
  - The error is `ErrGeneratedIDCollision`.
  - Record A and the audit history are unchanged.
  - The table has no new row.
  - No connection is left in use.
  - A later create succeeds.
- **Fault seeding.** Removing any one guard fails its own case. So does
  returning the db error without setting `err`, which leaves the transaction
  open; inverting the deployment guard fails the explicit-update test.

Other existing tests cover the rest of the explicit-ID behaviour: updates,
including created-at preservation, and cross-project isolation.

Still open in Slice C: imported-source identity, replay, backup and restore,
and revision history. This repair does not complete Slice C.
