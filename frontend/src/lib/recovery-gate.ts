// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// New projects are encrypted, so creating or encrypting one needs a way
// back in if the password is forgotten. recoveryGateNeeded asks the backend
// (RecoveryCodeStatus.encryption_ready, computed by the same check that
// refuses the action) whether RecoveryCodesGate must be shown first:
//   - 'legacy': the user's codes are from an older version and must be
//     replaced;
//   - 'none': the user has no unused codes (they may accept going without);
//   - null: go ahead.
//
// If the status cannot be read, it returns null and lets the backend decide:
// the action itself is refused when it should be, and callers ask again.

export type RecoveryGate = 'none' | 'legacy';

export async function recoveryGateNeeded(): Promise<RecoveryGate | null> {
  try {
    const status = await window.go.main.App.RecoveryCodeStatus();
    if (status.encryption_ready) return null;
    return status.legacy ? 'legacy' : 'none';
  } catch {
    return null;
  }
}
