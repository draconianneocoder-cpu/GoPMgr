// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, waitFor } from '@testing-library/svelte';

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
