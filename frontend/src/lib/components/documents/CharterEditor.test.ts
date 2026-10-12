// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';
import { tick } from 'svelte';
import { autosave } from '../../autosave.svelte';

import CharterEditor from './CharterEditor.svelte';
import { cancelNavigation, navigation, requestNavigation, session } from '../../session.svelte';

let app: Record<string, ReturnType<typeof vi.fn>>;
let register: ReturnType<typeof vi.spyOn>;

const document: DocumentRecord = {
  id: 'doc-1', project_id: 'p1', kind: 'charter', title: 'Project Charter',
  content: '{"purpose":"original"}', template_id: '', version: 1,
  status: 'draft', created_at: '', updated_at: '',
};

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((done, fail) => { resolve = done; reject = fail; });
  return { promise, resolve, reject };
}

function saved(record: DocumentRecord): DocumentRecord {
  return { ...record, version: record.version + 1, updated_at: 'server-update' };
}

beforeEach(() => {
  session.editingId = 'doc-1';
  session.view = 'charter';
  autosave.setInterval(0);
  register = vi.spyOn(autosave, 'register');
  app = {
    GetDocument: vi.fn(async () => ({ ...document })),
    SaveDocument: vi.fn(async (record: DocumentRecord) => saved(record)),
    ExportDocumentPDF: vi.fn(async () => '/fixture/export.pdf'),
    ListDocumentKinds: vi.fn(async () => [
      { kind: 'charter', name: 'Charter', phase: 'initiate', description: '',
        fields: [{ key: 'purpose', label: 'Purpose', type: 'string', required: false }] },
    ]),
  };
  (window as unknown as { go: unknown }).go = { main: { App: app } };
});

afterEach(() => {
  cleanup();
  // Remove registrations leaked by a fault-seeded old mount, after assertions.
  for (const result of register.mock.results) {
    if (result.type === 'return') result.value();
  }
  autosave.discardAll();
  autosave.setInterval(0);
  cancelNavigation();
  navigation.saving = false;
  navigation.error = '';
  vi.restoreAllMocks();
  session.editingId = null;
  session.view = 'portfolio';
});

async function mountEditor() {
  const utils = render(CharterEditor);
  await waitFor(() => expect(register).toHaveBeenCalledTimes(1));
  return utils;
}

describe('CharterEditor persistence and lifecycle', () => {
  it('normalizes acknowledged JSON formatting without a permanently dirty draft', async () => {
    app.SaveDocument.mockImplementationOnce(async (record: DocumentRecord) => ({
      ...saved(record), content: JSON.stringify(JSON.parse(record.content), null, 2),
    }));
    const utils = await mountEditor();
    await fireEvent.input(utils.getByLabelText('Purpose'), { target: { value: 'new purpose' } });
    expect(await autosave.saveAll()).toBe(true);
    expect(autosave.hasDirty()).toBe(false);
    expect(utils.getByLabelText('Purpose')).toHaveValue('new purpose');
    expect(utils.queryByText('Unsaved changes')).not.toBeInTheDocument();
  });
  for (const field of ['title', 'content', 'status'] as const) {
    it(`preserves ${field} edited back to its original value while saving`, async () => {
      const pending = deferred<DocumentRecord>();
      app.SaveDocument.mockReturnValueOnce(pending.promise);
      const utils = await mountEditor();
      const control = field === 'status'
        ? utils.getByRole('combobox')
        : utils.getByLabelText(field === 'title' ? 'Title' : 'Purpose');
      const original = field === 'title' ? document.title : field === 'content' ? 'original' : 'draft';
      const submitted = field === 'status' ? 'archived' : 'first edit';
      await fireEvent[field === 'status' ? 'change' : 'input'](control, { target: { value: submitted } });
      // Status changes initiate their own save; title/content use the toolbar.
      if (field !== 'status') await fireEvent.click(utils.getByRole('button', { name: 'Save', exact: true }));
      await waitFor(() => expect(app.SaveDocument).toHaveBeenCalledTimes(1));
      const payload = { ...app.SaveDocument.mock.calls[0][0] } as DocumentRecord;
      await fireEvent[field === 'status' ? 'change' : 'input'](control, { target: { value: original } });
      expect(app.SaveDocument).toHaveBeenCalledTimes(1);
      requestNavigation('dashboard');
      expect(navigation.pending?.view).toBe('dashboard');
      expect(session.view).toBe('charter');
      cancelNavigation();
      pending.resolve(saved(payload));
      await waitFor(() => expect(utils.getByRole('button', { name: 'Save', exact: true })).toBeEnabled());
      expect(control).toHaveValue(original);
      expect(utils.getByText('Unsaved changes')).toBeInTheDocument();
      expect(autosave.hasDirty()).toBe(true);
      expect(await autosave.saveAll()).toBe(true);
      const latest = app.SaveDocument.mock.calls[1][0] as DocumentRecord;
      expect(field === 'content' ? JSON.parse(latest.content).purpose : latest[field]).toBe(original);
      expect(latest.version).toBe(2);
      expect(autosave.hasDirty()).toBe(false);
      expect(utils.queryByText('Unsaved changes')).not.toBeInTheDocument();
    });
  }

  it('does not overlap a guarded save with keyboard or status saves', async () => {
    const pending = deferred<DocumentRecord>();
    app.SaveDocument.mockReturnValueOnce(pending.promise);
    const utils = await mountEditor();
    await fireEvent.input(utils.getByLabelText('Title'), { target: { value: 'first edit' } });
    const guard = autosave.saveAll();
    const payload = { ...app.SaveDocument.mock.calls[0][0] } as DocumentRecord;
    await fireEvent.keyDown(window, { key: 's', metaKey: true });
    await fireEvent.change(utils.getByRole('combobox'), { target: { value: 'archived' } });
    expect(app.SaveDocument).toHaveBeenCalledTimes(1);
    pending.resolve(saved(payload));
    expect(await guard).toBe(false);
    expect(autosave.hasDirty()).toBe(true);
    expect(await autosave.saveAll()).toBe(true);
    expect(app.SaveDocument.mock.calls[1][0].status).toBe('archived');
  });

  it('blocks export after late edits and exports the latest state after retry', async () => {
    const pending = deferred<DocumentRecord>();
    app.SaveDocument.mockReturnValueOnce(pending.promise);
    const utils = await mountEditor();
    await fireEvent.input(utils.getByLabelText('Title'), { target: { value: 'first edit' } });
    await fireEvent.click(utils.getByRole('button', { name: 'Export PDF', exact: true }));
    const payload = { ...app.SaveDocument.mock.calls[0][0] } as DocumentRecord;
    await fireEvent.input(utils.getByLabelText('Title'), { target: { value: 'late edit' } });
    pending.resolve(saved(payload));
    await waitFor(() => expect(utils.getByRole('button', { name: 'Save', exact: true })).toBeEnabled());
    expect(app.ExportDocumentPDF).not.toHaveBeenCalled();
    await fireEvent.click(utils.getByRole('button', { name: 'Export PDF', exact: true }));
    await waitFor(() => expect(app.ExportDocumentPDF).toHaveBeenCalledWith('doc-1'));
    expect(app.SaveDocument.mock.calls[1][0].title).toBe('late edit');
  });

  it('keeps a failed status save dirty and blocks export until retry', async () => {
    app.SaveDocument.mockRejectedValueOnce(new Error('fixture write failure'));
    const utils = await mountEditor();
    await fireEvent.change(utils.getByRole('combobox'), { target: { value: 'archived' } });
    await waitFor(() => expect(utils.getByText(/Save failed:.*fixture write failure/)).toBeInTheDocument());
    expect(autosave.hasDirty()).toBe(true);
    app.SaveDocument.mockRejectedValueOnce(new Error('still failing'));
    await fireEvent.click(utils.getByRole('button', { name: 'Export PDF', exact: true }));
    await waitFor(() => expect(utils.getByText(/Save failed:.*still failing/)).toBeInTheDocument());
    expect(app.ExportDocumentPDF).not.toHaveBeenCalled();
    expect(await autosave.saveAll()).toBe(true);
    expect(app.SaveDocument.mock.calls[2][0].status).toBe('archived');
  });

  for (const method of ['GetDocument', 'ListDocumentKinds'] as const) {
    for (const outcome of ['resolve', 'reject'] as const) {
      it(`does not register after unmount when ${method} ${outcome}s`, async () => {
        const pending = deferred<unknown>();
        app[method].mockReturnValueOnce(pending.promise);
        const utils = render(CharterEditor);
        await waitFor(() => expect(app[method]).toHaveBeenCalledTimes(1));
        utils.unmount();
        if (outcome === 'reject') pending.reject(new Error('late load failure'));
        else pending.resolve(method === 'GetDocument' ? { ...document } : []);
        await pending.promise.catch(() => undefined);
        await tick();
        expect(register).not.toHaveBeenCalled();
        if (method === 'GetDocument') expect(app.ListDocumentKinds).not.toHaveBeenCalled();
        expect(autosave.hasDirty()).toBe(false);
      });
    }
  }

  for (const outcome of ['resolve', 'reject'] as const) {
    it(`does not export or retain a draft when a save ${outcome}s after unmount`, async () => {
      const pending = deferred<DocumentRecord>();
      app.SaveDocument.mockReturnValueOnce(pending.promise);
      const utils = await mountEditor();
      await fireEvent.input(utils.getByLabelText('Title'), { target: { value: 'submitted' } });
      await fireEvent.click(utils.getByRole('button', { name: 'Export PDF', exact: true }));
      const payload = { ...app.SaveDocument.mock.calls[0][0] } as DocumentRecord;
      utils.unmount();
      if (outcome === 'resolve') pending.resolve(saved(payload));
      else pending.reject(new Error('late write failure'));
      await pending.promise.catch(() => undefined);
      await tick();
      expect(app.ExportDocumentPDF).not.toHaveBeenCalled();
      expect(autosave.hasDirty()).toBe(false);
      expect(register).toHaveBeenCalledTimes(1);
    });
  }
});

// Added 2026-08-19 alongside migrating the header "&larr; Dashboard" nav
// link (previously a raw <button>) to the `nav` Button variant — see
// Button.svelte's `nav` branch comment (tier-3 deferred file, closed this
// cycle). Independently fault-seeded (see TEST_COVERAGE_LEDGER.md) —
// covers the autosave-registering mount path, the most complex of this
// cycle's 7 files alongside SigmaProjectView.
describe('CharterEditor migrated header "&larr; Dashboard" button', () => {
  it('Button variant="nav"', async () => {
    const { getByText } = render(CharterEditor);
    await waitFor(() => expect(app.GetDocument).toHaveBeenCalled());

    const btn = getByText('← Dashboard');
    expect(btn.className.split(/\s+/).filter(Boolean).sort()).toEqual(
      'text-xs text-slate-400 hover:text-cyan-400 disabled:opacity-50'.split(/\s+/).sort(),
    );
  });
});
