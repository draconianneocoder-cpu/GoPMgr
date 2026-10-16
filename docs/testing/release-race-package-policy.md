<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Explicit race-test package budget

## Policy and scope

`make race` and the race step of `make check-release` run with `-p=1`
and `-count=1`. The outer Go command schedules one package test binary at
a time and executes tests rather than reusing cached results. This bounds
package overlap, not total process count, CPU use or memory consumption:
SQLCipher compatibility tests still start child Go/C helpers.

All package scopes, build tags, assertions, production cryptography and the
default ten-minute **per-package** deadline remain unchanged. `t.Parallel`,
application goroutines, concurrent database connections and explicit
test barriers remain enabled. Make includes `. ./internal/... ./tools/...`;
the release race step includes `. ./internal/... ./tools/update-manifest`.
The complete command can take longer than ten minutes.
No shipped application API, schema, cryptographic parameter or UI changes.
Reverting this verification policy requires no data migration.

The cost motivating the policy is real fixture work, not a demonstrated
deadlock. Password hashing uses 64 MiB, three passes and four lanes. Key
wrapping uses 64 MiB, one pass and four lanes. A keyed recovery-code set
performs eight hashes and eight wraps. CPU-count-based package fanout
allows these operations in the root and account binaries to overlap.
The explicit one-package budget avoids that overlap without replacing the
cryptography or disabling in-package concurrency mechanisms.

This removes incidental cross-package scheduling stress. A bounded source
review found no test assertion requiring independently scheduled package
binaries to run together: root/accounts use temporary databases; concurrency
tests coordinate their own callers, connections and local test servers.
That review is not proof of identical schedule coverage. Dedicated
cross-process and native-C behavior needs its own explicit tests, not
accidental overlap between unrelated package suites.

## Observations, 2026-10-16

Source baseline: `8604ea4620e6b734daa4799976652a9475eeb60f`, macOS arm64,
Go 1.27.2, CGO enabled, ten logical CPUs and 32 GiB RAM. No other heavy
verification was deliberately started by this task during the diagnostic
race runs. The machine was **not isolated**: unrelated Rust work was observed,
load varied, and permissions differed between runs. Allocated swap was not
treated as proof of active paging. These are observations, not a controlled
benchmark or proof of the earlier timeout's cause.

```sh
# Baseline: default package scheduling, sandboxed.
go test -race -count=1 -json -tags webkit2_41 . ./internal/... ./tools/update-manifest
# Diagnostic: one package at a time, with local test-server/filesystem permissions.
go test -race -p=1 -count=1 -json -tags webkit2_41 . ./internal/... ./tools/update-manifest
# Actual complete release gate, with strict native PDF/A validation.
GOPMGR_VERAPDF_FORCE_CLI=1 make check-release
```

| Run | Observed result |
| --- | --- |
| Baseline | Exit 1. Accounts timed out at 600.201s after 209 of 287 top-level tests passed; the active test was performing key-wrap Argon2 work. Root finished at 591.674s: 320 passed and its invalid-filename fixture failed with sandbox `EPERM`. Update's local TLS listener was also denied. These permission failures are not product regressions. |
| Diagnostic | Exit 0. All 44 expected packages passed; root 304.203s and accounts 418.174s. Exact inventories matched all 321 root and 287 account top-level names, with no duplicates or unfinished observed test lifecycles. Root's existing APFS invalid-UTF-8 exclusion was the only skip; there were no account skips, failures, cache markers or race warnings. |
| First updated full release gate | Exit 2 after 898 seconds. All race packages passed (root 310.787s, accounts 395.332s), followed by the build and encrypted-storage checks. The PDF/A harness then failed from an inherited backend setting, diagnosed below. This is not a full-gate pass. |
| Full release gate after the harness correction | Exit 0 after 1,257 seconds. All 44 expected race packages passed uncached; root 396.096s and accounts 582.035s. Production build, encrypted storage, PDF/A harness and strict validation of all four samples, then PAdES harnesses passed. |

The previous review also observed root and accounts timeouts; older
[account verification](account-race-assurance.md) recorded complete passes
at its stated source revision. Preserve those historical results. Neither a
passing comparison nor this policy establishes why every previous timeout
occurred or guarantees completion on an arbitrarily loaded host.
The last account run had only about 18 seconds left before its deadline;
external contention and future workload growth remain material runtime risks.

Optional sample-output variables (`GOPMGR_REPAIR_SAMPLE`,
`GOPMGR_RECOVERY_SHEET_SAMPLE`, `GOPMGR_MONTECARLO_REPORT_SAMPLE`) and the
optional SQLCipher sanitizer setting were unset in the diagnostic runs.
No real account profile or project was used. The Go race detector does not
instrument native C, and no Windows/Linux GUI or package lifecycle claim
follows from these checks.

## Regression guard and acceptance

`scripts/release-gate-scope-check.sh` checks the active Make race recipe,
the release race section, their complete package definitions and failure
propagation. It deliberately accepts a narrow reviewed command shape, not
arbitrary equivalent shell programs. Its existing fixture suite copies the
working entrypoints before testing them, rather than silently testing only
the last commit. Negative cases remove the package bound, race detector or
cache-disabling flag; filter or shorten tests; narrow scope; add a conflicting
bound; ignore failure; or duplicate an entrypoint/section.

Source checks cannot prove that tests completed or never conditionally skip.
Acceptance also requires a complete uncached race run with exact package and
root/account test-name reconciliation, explicit exclusions, process exit
zero, and the actual updated full release gate. Strict native PDF/A
validation uses the documented `GOPMGR_VERAPDF_FORCE_CLI=1` override when
Docker is unavailable. This run used veraPDF 1.30.0; the pinned Docker image
is 1.30.2 and no real Docker validation was performed. Native installation,
migration/recovery, editor data-loss, trusted-signature and publication
evidence remain separate gates.

## Related blocker: PDF/A fixture backend isolation

Running the full gate with the documented native-CLI override exposed a
confirmed harness defect. Its Docker fixture inherited
`GOPMGR_VERAPDF_FORCE_CLI=1`, selected the fake CLI instead of fake Docker,
and never wrote the Docker argument log its assertions require.

The Docker fixture now explicitly sets `GOPMGR_VERAPDF_FORCE_CLI=0` for
its child invocation. A subshell first models a caller forcing CLI mode,
so the test exposes a missing override even under the default environment.
It does not change the caller's backend or production backend selection.
Existing image/path substring and mutable-tag assertions remain intact;
this is not real Docker execution or exact argument-vector validation.

The original harness failed with the inherited override. With the correction,
the harness passed with caller values zero and one. Removing only the
fixture's correction also failed under caller value zero at the missing
Docker log; the correction was restored byte-for-byte, then both values
passed again. An earlier asynchronous mutation attempt restored the source
before the shell finished reading it; that attempt is invalid evidence and
is not counted as a successful sensitivity check.

```sh
GOPMGR_VERAPDF_FORCE_CLI=0 bash scripts/validate-pdfa-lib_test.sh
GOPMGR_VERAPDF_FORCE_CLI=1 bash scripts/validate-pdfa-lib_test.sh
```

## Verification status

The diagnostic run and the entrypoint fixture suite passed. All 24 negative
fixtures rejected the intended policy violation, including indented scope
redefinitions, exported scope and missing entrypoints. `make verify`,
`make release-scope lint-all license-check` and the additional
`go test -race -p=1 -count=1 ./tools/code-map` passed. REUSE covered all
962 files. Shell syntax and whitespace checks passed. The full release gate
after the PDF harness correction passed. `make coverage-ledger-drift` also
passed: all 46 package headings matched, with all 49 ledger headings
accounted for. Independent read-only adversarial review found no actionable
introduced defect and approved the six-file change. It reconciled the
diagnostic inventories and final log's uncached package passes; it did not
rerun tests or independently observe the runner's process exit status.
This is local verification, not release approval or trusted-certificate evidence.

ShellCheck reports pre-existing informational diagnostics, reproduced
against the original files: SC2086 on intentionally splitting the fixed
package-list string into arguments, and SC2016 on matching literal
workflow-variable and image-version expressions in source. No new diagnostic
was found; the complete ShellCheck invocation is not claimed as a clean pass.
Neither diagnostic was suppressed or used to justify unrelated rewriting.

The build retained its macOS 13 object / 11 link-target warning and Wails
`time.Time` binding diagnostics. Passing frontend tests emitted jsdom canvas
not-implemented notices. These checks establish neither older-macOS runtime
compatibility nor visual chart rendering or native accessibility.
