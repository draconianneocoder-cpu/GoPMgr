<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Decompression dependency repair

Baseline `c7ebc34`, 2026-11-06. The baseline local scanner reports called
[GO-2026-6888](https://vuln.go.dev/ID/GO-2026-6888.json) in Brotli v1.2.0 and
[GO-2026-6865](https://vuln.go.dev/ID/GO-2026-6865.json) in compress v1.18.7.
Both advisories were published on 2026-11-05. Scanner call traces establish
static reachability, not independently demonstrated application exploit paths.
This repair takes priority over the remaining native scanner-environment work.

## Decision and plan

Adopt the minimum-fixed stable tags Brotli v1.2.7 and compress v1.20.2.
The official advisories identify these boundaries; downloaded sources contain
the block-switch error propagation and FSE/Huffman validation fixes. Their
Go requirements are 1.22 and 1.25, below the application's 1.27.3 minimum.
The compress upgrade crosses intervening minor releases; it is not a
security-only source patch.

Newer stable patches v1.2.8 and v1.20.3 were also available from the official
module proxy at investigation time. They are deferred to deliberate maintenance:
the latter includes additional ARM64 decoder performance changes unrelated to
this repair. Retaining affected versions would retain the called advisories.
No replacement fork, new codec, file-format or runtime architecture is needed.

1. Capture baseline selected modules, scanner identity/database, official
   advisories and existing analytics/crypto/update regression results.
2. Exercise the upstream compress count-overflow, invalid Huffman-table and
   two-block decoder regressions against old and candidate dependencies in
   disposable modules. Include streaming as well as whole-buffer decoding.
3. Resolve only the two exact fixed tags using normal Go checksum verification.
   Compare complete selected/linked module inventories; stop for unexpected
   dependencies, toolchain changes or API repairs.
4. Regenerate module notices and check SBOM metadata against its authoritative
   inputs. Review license/native packaging differences rather than assuming
   that a version-only edit has no distribution consequences.
5. Run the independent valid Parquet codec fixtures and existing malformed-input,
   limits and cancellation tests; broaden to canonical verification, native
   Ubuntu relevant tests and actual local encrypted-database/PDF validators.
6. Require removal of the two advisories in a fresh pinned-scanner run, review
   the complete diff independently, document proof limits and commit locally.

Acceptance: exactly these two selected dependency versions change; Go, Wails,
replacements and existing application interfaces/formats remain unchanged;
checksum/notices/SBOM checks pass; the bounded upstream regressions reject the
old dependency behavior and pass with the candidate; existing application
compatibility checks pass; the two called advisories disappear from the new
scan. No hosted CI or publication is part of this repair.

## Validation results

- Normal Go module checksum verification and `go mod verify` pass. Selected
  and linked module inventories change only the two intended versions, with
  no module additions/removals. Go/Wails pins and the SQLCipher replacement
  remain unchanged. Old checksum entries do not select old versions.
- Complete archive-difference inventories and intervening compress release
  notes were reviewed. The range includes ARM64 assembly, dictionary/reset,
  streaming and validation changes, not just the advisory patch. Existing
  application imports compile unchanged on native macOS ARM64 and Linux AMD64.
- Three public-decoder regressions adapted from the
  [upstream fix](https://github.com/klauspost/compress/commit/7f5d8528720c895a6106f201719f87eb030a565d)
  fail against v1.18.7 and pass against v1.20.2: FSE count overflow, invalid
  Huffman-weight table and two-block Zstd decoding. A fourth, streaming-only
  variant independently reproduces the old decoder-goroutine panic and passes
  with the candidate. The original buffer panic prevented that old case from
  reaching streaming, so the separate variant is necessary evidence.
- Initial disposable-fixture checksum/setup and qualification-name mistakes
  were corrected before assessing runtime failures. Those setup errors are
  excluded from the RED evidence. Independent review also corrected the
  copied tests' origin labels: they originate in v1.20.2, tested against both
  versions. Reproducers are archived with local receipts, outside public scope.
- Existing analytics/crypto/update tests pass before and after the pin change.
  Analytics includes independent valid Parquet fixtures for all six codecs,
  malformed-input, cancellation, path identity and allocator-limit checks.
  Candidate analytics tests also pass with the race detector on macOS.
- Native Ubuntu tests for those three packages and module verification pass
  with effective Go 1.27.3. All 864 transferred runtime/build/test inputs match
  their macOS snapshot before and after execution. The final source differs
  from that snapshot only in the allocator explanation comment, with no
  executable change. The temporary remote checkout was removed.
- Final `make verify` passes, including Go tests, 742 frontend tests, Svelte
  checks and the build/contracts. An earlier run correctly found disposable
  test files missing from the coverage ledger; archiving and removing those
  task-owned fixtures resolved it without changing the gate. Existing browser
  fixture canvas diagnostics remain visible; Svelte has no errors or warnings.
- `make module-notices sbom-check` passes. Root license texts are unchanged;
  generated notices change only four version references. Reviewed SBOM component
  metadata needs no edit. REUSE 3.3 validation passes.
- Real local encrypted-database and PAdES gates pass. PDF/A passes all nine
  samples with the verified task-local veraPDF 1.30.2 CLI.
- The fresh macOS `govulncheck` v1.9.0 scan, built with Go 1.27.3 against
  `https://vuln.go.dev`, reports no vulnerabilities for the explicitly scoped
  root/internal/update-manifest packages. The baseline database reported an
  update time of 2026-11-05 23:49:57 UTC. This is dated platform/build evidence,
  not permanent clearance or a native Linux scanner result.

All in-scope acceptance criteria pass. No application test was added merely to
assert version strings: existing application fixtures establish compatibility,
while disposable upstream-derived regressions establish defect sensitivity.
Independent plan, mid-work and final reviews approved the repair after the
test-provenance and streaming-evidence corrections; no blocking finding remains.
Native minimum-OS, full release/packaging, signing, trusted external PAdES and
Windows execution remain separate gates; no release artifact was produced.

## Limits and deferred scanner work

Valid fixtures are independent compatibility oracles, not malicious-input
security proof. The Brotli patch adds no upstream regression in its fix commit;
without a bounded reproducer, advisory/source provenance and compatibility
checks do not establish a universal bound on decompressor allocations. Existing
Arrow allocator tests do not cover all allocations inside codecs.

The standalone memory scanner still bypasses the macOS 26 environment guard
for vet, its generated helper and installed optional analyzers. Its helper also
erases a nonzero exit with empty output and traverses beyond the source roots
claimed in the script. Follow-up: test those failures first, apply the existing
environment guard, distinguish execution failure from a clean result, and
review traversal/error handling separately without weakening checks.

The baseline also reports fourteen gosec findings, including integer conversions,
dynamic paths/commands and SQL construction. These are unadjudicated scanner
findings, not confirmed vulnerabilities or cleared results. Retain the receipts
and trace each source-to-sink path before changing code or suppressing findings.
The scanner's existing advisory/strict policy remains unchanged by this repair.
