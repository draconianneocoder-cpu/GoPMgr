<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Gosec findings: adjudication and restore repair plan

Status: owner approved native no-replace publication; implementation and local
qualification are recorded below. Baseline `a9decb6`,
2026-11-06; checkout/index initially clean. No suppressions, exclusions or
scanner-policy changes have been made.

The local gosec 2.30.0 JSON scan exits 1 and reproduces all fourteen findings
over the root, internal packages and update-manifest tool. It runs through the
existing native Go launcher. Severity labels are scanner assessments, not
confirmed exploitability. Local raw receipts are retained outside public scope.

## Finding-specific dispositions

Locations below refer to the baseline revision. Rejection applies to the
reported mechanism, not every security property of the surrounding feature.

| ID | Rule and baseline location | Disposition and evidence |
|---|---|---|
| GS-01 | G115, money/money.go:84 | Rejected-disproven: ParseDecimal bounds negative magnitude before conversion and handles MinInt64 separately. Full-range parsing and invalid-input tests exercise the boundary. |
| GS-02 | G115, money/money.go:86 | Rejected-disproven: the multiplication/addition bound limits positive minor units to MaxInt64 before narrowing. Same parsing tests apply. |
| GS-03 | G115, analytics/rollup.go:27 | Rejected-disproven: conversion intentionally forms the low word of signed 128-bit addition, with sign extension in the high word. PortfolioRollupOverflow covers final range errors and intermediate cancellation. |
| GS-04 | G115, analytics/rollup.go:28 | Rejected-disproven: bits.Add64 carry is zero or one, representable as int64. The accumulator tests exercise carry/overflow behavior. |
| GS-05 | G115, analytics/rollup.go:32 | Rejected-disproven: narrowing supplies a candidate result; the high-word sign-extension check must pass before PortfolioRollup accepts it. Overflow/underflow tests apply. |
| GS-06 | G115, crypto/escrow.go:78 | Accepted-fixed: [escrow field-length repair](escrow-field-length.md) checks the inclusive uint32 byte-length bound before narrowing/copying and propagates errors. Frozen baseline ciphertexts preserve valid v1 compatibility. Application exploitability was not demonstrated; no suppression was added. |
| GS-07 | G204, printing/printing.go:88 | No supported command-injection defect: Unix uses separate arguments and a listed printer; Windows uses a fixed script, printer environment value and text stdin. Existing print-command/membership tests pass. Windows trusts inherited SystemRoot and has no native execution evidence here. |
| GS-08 | G202, db/stakeholders.go:161 | Rejected-disproven: legacyReadColumns selects from caller-fixed identifiers/defaults; file-derived schema names affect membership only. Project/category data are bound parameters. Existing stakeholder tests pass; no dedicated malicious-schema test is claimed. |
| GS-09 | G202, db/cost_control.go:255 | Rejected-disproven: the same fixed-column/default construction applies; project ID is parameterized. Existing cost-control/migration tests pass. |
| GS-10 | G304, desktop/app_documents.go:1310 | No direct pathname-injection path identified: ReadFile consumes the generated GnuPG temporary pathname, not a ZIP/document-provided pathname. Trust in the local GnuPG executable and writable destination parent remains a separate boundary; sibling-path race resistance is not proved by this tracing. |
| GS-11 | G304, desktop/app_documents.go:527 | Same generated-path boundary for combined reports. Final output uses the established private no-overwrite publisher. This does not establish that the earlier GnuPG temporary-path lifecycle is race-free. |
| GS-12 | G304, db/backup_restore.go:223 | The caller supplies the destination; archive entry names do not become extraction paths. However, tracing exposes the confirmed publication/scratch races below. The publication/scratch races are repaired as described below; this is not protection against a hostile writer replacing parent paths. |
| GS-13 | G110, db/backup_restore.go:232 | Reported unbounded decoded-output mechanism is constrained by entry-size checks and Go 1.27.3's ZIP checksum reader. It returns ErrFormat before forwarding a read exceeding the declared size. A small understated-size archive is rejected without creating output. No universal CPU/internal-allocation bound is claimed. |
| GS-14 | G110, db/backup_restore.go:207 | Same size enforcement applies while hashing. The malformed archive reaches the hash read, produces the wrapped ZIP format error, and never publishes. Certificate/project/manifest limits remain unchanged. |

Focused money, analytics, crypto, printing and database safeguard tests pass.
Independent tracing supports the arithmetic, SQL and printing classifications.
The restore-only scan exited 1 with thirteen findings after its direct
OpenFile sink was removed. The subsequent escrow repair scan exits 1 with
twelve findings: GS-06 is no longer reported and the other reports remain
visible.
Documentation is not a suppression or a clean security-scan result.

## Confirmed restore defects and acceptance criteria

Restore checks destination absence, extracts to the predictable
`destination.tmp.restore`, then uses a replace-capable rename. A destination
created during extraction is overwritten. Removing that predictable scratch
path can also delete a different operation's file. Both violate the documented
new-project/no-overwrite contract.

Two deterministic regressions fail against the baseline. A test-only ZIP
decompressor creates the competing destination at the extraction read, after
the hash pass and availability check. The baseline replaces its sentinel
bytes. A second case succeeds at restore but deletes a pre-existing scratch
sentinel. There are no production test hooks or timing sleeps.

Required repair behavior:

1. Use uniquely owned private staging in the destination directory; never
   remove another operation's predictable scratch path.
2. Finish extraction, ZIP integrity checks, sync and close before publication.
   Preserve an existing or concurrently created destination's bytes and mode.
3. Publish without replacement; do not add an overwrite-capable fallback.
   Preserve the v1/v2 archive format, entry allowlist and existing size limits.
4. Demonstrate both regressions GREEN, cleanup after failure, successful restore,
   encrypted-file compatibility, and existing export publication behavior.
5. Run focused tests/race checks, relevant native macOS/Ubuntu checks, canonical
   verification, licensing, and independent final review before committing.

The owner approved native no-replace publication instead of introducing the
hard-link requirement of exportfs.WriteNewPrivateStream. Restore now creates a
unique 0600 sibling staging file, copies the ZIP entry into it while checking
SHA-256 against the manifest, closes the entry reader, syncs/closes staging and
closes the source archive before publication. Cleanup reports failures and
only removes owned staging; successful publication consumes that pathname.

| Platform | Publication operation |
|---|---|
| macOS | Pinned x/sys RenamexNp with RENAME_EXCL |
| Linux | Pinned x/sys Renameat2 with RENAME_NOREPLACE |
| Windows | MoveFileEx with MOVEFILE_WRITE_THROUGH only; no replace, copy or delayed-execution flags |
| Other targets | Explicit unsupported error |

There is no overwrite-capable or hard-link fallback. These operations avoid
requiring hard-link support; actual filesystem support is still required.
The [Linux contract](https://man7.org/linux/man-pages/man2/rename.2.html) makes
that filesystem dependency explicit. The [Windows flags](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-movefileexw)
do not request replacement or copying across volumes. The macOS flag is verified
against the installed SDK and the pinned x/sys v0.49.0 implementation.

During adversarial review, extraction also proved vulnerable to archive
mutation between its first hash check and second read. A twelve-byte fixture
pair has equal CRC32 but different SHA-256; replacing the entry bytes on the
extraction read passed ZIP integrity checks and the baseline published the
replacement. Extraction now verifies the bytes successfully written against
the manifest again. This preserves the archive format and original all-entry
validation order.

## Validation and review

- The two original restore regressions and the CRC32-collision case failed
  against the baseline for the intended data-loss/integrity reasons. They now
  pass alongside legacy schema-v1 and current schema-v2 restore tests.
- Native macOS 27.2 on APFS passes the restore/publication cases with the race
  detector and two executions to check process-wide test-decoder registration.
  Direct publication cases cover absent destinations, files, directories,
  dangling symlinks and missing parents. Existing bytes, mode and identity
  remain unchanged on refusal; successful publication consumes staging.
- Native Ubuntu 26.04 on tmpfs, using Go 1.27.3, passes full database and exportfs
  race tests. All 870 transferred source inputs match before and after execution.
  The temporary checkout was removed. Existing SQLCipher discarded-qualifier
  compiler diagnostics remain visible; these are not new runtime failures.
- A disposable copy of the final restore code injects unsupported-publication
  and archive-close errors. Both errors propagate, owned staging is removed and
  no destination is created. The close-error case never calls publication.
  This is failure-injection evidence, not testing an unsupported filesystem.
- The Windows helper and its direct tests cross-compile; the unsupported-target
  stub also compiles. No native Windows execution is claimed. Review corrected
  tests to compare actual modes across publication and preserve separate Unix
  privacy assertions; Windows ACLs are not qualified by Unix mode bits.
- Final canonical make verify passes, including Go tests, 742 frontend tests,
  Svelte checks and build/contracts. Its first run exposed a stale generated
  dependency map; regeneration adds only the database's x/sys/unix edge.
  The coverage ledger includes both new test files. Existing browser-fixture
  canvas diagnostics remain visible; Svelte has no errors or warnings.
- The encrypted-database gate and REUSE 3.3 pass. Go/dependency pins, backup
  formats, size limits and scanner policy are unchanged. The existing exportfs
  publisher was not modified.

The native publication and extraction-integrity acceptance criteria pass within
the tested scope. The broader gosec investigation still has the explicitly open
questions below; remaining reports are not silently waived.
Independent plan, mid-work and final reviews approved the repair after the
Windows test-mode correction. No blocking finding remains in this repair.

## Remaining plans and proof limits

- Escrow: the [length-invariant repair](escrow-field-length.md) preserves
  public signatures and valid v1 bytes. Numeric boundary tests and bounded
  propagation/fault checks do not establish a general memory budget or actual
  multi-gigabyte allocation behavior. No format migration is required.
- GnuPG temporary paths: qualify whether a sibling writer in a shared
  destination directory can replace the removed temporary pathname before
  GnuPG creates its output. If confirmed, use private staging and validate
  output before publication; do not infer race resistance from random names.
- Scanner: native-environment propagation, erased silent helper failures and
  root-wide traversal remain separately documented work. Its advisory/strict
  policy remains unchanged.

Native Windows execution and actual exFAT qualification are unavailable; API
availability or cross-compilation does not establish those results. The tested
contract covers competing destination creation, owned staging and extraction
integrity. It does not establish resistance to a hostile writer replacing the
parent directory/staging pathname, or crash durability across power loss.
Minimum-version OS, full release and packaging qualification remain separate.
No push, hosted CI, tool installation or live user-data operation occurred.
