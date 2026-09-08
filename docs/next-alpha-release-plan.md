<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Next alpha release plan

This is the version-neutral execution plan for GoPMgr's next alpha. It does not
authorize a tag, push, hosted workflow, or publication, and it does not turn an
unverified package into a supported-platform claim.

The completed designation, risk dispositions, recovery action, and evidence
for the published `v1.1.0-alpha.2` prerelease are recorded in
[releases/v1.1.0-alpha.2.md](releases/v1.1.0-alpha.2.md).

## Release intent

The next alpha is a limited engineering-validation release. Its purpose is to
collect install, launch, update, persistence, export, and accessibility
evidence from explicitly named target environments. It is not the wider beta:
the P0 exit criterion in [beta-release-backlog.md](beta-release-backlog.md)
still applies to wider distribution.

The candidate must contain no known unfixed critical data-loss, project-data
isolation, update-integrity, or export-integrity defect in its advertised
scope. Package formats without current lifecycle evidence may be attached for
engineering evaluation only if the release notes label them as unverified;
they must not be described as supported.

## Decisions required before candidate preparation

| Decision | Required record |
| --- | --- |
| Version and tag | Confirm the version of record in `internal/cli/parser.go` and `wails.json`, then choose the symbolic form `v<version-of-record>-alpha.<sequence>`. |
| Target matrix | Name each OS, architecture, package format, and test host that the alpha will advertise. |
| Trust posture | Decide whether this alpha remains explicitly unsigned or waits for platform signing, macOS notarization, the signed-update keypair, and trusted-certificate PAdES evidence. |
| Residual-risk acceptance | Review every open P0/P1 item and state whether it blocks this limited alpha, remains a disclosed alpha limitation, or is outside the advertised scope. |
| Release operator | Name the person responsible for the tag, hosted run, artifact/digest review, publication, and rollback decision. |

No default owner, date, candidate tag, or platform claim is inferred by this
document.

## Candidate entry gate

Run from the exact clean commit proposed for the tag:

```sh
git status --short
make verify
make lint-all
make license-check
make coverage-ledger-drift
make release-scope
make check-release
GOPMGR_RELEASE_TAG=v<version-of-record>-alpha.<sequence> make tag-preflight
```

Before the tag, also require:

- an adversarial review of the complete candidate diff and release claims;
- current dependency/advisory results from the full release gate;
- a release note draft that distinguishes verified facts, blocked checks,
  unrun checks, and known limitations;
- no tracked private handoff material, credentials, user databases, exports,
  certificates, or local-only branding archives;
- a recorded recovery action for any schema or encryption migration included
  since the prior alpha.

## Platform evidence matrix

| Surface | Current evidence boundary | Alpha requirement |
| --- | --- | --- |
| macOS `.dmg` / `.pkg` | Unsigned local arm64 packaging and selected lifecycle flows have prior evidence. | Rebuild from the candidate; inspect package contents; install, launch, preserve user data across relaunch/upgrade, and remove the app without removing user data. Run Gatekeeper/notarization checks only if credentials are provisioned. |
| Linux `.deb` / `.rpm` | Package structure and version ordering have evidence; a real root-level install/launch/removal cycle is still unavailable on the SSH-only host. | Obtain an interactive display and installation authority, or label the packages unverified and do not advertise Linux support. |
| Windows `.exe` | No Windows host evidence is recorded. | Obtain a Windows host and run install, launch, update/upgrade, uninstall, data-preservation, and Narrator/keyboard smoke checks, or label the artifact unverified and do not advertise Windows support. |
| Signed update channel | Code and manifest verification are tested, but the release keypair is not provisioned. | Provision and protect the Ed25519 keypair, verify the embedded public key and manifest URL, test a real alpha-to-newer-alpha update, or state that updates are disabled. |
| PDF/A and PAdES | Deterministic and external-validator harnesses exist; trusted-certificate evidence is blocked. | Run the local/external gates. Do not claim trusted-chain validity without a separately supplied trusted sample and `make check-pades-trusted` evidence. |
| Keyboard and assistive technology | Focused macOS keyboard work exists; broad VoiceOver and all Windows AT coverage remain open. | Run the flows advertised in the alpha notes and record the exact screen reader, OS, route, and outcome. |

## Publication sequence

1. Freeze the candidate commit and complete the entry gate.
2. Resolve the decisions above and write release notes from the evidence matrix.
3. Push the reviewed branch/commit only after explicit approval.
4. Create and push the approved prerelease tag. The tag-triggered workflow must
   pass preflight before any package job can build artifacts; its final job
   creates a draft GitHub prerelease rather than publishing immediately.
5. Review every hosted job and every draft-release package name, platform
   metadata, signature/trust result, SHA-256 digest, generated note, and
   required limitation before publishing the draft. Review the signed-update
   manifest when the channel is configured; otherwise record that the channel
   is disabled and no manifest was produced.
6. Publish the reviewed draft as a GitHub prerelease, not a general release.
   Add the tag to
   `published-release-tags.txt` only after the public release exists and has
   been independently checked.
7. Repeat the advertised install/launch/persistence/removal checks against the
   downloaded artifacts, not the local pre-publication copies.

## Stop and rollback conditions

Stop publication for a failed required gate, mismatched version/channel,
missing or unexpected artifact, invalid digest/signature, project-data loss or
cross-project disclosure, migration failure, or a release note that overstates
evidence. If a defect is found after publication, mark the release affected,
remove it from recommendation, preserve diagnostics without user data, fix on
a new commit, and issue a new alpha sequence. Never move or reuse the original
tag.

## Evidence record

For each candidate, preserve the commit, tag, commands, tool versions, host OS
and architecture, package digest, outcome, and reviewer. Use only these states:
`Verified`, `Failed`, `Blocked`, `Not run`, and `Decision needed`. A structural
source check is not native lifecycle evidence, and a local pass is not hosted
workflow evidence.
