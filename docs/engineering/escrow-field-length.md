<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Escrow context field-length repair

Baseline `cda84c3`, 2026-11-06. GS-06 identified an unchecked conversion of a
context field's byte length to uint32. Normal account names, generated IDs and
keys are small, but that does not enforce the invariant at every crypto entry
point. No application exploit is demonstrated by this finding.

## Design and acceptance

Enforce the existing format's inclusive uint32 maximum, rather than relying on
current callers or migrating stored data to a different prefix format. Public
signatures, purpose labels, key derivation and valid v1 bytes stay unchanged.
Only length-prefixed context fields are restricted; this is not a new limit on
plaintext/ciphertext payloads or a general memory-allocation budget.

1. Freeze old ciphertexts through the baseline's public APIs, using clearly
   synthetic test keys. Require the candidate to read those exact bytes and
   reproduce deterministic PIN/attestation vectors. Keep mismatch refusals.
2. Test zero, MaxUint32 minus one, MaxUint32, MaxUint32 plus one and MaxUint64
   numerically without allocating large buffers. Reject before narrowing.
3. Make private binding helpers return errors. String fields remain strings
   until their byte lengths are checked; binary context fields are checked
   before conversion. No partial binding or public result escapes on failure.
4. Propagate errors through every affected public operation and verifier.
   Preserve derived-key clearing, meaningful key/code checks and entropy
   failure handling. Share validated recovery info between HKDF and AEAD.
5. In a disposable source copy, lower only the limit to 64 and use one 65-byte
   context field at a time with valid keys, labels and controls. Require the
   specific length error and nil/false output. Test bypass and off-by-one
   faults; do not ship a configurable test limit or fake oversized slices.
6. Run focused compatibility/security tests, broader repository checks, local
   Ubuntu qualification, scanner and licensing checks. Independently review
   the plan, implementation and final diff; record limitations before commit.

Acceptance requires all valid baseline vectors to pass, the exact inclusive
boundary to hold, every field/error path to be exercised with bounded evidence,
no public API or stored-format migration, unchanged dependencies, and no
suppression of the finding. Scanner output supports, but does not replace,
the behavioral evidence. No push, hosted CI or release is part of this slice.

Independent plan review accepted this design after correcting the artificial
test limit: a limit of three would reject the fixed labels before the intended
field. Production boundary tests and artificial propagation tests are separate.

## Evidence limits

The frozen keys are public synthetic fixture material, never production
credentials. Randomized ciphertexts are captured once from the baseline and
must not be regenerated with the candidate. Literal binding bytes independently
check the encoding; frozen ciphertexts establish cross-version read behavior,
not independent cryptographic algorithm verification.

Numeric and disposable-threshold checks avoid multi-gigabyte allocations. They
do not claim an actual multi-gigabyte request was executed, nor establish a
resource budget for every representable context. GnuPG staging, scanner-helper
repairs and other gosec dispositions remain separate work.

## Implementation and defect sensitivity

Private binding helpers now return an error, checking string byte lengths
before appending them. Public crypto signatures and purpose labels are
unchanged. PIN and attestation binary contexts are checked before string
conversion. Recovery derives its key and authenticates its ciphertext using
the same validated binding. Existing derived-key clearing remains in place.
No migration, operational size budget, dependency change or public error
sentinel is introduced.

The unchecked conversion first failed numeric overflow assertions. The repair
passes exact boundaries and a hand-written encoding oracle covering UTF-8,
binary and empty fields. Frozen synthetic baseline DEK, grant, personal-key
and recovery ciphertexts open; PIN and attestation bytes match exactly.
Wrong owner, key ID and purpose release no plaintext. Representable contexts
beyond the UI account-name policy still work.

A disposable copy of the actual source lowers only the guard to 64 bytes.
Twenty-two individual 65-byte context fields across twelve public APIs are
rejected with the specific length error and nil/false output. Twelve normal
operation controls, three private binding refusals and three 65-byte payload
round-trips distinguish context validation from unrelated crypto rejection.
Off-by-one rejection of MaxUint32, a disabled guard and bypassing the checked
field emitter each fail their intended runtime assertion. Compilation/setup
failure is not counted as detection. Production has no configurable test limit.

Independent plan review corrected the artificial limit before implementation;
mid-work and implementation reviews found no remaining blocking issue.
The reusable lesson is to set an artificial boundary above every fixed label
and include valid controls, otherwise a test can fail before its target field.
Unchanged crypto algorithms and storage interfaces require no user migration
or API-reference revision.

## Repository validation

On macOS 27.2 with Go 1.27.3, the native Go launcher passes full crypto/users
race tests. Canonical make verify passes Go tests, 742 frontend tests,
Svelte checking with zero errors/warnings, frontend build and repository
contracts. The encrypted-database gate and REUSE 3.3 pass. Existing browser
fixture canvas diagnostics are not native UI evidence.

A fresh gosec 2.30.0 scan of the root, internal packages and update-manifest
exits 1 with twelve findings. The escrow narrowing report is absent without
suppression. The earlier restore repair removed one of the original fourteen;
this repair removes one more. The remaining reports retain their individual
[dispositions and follow-up plans](gosec-adjudication.md).

Commands used locally:

- python3 scripts/native-go-env.py go -- go test -race -count=1 ./internal/crypto ./internal/users
- make verify
- make check-encrypted-db
- make license-check
- python3 scripts/native-go-env.py go -- gosec -quiet -fmt=json -out=.tmp/escrow-field-length/gosec-after.json . ./internal/... ./tools/update-manifest

The disposable threshold and controlled-fault receipts are local qualification
artifacts, outside committed source. Numeric boundary, frozen compatibility,
encoding and representable-context tests remain in the repository. Minimum
macOS 26/Ubuntu 24.04 execution, Windows, actual multi-gigabyte inputs and full
package/release qualification are not claimed by this repair.

Native Ubuntu 26.04 with Go 1.27.3 also passes full crypto/users race tests.
All 872 transferred source inputs match SHA-256 before and after that run;
the owned temporary checkout was removed. The existing SQLCipher C
const-qualifier warning remains visible. An earlier successful run lost its
temporary checkout before post-run inspection, so only the replacement run
supports the before/after integrity claim.

Final acceptance: inclusive boundary, error propagation without partial
results, frozen v1 compatibility, no narrower identity/payload policy,
unchanged public interfaces/dependencies, canonical checks, native race tests,
licensing and independent review all pass within the stated proof limits.
No material adversarial finding remains in this repair. The actual diff was
reviewed for unintended files, credentials, generated changes and unrelated
work. Only this task's source, tests and documentation are intended for commit.
