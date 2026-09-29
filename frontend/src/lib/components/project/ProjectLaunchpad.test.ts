// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, waitFor, within } from '@testing-library/svelte';

import ProjectLaunchpad from './ProjectLaunchpad.svelte';

const policies = [
  {
    country_code: 'US',
    name: 'United States',
    time_zones: ['America/New_York', 'America/Chicago'],
  },
  {
    country_code: 'JP',
    name: 'Japan',
    time_zones: ['Asia/Tokyo'],
  },
];

let app: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  app = {
    ListCalendarPolicies: vi.fn(async () => policies),
    LaunchpadEvaluate: vi.fn(async () => ['charter']),
    CreateProjectFromLaunchpad: vi.fn(async () => ({
      project: { id: 'project-1', name: 'Tokyo Delivery' },
      seeds: [],
      path: '/tmp/Tokyo Delivery.gopmgr',
    })),
    RecoveryCodeStatus: vi.fn(async () => ({ unused: 8, total: 8, legacy: false, encryption_ready: true })),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
});

async function reachSetup(onCreated = vi.fn()) {
  const utils = render(ProjectLaunchpad, {
    props: { onCreated, onCancel: vi.fn() },
  });

  await fireEvent.click(utils.getByRole('button', { name: /software/i }));
  expect(utils.getByRole('heading', { name: /what kind of project/i })).toBeInTheDocument();
  await fireEvent.click(utils.getByRole('button', { name: /continue/i }));

  await fireEvent.click(utils.getByRole('button', { name: /^web dev$/i }));
  await fireEvent.click(utils.getByRole('button', { name: /continue/i }));

  await fireEvent.click(utils.getByRole('button', { name: /^scrum time-boxed/i }));
  await fireEvent.click(utils.getByRole('button', { name: /continue/i }));

  await waitFor(() => expect(app.LaunchpadEvaluate).toHaveBeenCalledWith('software', 'scrum'));
  return utils;
}

describe('project launchpad', () => {
  it('labels progress and requires explicit confirmation before advancing', async () => {
    const { getByRole } = render(ProjectLaunchpad, {
      props: { onCreated: vi.fn(), onCancel: vi.fn() },
    });

    const progress = getByRole('list', { name: /project creation progress/i });
    expect(progress).toHaveTextContent('Industry');
    expect(progress).toHaveTextContent('Focus');
    expect(progress).toHaveTextContent('Method');
    expect(progress).toHaveTextContent('Setup');

    await fireEvent.click(getByRole('button', { name: /software/i }));
    expect(getByRole('heading', { name: /what kind of project/i })).toBeInTheDocument();
    await fireEvent.click(getByRole('button', { name: /continue/i }));
    expect(getByRole('heading', { name: /narrow it down/i })).toBeInTheDocument();
  });

  it('uses backend policies and sends the selected time zone during creation', async () => {
    const onCreated = vi.fn();
    const utils = await reachSetup(onCreated);

    await waitFor(() => expect(app.ListCalendarPolicies).toHaveBeenCalledOnce());
    await fireEvent.change(utils.getByLabelText(/business calendar policy/i), {
      target: { value: 'JP' },
    });
    expect(utils.getByLabelText(/schedule and chart time zone/i)).toHaveValue('Asia/Tokyo');

    await fireEvent.input(utils.getByLabelText(/project name/i), {
      target: { value: 'Tokyo Delivery' },
    });
    await fireEvent.click(utils.getByRole('button', { name: /create project/i }));

    await waitFor(() =>
      expect(app.CreateProjectFromLaunchpad).toHaveBeenCalledWith(
        'Tokyo Delivery',
        '',
        'software',
        'Web Dev',
        'scrum',
        'JP',
        'Asia/Tokyo',
        ['charter'],
      ),
    );
    expect(onCreated).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'project-1', name: 'Tokyo Delivery' }),
      '/tmp/Tokyo Delivery.gopmgr',
    );
  });
});

// Added 2026-08-19 alongside migrating this wizard's three nav-style buttons
// ("Cancel" and two step-gated "← Back" buttons) from raw <button>s to the
// new `nav` Button variant. The two "← Back" buttons are NOT both reachable
// from one mount: the first is scoped to `{#if step === 4}`, the second to
// `{#if step < 4 && step > 1}` — they're mutually exclusive by step, so this
// covers each from the step where it actually renders, rather than assuming
// a single mount reaches all three (an initial-plan error caught by Gate A:
// a test that only ever reaches the step-4 "Cancel"/"Back" pair would leave
// the step-2/3 "Back" button's migration unverified).
const navExpected = 'text-xs text-slate-400 hover:text-cyan-400 disabled:opacity-50'.split(/\s+/).sort();

describe('ProjectLaunchpad migrated nav buttons', () => {
  it('"Cancel" (step 1, header): Button variant="nav"', () => {
    const { getByRole } = render(ProjectLaunchpad, {
      props: { onCreated: vi.fn(), onCancel: vi.fn() },
    });

    const btn = getByRole('button', { name: 'Cancel' });
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(navExpected);
  });

  it('"← Back" (step 2, step < 4 && step > 1 block): Button variant="nav"', async () => {
    const { getByRole } = render(ProjectLaunchpad, {
      props: { onCreated: vi.fn(), onCancel: vi.fn() },
    });

    await fireEvent.click(getByRole('button', { name: /software/i }));
    await fireEvent.click(getByRole('button', { name: /continue/i }));

    const btn = getByRole('button', { name: /back/i });
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(navExpected);
  });

  it('"← Back" (step 4, step === 4 block): Button variant="nav"', async () => {
    const utils = await reachSetup();

    const btn = utils.getByRole('button', { name: /back/i });
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(navExpected);
  });
});

describe('project launchpad recovery-code gate', () => {
  it('asks for a way back in before creating, and creates once the user accepts', async () => {
    let accepted = false;
    Object.assign(app, {
      RecoveryCodeStatus: vi.fn(async () => ({ unused: 0, total: 8, legacy: false, encryption_ready: accepted })),
      AcceptEncryptionWithoutRecoveryCodes: vi.fn(async () => {
        accepted = true;
      }),
    });
    const onCreated = vi.fn();
    const utils = await reachSetup(onCreated);
    await fireEvent.input(utils.getByLabelText(/project name/i), { target: { value: 'Tokyo Delivery' } });
    await fireEvent.click(utils.getByRole('button', { name: /create project/i }));

    const gate = await utils.findByRole('region', { name: /save a way back in/i });
    expect(app.CreateProjectFromLaunchpad).not.toHaveBeenCalled();
    await fireEvent.click(within(gate).getByLabelText(/projects I create or encrypt until I sign out/));
    await fireEvent.click(within(gate).getByRole('button', { name: 'Continue without recovery codes' }));

    await waitFor(() => expect(app.CreateProjectFromLaunchpad).toHaveBeenCalledOnce());
    await waitFor(() => expect(onCreated).toHaveBeenCalledOnce());
  });
});

describe('ProjectLaunchpad blank project', () => {
  async function openBlank(onCreated = vi.fn()) {
    app.CreateProject = vi.fn(async () => ({ path: '/p/Blank.gopmgr', name: 'Blank', modified: '' }));
    app.OpenProject = vi.fn(async () => ({ id: 'project-9', name: 'Blank' }));
    const utils = render(ProjectLaunchpad, { props: { onCreated, onCancel: vi.fn() } });
    await fireEvent.click(utils.getByRole('button', { name: 'Start with a blank project' }));
    return { utils, onCreated };
  }

  it('creates and opens a blank project from a name alone', async () => {
    const { utils, onCreated } = await openBlank();
    const create = utils.getByRole('button', { name: 'Create blank project' });
    expect(create).toBeDisabled();
    await fireEvent.input(utils.getByLabelText('Project name'), { target: { value: '  Blank  ' } });
    await fireEvent.click(create);
    await waitFor(() => expect(onCreated).toHaveBeenCalledWith({ id: 'project-9', name: 'Blank' }, '/p/Blank.gopmgr'));
    expect(app.CreateProject).toHaveBeenCalledWith('Blank', '');
    expect(app.OpenProject).toHaveBeenCalledWith('/p/Blank.gopmgr');
    expect(app.CreateProjectFromLaunchpad).not.toHaveBeenCalled();
  });

  it('asks for a way back in before creating when there are no recovery codes', async () => {
    app.RecoveryCodeStatus = vi.fn(async () => ({ unused: 0, total: 0, legacy: false, encryption_ready: false }));
    const { utils, onCreated } = await openBlank();
    await fireEvent.input(utils.getByLabelText('Project name'), { target: { value: 'Blank' } });
    await fireEvent.click(utils.getByRole('button', { name: 'Create blank project' }));
    expect(await utils.findByRole('heading', { name: /save a way back in/i })).toBeInTheDocument();
    expect(app.CreateProject).not.toHaveBeenCalled();
    expect(onCreated).not.toHaveBeenCalled();
  });

  it('returns to the guided setup', async () => {
    const { utils } = await openBlank();
    await fireEvent.click(utils.getByRole('button', { name: /Back to the guided setup/ }));
    expect(utils.getByRole('heading', { name: 'What kind of project is this?' })).toBeInTheDocument();
  });
});
