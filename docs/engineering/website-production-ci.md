<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->
# Website production verification in CI

The owner authorized this next slice on 2026-11-12 after PR 206: extend the
existing website check to build and check the site on every pull request and
main push, keeping all existing checks. This supersedes the checker-only
restriction for that job. It authorizes no publication or repository settings.

## Contract

The existing `Website checks (source pins + output rules)` job retains its name,
runner and full-history checkout with credentials disabled. It runs checker
tests, sets up the root-pinned Go without a shared cache, then runs
`make website-ci`. No application job, required check, workflow trigger or
application build prerequisite changes. There is no deployment job or upload.

The Make target calls `website/tools/ci.sh`. It provisions Hugo Extended
0.169.0 for Linux AMD64, runs the existing production builder, parses its JSON
result, and checks the exact returned report. Errors propagate; it never finds
an artifact by directory age. The module remains isolated with `GOWORK=off`.

`website/quality/dependencies.json` records both archive and extracted executable
SHA-256 values. Provisioning checks the archive before parsing, permits only the
expected regular archive members, bounds download and expansion, and verifies
the executable before an atomic task-local installation. It never uses
`extractall` or installs a system tool. Every cached executable is rechecked;
a symlink, checksum mismatch or non-executable file fails closed.

Linux release identity was checked against the official release API digest and
`hugo_0.169.0_checksums.txt` before adding the pin. Download redirects are limited
to HTTPS GitHub release hosts. The existing macOS pin is unchanged.

## Local evidence and limits

On 2026-11-12 a disposable full-history checkout on upstairsserver, native
Ubuntu 26.04.1 x86_64, passed `make website-tests website-ci` with Python 3.14.4,
task-local Go 1.27.4 and the approved Linux Hugo. All 33 checker/provisioning tests
passed, then the actual production artifact passed its output checks. This is
local Ubuntu evidence, not a hosted Actions run or native application release.

The provisioning/orchestration tests cover verified installation, corrupted
archive and binary bytes, poisoned cache, symlinks/hardlinks, ambiguous archive
paths, expansion limits, URL and redirect policy, download bounds, exact report forwarding and failed build,
JSON or check propagation. Three disposable faults disabling archive, binary
and cache checksum guards were each detected by their intended assertion.
All non-website workflow jobs were compared byte-for-byte to the base revision.
The macOS build produced the same output digest as Ubuntu:
`8f069cca1d5f8baf8662929498a81af0b493555d9432120a3ccd1a4b8931a170`.
Local `make verify` passed without skip flags after actual Wails binding
regeneration and frontend setup. Root config tests and REUSE passed. The
repository's pinned `workflow-lint` and `workflow-security` checks passed
(three workflows, no findings), as did ShellCheck on the new wrapper.
Independent source review approved this bounded slice.

After owner-authorized push, PR 207 at `709a31d9fcdf1a7e53443409650fac068e7dfc04`
passed all 14 hosted checks on 2026-11-12. The website job passed in 22 seconds
in [run 43309793356](https://github.com/draconianneocoder-cpu/GoPMgr/actions/runs/43309793356).
This establishes the production build on that reviewed PR revision, not on a
later local revision or on main. PR 207 is not merged. The website check is
not made required by this source change.
No acceptance record is issued; generated reports still say Stage P BLOCKED.

## Execution regression guard

The local configuration checker now requires the existing four ordered website
steps: event-source checkout, checker tests, root-pinned Go setup and production
checks. It rejects missing or conditional jobs, dependencies that can propagate
skips, tolerated failures, altered commands, custom shells and redirected working
directories. An inherited `.` directory remains valid. The PR trigger retains
both main and master with no path or event-type filters.

Checkout must retain full history and disabled credentials, with no source or
path overrides. Go setup must use root `go.mod` with caching disabled. Action
references must be full commit hashes; exact input-key sets reject conflicting
case variants. Workflow, job and step environments cannot set `MAKEFLAGS`,
`GNUMAKEFLAGS` or `MAKEFILES`, which could turn checks into dry runs or load
another Makefile. Adding a pipeline step requires reviewing this contract.

Regression fixtures demonstrated that the old validator accepted 52 mutations
that bypassed this contract; the corrected validator rejects each for its named
cause. A separate controlled fault removing only the inherited-shell guard was
detected by its specific regression. Positive cases preserve inherited root
directories. Existing assurance-job tests remain intact. This is a source regression
guard, not protection against arbitrary malicious workflow or checker edits,
and it does not configure required checks or authorize deployment.

## Output navigation verification

A local follow-up audit reproduced two output-checker gaps: an automatic meta
refresh passed validation, and duplicate attributes could hide it because the
checker retained the last value while a browser retained the first. The current
generated site contains neither fault. This was a verifier defect, not evidence
of a published redirect.

The checker now rejects duplicate attribute names before constructing its
attribute dictionary, and rejects meta refresh regardless of delay or target.
Tests cover external and local destinations, case and character references,
self-closing tags, and ambiguous redirect, script and link attributes. Ordinary
metadata, the reviewed CSP and repeated attributes on separate elements remain
valid. The new negative tests failed against the old code for the intended
missing-rejection reason.

Chrome 157.0.8103.78, driven through Playwright on 2026-11-12, navigated both
redirect fixtures despite `default-src 'none'`. Source and destination responses
were intercepted fixtures, not a live external service. This corroborates the
[HTML refresh semantics](https://html.spec.whatwg.org/multipage/semantics.html#attr-meta-http-equiv-refresh).
Browser fixture evidence does not establish publication acceptance or complete
HTML validation. The public site templates, content policy and appearance are
unchanged; screen-reader and zoom acceptance remain open.

## Next publication slice

GitHub Pages remains off. Before activating artifact deployment, resolve the
manual accessibility checks, create artifact-bound acceptance evidence, and
verify the protected environment exists with main-only deployment and an
explicit reviewer/self-review policy. The future privileged deployment job
must not check out source or rebuild. Pin the artifact to its main SHA/run,
fail closed on missing or unsuccessful required checks (including any separate
CodeQL gate), reject stale candidates, and retain accepted rollback artifacts.
Update the existing `gh-pages` architecture before adopting the artifact route.
Environment approval is not a substitute for those checks.
