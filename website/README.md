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

Use Python 3.11 or later, Go, and the exact Hugo executable recorded in
`quality/dependencies.json`. Set `HUGO` to that executable's absolute path. Binary provenance is recorded for macOS ARM64 and Linux AMD64. Other
platforms need reviewed archive and executable hashes before building.

The owner approved Hugo 0.170.0 for publication acceptance on 2026-11-15.
This release unifies the former standard/Extended editions and removes LibSass;
the pinned Hextra site uses CSS and needs no Sass tool. The retired
`hugoVersion.extended` setting is rejected rather than retained as an ineffective
constraint. The update fixes the `highlight` wrapper-class and vendored-module
path advisories published on 2026-11-13; archive and executable hashes remain
mandatory.

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

Visual acceptance also checks the three documentation routes at 767, 768 and
769 CSS pixels in both themes. Hextra 0.13's inclusive mobile drawer rule overlaps
its desktop utilities at exactly 48rem; the local CSS restores desktop sidebar
geometry. Verify heading and body content are unobstructed with screenshots and
point hit-testing inside `main`, plus mobile open/close, focus and scrolling.
Zero axe violations or horizontal overflow alone cannot establish this contract.

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
The separately reviewed `tools/publisher.py` implements the conditional network
boundary. It is not called by any Make build/preparation target, CI job or hook.
Its default command is read-only `preflight`; `publish` is an explicit operation
requiring a separate, current owner grant. Implementing this tool does not grant
permission to run `publish`. No live publication grant is supplied in the tree.

The private JSON grant must have exactly these fields: `schema_version` (1),
`approval_id`, `status` (`approved`, or `revoked` to deny), timezone-aware
`expires_at`, `repository`, `repository_id`, `ref`, `approved_by`, `publisher_id`,
`commit`, `tree`, `expected_tip` (null only for initial absence), `source_digest`,
`output_digest`, and `acceptance_sha256`. The maintainer creates and controls this
file only after explicit publication approval. It binds the reviewed candidate
and acceptance record; it is neither a signature nor an independent source of
owner consent. Do not generate it automatically from an all-PASS acceptance file.
Keep it and its revocation history outside the public payload.

The destination is fixed to `draconianneocoder-cpu/GoPMgr`, repository ID
`1277267642`, and `refs/heads/gh-pages`. Recreating/transferring the repository or
changing its owner requires a reviewed policy change. The tool checks the
immutable authenticated publisher ID, approving owner ID and repository push
permission with the same credential used for Git. Credentials stay in child
process environments, never arguments or receipts; Git error output is suppressed.
The object transfer uses a fresh bare repository, not candidate/global Git
configuration, then rechecks the raw tree and parents. The conditional update
names the immutable commit and exact expected remote value. It never configures
Pages or retries against a changed tip.

The grant and acceptance are re-read before transport. Revocation is a final
point-in-time check; it cannot atomically cancel a remote update already in
flight. A timeout or failed update is unconfirmed: inspect the remote read-only
before deciding any next action. Successful branch transfer still requires
separate Pages delivery and served-byte verification. Local disposable-repository
tests establish these control paths, not live GitHub publication or hosting.

## What a visitor's browser may load

Every page carries a content security policy in a `<meta>` tag, set in
`layouts/_partials/custom/head-end.html`. It allows scripts, styles, images,
fonts and data from this site only, and no inline script. The same text is the
`CSP` constant in `tools/site.py`; the build fails if any page lacks it, has a
second policy, runs inline script, sets `<base>` or names the Hugo version.
Change both places together and check search and the theme switch in a browser.

The output checker also rejects automatic meta refresh and duplicate attributes.
Refresh can navigate away despite the content policy; duplicate attributes can
make a browser and a simple attribute dictionary interpret the same tag differently.

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
