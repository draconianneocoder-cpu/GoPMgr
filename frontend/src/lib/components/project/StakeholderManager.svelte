<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // StakeholderManager is the project-level address book. Each
  // stakeholder carries (name, role, organisation, contact, category,
  // hourly_rate, contract_value, notes). The budget rollup reads
  // hourly_rate × work-item assignee matches, and contract_value sums
  // for vendor rows.

  import { onMount, onDestroy } from 'svelte';
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
    editing = {
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
    };
  }

  function openExisting(s: Stakeholder) {
    editing = { ...s };
  }

  async function save() {
    if (!editing || !editing.name) return;
    busy = true;
    error = '';
    try {
      await window.go.main.App.SaveStakeholder(editing);
      editing = null;
      await refresh();
    } catch (err: any) {
      error = `Save failed: ${err}`;
    } finally {
      busy = false;
    }
  }

  async function destroy(s: Stakeholder) {
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

  onDestroy(() => {});
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
    onkeydown={(e) => e.key === 'Escape' && (editing = null)}
    tabindex="-1"
  >
    <div class="w-full max-w-xl bg-slate-900 border border-slate-700 rounded-xl shadow-2xl">
      <header class="px-5 py-3 border-b border-slate-800 flex items-center justify-between">
        <h2 class="text-sm font-bold tracking-widest uppercase text-slate-50">
          {editing.id ? 'Edit stakeholder' : 'New stakeholder'}
        </h2>
        <button onclick={() => (editing = null)} class="text-slate-500 hover:text-slate-200">×</button>
      </header>
      <div class="p-5 space-y-3 max-h-[70vh] overflow-y-auto">
        <div class="grid grid-cols-2 gap-3">
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Name</span>
            <input
              bind:value={editing.name}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Category</span>
            <select bind:value={editing.category} class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded">
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
              placeholder="e.g. Tech lead"
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Organisation</span>
            <input
              bind:value={editing.organisation}
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
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Phone</span>
            <input
              bind:value={editing.phone}
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
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Contract value</span>
            <input
              type="number"
              step="100"
              bind:value={editing.contract_value}
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
            rows="3"
            class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          ></textarea>
        </label>
      </div>
      <footer class="px-5 py-3 border-t border-slate-800 flex justify-end gap-2">
        <button onclick={() => (editing = null)} class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded">
          Cancel
        </button>
        <button
          onclick={save}
          disabled={busy || !editing.name}
          class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-3 py-1 rounded"
        >
          {busy ? 'Saving…' : 'Save'}
        </button>
      </footer>
    </div>
  </div>
{/if}
