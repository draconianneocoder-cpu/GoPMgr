<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Identifier generation: bounded Slice C repair

**Review date:** 2026-11-14. **Baseline:** `main` at
`44d9f6de883af21c6fef6e981cef0010c33a1d1c`.
This repair addresses generation width, not the whole Slice C contract.

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

## Open Slice C work: collision-safe creation

The width repair does not establish collision-safe creation. `UpsertProject`,
`SaveChart` and `agile.Store.SaveWorkItem` generate an ID for an empty input
and then use upsert semantics. An exact generated-ID collision can therefore
be interpreted as an update rather than rejected as a conflicting creation.
Database uniqueness alone does not prevent this. Review the other generator
callers in `internal/db` and `internal/agile` for the same distinction.

The next bounded Slice C repair must distinguish generated creation from an
explicit update, enforce uniqueness within the transaction, and return a
bounded error or use an explicitly defined bounded retry policy. Deterministic
collision fixtures must prove that the existing record, its references and
audit history remain unchanged and that no partial new effect commits.
Preserve explicit-ID updates and cross-project isolation. Do not weaken the
directive's remaining imported-source identity, replay, backup/restore and
revision-history requirements or report full Slice C completion from this fix.
