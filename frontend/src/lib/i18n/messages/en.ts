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

  'sigma.solutions.cost': 'Cost',
  'sigma.solutions.costIn': 'Cost ({currency})',

  'portfolio.analyticsFailed': 'Analytics failed: {error}',
  'portfolio.importFailed': 'Import failed: {error}',
  'portfolio.importTruncated': 'The file has more rows than this. Only the first {count} were read.',

  'common.close': 'Close',
  'common.delete': 'Delete',
  'common.save': 'Save',

  // The agile work-item editor. Type, priority and backlog labels name the
  // stored values, which stay in English in the database.
  'workItem.title.edit': 'Edit work item',
  'workItem.title.new': 'New work item',
  'workItem.field.title': 'Title',
  'workItem.field.type': 'Type',
  'workItem.field.priority': 'Priority',
  'workItem.field.assignee': 'Assignee',
  'workItem.field.state': 'State / Column',
  'workItem.field.sprint': 'Sprint',
  'workItem.field.description': 'Description',
  // The estimate field's name depends on the project's methodology (see
  // i18n/methodology.ts): story points for Scrum and Scrumban.
  'workItem.field.points': 'Points',
  'workItem.field.storyPoints': 'Story points',
  'workItem.unassigned': '(unassigned)',
  'workItem.noSprint': '(none)',
  'workItem.type.story': 'story',
  'workItem.type.bug': 'bug',
  'workItem.type.task': 'task',
  'workItem.type.epic': 'epic',
  'workItem.priority.low': 'low',
  'workItem.priority.medium': 'medium',
  'workItem.priority.high': 'high',
  'workItem.priority.urgent': 'urgent',
  'workItem.state.backlog': 'backlog',
  'workItem.id': 'ID: {id}',
  'workItem.closedAt': '· closed at {date}',
  'workItem.error.titleRequired': 'Title is required.',
  'workItem.delete.title': 'Delete work item',
  'workItem.delete.message': 'Delete "{title}"? This cannot be undone.',

  // Shared by the chart editors (Gantt, CPM, RACI). Schedule rows are
  // "tasks" for every methodology (ADR-008, 2026-11-09 decision).
  'chart.loadFailed': 'Could not load this chart: {error}',
  'chart.backToDashboard': 'Back to dashboard',
  'chart.dashboard': '← Dashboard',
  'chart.savedAt': 'Saved {time}',
  'chart.autosaveHint': 'Charts save automatically as you edit',
  'chart.saved': 'Saved',
  'chart.baselineSet': 'Baseline set',
  'chart.taskDeleted': 'Task deleted',
  'chart.newTask': 'New task',
  'chart.addTask': '+ Task',
  'chart.task': 'Task',
  'chart.deleteTask': 'Delete task',
  'chart.milestone': 'Milestone',
  'chart.previewSplitting': 'Preview splitting',
  'chart.previewSplitting.hint':
    'Check (read-only) whether splitting tasks across non-contiguous days would clear overallocation. Nothing is saved.',
  'chart.setBaseline': 'Set baseline',
  // Ends a sentence built from a list, e.g. a bar's tooltip.
  'common.sentence': '{text}.',

  'gantt.title': 'Gantt Chart',
  'gantt.zoomOut': 'Zoom out',
  'gantt.zoomIn': 'Zoom in',
  'gantt.setBaseline.hint': 'Snapshot for baseline ghost bars',
  'gantt.levelSplit': 'Level (split)',
  'gantt.levelSplit.hint':
    'Level resources allowing tasks to be split across non-contiguous days; split tasks render as interrupted bars',
  'gantt.days': 'Days',
  'gantt.links': 'Links (FS/SS/FF/SF ± lag)',
  'gantt.link.from': 'from…',
  'gantt.link.to': 'to…',
  'gantt.legend':
    'Red bars are critical; teal strip = % complete; grey ghost = baseline; orange outline = overallocated resource; amber ! = constraint violated. Split tasks show interrupted bars. Real dates appear when the project has a start date. Link labels accept FS/SS/FF/SF with ±lag days.',
  // The bar canvas's description for screen readers. {tasks} and {days} are
  // gantt.summary.tasks and gantt.summary.days.
  'gantt.summary': 'Gantt chart: {tasks} across {days}.',
  'gantt.summary.critical': 'Gantt chart: {tasks} across {days}, {critical} on the critical path.',
  'gantt.summary.tasks': { one: '{count} task', other: '{count} tasks' },
  'gantt.summary.days': { one: '{count} day', other: '{count} days' },
  // Parts of one bar's tooltip, listed in order.
  'gantt.bar.untitled': '(untitled task)',
  'gantt.bar.milestone': 'milestone at day {day}',
  'gantt.bar.days': 'day {start} to {finish}',
  'gantt.bar.complete': '{percent}% complete',
  'gantt.bar.critical': 'critical path',
  'gantt.bar.overallocated': 'resource overallocated',
  'gantt.bar.constraintViolated': 'constraint violated',
  'gantt.bar.dates': '{start} to {finish}',

  // Resource-leveling results, shared by the Gantt and CPM editors.
  'leveling.moreLabels': '{labels} +{count} more',
  'leveling.pinned': {
    one: 'Levelled: {count} task pinned (SNET)',
    other: 'Levelled: {count} tasks pinned (SNET)',
  },
  'leveling.noneShifted': 'No tasks could be shifted into free capacity',
  'leveling.alreadyLevel': 'Already level: nothing moved',
  'leveling.stillOverallocated': {
    one: '{count} task still overallocated: {labels}',
    other: '{count} tasks still overallocated: {labels}',
  },
  'leveling.stillOverallocated.hint':
    'Demand exceeds available capacity for: {labels}. Reduce assigned units, add resource capacity, or split the work across days.',
  'leveling.noSplitNeeded': 'No tasks need splitting at current capacity',
  'leveling.splitClears': {
    one: 'Splitting {count} task would clear overallocation: {labels}',
    other: 'Splitting {count} tasks would clear overallocation: {labels}',
  },
  'leveling.splitClears.hint':
    'Interrupting these tasks across non-contiguous days resolves all resource conflicts: {labels}. Splitting is analysis-only and is not saved.',
  'leveling.splitStuck': {
    one: 'Even with splitting, {count} resource stays over capacity',
    other: 'Even with splitting, {count} resources stay over capacity',
  },
  'leveling.splitStuck.hint':
    'These resources have single-day demand above supply, which splitting cannot fix: {labels}. Reduce assigned units or add capacity.',
  'leveling.splitApplied': {
    one: 'Levelled; split {count} task: {labels}',
    other: 'Levelled; split {count} tasks: {labels}',
  },
  'leveling.splitNone': 'Levelled; no tasks needed splitting',

  'raci.title': 'RACI Matrix',
  'raci.issues': { one: '{count} issue', other: '{count} issues' },
  'raci.layoutFailed': 'Layout failed: {error}',
  'raci.roleExists': 'Role {name} already exists.',
  'raci.roleDeleted': 'Role "{name}" deleted',
  'raci.savedAt': 'Saved at {time}.',
  'raci.saveFailed': 'Save failed: {error}',
  'raci.newRole': 'New role',
  'raci.newRole.example': 'e.g. Engineering Lead',
  'raci.addRole': '+ Role',
  'raci.newTask': 'New task',
  'raci.newTask.example': 'e.g. Draft the project charter',
  'raci.empty': 'Add at least one role and one task to start the matrix.',
  'raci.removeRole': 'Remove role {role}',
  'raci.remove': 'remove',
  'raci.taskName': 'Task name',
  'raci.cell': '{task} — {role} responsibility',
  'raci.removeTask': 'Remove task',
  'raci.validation': 'Validation',
  // The letters R, A, C and I are stored values and stay as written.
  'raci.legend.responsible': 'Responsible — does the work.',
  'raci.legend.accountable': 'Accountable — owns the outcome (exactly one per task).',
  'raci.legend.consulted': 'Consulted — provides input before work happens.',
  'raci.legend.informed': 'Informed — kept up to date after work happens.',

  // The CPM editor. Abbreviations such as ES, LF, BAC and SPI are the
  // standard schedule and earned-value terms; most languages keep them.
  'cpm.title': 'CPM Chart',
  'cpm.days': '{days}d',
  'cpm.variance.onPlan': 'on plan',
  'cpm.variance.late': '+{days}d late',
  'cpm.variance.early': '{days}d early',
  // A dismissable warning button; {warning} is a leveling message.
  'cpm.resourceWarn.dismiss': '⚠ {warning} (dismiss)',
  'cpm.histogramSaved': 'Histogram saved: {title}',
  'cpm.exportedTo': 'Exported to: {path}',
  'cpm.exportFailed': 'Export failed: {error}',
  'cpm.level.strategy.hint':
    'Leveling heuristic: least total float protects the project finish; earliest deadline favours per-task due dates.',
  'cpm.level.leastFloat': 'Least float',
  'cpm.level.earliestDeadline': 'Earliest deadline',
  'cpm.level.protectCritical': 'Protect critical',
  'cpm.level.protectCritical.hint':
    'Protect the critical path: critical tasks win resource contention ahead of floating tasks.',
  'cpm.level': 'Level',
  'cpm.level.hint': 'Delay contended tasks until resources fit capacity; delays persist as SNET constraints',
  'cpm.previewSplitting.hint':
    'Preview (read-only) whether interrupting tasks across non-contiguous days would clear overallocation. Not saved.',
  'cpm.histogram': 'Histogram',
  'cpm.histogram.hint': 'Save a bar chart of per-day resource demand (snapshot; regenerate after edits)',
  'cpm.baseline.hint': 'Snapshot the current schedule for planned-vs-actual comparison',
  'cpm.rebaseline': 'Re-baseline ({count})',
  'cpm.node.durationFloat': 'Dur: {duration} · Float: {float}',
  'cpm.node.early': 'ES {es} · EF {ef}',
  'cpm.node.late': 'LS {ls} · LF {lf}',
  'cpm.duration': 'Duration (days)',
  'cpm.estimate': 'Monte Carlo estimate',
  'cpm.estimate.clear': 'Clear',
  'cpm.estimate.use': 'Use estimate',
  'cpm.estimate.optimistic': 'Optimistic',
  'cpm.estimate.likely': 'Likely',
  'cpm.estimate.pessimistic': 'Pessimistic',
  'cpm.estimate.distribution': 'Distribution',
  'cpm.estimate.triangular': 'Triangular',
  'cpm.estimate.betaPert': 'Beta-PERT',
  'cpm.estimate.normal': 'Normal',
  'cpm.estimate.hint':
    'Leave blank for deterministic duration. Add a three-point estimate for schedule-risk simulation.',
  'cpm.percentComplete': '% Complete',
  'cpm.budgetedCost': 'Budgeted cost',
  'cpm.actualCost': 'Actual cost',
  'cpm.actualStart': 'Actual start',
  'cpm.actualFinish': 'Actual finish',
  'cpm.assignments': 'Assignments',
  'cpm.assignment.resource': 'Resource',
  'cpm.assignment.units': 'Units (1 = full-time)',
  'cpm.assignment.remove': 'Remove assignment',
  'cpm.assignment.calendar': 'Calendar',
  'cpm.assignment.calendar.hint': 'Optional named resource calendar',
  'cpm.assignment.max': 'Max',
  'cpm.assignment.max.hint': 'Optional max units for this assignment',
  'cpm.assignment.skills': 'Skills',
  'cpm.assignment.skills.hint': 'Comma-separated skill tags',
  'cpm.assignment.add': '+ Assign resource',
  'cpm.overallocated':
    'Overallocated: a resource on this task exceeds its capacity on at least one scheduled day, including Resource Capacity calendar overrides. Reduce units, move the task, or add capacity.',
  'cpm.constraint': 'Constraint',
  'cpm.constraint.asap': 'ASAP (default)',
  'cpm.constraint.alap': 'As late as possible',
  'cpm.constraint.snet': 'Start no earlier than',
  'cpm.constraint.fnlt': 'Finish no later than',
  'cpm.constraint.mfo': 'Must finish on',
  'cpm.constraint.date': 'Constraint date',
  'cpm.constraint.violated':
    'Constraint violated: precedence links make this constraint unsatisfiable. Links win; adjust the constraint date, the durations, or the links.',
  'cpm.startDate': 'Start date',
  'cpm.finishDate': 'Finish date',
  'cpm.es': 'Earliest start (ES)',
  'cpm.ef': 'Earliest finish (EF)',
  'cpm.ls': 'Latest start (LS)',
  'cpm.lf': 'Latest finish (LF)',
  'cpm.float': 'Float',
  'cpm.float.critical': '{float} (critical)',
  'cpm.baseline': 'Baseline',
  'cpm.startVsBaseline': 'Start vs baseline',
  'cpm.finishVsBaseline': 'Finish vs baseline',
  'cpm.help':
    "ES/EF/LS/LF computed server-side via internal/kernel CPM. Any node with Float = 0 lies on the critical path and is highlighted red. Real start/finish dates appear once the project has a start date (Project Settings); non-working days are skipped using the project country's calendar. Incoming-link labels set the dependency type and lag (FS/SS/FF/SF, e.g. SS+2); blank means FS. Date constraints (SNET/FNLT/MFO) also need the project start date; ALAP works without one. Links always win over constraints; impossible constraints flag the node amber instead. Resource assignments default to capacity 1.0 per resource; tasks whose resources are over capacity get an orange edge strip.",
  'cpm.evm.title': 'Earned value',
  'cpm.evm.statusDate': 'Status date (blank = today)',
  'cpm.evm.compute': 'Compute',
  'cpm.evm.computing': 'Computing…',
  'cpm.evm.bac': 'BAC',
  'cpm.evm.pv': 'PV',
  'cpm.evm.ev': 'EV',
  'cpm.evm.ac': 'AC',
  'cpm.evm.sv': 'SV',
  'cpm.evm.cv': 'CV',
  'cpm.evm.spi': 'SPI',
  'cpm.evm.cpi': 'CPI',
  'cpm.evm.eac': 'EAC',
  'cpm.evm.etc': 'ETC',
  'cpm.evm.vac': 'VAC',
  'cpm.evm.help':
    'PV from the schedule, EV from % complete × budgeted cost, AC from actual cost. SPI/CPI below 1 means behind schedule / over cost; n/a until work is planned or cost incurred.',
  'cpm.risk.title': 'Monte Carlo risk',
  'cpm.risk.iterations': 'Iterations',
  'cpm.risk.iterations.hint': 'Simulation iterations',
  'cpm.risk.run': 'Run',
  'cpm.risk.running': 'Running…',
  'cpm.risk.p50': 'P50',
  'cpm.risk.p80': 'P80',
  'cpm.risk.p90': 'P90',
  'cpm.risk.finishProbability': 'Finish probability',
  'cpm.risk.sCurve': 'S-curve',
  'cpm.risk.sCurve.label': 'Monte Carlo finish probability S-curve from {from} to {to}',
  'cpm.risk.tornado': 'Tornado drivers',
  'cpm.risk.runs': { one: '{count} run', other: '{count} runs' },
  'cpm.risk.noDrivers': 'No variable risk drivers detected.',
  'cpm.risk.export': 'Export PDF/A',
  'cpm.risk.exporting': 'Exporting…',
  'cpm.risk.report': 'Risk report',
  'cpm.risk.help':
    'P50/P80/P90 are finish-day confidence points. Tornado drivers rank critical-path frequency multiplied by P90-P50 duration spread.',
  'cpm.risk.empty':
    'Add optional task estimates, then run a simulation. Tasks without estimates use their fixed duration.',
  'projectPicker.restore.done':
    'Backup restored. Signing is switched off in a restored project. Set it up again in Project Settings if you need it.',
} as const satisfies Record<string, Message>;

export type MessageKey = keyof typeof en;
