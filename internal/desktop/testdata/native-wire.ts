// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import * as App from '../../../frontend/bindings/gopmgr/internal/desktop/app';
import { Call, Events } from '../../../frontend/node_modules/@wailsio/runtime';

function require(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

async function exercise() {
  const initial = await App.AccountSetup();
  require(initial.has_accounts === false && initial.has_admin === false, 'fresh account booleans');
  require(await App.CurrentUser() === null, 'nil account must remain null');
  try {
    await App.GetProjectMeta();
    throw new Error('missing project unexpectedly succeeded');
  } catch (error) {
    require(error instanceof Error && error.message.includes('no project open'), 'returned domain error');
  }
  const account = await App.CreateAccount('nativewire', 'Native bridge fixture', 'correct horse battery staple', false);
  require(account.username === 'nativewire' && account.is_admin === true, 'account fields and false input');
  await App.Logout();
  require(await App.CurrentUser() === null, 'logout null');
  await App.Login('nativewire', 'correct horse battery staple');
  await App.AcceptEncryptionWithoutRecoveryCodes();
  const file = await App.CreateProject('Native Wire', 'disposable qualification');
  const project = await App.OpenProject(file.path);
  require(project.budget === '0.00', 'explicit zero money');
  const money = '90071992547409.93';
  const saved = await App.UpdateProjectMeta({ ...project, budget: money });
  require(saved.budget === money, 'exact money beyond JS safe minor-unit integer');
  const read = await App.GetProjectMeta();
  require(read.budget === money && read.id === saved.id, 'persisted exact money and ID');
  require(typeof read.created_at === 'string' && !Number.isNaN(Date.parse(read.created_at)), 'timestamp string');
  const stakeholders = await App.ListStakeholders('');
  require(stakeholders === null, 'nil stakeholder list must remain null');
  const notices = await App.MyDataAccessNotices();
  require(Array.isArray(notices) && notices.length === 0, 'explicit empty list');
  await App.CloseProject();
  const reopened = await App.OpenProject(file.path);
  require(reopened.budget === money, 'encrypted project reopen');
  const values = await Call.ByName('gopmgr/internal/desktop.nativeWireFixtures.Values');
  require(values.zero === 0 && values.safe_integer === 9007199254740991, 'zero and maximum safe integer');
  require(values.large_integer === '9223372036854775807', 'text integer boundary');
  require(values.bytes === 'AAH+/w==', 'byte encoding');
  require(values.timestamp === '2026-11-03T12:34:56.123456789Z', 'timestamp precision');
  require(values.nil_list === null && values.nil_map === null, 'nil collections');
  require(Array.isArray(values.empty_list) && values.empty_list.length === 0, 'empty collection');
  require(Object.keys(values.empty_map).length === 0, 'empty map');
  const multiple = await Call.ByName('gopmgr/internal/desktop.nativeWireFixtures.Multiple');
  require(Array.isArray(multiple) && multiple[0] === 'literal' && multiple[1] === 0, 'multiple return values');
  await App.EnableNativeCloseGuard();
  if (new URLSearchParams(window.location.search).has('about')) {
    await Call.ByName('gopmgr/internal/desktop.nativeWireFixtures.ShowAbout');
    require(await Call.ByName('gopmgr/internal/desktop.nativeWireFixtures.NativeUIBusy'), 'About released its lease before dismissal');
    try {
      await App.CompleteNativeClose();
      throw new Error('quit accepted an active About dialog');
    } catch (error) {
      require(error instanceof Error && error.message.includes('native dialog'), 'busy quit refusal');
    }
    await Call.ByName('gopmgr/internal/desktop.nativeWireFixtures.SessionUsable');
    await Events.Emit('qualification:waiting-about');
    const deadline = Date.now() + 10000;
    while (await Call.ByName('gopmgr/internal/desktop.nativeWireFixtures.NativeUIBusy')) {
      require(Date.now() < deadline, 'About dismissal did not release its lease');
      await new Promise((resolve) => setTimeout(resolve, 20));
    }
    await Call.ByName('gopmgr/internal/desktop.nativeWireFixtures.SessionUsable');
  }
  const fileMode = new URLSearchParams(window.location.search).get('file-select');
  if (fileMode !== null) {
    require(fileMode === 'open' || fileMode === 'save', 'file selection mode');
    const root = await Call.ByName('gopmgr/internal/desktop.nativeWireFixtures.FileSelectionRoot');
    const pending = Call.ByName('gopmgr/internal/desktop.nativeWireFixtures.SelectFile', fileMode);
    const deadline = Date.now() + 10000;
    while (!await Call.ByName('gopmgr/internal/desktop.nativeWireFixtures.NativeUIBusy')) {
      require(Date.now() < deadline, 'file selection did not reserve native UI');
      await new Promise((resolve) => setTimeout(resolve, 20));
    }
    try {
      await App.CompleteNativeClose();
      throw new Error('quit accepted an active file dialog');
    } catch (error) {
      require(error instanceof Error && error.message.includes('native dialog'), 'file dialog busy quit refusal');
    }
    await Call.ByName('gopmgr/internal/desktop.nativeWireFixtures.SessionUsable');
    await Call.ByName('gopmgr/internal/desktop.nativeWireFixtures.FileSelectionReady');
    const selected = await pending;
    const literal = fileMode === 'open' ? "資料 input 'quoted'.txt" : "報告 output 'quoted'.txt";
    require(selected === `${root}/owned folder '資料'/${literal}`, 'literal native selected path');
    require(!await Call.ByName('gopmgr/internal/desktop.nativeWireFixtures.NativeUIBusy'), 'file selection retained native UI lease');
    require(await Call.ByName('gopmgr/internal/desktop.nativeWireFixtures.ConfirmFileSelection', selected) === 1, 'single selection RPC dispatch');
    await Call.ByName('gopmgr/internal/desktop.nativeWireFixtures.SessionUsable');
  }
  await Events.Emit('qualification:passed', file.path);
  await App.CompleteNativeClose();
}

exercise().catch((error) => Events.Emit('qualification:failed', String(error)));
