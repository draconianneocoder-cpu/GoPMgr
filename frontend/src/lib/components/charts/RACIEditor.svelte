<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { desktopApi } from '../../desktop-api';
  // RACIEditor renders a Roles × Tasks grid with R/A/C/I dropdowns
  // per cell. The backend's LayoutRACI runs validation (each task
  // must have exactly one A; should have at least one R) and the
  // results render as a coloured tray under the table.
  //
  // Storage shape (in db.charts.data):
  //   {
  //     "roles": ["PM", "Dev", "QA"],
  //     "tasks": [{"id":"t1","title":"Plan"}, ...],
  //     "assignments": {
  //       "t1": {"PM": "A", "Dev": "R", "QA": "C"},
  //       ...
  //     }
  //   }

  import { onMount, onDestroy } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import { showToast } from '../../toast.svelte';
  import { autosave } from '../../autosave.svelte';
  import Button from '../Button.svelte';
  import { formatTime, t } from '../../i18n/i18n.svelte';

  interface RACITask {
    id: string;
    title: string;
    note?: string;
  }
  interface RACIDoc {
    roles: string[];
    tasks: RACITask[];
    assignments: Record<string, Record<string, string>>;
  }
  interface RACICell {
    task_id: string;
    role: string;
    value: string;
  }
  interface Validation {
    issues?: string[];
    error_count: number;
  }
  interface RACILayout {
    roles: string[];
    tasks: RACITask[];
    cells: RACICell[];
    validation: Validation;
  }

  const VALUES: { value: string; label: string; tone: string }[] = [
    { value: '', label: '—', tone: 'bg-slate-900 text-slate-500' },
    { value: 'R', label: 'R', tone: 'bg-cyan-900 text-cyan-200' },
    { value: 'A', label: 'A', tone: 'bg-emerald-900 text-emerald-200' },
    { value: 'C', label: 'C', tone: 'bg-amber-900 text-amber-200' },
    { value: 'I', label: 'I', tone: 'bg-slate-700 text-slate-200' },
  ];

  let chart = $state<ChartRecord | null>(null);
  let doc = $state<RACIDoc>({ roles: [], tasks: [], assignments: {} });
  let layout = $state<RACILayout>({ roles: [], tasks: [], cells: [], validation: { error_count: 0 } });
  let status = $state('');
  let saving = $state(false);
  // Set on every successful SaveChart (auto-persist and manual save alike).
  let lastSavedAt = $state<Date | null>(null);
  // Set when the initial GetChart fails: renders a full-screen error with
  // a way back instead of a stuck editor + unhandled promise rejection.
  let loadError = $state('');

  // Form state for new-role / new-task quick-add inputs.
  let newRole = $state('');
  let newTaskTitle = $state('');

  let stopAutosave: (() => void) | null = null;

  onMount(async () => {
    if (!session.editingId) return;
    try {
      chart = await desktopApi.GetChart(session.editingId);
    } catch (err: any) {
      loadError = t('chart.loadFailed', { error: String(err?.message ?? err) });
      return;
    }
    try {
      const parsed = JSON.parse(chart.data) as RACIDoc;
      doc = {
        roles: parsed.roles ?? [],
        tasks: parsed.tasks ?? [],
        assignments: parsed.assignments ?? {},
      };
    } catch {
      doc = { roles: [], tasks: [], assignments: {} };
    }
    await refreshLayout();
    // Register for timed auto-save now the saved doc is loaded.
    stopAutosave = autosave.register(
      () => JSON.stringify(doc),
      () => save(),
    );
  });

  onDestroy(() => {
    stopAutosave?.();
  });

  async function refreshLayout() {
    if (!chart) return;
    try {
      const updated = await desktopApi.SaveChart({
        ...chart,
        data: JSON.stringify(doc),
      });
      chart = updated;
      lastSavedAt = new Date();
      const res = await desktopApi.LayoutChart(updated.id);
      layout = res.body as RACILayout;
    } catch (err: any) {
      status = t('raci.layoutFailed', { error: String(err) });
    }
  }

  function newTaskID(): string {
    return 't_' + Math.random().toString(36).slice(2, 7);
  }

  function addRole() {
    const name = newRole.trim();
    if (!name) return;
    if (doc.roles.includes(name)) {
      status = t('raci.roleExists', { name });
      return;
    }
    doc.roles.push(name);
    doc.roles = [...doc.roles];
    newRole = '';
    void refreshLayout();
  }
  function removeRole(name: string) {
    // Removing a role also deletes its column of assignments — snapshot for undo.
    const before = JSON.parse(JSON.stringify(doc)) as RACIDoc;
    doc.roles = doc.roles.filter((r) => r !== name);
    for (const task of doc.tasks) {
      const row = doc.assignments[task.id];
      if (row) delete row[name];
    }
    void refreshLayout();
    showToast(t('raci.roleDeleted', { name }), {
      type: 'info',
      undo: () => {
        doc = before;
        void refreshLayout();
      },
    });
  }

  function addTask() {
    const title = newTaskTitle.trim();
    if (!title) return;
    doc.tasks.push({ id: newTaskID(), title });
    doc.tasks = [...doc.tasks];
    newTaskTitle = '';
    void refreshLayout();
  }
  function removeTask(id: string) {
    const before = JSON.parse(JSON.stringify(doc)) as RACIDoc;
    doc.tasks = doc.tasks.filter((task) => task.id !== id);
    delete doc.assignments[id];
    void refreshLayout();
    showToast(t('chart.taskDeleted'), {
      type: 'info',
      undo: () => {
        doc = before;
        void refreshLayout();
      },
    });
  }
  function renameTask(id: string, title: string) {
    const task = doc.tasks.find((x) => x.id === id);
    if (task) task.title = title;
  }

  function getCell(taskId: string, role: string): string {
    return doc.assignments[taskId]?.[role] ?? '';
  }

  function setCell(taskId: string, role: string, value: string) {
    doc.assignments[taskId] ??= {};
    if (value === '') {
      delete doc.assignments[taskId][role];
    } else {
      doc.assignments[taskId][role] = value;
    }
    doc.assignments = { ...doc.assignments };
    void refreshLayout();
  }

  function valueTone(v: string): string {
    return VALUES.find((x) => x.value === v)?.tone ?? VALUES[0].tone;
  }

  async function save() {
    if (!chart) return false;
    saving = true;
    status = '';
    try {
      const updated = await desktopApi.SaveChart({
        ...chart,
        data: JSON.stringify(doc),
      });
      chart = updated;
      lastSavedAt = new Date();
      status = t('raci.savedAt', { time: formatTime(new Date()) });
      return true;
    } catch (err: any) {
      status = t('raci.saveFailed', { error: String(err) });
      return false;
    } finally {
      saving = false;
    }
  }
</script>

{#if loadError}
  <div class="min-h-screen bg-slate-950 text-slate-200 flex items-center justify-center">
    <div class="text-center space-y-4 px-6">
      <p class="text-sm text-red-400 break-words" role="alert">{loadError}</p>
      <Button variant="primary" size="md" onclick={() => goto('dashboard')}>
        {t('chart.backToDashboard')}
      </Button>
    </div>
  </div>
{:else}
<div class="min-h-screen bg-slate-950 text-slate-200">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <Button variant="nav" onclick={() => goto('dashboard')}>
        {t('chart.dashboard')}
      </Button>
      <h1 class="text-sm font-bold tracking-widest uppercase text-slate-50">{t('raci.title')}</h1>
      {#if layout.validation.error_count > 0}
        <span class="text-xs px-2 py-1 bg-red-900 text-red-200 rounded-full">
          {t('raci.issues', { count: layout.validation.error_count })}
        </span>
      {/if}
    </div>
    <div class="flex items-center gap-3">
      {#if lastSavedAt}
        <span class="text-[10px] text-slate-500 tabular-nums" title={t('chart.autosaveHint')}>
          {t('chart.savedAt', { time: formatTime(lastSavedAt) })}
        </span>
      {/if}
      <button
        onclick={save}
        disabled={saving}
        class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-3 py-1 rounded"
      >
        {saving ? t('common.saving') : t('common.save')}
      </button>
    </div>
  </header>

  <main class="p-6 space-y-6">
    {#if status}
      <p class="text-xs text-cyan-400" role="status" aria-live="polite">{status}</p>
    {/if}

    <!-- Add controls -->
    <section class="grid grid-cols-1 md:grid-cols-2 gap-4">
      <form
        onsubmit={(e) => { e.preventDefault(); addRole(); }}
        class="flex gap-2 items-end"
      >
        <label class="flex-1">
          <span class="text-xs text-slate-500 uppercase">{t('raci.newRole')}</span>
          <input
            bind:value={newRole}
            placeholder={t('raci.newRole.example')}
            class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          />
        </label>
        <button
          type="submit"
          class="bg-slate-800 hover:bg-slate-700 px-3 py-2 text-xs rounded"
        >
          {t('raci.addRole')}
        </button>
      </form>
      <form
        onsubmit={(e) => { e.preventDefault(); addTask(); }}
        class="flex gap-2 items-end"
      >
        <label class="flex-1">
          <span class="text-xs text-slate-500 uppercase">{t('raci.newTask')}</span>
          <input
            bind:value={newTaskTitle}
            placeholder={t('raci.newTask.example')}
            class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          />
        </label>
        <button
          type="submit"
          class="bg-slate-800 hover:bg-slate-700 px-3 py-2 text-xs rounded"
        >
          {t('chart.addTask')}
        </button>
      </form>
    </section>

    <!-- The matrix -->
    {#if doc.tasks.length === 0 || doc.roles.length === 0}
      <p class="text-sm text-slate-500 text-center py-8">
        {t('raci.empty')}
      </p>
    {:else}
      <div class="overflow-x-auto">
        <table class="w-full border border-slate-800 text-sm">
          <thead class="bg-slate-900">
            <tr>
              <th class="text-left p-2 border-b border-slate-800 sticky left-0 bg-slate-900">
                {t('chart.task')}
              </th>
              {#each doc.roles as role (role)}
                <th class="p-2 border-b border-slate-800 border-l text-center">
                  <div class="flex flex-col items-center gap-1">
                    <span>{role}</span>
                    <Button
                      variant="remove"
                      onclick={() => removeRole(role)}
                      class="text-[10px]"
                      aria-label={t('raci.removeRole', { role })} title={t('raci.removeRole', { role })}
                    >
                      {t('raci.remove')}
                    </Button>
                  </div>
                </th>
              {/each}
              <th class="p-2 border-b border-slate-800 w-8"></th>
            </tr>
          </thead>
          <tbody>
            {#each doc.tasks as task (task.id)}
              <tr>
                <td class="p-2 border-b border-slate-800 sticky left-0 bg-slate-950">
                  <input
                    value={task.title}
                    oninput={(e) => renameTask(task.id, (e.target as HTMLInputElement).value)}
                    onblur={refreshLayout}
                    aria-label={t('raci.taskName')}
                    class="w-full bg-transparent focus:bg-slate-900 px-2 py-1 rounded focus:outline focus:outline-cyan-500"
                  />
                </td>
                {#each doc.roles as role (role)}
                  {@const v = getCell(task.id, role)}
                  <td class="p-1 border-b border-slate-800 border-l text-center">
                    <select
                      value={v}
                      onchange={(e) => setCell(task.id, role, (e.target as HTMLSelectElement).value)}
                      aria-label={t('raci.cell', { task: task.title || t('chart.task'), role })}
                      class="w-full text-center font-bold rounded p-1 cursor-pointer border-none {valueTone(v)}"
                    >
                      {#each VALUES as opt (opt.value)}
                        <option value={opt.value}>{opt.label}</option>
                      {/each}
                    </select>
                  </td>
                {/each}
                <td class="p-1 border-b border-slate-800 text-center">
                  <Button
                    variant="remove"
                    onclick={() => removeTask(task.id)}
                    class="text-xs"
                    aria-label={t('raci.removeTask')} title={t('raci.removeTask')}
                  >
                    ×
                  </Button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>

      <!-- Validation tray -->
      {#if layout.validation.issues && layout.validation.issues.length > 0}
        <section class="border border-red-900 bg-red-950/40 rounded p-4">
          <h2 class="text-xs font-bold tracking-widest uppercase text-red-300 mb-2">
            {t('raci.validation')}
          </h2>
          <ul class="space-y-1 text-xs text-red-200">
            {#each layout.validation.issues as issue (issue)}
              <li>· {issue}</li>
            {/each}
          </ul>
        </section>
      {/if}

      <!-- Legend -->
      <section class="text-xs text-slate-500 space-y-1">
        <div><span class="text-cyan-300 font-bold">R</span> {t('raci.legend.responsible')}</div>
        <div><span class="text-emerald-300 font-bold">A</span> {t('raci.legend.accountable')}</div>
        <div><span class="text-amber-300 font-bold">C</span> {t('raci.legend.consulted')}</div>
        <div><span class="text-slate-200 font-bold">I</span> {t('raci.legend.informed')}</div>
      </section>
    {/if}
  </main>
</div>
{/if}
