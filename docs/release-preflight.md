<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Release pre-flight checklist

Use this checklist from the exact commit that will be tagged. Do not convert a
missing validator, target machine, certificate, or signing credential into a
passing claim.

## Before tagging

1. Confirm the version of record in `internal/cli/parser.go` and `wails.json`.
2. Run the full local gate:

   ```sh
   make check-release
   ```

3. Confirm the package and release documentation describe only verified
   platforms and known limitations.
4. Confirm the tracked license and release-reference records are current:

   ```sh
   make license-check
   make release-scope
   ```

5. Review [docs/beta-release-backlog.md](beta-release-backlog.md). Do not
   publish a wider beta while a P0 item is open.

## Signing and validation

- Confirm each installer has the expected platform signing or clearly retains
  its unsigned warning.
- Run `make check-pdfa`, `make check-pades`, and
  `make check-pades-external` when PDF or signing behavior changed.
- A self-signed PAdES fixture proves deterministic structure, not public trust.
  Run `make check-pades-trusted` with a separately supplied trusted sample
  before claiming certificate-chain validation.
- Record native installation, upgrade, and removal evidence for each package
  format that the release advertises.

## Tag and publication

The tag must match the version of record. Use `v<version-of-record>` for a
general release or `v<version-of-record>-<prerelease>` for a prerelease.

```sh
git tag "v<version-of-record>"
git push origin "v<version-of-record>"
```

The Release workflow repeats the release gate before packaging. Wait for every
required package job, verify the uploaded artifact digests, then update
`docs/published-release-tags.txt` only after the GitHub release is public.

## After publication

- Install and open every advertised package on its supported target.
- Preserve the resulting verification evidence outside source documentation if
  it contains machine-specific paths, account details, or credentials.
- Update user-facing installation guidance only with behavior that was
  actually verified.
