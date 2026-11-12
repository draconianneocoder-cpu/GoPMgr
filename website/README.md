<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->
# GoPMgr companion website

This Hugo module is independent of the desktop application. It does not mount,
copy into, or embed `frontend/dist`. The application Go module and release targets
have no dependency on the website targets. Generated output stays outside the
source tree in a fresh ignored `.tmp/website-build-*` directory.

## Local development

Use Python 3.11 or later, Go, and the exact Hugo Extended executable recorded in
`quality/dependencies.json`. Set `HUGO` to that executable's absolute path. Binary provenance is recorded for macOS ARM64 and Linux AMD64. Other
platforms need reviewed archive and executable hashes before building.

```
make website-build
make website-check WEBSITE_REPORT=/absolute/path/to/build-report.json
make website-prepare-publication WEBSITE_REPORT=/absolute/path/to/build-report.json
```

The final command is local only and fails closed while publication acceptance is
blocked. No website command activates Actions, pushes, changes Pages settings or
modifies the source checkout's Git index or refs. The owner-approved website CI job runs the checker tests and `make website-ci`
on every PR and main push. It provisions the pinned Linux Hugo, builds production
output, and checks the exact returned artifact. It has no deployment permissions
or publication step. Existing application assurance jobs remain unconditional.

These commands need the full Git history: each page is checked against the
commit its sources were reviewed at, which a shallow clone does not have.

Each build verifies canonical-source hashes, complete reviewed website inputs,
the exact Hugo executable, isolated module graph and module integrity. It records
tracked repository bytes plus website inputs, dirty status, output inventory and
aggregate digests. HEAD alone is not the source identity. Reports live beside the
artifact, outside the public output. Changing an implementation input invalidates
the reviewed-input register; changing a canonical dependency requires reviewing
its affected pages and updating their source commit and hash deliberately.

`quality/reviewed-inputs.json` excludes itself to avoid a circular hash; the full
source snapshot includes it. Reviewers must inspect changes before updating the
register. Tests use disposable directories and never authorize publication.

## Acceptance stages

Development checks cover each changed slice. Publication acceptance additionally
requires artifact-bound browser evidence, accessibility and privacy review,
release evidence refresh, complete license distribution, independent final
review, and an acceptance record that names this repository's GitHub
private-reporting channel. Automated HTML checks are not a screen-reader test
or a native application qualification.

GitHub private vulnerability reporting is enabled. The public security page
links <https://github.com/draconianneocoder-cpu/GoPMgr/security/advisories/new>.
No artifact is publication-accepted until the outstanding acceptance checks are
completed and the maintainer separately authorizes publication. Never change a
failed gate to PASS merely to generate a candidate.

The intended publication branch is output-only `gh-pages`. A local staging
repository must commit the exact verified bytes against the previously observed
remote tip; an authorized later push must name `refs/heads/gh-pages` explicitly
and use an exact force-with-lease after checking that the observed tip is its
single direct parent. A matching lease never permits a non-forward rewrite.
A stale tip aborts. Rollback is
a new commit with previously accepted bytes. Local bare-repository transaction
tests are separate from live GitHub Pages delivery evidence. No hosting claim
is established by these local tests.

The maintainer is the publication and maintenance owner. Before any separately
authorized transaction, archive the accepted report, public inventory, required
gate evidence and prior accepted artifact outside the public tree. Verify the
served bytes and GitHub's built-in Pages delivery after an authorized push;
local staging does not establish delivery identity. Keep failed delivery records
and restore by preparing a new forward commit from the previous accepted bytes.

For future accepted local preparation, pass `WEBSITE_ACCEPTANCE` as an absolute
path to a reviewed JSON record (schema_version 1, reviewer, reviewed_at,
source_digest, output_digest, private_channel, owner_channel_approval and gates).
`gates` must contain exactly A01 through A15 and R1 through R11, each `PASS`.
The record is a review attestation, not a substitute for running the checks, a
cryptographic signature, or permission to push. Do not put it in the public tree.
The shell helper also accepts `--parent-repository /absolute/local/mirror`
and `--expected-tip <observed SHA>`; the candidate is an isolated local Git tree.
No production network transport is implemented or activated.

## What a visitor's browser may load

Every page carries a content security policy in a `<meta>` tag, set in
`layouts/_partials/custom/head-end.html`. It allows scripts, styles, images,
fonts and data from this site only, and no inline script. The same text is the
`CSP` constant in `tools/site.py`; the build fails if any page lacks it, has a
second policy, runs inline script, sets `<base>` or names the Hugo version.
Change both places together and check search and the theme switch in a browser.

GitHub Pages cannot send response headers, so two protections are missing: the
site can be shown inside a frame on another site, and there is no HSTS header
of its own (GitHub serves `github.io` over HTTPS only). The site has no forms,
accounts or actions, so framing has nothing to hijack.

The theme import explicitly mounts assets, layouts, translations and icon data.
Its unused stock static branding and web manifest are excluded; the public
site uses the repository's Bobby assets and a local PNG favicon.

## Linux production verification

`make website-ci` downloads the official Hugo Extended Linux AMD64 archive into
ignored task-local storage, verifies its pinned SHA-256 before parsing it, then
verifies the executable before installing or running it. The cache is checked
on every reuse; corruption and symlinks fail rather than trigger a silent repair.
Downloads and archive expansion are bounded. It does not install system tools.
For an already downloaded archive, use
`python3 website/tools/provision_hugo.py --archive /absolute/archive.tar.gz`;
the same checks apply. The source pin is in `quality/dependencies.json`.

CI uses full Git history and the repository's pinned Go with `GOWORK=off`,
without a shared writable cache. The JSON build result supplies the exact report
path to the checker; no newest-directory lookup is used. Only the existing
website job is extended. No Pages artifact, protected environment, deployment
or required-check setting is created by this change. Linux build evidence does
not close the manual accessibility or publication-acceptance gates.
