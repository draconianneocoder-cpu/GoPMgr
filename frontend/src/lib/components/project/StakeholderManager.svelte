<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // StakeholderManager is the project-level address book. Each
  // stakeholder carries (name, role, organisation, contact, category,
  // hourly_rate, contract_value, notes). The budget rollup reads
  // hourly_rate × work-item assignee matches, and contract_value sums
  // for vendor rows.

  import { onDestroy, onMount } from 'svelte';
  import { autosave } from '../../autosave.svelte';
  import { session, goto } from '../../session.svelte';
  import Spinner from '../Spinner.svelte';

  let list = $state<Stakeholder[]>([]);
  let filter = $state<'' | StakeholderCategory>('');
  let editing = $state<Stakeholder | null>(null);
  let busy = $state(false);
  let error = $state('');
  // True only until the first load resolves, so the "no stakeholders"
  // empty state cannot flash while the list is still being fetched.
  let loading = $state(true);

  // Snapshot of `editing` as of the last open. Compared against the live
  // value to decide whether closing needs confirmation.
  let original = $state<string | null>(null);
  let dirty = $derived(editing !== null && original !== null && JSON.stringify(editing) !== original);
  let stopDirtyGuard: (() => void) | null = null;

  onMount(async () => {
    await refresh();
  });

  async function refresh() {
    try {
      list = (await window.go.main.App.ListStakeholders(filter)) ?? [];
    } catch (err: any) {
      error = `Could not load stakeholders: ${err}`;
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    // Re-fetch when filter changes.
    filter;
    void refresh();
  });

  function openNew() {
    if (editing || busy) return;
    openEditor({
      id: '',
      project_id: session.project!.id,
      name: '',
      role: '',
      organisation: '',
      email: '',
      phone: '',
      category: 'team',
      hourly_rate: 0,
      contract_value: 0,
      availability: 1,
      notes: '',
      created_at: '',
      updated_at: '',
    });
  }

  function openExisting(s: Stakeholder) {
    if (editing || busy) return;
    openEditor({ ...s });
  }

  // The shared Save/Discard/Cancel guard only knows about registered drafts.
  // Remove its entry before clearing or replacing `editing` so it never sees
  // a transient `null` snapshot as a new unsaved change.
  function stopEditing() {
    stopDirtyGuard?.();
    stopDirtyGuard = null;
    editing = null;
    original = null;
  }

  function openEditor(next: Stakeholder) {
    if (editing || busy) return;
    stopEditing();
    editing = next;
    original = JSON.stringify(next);
    error = '';
    // A successful manual save closes this modal, so timed autosave would
    // dismiss it unexpectedly. Registration still protects all shared exits.
    stopDirtyGuard = autosave.register(
      () => JSON.stringify(editing),
      () => save(),
      false,
    );
  }

  function savePayload(stakeholder: Stakeholder, baseline: Stakeholder): Stakeholder {
    // The database derives canonical minor units from major-unit form inputs
    // only when these fields are zero. Existing values must not silently
    // override a later edit to the rendered rate/value inputs.
    const baselineHourlyMinorUnits = baseline.hourly_rate_minor_units ?? 0;
    const baselineContractMinorUnits = baseline.contract_value_minor_units ?? 0;
    return {
      ...stakeholder,
      hourly_rate_minor_units: stakeholder.hourly_rate === baselineHourlyMinorUnits / 100
        ? stakeholder.hourly_rate_minor_units
        : 0,
      contract_value_minor_units: stakeholder.contract_value === baselineContractMinorUnits / 100
        ? stakeholder.contract_value_minor_units
        : 0,
    };
  }

  function hasUnsafeMinorUnits(stakeholder: Stakeholder): boolean {
    return !Number.isSafeInteger(stakeholder.hourly_rate_minor_units ?? 0)
      || !Number.isSafeInteger(stakeholder.contract_value_minor_units ?? 0);
  }

  function rebaseEditableChanges(
    saved: Stakeholder,
    savingStakeholder: Stakeholder,
    latestStakeholder: Stakeholder,
  ): Stakeholder {
    return {
      ...saved,
      name: latestStakeholder.name !== savingStakeholder.name ? latestStakeholder.name : saved.name,
      role: latestStakeholder.role !== savingStakeholder.role ? latestStakeholder.role : saved.role,
      organisation: latestStakeholder.organisation !== savingStakeholder.organisation
        ? latestStakeholder.organisation
        : saved.organisation,
      email: latestStakeholder.email !== savingStakeholder.email ? latestStakeholder.email : saved.email,
      phone: latestStakeholder.phone !== savingStakeholder.phone ? latestStakeholder.phone : saved.phone,
      category: latestStakeholder.category !== savingStakeholder.category
        ? latestStakeholder.category
        : saved.category,
      hourly_rate: latestStakeholder.hourly_rate !== savingStakeholder.hourly_rate
        ? latestStakeholder.hourly_rate
        : saved.hourly_rate,
      contract_value: latestStakeholder.contract_value !== savingStakeholder.contract_value
        ? latestStakeholder.contract_value
        : saved.contract_value,
      availability: latestStakeholder.availability !== savingStakeholder.availability
        ? latestStakeholder.availability
        : saved.availability,
      notes: latestStakeholder.notes !== savingStakeholder.notes ? latestStakeholder.notes : saved.notes,
    };
  }

  async function save(): Promise<boolean> {
    if (!editing || busy) return false;
    if (!editing.name.trim()) {
      error = 'Stakeholder name is required.';
      return false;
    }
    if (hasUnsafeMinorUnits(editing)) {
      error = 'This stakeholder has a money value too large to save safely in this version of the app.';
      return false;
    }
    // Keep the pre-canonicalization snapshot for late-edit detection. The
    // payload zeroes only a changed money field's minor units so the backend
    // recalculates that field from the major-unit value rendered in this modal.
    const savingStakeholder = { ...editing };
    const savingSnapshot = JSON.stringify(savingStakeholder);
    const baseline = original ? JSON.parse(original) as Stakeholder : savingStakeholder;
    busy = true;
    error = '';
    try {
      const saved = await window.go.main.App.SaveStakeholder(savePayload(savingStakeholder, baseline));
      const latestStakeholder = editing;
      original = JSON.stringify(saved);
      if (latestStakeholder && JSON.stringify(latestStakeholder) !== savingSnapshot) {
        editing = rebaseEditableChanges(saved, savingStakeholder, latestStakeholder);
        await refresh();
        return true;
      }
      stopEditing();
      await refresh();
      return true;
    } catch (err: any) {
      error = `Save failed: ${err}`;
      return false;
    } finally {
      busy = false;
    }
  }

  // Every path that discards `editing` (header close, Cancel, Escape) must
  // route through here rather than assigning `editing = null` directly, so
  // an edited-but-unsaved stakeholder is never dropped silently. No-ops
  // while a save is in flight so a close requested mid-save can't confirm
  // a "discard" for edits the in-flight request is about to persist anyway.
  function requestClose() {
    if (busy) return;
    if (dirty && !confirm('Discard unsaved changes to this stakeholder?')) return;
    stopEditing();
  }

  async function destroy(s: Stakeholder) {
    if (editing || busy) return;
    if (!confirm(`Delete ${s.name}?`)) return;
    try {
      await window.go.main.App.DeleteStakeholder(s.id);
      await refresh();
    } catch (err: any) {
      error = `Delete failed: ${err}`;
    }
  }

  function tone(cat: StakeholderCategory): string {
    switch (cat) {
      case 'vendor':   return 'bg-amber-900 text-amber-200';
      case 'sponsor':  return 'bg-emerald-900 text-emerald-200';
      case 'external': return 'bg-slate-700 text-slate-200';
      default:         return 'bg-cyan-900 text-cyan-200';
    }
  }

  onDestroy(() => stopDirtyGuard?.());
</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <button onclick={() => goto('dashboard')} class="text-xs text-slate-400 hover:text-cyan-400">
        &larr; Dashboard
      </button>
      <h1 class="text-sm font-bold tracking-widest uppercase text-slate-50">Stakeholders</h1>
      <span class="text-xs text-slate-500">{list.length}</span>
    </div>
    <div class="flex gap-2 items-center">
      <label class="text-xs text-slate-500 flex items-center gap-1">
        Show:
        <select
          bind:value={filter}
          class="bg-slate-900 border border-slate-800 px-2 py-1 rounded text-xs"
        >
          <option value="">All</option>
          <option value="team">Team</option>
          <option value="vendor">Vendor</option>
          <option value="sponsor">Sponsor</option>
          <option value="external">External</option>
        </select>
      </label>
      <button
        onclick={openNew}
        class="text-xs bg-cyan-600 hover:bg-cyan-500 text-white font-bold uppercase px-3 py-1 rounded"
      >
        + Stakeholder
      </button>
    </div>
  </header>

  <main class="p-6 max-w-4xl mx-auto">
    {#if error}
      <p class="text-xs text-red-400 mb-3" role="alert">{error}</p>
    {/if}

    {#if loading}
      <Spinner label="Loading stakeholders…" />
    {:else if list.length === 0}
      <p class="text-sm text-slate-500 text-center py-12">
        No stakeholders {filter ? `with category "${filter}"` : 'yet'}.
        Click <strong>+ Stakeholder</strong>.
      </p>
    {:else}
      <ul class="divide-y divide-slate-800 border border-slate-800 rounded">
        {#each list as s (s.id)}
          <li class="px-3 py-3 flex items-center gap-3 hover:bg-slate-900">
            <button
              onclick={() => openExisting(s)}
              class="flex-1 text-left min-w-0"
            >
              <div class="flex items-center gap-2">
                <span class="font-bold text-slate-50">{s.name}</span>
                <span class="text-[10px] px-2 py-0.5 rounded uppercase tracking-widest {tone(s.category)}">
                  {s.category}
                </span>
              </div>
              <div class="text-xs text-slate-500 mt-0.5">
                {s.role}{s.organisation ? ` · ${s.organisation}` : ''}
                {s.email ? ` · ${s.email}` : ''}
              </div>
              <div class="text-[10px] text-slate-500">
                {s.hourly_rate > 0 ? `${s.hourly_rate.toFixed(2)}/hr` : ''}
                {s.contract_value > 0 ? `${s.hourly_rate > 0 ? ' · ' : ''}${s.contract_value.toFixed(2)} contract` : ''}
              </div>
            </button>
            <button
              onclick={() => destroy(s)}
              class="text-xs text-slate-500 hover:text-red-400"
              aria-label="Delete stakeholder"
            >
              ×
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </main>
</div>

<!-- Edit modal -->
{#if editing}
  <div
    class="fixed inset-0 bg-black/60 z-40 flex items-center justify-center p-6"
    role="dialog"
    aria-modal="true"
    onkeydown={(e) => e.key === 'Escape' && requestClose()}
    tabindex="-1"
  >
    <div class="w-full max-w-xl bg-slate-900 border border-slate-700 rounded-xl shadow-2xl">
      <header class="px-5 py-3 border-b border-slate-800 flex items-center justify-between">
        <h2 class="text-sm font-bold tracking-widest uppercase text-slate-50">
          {editing.id ? 'Edit stakeholder' : 'New stakeholder'}
        </h2>
        <button
          onclick={requestClose}
          disabled={busy}
          class="text-slate-500 hover:text-slate-200"
          aria-label="Close"
        >×</button>
      </header>
      <div class="p-5 space-y-3 max-h-[70vh] overflow-y-auto">
        <div class="grid grid-cols-2 gap-3">
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Name</span>
            <input
              bind:value={editing.name}
              disabled={busy}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Category</span>
            <select bind:value={editing.category} disabled={busy} class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded">
              <option value="team">Team</option>
              <option value="vendor">Vendor</option>
              <option value="sponsor">Sponsor</option>
              <option value="external">External</option>
            </select>
          </label>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Role</span>
            <input
              bind:value={editing.role}
              disabled={busy}
              placeholder="e.g. Tech lead"
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Organisation</span>
            <input
              bind:value={editing.organisation}
              disabled={busy}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Email</span>
            <input
              type="email"
              bind:value={editing.email}
              disabled={busy}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Phone</span>
            <input
              bind:value={editing.phone}
              disabled={busy}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Hourly rate</span>
            <input
              type="number"
              step="0.5"
              bind:value={editing.hourly_rate}
              disabled={busy}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Contract value</span>
            <input
              type="number"
              step="100"
              bind:value={editing.contract_value}
              disabled={busy}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
        </div>
        <label class="block">
          <span class="text-xs text-slate-500 uppercase">Availability (units)</span>
          <input
            type="number"
            min="0.1"
            max="10"
            step="0.1"
            bind:value={editing.availability}
            disabled={busy}
            class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          />
          <span class="text-[10px] text-slate-500">
            Resource capacity for scheduling: 1 = full-time, 0.5 =
            half-time, 2 = a two-person pool. Overallocation flags and
            resource levelling use this.
          </span>
        </label>
        <label class="block">
          <span class="text-xs text-slate-500 uppercase">Notes</span>
          <textarea
            bind:value={editing.notes}
            disabled={busy}
            rows="3"
            class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          ></textarea>
        </label>
      </div>
      <footer class="px-5 py-3 border-t border-slate-800 flex justify-end gap-2">
        <button onclick={requestClose} disabled={busy} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded">
          Cancel
        </button>
        <button
          onclick={save}
          disabled={busy || !editing.name.trim()}
          class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-3 py-1 rounded"
        >
          {busy ? 'Saving…' : 'Save'}
        </button>
      </footer>
    </div>
  </div>
{/if}
