<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later

Put at the top of every release's notes by .github/workflows/release.yml
(owner decision DEC-WINDOWS, 2026-11-14, and the 2026-11-11 system.db
downgrade note). make release-scope fails if the Windows notice or the
downgrade sentence is removed or stops saying so.
-->
> **Windows: unverified preview.** The Windows installer (`GoPMgr-<version>-amd64-setup.exe`)
> is built automatically, but no one has built, installed or run this release on a real
> Windows computer. GoPMgr does not support Windows yet. Try it only with fictional
> projects, expect problems, and keep backups. The installer has no Authenticode
> signature, so Windows SmartScreen warns about it.

**Do not go back to v1.1.0-alpha.2.** That build, and any earlier one, does not check the account database reader level. It still opens the account data. It does not understand disabled accounts, administrator access, or the super administrator. After a later build has opened that data, going back is unsupported and untested.

---
