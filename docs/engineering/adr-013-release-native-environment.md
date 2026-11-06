<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# P6-E: standalone release and validator environments

Baseline `6193845`, 2026-11-05. This slice extends the existing
[native environment contract](adr-013-native-make-environment.md) to four
standalone scripts. These callers previously bypassed the Make launcher,
allowing macOS test/generator compilation under a different deployment floor
from the application's approved macOS 26 baseline.

## Plan and acceptance criteria

1. Exercise the actual scripts and launcher in disposable repositories before
   changing their Go calls. Capture child arguments, environment and failures.
2. Wrap only the callers below. Preserve arguments, licensing order, PDF
   strictness, publication ownership and downstream failure status.
3. Check PDF prerequisites before changing samples or acquiring locks. Reject
   conflicting native settings before compilation; preserve prior evidence.
4. Run focused regressions, real local validators, the canonical repository
   check and an independent adversarial review. Document excluded callers.

| Caller | Required behavior |
|---|---|
| Release race | Existing package list and `-race -p=1 -count=1`; successful REUSE precedes the new Go probe |
| Encrypted database | All three original package/filter commands receive the native environment |
| PDF/A | All four generators receive the environment; preflight follows validator identity/ICC prerequisites and precedes sample replacement |
| Local PAdES | Preflight follows scratch-path validation and precedes scratch creation, lock acquisition and cleanup ownership; generator receives the environment |

The existing launcher probes PATH Go, matching these scripts' original
executable selection. A Make `GO` override does not change standalone behavior.
Native Darwin receives the reviewed 26.0 settings; other hosts/targets retain
their environment. Each actual Go call is wrapped even after preflight, because
the preflight child cannot change its parent's environment. No installation,
dependency, compiler-selection or runtime/storage changes are introduced.

PDF/A retains its explicit non-strict SKIP contract for unmet prerequisites;
strict mode fails. A skip is not conformance evidence. REUSE has no skip mode.
Preflight protects prior PDF evidence from invalid settings, not every later
compiler or validator failure. Existing PAdES lock/publication recovery remains
responsible for failures after acquisition.

## Evidence and review

Eight behavioral tests exercise the real launcher and scripts with controlled
external commands. They cover every Go caller, original release race arguments,
failure propagation, REUSE ordering, conflicting settings, prior PDF samples,
an existing PAdES lock, explicit non-strict SKIP and unchanged Linux settings.
Controlled PDF fixtures prove caller behavior, not PDF conformance.

Initial RED tests exposed missing settings and compilation despite conflicts.
The scope regression also failed against an isolated old guard: an unscoped Go
test hidden behind the launcher escaped detection. The repaired guard retains
all 24 race-policy mutations and rejects that wrapped unscoped command.
An earlier overlapping scope-fixture attempt was invalid and is excluded from
the evidence. The cleanup fixture exposed another dependency mismatch during
canonical verification: its fake Go failed metadata discovery before reaching
the intended generator failure. Supplying valid metadata preserves the original
status-55, diagnostics and caller-owned-lock assertions.

Native macOS 27.2 runs passed the encrypted-database gate, local PAdES gate and
PDF/A gate using the official task-local veraPDF 1.30.2 CLI with all nine samples.
PDF/A library, PAdES lifecycle/lock/publication/external/trusted-source/parallel
fixtures and licensing/shell-contract regressions pass. Ubuntu 26.04 passes
the same eight boundary tests and a real installed-Go identity control that
leaves the Linux environment unchanged; transferred source hashes match.
This is Linux boundary evidence, not Linux product or PDF qualification.

The final canonical `make verify` passes, including Go tests, all 742 frontend
tests, Svelte checks and the build/contracts. ShellCheck and shfmt report zero
findings/formatting changes across 90 tracked scripts. REUSE 3.3 and the live
release-scope check pass. Existing browser-fixture canvas diagnostics remain
visible; Svelte reports zero errors and warnings. Independent plan, mid-work
and final diff reviews found no unresolved blocker after the fixture repair.
The final results addendum was self-reviewed after the independent reviewer
reached its usage limit; implementation approval had already been received.
All in-scope acceptance criteria above pass. This evidence does not include
rerunning the complete release/race pipeline.

Reproduce the focused checks with `make release-native-env-tests`,
`make shellcheck-tests license-check-tests`, `make check-encrypted-db`,
`make check-pades`, `make check-pdfa` (with verified veraPDF 1.30.2 on PATH),
`make pades-harness-tests`, `bash scripts/validate-pdfa-lib_test.sh`,
`bash scripts/release-gate-scope-check_test.sh` and `make verify license-check`.
The opt-in `make shellcheck shfmt-check` uses the provisioned pinned tools.

## Remaining work and limits

The [scanner-integrity repair](memory-scan-integrity-plan.md) now guards direct
memory-scan Go and installed analyzer execution, rejects mandatory failures and
qualifies the new explicit-root reader. Optional advisory/strict behavior is
unchanged. Remaining grep-stage operational errors are a separate follow-up.
Standalone coverage-script invocation still remains outside these slices;
canonical Make coverage is already guarded.

This does not complete all of P6-E, execute the full release pipeline or qualify
minimum-version macOS 26, Ubuntu 24.04, Windows, installer/notarization or trusted
external PAdES interoperability. No hosted checks or publication are authorized
by this slice. Local receipts are retained in ignored task state.
