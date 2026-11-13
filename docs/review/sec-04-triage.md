<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# SEC-04 triage: low-severity scan items

**Date:** 2026-11-13, against `main` at `d764e50`.
**Step:** A5 of the [response plan](evaluation-2026-11-10-response-plan.md).
**Source:** the 2026-11-10 multi-agent scan, summarised in the fifth pass of the
[code-health audit](../engineering/code-health-audit-2026-11-09.md). It reported
16 low-severity items. The full report is kept out of the repository; it was
consulted privately for this record. Items 1 to 6 below are the six the audit
names (item 6 stands for two npm advisories). Items 7 to 15 are the other nine,
triaged on 2026-11-13. All sixteen now have a result.

**Threat model.** GoPMgr is local-first ([SECURITY.md](../SECURITY.md)): the
boundary is the machine and the OS account. Someone who can act as the user's
OS account can already read `system.db` (plaintext), list the per-account
folders, and copy any file. The items below are judged against that.

Each item has a result (confirmed, refuted, or confirmed as designed), what was
done now, and what needs an owner decision.

| # | Item | Result | Done now | Owner decision |
| --- | --- | --- | --- | --- |
| 1 | Recovery codes left on the clipboard | **Confirmed.** `RecoveryCodeList.svelte` writes every code with `navigator.clipboard.writeText` and never clears it. A code unlocks the account's key, so it stays readable to any program, clipboard history or sync until something else is copied. | Nothing. | Clear the clipboard after a short time if it still holds the codes, warn beside the Copy button, or drop Copy and keep Save and Print. Clearing needs a desktop clipboard call that can read before it clears, so it does not wipe something else the user copied. |
| 2 | Plaintext `.pre-encryption.bak` kept after encryption | **Confirmed, as designed.** [SECURITY.md](../SECURITY.md) requires the copy to be kept. `db.MigratePlaintextToEncrypted` (`internal/db/encryption.go`) removes it if anything fails before the encrypted file is published, so it survives only a successful migration: it is a way back if the encrypted file later cannot be opened, not what makes a failed encryption recoverable. It holds the whole project unencrypted, with mode 0600. After encryption the screen said only "Backup retained at: …", and the help page did not say the copy is unencrypted. The notice is cleared on refresh or leaving the screen, and nothing reminds the user later. | The notice now says the copy is unencrypted, that anyone who can read it can read the project, and to delete it once the encrypted project opens (`ProjectSettings.svelte`, text in the message catalog; tested in `ProjectSettings.test.ts`, which fails on the old text). The help page says the same. | Whether GoPMgr should remind the user about the copy later, offer to delete it, or delete it itself once the encrypted file has opened. Deleting it changes what SECURITY.md requires. |
| 3 | No sign-in attempt limiting | **Confirmed, low value to fix.** Nothing limits attempts. A failed attempt costs one password hash (Argon2id, three passes over 64 MiB). Creating an account, which also wraps a key, measured 44 ms in the audit, so an attempt costs less. The password hashes are in plaintext `system.db`, so an attacker with the user's OS account can guess offline at the same cost without the app; a limit in the app would not slow that. | Nothing. | Accept as residual risk, or add a delay or lock-out for people at the keyboard. A lock-out also lets anyone lock a user out. |
| 4 | Sign-in timing reveals whether a username exists | **Confirmed; reveals nothing new.** An unknown name returns before the Argon2id work. Usernames are already readable locally: they are rows in plaintext `system.db` and the names of the per-account folders. A comment in `App.Login` claimed the timing was identical. | The comment is corrected (`internal/desktop/app_session.go`). The message is already the same for both cases. | Accept as residual risk. Equal timing would need a dummy password hash for unknown names, with the same time and 64 MiB of memory per attempt. |
| 5 | SQLCipher key kept in the connection's DSN | **Refuted as a disclosure.** The key is in the DSN string that `database/sql` keeps for reconnecting. No log or error message prints it: the driver's parse errors quote only an invalid option value, never the key. Anyone who can read the process's memory already has the key itself. The key `exportEncryptedCopy` puts in its ATTACH statement is not in any error either. Checked as well: a path containing `?` would be split at the wrong place by the driver, but all four keyed DSN builders refuse `?` and `#` in paths: `encryptedDSN` (also used by repair) and `readOnlyFileURI` in `internal/db`, and the opens in `internal/catalog` and `internal/deletionlog`. Tests pin this for the first two (`internal/db/encryption_readonly_test.go`), the deletion log (`internal/deletionlog/log_test.go`) and, since this triage, the catalog (`TestCatalogRejectsDSNCharactersInPath`; with the check removed, the driver creates a file at the truncated path). | Added the catalog's missing path test. | None needed. Moving the key out of the DSN (a `PRAGMA key` on each new connection) would not remove it from memory. |
| 6 | Development-only npm advisories | Already tracked as **SEC-09**, open by decision (no compatible fix). | Nothing. | None new. |
| 7 | A restored archive's certificate path copied any file into a later backup | **Fixed before this triage** (PR 198, and PR 202 for SEC-03). The path must end in `.p12` or `.pfx` and be a regular file within a size limit, a named pipe is refused without blocking, and a restore clears the archive's signing settings. Tests: `TestCreateArchivalBundleRefusesAFIFOWithoutBlocking`, `TestCreateArchivalBundleRefusesAFileThatIsNotACertificateBundle`, `TestRestoreClearsTheBackupsSigningSettings`. | Nothing. | None. |
| 8 | An old SQLCipher advisory (a crash during plaintext-to-encrypted export) | **Fixed before this triage.** The engine was upgraded to SQLCipher 4.20.0 / SQLite 3.54.1 (PR 196). `TestVendoredEngineIsTheReviewedOne` (A4, this pull request) fails if the vendored engine changes from the reviewed one. | Nothing. | None. |
| 9 | Prebuilt Rust libraries in `zen-go` that Go's vulnerability tools cannot see | **Fixed before this triage.** `zen-go` was removed and the Launchpad rules are evaluated in Go (PR 199); `go.mod` no longer names it. | Nothing. | None. |
| 10 | A restored archive's own database schema was trusted | **Fixed before this triage** as SEC-01 and SEC-02 (PRs 196, 199 and 202): the engine distrusts file schemas and runs in defensive mode, and restore refuses triggers, views, virtual tables and any table, column, constraint or index beyond the schema the build creates. Tests: `TestSchemaIsUntrustedByDefault`, `TestDefensiveModeIsOnByDefault`, `TestRestoreRefusesAnArchiveWithAPlantedTrigger`, `TestUnexpectedSchemaReportsWhatGoPMgrNeverCreates`. | Nothing. | None. |
| 11 | Ledger attachment import opened a named pipe and could hang | **Confirmed and fixed.** `readBoundedFile` opened the chosen path with a blocking open and no regular-file check, unlike the other import readers. A named pipe with no writer hung the bound call. | It now opens without blocking and refuses anything that is not a regular file, checked on the open handle (`internal/desktop/app_cost_control_attachments.go`, `attachment_open_*.go`). `TestReadBoundedFileRefusesANamedPipe` hung for its 5-second limit before the fix; `TestReadBoundedFileRefusesADirectory` covers the check itself. | None. |
| 12 | A project named with `#` could not be created, and left an empty folder | **Confirmed and fixed.** The name became part of the folder path, which the encrypted database refuses when it contains `#` or `?`. The failure left the reserved folder behind. A comment in `encryptedDSN` wrongly said project paths never contain these characters. | Project folder names now replace `#` and `?` (`projectFolderSafe`). The project's database keeps the name as typed; the project list and a clone's name come from the folder, so they show `_` for `#`, as they already did for the other characters folder names replace. A failed create removes the empty folder, in `CreateProject` and the Launchpad. The comment is corrected. Tests: `TestCreateProjectAcceptsAHashInItsName` (it failed before), `TestCreateProjectRemovesTheFolderItCouldNotUse` (one folder was left behind before). | None. |
| 13 | The plaintext opener had no `?` guard | **Confirmed as a hardening gap, not an exploit;** the scan's own refutation pass agreed. `InitDB` passed the path to the driver, which reads everything after the first `?` as connection options: `proj?_journal_mode=OFF` opened `proj` with weaker settings. Every caller passes a path GoPMgr built, except the command line's own `--project`. | `InitDB` refuses a path containing `?` and creates nothing (`TestInitDBRefusesAQuestionMarkInThePath`; before, it created the truncated file). A `#` is ordinary in a plaintext path, unlike in the encrypted path's URI, so existing plaintext projects named with one still open and can be migrated (`TestInitDBStillOpensAHashInThePath`). | None. |
| 14, 15 | Two further findings | **Confirmed.** Details are held privately and are not published while unfixed. | Nothing. | A design decision for the owner. |

## Tests

The evaluation asks for a deterministic regression test per item. Item 2 has
one (`ProjectSettings.test.ts`). Item 5's path check has tests at all four
keyed opens. Items 1, 3 and 4 change nothing yet, so a test follows the
owner's decision. Item 6 is SEC-09. Items 7 to 10 cite the tests that prove
the earlier fixes. Items 11 to 13 each have a test that failed before its fix,
and each fix was removed again to confirm its test fails. Items 14 and 15
wait for the owner's decision.

## Not covered

- No item here was re-run against Windows or macOS; the code paths are
  platform-neutral except the clipboard, whose behaviour depends on the
  webview.
