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
    HasAnyAdmin: vi.fn(async () => true),
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

  it('fails open to account creation only after the administrator check fails', async () => {
    const adminCheck = deferred<boolean>();
    const app = installApp({ HasAnyAdmin: vi.fn(() => adminCheck.promise) });
    const utils = render(Login);

    await waitFor(() => expect(app.HasAnyAdmin).toHaveBeenCalledOnce());
    expect(utils.queryByRole('button', { name: 'Create a new account' })).not.toBeInTheDocument();
    expect(utils.queryByText(/contact your administrator/i)).not.toBeInTheDocument();

    adminCheck.reject(new Error('system database unavailable'));

    expect(await utils.findByRole('button', { name: 'Create a new account' })).toBeInTheDocument();
    expect(utils.queryByText(/contact your administrator/i)).not.toBeInTheDocument();
  });
});
