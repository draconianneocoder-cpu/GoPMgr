<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-013 P3: approved shfmt contract

Status: maintainer approved on 2026-11-05. Implementation and normalization
are separate local commits; current evidence is in [the P3 record](adr-013-shfmt.md). Baseline: `50240432c78b8071574d060165275c2bcb5c4449`,
clean tracked worktree and index. P1 pins shfmt v3.14.1; P2 supplies the tracked
first-party shell inventory and dialect checks.

## Style and scope

Use tabs (`-i=0`) and otherwise shfmt printer defaults, with each declared Bash
or POSIX dialect supplied explicitly. Do not simplify or minify shell programs.
Do not inherit ambient EditorConfig settings or ignore rules. Reuse P2's tracked
scope and ownership exclusions; do not recursively discover extra sources.

A read-only comparison on the baseline found 90 eligible Bash scripts, with
3,244 tab-indented and 719 space-indented lines. These are raw line counts,
including fixture bodies, not proof of semantic indentation. Default tab style
would change 48 scripts; two-space style 86; four-space style 85. Recommend tabs
because it follows the majority and needs fewer file changes. The full private
comparison is retained in `.tmp/adr-013-p3/style-baseline.json`.

## Implementation sequence

1. Obtain maintainer approval of this style and the separate formatting pass.
2. Write behavioral tests first for the read-only check, using real pinned shfmt:
   valid input, drift, parser errors, missing or substituted tools, ambient
   settings, declared dialect, unusual paths, and unchanged source/index bytes.
3. Add opt-in `make shfmt-check` and focused tests, reusing P1 integrity checks
   and P2 inventory. Missing tools, parser failures and drift must fail visibly.
   Never install, update, rewrite, execute shell sources or access the network.
   Do not add a mandatory verification dependency, hosted job or write target.
4. Review and locally commit the check, policy and regression tests. Existing
   formatting drift remains a visible failure until the next isolated commit.
5. Normalize only eligible scripts in a separate formatting-only commit. Review
   every diff for heredocs, embedded source, literal guards and runtime output.
   The byte-exact release guard also needs a reviewed update of its expected
   indentation: formatting `check-release.sh` changes leading spaces while
   shfmt preserves the guard's heredoc bytes. Permit only these whitespace
   witness updates alongside normalization, retaining exact command arguments,
   package scope, failure behavior and all 24 race-policy mutation cases.
   These are test expectation alignment, not authorization to weaken a guard.
   Any functional repair must be separated from formatting changes.
6. Run focused tests, ShellCheck, affected shell regressions and canonical
   `make verify` on macOS. Qualify the checker on local Ubuntu when available.
   Verify formatted heredoc/script fixtures preserve exact runtime output and
   a seeded style violation fails without mutation. Independently review the
   final diff, update ADR/roadmap/coverage evidence and record proof limits.

## Acceptance and recovery

- Every eligible tracked script is checked with its declared dialect.
- Tool integrity is checked before execution; ambient configuration cannot
  suppress drift. Valid input passes; drift and operational errors fail.
- Check mode does not change source, index, config, caches or generated output.
- Formatter changes contain no unreviewed semantic alterations; existing
  literal guards and runtime regression assertions remain effective.
- Formatting-only changes and necessary reviewed whitespace-witness alignment
  are separate from implementation and functional fixes. A stale-witness
  counterexample must fail before alignment; all guard mutation cases must
  still reject their intended defects afterward.
- Final local formatting and ShellCheck scans are clean; required regressions
  pass. No hosted-CI, release, minimum-OS or Windows qualification is inferred.

No runtime, public interface, storage or dependency pin changes are planned.
Recovery is to revert the dedicated formatting commit or opt-in checker without
changing canonical Make contracts. Mandatory adoption remains a separate gate.

Reference: [pinned upstream shfmt manual](https://github.com/mvdan/sh/blob/v3.14.1/cmd/shfmt/shfmt.1.scd).
