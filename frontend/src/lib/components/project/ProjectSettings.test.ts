// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, waitFor } from '@testing-library/svelte';

import { session } from '../../session.svelte';
import ProjectSettings from './ProjectSettings.svelte';

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
    const { findByRole, getByLabelText } = render(ProjectSettings);

    expect(
      await findByRole('checkbox', { name: /add an RFC 3161 timestamp/i }),
    ).toBeChecked();
    expect(getByLabelText(/timestamp authority HTTPS endpoint/i)).toHaveValue(
      'https://tsa.example.test/timestamp',
    );
    expect(getByLabelText(/TSA policy OID/i)).toHaveValue('1.3.6.1.4.1.55555.7');
  });

  it('never saves timestamping as enabled for a non-PAdES method', async () => {
    const { findByLabelText, getByRole } = render(ProjectSettings);
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
