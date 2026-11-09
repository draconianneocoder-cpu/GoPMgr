<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# GnuPG signature staging repair plan

Status: owner approved Option 1; implemented and locally qualified. Baseline d0a1919,
2026-11-06; worktree/index initially clean. Validation below determines repair readiness.

## Confirmed problem and priority

At the baseline, ExportDocumentPDFGnuPG and ExportCombinedReportGnuPG reserved
a temporary pathname beside the destination, closed and removed the file,
then passed the absent pathname to GnuPG with --yes. A competing writer can plant a symlink
between removal and signing. The final no-overwrite publisher does not protect
this earlier write. A local native GnuPG probe using a disposable synthetic
keyring followed such a symlink, overwrote sentinel bytes and exited zero.
This demonstrates the filesystem mechanism, not an observed real-user attack
or a complete concurrent application exploit.

Prioritize this data-integrity boundary over scanner-helper reliability work.
GS-10/GS-11 previously rejected direct input pathname injection but explicitly
left this lifecycle unqualified. Those narrower dispositions are not race
clearance. Baseline signing and exportfs tests pass.

## Alternatives

1. Recommended: own a private operation directory under the OS temporary
   directory; GnuPG writes only within it. Validate/read its result, clean up,
   then use the established no-overwrite publisher for the final sidecar.
   This retains command-runner, GnuPG key selection and public API contracts.
2. Capture signature bytes on stdout. This removes temporary signature files,
   but the existing runner combines stdout/stderr. Correct separation requires
   a distinct runner contract and broader command/error tests. Defer it unless
   the private-directory approach proves inadequate.

Randomizing another absent sibling filename does not close the race. Holding
an output file open while GnuPG separately opens its pathname does not establish
file identity. Do not use either as the repair.

## Approved implementation

1. Add deterministic regression evidence before changing production. Exercise
   actual export orchestration with a controlled external command boundary;
   prove the unsafe baseline can alter an unrelated sentinel. Do not add
   production-only test hooks or rely on scheduling sleeps.
2. Consolidate signature staging for both desktop exports in one private
   helper. Create a unique OS-temporary directory using MkdirTemp, retaining
   its owner-only Unix permissions. Pass an initially absent signature.asc
   child path to the existing signing function. Do not stage in the shared
   destination directory and do not change the final artifact names.
3. After successful command completion, require a nonempty regular result;
   reject missing files, symlinks, directories and special files before
   reading. This is structural validation, not cryptographic verification.
   Preserve the existing two-minute context, key trimming and command errors.
4. Remove only the owned directory on every return. Surface cleanup errors
   and return no signature bytes on cleanup failure. Finish cleanup before
   publishing the final sidecar. Preserve existing PDF retention on signature
   failure and success/failure audit semantics; do not redesign multi-artifact
   export atomicity in this repair.
5. Test successful bytes/publication, external failure and cancellation,
   malformed/missing output, cleanup, and a competing final destination.
   Check both public export entry points and retain independent no-overwrite
   publisher tests. Use controlled faults in disposable source to verify
   meaningful assertions reject the old unsafe staging behavior.
6. Run focused tests/race, canonical make verify, REUSE and a fresh gosec scan.
   Qualify on available macOS and Ubuntu hosts; keep source hashes tied to
   native results. Use an isolated synthetic keyring for real-GnuPG evidence,
   never user signing credentials. Obtain independent plan, mid-work and final
   review; update findings, roadmap and coverage ledger before local commit.

## Acceptance criteria and boundaries

- Neither export exposes a removed sibling staging pathname for GnuPG writes.
- Private staging is owned and cleaned on success/error; failed cleanup is
  visible and prevents a success result.
- Missing/nonregular/empty results are not published or logged as signed.
- Existing destinations are preserved; valid sidecars and public return shapes,
  PDF contents, signing algorithm and key selection remain compatible.
- Critical regressions fail for the intended reason on the unsafe behavior,
  pass after repair, and have complementary native evidence.
- No dependency, public signing API, storage format, hosted-CI or release change.

Assume the OS temporary parent and selected local GnuPG executable are trusted.
Unix owner-only permissions exclude other OS identities, not a malicious
same-UID process, root or a hostile ancestor-directory owner. Do not claim
otherwise. The selected input PDF can still be changed by another writer;
input snapshot/signature verification and stronger same-user isolation are
separate decisions. Windows ACL and unavailable minimum-OS release checks are
not established by Unix permission tests. No implementation or publication
is authorized merely by this document.

Expected files: internal/desktop/app_documents.go, a narrow private staging
helper/test if needed, existing signing tests as applicable,
docs/engineering/gosec-adjudication.md, docs/ROADMAP.md,
docs/TEST_COVERAGE_LEDGER.md and this plan/evidence record.

Independent plan review: APPROVE for owner design acceptance. Lstat detects
an observed symlink, not an atomic no-follow guarantee; private-directory
ownership and the stated trust assumptions bound the read. Regression fakes
must model an attacker restricted to the export directory, not a malicious
signer with access to private staging. No blocking plan finding remains.

## Implementation sequence after owner acceptance

The owner approved Option 1 on 2026-11-06. Execute the reviewed plan inline:
first demonstrate failures in both real export handlers with a controlled
external GnuPG executable; then add one private desktop staging helper and
adapt both callers. Add failure/cancellation and cleanup qualification,
run targeted and canonical checks, update evidence, obtain final independent
review, inspect/stage only task-owned changes and commit locally. No push or
hosted CI is included.

## Implementation and evidence

Both public export handlers now obtain signature bytes from one private
helper. Its unique OS-temporary directory keeps GnuPG output outside the
export directory. The helper preserves the two-minute context and trimmed
key selection, rejects missing/empty/nonregular output and completes owned
cleanup before returning. Cleanup errors join any primary error, clear the
returned bytes and prevent sidecar publication. The existing final publisher
and public signing API are unchanged. Failed attempts retain their PDF and
do not record a signing-success event.

The test-first handler matrix failed on the baseline: the export-directory
writer's planted symlink caused sentinel replacement, and empty/symlink output
was published. It now passes for both handlers. Separate cases cover command
failure with diagnostics, missing/empty/symlink/directory output, a competing
final sidecar and denied staging cleanup. Tests inspect owner-only staging,
its removal, PDF retention, published bytes/mode and signing-success audit
counts. Permission-denial cases require an unprivileged Unix identity;
Windows executable/ACL behavior is not inferred from them. Cancellation is
tested with an otherwise discoverable executable and an already cancelled
context, without relying on a real installed signer.

Native GnuPG tests are explicitly enabled only with a marked disposable
synthetic keyring. Both public handlers produce signatures that verify against
their actual exported PDFs; modifying those PDFs makes verification fail.
Normal repository tests skip this external qualification rather than touching
a developer's keyring. A disposable actual-source copy confirms two faults
are detected by runtime assertions: bypassing structural output checks and
erasing cleanup errors. Compilation failures are not counted as detection.

Independent plan, mid-work and final implementation reviews approved the
bounded design and implementation; completion review also checks the platform
receipts, final documents and generated metadata. No dependency changes, compatibility migration, new isolation claim,
push, hosted CI or release is part of this repair.

## Recovery and remaining limits

If cleanup fails, signing returns an error and no new sidecar is published.
The error identifies the staging path through the underlying filesystem error.
It may still contain the generated signature; inspect and remove only that
operation-owned directory once the operation has ended. Do not delete whole
OS-temporary trees or another operation's staging. A retained PDF is unsigned
unless a separate successful signature exists. Multi-artifact rollback is not
introduced by this repair.

The fresh gosec scan still reports the consolidated ReadFile in the private
helper. It is a generated path within owned staging, not a user-supplied path;
the same trust and same-UID limitations above apply. Consolidating two reads
into one reduces the report count from twelve to eleven, not to zero. No
suppression or scanner-policy change is used as the repair.

Native GnuPG signing/verification passes on both available macOS and Ubuntu
hosts using separate marked synthetic keyrings. Verification covers each
handler's exported PDF and a negative changed-PDF case. It establishes those
native signing paths, not minimum-OS, Windows, production-credential or
package-release qualification. The existing Ubuntu SQLCipher C qualifier
warning remains visible and is not introduced by this repair.

## Validation commands and platform receipts

- python3 scripts/native-go-env.py go -- go test -race -count=1 ./internal/desktop ./internal/signing ./internal/exportfs: passes on macOS 27.2 with Go 1.27.3.
- go test -race -count=1 ./internal/desktop ./internal/signing ./internal/exportfs: passes on Ubuntu 26.04.1 with Go 1.27.3; all 874 transferred source inputs match SHA-256 before and after. The owned temporary checkout and synthetic keyring were removed.
- Explicit TestGnuPGStagingNativeSigner execution: passes on each host, including both public export paths and changed-PDF refusals. Normal runs skip this native test unless the marked keyring is supplied.
- Disposable output-check and cleanup-error faults: each fails the intended runtime assertions. Correct production source is unchanged by fault execution.
- Fresh gosec 2.30.0 scan of root/internal/update-manifest: exits 1 with eleven findings, including the consolidated private helper read. Other findings retain their recorded dispositions.
- make license-check: REUSE 3.3 passes without suppressions or cleanup side effects.

Minimum macOS 26/Ubuntu 24.04 execution, Windows ACLs, full packaging/release
and crash recovery remain outside this local repair's evidence. Native
GnuPG success does not add production-side cryptographic verification.

The first canonical run passed Go tests, then stopped because removing the
duplicated staging blocks shifted API-source line numbers. make code-map
regenerated only fourteen line-number entries in code-map/app-api.json;
method signatures and package-use metadata are unchanged. This generated
metadata correction is included in scope and validated by the canonical rerun.
The Ubuntu source receipt predates this line-number-only artifact refresh;
all qualified production/test source hashes still match the final source.

Canonical make verify now passes Go tests, 742 frontend tests, Svelte checking
with zero errors/warnings, frontend build, coverage-ledger and code-map checks.
Existing browser-fixture canvas diagnostics remain visible and are not native
UI evidence. All repair acceptance criteria pass within the stated trust and
platform limits. No material adversarial finding remains. The required API-map
line refresh is the only incidental generated change; scanner-helper fixes
remain separately planned and are not started in this repair.
