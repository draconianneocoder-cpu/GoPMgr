// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, waitFor, within } from '@testing-library/svelte';

import { session } from '../../session.svelte';
import ProjectSettings from './ProjectSettings.svelte';

async function switchTab(
  container: HTMLElement,
  name: RegExp,
): Promise<void> {
  const tab = within(container).getByRole('tab', { name });
  await fireEvent.click(tab);
}

const timestampSettings = {
  default_password: '',
  export_theme: 'modern',
  auto_repair: true,
  cert_path: '/tmp/signer.p12',
  signature_enabled: true,
  signature_method: 'pades',
  gpg_key_id: '',
  timestamp_enabled: true,
  tsa_endpoint: 'https://tsa.example.test/timestamp',
  tsa_policy_oid: '1.3.6.1.4.1.55555.7',
  tsa_root_cert_path: '/tmp/tsa-root.pem',
  default_font: '',
  agile_enabled: false,
  compliance_mode: false,
};

let app: Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
  app = {
    ListCalendarPolicies: vi.fn(async () => []),
    GetProjectMeta: vi.fn(async () => ({
      id: 'project-1',
      name: 'Timestamped Project',
      description: '',
      industry: '',
      sub_category: '',
      methodology: '',
      country_code: 'US',
      time_zone: 'America/Chicago',
      status: 'planning',
      phase: 'planning',
    })),
    GetSettings: vi.fn(async () => ({ ...timestampSettings })),
    ListFonts: vi.fn(async () => []),
    GetDefaultFont: vi.fn(async () => ''),
    ListResourceCalendars: vi.fn(async () => []),
    ListCharts: vi.fn(async () => []),
    ListScheduleBaselines: vi.fn(async () => []),
    ListScenarios: vi.fn(async () => []),
    SaveSettings: vi.fn(async () => undefined),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
  session.projectPath = null;
  session.project = null;
});

describe('project PAdES timestamp settings', () => {
  it('loads the opt-in endpoint, policy, and trust-root configuration', async () => {
    const { container, findByRole, getByLabelText } = render(ProjectSettings);
    await findByRole('tablist');
    await switchTab(container, /exports & signing/i);

    expect(
      await findByRole('checkbox', { name: /add an RFC 3161 timestamp/i }),
    ).toBeChecked();
    expect(getByLabelText(/timestamp authority HTTPS endpoint/i)).toHaveValue(
      'https://tsa.example.test/timestamp',
    );
    expect(getByLabelText(/TSA policy OID/i)).toHaveValue('1.3.6.1.4.1.55555.7');
  });

  it('never saves timestamping as enabled for a non-PAdES method', async () => {
    const { container, findByLabelText, findByRole, getByRole } = render(ProjectSettings);
    await findByRole('tablist');
    await switchTab(container, /exports & signing/i);
    const method = await findByLabelText(/document signing method/i);

    await fireEvent.change(method, { target: { value: 'gpg' } });
    await fireEvent.click(getByRole('button', { name: /save export settings/i }));

    await waitFor(() => expect(app.SaveSettings).toHaveBeenCalledOnce());
    expect(app.SaveSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        signature_method: 'gpg',
        timestamp_enabled: false,
        tsa_endpoint: 'https://tsa.example.test/timestamp',
      }),
    );
  });
});

describe('tab wiring', () => {
  it('defaults to the General tab and hides the other tabs’ content', async () => {
    const { findByRole, queryByText } = render(ProjectSettings);
    await findByRole('tab', { name: /general/i });

    expect(await findByRole('textbox', { name: /project name/i })).toBeInTheDocument();
    expect(queryByText(/what-if scenarios/i)).not.toBeInTheDocument();
    expect(queryByText(/resource capacity/i)).not.toBeInTheDocument();
    expect(queryByText(/schedule reports/i)).not.toBeInTheDocument();
    expect(queryByText(/project backup/i)).not.toBeInTheDocument();
  });

  it('reveals each tab’s own content on switch and hides General’s', async () => {
    const { container, findByRole, findByText, queryByLabelText } = render(ProjectSettings);
    await findByRole('tab', { name: /general/i });

    await switchTab(container, /^scenarios$/i);
    expect(await findByText('What-if Scenarios')).toBeInTheDocument();
    expect(queryByLabelText(/project name/i)).not.toBeInTheDocument();

    await switchTab(container, /^resources$/i);
    expect(await findByText('Resource Capacity')).toBeInTheDocument();

    await switchTab(container, /exports & signing/i);
    expect(await findByText('Schedule Reports (CPM)')).toBeInTheDocument();
    expect(await findByText('Export & Signature Settings')).toBeInTheDocument();
    expect(await findByText('Document Font')).toBeInTheDocument();

    await switchTab(container, /data protection/i);
    expect(await findByText('Project Backup')).toBeInTheDocument();
    expect(await findByText('Database Encryption')).toBeInTheDocument();
  });

  it('preserves a Scenarios-tab draft across a tab switch away and back', async () => {
    const { container, findByLabelText, findByRole } = render(ProjectSettings);
    await findByRole('tab', { name: /general/i });

    await switchTab(container, /^scenarios$/i);
    const scenarioName = await findByLabelText(/scenario name/i);
    await fireEvent.input(scenarioName, { target: { value: 'Aggressive timeline' } });

    await switchTab(container, /^resources$/i);
    await switchTab(container, /^scenarios$/i);

    expect(await findByLabelText(/scenario name/i)).toHaveValue('Aggressive timeline');
  });
});

describe('methodology field', () => {
  it('renders an out-of-list current value as its own selectable option', async () => {
    app.GetProjectMeta = vi.fn(async () => ({
      id: 'project-1',
      name: 'Legacy Project',
      description: '',
      industry: 'software',
      sub_category: '',
      methodology: 'xp',
      country_code: 'US',
      time_zone: 'America/Chicago',
      status: 'planning',
      phase: 'planning',
    }));
    const { findByLabelText } = render(ProjectSettings);

    const methodology = (await findByLabelText(/methodology/i)) as HTMLSelectElement;
    expect(methodology).toHaveValue('xp');
    expect(within(methodology).getByText('xp (current)')).toBeInTheDocument();
  });

  it('clears methodology when Industry changes', async () => {
    app.GetProjectMeta = vi.fn(async () => ({
      id: 'project-1',
      name: 'Software Project',
      description: '',
      industry: 'software',
      sub_category: '',
      methodology: 'scrum',
      country_code: 'US',
      time_zone: 'America/Chicago',
      status: 'planning',
      phase: 'planning',
    }));
    const { findByLabelText } = render(ProjectSettings);

    const industry = await findByLabelText(/^industry$/i);
    const methodology = (await findByLabelText(/methodology/i)) as HTMLSelectElement;
    expect(methodology).toHaveValue('scrum');

    await fireEvent.change(industry, { target: { value: 'construction' } });

    expect(methodology).toHaveValue('');
  });

  it('reverting after an Industry change restores both industry and methodology as a selectable value', async () => {
    app.GetProjectMeta = vi.fn(async () => ({
      id: 'project-1',
      name: 'Software Project',
      description: '',
      industry: 'software',
      sub_category: '',
      methodology: 'scrum',
      country_code: 'US',
      time_zone: 'America/Chicago',
      status: 'planning',
      phase: 'planning',
    }));
    const { findByLabelText, findByRole } = render(ProjectSettings);

    const industry = (await findByLabelText(/^industry$/i)) as HTMLSelectElement;
    const methodology = (await findByLabelText(/methodology/i)) as HTMLSelectElement;
    await fireEvent.change(industry, { target: { value: 'construction' } });
    expect(methodology).toHaveValue('');

    const revertButton = await findByRole('button', { name: /revert details/i });
    await fireEvent.click(revertButton);

    expect(industry).toHaveValue('software');
    expect(methodology).toHaveValue('scrum');
    expect(within(methodology).getByRole('option', { name: 'Scrum' }).selected).toBe(true);
  });

  it('does not read as dirty on load when methodology is missing from the loaded project', async () => {
    app.GetProjectMeta = vi.fn(async () => ({
      id: 'project-1',
      name: 'Untagged Project',
      description: '',
      industry: 'software',
      sub_category: '',
      // methodology intentionally omitted, unlike a real db.Project JSON
      // payload (internal/db/project.go has no `omitempty` on this field)
      // -- this proves ProjectSettings.svelte's own onMount coercion
      // guards it regardless of what the caller sends.
      country_code: 'US',
      time_zone: 'America/Chicago',
      status: 'planning',
      phase: 'planning',
    }));
    const { findByRole } = render(ProjectSettings);

    await findByRole('tab', { name: /general/i });
    expect(await findByRole('button', { name: /^revert details$/i })).toBeDisabled();
    expect(await findByRole('button', { name: /^save details$/i })).toBeDisabled();
  });
});
