// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';

import CreateAccount from './CreateAccount.svelte';
import { session } from '../../session.svelte';

const account = {
  username: 'alice',
  display_name: 'alice',
  data_dir: '/tmp/gopmgr-test/alice',
  created_at: '',
  last_login: '',
  is_admin: true,
};

function installApp(overrides: Record<string, ReturnType<typeof vi.fn>> = {}) {
  const app = {
    AccountSetup: vi.fn(async () => ({ has_accounts: false, has_admin: false })),
    CreateAccount: vi.fn(async () => account),
    IssueRecoveryCodes: vi.fn(async () => ['AAAAAAAA-BBBBBBBB']),
    ...overrides,
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
  return app;
}

async function fillForm(utils: ReturnType<typeof render>) {
  await fireEvent.input(utils.getByLabelText('Username'), { target: { value: 'alice' } });
  await fireEvent.input(utils.getByLabelText('Password'), { target: { value: 'correct horse battery' } });
  await fireEvent.input(utils.getByLabelText('Confirm password'), { target: { value: 'correct horse battery' } });
}

afterEach(() => {
  cleanup();
  session.user = null;
  session.view = 'login';
  vi.restoreAllMocks();
});

describe('CreateAccount', () => {
  it('tells the first user their account becomes the administrator, with no role choice to make', async () => {
    installApp();
    const utils = render(CreateAccount);

    expect(await utils.findByText(/This account will be the GoPMgr administrator/)).toBeInTheDocument();
    expect(utils.queryByRole('checkbox')).not.toBeInTheDocument();
  });

  it('shows no first-user notice when accounts already exist or the check fails', async () => {
    for (const setup of [
      vi.fn(async () => ({ has_accounts: true, has_admin: true })),
      vi.fn(async () => { throw new Error('system database unavailable'); }),
    ]) {
      const app = installApp({ AccountSetup: setup });
      const utils = render(CreateAccount);
      await waitFor(() => expect(app.AccountSetup).toHaveBeenCalledOnce());
      expect(utils.queryByText(/This account will be the GoPMgr administrator/)).not.toBeInTheDocument();
      cleanup();
    }
  });

  it('leaves the role to the backend and signs the new account in', async () => {
    const app = installApp();
    const utils = render(CreateAccount);
    await fillForm(utils);
    await fireEvent.submit(utils.container.querySelector('form')!);

    await waitFor(() => expect(app.CreateAccount).toHaveBeenCalledOnce());
    expect(app.CreateAccount).toHaveBeenCalledWith('alice', 'alice', 'correct horse battery', false);
    expect(session.user).toEqual(account);
    expect(await utils.findByText('Save your recovery codes')).toBeInTheDocument();
  });

  it('points to App Settings when recovery codes could not be made', async () => {
    installApp({ IssueRecoveryCodes: vi.fn(async () => { throw new Error('entropy unavailable'); }) });
    const utils = render(CreateAccount);
    await fillForm(utils);
    await fireEvent.submit(utils.container.querySelector('form')!);

    expect(await utils.findByRole('alert')).toHaveTextContent(/Create your recovery codes in\s+App Settings, under Account/);
    expect(utils.getByRole('alert')).not.toHaveTextContent('Project Settings');
  });

  it('explains a refused creation in plain language', async () => {
    installApp({
      AccountSetup: vi.fn(async () => ({ has_accounts: true, has_admin: true })),
      CreateAccount: vi.fn(async () => { throw new Error('account creation requires administrator privileges'); }),
    });
    const utils = render(CreateAccount);
    await fillForm(utils);
    await fireEvent.submit(utils.container.querySelector('form')!);

    expect(await utils.findByRole('alert')).toHaveTextContent(
      'This computer already has GoPMgr accounts. Ask your administrator to create yours.',
    );
    expect(session.user).toBeNull();
  });
});
