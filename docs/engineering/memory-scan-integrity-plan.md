<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Memory-safety scanner integrity repair plan

Status: Option 1 accepted by owner; implemented and locally qualified.
Baseline a1a1301, 2026-11-06; working tree/index initially clean. The accepted implementation preserves
the optional scanner policy. Results below determine qualification.

## Findings and priority

The generated os.Open helper's status is discarded with `|| true`. A disposable
execution fixture makes its Go command exit 73 with no output; the actual shell
script exits zero and prints Memory-safety gate PASSED. Output alone is not a
reliable substitute for execution status. Missing Go also skips required vet
and helper execution instead of preventing a success claim.

The helper walks the entire working directory, although the script describes
its own-source scope as main.go, internal and tools. A second disposable fixture
runs the real embedded helper: a leak-like source file in unrelated/leak.go is
reported and fails the scan. The script also invokes Go and installed optional
analyzers without the existing native-go-env guard. This is source-inspection
evidence of the environment gap, distinct from the executed status/scope probes.

Repair these integrity failures before expanding security-scan coverage or
changing severity policy: they can make scan completion claims unreliable.
The preceding GnuPG/escrow repairs remain intact. Product behavior, interfaces,
dependencies and toolchain pins are outside this repair.

## Options

1. Recommended: replace generated helper source with a small checked-in,
   standard-library-only Go tool under tools/os-open-scan. It takes explicit
   source roots, reports read/traversal/scanner errors, and has direct behavior
   tests. The shell remains the orchestrator and Make remains canonical.
   This removes temporary source generation and its cleanup obligation.
2. Retain the embedded Go helper and fix its status, roots, I/O handling and
   cleanup. Fewer files change, but the helper remains harder to test directly
   and shell heredoc/source-lifecycle handling remains necessary.

Both approaches must meet the same acceptance criteria. Neither substitutes
an AST/type-based leak analysis for the existing ten-line heuristic.

## Approved implementation plan

1. Preserve the baseline false-PASSED and off-scope regressions. Add subprocess
   tests against the real shell entry point, controlling only external command
   boundaries. Verify clean completion, findings, silent/non-silent failure,
   missing Go and success text. The false-success case must fail before repair.
2. For Option 1, add the checked-in helper with explicit paths and no implicit
   working-directory traversal. Preserve the existing os.Open/comment/ten-line
   matching behavior. Cover multiple roots, empty/clean trees, real findings,
   missing/unreadable inputs, traversal and scanner errors, and vendor exclusion.
   Reject invocation without roots; explicitly supplied empty directories may
   complete cleanly. Reject symlink roots and symlink entries encountered
   within scanned roots; do not follow them. Prune vendor subtrees by path
   component before inspecting their contents. This is not an atomic defense
   against hostile concurrent pathname replacement. Do not label incomplete
   reads or traversal clean.
3. Define its internal result protocol: zero for completed/no findings, one
   for findings and a distinct nonzero execution-error status. The shell maps
   all required failures to its existing nonzero gate failure; preserve useful
   diagnostics and never interpret successful diagnostic output as findings
   merely because it is nonempty. This is an internal helper protocol, not a
   new product API or outer CLI exit-code contract.
4. Run mandatory Go checks and installed optional analyzers through the existing
   native-go-env launcher. Reject conflicting macOS targets before execution;
   preserve Linux and other-target behavior. Keep current production-tag usage,
   existing package roots, and optional missing-tool/advisory/strict semantics.
   Add only the new helper to memory-scan's explicit analyzed package list.
   Do not expand or rewrite the separate release script's package contract.
5. Add a focused memory-scan-tests Make target and include its behavioral
   regressions in make verify. Existing Make Go tests already include tools/...
   and therefore the helper's Go tests. Do not add full advisory scans to the
   fast verification gate or change hosted workflows.
6. Deliberately erase a helper failure or restore implicit root traversal in
   disposable source and confirm intended runtime assertions detect each fault.
   Verify cleanup/unchanged fixture inputs; never inject faults into live code.
7. Run targeted tests, relevant race checks, ShellCheck/shfmt, canonical
   verification and REUSE. Exercise the actual helper and scanner on available
   macOS/Ubuntu hosts; distinguish controlled optional tools from real installed
   scanner results, and record existing findings without suppressing them.
8. Update the roadmap, findings dispositions, native-environment follow-up and
   coverage ledger. Obtain independent mid-work/final reviews, inspect the
   actual/staged diff and commit only task-owned files locally after all gates.

## Acceptance criteria

- Missing Go, native-environment preflight failure, vet failure and helper
  execution failure prevent a successful gate summary, including silent
  failure. Other grep-based heuristics retain their current behavior; this
  repair does not claim comprehensive operational-error handling for them.
- The helper examines only supplied first-party roots with the stated vendor
  exclusion, and reports failures to examine required inputs.
- Existing clean/finding behavior and the heuristic's limitations are explicit;
  no claim of comprehensive leak or memory-safety verification is added.
- Native environment conflicts prevent affected commands from running;
  Linux pass-through and optional advisory/strict behavior remain tested.
- No dependency/toolchain/product-API changes, hosted CI, publication or new
  scanner-policy suppressions. New helper/tests/docs meet REUSE requirements.

## Limits and decisions

Owner accepted Option 1 before implementation, including aligning the
helper with the declared first-party filesystem roots and treating missing Go
as failure of mandatory checks, and rejecting symlink entries within the
scanned roots. Git inventory currently shows no tracked symlinks in these roots.
Preserve the optional-tool policy: this slice
does not make optional findings fatal by default or require absent analyzers.
Do not invent new file-size budgets or claim the heuristic proves resource
ownership. Operational-error hardening of the remaining grep-based stages was
a separate follow-up, completed on 2026-11-09 (see
[the follow-up](#follow-up-grep-stage-operational-errors-2026-11-09)). Minimum-OS and full release
qualification remain separate; local macOS/local Ubuntu are the test hosts.

Independent plan review requested two corrections, both accepted: define the
symlink/no-roots contract before implementation and restrict the failure
promise to the Go/native-environment/vet/helper boundaries actually repaired.
The revised plan incorporates both. Final independent verdict: APPROVE for
owner acceptance of Option 1, with no blocking finding. The owner subsequently
approved this design before implementation.

## Implementation and review results

The shell now uses command status rather than captured output to assess the
helper. Required Go/vet/helper/preflight failures prevent success. The checked-in
helper scans only explicit roots, refuses symlink inputs, prunes vendor
directories and reports incomplete reads/scans. It preserves the previous
1 MiB scanner-token limit and ten-line/comment heuristic. Its binary returns
0/1/2 for clean/findings/execution failure; go run itself collapses failing
child statuses to nonzero and prints the child status. The outer gate remains
zero/nonzero and does not infer completion from that diagnostic text.

The helper streams each open file and closes it before moving to the next.
Mid-work review caught an initial whole-file ReadFile allocation and stale
shell-contract fixtures. Both were corrected. A disposable 64 MiB sparse-line
probe failed with about 69.2 MB allocated before correction and passed with
about 2.1 MB after streaming was restored. Its allocation threshold is a local
regression probe, not a production file-size or total-memory budget. Retained
whole-file line collection still scales with source size.

Baseline subprocess tests reproduced false PASSED for silent helper failure.
The repaired nine-test shell suite covers status/diagnostic handling, missing
Go, vet/metadata/preflight refusal, all-command macOS settings, Linux pass-through
and each optional analyzer's advisory/strict policy. Existing package-argument
and terminal-color tests retain their original assertions with valid launcher
fixtures. Go tests cover actual files, errors, roots, vendor boundaries,
symlinks, token overflow and actual CLI exit statuses in child processes.

Disposable-source faults erasing helper failure, expanding roots to their
parents and removing the CLI main invocation each fail the intended runtime
assertions. Build failures are not counted as detection. Independent mid-work
and source reviews approved the corrections; the two newly visible gosec
reports were independently traced as authorized reader behavior under the
reviewed caller/trust model, not universally cleared path operations.

## Validation and proof limits

- make verify passes Go tests, 742 frontend tests, Svelte checks with zero errors/warnings, frontend build and repository contracts. The new memory-scan-tests target runs behavioral fixtures only, not installed optional analyzers.
- Native macOS 27.2 / Go 1.27.3 passes helper race tests, the nine shell tests, all five existing shell-contract tests and the actual memory scanner. Required vet/helper checks complete. Staticcheck is absent; gosec 2.30.0 reports thirteen findings as an advisory warning; govulncheck reports no vulnerabilities.
- Native Ubuntu 26.04 / Go 1.27.3 passes helper race tests, nine shell fixtures, both affected existing scanner shell-contract tests and the actual scanner. Optional staticcheck/gosec/govulncheck are absent there and explicitly skipped under the unchanged policy. All 980 transferred source/build-asset inputs matched SHA-256 before and after execution; owned remote scratch was removed. Subsequently updated AGENTS.md differs from that snapshot. The seven frozen source/build files still match locally.
- ShellCheck passes. The already provisioned pinned shfmt 3.14.1 reports no shell formatting drift. No tools were installed. Existing browser canvas diagnostics and Linux Wails/SQLCipher compiler warnings remain visible; these are not new source changes or native UI qualification.
- The generated dependency map adds only the new standard-library helper package with no dependency edges. No release-script package list, dependency, toolchain, product API, scanner suppression or hosted workflow is changed.

Useful commands: make memory-scan-tests; python3 -B scripts/engineering/shell_contract_test.py;
python3 scripts/native-go-env.py go -- go test -race -count=1 ./tools/os-open-scan;
bash scripts/memory-safety-scan.sh; make verify; make license-check.

The helper is a local filesystem reader: CLI users intentionally select roots;
the repository caller supplies fixed first-party roots. Reassess its trust
boundary if it becomes elevated, service-exposed or consumes less-trusted
roots. It does not defend against hostile concurrent pathname replacement,
prove ownership/lifetime correctness, qualify minimum macOS 26/Ubuntu 24.04 or
Windows, or complete release readiness. Grep-stage operational-error handling
was completed later (below); standalone coverage environment alignment remains
separate work.
Pinned upstream Linux compiler deprecations require separate dependency
maintenance; this repair does not modify vendored/platform code to silence them.
No push, hosted CI, release or live user-data operation occurred.

Final review corrected the source-receipt wording after the agent-guide update:
Ubuntu verified its complete transferred snapshot, while the later guide change
is documentation-only and the qualified source/build hashes remain unchanged.
No additional code finding was identified.

## Follow-up: grep-stage operational errors (2026-11-09)

Stages 2 (`unsafe.Pointer`), 4 (`sql.Open`), 5 (mutex copies) and 6
(goroutine inventory) ran `grep … 2>/dev/null | … || true`. An unreadable
source file, a missing root or any other grep error left an empty result, so
the stage reported OK and the gate could pass without having read the code.

Each search now writes to a private temporary file through `run_grep`: exit
status 0 or 1 is a result, a higher status fails the gate with a "could not
complete" message, and grep's own diagnostic stays visible. The secondary
filters in stages 5 and 6 are checked the same way. Stage 4 reads its match
list in the parent shell, takes each 30-line window with `sed`, treats a
`sed` failure or an unparseable match location as an operational error, and
no longer reports a read failure as a missing-Close finding. Stage 6 still
never fails on findings, but an incomplete inventory now fails the gate.

Two corrections came with the repair:

- `grep_sources` checks that every root exists. macOS BSD grep applies
  `--include='*.go'` to command-line operands too, so it skipped a missing
  root with exit status 0; GNU grep reports it.
- Vendored code is excluded by directory name (`--exclude-dir=vendor`, the
  same rule as `tools/os-open-scan`). The old `grep -v '/vendor/'` filtered on
  the whole output line, so a first-party line that merely contained
  `/vendor/` was hidden. The dead `.md`/`.sh` filters, unreachable under
  `--include='*.go'`, were removed.

The heuristics are otherwise unchanged; on the repository tree the output of
stages 2–6 is byte-identical to the previous script. One blind spot is now
pinned by a test rather than fixed: stage 5 still drops any line that also
contains a `.mu.` selector.

`scripts/memory-safety-scan_test.py` adds a characterization table for every
grep stage, logged fault injection that makes one grep call (or one filter)
exit 2 per stage, an unreadable file beside a readable match, a missing
root, a `sed` failure and a colon in a source path. Seeded faults (ignoring
grep status, dropping the root check, the vendor exclusion, the `.mu.`
filter, the location guard or the `sed` status, or widening the window) each
fail the intended test. The 15 tests and `make memory-scan` pass on macOS
27.2 (BSD grep) and on Ubuntu 26.04.1 (GNU grep 3.12, GNU sed 4.9, uutils
mktemp 0.10.0, non-root); ShellCheck and shfmt report no findings.
