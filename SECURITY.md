<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Security

PMForge is a local-first desktop application. Its primary security
boundary is the local machine and OS account. The app must protect
project data against casual local disclosure, accidental release drift,
and tampering of exported signed documents while remaining recoverable
for a legitimate local user.

## Local Accounts

- User credentials are stored in `system.db` as Argon2id PHC strings.
- Login errors should remain generic so unknown users and wrong
  passwords are not distinguishable.
- Per-user directories under `~/Documents/PMForge/<username>/` are
  created with restrictive POSIX permissions where supported.
- `system.db` file permissions are tightened to owner-only access where
  supported.

## Encryption At Rest

Project databases are SQLCipher-capable. The intended key hierarchy is:

1. Generate one per-user 32-byte data-encryption key.
2. Use that DEK as the SQLCipher raw key for the user's encrypted
   `.pmforge` databases.
3. Store the DEK only in wrapped form.
4. Wrap the DEK with the login password and with each valid recovery
   code.

`system.db` deliberately remains openable before login. It should contain
only bootstrap metadata such as password hashes, recovery-code metadata,
and wrapped DEKs, not project content.

Before enabling encryption for a user with legacy recovery codes, reissue
recovery codes so password reset can preserve the same DEK. Otherwise a
reset would orphan encrypted project databases.

Plaintext-to-encrypted migration must:

- Reject already encrypted sources.
- Verify the plaintext source before export.
- Export with `sqlcipher_export` into a temporary encrypted sibling.
- Verify encrypted integrity before publish.
- Retain the plaintext source as `<project>.pre-encryption.bak`.
- Tighten file permissions on the published encrypted database.

## Secrets

Never log or commit:

- Passwords.
- Recovery codes.
- Raw DEKs or wrapped key ciphertexts from real users.
- SQLCipher keys or raw key DSNs.
- Private signing keys.
- Local certificates unless they are test fixtures.
- Generated user project databases or exports containing real data.

Use deterministic test keys and temporary directories in tests.

## Document Security

PAdES signing must be the final PDF mutation because it signs byte
ranges. PDF/A metadata, XMP, output intents, and rendering changes must
happen before `pdfmeta.InjectPAdESSignature`.

The deterministic local PAdES sample is self-signed. Validator output can
prove structural correctness and tamper evidence, but trusted-chain
validity requires a trusted signing source in the release environment.

## Audit Integrity

Each project database keeps a tamper-evident `audit_events` hash chain.
Every event stores `event_hash = sha256(previous_event_hash ||
canonicalJSON(payload))`, chaining it to the prior event. Project, chart,
document, schedule-baseline, scenario, scenario-chart-copy, approval,
signature, and signed combined-report lifecycle actions append to the
chain. Canonical JSON (marshal, unmarshal, re-marshal) makes each digest
independent of key order.

`VerifyAuditChain` recomputes every hash and checks sequence continuity and
the previous-hash links. When a project has **Compliance Mode** enabled,
`OpenProject` runs this verification first (`verifyProjectAuditForOpen`) and
refuses to open a project whose chain has been altered. Compliance Mode is
opt-in per project; with it off, tampering is not detected on open. The
`Export audit verification report` and `Export audit repair evidence`
actions write JSON evidence to the user's private exports folder without
mutating the project database.

Audit integrity detects tampering; it does not by itself prevent it — a
holder of the DEK can rewrite the database. It complements, and does not
replace, encryption at rest and OS-level disk encryption.

## Release Safety

Run the relevant gates before release claims:

```sh
make license-check
make release-scope
make memory-scan
make check-pdfa
make check-pades
make check-pades-external
make check-release
```

`make release-scope` protects important public claims such as PDF/A,
PAdES, and encryption status from drifting away from supported behavior.

### Known upstream advisories

- [GO-2026-5932](https://pkg.go.dev/vuln/GO-2026-5932)
  (`golang.org/x/crypto`): affects a code path PMForge does not call
  (`govulncheck` reports 0 reachable symbols). No fixed upstream release
  exists yet; the dependency is at the latest version and will be bumped
  when a fix ships.

## Reporting

This repository does not currently publish a public vulnerability intake
address. For private or commercial deployment, add a dedicated security
contact and a supported disclosure process before public release.
