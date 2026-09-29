// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor, within } from '@testing-library/svelte';

import { autosave } from '../../autosave.svelte';
import { navigation, requestNavigation, saveAndContinueNavigation, session } from '../../session.svelte';
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

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  autosave.discardAll();
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

describe('reporting currency', () => {
  it('defaults a legacy or stale response to USD and saves a supported selection', async () => {
    app.GetProjectMeta = vi.fn(async () => ({
      id: 'project-1',
      name: 'Currency Project',
      description: '',
      industry: '',
      sub_category: '',
      methodology: '',
      country_code: 'US',
      time_zone: 'America/Chicago',
      status: 'planning',
      phase: 'planning',
    }));
    app.UpdateProjectMeta = vi.fn(async (p: unknown) => p);
    app.UpdateProjectIndustry = vi.fn(async () => ({
      id: 'project-1',
      name: 'Currency Project',
      description: '',
      industry: '',
      sub_category: '',
      methodology: '',
      country_code: 'US',
      time_zone: 'America/Chicago',
      status: 'planning',
      phase: 'planning',
      currency_code: 'EUR',
    }));
    const { findByLabelText, findByRole, findByText } = render(ProjectSettings);

    const currency = (await findByLabelText(/reporting currency/i)) as HTMLSelectElement;
    expect(currency).toHaveValue('USD');
    await fireEvent.change(currency, { target: { value: 'EUR' } });
    await fireEvent.click(await findByRole('button', { name: /^save details$/i }));

    await waitFor(() => expect(app.UpdateProjectMeta).toHaveBeenCalledOnce());
    expect(app.UpdateProjectMeta).toHaveBeenCalledWith(expect.objectContaining({ currency_code: 'EUR' }));
    expect(await findByLabelText(/reporting currency/i)).toHaveValue('EUR');
    expect(await findByText(/can change only while this project has no budget value, cost control entry, non-zero reserve balance, or approved cost control baseline/i)).toBeInTheDocument();
  });

	it('retains a legacy JPY selection only for inspection and explains the read-only policy', async () => {
		app.GetProjectMeta = vi.fn(async () => ({
			id: 'project-1', name: 'Legacy JPY Project', description: '', industry: '', sub_category: '', methodology: '',
			country_code: 'JP', time_zone: 'Asia/Tokyo', status: 'planning', phase: 'planning', currency_code: 'JPY',
		}));
		const { findByLabelText, findByText } = render(ProjectSettings);
		const currency = (await findByLabelText(/reporting currency/i)) as HTMLSelectElement;
		expect(currency).toHaveValue('JPY');
		expect(within(currency).getByRole('option', { name: /legacy read-only/i })).toBeInTheDocument();
		expect(await findByText(/cost control changes are disabled; no values were converted/i)).toBeInTheDocument();
	});

  it('restores locked legacy JPY after a rejected currency change', async () => {
    app.GetProjectMeta = vi.fn(async () => ({
      id: 'project-1', name: 'Locked Legacy JPY Project', description: '', industry: '', sub_category: '', methodology: '',
      country_code: 'JP', time_zone: 'Asia/Tokyo', status: 'planning', phase: 'planning', currency_code: 'JPY',
    }));
    app.UpdateProjectMeta = vi.fn(async () => {
      throw new Error('reporting currency cannot change after a Cost Control ledger entry');
    });
    const { findByLabelText, findByRole, findByText } = render(ProjectSettings);
    const currency = (await findByLabelText(/reporting currency/i)) as HTMLSelectElement;

    await fireEvent.change(currency, { target: { value: 'USD' } });
    await fireEvent.click(await findByRole('button', { name: /^save details$/i }));

    await findByText(/save failed:.*reporting currency cannot change/i);
    expect(currency).toHaveValue('JPY');
    expect(within(currency).getByRole('option', { name: /legacy read-only/i })).toBeInTheDocument();
    expect(await findByText(/cost control changes are disabled; no values were converted/i)).toBeInTheDocument();
  });
});

describe('legacy Budget decimal transport', () => {
  it('submits an exact decimal string without a stale minor-unit field', async () => {
    app.GetProjectMeta = vi.fn(async () => ({
      id: 'project-1', name: 'Exact Budget Project', description: '', industry: '', sub_category: '', methodology: '',
      country_code: 'US', time_zone: 'America/Chicago', status: 'planning', phase: 'planning',
      budget: '90071992547409.91', currency_code: 'USD',
    }));
    app.UpdateProjectMeta = vi.fn(async (p: unknown) => p);
    app.UpdateProjectIndustry = vi.fn(async () => ({
      id: 'project-1', name: 'Exact Budget Project', description: '', industry: '', sub_category: '', methodology: '',
      country_code: 'US', time_zone: 'America/Chicago', status: 'planning', phase: 'planning',
      budget: '92233720368547758.07', currency_code: 'USD',
    }));
    const { container, findByRole } = render(ProjectSettings);
    await findByRole('tab', { name: /general/i });
    const budget = container.querySelector('input[inputmode="decimal"]');
    expect(budget).not.toBeNull();

    await fireEvent.input(budget!, { target: { value: '92233720368547758.07' } });
    await fireEvent.click(await findByRole('button', { name: /^save details$/i }));

    await waitFor(() => expect(app.UpdateProjectMeta).toHaveBeenCalledOnce());
    expect(app.UpdateProjectMeta).toHaveBeenCalledWith(expect.objectContaining({
      budget: '92233720368547758.07',
    }));
    expect((app.UpdateProjectMeta.mock.calls[0][0] as Record<string, unknown>).budget_minor_units).toBeUndefined();
  });
});

// Regression coverage for a defect found via packaged-GUI testing on
// 2026-08-18: UpdateProjectIndustry always returns a fresh server-stamped
// `updated_at` (internal/db/project.go's UpsertProject). The shared
// navigation/native-close guard's "Save and continue" button routes through
// session.svelte's saveAndContinueNavigation -> autosave.saveAll(), which is
// a different path from this component's own "Save details" button (that
// one calls save() directly and never touches autosave's bookkeeping). A
// registered snapshot that included updated_at never converged on the
// guarded path, so "Save and continue" looped forever reporting "Changes
// were made while saving" even though nothing further was edited. See
// docs/beta-release-backlog.md's "Prevent silent editor data loss" row for
// the live reproduction (dirty ProjectSettings -> "<- Dashboard" -> Save and
// continue, twice, same error both times).
describe('guarded-navigation save convergence (updated_at exclusion)', () => {
  function projectMeta(overrides: Record<string, unknown> = {}) {
    return {
      id: 'project-1',
      name: 'Convergence Project',
      description: 'Original description',
      industry: 'software',
      sub_category: '',
      methodology: 'scrum',
      country_code: 'US',
      time_zone: 'America/Chicago',
      status: 'planning',
      phase: 'planning',
      updated_at: '2026-01-01T00:00:00Z',
      ...overrides,
    };
  }

  it('reports clean and completes the pending navigation after "Save and continue", despite a fresh updated_at', async () => {
    app.GetProjectMeta = vi.fn(async () => projectMeta());
    app.UpdateProjectMeta = vi.fn(async (p: unknown) => p);
    app.UpdateProjectIndustry = vi.fn(async () =>
      projectMeta({ description: 'Edited description', updated_at: '2026-01-01T00:00:05Z' }),
    );

    const registerSpy = vi.spyOn(autosave, 'register');
    const { findByLabelText, findByRole } = render(ProjectSettings);
    await findByRole('tab', { name: /general/i });
    // register() runs at the tail of onMount's async chain (after the
    // calendar/font/scenario/encryption loads) -- later than the fields
    // themselves render. Editing before it captures its baseline snapshot
    // would bake the edit into the "clean" baseline instead of testing it.
    await waitFor(() => expect(registerSpy).toHaveBeenCalled());

    const description = await findByLabelText(/description/i);
    await fireEvent.input(description, { target: { value: 'Edited description' } });
    expect(autosave.hasDirty()).toBe(true);

    session.view = 'project_settings';
    requestNavigation('dashboard');
    expect(navigation.pending?.view).toBe('dashboard');

    await saveAndContinueNavigation();

    expect(app.UpdateProjectIndustry).toHaveBeenCalledOnce();
    expect(autosave.hasDirty()).toBe(false);
    expect(navigation.pending).toBeNull();
    expect(navigation.error).toBe('');
    expect(session.view).toBe('dashboard');
  });

  it('stays dirty, and leaves navigation pending, if the user edits again while a slow save is in flight', async () => {
    app.GetProjectMeta = vi.fn(async () => projectMeta());
    app.UpdateProjectMeta = vi.fn(async (p: unknown) => p);
    let resolveIndustry: (value: unknown) => void = () => {};
    app.UpdateProjectIndustry = vi.fn(
      () =>
        new Promise((resolve) => {
          resolveIndustry = resolve;
        }),
    );

    const registerSpy = vi.spyOn(autosave, 'register');
    const { findByLabelText, findByRole } = render(ProjectSettings);
    await findByRole('tab', { name: /general/i });
    await waitFor(() => expect(registerSpy).toHaveBeenCalled());

    const description = await findByLabelText(/description/i);
    await fireEvent.input(description, { target: { value: 'First edit' } });
    expect(autosave.hasDirty()).toBe(true);

    session.view = 'project_settings';
    requestNavigation('dashboard');
    const continuing = saveAndContinueNavigation();

    // A further edit lands while the save is still awaiting its response.
    await fireEvent.input(description, { target: { value: 'Second edit, made mid-save' } });
    resolveIndustry(projectMeta({ description: 'First edit', updated_at: '2026-01-01T00:00:05Z' }));
    await continuing;

    expect(autosave.hasDirty()).toBe(true);
    expect(navigation.pending).not.toBeNull();
    expect(session.view).toBe('project_settings');
    expect(navigation.error).toMatch(/save again/i);
    // The data-loss assertion proper: the mid-save edit must survive, not
    // be silently overwritten by the first save's (now-stale) response.
    expect(description).toHaveValue('Second edit, made mid-save');
  });
});

// Added 2026-08-19 alongside migrating the header "&larr; Dashboard" nav
// link (previously a raw <button>) to the new `nav` Button variant.
describe('ProjectSettings migrated header "&larr; Dashboard" button', () => {
  it('Button variant="nav"', async () => {
    const { getByText, findByRole } = render(ProjectSettings);
    await findByRole('tablist');

    const btn = getByText('← Dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'text-xs text-slate-400 hover:text-cyan-400 disabled:opacity-50'.split(/\s+/).sort(),
    );
  });
});

// A fake of the recovery-code backend whose status follows the calls made,
// so the gate's checks see what the real backend would report.
function codesBackend(initial: { unused: number; legacy: boolean }) {
  const state = { ...initial, accepted: false };
  return {
    RecoveryCodeStatus: vi.fn(async () => ({
      unused: state.unused,
      total: 8,
      legacy: state.legacy,
      encryption_ready: !state.legacy && (state.unused > 0 || state.accepted),
    })),
    PrepareRecoveryCodes: vi.fn(async () => ['AAAAAAAA-BBBBBBBB']),
    ConfirmRecoveryCodes: vi.fn(async () => {
      state.unused = 8;
      state.legacy = false;
    }),
    DiscardRecoveryCodes: vi.fn(async () => undefined),
    AcceptEncryptionWithoutRecoveryCodes: vi.fn(async () => {
      state.accepted = true;
    }),
    IssueRecoveryCodes: vi.fn(async () => ['ONE-STEP']),
  };
}

async function openProtection(backend: ReturnType<typeof codesBackend>, extra: Record<string, unknown> = {}) {
  Object.assign(app, backend, {
    IsProjectEncrypted: vi.fn(async () => false),
    EncryptProjectAtRest: vi.fn(async () => '/tmp/gopmgr/plan.gopmgr.pre-encryption.bak'),
    ...extra,
  });
  session.projectPath = '/tmp/gopmgr/plan.gopmgr';
  const utils = render(ProjectSettings);
  await utils.findByRole('tab', { name: /data protection/i });
  await switchTab(utils.container, /data protection/i);
  return utils;
}

async function saveNewCodes(utils: ReturnType<typeof render>) {
  await fireEvent.click(utils.getByRole('button', { name: 'Create new recovery codes' }));
  await fireEvent.input(utils.getByLabelText('Current password'), { target: { value: 'current-password' } });
  await fireEvent.click(utils.getByRole('button', { name: 'Create codes' }));
  await utils.findByText('AAAAAAAA-BBBBBBBB');
  await fireEvent.click(utils.getByLabelText('I have saved these codes somewhere safe.'));
  await fireEvent.click(utils.getByRole('button', { name: 'Use the new codes' }));
}

describe('database encryption and recovery codes', () => {
  it('encrypts at once for a user with working codes', async () => {
    const utils = await openProtection(codesBackend({ unused: 8, legacy: false }));
    await fireEvent.click(await utils.findByRole('button', { name: 'Encrypt database' }));
    expect(await utils.findByText('Database encrypted.')).toBeInTheDocument();
    expect(app.EncryptProjectAtRest).toHaveBeenCalledOnce();
    expect(utils.queryByRole('region', { name: /save a way back in/i })).not.toBeInTheDocument();
  });

  it('with legacy codes, guides through new codes and then encrypts, never offering to skip', async () => {
    const backend = codesBackend({ unused: 8, legacy: true });
    const utils = await openProtection(backend);
    await fireEvent.click(await utils.findByRole('button', { name: 'Encrypt database' }));

    const gate = await utils.findByRole('region', { name: /save a way back in/i });
    expect(gate).toHaveTextContent(/from an older version of GoPMgr/);
    expect(within(gate).queryByRole('button', { name: 'Continue without recovery codes' })).not.toBeInTheDocument();
    expect(app.EncryptProjectAtRest).not.toHaveBeenCalled();
    expect(utils.getByRole('button', { name: 'Encrypt database' })).toBeDisabled();

    await saveNewCodes(utils);
    expect(await utils.findByText('Database encrypted.')).toBeInTheDocument();
    expect(app.EncryptProjectAtRest).toHaveBeenCalledOnce();
    expect(backend.IssueRecoveryCodes).not.toHaveBeenCalled();
  });

  it('with no codes, encrypts after the user accepts going without them', async () => {
    const backend = codesBackend({ unused: 0, legacy: false });
    const utils = await openProtection(backend);
    await fireEvent.click(await utils.findByRole('button', { name: 'Encrypt database' }));

    const gate = await utils.findByRole('region', { name: /save a way back in/i });
    const skip = within(gate).getByRole('button', { name: 'Continue without recovery codes' });
    expect(skip).toBeDisabled();
    await fireEvent.click(skip);
    expect(backend.AcceptEncryptionWithoutRecoveryCodes).not.toHaveBeenCalled();

    await fireEvent.click(within(gate).getByLabelText(/projects I create or encrypt until I sign out/));
    await fireEvent.click(skip);
    expect(await utils.findByText('Database encrypted.')).toBeInTheDocument();
    expect(backend.AcceptEncryptionWithoutRecoveryCodes).toHaveBeenCalledOnce();
    expect(app.EncryptProjectAtRest).toHaveBeenCalledOnce();
  });

  it('does not encrypt when the guided step is cancelled', async () => {
    const utils = await openProtection(codesBackend({ unused: 0, legacy: false }));
    await fireEvent.click(await utils.findByRole('button', { name: 'Encrypt database' }));
    await fireEvent.click(await utils.findByRole('button', { name: 'Cancel' }));

    expect(utils.queryByRole('region', { name: /save a way back in/i })).not.toBeInTheDocument();
    expect(utils.getByRole('button', { name: 'Encrypt database' })).toBeEnabled();
    expect(app.EncryptProjectAtRest).not.toHaveBeenCalled();
  });

  it('renewing codes from the panel alone does not encrypt, and Encrypt waits for unsaved codes', async () => {
    const utils = await openProtection(codesBackend({ unused: 0, legacy: false }));
    expect(await utils.findByText(/You have no unused recovery codes/)).toBeInTheDocument();

    await fireEvent.click(utils.getByRole('button', { name: 'Create new recovery codes' }));
    await fireEvent.input(utils.getByLabelText('Current password'), { target: { value: 'current-password' } });
    await fireEvent.click(utils.getByRole('button', { name: 'Create codes' }));
    await utils.findByText('AAAAAAAA-BBBBBBBB');
    const encrypt = utils.getByRole('button', { name: 'Encrypt database' });
    expect(encrypt).toBeDisabled();
    expect(utils.getByText('Save or discard your new recovery codes before encrypting.')).toBeInTheDocument();
    await fireEvent.click(encrypt);
    expect(app.EncryptProjectAtRest).not.toHaveBeenCalled();

    await fireEvent.click(utils.getByLabelText('I have saved these codes somewhere safe.'));
    await fireEvent.click(utils.getByRole('button', { name: 'Use the new codes' }));
    expect(await utils.findByText('New recovery codes saved. You can encrypt the database now.')).toBeInTheDocument();
    expect(app.EncryptProjectAtRest).not.toHaveBeenCalled();

    await fireEvent.click(utils.getByRole('button', { name: 'Encrypt database' }));
    await waitFor(() => expect(app.EncryptProjectAtRest).toHaveBeenCalledOnce());
  });

  it('shows the guided step, not the raw error, when the backend refuses after a stale check', async () => {
    const backend = codesBackend({ unused: 0, legacy: false });
    backend.RecoveryCodeStatus
      .mockResolvedValueOnce({ unused: 8, total: 8, legacy: false, encryption_ready: true })
      .mockResolvedValueOnce({ unused: 8, total: 8, legacy: false, encryption_ready: true });
    const refusal = 'Save recovery codes before creating or encrypting a project, or confirm that you understand it cannot be recovered if you forget your password.';
    const utils = await openProtection(backend, {
      EncryptProjectAtRest: vi.fn().mockRejectedValueOnce(new Error(refusal)),
    });
    await fireEvent.click(await utils.findByRole('button', { name: 'Encrypt database' }));

    expect(await utils.findByRole('region', { name: /save a way back in/i })).toBeInTheDocument();
    expect(utils.queryByText(refusal)).not.toBeInTheDocument();
  });
});
