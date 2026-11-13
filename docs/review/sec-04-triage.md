<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# SEC-04 triage: low-severity scan items

**Date:** 2026-11-13, against `main` at `d764e50`.
**Step:** A5 of the [response plan](evaluation-2026-11-10-response-plan.md).
**Source:** the 2026-11-10 multi-agent scan, summarised in the fifth pass of the
[code-health audit](../engineering/code-health-audit-2026-11-09.md). It reported
16 low-severity items. The full report was kept out of the repository, and the
audit names only six of them. This record covers those six. **The other ten
cannot be triaged until the owner supplies the report;** A5 stays open until
then.

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

## Tests

The evaluation asks for a deterministic regression test per item. Item 2 has
one (`ProjectSettings.test.ts`). Item 5's path check has tests at all four
keyed opens. Items 1, 3 and 4 change nothing yet, so a test follows the
owner's decision. Item 6 is SEC-09.

## Not covered

- The ten unnamed items in the scan report.
- No item here was re-run against Windows or macOS; the code paths are
  platform-neutral except the clipboard, whose behaviour depends on the
  webview.
