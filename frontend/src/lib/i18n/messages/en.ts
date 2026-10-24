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
  'common.password.minLength': 'At least 8 characters',
  'common.password.match': '✓ Passwords match',
  'common.password.mismatch': 'Passwords don’t match yet',
  'common.password.error.mismatch': 'Passwords do not match.',
  'common.password.current': 'Current password',
  'common.password.new': 'New password',
  'common.password.confirmNew': 'Confirm new password',
  'common.backToSignIn': 'Back to sign in',
  'common.cancel': 'Cancel',
  'common.tryAgain': 'Try again',
  'common.loading': 'Loading…',
  'common.creating': 'Creating…',
  'common.saving': 'Saving…',

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
  'createAccount.confirm': 'Confirm password',
  'createAccount.error.username': 'Username must be 3–32 letters, digits, _ or -.',
  'createAccount.error.password': 'Password must be at least 8 characters.',
  'createAccount.error.notFirst': 'This computer already has GoPMgr accounts. Ask your administrator to create yours.',
  'createAccount.submit': 'CREATE ACCOUNT',
  'createAccount.submitting': 'Creating…',

  'recoveryReset.title': 'Reset password',
  'recoveryReset.intro': 'Enter one of the recovery codes you saved when the account was created.',
  'recoveryReset.passwordOnly': 'This changes only your password. Your encrypted projects stay readable.',
  'recoveryReset.code': 'Recovery code',
  'recoveryReset.done': 'Password reset. Your projects are unchanged. Sign in with your new password.',
  'recoveryReset.done.back': 'BACK TO SIGN IN',
  'recoveryReset.submit': 'RESET PASSWORD',
  'recoveryReset.submitting': 'Resetting…',
  'recoveryReset.error.required': 'All fields are required.',
  'recoveryReset.error.length': 'New password must be at least 8 characters.',
  'recoveryReset.error.legacy':
    "This recovery code is from an older version of GoPMgr and can't unlock your encryption key, so nothing was changed and your projects are safe. Sign in with your password if you remember it.",
  'recoveryReset.error.invalid': 'Invalid username or recovery code.',

  'changePassword.title': 'Change password',
  'changePassword.showPasswords': 'Show passwords',
  'changePassword.submit': 'Change password',
  'changePassword.submitting': 'Changing…',
  'changePassword.changed': 'Password changed. Your projects and recovery codes are unaffected.',

  // {action} is recoveryPanel.useNew, the button name, so it reads the same.
  'recoveryPanel.title': 'Recovery codes',
  'recoveryPanel.statusFailed': 'Could not check your recovery codes: {error}',
  'recoveryPanel.unused': {
    one: '{unused} of {count} recovery code unused.',
    other: '{unused} of {count} recovery codes unused.',
  },
  'recoveryPanel.legacy':
    "Your recovery codes are from an older version of GoPMgr and can't recover encrypted projects. Create new ones.",
  'recoveryPanel.none':
    "You have no unused recovery codes. If you forget your password, you can't get back into your encrypted projects. Create new ones now.",
  'recoveryPanel.oneLeft': 'Only one recovery code is left. Create new ones before you need it.',
  'recoveryPanel.unusedFiles': {
    one: "The codes saved to {files} were never put in use and won't work. Delete that file and any copy you printed.",
    other:
      "The codes saved to {files} were never put in use and won't work. Delete those files and any copies you printed.",
  },
  'recoveryPanel.create': 'Create new recovery codes',
  'recoveryPanel.keepWorking': 'Your current codes keep working until you use the new ones.',
  'recoveryPanel.createCodes': 'Create codes',
  'recoveryPanel.saveThese':
    'Save these codes somewhere safe, such as a password manager or a printed copy. Each works once, and they are shown only now. Your current codes keep working until you choose “{action}”.',
  'recoveryPanel.savedHint': 'These codes start working only when you choose “{action}”.',
  'recoveryPanel.savedCheck': 'I have saved these codes somewhere safe.',
  'recoveryPanel.useNew': 'Use the new codes',
  'recoveryPanel.keepCurrent': 'Keep my current codes',
  'recoveryPanel.renewed': 'New recovery codes are in use. Your old codes no longer work.',

  'recoveryReminder.title': 'Keep a way back into your account',
  'recoveryReminder.notNow': 'Not now',

  'recoveryGate.title': 'Before you continue: save a way back in',
  'recoveryGate.legacy':
    "Your recovery codes are from an older version of GoPMgr and can't recover encrypted projects. Create new ones below; GoPMgr carries on as soon as they're saved.",
  'recoveryGate.none':
    "Projects are encrypted, and only your password or a recovery code opens them. You have no unused recovery codes, so if you forget your password this project can't be recovered. Create codes below; GoPMgr carries on as soon as they're saved.",
  'recoveryGate.understood':
    "I understand that projects I create or encrypt until I sign out can't be recovered if I forget my password.",
  'recoveryGate.continueWithout': 'Continue without recovery codes',
  'recoveryGate.error': 'Could not continue: {error}',

  'adminAccess.title': 'Administrator access to your data',
  'adminAccess.reason': 'Reason: {reason}',
  'adminAccess.history.loadFailed': 'Could not load the record of administrator access: {error}',
  'adminAccess.history.none': 'No administrator has opened your data.',
  'adminAccess.history.entry': '{time} — {actor} opened your data.',
  'adminAccess.notice.checkFailed': "GoPMgr couldn't check whether an administrator opened your data: {error}",
  'adminAccess.notice.openedTitle': {
    one: 'An administrator opened your data',
    other: 'Administrators opened your data',
  },
  'adminAccess.notice.newKeyTitle': {
    one: 'A new administrator key was made',
    other: 'New administrator keys were made',
  },
  'adminAccess.notice.readOnly':
    'They could see your projects read only and could not change anything. App Settings, under Account, lists every time this has happened.',
  'adminAccess.notice.opened': '{actor} opened your data on {time}.',
  'adminAccess.notice.newKeyBody':
    "The administrator key lets the super administrator open your data. Nobody could open the old one, so a new one was made, and your data is moved to it when you sign in. You're told here each time an administrator opens your data with it.",
  'adminAccess.notice.newKey': '{actor} made a new administrator key on {time}.',
  'adminAccess.notice.ackFailed': "Could not save that you've read this: {error}",
  'adminAccess.notice.acknowledge': "I've read this",

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
