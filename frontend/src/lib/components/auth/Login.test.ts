// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';

import Login from './Login.svelte';
import { session } from '../../session.svelte';

const account = {
  username: 'alice',
  display_name: 'Alice',
  data_dir: '/tmp/gopmgr-test/alice',
  created_at: '',
  last_login: '',
  is_admin: false,
};

function deferred<T>() {
  let resolve: (value: T) => void;
  let reject: (reason?: unknown) => void;
  const promise = new Promise<T>((done, fail) => { resolve = done; reject = fail; });
  return { promise, resolve: resolve!, reject: reject! };
}

function installApp(overrides: Record<string, ReturnType<typeof vi.fn>> = {}) {
  const app = {
    AccountSetup: vi.fn(async () => ({ has_accounts: true, has_admin: true })),
    Login: vi.fn(async () => account),
    ...overrides,
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
  return app;
}

async function fillCredentials(utils: ReturnType<typeof render>) {
  await fireEvent.input(utils.getByLabelText('Username'), { target: { value: 'alice' } });
  await fireEvent.input(utils.getByLabelText('Password'), { target: { value: 'correct horse battery staple' } });
}

afterEach(() => {
  cleanup();
  session.user = null;
  session.view = 'login';
  vi.restoreAllMocks();
});

describe('Login', () => {
  it('submits the exact credentials once, stores the returned account, and opens the portfolio', async () => {
    const app = installApp();
    const utils = render(Login);
    await fillCredentials(utils);

    await fireEvent.submit(utils.container.querySelector('form')!);

    await waitFor(() => expect(app.Login).toHaveBeenCalledWith('alice', 'correct horse battery staple'));
    expect(app.Login).toHaveBeenCalledTimes(1);
    expect(session.user).toEqual(account);
    expect(session.view).toBe('portfolio');
  });

  it('accepts only one submission while authentication is pending', async () => {
    const pendingLogin = deferred<typeof account>();
    const app = installApp({ Login: vi.fn(() => pendingLogin.promise) });
    const utils = render(Login);
    await fillCredentials(utils);
    const form = utils.container.querySelector('form')!;

    await fireEvent.submit(form);
    await fireEvent.submit(form);

    expect(app.Login).toHaveBeenCalledTimes(1);
    expect(utils.getByRole('button', { name: 'Signing in…' })).toBeDisabled();

    pendingLogin.resolve(account);
    await waitFor(() => expect(session.view).toBe('portfolio'));
    expect(session.user).toEqual(account);
  });

  it('keeps the session on Login and exposes no backend details after authentication fails', async () => {
    const app = installApp({ Login: vi.fn(async () => { throw new Error('database password verification failed'); }) });
    const utils = render(Login);
    await fillCredentials(utils);

    await fireEvent.submit(utils.container.querySelector('form')!);

    expect(await utils.findByRole('alert')).toHaveTextContent('Invalid username or password.');
    expect(utils.queryByText('database password verification failed')).not.toBeInTheDocument();
    expect(session.user).toBeNull();
    expect(session.view).toBe('login');
    expect(utils.getByRole('button', { name: 'SIGN IN' })).toBeEnabled();
    expect(app.Login).toHaveBeenCalledTimes(1);
  });

  it('tells a disabled account why it cannot sign in, and keeps every other failure generic', async () => {
    const app = installApp({
      Login: vi.fn()
        .mockRejectedValueOnce(new Error('this account is disabled; ask your administrator to enable it'))
        .mockRejectedValueOnce(new Error('users: update last_login: database is locked')),
    });
    const utils = render(Login);
    await fillCredentials(utils);
    await fireEvent.submit(utils.container.querySelector('form')!);
    expect(await utils.findByRole('alert')).toHaveTextContent('This account is disabled. Ask your administrator to enable it.');

    await fireEvent.submit(utils.container.querySelector('form')!);
    await waitFor(() => expect(app.Login).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(utils.getByRole('alert')).toHaveTextContent('Invalid username or password.'));
    expect(session.user).toBeNull();
  });

  it('fails open to account creation only after the setup check fails', async () => {
    const setupCheck = deferred<{ has_accounts: boolean; has_admin: boolean }>();
    const app = installApp({ AccountSetup: vi.fn(() => setupCheck.promise) });
    const utils = render(Login);

    await waitFor(() => expect(app.AccountSetup).toHaveBeenCalledOnce());
    expect(utils.queryByRole('button', { name: 'Create a new account' })).not.toBeInTheDocument();
    expect(utils.queryByText(/contact your administrator/i)).not.toBeInTheDocument();

    setupCheck.reject(new Error('system database unavailable'));

    expect(await utils.findByRole('button', { name: 'Create a new account' })).toBeInTheDocument();
    expect(utils.queryByText(/contact your administrator/i)).not.toBeInTheDocument();
  });

  it('offers account creation on a machine with no accounts and says the first becomes administrator', async () => {
    installApp({ AccountSetup: vi.fn(async () => ({ has_accounts: false, has_admin: false })) });
    const utils = render(Login);

    expect(await utils.findByRole('button', { name: 'Create a new account' })).toBeInTheDocument();
    expect(utils.getByText(/first account you create becomes this computer's GoPMgr administrator/i)).toBeInTheDocument();
    expect(utils.queryByText(/contact your administrator/i)).not.toBeInTheDocument();
  });

  it('withholds account creation when accounts exist but none is an administrator', async () => {
    installApp({ AccountSetup: vi.fn(async () => ({ has_accounts: true, has_admin: false })) });
    const utils = render(Login);

    expect(await utils.findByText(/Sign in, then use Become administrator in App Settings/)).toBeInTheDocument();
    expect(utils.queryByRole('button', { name: 'Create a new account' })).not.toBeInTheDocument();
    expect(utils.queryByText(/contact your administrator/i)).not.toBeInTheDocument();
  });

  it('points to the administrator once one exists', async () => {
    installApp();
    const utils = render(Login);

    expect(await utils.findByText(/contact your administrator/i)).toBeInTheDocument();
    expect(utils.queryByRole('button', { name: 'Create a new account' })).not.toBeInTheDocument();
    expect(utils.queryByText(/No accounts yet|No administrator is configured/)).not.toBeInTheDocument();
  });
});
