// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import type { Message } from '../catalog';

// English, the source language. Every other catalog translates these keys
// (docs/design/ADR-008-localization.md). Keys are grouped by screen; text in
// {braces} is a placeholder and stays as written in every language.
export const en = {
  'common.password.show': 'Show password',
  'common.password.hide': 'Hide password',
  'common.username': 'Username',
  'common.password': 'Password',

  'login.tagline': 'Local-first project controls',
  'login.noAccounts': "No accounts yet. The first account you create becomes this computer's GoPMgr administrator.",
  'login.noAdmin': 'No administrator is configured. Sign in, then use Become administrator in App Settings.',
  'login.submit': 'SIGN IN',
  'login.submitting': 'Signing in…',
  'login.createAccount': 'Create a new account',
  'login.contactAdmin': 'Contact your administrator to create an account on this machine.',
  'login.forgotPassword': 'Forgot password? Use a recovery code',
  'login.error.disabled': 'This account is disabled. Ask your administrator to enable it.',
  'login.error.invalid': 'Invalid username or password.',

  'createAccount.title': 'Create your account',
  'createAccount.local': 'Everything you create stays on this computer — nothing is uploaded.',
  'createAccount.firstUser.title': "You're the first user on this computer",
  'createAccount.firstUser.body':
    'This account will be the GoPMgr administrator. Administrators create and remove the other accounts on this computer.',
  'createAccount.username.hint': '3–32 characters; letters, digits, _ or -',
  'createAccount.displayName': 'Display name',
  'createAccount.displayName.hint': 'Optional — shown in the app. Defaults to your username.',
  'createAccount.password.hint': 'At least 8 characters',
  'createAccount.confirm': 'Confirm password',
  'createAccount.confirm.match': '✓ Passwords match',
  'createAccount.confirm.mismatch': 'Passwords don’t match yet',
  'createAccount.error.username': 'Username must be 3–32 letters, digits, _ or -.',
  'createAccount.error.password': 'Password must be at least 8 characters.',
  'createAccount.error.mismatch': 'Passwords do not match.',
  'createAccount.error.notFirst': 'This computer already has GoPMgr accounts. Ask your administrator to create yours.',
  'createAccount.submit': 'CREATE ACCOUNT',
  'createAccount.submitting': 'Creating…',
  'createAccount.back': 'Back to sign in',

  'recoveryCodes.title': 'Save your recovery codes',
  'recoveryCodes.failed.title': 'Recovery codes could not be generated',
  'recoveryCodes.failed.body':
    "Your account was created and you're signed in. Create your recovery codes in App Settings, under Account. ({error})",
  'recoveryCodes.onlyWay.title': 'These are the only way back into your account',
  'recoveryCodes.onlyWay.body':
    "If you forget your password, a recovery code is the only way to reset it and keep your encrypted projects. Save them somewhere safe — you won't be shown them again.",
  'recoveryCodes.saved': 'I have saved my recovery codes somewhere safe.',
  'recoveryCodes.continue': 'CONTINUE TO GOPMGR',
} as const satisfies Record<string, Message>;

export type MessageKey = keyof typeof en;
