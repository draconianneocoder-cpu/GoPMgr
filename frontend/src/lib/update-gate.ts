// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Decides whether App.svelte should fire an automatic update check on this
// sign-in. Pulled out of App.svelte (which has no test harness) so the
// privacy-relevant part of that decision -- never checking without the
// user's opt-in -- has a direct unit test instead of resting on an
// untested conditional.
export function shouldAutoCheckForUpdates(
  settings: { auto_check_updates?: boolean } | undefined | null,
  alreadyCheckedThisSession: boolean,
): boolean {
  return Boolean(settings?.auto_check_updates) && !alreadyCheckedThisSession;
}
