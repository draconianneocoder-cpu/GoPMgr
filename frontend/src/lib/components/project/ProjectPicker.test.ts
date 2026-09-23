// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { cleanup, fireEvent, render, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';

import ProjectPicker from './ProjectPicker.svelte';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

function deletion(overrides: Partial<ProjectDeletion>): ProjectDeletion {
  return {
    id: 'deletion_1',
    project_id: 'project_1',
    project_name: 'Bridge Retrofit',
    location: '20260922-150405-Bridge Retrofit/project.gopmgr',
    deleted_by: 'alice',
    requested_at: '2026-09-22T15:04:05Z',
    outcome: 'deleted',
    outcome_at: '2026-09-22T15:04:06Z',
    detail: '',
    audit_events: 7,
    audit_valid: true,
    audit_terminal_hash: '0123456789abcdef0123456789abcdef',
    ...overrides,
  };
}

function installApp(opts: { projects?: ProjectFile[]; deletions?: () => Promise<ProjectDeletion[]> }) {
  const app = {
    ListProjects: vi.fn(async () => opts.projects ?? []),
    ListProjectDeletions: vi.fn(opts.deletions ?? (async () => [])),
    DeleteProject: vi.fn(async () => {}),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
  return app;
}

describe('ProjectPicker deleted-projects history', () => {
  it('describes every outcome and audit state in the deletion log', async () => {
    installApp({
      deletions: async () => [
        deletion({ id: 'd1', project_name: 'Verified' }),
        deletion({ id: 'd2', project_name: 'Tampered', audit_valid: false }),
        deletion({ id: 'd3', project_name: 'Empty', project_id: '', audit_events: 0, audit_terminal_hash: '' }),
        deletion({ id: 'd4', project_name: 'Locked', outcome: 'failed', detail: 'permission denied' }),
        deletion({ id: 'd5', project_name: 'Interrupted', outcome: '', outcome_at: '' }),
        deletion({ id: 'd6', project_name: 'Pre-audit', audit_events: 0, audit_terminal_hash: '' }),
        deletion({ id: 'd7', project_name: 'Tampered early', audit_valid: false, audit_events: 0, audit_terminal_hash: '' }),
      ],
    });
    const { findByText, getByRole } = render(ProjectPicker);

    expect(await findByText('Deleted projects (7)')).toBeInTheDocument();
    const list = within(getByRole('list', { name: 'Deleted projects' }));
    const item = (name: string) => list.getByText(name).closest('li') as HTMLElement;

    expect(item('Verified')).toHaveTextContent('Deleted');
    expect(item('Verified')).toHaveTextContent('by alice');
    expect(item('Verified')).toHaveTextContent('Audit trail verified: 7 events, final hash 0123456789ab…');
    expect(item('Tampered')).toHaveTextContent('Audit trail failed verification before deletion.');
    expect(item('Empty')).toHaveTextContent('Empty file with no project data.');
    expect(item('Locked')).toHaveTextContent('Deletion failed');
    expect(item('Locked')).toHaveTextContent('permission denied. Some files may already be gone.');
    expect(item('Interrupted')).toHaveTextContent('the result was not recorded.');
    // A valid chain with no events predates the audit trail; it is not "verified".
    expect(item('Pre-audit')).toHaveTextContent('No audit trail was recorded for this project.');
    expect(item('Pre-audit')).not.toHaveTextContent('verified');
    // An invalid chain must always read as failed, whatever its count.
    expect(item('Tampered early')).toHaveTextContent('Audit trail failed verification before deletion.');
  });

  it('stays hidden while nothing has been deleted', async () => {
    const app = installApp({ projects: [{ path: '/p/a.gopmgr', name: 'Alpha', modified: '' }] });
    const { findByText, queryByText } = render(ProjectPicker);

    expect(await findByText('Alpha')).toBeInTheDocument();
    expect(app.ListProjectDeletions).toHaveBeenCalled();
    expect(queryByText(/Deleted projects/)).toBeNull();
  });

  it('reports a failed history load without hiding the project list', async () => {
    installApp({
      projects: [{ path: '/p/a.gopmgr', name: 'Alpha', modified: '' }],
      deletions: async () => {
        throw new Error('open project deletion log: locked');
      },
    });
    const { findByText, getByRole } = render(ProjectPicker);

    expect(await findByText('Alpha')).toBeInTheDocument();
    expect(await findByText(/Could not load deleted projects: .*locked/)).toBeInTheDocument();
    expect(getByRole('alert')).toHaveTextContent('Could not load deleted projects');
  });

  it('shows a new entry right after a project is deleted', async () => {
    let history: ProjectDeletion[] = [];
    const app = installApp({
      projects: [{ path: '/p/a.gopmgr', name: 'Alpha', modified: '' }],
      deletions: async () => history,
    });
    app.DeleteProject.mockImplementation(async () => {
      history = [deletion({ id: 'd9', project_name: 'Alpha' })];
      app.ListProjects.mockResolvedValue([]);
    });
    const { findByText, findByRole, getByRole } = render(ProjectPicker);

    await fireEvent.click(await findByRole('button', { name: 'Delete Alpha' }));
    await fireEvent.click(getByRole('button', { name: 'Confirm delete Alpha' }));

    expect(await findByText('Deleted projects (1)')).toBeInTheDocument();
    expect(app.DeleteProject).toHaveBeenCalledWith('/p/a.gopmgr');
  });
});
