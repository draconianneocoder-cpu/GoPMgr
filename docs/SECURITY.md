<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Security

GoPMgr is a local-first desktop application. Its primary security
boundary is the local machine and OS account. The app must protect
project data against casual local disclosure, accidental release drift,
and tampering of exported signed documents while remaining recoverable
for a legitimate local user.

## Local Accounts

- User credentials are stored in `system.db` as Argon2id PHC strings.
  Passwords must be at least 8 bytes (`users.ValidatePassword`, applied at
  account creation, password change, and recovery reset). Changing a
  password (`Store.ChangePassword`) verifies the current one and re-wraps
  the same DEK in one compare-and-swap write, so encrypted projects and
  recovery codes are unaffected and a concurrent change is not overwritten.
- Login errors should remain generic so unknown users and wrong
  passwords are not distinguishable.
- Per-user directories under the GoPMgr application data root are created with
  restrictive POSIX permissions where supported. A new account's folder is
  created with an exclusive `os.Mkdir`: if anything already exists at that
  path, such as a folder left by an account deleted before permanent
  deletion existed or by a deletion that could not finish (in any letter
  case on a case-insensitive filesystem), creation is refused instead of handing the
  old projects, certificates, and exports to the new account. `logs` is
  reserved for GoPMgr's own log folder.
- `system.db` file permissions are tightened to owner-only access where
  supported.
- The first account on a machine is always an administrator. After that,
  only a signed-in administrator can create accounts. `Store.CreateAccountAs`
  checks this in the same write transaction as the insert, reading the
  caller's role from `system.db`. Accounts from releases before this rule
  may have no administrator; one of them claims the role with
  `BecomeAdmin`, and until then nobody can add accounts.
- Administrators take an account out of use by disabling it (sign-in
  refused after the password matches; projects, key wraps, and recovery
  codes kept) or by permanently deleting it (`Store.PurgeAccount`: the
  account row, its recovery codes, and its folder, after the username is
  typed). Both read the acting administrator's role inside the transaction,
  so a session demoted or disabled by another GoPMgr process cannot still
  act. Purge is refused when another account's name differs only in letter
  case (such pairs predate the case-insensitive duplicate check of
  2026-06-20 and may share one folder). Last-administrator guards count
  only administrators who can sign in. Each disable, enable, and deletion is written to `account_events` in
  `system.db` in the same transaction as the change. Limits: the history is
  plaintext and anyone who can write `system.db` can change it (triggers
  only stop the app); a recovery-code reset still works on a disabled
  account but does not sign it in; a disabled user or demoted administrator
  already signed in through another GoPMgr process keeps working until they
  sign out; and the unauthenticated `ListUsers` method now also exposes
  each account's disabled flag.
- Administrators may read another user's data, and each access must be
  recorded with who read which account, when, and why (owner decision,
  2026-09-24, replacing the same day's rule that they must not). **Planned,
  not built:** an in-app way to do this, which needs the administrator to
  hold a copy of each user's encryption key and so changes ADR-001; the
  accepted design is
  [ADR-004](design/ADR-004-administrator-access-to-user-data.md): read-only,
  projects only, the user told at their next sign-in. Until then, an administrator can read only
  accounts they created, using the initial password or recovery codes they
  were given, outside the app, and nothing records it.
- Every IPC method that opens, mutates, or archives a project by a
  frontend-supplied path (`OpenProject`, `DeleteProject`, `CloneProject`,
  `EncryptProjectAtRest`, `SecureArchive`, etc.) is confined to the
  signed-in user's own `projects/` directory via `projectPathFor` in
  `app_projects.go`, rejecting anything outside it before touching disk. The
  target must also be an existing regular file in a real project folder: a
  symlinked project file or folder is refused, because the containment check
  is made on the path, and a missing file is refused rather than letting
  SQLite create an empty database in its place.
- Chart, schedule-baseline, document, stakeholder, and resource-calendar IPC
  methods resolve
  frontend-supplied record IDs against the currently open project. Combined
  reports apply the same rule to selected documents and linked charts. A
  record attached to another project row in the same database file is returned
  as not found rather than read, changed, exported, or deleted.

## Encryption At Rest

Project databases are SQLCipher-capable. The intended key hierarchy is:

1. Generate one per-user 32-byte data-encryption key.
2. Use that DEK as the SQLCipher raw key for the user's encrypted
   `.gopmgr` databases.
3. Store the DEK only in wrapped form.
4. Wrap the DEK with the login password and with each valid recovery
   code.

`system.db` deliberately remains openable before login. It should contain
only bootstrap metadata such as password hashes, recovery-code metadata,
and wrapped DEKs, not project content.

Before enabling encryption for a user with legacy recovery codes, reissue
recovery codes so password reset can preserve the same DEK. Otherwise a
reset would orphan encrypted project databases.

Recovery codes are a lasting way into an account, so App Settings issues new
ones only after the current password is verified, and wraps the DEK
unwrapped from that password into every code. The new set is prepared in
memory (`Store.PrepareRecoveryCodes`) and replaces the old codes only when
the user confirms they saved it (`Store.ConfirmRecoveryCodes`), in one
transaction that refuses if the codes changed meanwhile; signing out or
leaving the page discards it, so renewing codes never takes away working
codes before the new ones are saved. `App.IssueRecoveryCodes` refuses a session without its DEK instead of
issuing codes that could not recover it. App Settings counts a legacy code
(no DEK wrap) as unsafe, not as a working code.

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

A project's chain is deleted with the project, so deleting a project is
recorded outside it, in the signed-in user's deletion log
(`<data dir>/deletions.gopmgr`). The log is SQLCipher-encrypted under a
subkey of the session key, like the reusable catalog, and triggers refuse any
update or delete of its rows. `DeleteProject` commits a `requested` entry
(synchronous=FULL) before removing anything, then records `deleted` or
`failed`. Each entry keeps the project's ID, name, location, who deleted it,
and the audit chain's validity, verified-event count, and terminal hash at
that moment (for a chain that fails verification, the count and hash stop
at the first bad event), so a deleted project's name stays in the log. A file the user's key
cannot open is not deleted; a tampered chain does not block deletion and is
recorded as invalid.

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
  (`golang.org/x/crypto`): affects a code path GoPMgr does not call
  (`govulncheck` reports 0 reachable symbols). No fixed release exists,
  and none is expected: the advisory records that `x/crypto/openpgp` is
  unmaintained and unsafe by design, so its OSV entry carries no fixed
  version at all rather than a fix that has yet to ship (`govulncheck`
  reports `Fixed in: N/A`). It leaves this list only if GoPMgr's
  dependency tree stops reaching the package. The dependency tracks
  current upstream regardless. No version is named here on purpose: the
  next sentence licenses exactly the bumps that would falsify one, so
  `go.mod` is the only accurate source for the version in force. Routine
  version bumps that don't fix this advisory may be skipped without
  updating this note.

  A 2026-08-28 documentation-drift pass found `govulncheck` additionally
  reporting three advisories not previously tracked here — all likewise
  0 reachable, but each already had a fix available:
  [GO-2026-6303](https://pkg.go.dev/vuln/GO-2026-6303) (`x/crypto/ssh`,
  fixed in v0.55.0) and [GO-2026-6180](https://pkg.go.dev/vuln/GO-2026-6180)/
  [GO-2026-6179](https://pkg.go.dev/vuln/GO-2026-6179) (`x/mod/sumdb`, fixed
  in v0.40.0). Rather than document them as accepted risk, `x/crypto` was
  bumped to v0.55.0 and `x/mod` to v0.40.0 in the same pass, closing all
  three; `go build ./...`, `go vet ./...`, `go test ./...`, and
  `go test -race` on `internal/crypto`/`internal/db`/`internal/auth` all
  passed clean afterward. This is the mechanism by which advisories should
  keep leaving this list going forward: fix when a fix exists and nothing
  else breaks, document as accepted risk only when it doesn't.

## Reporting

This repository does not currently publish a public vulnerability intake
address. For private or commercial deployment, add a dedicated security
contact and a supported disclosure process before public release.
